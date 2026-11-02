package responses

import (
	"encoding/json"
	"github.com/ifitsmanu/xai-sdk-go/internal/wire"
	"strings"
)

// Request is a POST /v1/responses body. Zero values are omitted so the server default applies.
type Request struct {
	Model              string            `json:"model"`
	Input              Input             `json:"input"`
	Instructions       string            `json:"instructions,omitempty"`
	Tools              []Tool            `json:"tools,omitempty"`
	ToolChoice         *ToolChoice       `json:"tool_choice,omitempty"`
	ParallelToolCalls  *bool             `json:"parallel_tool_calls,omitempty"`
	PreviousResponseID string            `json:"previous_response_id,omitempty"`
	Store              *bool             `json:"store,omitempty"`
	Reasoning          *Reasoning        `json:"reasoning,omitempty"`
	Text               *TextConfig       `json:"text,omitempty"`
	MaxOutputTokens    int               `json:"max_output_tokens,omitempty"`
	MaxTurns           int               `json:"max_turns,omitempty"`
	Temperature        *float64          `json:"temperature,omitempty"`
	TopP               *float64          `json:"top_p,omitempty"`
	ServiceTier        string            `json:"service_tier,omitempty"`
	PromptCacheKey     string            `json:"prompt_cache_key,omitempty"`
	Include            []string          `json:"include,omitempty"`
	Metadata           map[string]string `json:"metadata,omitempty"`
	User               string            `json:"user,omitempty"`
	SafetyIdentifier   string            `json:"safety_identifier,omitempty"`
	Background         bool              `json:"background,omitempty"`
}

// Input is Text or Items.
type Input interface{ isInput() }

// Text is a single user message.
type Text string

// Items is a conversation: messages, earlier output items, and tool results.
type Items []Item

func (Text) isInput()  {}
func (Items) isInput() {}

// Item is one conversation entry. Output items can be sent back as input.
type Item interface{ isItem() }

// Message is a user, system, developer, or assistant message.
type Message struct {
	ID      string    `json:"id,omitempty"`
	Role    string    `json:"role"`
	Status  string    `json:"status,omitempty"`
	Content []Content `json:"content"`
}

// Content is one part of a message. Type is input_text, input_image, output_text, or refusal.
type Content struct {
	Type        string            `json:"type"`
	Text        string            `json:"text,omitempty"`
	ImageURL    string            `json:"image_url,omitempty"`
	FileID      string            `json:"file_id,omitempty"`
	Detail      string            `json:"detail,omitempty"`
	Refusal     string            `json:"refusal,omitempty"`
	Annotations []json.RawMessage `json:"annotations,omitempty"`
}

