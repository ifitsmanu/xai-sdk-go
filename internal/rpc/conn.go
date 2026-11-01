// Package rpc dials the xAI API and logs every gRPC call.
package rpc

import (
	"context"
	"crypto/tls"
	"log/slog"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

const (
	// DefaultHost is the production API endpoint.
	DefaultHost = "api.x.ai:443"
	// DefaultTimeout is applied when a call context has no deadline.
	DefaultTimeout = 27 * time.Minute
	maxMessageSize = 20 << 20
)

// Reads retry UNAVAILABLE. Calls that start work or spend quota do not.
const serviceConfig = `{
  "methodConfig": [
    {
      "name": [
        {"service": "xai_api.Chat", "method": "GetCompletion"},
        {"service": "xai_api.Chat", "method": "GetCompletionChunk"},
        {"service": "xai_api.Chat", "method": "StartDeferredCompletion"},
        {"service": "xai_api.Chat", "method": "DeleteStoredCompletion"},
        {"service": "xai_api.Chat", "method": "CompactContext"},
        {"service": "xai_api.Image", "method": "GenerateImage"},
        {"service": "xai_api.Video", "method": "GenerateVideo"},
        {"service": "xai_api.Video", "method": "ExtendVideo"},
        {"service": "xai_api.Files", "method": "UploadFile"},
        {"service": "xai_api.Files", "method": "DeleteFile"},
        {"service": "xai_api.Files", "method": "CreatePublicUrl"},
        {"service": "xai_api.Files", "method": "RevokePublicUrl"},
        {"service": "xai_api.Sample", "method": "SampleText"},
        {"service": "xai_api.Sample", "method": "SampleTextStreaming"},
        {"service": "xai_api.Embedder", "method": "Embed"},
        {"service": "xai_api.BatchMgmt", "method": "CreateBatch"},
        {"service": "xai_api.BatchMgmt", "method": "AddBatchRequests"},
        {"service": "xai_api.BatchMgmt", "method": "CancelBatch"}
      ]
    },
    {
      "name": [{}],
      "retryPolicy": {
        "maxAttempts": 5,
        "initialBackoff": "0.1s",
        "maxBackoff": "1s",
        "backoffMultiplier": 2,
        "retryableStatusCodes": ["UNAVAILABLE"]
      }
    }
  ]
}`

// Config is the dial input.
type Config struct {
	Host      string
	APIKey    string
	Insecure  bool
	Timeout   time.Duration
	UserAgent string
	Logger    *slog.Logger
	// LogBodies includes truncated protobuf payloads at debug level.
	LogBodies bool
	Metadata  map[string]string
	DialOpts  []grpc.DialOption
}

// Conn is a logged, authenticated gRPC connection.
type Conn struct {
	GRPC    *grpc.ClientConn
	Timeout time.Duration
	Logger  *slog.Logger
}

// Dial opens a connection. Logger must be non-nil.
func Dial(cfg Config) (*Conn, error) {
	if cfg.Timeout <= 0 {
		cfg.Timeout = DefaultTimeout
	}
	if cfg.Logger == nil {
		cfg.Logger = NopLogger()
	}
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(transportCreds(cfg.Insecure)),
		grpc.WithPerRPCCredentials(apiKeyCredentials{
			key:      cfg.APIKey,
			metadata: cfg.Metadata,
			insecure: cfg.Insecure,
		}),
		grpc.WithUserAgent(cfg.UserAgent),
		grpc.WithDefaultServiceConfig(serviceConfig),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(maxMessageSize),
			grpc.MaxCallSendMsgSize(maxMessageSize),
		),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                30 * time.Second,
			Timeout:             10 * time.Second,
			PermitWithoutStream: true,
		}),
		grpc.WithChainUnaryInterceptor(UnaryLogger(cfg.Logger, cfg.LogBodies)),
		grpc.WithChainStreamInterceptor(StreamLogger(cfg.Logger, cfg.LogBodies)),
	}
	opts = append(opts, cfg.DialOpts...)

	cc, err := grpc.NewClient(cfg.Host, opts...)
	if err != nil {
		return nil, err
	}
	cc.Connect()
	return &Conn{GRPC: cc, Timeout: cfg.Timeout, Logger: cfg.Logger}, nil
}

// Close closes the underlying connection.
func (c *Conn) Close() error {
	if c == nil || c.GRPC == nil {
		return nil
	}
	return c.GRPC.Close()
}

// Context applies the default timeout when ctx has no deadline.
func (c *Conn) Context(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	if _, ok := ctx.Deadline(); ok || c == nil || c.Timeout <= 0 {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, c.Timeout)
}

func transportCreds(insecureDial bool) credentials.TransportCredentials {
	if insecureDial {
		return insecure.NewCredentials()
	}
	return credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12})
}
