// Package wsx is a small authenticated WebSocket helper.
package wsx

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"

	"github.com/coder/websocket"
)

// Conn is one WebSocket.
type Conn struct {
	ws *websocket.Conn
}

// Dial opens a WebSocket. key is sent as a bearer token and is not logged.
// readLimit is the largest message accepted. Realtime audio deltas exceed 100 KB,
// so the library default of 32 KiB is too small.
func Dial(ctx context.Context, url, key, userAgent string, readLimit int64) (*Conn, error) {
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
	ws.SetReadLimit(readLimit)
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

// SendBinary writes one binary message, such as raw audio.
func (c *Conn) SendBinary(ctx context.Context, data []byte) error {
	return c.ws.Write(ctx, websocket.MessageBinary, data)
}

// Recv reads the next message. A normal close from the server returns io.EOF.
func (c *Conn) Recv(ctx context.Context) (json.RawMessage, error) {
	_, raw, err := c.ws.Read(ctx)
	if err != nil {
		if closedNormally(err) {
			return nil, io.EOF
		}
		return nil, err
	}
	return json.RawMessage(raw), nil
}

// Close closes the socket. A socket the server already closed is not an error.
func (c *Conn) Close() error {
	if c == nil || c.ws == nil {
		return nil
	}
	err := c.ws.Close(websocket.StatusNormalClosure, "")
	if err == nil || closedNormally(err) {
		return nil
	}
	return err
}

func closedNormally(err error) bool {
	switch websocket.CloseStatus(err) {
	case websocket.StatusNormalClosure, websocket.StatusGoingAway:
		return true
	}
	return errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed)
}
