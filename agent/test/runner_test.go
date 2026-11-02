package agent_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"github.com/ifitsmanu/xai-sdk-go/agent"
	"github.com/ifitsmanu/xai-sdk-go/chat"
	"github.com/ifitsmanu/xai-sdk-go/xaiapi"
)

type scripted struct {
	mu   sync.Mutex
	reqs []*xaiapi.GetCompletionsRequest
	fn   func(req *xaiapi.GetCompletionsRequest) (*xaiapi.GetChatCompletionResponse, error)
}

func (s *scripted) Complete(_ context.Context, req *xaiapi.GetCompletionsRequest) (*xaiapi.GetChatCompletionResponse, error) {
	s.mu.Lock()
	s.reqs = append(s.reqs, req)
	fn := s.fn
	s.mu.Unlock()
	return fn(req)
}

func (s *scripted) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.reqs)
}

func assistant(content string, calls ...*xaiapi.ToolCall) *xaiapi.GetChatCompletionResponse {
	return &xaiapi.GetChatCompletionResponse{
		Id:    "resp",
		Model: "grok-test",
		Outputs: []*xaiapi.CompletionOutput{{
			Message: &xaiapi.CompletionMessage{
				Role:      xaiapi.MessageRole_ROLE_ASSISTANT,
				Content:   content,
				ToolCalls: calls,
			},
		}},
	}
}

func call(id, name, args string) *xaiapi.ToolCall {
	return &xaiapi.ToolCall{
		Id:   id,
		Type: xaiapi.ToolCallType_TOOL_CALL_TYPE_CLIENT_SIDE_TOOL,
		Tool: &xaiapi.ToolCall_Function{Function: &xaiapi.FunctionCall{Name: name, Arguments: args}},
	}
}

func TestRunnerExecutesToolThenAnswers(t *testing.T) {
	var seenArgs string
	ag := &agent.Agent{
		Name:        "weather",
		Model:       "grok-test",
		Instruction: "Use tools.",
		Tools: []agent.Tool{
			agent.Func("get_weather", "Get weather", map[string]any{"type": "object"}, func(_ context.Context, args json.RawMessage) (string, error) {
				seenArgs = string(args)
				return "sunny", nil
			}),
		},
	}
	step := 0
	comp := &scripted{fn: func(req *xaiapi.GetCompletionsRequest) (*xaiapi.GetChatCompletionResponse, error) {
		step++
		if step == 1 {
			return assistant("", call("c1", "get_weather", `{"city":"Austin"}`)), nil
		}
		var sawTool bool
		for _, msg := range req.Messages {
			if msg.GetRole() == xaiapi.MessageRole_ROLE_TOOL && msg.GetToolCallId() == "c1" {
				sawTool = true
				if msg.Content[0].GetText() != "sunny" {
					t.Errorf("tool text = %q", msg.Content[0].GetText())
				}
			}
		}
		if !sawTool {
			t.Error("second request missing tool result")
		}
		return assistant("It is sunny."), nil
	}}
	runner := agent.NewRunner(comp)
	session := agent.NewSession()
	var kinds []agent.Kind
	var text string
	for ev, err := range runner.Run(context.Background(), session, ag, "weather in Austin?") {
		if err != nil {
			t.Fatal(err)
		}
		kinds = append(kinds, ev.Kind)
		if ev.Kind == agent.KindText {
			text = ev.Text
		}
	}
	if text != "It is sunny." {
		t.Fatalf("text = %q", text)
	}
	if seenArgs != `{"city":"Austin"}` {
		t.Fatalf("args = %s", seenArgs)
	}
	if comp.count() != 2 {
		t.Fatalf("calls = %d", comp.count())
	}
	if len(kinds) == 0 || kinds[len(kinds)-1] != agent.KindText {
		t.Fatalf("kinds = %v", kinds)
	}
}

