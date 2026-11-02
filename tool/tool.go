// Package tool defines a tool once and converts it to the chat, Responses,
// and realtime voice wire shapes, which differ from each other.
//
// Function tools run in your process. Server tools (web search, X search,
// code execution, collections, image generation, remote MCP) run on xAI.
package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/ifitsmanu/xai-sdk-go/responses"
	"github.com/ifitsmanu/xai-sdk-go/voice"
	"github.com/ifitsmanu/xai-sdk-go/xaiapi"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Tool is a Function or a server tool.
type Tool interface{ isTool() }

// Function is a tool the agent runs in this process.
type Function struct {
	Name        string
	Description string
	Parameters  any // JSON Schema object
	Run         func(ctx context.Context, args json.RawMessage) (string, error)
	// Writes marks a call with side effects. A runner asks for confirmation before running it.
	Writes bool
}

// WebSearch searches the web on xAI. Set at most one domain list.
type WebSearch struct {
	AllowedDomains     []string
	ExcludedDomains    []string
	ImageUnderstanding bool
}

// XSearch searches X posts on xAI. Set at most one handle list. Zero times are unset.
type XSearch struct {
	AllowedHandles     []string
	ExcludedHandles    []string
	From, To           time.Time
	ImageUnderstanding bool
	VideoUnderstanding bool
}

// CodeExecution runs code on xAI.
type CodeExecution struct{}

// Collections searches collections on xAI.
type Collections struct {
	IDs   []string
	Limit int
}

// ImageGeneration lets the model create or edit images. Action is auto, generate, or edit.
type ImageGeneration struct{ Action string }

// MCP is a remote MCP server that xAI calls. An empty AllowedTools allows all of them.
type MCP struct {
	Label         string
	URL           string
	Description   string
	AllowedTools  []string
	Authorization string
	Headers       map[string]string
}

func (*Function) isTool()       {}
func (WebSearch) isTool()       {}
func (XSearch) isTool()         {}
func (CodeExecution) isTool()   {}
func (Collections) isTool()     {}
func (ImageGeneration) isTool() {}
func (MCP) isTool()             {}

// Func builds a Function from a typed Go function. The parameter schema comes
// from A's fields and json tags; add `jsonschema:"description"` tags to describe them.
// A string result is returned as is; any other result is encoded as JSON.
func Func[A, R any](name, description string, fn func(context.Context, A) (R, error)) (*Function, error) {
	schema, err := jsonschema.For[A](nil)
	if err != nil {
		return nil, fmt.Errorf("tool %s: %w", name, err)
	}
	resolved, err := schema.Resolve(nil)
	if err != nil {
		return nil, fmt.Errorf("tool %s: %w", name, err)
	}
	return &Function{
		Name:        name,
		Description: description,
		Parameters:  schema,
		Run: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var args A
			if len(raw) > 0 {
				var generic any
				if err := json.Unmarshal(raw, &generic); err != nil {
					return "", fmt.Errorf("tool %s arguments: %w", name, err)
				}
				if err := resolved.Validate(generic); err != nil {
					return "", fmt.Errorf("tool %s arguments: %w", name, err)
				}
				if err := json.Unmarshal(raw, &args); err != nil {
					return "", fmt.Errorf("tool %s arguments: %w", name, err)
				}
			}
			out, err := fn(ctx, args)
			if err != nil {
				return "", err
			}
			if s, ok := any(out).(string); ok {
				return s, nil
			}
			b, err := json.Marshal(out)
			return string(b), err
		},
	}, nil
}

// Functions indexes the function tools by name.
func Functions(tools []Tool) map[string]*Function {
	out := map[string]*Function{}
	for _, t := range tools {
		if f, ok := t.(*Function); ok {
			out[f.Name] = f
		}
	}
	return out
}

