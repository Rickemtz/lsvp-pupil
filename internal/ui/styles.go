package ui

import "github.com/charmbracelet/lipgloss"

// Todos los colores y estilos de la interfaz.
var (
	colorAccent = lipgloss.AdaptiveColor{Light: "#1B7F3B", Dark: "#5FD787"}
	colorMuted  = lipgloss.AdaptiveColor{Light: "#6C6C6C", Dark: "#8A8A8A"}
	colorWarn   = lipgloss.AdaptiveColor{Light: "#B58900", Dark: "#FFD75F"}
	colorDanger = lipgloss.AdaptiveColor{Light: "#C62828", Dark: "#FF6B6B"}

	styleTitle    = lipgloss.NewStyle().Bold(true).Foreground(colorAccent)
	styleSubtitle = lipgloss.NewStyle().Foreground(colorMuted)
	styleItem     = lipgloss.NewStyle().PaddingLeft(2)
	styleSelected = lipgloss.NewStyle().PaddingLeft(0).Bold(true).Foreground(colorAccent)
	styleDisabled = lipgloss.NewStyle().PaddingLeft(2).Foreground(colorMuted)
	styleHelp     = lipgloss.NewStyle().Foreground(colorMuted)
	styleStatus   = lipgloss.NewStyle().Foreground(colorWarn)
	styleFrame    = lipgloss.NewStyle().Padding(1, 2)

	styleTimer       = lipgloss.NewStyle().Bold(true)
	styleTimerWarn   = lipgloss.NewStyle().Bold(true).Foreground(colorWarn)
	styleTimerDanger = lipgloss.NewStyle().Bold(true).Foreground(colorDanger)
	styleCorrect     = lipgloss.NewStyle().Bold(true).Foreground(colorAccent)
	styleWrong       = lipgloss.NewStyle().Bold(true).Foreground(colorDanger)
	styleHint        = lipgloss.NewStyle().Italic(true).Foreground(colorWarn)
	styleScore       = lipgloss.NewStyle().Bold(true)
	styleCombo       = lipgloss.NewStyle().Foreground(colorAccent)

	styleLabel    = lipgloss.NewStyle().Foreground(colorMuted).Width(12)
	styleBarFill  = lipgloss.NewStyle().Foreground(colorAccent)
	styleBarEmpty = lipgloss.NewStyle().Foreground(colorMuted)
	styleRankGood = lipgloss.NewStyle().Bold(true).Foreground(colorAccent)
	styleRankMid  = lipgloss.NewStyle().Bold(true).Foreground(colorWarn)
	styleRankLow  = lipgloss.NewStyle().Bold(true).Foreground(colorDanger)
)
