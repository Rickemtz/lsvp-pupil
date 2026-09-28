package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"lsvp-pupil/internal/content"
	"lsvp-pupil/internal/quiz"
	"lsvp-pupil/internal/scoring"
	"lsvp-pupil/internal/session"
)

// quizScreen muestra una pregunta teórica con límite de tiempo.
// En modo Lecciones permite 2 intentos y pistas con F1.
type quizScreen struct {
	ctx   exerciseCtx
	ex    content.Exercise
	width int

	choices   []string
	cursor    int
	selected  map[int]bool
	input     textinput.Model
	failed    int
	hints     int
	started   time.Time
	remaining time.Duration
	notice    string

	answered bool
	timedOut bool
	last     session.Result
}

func newQuiz(ctx exerciseCtx, ex content.Exercise) *quizScreen {
	in := textinput.New()
	in.Prompt = "> "
	in.Placeholder = strQuizInputHolder
	in.CharLimit = 120
	q := &quizScreen{ctx: ctx, ex: ex, input: in, selected: map[int]bool{}}
	q.choices = quiz.Shuffle(ex, ctx.rng)
	if ex.Format == content.FormatShortAnswer {
		q.input.Focus()
	}
	q.started = ctx.now()
	q.remaining = q.limit()
	return q
}

func (q *quizScreen) limit() time.Duration { return q.ex.TimeLimit(q.ctx.lessons) }

func (q *quizScreen) maxAttempts() int {
	if q.ctx.lessons {
		return 2
	}
	return 1
}

func (q *quizScreen) Init() tea.Cmd        { return tea.Batch(tick(q.ctx.id), textinput.Blink) }
func (q *quizScreen) SetSize(width, _ int) { q.width = width }
func (q *quizScreen) Close()               {}

func (q *quizScreen) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tickMsg:
		if msg.id != q.ctx.id || q.answered {
			return nil
		}
		q.remaining = q.limit() - q.ctx.now().Sub(q.started)
		if q.remaining <= 0 {
			q.remaining = 0
			q.finish(false, true)
			return nil
		}
		return tick(q.ctx.id)
	case tea.KeyMsg:
		k := msg.String()
		switch {
		case k == "esc":
			return send(backToLessonMsg{})
		case q.answered:
			if k == "enter" || k == " " {
				return send(nextExerciseMsg{})
			}
			return nil
		}
		return q.updateAnswering(msg)
	}
	// Otros mensajes, como el parpadeo del cursor, son para la entrada de texto.
	var cmd tea.Cmd
	q.input, cmd = q.input.Update(msg)
	return cmd
}

func (q *quizScreen) updateAnswering(msg tea.KeyMsg) tea.Cmd {
	k := msg.String()
	if k == "f1" {
		if q.ctx.lessons && q.hints < len(q.ex.Hints) {
			q.hints++
		}
		return nil
	}
	if q.ex.Format == content.FormatShortAnswer {
		if k == "enter" {
			if strings.TrimSpace(q.input.Value()) != "" {
				q.submit([]string{q.input.Value()})
			}
			return nil
		}
		var cmd tea.Cmd
		q.input, cmd = q.input.Update(msg)
		return cmd
	}

	multi := q.ex.Format == content.FormatMultiSelect
	n := len(q.choices)
	switch k {
	case "up", "k":
		q.cursor = (q.cursor - 1 + n) % n
	case "down", "j":
		q.cursor = (q.cursor + 1) % n
	case " ", "x":
		if multi {
			q.selected[q.cursor] = !q.selected[q.cursor]
		}
	case "enter":
		if !multi {
			q.submit([]string{q.choices[q.cursor]})
			break
		}
		var given []string
		for i, c := range q.choices {
			if q.selected[i] {
				given = append(given, c)
			}
		}
		if len(given) > 0 {
			q.submit(given)
		}
	default:
		if len(k) == 1 && k[0] >= '1' && int(k[0]-'1') < n {
			q.cursor = int(k[0] - '1')
			if multi {
				q.selected[q.cursor] = !q.selected[q.cursor]
			}
		}
	}
	return nil
}

func (q *quizScreen) submit(given []string) {
	ok, err := quiz.Check(q.ex, given)
	if err != nil {
		q.notice = fmt.Sprintf(strQuizErrorFmt, err)
		return
	}
	if ok {
		q.finish(true, false)
		return
	}
	q.failed++
	if q.failed >= q.maxAttempts() {
		q.finish(false, false)
		return
	}
	q.notice = fmt.Sprintf(strQuizTryAgainFmt, q.maxAttempts()-q.failed)
	q.input.Reset()
}

func (q *quizScreen) finish(correct, timedOut bool) {
	q.last = q.ctx.session.Record(q.ex, scoring.Outcome{
		Kind:           scoring.Theory,
		Difficulty:     q.ex.Difficulty,
		TimeUsed:       q.ctx.now().Sub(q.started),
		TimeLimit:      q.limit(),
		Solved:         correct,
		FailedAttempts: q.failed,
		MaxAttempts:    q.maxAttempts(),
		HintsUsed:      q.hints,
	})
	q.answered, q.timedOut = true, timedOut
	q.input.Blur()
}

