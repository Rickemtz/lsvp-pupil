package ui

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// aboutModel es la pantalla «Acerca de»: créditos al curso del LSVP-UAMI y datos de la instalación.
type aboutModel struct {
	version   string
	backend   string
	configDir string
	width     int
}

func (m aboutModel) Update(msg tea.Msg) (aboutModel, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc", "q", "enter":
			return m, send(backToMenuMsg{})
		}
	}
	return m, nil
}

func (m aboutModel) View() string {
	wrap := lipgloss.NewStyle().Width(textWidth(m.width))
	var b strings.Builder
	b.WriteString(styleTitle.Render(strAppTitle) + " " + styleSubtitle.Render(m.version) + "\n\n")
	b.WriteString(wrap.Render(strAboutIntro) + "\n\n")
	b.WriteString(styleTitle.Render(strAboutCreditsTitle) + "\n")
	b.WriteString(wrap.Render(strAboutCredits) + "\n")
	b.WriteString(styleSubtitle.Render(strAboutCourseURL) + "\n\n")
	b.WriteString(wrap.Render(strAboutLicense) + "\n\n")
	configDir := m.configDir
	if home, err := os.UserHomeDir(); err == nil && strings.HasPrefix(configDir, home+"/") {
		configDir = "~" + strings.TrimPrefix(configDir, home)
	}
	if configDir == "" {
		configDir = strAboutNoConfig
	}
	labelWidth := lipgloss.Width(styleLabel.Render(""))
	fmt.Fprintf(&b, "%s%s\n", styleLabel.Render(strAboutSandbox), m.backend)
	fmt.Fprintf(&b, "%s%s\n", styleLabel.Render(strAboutProgress),
		indent(hardWrap(configDir, textWidth(m.width)-labelWidth), strings.Repeat(" ", labelWidth))[labelWidth:])
	b.WriteString("\n" + styleHelp.Render(strAboutHelp))
	return b.String()
}
