package xmcp

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"sync"

	"github.com/ifitsmanu/xai-sdk-go/tool"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Connect opens a session to one server, within its startup timeout.
func Connect(ctx context.Context, c ServerConfig) (*mcp.ClientSession, error) {
	var transport mcp.Transport
	switch {
	case c.URL != "":
		transport = &mcp.StreamableClientTransport{Endpoint: c.URL, HTTPClient: &http.Client{Transport: headers(c.Headers)}}
	case c.Command != "":
		cmd := exec.Command(c.Command, c.Args...)
		cmd.Env = os.Environ()
		for k, v := range c.Env {
			cmd.Env = append(cmd.Env, k+"="+v)
		}
		transport = &mcp.CommandTransport{Command: cmd}
	default:
		return nil, fmt.Errorf("xmcp: server %s has neither a url nor a command", c.Name)
	}
	ctx, cancel := context.WithTimeout(ctx, cmp.Or(c.StartupTimeout, DefaultStartupTimeout))
	defer cancel()
	client := mcp.NewClient(&mcp.Implementation{Name: "xai-sdk-go", Version: "0"}, nil)
	return client.Connect(ctx, transport, nil)
}

// Toolset is a set of connected servers and their tools.
type Toolset struct {
	Tools []tool.Tool
	// Errors holds the servers that failed; the others still work.
	Errors   map[string]error
	sessions []*mcp.ClientSession
}

// Open connects to every server at once and names each tool server__tool, as
// Grok Build does. opts.Prefix is ignored; the other options apply to every server.
func Open(ctx context.Context, servers map[string]ServerConfig, opts Options) *Toolset {
	ts := &Toolset{Errors: map[string]error{}}
	var mu sync.Mutex
	var wg sync.WaitGroup
	for name, c := range servers {
		wg.Go(func() {
			session, err := Connect(ctx, c)
			var tools []tool.Tool
			if err == nil {
				o := opts
				o.Prefix, o.ToolTimeout = name+"__", cmp.Or(c.ToolTimeout, DefaultToolTimeout)
				if tools, err = Tools(ctx, session, o); err != nil {
					_ = session.Close()
				}
			}
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				ts.Errors[name] = err
				return
			}
			ts.sessions = append(ts.sessions, session)
			ts.Tools = append(ts.Tools, tools...)
		})
	}
	wg.Wait()
	sort.Slice(ts.Tools, func(i, j int) bool { return ts.Tools[i].(*tool.Function).Name < ts.Tools[j].(*tool.Function).Name })
	return ts
}

// Close ends every session and stops stdio servers.
func (t *Toolset) Close() error {
	var errs []error
	for _, s := range t.sessions {
		errs = append(errs, s.Close())
	}
	return errors.Join(errs...)
}

type headers map[string]string

func (h headers) RoundTrip(r *http.Request) (*http.Response, error) {
	if len(h) > 0 {
		r = r.Clone(r.Context())
		for k, v := range h {
			r.Header.Set(k, v)
		}
	}
	return http.DefaultTransport.RoundTrip(r)
}
