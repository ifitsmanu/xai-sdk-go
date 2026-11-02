package voice

import (
	"context"
	"encoding/base64"
	"encoding/json"

	"github.com/ifitsmanu/xai-sdk-go/internal/wire"
	"github.com/ifitsmanu/xai-sdk-go/internal/wsx"
)

// Session configures a realtime conversation. Zero values keep the server default.
type Session struct {
	Model         string            `json:"model,omitempty"`
	Instructions  string            `json:"instructions,omitempty"`
	Voice         string            `json:"voice,omitempty"`
	Reasoning     *Effort           `json:"reasoning,omitempty"`
	TurnDetection *TurnDetection    `json:"turn_detection,omitempty"`
	Audio         *AudioConfig      `json:"audio,omitempty"`
	Tools         []Tool            `json:"tools,omitempty"`
	Replace       map[string]string `json:"replace,omitempty"`
	Resumption    *struct {
		Enabled bool `json:"enabled"`
	} `json:"resumption,omitempty"`
}

// Effort is high or none.
type Effort struct {
	Effort string `json:"effort"`
}

// TurnDetection is server_vad for automatic turns. IdleTimeoutMS re-engages a silent user.
type TurnDetection struct {
	Type          string `json:"type"`
	IdleTimeoutMS int    `json:"idle_timeout_ms,omitempty"`
}

// AudioConfig sets input and output formats.
type AudioConfig struct {
	Input  *AudioInput  `json:"input,omitempty"`
	Output *AudioOutput `json:"output,omitempty"`
}

// AudioInput is the caller's microphone format and transcription bias.
type AudioInput struct {
	Format        *AudioFormat `json:"format,omitempty"`
	Transport     string       `json:"transport,omitempty"`
	Transcription *struct {
		LanguageHint string   `json:"language_hint,omitempty"`
		Keyterms     []string `json:"keyterms,omitempty"`
	} `json:"transcription,omitempty"`
}

// AudioOutput is the assistant's audio format and speed (0.7 to 1.5).
type AudioOutput struct {
	Format    *AudioFormat `json:"format,omitempty"`
	Transport string       `json:"transport,omitempty"`
	Speed     float64      `json:"speed,omitempty"`
}

// AudioFormat is audio/pcm, audio/pcmu, audio/pcma, or audio/opus. Rate applies to PCM.
type AudioFormat struct {
	Type string `json:"type"`
	Rate int    `json:"rate,omitempty"`
}

// Tool is a tool the voice agent can use, in the realtime wire shape.
type Tool interface{ isTool() }

// Function is a tool the caller runs when a function_call_arguments.done event arrives.
type Function struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Parameters  any    `json:"parameters"`
}

// WebSearch searches the web on the server. Set at most one domain list.
type WebSearch struct {
	AllowedDomains     []string  `json:"allowed_domains,omitempty"`
	ExcludedDomains    []string  `json:"excluded_domains,omitempty"`
	Location           *Location `json:"location,omitempty"`
	ImageUnderstanding bool      `json:"enable_image_understanding,omitempty"`
}

