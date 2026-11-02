package xmcp_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ifitsmanu/xai-sdk-go/xmcp"
)

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

// Servers load from the same files, in the same priority, as Grok Build.
func TestLoadMatchesGrokBuild(t *testing.T) {
	home, repo := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("DOCS_TOKEN", "abc")
	dir := filepath.Join(repo, "svc", "api")
	write(t, filepath.Join(repo, ".git", "HEAD"), "ref")

	write(t, filepath.Join(home, ".claude.json"), `{
		"mcpServers": {"shared": {"url": "https://claude.example/mcp"}, "claude-only": {"command": "claude-tool"}},
		"projects": {"`+dir+`": {"mcpServers": {"proj-claude": {"url": "https://p.example/mcp"}}}}}`)
	write(t, filepath.Join(home, ".cursor", "mcp.json"), `{"mcpServers": {"cursor-only": {"url": "https://cursor.example/mcp"}}}`)
	write(t, filepath.Join(dir, ".mcp.json"), `{"mcpServers": {"off": {"command": "x", "disabled": true}}}`)
	write(t, filepath.Join(home, ".grok", "config.toml"), `
[compat.cursor]
mcps = false

[mcp_servers.shared]
url = "https://grok.example/mcp"
headers = { Authorization = "Bearer ${DOCS_TOKEN}" }

[mcp_servers.local]
command = "npx"
args = ["-y", "server", "${MISSING:-/tmp}"]
startup_timeout_sec = 5
`)
	write(t, filepath.Join(repo, ".grok", "config.toml"), "[mcp_servers.local]\ncommand = \"repo-tool\"\n")
	write(t, filepath.Join(repo, "svc", ".grok", "config.toml"), "[mcp_servers.local]\ncommand = \"svc-tool\"\n")

	got, err := xmcp.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got["cursor-only"]; ok {
		t.Error("[compat.cursor] mcps = false was ignored")
	}
	if _, ok := got["off"]; ok {
		t.Error("a disabled server was loaded")
	}
	if got["claude-only"].Command != "claude-tool" || got["proj-claude"].URL != "https://p.example/mcp" {
		t.Errorf("claude servers = %+v %+v", got["claude-only"], got["proj-claude"])
	}
	if s := got["shared"]; s.URL != "https://grok.example/mcp" || s.Headers["Authorization"] != "Bearer abc" {
		t.Errorf("config.toml should win over ~/.claude.json and expand ${VAR}: %+v", s)
	}
	if got["local"].Command != "svc-tool" {
		t.Errorf("the nearest project .grok/config.toml should win: %q", got["local"].Command)
	}
}

func TestExpandDefault(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	write(t, filepath.Join(home, ".grok", "config.toml"), "[mcp_servers.a]\ncommand = \"run\"\nargs = [\"${NOPE:-fallback}\"]\nstartup_timeout_sec = 5\n")
	got, err := xmcp.Load(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if a := got["a"]; a.Args[0] != "fallback" || a.StartupTimeout.Seconds() != 5 {
		t.Errorf("a = %+v", a)
	}
}
