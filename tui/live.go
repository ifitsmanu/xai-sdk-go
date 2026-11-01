package tui

import (
	"context"
	"errors"
	"strings"

	"github.com/ifitsmanu/xai-sdk-go"
	"github.com/ifitsmanu/xai-sdk-go/chat"
	"github.com/ifitsmanu/xai-sdk-go/xaiapi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Turn is one finished user or assistant message sent back to the model.
type Turn struct {
	Role string
	Text string
}

// Piece is one streaming fragment.
type Piece struct {
	Text      string
	Reasoning string
	Err       error
}

// Sender streams a reply for the transcript so far, including the newest user turn.
type Sender interface {
	Stream(ctx context.Context, model, system string, history []Turn) (<-chan Piece, error)
}

// ClientSender streams through the xAI chat API.
type ClientSender struct {
	Client *xai.Client
}

// Stream opens a chat completion stream.
func (s ClientSender) Stream(ctx context.Context, model, system string, history []Turn) (<-chan Piece, error) {
	if s.Client == nil {
		return nil, errors.New("xai: client is nil")
	}
	msgs := make([]*xaiapi.Message, 0, len(history)+1)
	if strings.TrimSpace(system) != "" {
		msgs = append(msgs, chat.SystemText(system))
	}
	for _, turn := range history {
		switch turn.Role {
		case "user":
			msgs = append(msgs, chat.UserText(turn.Text))
		case "assistant":
			msgs = append(msgs, chat.Assistant(chat.Text(turn.Text)))
		}
	}
	conv := s.Client.Chat.Create(chat.Config{Model: model, Messages: msgs})
	out := make(chan Piece, 32)
	go func() {
		defer close(out)
		for delta, err := range conv.Stream(ctx) {
			if err != nil {
				out <- Piece{Err: err}
				return
			}
			text, reasoning := pieceText(delta)
			if text == "" && reasoning == "" {
				continue
			}
			out <- Piece{Text: text, Reasoning: reasoning}
		}
	}()
	return out, nil
}

func pieceText(delta *chat.StreamDelta) (string, string) {
	if delta == nil || delta.Chunk == nil {
		return "", ""
	}
	var text, reasoning strings.Builder
	for _, output := range delta.Chunk.Outputs {
		if output == nil || output.Delta == nil {
			continue
		}
		text.WriteString(output.Delta.Content)
		reasoning.WriteString(output.Delta.ReasoningContent)
	}
	return text.String(), reasoning.String()
}

func quietCancel(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) {
		return true
	}
	return status.Code(err) == codes.Canceled
}
