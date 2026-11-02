// Package responses is the Responses API at POST /v1/responses.
// The docs recommend it for new work. It is REST, not a gRPC method.
package responses

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"iter"
	"net/http"
	"strings"

	"github.com/ifitsmanu/xai-sdk-go/internal/rest"
	"github.com/ifitsmanu/xai-sdk-go/internal/rpc"
	"github.com/ifitsmanu/xai-sdk-go/internal/wsx"
)

// Service is the Responses API.
type Service struct {
	conn *rpc.Conn
}

// NewService binds the Responses API to a connection.
func NewService(conn *rpc.Conn) *Service {
	return &Service{conn: conn}
}

// Request is a POST /v1/responses body.
// Input is a string or a list of input items. Leave pointer fields nil to omit them.
type Request struct {
	Model              string          `json:"model,omitempty"`
	Input              any             `json:"input,omitempty"`
	Instructions       *string         `json:"instructions,omitempty"`
	MaxOutputTokens    *int            `json:"max_output_tokens,omitempty"`
	MaxTurns           *int            `json:"max_turns,omitempty"`
	Temperature        *float64        `json:"temperature,omitempty"`
	TopP               *float64        `json:"top_p,omitempty"`
	Store              *bool           `json:"store,omitempty"`
	Stream             *bool           `json:"stream,omitempty"`
	PreviousResponseID *string         `json:"previous_response_id,omitempty"`
	SafetyIdentifier   *string         `json:"safety_identifier,omitempty"`
	User               *string         `json:"user,omitempty"`
	ReasoningEffort    *string         `json:"reasoning_effort,omitempty"`
	ServiceTier        *string         `json:"service_tier,omitempty"`
	Tools              json.RawMessage `json:"tools,omitempty"`
	ToolChoice         json.RawMessage `json:"tool_choice,omitempty"`
	Text               json.RawMessage `json:"text,omitempty"`
	Reasoning          json.RawMessage `json:"reasoning,omitempty"`
	Metadata           json.RawMessage `json:"metadata,omitempty"`
	Include            []string        `json:"include,omitempty"`
}

// Response is a stored or created model response.
type Response struct {
	ID     string            `json:"id"`
	Model  string            `json:"model"`
	Status string            `json:"status"`
	Output []json.RawMessage `json:"output"`
	Raw    json.RawMessage   `json:"-"`
}

// Text joins output_text parts from the output items.
func (r *Response) Text() string {
	if r == nil {
		return ""
	}
	var b strings.Builder
	for _, item := range r.Output {
		var msg struct {
			Type    string `json:"type"`
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
			Text string `json:"text"`
		}
		if err := json.Unmarshal(item, &msg); err != nil {
			continue
		}
		if msg.Text != "" && (msg.Type == "output_text" || msg.Type == "") {
			b.WriteString(msg.Text)
		}
		for _, part := range msg.Content {
			if part.Type == "output_text" || part.Type == "text" || part.Type == "" {
				b.WriteString(part.Text)
			}
		}
	}
	return b.String()
}

// Create sends a response request. For a streamed body, use Stream.
func (s *Service) Create(ctx context.Context, req Request) (*Response, error) {
	if req.Stream != nil && *req.Stream {
		return nil, errors.New("xai: use Responses.Stream for a streaming response")
	}
	return s.do(ctx, http.MethodPost, "/v1/responses", req)
}

// Get loads a stored response.
func (s *Service) Get(ctx context.Context, id string) (*Response, error) {
	return s.do(ctx, http.MethodGet, "/v1/responses/"+id, nil)
}

// Delete removes a stored response.
func (s *Service) Delete(ctx context.Context, id string) error {
	return s.conn.CallJSON(ctx, http.MethodDelete, "/v1/responses/"+id, nil, nil)
}

// InputItems lists the input items stored for a response.
func (s *Service) InputItems(ctx context.Context, id string, limit int, after, order string) (json.RawMessage, error) {
	path := "/v1/responses/" + id + "/input_items"
	q := query(map[string]string{
		"limit": itoa(limit),
		"after": after,
		"order": order,
	})
	raw, err := s.conn.RESTClient().Do(ctx, http.MethodGet, path+q, nil)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(raw), nil
}

