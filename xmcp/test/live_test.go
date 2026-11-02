//go:build live

package xmcp_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ifitsmanu/xai-sdk-go"
	"github.com/ifitsmanu/xai-sdk-go/agent"
	"github.com/ifitsmanu/xai-sdk-go/models"
	"github.com/ifitsmanu/xai-sdk-go/xmcp"
)

// A Grok agent answers from xAI's documentation through the hosted Docs MCP server.
func TestAgentUsesXAIDocsMCP(t *testing.T) {
	if os.Getenv("XAI_API_KEY") == "" {
		t.Skip("XAI_API_KEY is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	docs := xmcp.Open(ctx, map[string]xmcp.ServerConfig{"xai-docs": xmcp.XAIDocs}, xmcp.Options{})
	defer func() { _ = docs.Close() }()
	if err := docs.Errors["xai-docs"]; err != nil {
		t.Fatal(err)
	}
	t.Logf("%d docs tools", len(docs.Tools))

	client, err := xai.NewClient()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = client.Close() }()
	model, err := client.Models.Select(ctx, models.Need{})
	if err != nil {
		t.Fatal(err)
	}
	// MCP tools are treated as writes until approved; approve this read-only docs server.
	runner := client.Runner(
		agent.WithLimits(agent.Limits{CostUSD: 0.10, Turns: 8}),
		agent.WithConfirm(func(_ context.Context, c agent.Call) (bool, error) {
			return strings.HasPrefix(c.Name, "xai-docs__"), nil
		}),
	)
	ag := &agent.Agent{Name: "docs", Model: model, Tools: docs.Tools,
		Instruction: "Answer from the xAI documentation tools only. Be brief."}
	var answer, used string
	for e, err := range runner.Run(ctx, agent.NewSession(), ag, "What is the WebSocket endpoint for realtime voice? Give the URL.") {
		if err != nil {
			t.Fatal(err)
		}
		switch e.Kind {
		case agent.KindToolCall:
			used += e.Tool + " "
		case agent.KindMessage:
			answer = e.Text
		}
	}
	t.Logf("tools: %s\nanswer: %s", used, answer)
	if used == "" || !strings.Contains(answer, "wss://api.x.ai/v1/realtime") {
		t.Fatal("the agent did not answer from the docs server")
	}
}
