package xai

import (
	"crypto/tls"
	"log/slog"
	"time"

	"google.golang.org/grpc"
)

// Log levels match log/slog. Debug is the most verbose. Error is the least.
const (
	LevelDebug = slog.LevelDebug
	LevelInfo  = slog.LevelInfo
	LevelWarn  = slog.LevelWarn
	LevelError = slog.LevelError
)

// Option configures a Client.
type Option func(*options)

type options struct {
	apiKey        string
	host          string
	insecure      bool
	timeout       time.Duration
	metadata      map[string]string
	dialOpts      []grpc.DialOption
	logger        *slog.Logger
	logLevel      slog.Level
	logSet        bool
	logBodies     bool
	managementKey string
	clientCert    *tls.Certificate
}

// WithAPIKey sets the bearer token. The default is the XAI_API_KEY environment variable.
func WithAPIKey(key string) Option {
	return func(o *options) { o.apiKey = key }
}

// WithHost sets the API host. A missing port becomes 443. The default is api.x.ai:443.
// HostUS and HostMTLS are the other documented hosts.
func WithHost(host string) Option {
	return func(o *options) { o.host = host }
}

// WithManagementKey sets the management API key. The default is XAI_MANAGEMENT_API_KEY.
// Collections, billing, audit, and team keys use this secret, not XAI_API_KEY.
func WithManagementKey(key string) Option {
	return func(o *options) { o.managementKey = key }
}

// WithClientCertificate presents a client certificate to https://mtls.api.x.ai.
// Pair it with WithHost(HostMTLS).
func WithClientCertificate(cert tls.Certificate) Option {
	return func(o *options) { o.clientCert = &cert }
}

// WithInsecure dials without TLS. The host must be localhost.
func WithInsecure() Option {
	return func(o *options) { o.insecure = true }
}

// WithTimeout is the deadline applied when a call's context has none.
func WithTimeout(d time.Duration) Option {
	return func(o *options) { o.timeout = d }
}

// WithMetadata adds gRPC metadata to every request. Values are not logged.
func WithMetadata(md map[string]string) Option {
	return func(o *options) {
		if o.metadata == nil {
			o.metadata = map[string]string{}
		}
		for k, v := range md {
			o.metadata[k] = v
		}
	}
}

// WithDialOptions appends raw gRPC dial options after the SDK's interceptors.
func WithDialOptions(opts ...grpc.DialOption) Option {
	return func(o *options) { o.dialOpts = append(o.dialOpts, opts...) }
}

// WithLogger sets the structured logger. The API key is never written.
// WithLogLevel still filters this logger.
func WithLogger(logger *slog.Logger) Option {
	return func(o *options) { o.logger = logger }
}

// WithLogLevel sets the minimum level and turns logging on.
// Without WithLogger, records go to stderr.
//
//	LevelDebug  RPC start, tool arguments, poll progress
//	LevelInfo   RPC finished, uploads, video ready, agent run and tool calls
//	LevelWarn   canceled calls, deadlines, expired video results
//	LevelError  failed RPCs and failed generations
func WithLogLevel(level slog.Level) Option {
	return func(o *options) {
		o.logLevel = level
		o.logSet = true
	}
}

// WithLogBodies logs truncated protobuf JSON at debug level.
// Leave this off unless you are debugging a request. Bodies can contain prompts.
func WithLogBodies() Option {
	return func(o *options) { o.logBodies = true }
}
