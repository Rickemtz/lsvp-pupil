package ui

import (
	"fmt"
	"math"
	"net/url"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"lsvp-pupil/internal/content"
	"lsvp-pupil/internal/scoring"
	"lsvp-pupil/internal/session"
)

const barWidth = 20

// resultsModel muestra el resumen final de una partida.
type resultsModel struct {
	title   string
	summary session.Summary
	catalog *content.Catalog
	width   int
	height  int
	offset  int      // primera línea visible cuando no cabe todo
	notes   []string // récord, desbloqueos, avisos de guardado...
	back    tea.Msg  // a dónde volver: la lista de módulos o el menú
}

func (r resultsModel) Update(msg tea.Msg) (resultsModel, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		lines := strings.Count(r.content(), "\n") + 1
		room := r.room()
		switch key.String() {
		case "up", "k":
			r.offset = max(0, r.offset-1)
		case "down", "j":
			r.offset = max(0, min(r.offset+1, lines-room))
		case "pgup":
			r.offset = max(0, r.offset-room)
		case "pgdown":
			r.offset = max(0, min(r.offset+room, lines-room))
		case "enter", " ", "esc", "q":
			if r.back == nil {
				return r, send(backToLessonMsg{})
			}
			return r, send(r.back)
		}
	}
	return r, nil
}

// room es cuántas líneas caben (styleFrame usa dos, y una queda para el indicador de desplazamiento).
func (r resultsModel) room() int {
	if r.height == 0 {
		return minHeight - 3
	}
	return max(5, r.height-3)
}

// View muestra lo que cabe; si sobra contenido, se desplaza con ↑/↓.
func (r resultsModel) View() string {
	lines := strings.Split(r.content(), "\n")
	room := r.room()
	if len(lines) <= room+1 {
		return strings.Join(lines, "\n")
	}
	end := min(len(lines), r.offset+room)
	return strings.Join(lines[r.offset:end], "\n") + "\n" + styleHelp.Render(strResultsScroll)
}

func (r resultsModel) content() string {
	s := r.summary
	var b strings.Builder
	b.WriteString(styleTitle.Render(fmt.Sprintf(strResultsTitleFmt, r.title)) + "\n\n")

	row := func(label, value string) {
		b.WriteString("  " + styleLabel.Render(label) + value + "\n")
	}
	row(strResultsRank, rankStyle(s.Rank).Render(string(s.Rank)))
	row(strResultsScore, fmt.Sprintf(strResultsScoreFmt, s.Total, s.Max, s.Percent))
	row(strResultsTime, formatDuration(s.Duration))
	row(strResultsAccuracy, fmt.Sprintf(strResultsAccFmt, s.FirstTry, s.Exercises, s.Accuracy()*100))
	row(strResultsMaxCombo, fmt.Sprintf(strHUDComboFmt, s.MaxCombo))
	for _, n := range r.notes {
		b.WriteString("  " + styleCorrect.Render(n) + "\n")
	}

	b.WriteString("\n" + styleTitle.Render(strResultsByModule) + "\n")
	for _, m := range s.ByModule {
		fmt.Fprintf(&b, "  %-26s %s %3.0f%%\n", r.moduleTitle(m.Module), bar(m.Percent), m.Percent)
	}

	b.WriteString("\n" + styleTitle.Render(strResultsReview) + "\n")
	if len(s.Review) == 0 {
		b.WriteString("  " + strResultsNoReview + "\n")
	}
	width := r.width - 8
	if r.width == 0 {
		width = minWidth - 8
	}
	for _, t := range s.Review {
		misses := fmt.Sprintf(strResultsMissesFmt, t.Misses)
		if t.Misses == 1 {
			misses = fmt.Sprintf(strResultsMissFmt, t.Misses)
		}
		fmt.Fprintf(&b, "  • %s · %s\n", r.topicLabel(t), styleWrong.Render(misses))
		b.WriteString(styleSubtitle.Render(indent(hardWrap(t.Source, width), "    ")) + "\n")
	}
	help := strResultsHelp
	if _, ok := r.back.(backToMenuMsg); ok {
		help = strResultsHelpMenu
	}
	b.WriteString("\n" + styleHelp.Render(help))
	return b.String()
}

func (r resultsModel) moduleTitle(id int) string {
	if m, ok := r.catalog.Module(id); ok {
		return fmt.Sprintf("%d. %s", m.ID, m.Title)
	}
	return fmt.Sprint(id)
}

// topicLabel arma un nombre legible con el título del módulo y el ancla de la sección.
func (r resultsModel) topicLabel(t session.ReviewTopic) string {
	label := r.moduleTitle(t.Module)
	if sec := sectionName(t.Source); sec != "" {
		label += " › " + sec
	}
	return label
}

// sectionName convierte el ancla de una URL ("#breve-historia") en texto ("Breve historia").
func sectionName(source string) string {
	u, err := url.Parse(source)
	if err != nil || u.Fragment == "" {
		return ""
	}
	name := strings.TrimSpace(strings.NewReplacer("-", " ", "_", " ").Replace(u.Fragment))
	if name == "" {
		return ""
	}
	first := []rune(name)
	return strings.ToUpper(string(first[0])) + string(first[1:])
}

func bar(percent float64) string {
	filled := int(math.Round(math.Min(percent, 100) / 100 * barWidth))
	filled = max(0, filled)
	return styleBarFill.Render(strings.Repeat("█", filled)) +
		styleBarEmpty.Render(strings.Repeat("░", barWidth-filled))
}

func rankStyle(r scoring.Rank) lipgloss.Style {
	switch {
	case r.AtLeast(scoring.RankA):
		return styleRankGood
	case r.AtLeast(scoring.RankC):
		return styleRankMid
	}
	return styleRankLow
}

func formatDuration(d time.Duration) string {
	secs := int(d.Round(time.Second).Seconds())
	return fmt.Sprintf("%d:%02d", secs/60, secs%60)
}

func indent(s, prefix string) string {
	return prefix + strings.ReplaceAll(s, "\n", "\n"+prefix)
}
