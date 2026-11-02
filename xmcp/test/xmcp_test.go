package xmcp_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ifitsmanu/xai-sdk-go/tool"
	"github.com/ifitsmanu/xai-sdk-go/xmcp"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type bearer struct{ token string }

func (b bearer) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+b.token)
	return http.DefaultTransport.RoundTrip(r)
}

func connect(ctx context.Context, url, token string) (*mcp.ClientSession, error) {
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	return client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: url, HTTPClient: &http.Client{Transport: bearer{token}}}, nil)
}

// Our tools served over MCP with a token come back as agent tools that work,
// and read-only hints count only when the caller trusts the server.
func TestServeAndConsumeOverHTTP(t *testing.T) {
	ctx := context.Background()
	add, err := tool.Func("add", "Adds two numbers.", func(_ context.Context, in struct{ A, B int }) (int, error) { return in.A + in.B, nil })
	if err != nil {
		t.Fatal(err)
	}
	fail := &tool.Function{Name: "fail", Run: func(context.Context, json.RawMessage) (string, error) { return "", errors.New("boom") }}
	post := &tool.Function{Name: "post", Writes: true, Run: func(context.Context, json.RawMessage) (string, error) { return "posted", nil }}
	srv, err := xmcp.Server(&mcp.Implementation{Name: "ours", Version: "1"}, []tool.Tool{add, fail, post})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(xmcp.Handler(srv, "s3cret"))
	defer ts.Close()

	if _, err := connect(ctx, ts.URL, "wrong"); err == nil {
		t.Fatal("connected with the wrong token")
	}
	session, err := connect(ctx, ts.URL, "s3cret")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = session.Close() }()

	untrusted, err := xmcp.Tools(ctx, session, xmcp.Options{Prefix: "ours_"})
	if err != nil {
		t.Fatal(err)
	}
	fns := tool.Functions(untrusted)
	if !fns["ours_add"].Writes {
		t.Error("an untrusted server's read-only hint was believed")
	}
	if got, err := fns["ours_add"].Run(ctx, json.RawMessage(`{"A":2,"B":3}`)); err != nil || got != "5" {
		t.Errorf("add = %q, %v", got, err)
	}
	if _, err := fns["ours_fail"].Run(ctx, json.RawMessage(`{}`)); err == nil || err.Error() != "boom" {
		t.Errorf("tool error = %v, want boom", err)
	}

	trusted, err := xmcp.Tools(ctx, session, xmcp.Options{Trusted: true, Allow: []string{"add", "post"}})
	if err != nil {
		t.Fatal(err)
	}
	fns = tool.Functions(trusted)
	if len(fns) != 2 || fns["add"].Writes || !fns["post"].Writes {
		t.Errorf("trusted tools = add writes %v, post writes %v, count %d", fns["add"].Writes, fns["post"].Writes, len(fns))
	}
}
