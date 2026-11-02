// Package agent runs Grok agents on the Responses API: streaming turns,
// local and server tools, handoffs, agents as tools, structured output, and
// run-wide limits on turns, tokens, cost, and time.
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/ifitsmanu/xai-sdk-go/responses"
	"github.com/ifitsmanu/xai-sdk-go/tool"
)

// HandoffTool is the function name used to move the conversation to another agent.
const HandoffTool = "handoff"

// Agent is a model, an instruction, and the tools it may use.
type Agent struct {
	Name        string
	Description string
	Model       string
	// Instruction may contain {key} placeholders filled from session state.
	Instruction string
	// Tools mixes functions this process runs with server tools xAI runs.
	Tools     []tool.Tool
	SubAgents []*Agent
	// Output is a JSON Schema the final answer must match.
	Output any
	// OutputKey stores the final answer text in session state under this key.
	OutputKey string
	// Reasoning is the effort for reasoning models; see models.Capabilities.
	Reasoning string
	// Configure adjusts each request, for settings without a field here.
	Configure func(*responses.Request)
	Hooks     Hooks
}

// Hooks observe or stop a run. A non-nil error from a Before hook stops the run.
type Hooks struct {
	BeforeModel func(ctx context.Context, agent string, req *responses.Request) error
	AfterModel  func(ctx context.Context, agent string, resp *responses.Response) error
	BeforeTool  func(ctx context.Context, call Call) error
	AfterTool   func(ctx context.Context, call Call, result string, err error)
}

// Call is one function call from the model.
type Call struct {
	ID   string
	Name string
	Args json.RawMessage
}

func (a *Agent) request(s *Session) (responses.Request, error) {
	tools := a.Tools
	if len(a.SubAgents) > 0 {
		tools = append(slices.Clip(tools), handoffTool(a.SubAgents))
	}
	specs, err := tool.ForResponses(tools)
	if err != nil {
		return responses.Request{}, fmt.Errorf("agent %s: %w", a.Name, err)
	}
	req := responses.Request{
		Model:          a.Model,
		Instructions:   a.instruction(s),
		Input:          responses.Items(slices.Clone(s.Items)),
		Tools:          specs,
		PromptCacheKey: s.ID,
	}
	if a.Output != nil {
		req.Text = responses.JSONSchema(a.Output)
	}
	if a.Reasoning != "" {
		req.Reasoning = &responses.Reasoning{Effort: a.Reasoning}
	}
	if a.Configure != nil {
		a.Configure(&req)
	}
	return req, nil
}

var placeholder = regexp.MustCompile(`\{([A-Za-z_][A-Za-z0-9_.]*)\}`)

func (a *Agent) instruction(s *Session) string {
	text := placeholder.ReplaceAllStringFunc(a.Instruction, func(m string) string {
		if v, ok := s.Get(m[1 : len(m)-1]); ok {
			return fmt.Sprint(v)
		}
		return m
	})
	if len(a.SubAgents) == 0 {
		return text
	}
	var b strings.Builder
	b.WriteString(text)
	b.WriteString("\n\nCall " + HandoffTool + " to pass the conversation to one of these agents:\n")
	for _, sub := range a.SubAgents {
		fmt.Fprintf(&b, "- %s: %s\n", sub.Name, sub.Description)
	}
	return b.String()
}

func (a *Agent) findSub(name string) *Agent {
	for _, sub := range a.SubAgents {
		if sub != nil && sub.Name == name {
			return sub
		}
	}
	return nil
}

func handoffTool(subs []*Agent) *tool.Function {
	names := make([]string, 0, len(subs))
	for _, sub := range subs {
		if sub != nil {
			names = append(names, sub.Name)
		}
	}
	return &tool.Function{
		Name:        HandoffTool,
		Description: "Pass the conversation to another agent.",
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{"agent": map[string]any{"type": "string", "enum": names}},
			"required":   []string{"agent"},
		},
	}
}
