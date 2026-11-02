// Package responses is the Responses API at POST /v1/responses.
// The docs recommend it for new work. It is REST, not a gRPC method.
package responses

import (
	"bufio"
	"context"
	"encoding/json"
	"iter"
	"net/http"
	"net/url"
	"strconv"
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

// Create sends a request and waits for the whole response. Use Stream for tokens as they arrive.
func (s *Service) Create(ctx context.Context, req Request) (*Response, error) {
	return s.do(ctx, http.MethodPost, "/v1/responses", req)
}

// Get loads a stored response.
func (s *Service) Get(ctx context.Context, id string) (*Response, error) {
	return s.do(ctx, http.MethodGet, rest.Path("/v1/responses", id), nil)
}

// Delete removes a stored response.
func (s *Service) Delete(ctx context.Context, id string) error {
	return s.conn.RESTClient().Call(ctx, http.MethodDelete, rest.Path("/v1/responses", id), nil, nil)
}

// ItemPage is one page of stored input items.
type ItemPage struct {
	Items   []Item
	FirstID string
	LastID  string
	HasMore bool
}

// InputItems lists the input items stored for a response. Zero and empty values use server defaults.
func (s *Service) InputItems(ctx context.Context, id string, limit int, after, order string) (*ItemPage, error) {
	q := url.Values{"after": {after}, "order": {order}}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	var wire struct {
		Data    []json.RawMessage `json:"data"`
		FirstID string            `json:"first_id"`
		LastID  string            `json:"last_id"`
		HasMore bool              `json:"has_more"`
	}
	if err := s.conn.RESTClient().Call(ctx, http.MethodGet, rest.Query(rest.Path("/v1/responses", id, "input_items"), q), nil, &wire); err != nil {
		return nil, err
	}
	page := &ItemPage{FirstID: wire.FirstID, LastID: wire.LastID, HasMore: wire.HasMore}
	for _, raw := range wire.Data {
		item, err := decodeItem(raw)
		if err != nil {
			return nil, err
		}
		page.Items = append(page.Items, item)
	}
	return page, nil
}

// Compact shortens a conversation. Send the returned output as the next input.
func (s *Service) Compact(ctx context.Context, model string, input Input) (*Response, error) {
	return s.do(ctx, http.MethodPost, "/v1/responses/compact", map[string]any{"model": model, "input": input})
}

// Stream sends a request and yields events as the server sends them.
// The response.completed event carries the whole response.
func (s *Service) Stream(ctx context.Context, req Request) iter.Seq2[*Event, error] {
	return func(yield func(*Event, error) bool) {
		body, err := rest.JSON(struct {
			Request
			Stream bool `json:"stream"`
		}{req, true})
		if err != nil {
			yield(nil, err)
			return
		}
		resp, err := s.conn.RESTClient().Send(ctx, http.MethodPost, "/v1/responses", body, "text/event-stream")
		if err != nil {
			yield(nil, err)
			return
		}
		defer func() { _ = resp.Body.Close() }()
		sc := bufio.NewScanner(resp.Body)
		sc.Buffer(make([]byte, 64<<10), int(s.conn.RESTClient().Limit()))
		for sc.Scan() {
			data, ok := strings.CutPrefix(sc.Text(), "data:")
			if !ok {
				continue
			}
			data = strings.TrimSpace(data)
			if data == "[DONE]" {
				return
			}
			if data == "" {
				continue
			}
			var event Event
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				yield(nil, err)
				return
			}
			if !yield(&event, nil) {
				return
			}
		}
		if err := sc.Err(); err != nil {
			yield(nil, err)
		}
	}
}

// Dial opens the Responses WebSocket at /v1/responses for low-latency multi-turn use.
func (s *Service) Dial(ctx context.Context) (*Socket, error) {
	ws, err := s.conn.DialWS(ctx, "/v1/responses")
	if err != nil {
		return nil, err
	}
	return &Socket{conn: ws}, nil
}

// Socket is one Responses WebSocket.
type Socket struct {
	conn *wsx.Conn
}

// Create starts a turn. Read its events with Next until response.completed.
func (s *Socket) Create(ctx context.Context, req Request) error {
	return s.conn.Send(ctx, struct {
		Type string `json:"type"`
		Request
	}{"response.create", req})
}

// Next reads the next event. It returns io.EOF after the server closes.
func (s *Socket) Next(ctx context.Context) (*Event, error) {
	raw, err := s.conn.Recv(ctx)
	if err != nil {
		return nil, err
	}
	var event Event
	return &event, json.Unmarshal(raw, &event)
}

// Close closes the socket.
func (s *Socket) Close() error { return s.conn.Close() }

func (s *Service) do(ctx context.Context, method, path string, in any) (*Response, error) {
	var resp Response
	if err := s.conn.RESTClient().Call(ctx, method, path, in, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