// Location biases search results.
type Location struct {
	Country  string `json:"country,omitempty"`
	City     string `json:"city,omitempty"`
	Region   string `json:"region,omitempty"`
	Timezone string `json:"timezone,omitempty"`
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

// FileSearch searches collections. VectorStoreIDs are collection ids.
type FileSearch struct {
	VectorStoreIDs []string `json:"vector_store_ids"`
	MaxNumResults  int      `json:"max_num_results,omitempty"`
}

// MCP is a remote MCP server xAI calls during the conversation.
type MCP struct {
	ServerLabel       string            `json:"server_label"`
	ServerURL         string            `json:"server_url"`
	ServerDescription string            `json:"server_description,omitempty"`
	AllowedTools      []string          `json:"allowed_tools,omitempty"`
	Authorization     string            `json:"authorization,omitempty"`
	Headers           map[string]string `json:"headers,omitempty"`
}

func (Function) isTool()   {}
func (WebSearch) isTool()  {}
func (XSearch) isTool()    {}
func (FileSearch) isTool() {}
func (MCP) isTool()        {}

func (t Function) MarshalJSON() ([]byte, error) {
	type plain Function
	return json.Marshal(struct {
		Type     string `json:"type"`
		Function plain  `json:"function"`
	}{"function", plain(t)})
}

func (t WebSearch) MarshalJSON() ([]byte, error) {
	type plain WebSearch
	return wire.Typed("web_search", plain(t))
}

func (t XSearch) MarshalJSON() ([]byte, error) {
	type plain XSearch
	return wire.Typed("x_search", plain(t))
}

func (t FileSearch) MarshalJSON() ([]byte, error) {
	type plain FileSearch
	return wire.Typed("file_search", plain(t))
}

func (t MCP) MarshalJSON() ([]byte, error) {
	type plain MCP
	return wire.Typed("mcp", plain(t))
}

// Event is one server event. Fields are set according to Type.
type Event struct {
	Type           string          `json:"type"`
	EventID        string          `json:"event_id"`
	ResponseID     string          `json:"response_id"`
	ItemID         string          `json:"item_id"`
	PreviousItemID string          `json:"previous_item_id"`
	OutputIndex    int             `json:"output_index"`
	ContentIndex   int             `json:"content_index"`
	CallID         string          `json:"call_id"`
	Name           string          `json:"name"`
	Arguments      string          `json:"arguments"`
	Delta          string          `json:"delta"`
	Transcript     string          `json:"transcript"`
	AudioStartMS   int64           `json:"audio_start_ms"`
	AudioEndMS     int64           `json:"audio_end_ms"`
	Item           json.RawMessage `json:"item"`
	Response       *struct {
		ID     string `json:"id"`
		Status string `json:"status"`
		Usage  *struct {
			InputTokens  int64 `json:"input_tokens"`
			OutputTokens int64 `json:"output_tokens"`
			TotalTokens  int64 `json:"total_tokens"`
		} `json:"usage"`
	} `json:"response"`
	Error *struct {
		Type    string `json:"type"`
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// Server event types an agent usually handles.
const (
	EventAudioDelta      = "response.output_audio.delta"
	EventTranscriptDelta = "response.output_audio_transcript.delta"
	EventTextDelta       = "response.output_text.delta"
	EventFunctionCall    = "response.function_call_arguments.done"
	EventResponseDone    = "response.done"
	EventSpeechStarted   = "input_audio_buffer.speech_started"
	EventUserTranscript  = "conversation.item.input_audio_transcription.completed"
	EventError           = "error"
)

// Audio decodes the base64 audio in an audio delta.
func (e *Event) Audio() ([]byte, error) {
	return base64.StdEncoding.DecodeString(e.Delta)
}

// Socket is one voice WebSocket.
type Socket struct {
	*wsx.Conn
}

// Next reads the next server event. It returns io.EOF after the server closes.
func (s *Socket) Next(ctx context.Context) (*Event, error) {
	raw, err := s.Recv(ctx)
	if err != nil {
		return nil, err
	}
	var e Event
	return &e, json.Unmarshal(raw, &e)
}

// Update sends session.update.
func (s *Socket) Update(ctx context.Context, session Session) error {
	return s.Send(ctx, map[string]any{"type": "session.update", "session": session})
}

// AppendAudio streams microphone audio in the session's input format.
func (s *Socket) AppendAudio(ctx context.Context, pcm []byte) error {
	return s.Send(ctx, map[string]string{"type": "input_audio_buffer.append", "audio": base64.StdEncoding.EncodeToString(pcm)})
}

// Commit ends the user's turn when turn detection is off.
func (s *Socket) Commit(ctx context.Context) error {
	return s.Send(ctx, map[string]string{"type": "input_audio_buffer.commit"})
}

// SendText adds a text message from role (user, assistant, or system).
func (s *Socket) SendText(ctx context.Context, role, text string) error {
	kind := "input_text"
	if role == "assistant" {
		kind = "text"
	}
	return s.createItem(ctx, map[string]any{"type": "message", "role": role,
		"content": []map[string]string{{"type": kind, "text": text}}})
}

// SendFunctionOutput returns a function result. Call Respond to let the model continue.
func (s *Socket) SendFunctionOutput(ctx context.Context, callID, output string) error {
	return s.createItem(ctx, map[string]string{"type": "function_call_output", "call_id": callID, "output": output})
}

// Respond asks the model for a response. Server turn detection does this automatically.
func (s *Socket) Respond(ctx context.Context) error {
	return s.Send(ctx, map[string]string{"type": "response.create"})
}

// Cancel stops an in-progress response.
func (s *Socket) Cancel(ctx context.Context) error {
	return s.Send(ctx, map[string]string{"type": "response.cancel"})
}

func (s *Socket) createItem(ctx context.Context, item any) error {
	return s.Send(ctx, map[string]any{"type": "conversation.item.create", "item": item})
}