func (q *quizScreen) View() string {
	w := textWidth(q.width)
	wrap := lipgloss.NewStyle().Width(w)
	var b strings.Builder

	b.WriteString(renderHUD(q.ctx, q.remaining, q.limit(), "") + "\n\n")
	b.WriteString(wrap.Render(reflow(q.ex.Question)) + "\n\n")

	if q.ex.Format == content.FormatShortAnswer {
		b.WriteString(q.input.View() + "\n")
	} else {
		for i, c := range q.choices {
			mark := ""
			if q.ex.Format == content.FormatMultiSelect {
				mark = "[ ] "
				if q.selected[i] {
					mark = "[x] "
				}
			}
			cursor := "  "
			style := lipgloss.NewStyle()
			if i == q.cursor && !q.answered {
				cursor, style = "> ", styleSelected
			}
			prefix := fmt.Sprintf("%s%d. %s", cursor, i+1, mark)
			b.WriteString(style.Render(hangingIndent(prefix, displayChoice(c), w)) + "\n")
		}
	}
	b.WriteString("\n")

	if q.answered {
		b.WriteString(q.viewFeedback(wrap, w))
		b.WriteString(styleHelp.Render(wrap.Render(strHelpNext)))
		return b.String()
	}
	for i := 0; i < q.hints; i++ {
		b.WriteString(styleHint.Render(wrap.Render(fmt.Sprintf(strHintFmt, i+1, q.ex.Hints[i]))) + "\n")
	}
	if q.notice != "" {
		b.WriteString(styleWrong.Render(q.notice) + "\n")
	}
	b.WriteString(styleHelp.Render(wrap.Render(q.helpLine())))
	return b.String()
}

func (q *quizScreen) viewFeedback(wrap lipgloss.Style, width int) string {
	var b strings.Builder
	solved := q.last.Outcome.Solved
	switch {
	case solved:
		b.WriteString(styleCorrect.Render(strCorrect) + "  " + styleScore.Render(pointsLabel(q.last)) + "\n")
	case q.timedOut:
		b.WriteString(styleWrong.Render(strTimeout) + "\n")
	default:
		b.WriteString(styleWrong.Render(strWrong) + "\n")
	}
	if !solved {
		b.WriteString(wrap.Render(fmt.Sprintf(strQuizAnswerFmt, displayAnswer(q.ex))) + "\n")
	}
	b.WriteString(wrap.Render(reflow(q.ex.Explanation)) + "\n")
	b.WriteString(styleSubtitle.Render(strSource) + "\n")
	b.WriteString(styleSubtitle.Render(hardWrap(q.ex.Source, width)) + "\n\n")
	return b.String()
}

func (q *quizScreen) helpLine() string {
	help := strQuizHelpChoice
	switch q.ex.Format {
	case content.FormatShortAnswer:
		help = strQuizHelpText
	case content.FormatMultiSelect:
		help = strQuizHelpMulti
	}
	if q.ctx.lessons && q.hints < len(q.ex.Hints) {
		help += strHelpHint
	}
	return help + strHelpBack
}

// hangingIndent ajusta text al ancho y alinea las líneas siguientes con el final de prefix.
func hangingIndent(prefix, text string, width int) string {
	pad := lipgloss.Width(prefix)
	body := lipgloss.NewStyle().Width(max(10, width-pad)).Render(text)
	return prefix + strings.ReplaceAll(body, "\n", "\n"+strings.Repeat(" ", pad))
}

// hardWrap corta s cada width runas. Para URLs, que no tienen espacios donde partir.
func hardWrap(s string, width int) string {
	r := []rune(s)
	var lines []string
	for len(r) > width {
		lines = append(lines, string(r[:width]))
		r = r[width:]
	}
	return strings.Join(append(lines, string(r)), "\n")
}

// reflow une las líneas de cada párrafo para que el ajuste al ancho de la terminal
// no deje cortes a medias. Una línea en blanco separa párrafos.
func reflow(text string) string {
	paras := strings.Split(strings.TrimSpace(text), "\n\n")
	for i, p := range paras {
		paras[i] = strings.Join(strings.Fields(p), " ")
	}
	return strings.Join(paras, "\n\n")
}

func displayAnswer(ex content.Exercise) string {
	labels := make([]string, len(ex.Answer))
	for i, a := range ex.Answer {
		labels[i] = displayChoice(a)
	}
	return strings.Join(labels, strQuizAnswerJoiner)
}

// displayChoice traduce los valores canónicos de true_false a su texto visible.
func displayChoice(c string) string {
	switch c {
	case content.True:
		return strQuizTrue
	case content.False:
		return strQuizFalse
	}
	return c
}
