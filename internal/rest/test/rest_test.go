package rest_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/ifitsmanu/xai-sdk-go/internal/rest"
)

func server(t *testing.T, h http.HandlerFunc) *rest.Client {
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return &rest.Client{Base: srv.URL, Key: "k", Attempts: 3}
}

// A read that hits a transient 503 succeeds on the next attempt.
func TestGetRetriesTransientFailure(t *testing.T) {
	var calls atomic.Int32
	c := server(t, func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(`{}`))
	})
	if _, err := c.Do(context.Background(), http.MethodGet, "/v1/me", nil); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 {
		t.Fatalf("calls = %d, want 2", calls.Load())
	}
}

// A POST may have created or billed something, so it is never repeated.
func TestPostIsNotRetried(t *testing.T) {
	var calls atomic.Int32
	c := server(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	_, err := c.Do(context.Background(), http.MethodPost, "/v1/responses", map[string]string{"input": "hi"})
	var apiErr *rest.Error
	if !errors.As(err, &apiErr) || apiErr.Status != http.StatusServiceUnavailable {
		t.Fatalf("err = %v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("calls = %d, want 1", calls.Load())
	}
}

// A body over the limit fails loudly instead of returning truncated audio or JSON.
func TestOversizedBodyFails(t *testing.T) {
	c := server(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("x", 2048)))
	})
	c.MaxBody = 1024
	if _, err := c.Do(context.Background(), http.MethodGet, "/v1/tts/voices", nil); !errors.Is(err, rest.ErrBodyTooLarge) {
		t.Fatalf("err = %v, want ErrBodyTooLarge", err)
	}
}

func TestPathAndQueryEscape(t *testing.T) {
	got := rest.Query(rest.Path("/v1/responses", "a/b c", "input_items"), map[string][]string{"after": {"x&y"}, "order": {""}})
	if want := "/v1/responses/a%2Fb%20c/input_items?after=x%26y"; got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}
