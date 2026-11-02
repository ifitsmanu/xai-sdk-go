// Package wsx is a small authenticated WebSocket helper.
package wsx

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/coder/websocket"
)

// Conn is one WebSocket.
type Conn struct {
	ws *websocket.Conn
}

// Dial opens a WebSocket. key is sent as a bearer token and is not logged.
func Dial(ctx context.Context, url, key, userAgent string) (*Conn, error) {
	header := http.Header{}
	if key != "" {
		header.Set("Authorization", "Bearer "+key)
	}
	if userAgent != "" {
		header.Set("User-Agent", userAgent)
	}
	ws, _, err := websocket.Dial(ctx, url, &websocket.DialOptions{HTTPHeader: header})
	if err != nil {
		return nil, err
	}
	return &Conn{ws: ws}, nil
}

// Send writes one JSON text message.
func (c *Conn) Send(ctx context.Context, v any) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return c.ws.Write(ctx, websocket.MessageText, raw)
}

// Recv reads the next text message.
func (c *Conn) Recv(ctx context.Context) (json.RawMessage, error) {
	_, raw, err := c.ws.Read(ctx)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(raw), nil
}

// Close closes the socket.
func (c *Conn) Close() error {
	if c == nil || c.ws == nil {
		return nil
	}
	return c.ws.Close(websocket.StatusNormalClosure, "")
}