// CompactRequest is a POST /v1/responses/compact body.
type CompactRequest struct {
	Model string `json:"model"`
	Input any    `json:"input"`
}

// Compact shortens a Responses input window.
func (s *Service) Compact(ctx context.Context, req CompactRequest) (*Response, error) {
	return s.do(ctx, http.MethodPost, "/v1/responses/compact", req)
}

// Stream sends a response request with stream set and yields SSE data payloads.
func (s *Service) Stream(ctx context.Context, req Request) iter.Seq2[json.RawMessage, error] {
	on := true
	req.Stream = &on
	return func(yield func(json.RawMessage, error) bool) {
		raw, err := json.Marshal(req)
		if err != nil {
			yield(nil, err)
			return
		}
		ctx, cancel := s.conn.Context(ctx)
		defer cancel()
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, s.conn.RESTBase+"/v1/responses", bytes.NewReader(raw))
		if err != nil {
			yield(nil, err)
			return
		}
		httpReq.Header.Set("Authorization", "Bearer "+s.conn.APIKey)
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Accept", "text/event-stream")
		if s.conn.UserAgent != "" {
			httpReq.Header.Set("User-Agent", s.conn.UserAgent)
		}
		client := s.conn.RESTClient().HTTP
		if client == nil {
			client = http.DefaultClient
		}
		resp, err := client.Do(httpReq)
		if err != nil {
			yield(nil, err)
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 300))
			yield(nil, &rest.Error{Method: http.MethodPost, Path: "/v1/responses", Status: resp.StatusCode, Body: strings.TrimSpace(string(body))})
			return
		}
		sc := bufio.NewScanner(resp.Body)
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if line == "" || strings.HasPrefix(line, ":") {
				continue
			}
			data, ok := strings.CutPrefix(line, "data:")
			if !ok {
				continue
			}
			data = strings.TrimSpace(data)
			if data == "" || data == "[DONE]" {
				if data == "[DONE]" {
					return
				}
				continue
			}
			if !yield(json.RawMessage(data), nil) {
				return
			}
		}
		if err := sc.Err(); err != nil {
			yield(nil, err)
		}
	}
}

// Dial opens the Responses WebSocket at /v1/responses.
// Each turn is a response.create message with the same body as Create, without stream or background.
func (s *Service) Dial(ctx context.Context) (*Socket, error) {
	ws, err := wsx.Dial(ctx, rest.WebSocketBase(s.conn.RESTBase)+"/v1/responses", s.conn.APIKey, s.conn.UserAgent)
	if err != nil {
		return nil, err
	}
	return &Socket{conn: ws}, nil
}

// Socket is one Responses WebSocket.
type Socket struct {
	conn *wsx.Conn
}

// Send writes one JSON event, such as response.create.
func (s *Socket) Send(ctx context.Context, event any) error { return s.conn.Send(ctx, event) }

// Recv reads the next server event.
func (s *Socket) Recv(ctx context.Context) (json.RawMessage, error) { return s.conn.Recv(ctx) }

// Close closes the socket.
func (s *Socket) Close() error { return s.conn.Close() }

func (s *Service) do(ctx context.Context, method, path string, in any) (*Response, error) {
	raw, err := s.conn.RESTClient().Do(ctx, method, path, in)
	if err != nil {
		return nil, err
	}
	var resp Response
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, err
	}
	resp.Raw = json.RawMessage(raw)
	return &resp, nil
}

func query(pairs map[string]string) string {
	var b strings.Builder
	for k, v := range pairs {
		if v == "" || v == "0" {
			continue
		}
		if b.Len() == 0 {
			b.WriteByte('?')
		} else {
			b.WriteByte('&')
		}
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(v)
	}
	return b.String()
}

func itoa(n int) string {
	if n <= 0 {
		return ""
	}
	return jsonNumber(n)
}

func jsonNumber(n int) string {
	raw, _ := json.Marshal(n)
	return string(raw)
}