// ForChat converts tools to the gRPC chat shape.
func ForChat(tools []Tool) ([]*xaiapi.Tool, error) {
	out := make([]*xaiapi.Tool, 0, len(tools))
	for _, t := range tools {
		var spec *xaiapi.Tool
		switch t := t.(type) {
		case *Function:
			params, err := json.Marshal(orEmptyObject(t.Parameters))
			if err != nil {
				return nil, fmt.Errorf("tool %s parameters: %w", t.Name, err)
			}
			spec = &xaiapi.Tool{Tool: &xaiapi.Tool_Function{Function: &xaiapi.Function{Name: t.Name, Description: t.Description, Parameters: string(params)}}}
		case WebSearch:
			spec = &xaiapi.Tool{Tool: &xaiapi.Tool_WebSearch{WebSearch: &xaiapi.WebSearch{
				AllowedDomains: t.AllowedDomains, ExcludedDomains: t.ExcludedDomains, EnableImageUnderstanding: flag(t.ImageUnderstanding),
			}}}
		case XSearch:
			x := &xaiapi.XSearch{
				AllowedXHandles: t.AllowedHandles, ExcludedXHandles: t.ExcludedHandles,
				EnableImageUnderstanding: flag(t.ImageUnderstanding), EnableVideoUnderstanding: flag(t.VideoUnderstanding),
			}
			if !t.From.IsZero() {
				x.FromDate = timestamppb.New(t.From)
			}
			if !t.To.IsZero() {
				x.ToDate = timestamppb.New(t.To)
			}
			spec = &xaiapi.Tool{Tool: &xaiapi.Tool_XSearch{XSearch: x}}
		case CodeExecution:
			spec = &xaiapi.Tool{Tool: &xaiapi.Tool_CodeExecution{CodeExecution: &xaiapi.CodeExecution{}}}
		case Collections:
			c := &xaiapi.CollectionsSearch{CollectionIds: t.IDs}
			if t.Limit > 0 {
				limit := int32(t.Limit)
				c.Limit = &limit
			}
			spec = &xaiapi.Tool{Tool: &xaiapi.Tool_CollectionsSearch{CollectionsSearch: c}}
		case ImageGeneration:
			g := &xaiapi.ImageGeneration{}
			if t.Action != "" {
				g.Action = &t.Action
			}
			spec = &xaiapi.Tool{Tool: &xaiapi.Tool_ImageGeneration{ImageGeneration: g}}
		case MCP:
			m := &xaiapi.MCP{ServerLabel: t.Label, ServerUrl: t.URL, ServerDescription: t.Description, AllowedToolNames: t.AllowedTools, ExtraHeaders: t.Headers}
			if t.Authorization != "" {
				m.Authorization = &t.Authorization
			}
			spec = &xaiapi.Tool{Tool: &xaiapi.Tool_Mcp{Mcp: m}}
		default:
			return nil, fmt.Errorf("tool: %T is not supported by chat", t)
		}
		out = append(out, spec)
	}
	return out, nil
}

// ForResponses converts tools to the Responses API shape.
func ForResponses(tools []Tool) ([]responses.Tool, error) {
	out := make([]responses.Tool, 0, len(tools))
	for _, t := range tools {
		switch t := t.(type) {
		case *Function:
			out = append(out, responses.Function{Name: t.Name, Description: t.Description, Parameters: orEmptyObject(t.Parameters)})
		case WebSearch:
			w := responses.WebSearch{}
			if len(t.AllowedDomains)+len(t.ExcludedDomains) > 0 {
				w.Filters = &responses.SearchFilters{AllowedDomains: t.AllowedDomains, ExcludedDomains: t.ExcludedDomains}
			}
			out = append(out, w)
		case XSearch:
			out = append(out, responses.XSearch{
				AllowedHandles: t.AllowedHandles, ExcludedHandles: t.ExcludedHandles, FromDate: date(t.From), ToDate: date(t.To),
				ImageUnderstanding: t.ImageUnderstanding, VideoUnderstanding: t.VideoUnderstanding,
			})
		case CodeExecution:
			out = append(out, responses.CodeInterpreter{})
		case Collections:
			out = append(out, responses.FileSearch{VectorStoreIDs: t.IDs, MaxNumResults: t.Limit})
		case ImageGeneration:
			out = append(out, responses.ImageGeneration{Action: t.Action})
		case MCP:
			out = append(out, responses.MCP{ServerLabel: t.Label, ServerURL: t.URL, ServerDescription: t.Description,
				AllowedTools: t.AllowedTools, Authorization: t.Authorization, Headers: t.Headers})
		default:
			return nil, fmt.Errorf("tool: %T is not supported by the Responses API", t)
		}
	}
	return out, nil
}

// ForVoice converts tools to the realtime voice shape. Voice has no code
// execution or image generation tool.
func ForVoice(tools []Tool) ([]voice.Tool, error) {
	out := make([]voice.Tool, 0, len(tools))
	for _, t := range tools {
		switch t := t.(type) {
		case *Function:
			out = append(out, voice.Function{Name: t.Name, Description: t.Description, Parameters: orEmptyObject(t.Parameters)})
		case WebSearch:
			out = append(out, voice.WebSearch{AllowedDomains: t.AllowedDomains, ExcludedDomains: t.ExcludedDomains, ImageUnderstanding: t.ImageUnderstanding})
		case XSearch:
			out = append(out, voice.XSearch{
				AllowedHandles: t.AllowedHandles, ExcludedHandles: t.ExcludedHandles, FromDate: date(t.From), ToDate: date(t.To),
				ImageUnderstanding: t.ImageUnderstanding, VideoUnderstanding: t.VideoUnderstanding,
			})
		case Collections:
			out = append(out, voice.FileSearch{VectorStoreIDs: t.IDs, MaxNumResults: t.Limit})
		case MCP:
			out = append(out, voice.MCP{ServerLabel: t.Label, ServerURL: t.URL, ServerDescription: t.Description,
				AllowedTools: t.AllowedTools, Authorization: t.Authorization, Headers: t.Headers})
		default:
			return nil, fmt.Errorf("tool: %T is not supported by realtime voice", t)
		}
	}
	return out, nil
}

func orEmptyObject(schema any) any {
	if schema == nil {
		return map[string]any{"type": "object", "properties": map[string]any{}}
	}
	return schema
}

func flag(on bool) *bool {
	if !on {
		return nil
	}
	return &on
}

func date(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.DateOnly)
}
