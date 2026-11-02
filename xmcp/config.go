package xmcp

import (
	"encoding/json"
	"errors"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"time"

	"github.com/pelletier/go-toml/v2"
)

// ServerConfig is one MCP server: a Command for stdio or a URL for streamable HTTP.
type ServerConfig struct {
	Name           string
	Command        string
	Args           []string
	Env            map[string]string
	URL            string
	Headers        map[string]string
	StartupTimeout time.Duration // zero is DefaultStartupTimeout
	ToolTimeout    time.Duration // zero is DefaultToolTimeout
	Origin         string        // the file it was read from
}

// Defaults match Grok Build.
const (
	DefaultStartupTimeout = 30 * time.Second
	DefaultToolTimeout    = 6000 * time.Second
)

// XAIDocs is xAI's hosted documentation server. It needs no key.
var XAIDocs = ServerConfig{Name: "xai-docs", URL: "https://docs.x.ai/api/mcp"}

// Load reads MCP servers the way Grok Build does, so servers set up for Grok
// Build, Cursor, or Claude work unchanged. From lowest to highest priority:
// ~/.claude.json (top level and the entry for dir), ~/.cursor/mcp.json,
// .cursor/mcp.json and .mcp.json in dir, ~/.grok/config.toml, then each
// .grok/config.toml from the git root down to dir. A later server replaces an
// earlier one with the same name. ${VAR} and ${VAR:-default} expand from the
// environment. Missing files are skipped.
func Load(dir string) (map[string]ServerConfig, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	dir, err = filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	userGrok := filepath.Join(home, ".grok", "config.toml")
	compat, err := readCompat(userGrok)
	if err != nil {
		return nil, err
	}
	out := map[string]ServerConfig{}
	add := func(servers map[string]ServerConfig, err error) error {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		maps.Copy(out, servers)
		return err
	}
	var errs []error
	if compat.claude {
		errs = append(errs, add(readClaude(filepath.Join(home, ".claude.json"), dir)))
	}
	if compat.cursor {
		errs = append(errs, add(readJSON(filepath.Join(home, ".cursor", "mcp.json"))))
		errs = append(errs, add(readJSON(filepath.Join(dir, ".cursor", "mcp.json"))))
	}
	errs = append(errs, add(readJSON(filepath.Join(dir, ".mcp.json"))))
	errs = append(errs, add(readGrok(userGrok)))
	for _, d := range projectDirs(dir) {
		errs = append(errs, add(readGrok(filepath.Join(d, ".grok", "config.toml"))))
	}
	return out, errors.Join(errs...)
}

// projectDirs lists dir and its parents up to the git root, root first.
func projectDirs(dir string) []string {
	var dirs []string
	for d := dir; ; d = filepath.Dir(d) {
		dirs = append(dirs, d)
		if _, err := os.Stat(filepath.Join(d, ".git")); err == nil || filepath.Dir(d) == d {
			break
		}
	}
	slices.Reverse(dirs)
	return dirs
}

type rawServer struct {
	Type              string            `json:"type" toml:"type"`
	Command           string            `json:"command" toml:"command"`
	Args              []string          `json:"args" toml:"args"`
	Env               map[string]string `json:"env" toml:"env"`
	URL               string            `json:"url" toml:"url"`
	Headers           map[string]string `json:"headers" toml:"headers"`
	Disabled          bool              `json:"disabled" toml:"disabled"`
	StartupTimeoutSec float64           `json:"startup_timeout_sec" toml:"startup_timeout_sec"`
	ToolTimeoutSec    float64           `json:"tool_timeout_sec" toml:"tool_timeout_sec"`
}

func convert(raw map[string]rawServer, origin string) map[string]ServerConfig {
	out := map[string]ServerConfig{}
	for name, r := range raw {
		if r.Disabled {
			continue
		}
		out[name] = ServerConfig{
			Name:           name,
			Command:        expand(r.Command),
			Args:           expandAll(r.Args),
			Env:            expandMap(r.Env),
			URL:            expand(r.URL),
			Headers:        expandMap(r.Headers),
			StartupTimeout: time.Duration(r.StartupTimeoutSec * float64(time.Second)),
			ToolTimeout:    time.Duration(r.ToolTimeoutSec * float64(time.Second)),
			Origin:         origin,
		}
	}
	return out
}

func readGrok(path string) (map[string]ServerConfig, error) {
	var file struct {
		Servers map[string]rawServer `toml:"mcp_servers"`
	}
	if err := decode(path, toml.Unmarshal, &file); err != nil {
		return nil, err
	}
	return convert(file.Servers, path), nil
}

func readJSON(path string) (map[string]ServerConfig, error) {
	var file struct {
		Servers map[string]rawServer `json:"mcpServers"`
	}
	if err := decode(path, json.Unmarshal, &file); err != nil {
		return nil, err
	}
	return convert(file.Servers, path), nil
}

// readClaude reads the global servers and the servers for project dir.
func readClaude(path, dir string) (map[string]ServerConfig, error) {
	var file struct {
		Servers  map[string]rawServer `json:"mcpServers"`
		Projects map[string]struct {
			Servers map[string]rawServer `json:"mcpServers"`
		} `json:"projects"`
	}
	if err := decode(path, json.Unmarshal, &file); err != nil {
		return nil, err
	}
	out := convert(file.Servers, path)
	maps.Copy(out, convert(file.Projects[dir].Servers, path))
	return out, nil
}

type compat struct{ claude, cursor bool }

// readCompat reads [compat.claude] and [compat.cursor] mcps switches; both default on.
func readCompat(path string) (compat, error) {
	var file struct {
		Compat map[string]struct {
			MCPs *bool `toml:"mcps"`
		} `toml:"compat"`
	}
	err := decode(path, toml.Unmarshal, &file)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return compat{}, err
	}
	on := func(vendor string) bool {
		c, ok := file.Compat[vendor]
		return !ok || c.MCPs == nil || *c.MCPs
	}
	return compat{claude: on("claude"), cursor: on("cursor")}, nil
}

func decode(path string, unmarshal func([]byte, any) error, v any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := unmarshal(raw, v); err != nil {
		return &os.PathError{Op: "parse", Path: path, Err: err}
	}
	return nil
}

var variable = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)(?::-([^}]*))?\}`)

func expand(s string) string {
	return variable.ReplaceAllStringFunc(s, func(m string) string {
		parts := variable.FindStringSubmatch(m)
		if v, ok := os.LookupEnv(parts[1]); ok && v != "" {
			return v
		}
		return parts[2]
	})
}

func expandAll(in []string) []string {
	out := make([]string, len(in))
	for i, s := range in {
		out[i] = expand(s)
	}
	return out
}

func expandMap(in map[string]string) map[string]string {
	if in == nil {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = expand(v)
	}
	return out
}
