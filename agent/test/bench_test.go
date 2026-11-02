package agent_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/ifitsmanu/xai-sdk-go/agent"
	"github.com/ifitsmanu/xai-sdk-go/tool"
	"github.com/ifitsmanu/xai-sdk-go/xaiapi"
)

type ready struct{}

func (ready) Complete(_ context.Context, req *xaiapi.GetCompletionsRequest) (*xaiapi.GetChatCompletionResponse, error) {
	for _, msg := range req.Messages {
		if msg.GetRole() == xaiapi.MessageRole_ROLE_TOOL {
			return assistant("done"), nil
		}
	}
	return assistant("", call("c1", "echo", `{}`)), nil
}

func BenchmarkRun(b *testing.B) {
	ag := &agent.Agent{
		Name:        "bench",
		Model:       "grok-test",
		Instruction: "Be fast.",
		Tools: []tool.Tool{
			fn("echo", "Echo", map[string]any{"type": "object"}, func(context.Context, json.RawMessage) (string, error) {
				return "ok", nil
			}),
		},
	}
	runner := agent.NewRunner(ready{})
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, err := range runner.Run(ctx, agent.NewSession(), ag, "hi") {
			if err != nil {
				b.Fatal(err)
			}
		}
	}
}
