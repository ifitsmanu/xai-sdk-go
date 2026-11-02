package tui

import (
	"context"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	roleUser role = iota
	roleAssistant
	roleError
	roleInfo
)

type role int

type message struct {
	role      role
	body      string
	reasoning string
	bodyBuf   []byte
	reasonBuf []byte
	pending   bool
}

func (msg message) text() string {
	if msg.bodyBuf != nil {
		return string(msg.bodyBuf)
	}
	return msg.body
}

func (msg message) reason() string {
	if msg.reasonBuf != nil {
		return string(msg.reasonBuf)
	}
	return msg.reasoning
}

// Config is the test UI.
type Config struct {
	Sender    Sender
	ModelName string
	Models    []string
	HasKey    bool
	Width     int
	Height    int
}

// Model is the chat terminal.
type Model struct {
	sender Sender
	hasKey bool
	styles styles

	modelName   string
	models      []string
	system      string
	messages    []message
	busy        bool
	help        bool
	follow      bool
	gen         int
	events      <-chan Piece
	cancel      context.CancelFunc
	spin        int
	sentAt      time.Time
	lastLatency time.Duration

	width  int
	height int

	viewport viewport.Model
	input    textarea.Model
}

// New builds the chat UI. A nil sender still renders; sending explains that.
func New(cfg Config) *Model {
	models := cfg.Models
	name := cfg.ModelName
	if name == "" && len(models) > 0 {
		name = models[0]
	}
	input := textarea.New()
	input.Prompt = "› "
	input.Placeholder = "Message"
	input.ShowLineNumbers = false
	input.CharLimit = 0
	input.SetHeight(3)
	input.MaxHeight = 3
	input.KeyMap.InsertNewline = key.NewBinding(key.WithKeys("alt+enter", "ctrl+j"))
	input.FocusedStyle.Base = lipgloss.NewStyle()
	input.BlurredStyle.Base = lipgloss.NewStyle()
	input.FocusedStyle.Text = lipgloss.NewStyle().Foreground(lipgloss.Color(colorFG))
	input.FocusedStyle.Placeholder = lipgloss.NewStyle().Foreground(lipgloss.Color(colorFaint))
	input.FocusedStyle.Prompt = lipgloss.NewStyle().Foreground(lipgloss.Color(colorMuted))
	input.FocusedStyle.CursorLine = lipgloss.NewStyle()
	input.Cursor.Style = lipgloss.NewStyle().Foreground(lipgloss.Color(colorAccent))

	vp := viewport.New(0, 0)
	vp.MouseWheelEnabled = true

	m := &Model{
		sender:    cfg.Sender,
		hasKey:    cfg.HasKey,
		styles:    newStyles(),
		modelName: name,
		models:    models,
		follow:    true,
		width:     cfg.Width,
		height:    cfg.Height,
		viewport:  vp,
		input:     input,
	}
	if cfg.Width > 0 && cfg.Height > 0 {
		m.layout(cfg.Width, cfg.Height)
	}
	return m
}

// Init starts the cursor blink.
func (m *Model) Init() tea.Cmd {
	return m.input.Focus()
}

// Update handles keys, streaming pieces, and window size.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.layout(msg.Width, msg.Height)
		return m, nil
	case tea.MouseMsg:
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		m.follow = m.viewport.AtBottom()
		return m, cmd
	case tickMsg:
		if !m.busy {
			return m, nil
		}
		m.spin = (m.spin + 1) % len(spinFrames)
		return m, tick()
	case genMsg:
		return m.onPiece(msg)
	case tea.KeyMsg:
		return m.onKey(msg)
	default:
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
}

func (m *Model) onKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		m.stop()
		return m, tea.Quit
	case "esc":
		if m.help {
			m.help = false
			m.refresh()
			return m, nil
		}
		if m.busy {
			m.stop()
			m.note("Stopped.")
			return m, nil
		}
	case "?":
		if strings.TrimSpace(m.input.Value()) == "" {
			m.help = !m.help
			m.refresh()
			return m, nil
		}
	case "pgup":
		m.viewport.ScrollUp(m.viewport.Height / 2)
		m.follow = false
		return m, nil
	case "pgdown":
		m.viewport.ScrollDown(m.viewport.Height / 2)
		m.follow = m.viewport.AtBottom()
		return m, nil
	case "enter":
		if m.busy {
			return m, nil
		}
		return m.submit()
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m *Model) submit() (tea.Model, tea.Cmd) {
	text := strings.TrimSpace(m.input.Value())
	m.input.Reset()
	if text == "" {
		return m, nil
	}
	if strings.HasPrefix(text, "/") {
		cmd := strings.Fields(text)[0]
		if cmd == "/quit" || cmd == "/q" {
			m.stop()
			return m, tea.Quit
		}
		m.command(text)
		return m, nil
	}
	if m.sender == nil || !m.hasKey {
		m.messages = append(m.messages, message{role: roleUser, body: text})
		m.note("Set XAI_API_KEY to send this.")
		return m, nil
	}
	if m.modelName == "" {
		m.messages = append(m.messages, message{role: roleUser, body: text})
		m.note("No model listed. Use /model and an id from the API.")
		return m, nil
	}
	m.help = false
	m.messages = append(m.messages,
		message{role: roleUser, body: text},
		message{role: roleAssistant, pending: true},
	)
	m.busy = true
	m.sentAt = time.Now()
	m.follow = true
	m.gen++
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	ch, err := m.sender.Stream(ctx, m.modelName, m.system, m.history())
	if err != nil {
		m.finish(err)
		return m, nil
	}
	m.events = ch
	m.refresh()
	return m, tea.Batch(wait(m.gen, ch), tick())
}

