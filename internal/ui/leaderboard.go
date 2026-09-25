package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"lsvp-pupil/internal/storage"
)

// leaderboardModel muestra el top 10 de cada modo; ←/→ cambia de modo.
type leaderboardModel struct {
	scores storage.Scores
	idx    int
}

func (l leaderboardModel) Update(msg tea.Msg) (leaderboardModel, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return l, nil
	}
	n := len(leaderboardModes)
	switch key.String() {
	case "left", "h":
		l.idx = (l.idx - 1 + n) % n
	case "right", "l", "tab":
		l.idx = (l.idx + 1) % n
	case "esc", "q", "enter":
		return l, send(backToMenuMsg{})
	}
	return l, nil
}

func (l leaderboardModel) View() string {
	mode := leaderboardModes[l.idx]
	var b strings.Builder
	b.WriteString(styleTitle.Render(strLeaderboardTitle) + "\n\n")
	b.WriteString("  " + styleSelected.Render("← "+mode.title()+" →") + "  " +
		styleSubtitle.Render(fmt.Sprintf("%d/%d", l.idx+1, len(leaderboardModes))) + "\n\n")
	list := l.scores[mode.scoreKey()]
	if len(list) == 0 {
		b.WriteString("  " + styleSubtitle.Render(strLeaderboardEmpty) + "\n")
	} else {
		// Mismos anchos en el encabezado y en las filas; el rango se rellena antes de darle color.
		b.WriteString(styleSubtitle.Render(fmt.Sprintf("  %3s  %7s   %-5s  %6s  %7s   %s",
			"#", strColPoints, strColRank, strColPercent, strColTime, strColDate)) + "\n")
		for i, s := range list {
			fmt.Fprintf(&b, "  %2d.  %7d   %s  %5.0f%%  %7s   %s\n", i+1, s.Points,
				rankStyle(s.Rank).Render(fmt.Sprintf("%-5s", s.Rank)), s.Percent, formatDuration(s.Duration),
				s.Date.Local().Format("2006-01-02 15:04"))
		}
	}
	b.WriteString("\n" + styleHelp.Render(strLeaderboardHelp))
	return b.String()
}
