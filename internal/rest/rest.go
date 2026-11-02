// Package rest calls the xAI HTTP APIs that are not gRPC methods.
package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// Client is one authenticated HTTP origin.
type Client struct {
	Base      string
	Key       string
	UserAgent string
	HTTP      *http.Client
	Logger    *slog.Logger
	Timeout   time.Duration
}

// Error is a non-2xx HTTP response. The body is truncated and the key is not included.
type Error struct {
	Method string
	Path   string
	Status int
	Body   string
}

func (e *Error) Error() string {
	if e.Body == "" {
		return fmt.Sprintf("xai: %s %s: status %d", e.Method, e.Path, e.Status)
	}
	return fmt.Sprintf("xai: %s %s: status %d: %s", e.Method, e.Path, e.Status, e.Body)
}

// Do sends one JSON request and returns the response body.
// A nil body sends no payload. The key is sent as a bearer token and is not logged.
func (c *Client) Do(ctx context.Context, method, path string, body any) ([]byte, error) {
	if c == nil || c.Base == "" {
		return nil, fmt.Errorf("xai: REST base URL is empty")
	}
	if c.Key == "" {
		return nil, fmt.Errorf("xai: API key is empty")
	}
	ctx, cancel := c.context(ctx)
	defer cancel()

	var payload io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		payload = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.Base+path, payload)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.Key)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.UserAgent != "" {
		req.Header.Set("User-Agent", c.UserAgent)
	}
	httpClient := c.HTTP
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(raw))
		if len(msg) > 300 {
			msg = msg[:300]
		}
		if c.Logger != nil {
			c.Logger.DebugContext(ctx, "rest call", "method", method, "path", path, "status", resp.StatusCode)
		}
		return nil, &Error{Method: method, Path: path, Status: resp.StatusCode, Body: msg}
	}
	return raw, nil
}

// Call sends a JSON request and decodes a JSON response. out may be nil.
func (c *Client) Call(ctx context.Context, method, path string, in, out any) error {
	raw, err := c.Do(ctx, method, path, in)
	if err != nil {
		return err
	}
	if out == nil || len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, out)
}

func (c *Client) context(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	if _, ok := ctx.Deadline(); ok || c.Timeout <= 0 {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, c.Timeout)
}

// WebSocketBase converts an https origin to wss, or http to ws.
func WebSocketBase(restBase string) string {
	switch {
	case strings.HasPrefix(restBase, "https://"):
		return "wss://" + strings.TrimPrefix(restBase, "https://")
	case strings.HasPrefix(restBase, "http://"):
		return "ws://" + strings.TrimPrefix(restBase, "http://")
	default:
		return restBase
	}
}
