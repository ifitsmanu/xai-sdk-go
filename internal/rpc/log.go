package rpc

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

const maxBodyLog = 512

// LevelHandler drops records below min before forwarding them.
type LevelHandler struct {
	Min  slog.Level
	Next slog.Handler
}

func (h LevelHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= h.Min && (h.Next == nil || h.Next.Enabled(ctx, level))
}

func (h LevelHandler) Handle(ctx context.Context, rec slog.Record) error {
	if h.Next == nil || !h.Enabled(ctx, rec.Level) {
		return nil
	}
	return h.Next.Handle(ctx, rec)
}

func (h LevelHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if h.Next == nil {
		return h
	}
	return LevelHandler{Min: h.Min, Next: h.Next.WithAttrs(attrs)}
}

func (h LevelHandler) WithGroup(name string) slog.Handler {
	if h.Next == nil {
		return h
	}
	return LevelHandler{Min: h.Min, Next: h.Next.WithGroup(name)}
}

type nopHandler struct{}

func (nopHandler) Enabled(context.Context, slog.Level) bool  { return false }
func (nopHandler) Handle(context.Context, slog.Record) error { return nil }
func (nopHandler) WithAttrs([]slog.Attr) slog.Handler        { return nopHandler{} }
func (nopHandler) WithGroup(string) slog.Handler             { return nopHandler{} }

// NopLogger discards every record.
func NopLogger() *slog.Logger { return slog.New(nopHandler{}) }

// UnaryLogger logs one line when a unary RPC finishes.
// Debug logs the start. Info logs success. Warn logs cancel and deadline.
// Error logs every other status. Bodies are logged only at debug when logBodies is set.
func UnaryLogger(logger *slog.Logger, logBodies bool) grpc.UnaryClientInterceptor {
	if logger == nil {
		logger = NopLogger()
	}
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		if logger.Enabled(ctx, slog.LevelDebug) {
			logger.DebugContext(ctx, "grpc call started", "rpc.system", "grpc", "rpc.method", method)
		}
		start := time.Now()
		err := invoker(ctx, method, req, reply, cc, opts...)
		logFinish(ctx, logger, method, time.Since(start), err)
		if logBodies {
			logBodiesAtDebug(ctx, logger, method, req, reply)
		}
		return err
	}
}

// StreamLogger logs the start and the terminal recv or send error of a stream.
func StreamLogger(logger *slog.Logger, logBodies bool) grpc.StreamClientInterceptor {
	if logger == nil {
		logger = NopLogger()
	}
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		// A disabled logger skips the wrapper so Recv does not pay an extra call.
		if !logBodies && !logger.Enabled(ctx, slog.LevelError) {
			return streamer(ctx, desc, cc, method, opts...)
		}
		if logger.Enabled(ctx, slog.LevelDebug) {
			logger.DebugContext(ctx, "grpc stream started", "rpc.system", "grpc", "rpc.method", method, "client_stream", desc.ClientStreams, "server_stream", desc.ServerStreams)
		}
		start := time.Now()
		cs, err := streamer(ctx, desc, cc, method, opts...)
		if err != nil {
			logFinish(ctx, logger, method, time.Since(start), err)
			return nil, err
		}
		return &loggedStream{ClientStream: cs, ctx: ctx, logger: logger, method: method, start: start, logBodies: logBodies}, nil
	}
}

type loggedStream struct {
	grpc.ClientStream
	ctx       context.Context
	logger    *slog.Logger
	method    string
	start     time.Time
	logBodies bool
	once      sync.Once
}

func (s *loggedStream) SendMsg(m any) error {
	err := s.ClientStream.SendMsg(m)
	if err != nil {
		s.finish(err)
	}
	if s.logBodies && err == nil {
		logBodiesAtDebug(s.ctx, s.logger, s.method, m, nil)
	}
	return err
}

func (s *loggedStream) RecvMsg(m any) error {
	err := s.ClientStream.RecvMsg(m)
	if err != nil {
		if errors.Is(err, io.EOF) {
			s.finish(nil)
		} else {
			s.finish(err)
		}
		return err
	}
	if s.logBodies {
		logBodiesAtDebug(s.ctx, s.logger, s.method, nil, m)
	}
	return nil
}

func (s *loggedStream) finish(err error) {
	s.once.Do(func() {
		logFinish(s.ctx, s.logger, s.method, time.Since(s.start), err)
	})
}

func logFinish(ctx context.Context, logger *slog.Logger, method string, d time.Duration, err error) {
	code := codes.OK
	level := slog.LevelInfo
	if err != nil {
		code = status.Code(err)
		if code == codes.Canceled || code == codes.DeadlineExceeded {
			level = slog.LevelWarn
		} else {
			level = slog.LevelError
		}
	}
	if logger == nil || !logger.Enabled(ctx, level) {
		return
	}
	attrs := []any{
		"rpc.system", "grpc",
		"rpc.method", method,
		"rpc.grpc.status_code", code.String(),
		"duration", d,
	}
	if err != nil {
		attrs = append(attrs, "error", status.Convert(err).Message())
	}
	msg := "grpc call finished"
	if err != nil {
		msg = "grpc call ended"
		if level == slog.LevelError {
			msg = "grpc call failed"
		}
	}
	logger.Log(ctx, level, msg, attrs...)
}

func logBodiesAtDebug(ctx context.Context, logger *slog.Logger, method string, req, reply any) {
	if !logger.Enabled(ctx, slog.LevelDebug) {
		return
	}
	attrs := []any{"rpc.method", method}
	if req != nil {
		attrs = append(attrs, "request", truncateProto(req))
	}
	if reply != nil {
		attrs = append(attrs, "response", truncateProto(reply))
	}
	logger.DebugContext(ctx, "grpc payload", attrs...)
}

func truncateProto(v any) string {
	msg, ok := v.(proto.Message)
	if !ok || msg == nil {
		return ""
	}
	b, err := protojson.Marshal(msg)
	if err != nil {
		return ""
	}
	if len(b) > maxBodyLog {
		return string(b[:maxBodyLog]) + "…"
	}
	return string(b)
}
