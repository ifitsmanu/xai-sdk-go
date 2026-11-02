// Package rest calls the xAI HTTP APIs that are not gRPC methods.
package rest

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// ErrBodyTooLarge is returned when a response body is larger than Client.MaxBody.
var ErrBodyTooLarge = errors.New("xai: response body is larger than the read limit")

// Client is one authenticated HTTP origin.
type Client struct {
	Base      string
	Key       string
	UserAgent string
	HTTP      *http.Client
	Logger    *slog.Logger
	Timeout   time.Duration
	// MaxBody bounds a body read by Read. Larger bodies fail instead of being cut short.
	MaxBody int64
	// Attempts is the total tries for a bodiless GET or DELETE that fails with
	// a network error, 429, or 5xx. Requests with a body are never retried.
	Attempts int
	// MaxWait caps how long one retry waits for a rate limit to reset. A longer
	// wait returns the error instead; its RetryAfter says when to try again.
	// Zero is DefaultMaxWait.
	MaxWait time.Duration
}

// DefaultMaxWait is the longest a retry waits for a rate limit by default.
const DefaultMaxWait = time.Minute

// Error is a non-2xx HTTP response. The body is truncated and the key is not included.
type Error struct {
	Method     string
	Path       string
	Status     int
	Body       string
	RequestID  string
	RetryAfter time.Duration
}

func (e *Error) Error() string {
	if e.Body == "" {
		return fmt.Sprintf("xai: %s %s: status %d", e.Method, e.Path, e.Status)
	}
	return fmt.Sprintf("xai: %s %s: status %d: %s", e.Method, e.Path, e.Status, e.Body)
}

// Body is a request payload with its content type.
type Body struct {
	Reader      io.Reader
	ContentType string
}