func (m *Model) command(text string) {
	fields := strings.Fields(text)
	switch fields[0] {
	case "/help", "/?":
		m.help = !m.help
	case "/clear":
		m.messages = nil
		m.help = false
	case "/model":
		if len(fields) == 1 {
			if len(m.models) == 0 {
				m.note("No models listed.")
				break
			}
			m.modelName = m.nextModel()
			m.note("Model is " + m.modelName + ".")
		} else {
			m.modelName = strings.TrimSpace(strings.TrimPrefix(text, "/model"))
			m.note("Model is " + m.modelName + ".")
		}
	case "/system":
		m.system = strings.TrimSpace(strings.TrimPrefix(text, "/system"))
		if m.system == "" {
			m.note("System prompt cleared.")
		} else {
			m.note("System prompt set.")
		}
	default:
		m.note("Unknown command. ? lists them.")
	}
	m.refresh()
}

func (m *Model) onPiece(msg genMsg) (tea.Model, tea.Cmd) {
	if msg.id != m.gen {
		return m, nil
	}
	if msg.closed {
		m.finish(nil)
		return m, nil
	}
	if msg.piece.Err != nil {
		m.finish(msg.piece.Err)
		return m, nil
	}
	m.appendPiece(msg.piece)
	return m, wait(m.gen, m.events)
}

func (m *Model) appendPiece(p Piece) {
	if len(m.messages) == 0 || m.messages[len(m.messages)-1].role != roleAssistant {
		m.messages = append(m.messages, message{role: roleAssistant, pending: true})
	}
	last := &m.messages[len(m.messages)-1]
	if p.Text != "" {
		last.bodyBuf = append(last.bodyBuf, p.Text...)
	}
	if p.Reasoning != "" {
		last.reasonBuf = append(last.reasonBuf, p.Reasoning...)
	}
	last.pending = true
	m.refresh()
}

func (m *Model) finish(err error) {
	if m.busy && !m.sentAt.IsZero() {
		m.lastLatency = time.Since(m.sentAt)
	}
	m.busy = false
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
	if n := len(m.messages); n > 0 && m.messages[n-1].pending {
		m.messages[n-1].pending = false
		if m.messages[n-1].bodyBuf != nil {
			m.messages[n-1].body = string(m.messages[n-1].bodyBuf)
			m.messages[n-1].bodyBuf = nil
		}
		if m.messages[n-1].reasonBuf != nil {
			m.messages[n-1].reasoning = string(m.messages[n-1].reasonBuf)
			m.messages[n-1].reasonBuf = nil
		}
		if strings.TrimSpace(m.messages[n-1].body) == "" && strings.TrimSpace(m.messages[n-1].reasoning) == "" {
			m.messages = m.messages[:n-1]
		}
	}
	if err != nil && !quietCancel(err) {
		m.messages = append(m.messages, message{role: roleError, body: err.Error()})
	}
	m.refresh()
}

func (m *Model) stop() {
	if m.cancel != nil {
		m.cancel()
	}
}

func (m *Model) note(text string) {
	m.messages = append(m.messages, message{role: roleInfo, body: text})
	m.refresh()
}

func (m *Model) history() []Turn {
	out := make([]Turn, 0, len(m.messages))
	for _, msg := range m.messages {
		if msg.pending {
			continue
		}
		switch msg.role {
		case roleUser:
			out = append(out, Turn{Role: "user", Text: msg.body})
		case roleAssistant:
			if strings.TrimSpace(msg.body) != "" {
				out = append(out, Turn{Role: "assistant", Text: msg.body})
			}
		}
	}
	return out
}

func (m *Model) layout(width, height int) {
	m.width = width
	m.height = height
	innerH := 3
	if height < 14 {
		innerH = 1
		m.input.MaxHeight = 1
		m.input.SetHeight(1)
	} else {
		m.input.MaxHeight = 3
		m.input.SetHeight(3)
	}
	chrome := 2 + innerH + 2 + 1 // header, rule, composer text, border, status
	vpH := height - chrome
	if vpH < 1 {
		vpH = 1
	}
	contentW := width - 4
	if contentW < 8 {
		contentW = 8
	}
	m.viewport.Width = width
	m.viewport.Height = vpH
	m.input.SetWidth(contentW)
	m.refresh()
}

func (m *Model) refresh() {
	if m.width == 0 {
		return
	}
	atBottom := m.follow || m.viewport.AtBottom()
	m.viewport.SetContent(m.transcript())
	if atBottom {
		m.viewport.GotoBottom()
		m.follow = true
	}
}

type tickMsg time.Time

func tick() tea.Cmd {
	return tea.Tick(90*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

type genMsg struct {
	id     int
	piece  Piece
	closed bool
}

func wait(id int, ch <-chan Piece) tea.Cmd {
	return func() tea.Msg {
		piece, ok := <-ch
		if !ok {
			return genMsg{id: id, closed: true}
		}
		return genMsg{id: id, piece: piece}
	}
}

var spinFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func (m *Model) spinFrame() string {
	return spinFrames[m.spin%len(spinFrames)]
}

func (m *Model) nextModel() string {
	if len(m.models) == 0 {
		return m.modelName
	}
	for i, name := range m.models {
		if name == m.modelName {
			return m.models[(i+1)%len(m.models)]
		}
	}
	return m.models[0]
}
