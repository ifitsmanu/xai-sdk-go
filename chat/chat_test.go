package chat

import (
	"context"
	"testing"

	"github.com/ifitsmanu/xai-sdk-go/xaiapi"
)

func TestMessagesFromResponseCopiesToolCallID(t *testing.T) {
	resp := &Response{Proto: &xaiapi.GetChatCompletionResponse{
		Outputs: []*xaiapi.CompletionOutput{{
			Message: &xaiapi.CompletionMessage{
				Role: xaiapi.MessageRole_ROLE_TOOL,
				ToolCalls: []*xaiapi.ToolCall{{
					Id:   "call_1",
					Type: xaiapi.ToolCallType_TOOL_CALL_TYPE_CLIENT_SIDE_TOOL,
					Tool: &xaiapi.ToolCall_Function{Function: &xaiapi.FunctionCall{Name: "lookup", Arguments: `{}`}},
				}},
			},
		}},
	}}
	msgs := MessagesFromResponse(resp)
	if len(msgs) != 1 {
		t.Fatalf("messages = %d", len(msgs))
	}
	if msgs[0].GetToolCallId() != "call_1" {
		t.Fatalf("tool call id = %q", msgs[0].GetToolCallId())
	}
}

func TestStreamAccumulator(t *testing.T) {
	resp := &Response{Proto: &xaiapi.GetChatCompletionResponse{}}
	resp.applyChunk(&xaiapi.GetChatCompletionChunk{
		Id:    "resp_1",
		Model: "grok-test",
		Outputs: []*xaiapi.CompletionOutputChunk{{
			Index: 0,
			Delta: &xaiapi.Delta{Role: xaiapi.MessageRole_ROLE_ASSISTANT, Content: "Hello"},
		}},
	})
	resp.applyChunk(&xaiapi.GetChatCompletionChunk{
		Citations: []string{"https://x.ai"},
		Outputs: []*xaiapi.CompletionOutputChunk{{
			Index:        0,
			FinishReason: xaiapi.FinishReason_REASON_STOP,
			Delta:        &xaiapi.Delta{Content: " world"},
		}},
	})
	if resp.Content() != "Hello world" {
		t.Fatalf("content = %q", resp.Content())
	}
	if resp.ID() != "resp_1" || resp.Model() != "grok-test" {
		t.Fatalf("id=%s model=%s", resp.ID(), resp.Model())
	}
	if len(resp.Citations()) != 1 || resp.Citations()[0] != "https://x.ai" {
		t.Fatalf("citations = %#v", resp.Citations())
	}
	if resp.Proto.Outputs[0].FinishReason != xaiapi.FinishReason_REASON_STOP {
		t.Fatalf("finish = %s", resp.Proto.Outputs[0].FinishReason)
	}
	resp.flush()
	if resp.Proto.Outputs[0].Message.Content != "Hello world" {
		t.Fatalf("flushed content = %q", resp.Proto.Outputs[0].Message.Content)
	}
	if resp.Content() != "Hello world" {
		t.Fatalf("content after flush = %q", resp.Content())
	}
}

func TestFunctionSchema(t *testing.T) {
	tool, err := Function("get_weather", "Weather", map[string]any{
		"type": "object",
		"properties": map[string]any{
			"city": map[string]any{"type": "string"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	fn := tool.GetFunction()
	if fn.GetName() != "get_weather" || fn.GetParameters() == "" {
		t.Fatalf("function = %+v", fn)
	}
}

func TestClientFunctionIgnoresServerTools(t *testing.T) {
	_, ok := ClientFunction(&xaiapi.ToolCall{
		Id:   "call_web",
		Type: xaiapi.ToolCallType_TOOL_CALL_TYPE_WEB_SEARCH_TOOL,
		Tool: &xaiapi.ToolCall_Function{Function: &xaiapi.FunctionCall{Name: "web_search", Arguments: `{}`}},
	})
	if ok {
		t.Fatal("server-side tool was treated as a client function")
	}
	fn, ok := ClientFunction(&xaiapi.ToolCall{
		Id:   "call_fn",
		Type: xaiapi.ToolCallType_TOOL_CALL_TYPE_CLIENT_SIDE_TOOL,
		Tool: &xaiapi.ToolCall_Function{Function: &xaiapi.FunctionCall{Name: "get_weather", Arguments: `{"city":"Austin"}`}},
	})
	if !ok || fn.GetName() != "get_weather" {
		t.Fatalf("client function = %+v ok=%v", fn, ok)
	}
}

func TestChatAppendAndEmptySample(t *testing.T) {
	svc := &Service{}
	conv := svc.Create(Config{Model: "grok-test"})
	if _, err := conv.Sample(context.Background()); err != ErrNoMessages {
		t.Fatalf("sample err = %v", err)
	}
	conv.Append(SystemText("Be brief.")).Append(UserText("Hi"))
	if len(conv.Request().Messages) != 2 {
		t.Fatalf("messages = %d", len(conv.Request().Messages))
	}
	conv.UseCompacted(&xaiapi.CompactContextResponse{EncryptedContent: "blob"})
	if len(conv.Request().Messages) != 1 || conv.Request().Messages[0].EncryptedContent != "blob" {
		t.Fatalf("compacted = %+v", conv.Request().Messages)
	}
}
