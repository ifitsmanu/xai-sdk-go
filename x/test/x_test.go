package x_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ifitsmanu/xai-sdk-go/internal/rest"
	"github.com/ifitsmanu/xai-sdk-go/tool"
	"github.com/ifitsmanu/xai-sdk-go/x"
	"golang.org/x/oauth2"
)

// An expired user token is refreshed on first use, saved with mode 0600, and sent.
func TestRefreshAndSave(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/2/oauth2/token":
			_ = r.ParseForm()
			if r.Form.Get("grant_type") != "refresh_token" || r.Form.Get("refresh_token") != "r1" {
				t.Errorf("token form = %v", r.Form)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"access_token":"fresh","refresh_token":"r2","token_type":"bearer","expires_in":7200}`)
		case "/2/users/me":
			if r.Header.Get("Authorization") != "Bearer fresh" {
				t.Errorf("auth = %q", r.Header.Get("Authorization"))
			}
			_, _ = io.WriteString(w, `{"data":{"id":"1","name":"Grok","username":"grok"}}`)
		}
	}))
	defer srv.Close()
	conf := x.OAuthConfig("client", "", "http://localhost/cb")
	conf.Endpoint.TokenURL = srv.URL + "/2/oauth2/token"
	expired := &oauth2.Token{AccessToken: "old", RefreshToken: "r1", Expiry: time.Now().Add(-time.Hour)}
	path := filepath.Join(t.TempDir(), "x-token.json")

	c := x.New(x.SaveTokens(path, conf.TokenSource(context.Background(), expired)), x.Options{Base: srv.URL})
	me, err := c.Me(context.Background())
	if err != nil || me.Username != "grok" {
		t.Fatalf("me = %+v, %v", me, err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("token file: %v %v", info, err)
	}
	if saved, _ := x.LoadToken(path); saved.RefreshToken != "r2" {
		t.Fatalf("saved = %+v", saved)
	}
}

// A read waits for X's rate-limit reset and retries; a post is never retried;
// a reset beyond MaxWait comes back as an error that says when to retry.
func TestRateLimits(t *testing.T) {
	var reads, posts atomic.Int32
	reset := func(w http.ResponseWriter, in time.Duration) {
		w.Header().Set("x-rate-limit-reset", strconv.FormatInt(time.Now().Add(in).Unix(), 10))
		w.WriteHeader(http.StatusTooManyRequests)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/2/tweets" && r.Method == http.MethodPost:
			posts.Add(1)
			reset(w, time.Second)
		case r.URL.Path == "/2/users/by/username/slow":
			reset(w, time.Hour)
		case reads.Add(1) == 1:
			reset(w, time.Second)
		default:
			_, _ = io.WriteString(w, `{"data":{"id":"7","username":"xai"}}`)
		}
	}))
	defer srv.Close()
	c := x.New(x.Bearer("app"), x.Options{Base: srv.URL, MaxWait: 5 * time.Second})
	ctx := context.Background()

	if u, err := c.UserByUsername(ctx, "@xai"); err != nil || u.ID != "7" || reads.Load() != 2 {
		t.Fatalf("user = %+v, %v after %d reads", u, err, reads.Load())
	}
	if _, err := c.CreatePost(ctx, x.NewPost{Text: "hi"}); err == nil || posts.Load() != 1 {
		t.Fatalf("post err = %v after %d tries", err, posts.Load())
	}
	start := time.Now()
	_, err := c.UserByUsername(ctx, "slow")
	var apiErr *rest.Error
	if !errors.As(err, &apiErr) || apiErr.RetryAfter < 50*time.Minute || time.Since(start) > 2*time.Second {
		t.Fatalf("err = %v, waited %v", err, time.Since(start))
	}
}

// Writes are absent unless asked for; when present they act as the token's user.
func TestToolsActAsUser(t *testing.T) {
	var liked map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/2/users/me":
			_, _ = io.WriteString(w, `{"data":{"id":"42","username":"me"}}`)
		case "/2/users/42/likes":
			_ = json.NewDecoder(r.Body).Decode(&liked)
			_, _ = io.WriteString(w, `{"data":{"liked":true}}`)
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()
	c := x.New(x.Bearer("user"), x.Options{Base: srv.URL})

	reads, err := x.Tools(c, x.ToolOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range tool.Functions(reads) {
		if f.Writes {
			t.Errorf("%s is a write in the read-only set", f.Name)
		}
	}
	all, err := x.Tools(c, x.ToolOptions{Writes: true})
	if err != nil {
		t.Fatal(err)
	}
	like := tool.Functions(all)["x_like"]
	if like == nil || !like.Writes {
		t.Fatalf("x_like = %+v", like)
	}
	if out, err := like.Run(context.Background(), json.RawMessage(`{"id":"99"}`)); err != nil || out != "done" || liked["tweet_id"] != "99" {
		t.Fatalf("like = %q, %v, body %v", out, err, liked)
	}
}
