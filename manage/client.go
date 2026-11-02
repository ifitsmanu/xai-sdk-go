// Package manage calls https://management-api.x.ai.
// It uses a management key, which is a different secret from XAI_API_KEY.
package manage

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/ifitsmanu/xai-sdk-go/internal/rest"
)

// BaseURL is the management API origin.
const BaseURL = "https://management-api.x.ai"

// Client is the management API.
type Client struct {
	rest        *rest.Client
	Collections *Collections
	Billing     *Billing
	Audit       *Audit
	Auth        *Auth
}

// Config opens a management client.
type Config struct {
	Key       string
	Base      string
	UserAgent string
	Logger    *slog.Logger
	Timeout   time.Duration
	HTTP      *http.Client
}

// New returns a client. The key is not logged.
func New(cfg Config) *Client {
	if cfg.Base == "" {
		cfg.Base = BaseURL
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.New(slog.DiscardHandler)
	}
	rc := &rest.Client{
		Base:      cfg.Base,
		Key:       cfg.Key,
		UserAgent: cfg.UserAgent,
		HTTP:      cfg.HTTP,
		Logger:    cfg.Logger,
		Timeout:   cfg.Timeout,
	}
	c := &Client{rest: rc}
	c.Collections = &Collections{c: c}
	c.Billing = &Billing{c: c}
	c.Audit = &Audit{c: c}
	c.Auth = &Auth{c: c}
	return c
}

// Call sends a JSON request to a documented management path and decodes the body.
func (c *Client) Call(ctx context.Context, method, path string, in, out any) error {
	return c.rest.Call(ctx, method, path, in, out)
}