// FunctionCall is a call the model asks the caller to run.
type FunctionCall struct {
	ID        string `json:"id,omitempty"`
	CallID    string `json:"call_id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
	Status    string `json:"status,omitempty"`
}

// FunctionOutput returns a function result to the model.
type FunctionOutput struct {
	CallID string `json:"call_id"`
	Output string `json:"output"`
}

// ReasoningItem is the model's reasoning. Send it back to keep encrypted reasoning across turns.
type ReasoningItem struct {
	ID               string    `json:"id,omitempty"`
	Summary          []Summary `json:"summary"`
	EncryptedContent string    `json:"encrypted_content,omitempty"`
	Status           string    `json:"status,omitempty"`
}

// Summary is one reasoning summary part.
type Summary struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// OtherItem is any item without its own type here, such as web_search_call or mcp_call.
// Raw is the item as the server sent it and is what gets sent back as input.
type OtherItem struct {
	Type   string          `json:"type"`
	ID     string          `json:"id"`
	Status string          `json:"status"`
	Raw    json.RawMessage `json:"-"`
}

func (*Message) isItem()        {}
func (*FunctionCall) isItem()   {}
func (*FunctionOutput) isItem() {}
func (*ReasoningItem) isItem()  {}
func (*OtherItem) isItem()      {}

// UserText is a user message with one text part.
func UserText(text string) *Message {
	return &Message{Role: "user", Content: []Content{{Type: "input_text", Text: text}}}
}

func (m *Message) MarshalJSON() ([]byte, error) {
	type plain Message
	return wire.Typed("message", (*plain)(m))
}

func (f *FunctionCall) MarshalJSON() ([]byte, error) {
	type plain FunctionCall
	return wire.Typed("function_call", (*plain)(f))
}

func (f *FunctionOutput) MarshalJSON() ([]byte, error) {
	type plain FunctionOutput
	return wire.Typed("function_call_output", (*plain)(f))
}

func (r *ReasoningItem) MarshalJSON() ([]byte, error) {
	type plain ReasoningItem
	return wire.Typed("reasoning", (*plain)(r))
}

func (o *OtherItem) MarshalJSON() ([]byte, error) { return o.Raw, nil }

// Tool is a function the caller runs, or a tool xAI runs on the server.
type Tool interface{ isTool() }

// Function is a tool the caller runs. Parameters is a JSON Schema object.
type Function struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Parameters  any    `json:"parameters"`
	Strict      *bool  `json:"strict,omitempty"`
}

// WebSearch searches the web on the server.
type WebSearch struct {
	Filters           *SearchFilters `json:"filters,omitempty"`
	SearchContextSize string         `json:"search_context_size,omitempty"`
}

// SearchFilters limits web search to or away from domains. Set one list, not both.
type SearchFilters struct {
	AllowedDomains  []string `json:"allowed_domains,omitempty"`
	ExcludedDomains []string `json:"excluded_domains,omitempty"`
}

// XSearch searches X posts on the server. Dates are YYYY-MM-DD.
type XSearch struct {
	AllowedHandles     []string `json:"allowed_x_handles,omitempty"`
	ExcludedHandles    []string `json:"excluded_x_handles,omitempty"`
	FromDate           string   `json:"from_date,omitempty"`
	ToDate             string   `json:"to_date,omitempty"`
	ImageUnderstanding bool     `json:"enable_image_understanding,omitempty"`
	VideoUnderstanding bool     `json:"enable_video_understanding,omitempty"`
}

// CodeInterpreter runs code on the server.
type CodeInterpreter struct{}

// FileSearch searches collections on the server. VectorStoreIDs are collection ids.
type FileSearch struct {
	VectorStoreIDs []string `json:"vector_store_ids"`
	MaxNumResults  int      `json:"max_num_results,omitempty"`
}

// ImageGeneration lets the model create or edit images. Action is auto, generate, or edit.
type ImageGeneration struct {
	Action string `json:"action,omitempty"`
}

// MCP is a remote MCP server that xAI calls. An empty AllowedTools allows all of them.
type MCP struct {
	ServerLabel       string            `json:"server_label"`
	ServerURL         string            `json:"server_url"`
	ServerDescription string            `json:"server_description,omitempty"`
	AllowedTools      []string          `json:"allowed_tools,omitempty"`
	Authorization     string            `json:"authorization,omitempty"`
	Headers           map[string]string `json:"headers,omitempty"`
	RequireApproval   string            `json:"require_approval,omitempty"`
}

func (Function) isTool()        {}
func (WebSearch) isTool()       {}
func (XSearch) isTool()         {}
func (CodeInterpreter) isTool() {}
func (FileSearch) isTool()      {}
func (ImageGeneration) isTool() {}
func (MCP) isTool()             {}

func (t Function) MarshalJSON() ([]byte, error) {
	type plain Function
	return wire.Typed("function", plain(t))
}

func (t WebSearch) MarshalJSON() ([]byte, error) {
	type plain WebSearch
	return wire.Typed("web_search", plain(t))
}

func (t XSearch) MarshalJSON() ([]byte, error) {
	type plain XSearch
	return wire.Typed("x_search", plain(t))
}

func (CodeInterpreter) MarshalJSON() ([]byte, error) {
	return wire.Typed("code_interpreter", struct{}{})
}

func (t FileSearch) MarshalJSON() ([]byte, error) {
	type plain FileSearch
	return wire.Typed("file_search", plain(t))
}

func (t ImageGeneration) MarshalJSON() ([]byte, error) {
	type plain ImageGeneration
	return wire.Typed("image_generation", plain(t))
}

func (t MCP) MarshalJSON() ([]byte, error) {
	type plain MCP
	return wire.Typed("mcp", plain(t))
}

// ToolChoice is a mode (none, auto, required) or one function by name.
type ToolChoice struct {
	Mode     string
	Function string
}

func (c ToolChoice) MarshalJSON() ([]byte, error) {
	if c.Function != "" {
		return json.Marshal(map[string]string{"type": "function", "name": c.Function})
	}
	return json.Marshal(c.Mode)
}

// Reasoning sets how hard a reasoning model thinks. Supported efforts come from models.Capabilities.
type Reasoning struct {
	Effort string `json:"effort,omitempty"`
}

// TextConfig sets the output format.
type TextConfig struct {
	Format Format `json:"format"`
}

// Format is text, json_object, or json_schema with Schema.
type Format struct {
	Type   string `json:"type"`
	Schema any    `json:"schema,omitempty"`
}

// JSONSchema asks for output that matches schema.
func JSONSchema(schema any) *TextConfig {
	return &TextConfig{Format: Format{Type: "json_schema", Schema: schema}}
}

// Response is a created or stored response.
type Response struct {
	ID                 string `json:"id"`
	Model              string `json:"model"`
	Status             string `json:"status"`
	CreatedAt          int64  `json:"created_at"`
	PreviousResponseID string `json:"previous_response_id"`
	Output             []Item `json:"-"`
	Usage              *Usage `json:"usage"`
	Error              *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	IncompleteDetails *struct {
		Reason string `json:"reason"`
	} `json:"incomplete_details"`
}

// Usage counts tokens and reports the cost the server charged.
type Usage struct {
	InputTokens        int64 `json:"input_tokens"`
	OutputTokens       int64 `json:"output_tokens"`
	TotalTokens        int64 `json:"total_tokens"`
	InputTokensDetails struct {
		CachedTokens int64 `json:"cached_tokens"`
	} `json:"input_tokens_details"`
	OutputTokensDetails struct {
		ReasoningTokens int64 `json:"reasoning_tokens"`
	} `json:"output_tokens_details"`
	NumSourcesUsed         int64 `json:"num_sources_used"`
	NumServerSideToolsUsed int64 `json:"num_server_side_tools_used"`
	// CostInUSDTicks is what the server charged. One dollar is TicksPerUSD ticks.
	CostInUSDTicks int64 `json:"cost_in_usd_ticks"`
}

// TicksPerUSD converts Usage.CostInUSDTicks to dollars.
const TicksPerUSD = 10_000_000_000

// USD is the charged cost in dollars.
func (u *Usage) USD() float64 {
	if u == nil {
		return 0
	}
	return float64(u.CostInUSDTicks) / TicksPerUSD
}

func (r *Response) UnmarshalJSON(data []byte) error {
	type plain Response
	var wire struct {
		*plain
		Output []json.RawMessage `json:"output"`
	}
	wire.plain = (*plain)(r)
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	r.Output = make([]Item, 0, len(wire.Output))
	for _, raw := range wire.Output {
		item, err := decodeItem(raw)
		if err != nil {
			return err
		}
		r.Output = append(r.Output, item)
	}
	return nil
}

// Text joins the output_text parts of the assistant messages.
func (r *Response) Text() string {
	var b strings.Builder
	for _, item := range r.Output {
		if m, ok := item.(*Message); ok {
			for _, c := range m.Content {
				if c.Type == "output_text" {
					b.WriteString(c.Text)
				}
			}
		}
	}
	return b.String()
}

// FunctionCalls are the calls the caller must run before the next turn.
func (r *Response) FunctionCalls() []*FunctionCall {
	var calls []*FunctionCall
	for _, item := range r.Output {
		if c, ok := item.(*FunctionCall); ok {
			calls = append(calls, c)
		}
	}
	return calls
}

// Event is one streamed event. Response is set on response.* lifecycle events,
// Item on output_item events, and Delta on text and argument deltas.
type Event struct {
	Type           string    `json:"type"`
	SequenceNumber int64     `json:"sequence_number"`
	OutputIndex    int       `json:"output_index"`
	ContentIndex   int       `json:"content_index"`
	ItemID         string    `json:"item_id"`
	Delta          string    `json:"delta"`
	Response       *Response `json:"response"`
	Item           Item      `json:"-"`
}

func (e *Event) UnmarshalJSON(data []byte) error {
	type plain Event
	var wire struct {
		*plain
		Item json.RawMessage `json:"item"`
	}
	wire.plain = (*plain)(e)
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	if len(wire.Item) == 0 {
		return nil
	}
	item, err := decodeItem(wire.Item)
	e.Item = item
	return err
}

func decodeItem(raw json.RawMessage) (Item, error) {
	var head struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(raw, &head); err != nil {
		return nil, err
	}
	var item Item
	switch head.Type {
	case "message":
		item = &Message{}
	case "function_call":
		item = &FunctionCall{}
	case "function_call_output":
		item = &FunctionOutput{}
	case "reasoning":
		item = &ReasoningItem{}
	default:
		other := &OtherItem{Raw: append(json.RawMessage(nil), raw...)}
		return other, json.Unmarshal(raw, other)
	}
	return item, json.Unmarshal(raw, item)
}
