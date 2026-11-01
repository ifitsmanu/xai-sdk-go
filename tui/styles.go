package tui

import "github.com/charmbracelet/lipgloss"

const (
	colorBG     = "#121212"
	colorFG     = "#ececec"
	colorMuted  = "#8d8d8d"
	colorFaint  = "#5e5e5e"
	colorLine   = "#2c2c2c"
	colorUser   = "#f2f2f2"
	colorAccent = "#ffffff"
	colorErr    = "#ff6b6b"
)

type styles struct {
	screen    lipgloss.Style
	brand     lipgloss.Style
	meta      lipgloss.Style
	rule      lipgloss.Style
	you       lipgloss.Style
	grok      lipgloss.Style
	body      lipgloss.Style
	thinking  lipgloss.Style
	err       lipgloss.Style
	info      lipgloss.Style
	emptyMark lipgloss.Style
	empty     lipgloss.Style
	hint      lipgloss.Style
	box       lipgloss.Style
	status    lipgloss.Style
	helpKey   lipgloss.Style
	helpText  lipgloss.Style
	cursor    lipgloss.Style
}

func newStyles() styles {
	return styles{
		screen:    lipgloss.NewStyle().Background(lipgloss.Color(colorBG)).Foreground(lipgloss.Color(colorFG)),
		brand:     lipgloss.NewStyle().Foreground(lipgloss.Color(colorAccent)).Bold(true),
		meta:      lipgloss.NewStyle().Foreground(lipgloss.Color(colorMuted)),
		rule:      lipgloss.NewStyle().Foreground(lipgloss.Color(colorLine)),
		you:       lipgloss.NewStyle().Foreground(lipgloss.Color(colorUser)).Bold(true),
		grok:      lipgloss.NewStyle().Foreground(lipgloss.Color(colorMuted)).Bold(true),
		body:      lipgloss.NewStyle().Foreground(lipgloss.Color(colorFG)),
		thinking:  lipgloss.NewStyle().Foreground(lipgloss.Color(colorFaint)).Italic(true),
		err:       lipgloss.NewStyle().Foreground(lipgloss.Color(colorErr)),
		info:      lipgloss.NewStyle().Foreground(lipgloss.Color(colorMuted)),
		emptyMark: lipgloss.NewStyle().Foreground(lipgloss.Color(colorAccent)).Bold(true),
		empty:     lipgloss.NewStyle().Foreground(lipgloss.Color(colorFG)),
		hint:      lipgloss.NewStyle().Foreground(lipgloss.Color(colorFaint)),
		box: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(colorLine)).
			Padding(0, 1),
		status:   lipgloss.NewStyle().Foreground(lipgloss.Color(colorFaint)),
		helpKey:  lipgloss.NewStyle().Foreground(lipgloss.Color(colorFG)).Bold(true),
		helpText: lipgloss.NewStyle().Foreground(lipgloss.Color(colorMuted)),
		cursor:   lipgloss.NewStyle().Foreground(lipgloss.Color(colorAccent)),
	}
}
