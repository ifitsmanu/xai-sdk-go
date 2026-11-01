package tui

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type scriptSender struct {
	pieces []Piece
}

func (s scriptSender) Stream(context.Context, string, string, []Turn) (<-chan Piece, error) {
	ch := make(chan Piece, len(s.pieces))
	for _, piece := range s.pieces {
		ch <- piece
	}
	close(ch)
	return ch, nil
}

func TestEmptyViewLooksLikeChat(t *testing.T) {
	m := New(Config{Width: 80, Height: 24, ModelName: "grok-4", HasKey: true})
	view := m.View()
	for _, want := range []string{"xai", "grok-4", "Ask anything.", "Message", "ready"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view missing %q\n%s", want, view)
		}
	}
}

func TestStreamPaintsReply(t *testing.T) {
	m := New(Config{Width: 80, Height: 24, ModelName: "grok-4", HasKey: true})
	m.messages = []message{{role: roleUser, body: "hi"}}
	m.appendPiece(Piece{Text: "Hello"})
	m.appendPiece(Piece{Text: " there"})
	m.finish(nil)
	view := m.View()
	if !strings.Contains(view, "you") || !strings.Contains(view, "hi") {
		t.Fatalf("user turn missing\n%s", view)
	}
	if !strings.Contains(view, "Hello there") {
		t.Fatalf("reply missing\n%s", view)
	}
	if !strings.Contains(view, "grok") {
		t.Fatalf("assistant label missing\n%s", view)
	}
}

func TestEnterStartsStream(t *testing.T) {
	m := New(Config{
		Width:  80,
		Height: 24,
		HasKey: true,
		Sender: scriptSender{pieces: []Piece{{Text: "ok"}}},
	})
	m.input.SetValue("hi")
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(*Model)
	if !m.busy || cmd == nil {
		t.Fatal("expected a running stream")
	}
	turns := m.history()
	if len(turns) != 1 || turns[0].Text != "hi" || turns[0].Role != "user" {
		t.Fatalf("history = %+v", turns)
	}
}

func TestModelCommand(t *testing.T) {
	m := New(Config{Width: 72, Height: 20, ModelName: "grok-4", HasKey: false})
	m.input.SetValue("/model grok-4.6")
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(*Model)
	if m.modelName != "grok-4.6" {
		t.Fatalf("model = %s", m.modelName)
	}
	if !strings.Contains(m.View(), "grok-4.6") {
		t.Fatalf("header did not update\n%s", m.View())
	}
}

func TestHelpOverlay(t *testing.T) {
	m := New(Config{Width: 72, Height: 22, HasKey: true})
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m = next.(*Model)
	if !strings.Contains(m.View(), "alt+enter") {
		t.Fatalf("help missing\n%s", m.View())
	}
}

func TestStatusShowsReplyLatency(t *testing.T) {
	m := New(Config{Width: 80, Height: 24, HasKey: true, ModelName: "grok-4"})
	m.lastLatency = 180 * time.Millisecond
	if !strings.Contains(m.View(), "180ms") {
		t.Fatalf("latency missing\n%s", m.View())
	}
}