func TestServerToolsAreSent(t *testing.T) {
	web := chat.WebSearch(chat.WebSearchConfig{AllowedDomains: []string{"x.ai"}})
	ag := &agent.Agent{
		Name:        "search",
		Model:       "grok-test",
		Instruction: "Search when needed.",
		ServerTools: []*xaiapi.Tool{web},
	}
	comp := &scripted{fn: func(req *xaiapi.GetCompletionsRequest) (*xaiapi.GetChatCompletionResponse, error) {
		if len(req.Tools) != 1 || req.Tools[0].GetWebSearch().GetAllowedDomains()[0] != "x.ai" {
			t.Fatalf("tools = %+v", req.Tools)
		}
		return assistant("done"), nil
	}}
	for _, err := range agent.NewRunner(comp).Run(context.Background(), agent.NewSession(), ag, "latest?") {
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestHandoffSwitchesAgent(t *testing.T) {
	writer := &agent.Agent{Name: "writer", Description: "Writes", Model: "grok-writer", Instruction: "Write plainly."}
	router := &agent.Agent{
		Name:        "router",
		Model:       "grok-router",
		Instruction: "Route the request.",
		SubAgents:   []*agent.Agent{writer},
	}
	step := 0
	comp := &scripted{fn: func(req *xaiapi.GetCompletionsRequest) (*xaiapi.GetChatCompletionResponse, error) {
		step++
		if step == 1 {
			if req.Model != "grok-router" {
				t.Errorf("model = %s", req.Model)
			}
			return assistant("", call("h1", agent.HandoffTool, `{"agent":"writer"}`)), nil
		}
		if req.Model != "grok-writer" {
			t.Errorf("model after handoff = %s", req.Model)
		}
		if len(req.Messages) == 0 || req.Messages[0].Content[0].GetText() == "" {
			t.Fatal("missing writer instruction")
		}
		if req.Messages[0].Content[0].GetText() != "Write plainly." {
			t.Errorf("instruction = %q", req.Messages[0].Content[0].GetText())
		}
		return assistant("A short note."), nil
	}}
	var handed string
	for ev, err := range agent.NewRunner(comp).Run(context.Background(), agent.NewSession(), router, "write a note") {
		if err != nil {
			t.Fatal(err)
		}
		if ev.Kind == agent.KindHandoff {
			handed = ev.Text
		}
	}
	if handed != "writer" {
		t.Fatalf("handoff = %q", handed)
	}
}

func TestSequentialPassesText(t *testing.T) {
	draft := &agent.Agent{Name: "draft", Model: "m1", Instruction: "Draft."}
	edit := &agent.Agent{Name: "edit", Model: "m2", Instruction: "Edit."}
	step := 0
	comp := &scripted{fn: func(req *xaiapi.GetCompletionsRequest) (*xaiapi.GetChatCompletionResponse, error) {
		step++
		if step == 1 {
			return assistant("raw draft"), nil
		}
		var sawDraft bool
		var duplicated bool
		for _, msg := range req.Messages {
			if len(msg.Content) == 0 {
				continue
			}
			text := msg.Content[0].GetText()
			if msg.Role == xaiapi.MessageRole_ROLE_ASSISTANT && text == "raw draft" {
				sawDraft = true
			}
			if msg.Role == xaiapi.MessageRole_ROLE_USER && text == "raw draft" {
				duplicated = true
			}
		}
		if !sawDraft || duplicated {
			t.Errorf("editor saw draft=%v duplicated=%v", sawDraft, duplicated)
		}
		return assistant("polished"), nil
	}}
	var text string
	session := agent.NewSession()
	for ev, err := range agent.NewRunner(comp).RunSequential(context.Background(), session, []*agent.Agent{draft, edit}, "start") {
		if err != nil {
			t.Fatal(err)
		}
		if ev.Kind == agent.KindText {
			text = ev.Text
		}
	}
	if text != "polished" {
		t.Fatalf("text = %q", text)
	}
}

func TestBeforeToolStopsRun(t *testing.T) {
	ag := &agent.Agent{
		Name:  "guarded",
		Model: "grok-test",
		Tools: []agent.Tool{agent.Func("danger", "nope", nil, func(context.Context, json.RawMessage) (string, error) {
			t.Fatal("tool ran")
			return "", nil
		})},
		Hooks: agent.Hooks{BeforeTool: func(context.Context, agent.Call) error {
			return errors.New("blocked")
		}},
	}
	comp := &scripted{fn: func(*xaiapi.GetCompletionsRequest) (*xaiapi.GetChatCompletionResponse, error) {
		return assistant("", call("c", "danger", `{}`)), nil
	}}
	sess := agent.NewSession()
	var got error
	for _, err := range agent.NewRunner(comp).Run(context.Background(), sess, ag, "go") {
		got = err
	}
	if got == nil || got.Error() != "blocked" {
		t.Fatalf("err = %v", got)
	}
	var sealed bool
	for _, msg := range sess.Messages {
		if msg.GetRole() == xaiapi.MessageRole_ROLE_TOOL && msg.GetToolCallId() == "c" {
			sealed = true
		}
	}
	if !sealed {
		t.Fatal("rejected tool left no tool result")
	}
}

func TestSessionState(t *testing.T) {
	s := agent.NewSession()
	s.Set("city", "Austin")
	s.Set("tags", map[string]any{"k": "v"})
	got, ok := s.Get("city")
	if !ok || got != "Austin" {
		t.Fatalf("state = %#v ok=%v", got, ok)
	}
	clone := s.Clone()
	tags, _ := clone.Get("tags")
	tags.(map[string]any)["k"] = "changed"
	orig, _ := s.Get("tags")
	if orig.(map[string]any)["k"] != "v" {
		t.Fatalf("clone wrote through: %#v", orig)
	}
}

func TestMemoryStore(t *testing.T) {
	store := agent.NewMemoryStore()
	loaded, err := store.Load(context.Background(), "abc")
	if err != nil {
		t.Fatal(err)
	}
	loaded.Messages = append(loaded.Messages, chat.UserText("hi"))
	if err := store.Save(context.Background(), loaded); err != nil {
		t.Fatal(err)
	}
	again, err := store.Load(context.Background(), "abc")
	if err != nil {
		t.Fatal(err)
	}
	if len(again.Messages) != 1 {
		t.Fatalf("messages = %d", len(again.Messages))
	}
	loaded.Messages = append(loaded.Messages, chat.UserText("later"))
	stored, err := store.Load(context.Background(), "abc")
	if err != nil {
		t.Fatal(err)
	}
	if len(stored.Messages) != 1 {
		t.Fatalf("store aliased the caller: %d", len(stored.Messages))
	}
}
