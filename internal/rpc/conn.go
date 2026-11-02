// Package rpc dials the xAI API and logs every gRPC call.
package rpc

import (
	"cmp"
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/ifitsmanu/xai-sdk-go/internal/rest"
	"github.com/ifitsmanu/xai-sdk-go/internal/wsx"
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
	// DefaultMaxMessageSize bounds one gRPC message, REST body, or WebSocket message.
	DefaultMaxMessageSize = rest.DefaultMaxBody
	// DefaultAttempts is how many times an idempotent call is tried. gRPC caps this at 5.
	DefaultAttempts = 5
)

// Reads retry UNAVAILABLE. Calls that start work or spend quota do not.
const serviceConfigTemplate = `{
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
        "maxAttempts": %d,
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
	// TLS is the client TLS config. Set Certificates for mTLS.
	TLS *tls.Config
	// MaxMessageSize bounds gRPC messages, REST bodies, and WebSocket messages.
	MaxMessageSize int
	// Attempts is the total tries for idempotent calls. 1 turns retries off.
	Attempts int
}

// Conn is a logged, authenticated gRPC connection.
// APIKey is kept for the REST calls that the public gRPC API does not expose.
// It is never written to the log.
type Conn struct {
	GRPC      *grpc.ClientConn
	Timeout   time.Duration
	Logger    *slog.Logger
	APIKey    string
	RESTBase  string
	UserAgent string
	REST      *rest.Client
	// MaxMessageSize bounds WebSocket messages. Zero uses DefaultMaxMessageSize.
	MaxMessageSize int
}

// Dial opens a connection. Logger must be non-nil.
func Dial(cfg Config) (*Conn, error) {
	if cfg.Timeout <= 0 {
		cfg.Timeout = DefaultTimeout
	}
	if cfg.Logger == nil {
		cfg.Logger = NopLogger()
	}
	if cfg.MaxMessageSize <= 0 {
		cfg.MaxMessageSize = DefaultMaxMessageSize
	}
	if cfg.Attempts <= 0 {
		cfg.Attempts = DefaultAttempts
	}
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(transportCreds(cfg.Insecure, cfg.TLS)),
		grpc.WithPerRPCCredentials(apiKeyCredentials{
			key:      cfg.APIKey,
			metadata: cfg.Metadata,
			insecure: cfg.Insecure,
		}),
		grpc.WithUserAgent(cfg.UserAgent),
		grpc.WithDefaultServiceConfig(serviceConfig(cfg.Attempts)),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(cfg.MaxMessageSize),
			grpc.MaxCallSendMsgSize(cfg.MaxMessageSize),
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
	base := restBase(cfg.Host, cfg.Insecure)
	httpClient := &http.Client{Transport: httpTransport(cfg.TLS)}
	return &Conn{
		GRPC:           cc,
		Timeout:        cfg.Timeout,
		Logger:         cfg.Logger,
		APIKey:         cfg.APIKey,
		RESTBase:       base,
		UserAgent:      cfg.UserAgent,
		MaxMessageSize: cfg.MaxMessageSize,
		REST: &rest.Client{
			Base:      base,
			Key:       cfg.APIKey,
			UserAgent: cfg.UserAgent,
			HTTP:      httpClient,
			Logger:    cfg.Logger,
			Timeout:   cfg.Timeout,
			MaxBody:   int64(cfg.MaxMessageSize),
			Attempts:  cfg.Attempts,
		},
	}, nil
}

// GetJSON performs an authenticated REST GET. path is the URL path, such as /v1/models.
func (c *Conn) GetJSON(ctx context.Context, path string) ([]byte, error) {
	return c.RESTClient().Do(ctx, http.MethodGet, path, nil)
}

// CallJSON sends a JSON request to the inference REST API and decodes the response.
func (c *Conn) CallJSON(ctx context.Context, method, path string, in, out any) error {
	return c.RESTClient().Call(ctx, method, path, in, out)
}

// RESTClient is the HTTP client for this connection.
// A Conn built in tests without Dial still works when RESTBase and APIKey are set.
func (c *Conn) RESTClient() *rest.Client {
	if c == nil {
		return &rest.Client{}
	}
	if c.REST != nil {
		return c.REST
	}
	return &rest.Client{
		Base:      c.RESTBase,
		Key:       c.APIKey,
		UserAgent: c.UserAgent,
		Logger:    c.Logger,
		Timeout:   c.Timeout,
	}
}

func restBase(host string, insecureDial bool) string {
	scheme := "https"
	if insecureDial {
		scheme = "http"
	}
	if h, port, err := net.SplitHostPort(host); err == nil && (port == "443" || port == "80") {
		host = h
	}
	return scheme + "://" + host
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

func transportCreds(insecureDial bool, cfg *tls.Config) credentials.TransportCredentials {
	if insecureDial {
		return insecure.NewCredentials()
	}
	if cfg == nil {
		cfg = &tls.Config{MinVersion: tls.VersionTLS12}
	}
	return credentials.NewTLS(cfg)
}

func httpTransport(cfg *tls.Config) *http.Transport {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if cfg != nil {
		transport.TLSClientConfig = cfg
	}
	return transport
}

// serviceConfig retries UNAVAILABLE on reads. One attempt means no retry policy.
func serviceConfig(attempts int) string {
	if attempts < 2 {
		return "{}"
	}
	return fmt.Sprintf(serviceConfigTemplate, min(attempts, 5))
}

// DialWS opens a WebSocket on this connection's REST origin with its key and size limit.
func (c *Conn) DialWS(ctx context.Context, pathAndQuery string) (*wsx.Conn, error) {
	return wsx.Dial(ctx, rest.WebSocketBase(c.RESTBase)+pathAndQuery, c.APIKey, c.UserAgent, int64(cmp.Or(c.MaxMessageSize, DefaultMaxMessageSize)))
}
