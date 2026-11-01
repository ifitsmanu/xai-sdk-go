package tui

import (
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// View draws the chat.
func (m *Model) View() string {
	if m.width < 8 || m.height < 6 {
		return "xai"
	}
	m.refresh()
	header := m.header()
	body := m.viewport.View()
	composer := m.composer()
	status := m.statusLine()
	ui := lipgloss.JoinVertical(lipgloss.Left, header, body, composer, status)
	return m.styles.screen.Width(m.width).Height(m.height).Render(ui)
}

func (m *Model) header() string {
	brand := m.styles.brand.Render("xai")
	meta := m.styles.meta.Render(m.modelName)
	gap := m.width - lipgloss.Width(brand) - lipgloss.Width(meta)
	if gap < 1 {
		gap = 1
	}
	row := brand + strings.Repeat(" ", gap) + meta
	rule := m.styles.rule.Render(strings.Repeat("─", m.width))
	return lipgloss.JoinVertical(lipgloss.Left, row, rule)
}

func (m *Model) composer() string {
	inner := m.width - 4
	if inner < 8 {
		inner = 8
	}
	return m.styles.box.Width(inner).Render(m.input.View())
}

func (m *Model) statusLine() string {
	parts := []string{m.modelName}
	switch {
	case m.busy:
		parts = append(parts, m.spinFrame()+" streaming")
	case !m.hasKey:
		parts = append(parts, "no API key")
	default:
		parts = append(parts, "ready")
	}
	if m.lastLatency > 0 && !m.busy {
		parts = append(parts, formatLatency(m.lastLatency))
	}
	parts = append(parts, "enter send", "? help")
	line := strings.Join(parts, "  ·  ")
	return m.styles.status.Width(m.width).Render(line)
}

func formatLatency(d time.Duration) string {
	if d < time.Millisecond {
		return d.Round(100 * time.Microsecond).String()
	}
	if d < time.Second {
		return d.Round(time.Millisecond).String()
	}
	return d.Round(10 * time.Millisecond).String()
}

func (m *Model) transcript() string {
	if m.help {
		return m.helpText()
	}
	if len(m.messages) == 0 {
		return m.empty()
	}
	width := m.viewport.Width - 2
	if width < 8 {
		width = 8
	}
	blocks := make([]string, 0, len(m.messages))
	for _, msg := range m.messages {
		blocks = append(blocks, m.renderMessage(msg, width))
	}
	return lipgloss.NewStyle().Padding(1, 2, 0, 2).Width(m.viewport.Width).Render(strings.Join(blocks, "\n\n"))
}

func (m *Model) renderMessage(msg message, width int) string {
	wrap := m.styles.body.Width(width)
	switch msg.role {
	case roleUser:
		return m.styles.you.Render("you") + "\n" + wrap.Render(msg.text())
	case roleError:
		return m.styles.err.Render(msg.body)
	case roleInfo:
		return m.styles.info.Render(msg.body)
	default:
		var b strings.Builder
		b.WriteString(m.styles.grok.Render("grok"))
		b.WriteByte('\n')
		if msg.reason() != "" && msg.text() == "" {
			b.WriteString(m.styles.thinking.Width(width).Render(msg.reason()))
		} else {
			text := msg.text()
			if msg.pending {
				text += m.styles.cursor.Render("▌")
			}
			b.WriteString(wrap.Render(text))
		}
		return b.String()
	}
}

func (m *Model) empty() string {
	mark := m.styles.emptyMark.Render("xai")
	title := m.styles.empty.Render("Ask anything.")
	var hint string
	if m.hasKey {
		hint = "Enter sends.   ? help."
	} else {
		hint = "Set XAI_API_KEY to talk to the API.   ? help."
	}
	block := lipgloss.JoinVertical(lipgloss.Center, mark, "", title, "", m.styles.hint.Render(hint))
	return lipgloss.Place(m.viewport.Width, m.viewport.Height, lipgloss.Center, lipgloss.Center, block)
}

func (m *Model) helpText() string {
	rows := [][2]string{
		{"enter", "send"},
		{"alt+enter", "new line"},
		{"esc", "stop the reply, or close this"},
		{"?", "help, when the composer is empty"},
		{"/model", "cycle, or /model <name>"},
		{"/system", "set the system prompt"},
		{"/clear", "clear the transcript"},
		{"/quit", "leave"},
		{"ctrl+c", "leave"},
		{"pgup", "scroll"},
	}
	lines := make([]string, 0, len(rows)+2)
	lines = append(lines, m.styles.emptyMark.Render("keys"), "")
	for _, row := range rows {
		key := m.styles.helpKey.Width(14).Render(row[0])
		lines = append(lines, key+m.styles.helpText.Render(row[1]))
	}
	return lipgloss.NewStyle().Padding(1, 2).Render(strings.Join(lines, "\n"))
}
