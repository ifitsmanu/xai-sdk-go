package rpc

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestLevelFilter(t *testing.T) {
	var buf bytes.Buffer
	base := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(LevelHandler{Min: slog.LevelInfo, Next: base})
	logger.Debug("hidden")
	logger.Info("shown", "rpc.method", "/xai_api.Chat/GetCompletion")
	if strings.Contains(buf.String(), "hidden") {
		t.Fatalf("debug leaked: %s", buf.String())
	}
	if !strings.Contains(buf.String(), "shown") {
		t.Fatalf("info missing: %s", buf.String())
	}
}

func TestUnaryLoggerLevels(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	interceptor := UnaryLogger(logger, false)

	_ = interceptor(context.Background(), "/xai_api.Chat/GetCompletion", nil, nil, nil,
		func(context.Context, string, any, any, *grpc.ClientConn, ...grpc.CallOption) error {
			return nil
		})
	out := buf.String()
	if !strings.Contains(out, "grpc call started") || !strings.Contains(out, "level=INFO") && !strings.Contains(out, "level=info") {
		if !strings.Contains(out, "grpc call finished") {
			t.Fatalf("success log missing: %s", out)
		}
	}
	if strings.Contains(strings.ToLower(out), "authorization") || strings.Contains(out, "Bearer") {
		t.Fatalf("credential leaked: %s", out)
	}

	buf.Reset()
	err := interceptor(context.Background(), "/xai_api.Chat/GetCompletion", nil, nil, nil,
		func(context.Context, string, any, any, *grpc.ClientConn, ...grpc.CallOption) error {
			return status.Error(codes.Unavailable, "down")
		})
	if status.Code(err) != codes.Unavailable {
		t.Fatalf("code = %s", status.Code(err))
	}
	if !strings.Contains(buf.String(), "ERROR") && !strings.Contains(buf.String(), "error") {
		t.Fatalf("error log missing: %s", buf.String())
	}
	if !strings.Contains(buf.String(), "Unavailable") {
		t.Fatalf("status missing: %s", buf.String())
	}
}

func TestWarnOnCancel(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	interceptor := UnaryLogger(logger, false)
	_ = interceptor(context.Background(), "/xai_api.Models/ListLanguageModels", nil, nil, nil,
		func(context.Context, string, any, any, *grpc.ClientConn, ...grpc.CallOption) error {
			return status.Error(codes.DeadlineExceeded, "timeout")
		})
	out := buf.String()
	if !strings.Contains(out, "WARN") && !strings.Contains(out, "warn") {
		t.Fatalf("warn missing: %s", out)
	}
}

func TestNopLogger(t *testing.T) {
	logger := NopLogger()
	if logger.Enabled(context.Background(), slog.LevelError) {
		t.Fatal("nop logger enabled")
	}
	logger.Info("quiet", "duration", time.Millisecond)
}
