package xai

import (
	"errors"
	"log/slog"
	"net"
	"os"
	"runtime"
	"strings"

	"github.com/ifitsmanu/xai-sdk-go/agent"
	"github.com/ifitsmanu/xai-sdk-go/auth"
	"github.com/ifitsmanu/xai-sdk-go/batch"
	"github.com/ifitsmanu/xai-sdk-go/chat"
	"github.com/ifitsmanu/xai-sdk-go/documents"
	"github.com/ifitsmanu/xai-sdk-go/embed"
	"github.com/ifitsmanu/xai-sdk-go/files"
	"github.com/ifitsmanu/xai-sdk-go/image"
	"github.com/ifitsmanu/xai-sdk-go/internal/rpc"
	"github.com/ifitsmanu/xai-sdk-go/models"
	"github.com/ifitsmanu/xai-sdk-go/sample"
	"github.com/ifitsmanu/xai-sdk-go/tokenize"
	"github.com/ifitsmanu/xai-sdk-go/video"
	"google.golang.org/grpc"
)

// ErrNoAPIKey is returned when no key is configured.
var ErrNoAPIKey = errors.New("xai: API key is empty; set XAI_API_KEY or use WithAPIKey")

// ErrInsecureHost is returned when WithInsecure is used for a non-loopback host.
var ErrInsecureHost = errors.New("xai: insecure dial is only allowed for localhost")

// Client is a connection to the xAI API plus one service client per API.
type Client struct {
	conn   *rpc.Conn
	logger *slog.Logger

	Chat      *chat.Service
	Image     *image.Service
	Video     *video.Service
	Files     *files.Service
	Models    *models.Service
	Batch     *batch.Service
	Auth      *auth.Service
	Tokenize  *tokenize.Service
	Embed     *embed.Service
	Documents *documents.Service
	Sample    *sample.Service
}

// NewClient dials the API. The key comes from WithAPIKey or XAI_API_KEY.
// The client stays quiet unless WithLogger or WithLogLevel is set.
// The API key is never written to the log.
func NewClient(opts ...Option) (*Client, error) {
	cfg := options{timeout: rpc.DefaultTimeout}
	for _, opt := range opts {
		opt(&cfg)
	}
	if cfg.apiKey == "" {
		cfg.apiKey = os.Getenv("XAI_API_KEY")
	}
	if cfg.apiKey == "" {
		return nil, ErrNoAPIKey
	}
	host := normalizeHost(cfg.host)
	if cfg.insecure && !isLoopback(host) {
		return nil, ErrInsecureHost
	}

	logger := cfg.logger
	switch {
	case logger == nil && !cfg.logSet && !cfg.logBodies:
		logger = rpc.NopLogger()
	case logger == nil:
		level := cfg.logLevel
		if !cfg.logSet {
			level = LevelDebug
		}
		logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
	default:
		min := cfg.logLevel
		if !cfg.logSet {
			min = LevelInfo
		}
		logger = slog.New(rpc.LevelHandler{Min: min, Next: logger.Handler()})
	}
	logger = logger.With("component", "xai-sdk")

	conn, err := rpc.Dial(rpc.Config{
		Host:      host,
		APIKey:    cfg.apiKey,
		Insecure:  cfg.insecure,
		Timeout:   cfg.timeout,
		UserAgent: "xai-sdk-go/" + Version,
		Logger:    logger,
		LogBodies: cfg.logBodies,
		Metadata:  sdkMetadata(cfg.metadata),
		DialOpts:  cfg.dialOpts,
	})
	if err != nil {
		return nil, err
	}
	return &Client{
		conn:      conn,
		logger:    logger,
		Chat:      chat.NewService(conn),
		Image:     image.NewService(conn),
		Video:     video.NewService(conn),
		Files:     files.NewService(conn),
		Models:    models.NewService(conn),
		Batch:     batch.NewService(conn),
		Auth:      auth.NewService(conn),
		Tokenize:  tokenize.NewService(conn),
		Embed:     embed.NewService(conn),
		Documents: documents.NewService(conn),
		Sample:    sample.NewService(conn),
	}, nil
}

// Close releases the underlying connection.
func (c *Client) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	c.logger.Info("client closed")
	return c.conn.Close()
}

// Logger is the logger used by gRPC interceptors and service helpers.
func (c *Client) Logger() *slog.Logger {
	if c == nil || c.logger == nil {
		return rpc.NopLogger()
	}
	return c.logger
}

// GRPC returns the underlying connection for callers that need a raw stub.
func (c *Client) GRPC() *grpc.ClientConn {
	if c == nil || c.conn == nil {
		return nil
	}
	return c.conn.GRPC
}

// Runner builds an agent runner that samples through this client's chat API
// and writes logs to the client logger.
func (c *Client) Runner(opts ...agent.Option) *agent.Runner {
	opts = append([]agent.Option{agent.WithLogger(c.Logger())}, opts...)
	return agent.NewRunner(c.Chat, opts...)
}

func normalizeHost(host string) string {
	host = strings.TrimSpace(host)
	if host == "" {
		return rpc.DefaultHost
	}
	if !strings.Contains(host, ":") {
		return host + ":443"
	}
	return host
}

func isLoopback(host string) bool {
	name, _, err := net.SplitHostPort(host)
	if err != nil {
		name = host
	}
	name = strings.Trim(name, "[]")
	if strings.EqualFold(name, "localhost") {
		return true
	}
	ip := net.ParseIP(name)
	return ip != nil && ip.IsLoopback()
}

func sdkMetadata(in map[string]string) map[string]string {
	out := make(map[string]string, len(in)+2)
	for k, v := range in {
		out[k] = v
	}
	out["xai-sdk-version"] = "go/" + Version
	out["xai-sdk-language"] = "go/" + goMinor()
	return out
}

func goMinor() string {
	v := strings.TrimPrefix(runtime.Version(), "go")
	parts := strings.Split(v, ".")
	if len(parts) >= 2 {
		return parts[0] + "." + parts[1]
	}
	return v
}