// JSON encodes v as a request body. A nil v sends no body.
func JSON(v any) (*Body, error) {
	if v == nil {
		return nil, nil
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return &Body{Reader: bytes.NewReader(raw), ContentType: "application/json"}, nil
}

// Send performs an authenticated request and returns the open response.
// The caller closes the body. Closing it also releases the request timeout.
// A non-2xx status is returned as *Error with the body already closed.
func (c *Client) Send(ctx context.Context, method, path string, body *Body, accept string) (*http.Response, error) {
	if c == nil || c.Base == "" {
		return nil, errors.New("xai: REST base URL is empty")
	}
	ctx, cancel := c.context(ctx)
	attempts := 1
	if body == nil && (method == http.MethodGet || method == http.MethodDelete) {
		attempts = max(c.Attempts, 1)
	}
	for try := 1; ; try++ {
		resp, err := c.once(ctx, method, path, body, accept)
		if err == nil && resp.StatusCode < 300 {
			resp.Body = &cancelBody{ReadCloser: resp.Body, cancel: cancel}
			return resp, nil
		}
		if err == nil {
			err = responseError(method, path, resp)
		}
		wait := backoff(try, err)
		if try >= attempts || !retryable(err) || wait > cmp.Or(c.MaxWait, DefaultMaxWait) || !sleep(ctx, wait) {
			cancel()
			return nil, err
		}
	}
}

func (c *Client) once(ctx context.Context, method, path string, body *Body, accept string) (*http.Response, error) {
	var reader io.Reader
	if body != nil {
		reader = body.Reader
	}
	req, err := http.NewRequestWithContext(ctx, method, c.Base+path, reader)
	if err != nil {
		return nil, err
	}
	if c.Key != "" {
		req.Header.Set("Authorization", "Bearer "+c.Key)
	}
	if body != nil && body.ContentType != "" {
		req.Header.Set("Content-Type", body.ContentType)
	}
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	if c.UserAgent != "" {
		req.Header.Set("User-Agent", c.UserAgent)
	}
	httpClient := c.HTTP
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	start := time.Now()
	resp, err := httpClient.Do(req)
	if c.Logger != nil {
		status := 0
		if resp != nil {
			status = resp.StatusCode
		}
		c.Logger.DebugContext(ctx, "rest call", "method", method, "path", path, "status", status, "duration", time.Since(start), "err", err)
	}
	return resp, err
}

func responseError(method, path string, resp *http.Response) *Error {
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
	msg := strings.TrimSpace(string(raw))
	if len(msg) > 500 {
		msg = msg[:500]
	}
	e := &Error{Method: method, Path: path, Status: resp.StatusCode, Body: msg, RequestID: resp.Header.Get("x-request-id")}
	if secs, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil && secs > 0 {
		e.RetryAfter = time.Duration(secs) * time.Second
	} else if reset, err := strconv.ParseInt(resp.Header.Get("x-rate-limit-reset"), 10, 64); err == nil && resp.StatusCode == http.StatusTooManyRequests {
		e.RetryAfter = max(time.Until(time.Unix(reset, 0)), time.Second)
	}
	return e
}

// retryable is true for network errors, 429, and 5xx. A canceled context is not retried.
func retryable(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var e *Error
	if errors.As(err, &e) {
		return e.Status == http.StatusTooManyRequests || e.Status >= 500
	}
	return true
}

// backoff honors Retry-After, otherwise 100ms doubling to 2s with full jitter.
func backoff(try int, err error) time.Duration {
	var e *Error
	if errors.As(err, &e) && e.RetryAfter > 0 {
		return e.RetryAfter
	}
	ceiling := min(100*time.Millisecond<<(try-1), 2*time.Second)
	return time.Duration(rand.Int64N(int64(ceiling)) + 1)
}

func sleep(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

// Do sends one JSON request and returns the response body.
func (c *Client) Do(ctx context.Context, method, path string, in any) ([]byte, error) {
	raw, _, err := c.DoBody(ctx, method, path, in)
	return raw, err
}

// DoBody sends one JSON request and returns the body and its Content-Type.
func (c *Client) DoBody(ctx context.Context, method, path string, in any) ([]byte, string, error) {
	body, err := JSON(in)
	if err != nil {
		return nil, "", err
	}
	return c.Read(ctx, method, path, body)
}

// Read sends a request and reads the whole response body.
func (c *Client) Read(ctx context.Context, method, path string, body *Body) ([]byte, string, error) {
	resp, err := c.Send(ctx, method, path, body, "")
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = resp.Body.Close() }()
	limit := c.Limit()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, "", err
	}
	if int64(len(raw)) > limit {
		return nil, "", ErrBodyTooLarge
	}
	return raw, resp.Header.Get("Content-Type"), nil
}

// DefaultMaxBody is used when MaxBody is zero.
const DefaultMaxBody = 64 << 20

// Limit is MaxBody, or DefaultMaxBody when unset.
func (c *Client) Limit() int64 {
	if c == nil || c.MaxBody <= 0 {
		return DefaultMaxBody
	}
	return c.MaxBody
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

// Path joins escaped segments onto a base path: Path("/v1/responses", id) is /v1/responses/<id>.
func Path(base string, segments ...string) string {
	var b strings.Builder
	b.WriteString(base)
	for _, s := range segments {
		b.WriteByte('/')
		b.WriteString(url.PathEscape(s))
	}
	return b.String()
}

// Query appends encoded non-empty values to path. Keys are sorted.
func Query(path string, values url.Values) string {
	for k, v := range values {
		if len(v) == 0 || (len(v) == 1 && v[0] == "") {
			delete(values, k)
		}
	}
	if encoded := values.Encode(); encoded != "" {
		return path + "?" + encoded
	}
	return path
}

func (c *Client) context(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	if _, ok := ctx.Deadline(); ok || c.Timeout <= 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, c.Timeout)
}

type cancelBody struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (b *cancelBody) Close() error {
	err := b.ReadCloser.Close()
	b.cancel()
	return err
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
