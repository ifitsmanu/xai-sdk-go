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

// MustFunction is Function that panics on a schema error.
func MustFunction(name, description string, parameters any) *xaiapi.Tool {
	tool, err := Function(name, description, parameters)
	if err != nil {
		panic(err)
	}
	return tool
}

// ToolChoiceMode forces auto, none, or required.
func ToolChoiceMode(mode xaiapi.ToolMode) *xaiapi.ToolChoice {
	return &xaiapi.ToolChoice{ToolChoice: &xaiapi.ToolChoice_Mode{Mode: mode}}
}

// RequiredFunction forces a call to the named function.
func RequiredFunction(name string) *xaiapi.ToolChoice {
	return &xaiapi.ToolChoice{ToolChoice: &xaiapi.ToolChoice_FunctionName{FunctionName: name}}
}

// WebSearch is the server-side web search tool.
func WebSearch(excludedDomains, allowedDomains []string) *xaiapi.Tool {
	return &xaiapi.Tool{Tool: &xaiapi.Tool_WebSearch{WebSearch: &xaiapi.WebSearch{
		ExcludedDomains: excludedDomains,
		AllowedDomains:  allowedDomains,
	}}}
}

// XSearch is the server-side X search tool.
func XSearch(allowedHandles, excludedHandles []string) *xaiapi.Tool {
	return &xaiapi.Tool{Tool: &xaiapi.Tool_XSearch{XSearch: &xaiapi.XSearch{
		AllowedXHandles:  allowedHandles,
		ExcludedXHandles: excludedHandles,
	}}}
}

// CodeExecution is the server-side code execution tool.
func CodeExecution() *xaiapi.Tool {
	return &xaiapi.Tool{Tool: &xaiapi.Tool_CodeExecution{CodeExecution: &xaiapi.CodeExecution{}}}
}

// ImageGeneration is the server-side image generation tool.
// action is "auto", "generate", or "edit". An empty action leaves the server default.
func ImageGeneration(action string) *xaiapi.Tool {
	gen := &xaiapi.ImageGeneration{}
	if action != "" {
		gen.Action = &action
	}
	return &xaiapi.Tool{Tool: &xaiapi.Tool_ImageGeneration{ImageGeneration: gen}}
}

// CollectionsSearch searches the given collections on the server.
func CollectionsSearch(collectionIDs []string, limit int32) *xaiapi.Tool {
	search := &xaiapi.CollectionsSearch{CollectionIds: collectionIDs}
	if limit > 0 {
		search.Limit = &limit
	}
	return &xaiapi.Tool{Tool: &xaiapi.Tool_CollectionsSearch{CollectionsSearch: search}}
}

// AttachmentSearch is the server-side attachment search tool.
func AttachmentSearch(limit int32) *xaiapi.Tool {
	search := &xaiapi.AttachmentSearch{}
	if limit > 0 {
		search.Limit = &limit
	}
	return &xaiapi.Tool{Tool: &xaiapi.Tool_AttachmentSearch{AttachmentSearch: search}}
}

// MCP connects a remote MCP server. authorization is optional.
func MCP(label, description, serverURL, authorization string, allowedTools []string) *xaiapi.Tool {
	mcp := &xaiapi.MCP{
		ServerLabel:       label,
		ServerDescription: description,
		ServerUrl:         serverURL,
		AllowedToolNames:  allowedTools,
	}
	if authorization != "" {
		mcp.Authorization = &authorization
	}
	return &xaiapi.Tool{Tool: &xaiapi.Tool_Mcp{Mcp: mcp}}
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
