package chat

import (
	"encoding/json"
	"fmt"

	"github.com/ifitsmanu/xai-sdk-go/xaiapi"
)

// Function declares a client-side function. parameters is a JSON Schema
// object, a json.RawMessage, or any value that json.Marshal can encode as that schema.
func Function(name, description string, parameters any) (*xaiapi.Tool, error) {
	raw, err := schemaJSON(parameters)
	if err != nil {
		return nil, fmt.Errorf("xai: function %s parameters: %w", name, err)
	}
	return &xaiapi.Tool{Tool: &xaiapi.Tool_Function{Function: &xaiapi.Function{
		Name:        name,
		Description: description,
		Parameters:  string(raw),
	}}}, nil
}

// ToolChoiceMode forces auto, none, or required.
func ToolChoiceMode(mode xaiapi.ToolMode) *xaiapi.ToolChoice {
	return &xaiapi.ToolChoice{ToolChoice: &xaiapi.ToolChoice_Mode{Mode: mode}}
}

// RequiredFunction forces a call to the named function.
func RequiredFunction(name string) *xaiapi.ToolChoice {
	return &xaiapi.ToolChoice{ToolChoice: &xaiapi.ToolChoice_FunctionName{FunctionName: name}}
}

// WebSearch is the server-side web search tool. A nil config uses server defaults.
func WebSearch(cfg *xaiapi.WebSearch) *xaiapi.Tool {
	return &xaiapi.Tool{Tool: &xaiapi.Tool_WebSearch{WebSearch: orNew(cfg)}}
}

// XSearch is the server-side X search tool. A nil config uses server defaults.
func XSearch(cfg *xaiapi.XSearch) *xaiapi.Tool {
	return &xaiapi.Tool{Tool: &xaiapi.Tool_XSearch{XSearch: orNew(cfg)}}
}

// CodeExecution is the server-side code execution tool.
func CodeExecution() *xaiapi.Tool {
	return &xaiapi.Tool{Tool: &xaiapi.Tool_CodeExecution{CodeExecution: &xaiapi.CodeExecution{}}}
}

// ImageGeneration is the server-side image generation tool. A nil config uses server defaults.
func ImageGeneration(cfg *xaiapi.ImageGeneration) *xaiapi.Tool {
	return &xaiapi.Tool{Tool: &xaiapi.Tool_ImageGeneration{ImageGeneration: orNew(cfg)}}
}

// CollectionsSearch searches collections on the server.
func CollectionsSearch(cfg *xaiapi.CollectionsSearch) *xaiapi.Tool {
	return &xaiapi.Tool{Tool: &xaiapi.Tool_CollectionsSearch{CollectionsSearch: orNew(cfg)}}
}

// AttachmentSearch searches files attached to the conversation. A nil config uses server defaults.
func AttachmentSearch(cfg *xaiapi.AttachmentSearch) *xaiapi.Tool {
	return &xaiapi.Tool{Tool: &xaiapi.Tool_AttachmentSearch{AttachmentSearch: orNew(cfg)}}
}

// MCP connects a remote MCP server that xAI calls on your behalf.
func MCP(cfg *xaiapi.MCP) *xaiapi.Tool {
	return &xaiapi.Tool{Tool: &xaiapi.Tool_Mcp{Mcp: orNew(cfg)}}
}

func orNew[T any](v *T) *T {
	if v == nil {
		return new(T)
	}
	return v
}

func schemaJSON(parameters any) ([]byte, error) {
	switch v := parameters.(type) {
	case nil:
		return []byte(`{"type":"object","properties":{}}`), nil
	case string:
		return []byte(v), nil
	case []byte:
		return v, nil
	case json.RawMessage:
		return v, nil
	default:
		return json.Marshal(v)
	}
}
