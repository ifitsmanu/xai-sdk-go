package tui_test

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ifitsmanu/xai-sdk-go/tui"
)

type scriptSender struct {
	pieces []tui.Piece
}

func (s scriptSender) Stream(context.Context, string, string, []tui.Turn) (<-chan tui.Piece, error) {
	time.Sleep(2 * time.Millisecond)
	ch := make(chan tui.Piece, len(s.pieces))
	for _, piece := range s.pieces {
		ch <- piece
	}
	close(ch)
	return ch, nil
}

func open(cfg tui.Config) *tui.Model {
	m := tui.New(cfg)
	cmd := m.Init()
	if cmd != nil {
		if msg := cmd(); msg != nil {
			next, _ := m.Update(msg)
			m = next.(*tui.Model)
		}
	}
	return m
}

func typeText(m *tui.Model, text string) *tui.Model {
	for _, r := range text {
		next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = next.(*tui.Model)
	}
	return m
}

func pressEnter(m *tui.Model) (*tui.Model, tea.Cmd) {
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	return next.(*tui.Model), cmd
}

func runCmds(m *tui.Model, cmd tea.Cmd, depth int) *tui.Model {
	if cmd == nil || depth > 40 {
		return m
	}
	msg := cmd()
	if msg == nil {
		return m
	}
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, child := range batch {
			m = runCmds(m, child, depth+1)
		}
		return m
	}
	next, nextCmd := m.Update(msg)
	return runCmds(next.(*tui.Model), nextCmd, depth+1)
}

func TestEmptyViewLooksLikeChat(t *testing.T) {
	m := open(tui.Config{Width: 80, Height: 24, ModelName: "grok-4", HasKey: true})
	view := m.View()
	for _, want := range []string{"xai", "grok-4", "Ask anything.", "Message", "ready"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view missing %q\n%s", want, view)
		}
	}
}

func TestStreamPaintsReply(t *testing.T) {
	m := open(tui.Config{
		Width:     80,
		Height:    24,
		ModelName: "grok-4",
		HasKey:    true,
		Sender:    scriptSender{pieces: []tui.Piece{{Text: "Hello"}, {Text: " there"}}},
	})
	m = typeText(m, "hi")
	m, cmd := pressEnter(m)
	m = runCmds(m, cmd, 0)
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
	if !strings.Contains(view, "ms") {
		t.Fatalf("latency missing\n%s", view)
	}
}

func TestEnterStartsStream(t *testing.T) {
	m := open(tui.Config{
		Width:     80,
		Height:    24,
		HasKey:    true,
		ModelName: "listed-model",
		Sender:    scriptSender{pieces: []tui.Piece{{Text: "ok"}}},
	})
	m = typeText(m, "hi")
	m, cmd := pressEnter(m)
	view := m.View()
	if cmd == nil || !strings.Contains(view, "streaming") || !strings.Contains(view, "hi") {
		t.Fatalf("expected a running stream\n%s", view)
	}
}

func TestModelCommand(t *testing.T) {
	m := open(tui.Config{Width: 72, Height: 20, ModelName: "grok-4", HasKey: false})
	m = typeText(m, "/model grok-4.6")
	m, _ = pressEnter(m)
	if !strings.Contains(m.View(), "grok-4.6") {
		t.Fatalf("header did not update\n%s", m.View())
	}
}

func TestHelpOverlay(t *testing.T) {
	m := open(tui.Config{Width: 72, Height: 22, HasKey: true})
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m = next.(*tui.Model)
	if !strings.Contains(m.View(), "alt+enter") {
		t.Fatalf("help missing\n%s", m.View())
	}
}
