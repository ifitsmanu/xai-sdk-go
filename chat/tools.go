package chat

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/ifitsmanu/xai-sdk-go/xaiapi"
	"google.golang.org/protobuf/types/known/timestamppb"
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

// WebSearchConfig is the server-side web search tool.
// AllowedDomains and ExcludedDomains cannot both be set. Five domains is the server maximum.
type WebSearchConfig struct {
	ExcludedDomains    []string
	AllowedDomains     []string
	ImageUnderstanding bool
	ImageSearch        bool
	Country            string
	City               string
	Region             string
	Timezone           string
}

// WebSearch is the server-side web search tool.
func WebSearch(cfg WebSearchConfig) *xaiapi.Tool {
	search := &xaiapi.WebSearch{
		ExcludedDomains:          cfg.ExcludedDomains,
		AllowedDomains:           cfg.AllowedDomains,
		EnableImageUnderstanding: flag(cfg.ImageUnderstanding),
		EnableImageSearch:        flag(cfg.ImageSearch),
	}
	if cfg.Country != "" || cfg.City != "" || cfg.Region != "" || cfg.Timezone != "" {
		search.UserLocation = &xaiapi.WebSearchUserLocation{
			Country:  strPtr(cfg.Country),
			City:     strPtr(cfg.City),
			Region:   strPtr(cfg.Region),
			Timezone: strPtr(cfg.Timezone),
		}
	}
	return &xaiapi.Tool{Tool: &xaiapi.Tool_WebSearch{WebSearch: search}}
}

// XSearchConfig is the server-side X search tool.
// AllowedHandles and ExcludedHandles cannot both be set. A zero time is unset.
type XSearchConfig struct {
	From               time.Time
	To                 time.Time
	AllowedHandles     []string
	ExcludedHandles    []string
	ImageUnderstanding bool
	VideoUnderstanding bool
}

// XSearch is the server-side X search tool.
func XSearch(cfg XSearchConfig) *xaiapi.Tool {
	search := &xaiapi.XSearch{
		AllowedXHandles:          cfg.AllowedHandles,
		ExcludedXHandles:         cfg.ExcludedHandles,
		EnableImageUnderstanding: flag(cfg.ImageUnderstanding),
		EnableVideoUnderstanding: flag(cfg.VideoUnderstanding),
	}
	if !cfg.From.IsZero() {
		search.FromDate = timestamppb.New(cfg.From)
	}
	if !cfg.To.IsZero() {
		search.ToDate = timestamppb.New(cfg.To)
	}
	return &xaiapi.Tool{Tool: &xaiapi.Tool_XSearch{XSearch: search}}
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

// CollectionsSearchConfig searches collections on the server.
// Retrieval is "hybrid", "semantic", or "keyword". Empty leaves the server default.
type CollectionsSearchConfig struct {
	CollectionIDs []string
	Limit         int32
	Instructions  string
	Retrieval     string
}

// CollectionsSearch searches the given collections on the server.
func CollectionsSearch(cfg CollectionsSearchConfig) *xaiapi.Tool {
	search := &xaiapi.CollectionsSearch{
		CollectionIds: cfg.CollectionIDs,
		Instructions:  strPtr(cfg.Instructions),
	}
	if cfg.Limit > 0 {
		limit := cfg.Limit
		search.Limit = &limit
	}
	switch cfg.Retrieval {
	case "hybrid":
		search.RetrievalMode = &xaiapi.CollectionsSearch_HybridRetrieval{HybridRetrieval: &xaiapi.HybridRetrieval{}}
	case "semantic":
		search.RetrievalMode = &xaiapi.CollectionsSearch_SemanticRetrieval{SemanticRetrieval: &xaiapi.SemanticRetrieval{}}
	case "keyword":
		search.RetrievalMode = &xaiapi.CollectionsSearch_KeywordRetrieval{KeywordRetrieval: &xaiapi.KeywordRetrieval{}}
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

// MCPConfig connects a remote MCP server.
// An empty AllowedTools list allows every tool on that server.
type MCPConfig struct {
	Label         string
	Description   string
	ServerURL     string
	Authorization string
	AllowedTools  []string
	ExtraHeaders  map[string]string
}

// MCP connects a remote MCP server.
func MCP(cfg MCPConfig) *xaiapi.Tool {
	mcp := &xaiapi.MCP{
		ServerLabel:       cfg.Label,
		ServerDescription: cfg.Description,
		ServerUrl:         cfg.ServerURL,
		AllowedToolNames:  cfg.AllowedTools,
		Authorization:     strPtr(cfg.Authorization),
		ExtraHeaders:      cfg.ExtraHeaders,
	}
	return &xaiapi.Tool{Tool: &xaiapi.Tool_Mcp{Mcp: mcp}}
}

func flag(on bool) *bool {
	if !on {
		return nil
	}
	v := true
	return &v
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
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
