package xai_test

import (
	"bytes"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/ifitsmanu/xai-sdk-go"
	"github.com/ifitsmanu/xai-sdk-go/internal/rpc"
)

func TestNewClientRequiresKey(t *testing.T) {
	t.Setenv("XAI_API_KEY", "")
	t.Setenv("XAI_MANAGEMENT_API_KEY", "")
	if _, err := xai.NewClient(); err == nil {
		t.Fatal("expected missing API key error")
	}
}

func TestInsecureRequiresLoopback(t *testing.T) {
	t.Setenv("XAI_API_KEY", "")
	t.Setenv("XAI_MANAGEMENT_API_KEY", "")
	if _, err := xai.NewClient(xai.WithAPIKey("test-key"), xai.WithInsecure(), xai.WithHost("api.x.ai")); !errors.Is(err, xai.ErrInsecureHost) {
		t.Fatalf("err = %v", err)
	}
	client, err := xai.NewClient(xai.WithAPIKey("test-key"), xai.WithInsecure(), xai.WithHost("127.0.0.1:1"))
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestLogLevelFiltersClientLogger(t *testing.T) {
	var buf bytes.Buffer
	base := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(rpc.LevelHandler{Min: xai.LevelInfo, Next: base})
	logger.Debug("nope")
	logger.Info("yes")
	if strings.Contains(buf.String(), "nope") {
		t.Fatalf("debug leaked: %s", buf.String())
	}
	if !strings.Contains(buf.String(), "yes") {
		t.Fatalf("info missing: %s", buf.String())
	}
}
