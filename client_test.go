package xai

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/ifitsmanu/xai-sdk-go/internal/rpc"
)

func TestNewClientRequiresKey(t *testing.T) {
	t.Setenv("XAI_API_KEY", "")
	if _, err := NewClient(); err == nil {
		t.Fatal("expected missing API key error")
	}
}

func TestNormalizeHost(t *testing.T) {
	if got := normalizeHost(""); got != "api.x.ai:443" {
		t.Fatalf("default host = %s", got)
	}
	if got := normalizeHost("api.x.ai"); got != "api.x.ai:443" {
		t.Fatalf("host = %s", got)
	}
	if got := normalizeHost("localhost:8080"); got != "localhost:8080" {
		t.Fatalf("host = %s", got)
	}
}

func TestInsecureRequiresLoopback(t *testing.T) {
	t.Setenv("XAI_API_KEY", "test-key")
	if _, err := NewClient(WithInsecure(), WithHost("api.x.ai")); err != ErrInsecureHost {
		t.Fatalf("err = %v", err)
	}
	if !isLoopback("127.0.0.1:9") || !isLoopback("[::1]:443") || !isLoopback("localhost:443") || isLoopback("api.x.ai:443") {
		t.Fatal("loopback check failed")
	}
}

func TestLogLevelFiltersClientLogger(t *testing.T) {
	var buf bytes.Buffer
	base := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(rpc.LevelHandler{Min: LevelInfo, Next: base})
	logger.Debug("nope")
	logger.Info("yes")
	if strings.Contains(buf.String(), "nope") {
		t.Fatalf("debug leaked: %s", buf.String())
	}
	if !strings.Contains(buf.String(), "yes") {
		t.Fatalf("info missing: %s", buf.String())
	}
}
