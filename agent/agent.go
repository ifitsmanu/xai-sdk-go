// Package agent runs multi-turn tool loops on top of the xAI chat API.
//
// An agent has an instruction, a model, and tools. A runner samples the
// model, executes client-side tool calls, and records the conversation on
// a session. Agents can hand work to other agents, or run in sequence or
// in parallel.
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ifitsmanu/xai-sdk-go/chat"
	"github.com/ifitsmanu/xai-sdk-go/xaiapi"
)

// HandoffTool is the function name used to move the conversation to another agent.
const HandoffTool = "handoff"

// DefaultMaxTurns is the client-side tool loop limit when Agent.MaxTurns is unset.
const DefaultMaxTurns = 8

// Agent is one model, one instruction, and the tools it may call.
// Instruction and SubAgents are read on every model call.
// A tool schema is encoded once, after the first successful Spec call.
type Agent struct {
	Name        string
	Description string
	Model       string
	Instruction string
	Tools       []Tool
	SubAgents   []*Agent
	// MaxTurns limits client-side model calls. Zero uses DefaultMaxTurns.
	MaxTurns int
	// ServerMaxTurns is sent as the API max_turns for server-side tools.
	ServerMaxTurns *int32
	Hooks          Hooks
}

// Hooks observe or reject a model call or a tool call.
// A non-nil error from BeforeModel or BeforeTool stops the run.
type Hooks struct {
	BeforeModel func(ctx context.Context, agentName string, req *xaiapi.GetCompletionsRequest) error
	AfterModel  func(ctx context.Context, agentName string, resp *chat.Response) error
	BeforeTool  func(ctx context.Context, call Call) error
	AfterTool   func(ctx context.Context, call Call, result string, err error)
}

// Call is one client-side function invocation.
type Call struct {
	ID   string
	Name string
	Args json.RawMessage
}

func (a *Agent) systemPrompt() string {
	if a == nil || len(a.SubAgents) == 0 {
		if a == nil {
			return ""
		}
		return a.Instruction
	}
	var b strings.Builder
	b.WriteString(a.Instruction)
	b.WriteString("\n\nYou can hand off by calling ")
	b.WriteString(HandoffTool)
	b.WriteString(" with the agent name.\nAgents:\n")
	for _, sub := range a.SubAgents {
		if sub == nil {
			continue
		}
		fmt.Fprintf(&b, "- %s: %s\n", sub.Name, sub.Description)
	}
	return b.String()
}

func (a *Agent) maxTurns() int {
	if a == nil || a.MaxTurns <= 0 {
		return DefaultMaxTurns
	}
	return a.MaxTurns
}

func (a *Agent) findSub(name string) *Agent {
	for _, sub := range a.SubAgents {
		if sub != nil && sub.Name == name {
			return sub
		}
	}
	return nil
}

func (a *Agent) toolSpecs() ([]*xaiapi.Tool, map[string]Tool, error) {
	specs := make([]*xaiapi.Tool, 0, len(a.Tools)+1)
	byName := make(map[string]Tool, len(a.Tools)+1)
	for _, tool := range a.Tools {
		if tool == nil {
			continue
		}
		spec, err := tool.Spec()
		if err != nil {
			return nil, nil, err
		}
		fn := spec.GetFunction()
		if fn == nil || fn.Name == "" {
			return nil, nil, fmt.Errorf("agent: tool spec is missing a function name")
		}
		specs = append(specs, spec)
		byName[fn.Name] = tool
	}
	if len(a.SubAgents) > 0 {
		spec, err := handoffSpec(a.SubAgents)
		if err != nil {
			return nil, nil, err
		}
		specs = append(specs, spec)
		byName[HandoffTool] = handoffTool{parent: a}
	}
	return specs, byName, nil
}

func handoffSpec(subs []*Agent) (*xaiapi.Tool, error) {
	names := make([]string, 0, len(subs))
	for _, sub := range subs {
		if sub != nil && sub.Name != "" {
			names = append(names, sub.Name)
		}
	}
	return chat.Function(HandoffTool, "Hand the conversation to another agent.", map[string]any{
		"type": "object",
		"properties": map[string]any{
			"agent": map[string]any{
				"type": "string",
				"enum": names,
			},
		},
		"required": []string{"agent"},
	})
}

type handoffTool struct{ parent *Agent }

func (h handoffTool) Spec() (*xaiapi.Tool, error) { return handoffSpec(h.parent.SubAgents) }

func (h handoffTool) Run(context.Context, json.RawMessage) (string, error) {
	return "", nil
}
