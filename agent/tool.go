package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/ifitsmanu/xai-sdk-go/chat"
	"github.com/ifitsmanu/xai-sdk-go/xaiapi"
)

// Tool is a client-side function the model may call.
type Tool interface {
	Spec() (*xaiapi.Tool, error)
	Run(ctx context.Context, args json.RawMessage) (string, error)
}

type funcTool struct {
	name        string
	description string
	parameters  any
	fn          func(ctx context.Context, args json.RawMessage) (string, error)

	mu   sync.Mutex
	spec *xaiapi.Tool
}

// Func builds a tool from a JSON Schema and a callback.
// parameters follows the same rules as chat.Function.
// The schema is encoded once, after the first successful call.
func Func(name, description string, parameters any, fn func(ctx context.Context, args json.RawMessage) (string, error)) Tool {
	return &funcTool{name: name, description: description, parameters: parameters, fn: fn}
}

func (f *funcTool) Spec() (*xaiapi.Tool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.spec != nil {
		return f.spec, nil
	}
	spec, err := chat.Function(f.name, f.description, f.parameters)
	if err != nil {
		return nil, err
	}
	f.spec = spec
	return spec, nil
}

func (f *funcTool) Run(ctx context.Context, args json.RawMessage) (string, error) {
	if f.fn == nil {
		return "", fmt.Errorf("agent: tool %s has no function", f.name)
	}
	return f.fn(ctx, args)
}

type doneTool struct {
	stop *bool

	mu   sync.Mutex
	spec *xaiapi.Tool
}

// Done builds a tool the model can call to end a loop early.
// The stop flag is set when the tool runs. Pass the same flag to RunLoop,
// or use the tool inside an agent and check the result text "done".
func Done(stop *bool) Tool {
	return &doneTool{stop: stop}
}

func (d *doneTool) Spec() (*xaiapi.Tool, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.spec != nil {
		return d.spec, nil
	}
	spec, err := chat.Function("done", "Call this when the task is finished and no further tool calls are needed.", map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	})
	if err != nil {
		return nil, err
	}
	d.spec = spec
	return spec, nil
}

func (d *doneTool) Run(context.Context, json.RawMessage) (string, error) {
	if d.stop != nil {
		*d.stop = true
	}
	return "done", nil
}
