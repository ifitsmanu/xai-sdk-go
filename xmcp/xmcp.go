// Package xmcp connects agents and MCP. Tools turns the tools of any MCP server
// into agent tools. Server and Handler publish agent tools as an MCP server that
// Grok's remote MCP tool, or any MCP client, can call.
//
// Connect to servers with the official SDK (github.com/modelcontextprotocol/go-sdk/mcp);
// this package only converts between its types and package tool.
package xmcp

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/ifitsmanu/xai-sdk-go/tool"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Options controls how a server's tools become agent tools.
type Options struct {
	// Prefix is prepended to each tool name, so tools from two servers cannot collide.
	Prefix string
	// Allow keeps only these tool names (before Prefix). Empty keeps all.
	Allow []string
	// Trusted uses the server's readOnlyHint. Otherwise every tool is marked
	// Writes and needs confirmation, because the MCP spec says annotations
	// from an untrusted server must not drive decisions.
	Trusted bool
	// ToolTimeout bounds each call. Zero leaves only the caller's context.
	ToolTimeout time.Duration
}

// Tools lists the tools of a connected MCP session as agent tools.
func Tools(ctx context.Context, session *mcp.ClientSession, opts Options) ([]tool.Tool, error) {
	var out []tool.Tool
	for t, err := range session.Tools(ctx, nil) {
		if err != nil {
			return nil, err
		}
		if len(opts.Allow) > 0 && !slices.Contains(opts.Allow, t.Name) {
			continue
		}
		readOnly := opts.Trusted && t.Annotations != nil && t.Annotations.ReadOnlyHint
		name := t.Name
		out = append(out, &tool.Function{
			Name:        opts.Prefix + name,
			Description: t.Description,
			Parameters:  t.InputSchema,
			Writes:      !readOnly,
			Run: func(ctx context.Context, args json.RawMessage) (string, error) {
				if opts.ToolTimeout > 0 {
					var cancel context.CancelFunc
					ctx, cancel = context.WithTimeout(ctx, opts.ToolTimeout)
					defer cancel()
				}
				res, err := session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
				if err != nil {
					return "", err
				}
				text := join(res)
				if res.IsError {
					return "", errors.New(text)
				}
				return text, nil
			},
		})
	}
	return out, nil
}

// Server publishes function tools as an MCP server. Tools marked Writes are
// annotated as destructive so clients can ask before calling them.
func Server(impl *mcp.Implementation, tools []tool.Tool) (*mcp.Server, error) {
	srv := mcp.NewServer(impl, nil)
	for _, t := range tools {
		fn, ok := t.(*tool.Function)
		if !ok {
			return nil, fmt.Errorf("xmcp: %T runs on xAI and cannot be served", t)
		}
		schema := fn.Parameters
		if schema == nil {
			schema = map[string]any{"type": "object"}
		}
		destructive := fn.Writes
		srv.AddTool(&mcp.Tool{
			Name:        fn.Name,
			Description: fn.Description,
			InputSchema: schema,
			Annotations: &mcp.ToolAnnotations{ReadOnlyHint: !fn.Writes, DestructiveHint: &destructive},
		}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			text, err := fn.Run(ctx, req.Params.Arguments)
			if err != nil {
				return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}}}, nil
			}
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}, nil
		})
	}
	return srv, nil
}

// Handler serves srv over streamable HTTP. When token is set, each request must
// carry it in the Authorization header, with or without a "Bearer " prefix.
func Handler(srv *mcp.Server, token string) http.Handler {
	h := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return srv }, nil)
	if token == "" {
		return h
	}
	want := []byte(token)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if subtle.ConstantTimeCompare([]byte(got), want) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		h.ServeHTTP(w, r)
	})
}

func join(res *mcp.CallToolResult) string {
	var parts []string
	for _, c := range res.Content {
		if t, ok := c.(*mcp.TextContent); ok {
			parts = append(parts, t.Text)
		}
	}
	if len(parts) == 0 && res.StructuredContent != nil {
		b, _ := json.Marshal(res.StructuredContent)
		return string(b)
	}
	return strings.Join(parts, "\n")
}
