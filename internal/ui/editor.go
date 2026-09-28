package ui

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"lsvp-pupil/internal/checks"
	"lsvp-pupil/internal/content"
	"lsvp-pupil/internal/sandbox"
	"lsvp-pupil/internal/scoring"
	"lsvp-pupil/internal/session"
)

type editorPhase int

const (
	editorPreparing editorPhase = iota
	editorEditing
	editorRunning // corriendo los tests o el editor externo
	editorDone
	editorBroken
)

type (
	scriptDoneMsg struct {
		id, gen   int
		results   []checks.TestResult
		noShebang bool
		err       error
	}
	externalEditorDoneMsg struct {
		id, gen int
		err     error
	}
)

// editorScreen es un ejercicio de script: se escribe en un editor integrado (o en nano/vim con F4)
// y F5 corre los tests en el sandbox.
type editorScreen struct {
	ctx           exerciseCtx
	ex            content.Exercise
	width, height int

	phase editorPhase
	gen   int
	sb    sandbox.Sandbox
	err   error

	ta        textarea.Model
	results   []checks.TestResult
	noShebang bool
	notice    string
	failed    int
	hints     int

	timerOn   bool
	started   time.Time
	remaining time.Duration

	solved   bool
	timedOut bool
	last     session.Result
}

func newEditor(ctx exerciseCtx, ex content.Exercise) *editorScreen {
	ta := textarea.New()
	ta.ShowLineNumbers = true
	ta.CharLimit = 0
	ta.Placeholder = strEditorPlaceholder
	ta.Prompt = ""
	e := &editorScreen{ctx: ctx, ex: ex, ta: ta}
	e.remaining = e.limit()
	return e
}

func (e *editorScreen) limit() time.Duration { return e.ex.TimeLimit(e.ctx.lessons) }

// scriptRel es la ruta del script relativa al home.
func (e *editorScreen) scriptRel() string { return path.Join(e.ex.StartDir, e.ex.Filename) }

func (e *editorScreen) Init() tea.Cmd {
	ex, id, gen, newSB := e.ex, e.ctx.id, e.gen, e.ctx.newSandbox
	return func() tea.Msg {
		sb, err := newSB(sandbox.Options{Fixture: ex.Fixture, StartDir: ex.StartDir, Setup: ex.Setup})
		return sandboxReadyMsg{id: id, gen: gen, sb: sb, err: err}
	}
}

func (e *editorScreen) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case sandboxReadyMsg:
		if msg.id != e.ctx.id || msg.gen != e.gen || e.phase == editorDone {
			if msg.sb != nil {
				_ = msg.sb.Close()
			}
			return nil
		}
		if msg.err != nil {
			e.phase, e.err = editorBroken, msg.err
			return nil
		}
		e.sb = msg.sb
		e.phase = editorEditing
		if data, err := os.ReadFile(e.sb.HostPath(e.scriptRel())); err == nil {
			e.ta.SetValue(string(data)) // el setup pudo dejar un borrador
		}
		e.ta.Focus()
		e.timerOn, e.started = true, e.ctx.now()
		e.layout()
		return tea.Batch(tick(e.ctx.id), textarea.Blink)
	case tickMsg:
		if msg.id != e.ctx.id || !e.timerOn || e.phase == editorDone {
			return nil
		}
		e.remaining = e.limit() - e.ctx.now().Sub(e.started)
		if e.remaining <= 0 {
			e.remaining, e.timedOut = 0, true
			e.finish(false)
			return nil
		}
		return tick(e.ctx.id)
	case scriptDoneMsg:
		return e.onTests(msg)
	case externalEditorDoneMsg:
		if msg.id != e.ctx.id || msg.gen != e.gen || e.phase != editorRunning {
			return nil
		}
		e.phase = editorEditing
		if msg.err != nil {
			e.notice = fmt.Sprintf(strPracticeRunErrFmt, msg.err)
		}
		if data, err := os.ReadFile(e.sb.HostPath(e.scriptRel())); err == nil {
			e.ta.SetValue(string(data))
		}
		e.ta.Focus()
		e.layout()
		return nil
	case tea.KeyMsg:
		return e.onKey(msg)
	}
	var cmd tea.Cmd
	e.ta, cmd = e.ta.Update(msg)
	return cmd
}

func (e *editorScreen) onKey(msg tea.KeyMsg) tea.Cmd {
	k := msg.String()
	if k == "esc" {
		return send(backToLessonMsg{})
	}
	switch e.phase {
	case editorDone, editorBroken:
		if k == "enter" {
			return send(nextExerciseMsg{})
		}
		return nil
	case editorEditing:
	default:
		return nil
	}
	switch k {
	case "f5":
		return e.runTests()
	case "f4":
		return e.openExternal()
	case "f1":
		if e.ctx.lessons && e.hints < len(e.ex.Hints) {
			e.hints++
			e.layout()
		}
		return nil
	case "tab":
		e.ta.InsertString("    ")
		return nil
	}
	var cmd tea.Cmd
	e.ta, cmd = e.ta.Update(msg)
	return cmd
}

func (e *editorScreen) save() error {
	return os.WriteFile(e.sb.HostPath(e.scriptRel()), []byte(e.ta.Value()), 0o644)
}

func (e *editorScreen) runTests() tea.Cmd {
	if err := e.save(); err != nil {
		e.notice = fmt.Sprintf(strPracticeRunErrFmt, err)
		return nil
	}
	e.phase = editorRunning
	e.notice = ""
	e.layout()
	ex, script, sb, id, gen := e.ex, e.ta.Value(), e.sb, e.ctx.id, e.gen
	return func() tea.Msg {
		if ex.RequireShebang && !checks.HasShebang(script) {
			return scriptDoneMsg{id: id, gen: gen, noShebang: true}
		}
		rs, err := checks.RunScript(ex, script, func(cmd string) (string, string, int, bool, error) {
			r, err := sb.Run(cmd)
			if err == nil && r.Blocked != nil {
				return "", guardMessage(*r.Blocked), 0, false, nil
			}
			return r.Stdout, r.Stderr, r.ExitCode, r.TimedOut, err
		}, func(rel string) (string, bool) {
			data, err := os.ReadFile(sb.HostPath(path.Join(ex.StartDir, rel)))
			return string(data), err == nil
		})
		return scriptDoneMsg{id: id, gen: gen, results: rs, err: err}
	}
}

func (e *editorScreen) onTests(msg scriptDoneMsg) tea.Cmd {
	if msg.id != e.ctx.id || msg.gen != e.gen || e.phase != editorRunning {
		return nil
	}
	e.phase = editorEditing
	defer e.layout()
	if msg.err != nil {
		e.notice = fmt.Sprintf(strPracticeRunErrFmt, msg.err)
		return nil
	}
	e.results, e.noShebang = msg.results, msg.noShebang
	if !msg.noShebang && checks.AllPassed(msg.results) {
		e.finish(true)
		return nil
	}
	e.failed++
	left := e.ex.MaxAttempts - e.failed
	switch {
	case left <= 0:
		e.finish(false)
	case left == 1:
		e.notice = strPracticeNotYetOne
	default:
		e.notice = fmt.Sprintf(strPracticeNotYetFmt, left)
	}
	return nil
}

// openExternal abre el script en nano o vim dentro del sandbox, con la terminal real.
func (e *editorScreen) openExternal() tea.Cmd {
	if err := e.save(); err != nil {
		e.notice = fmt.Sprintf(strPracticeRunErrFmt, err)
		return nil
	}
	cmd, blk, err := e.sb.Interactive(externalEditor()+" "+checks.ShellQuote(e.ex.Filename), os.Getenv("TERM"))
	if err == nil && blk != nil {
		err = errors.New(guardMessage(*blk))
	}
	if err != nil {
		e.notice = fmt.Sprintf(strPracticeRunErrFmt, err)
		return nil
	}
	e.phase = editorRunning
	id, gen := e.ctx.id, e.gen
	return tea.ExecProcess(cmd, func(err error) tea.Msg { return externalEditorDoneMsg{id: id, gen: gen, err: err} })
}

// externalEditor usa $LSVP_EDITOR o $EDITOR si es nano, vim o vi (lo que hay en el sandbox); si no, nano.
func externalEditor() string {
	for _, v := range []string{os.Getenv("LSVP_EDITOR"), os.Getenv("EDITOR")} {
		switch filepath.Base(v) {
		case "nano", "vim", "vi":
			return filepath.Base(v)
		}
	}
	return "nano"
}

func (e *editorScreen) finish(solved bool) {
	e.last = e.ctx.session.Record(e.ex, scoring.Outcome{
		Kind:           scoring.Practice,
		Difficulty:     e.ex.Difficulty,
		TimeUsed:       e.ctx.now().Sub(e.started),
		TimeLimit:      e.limit(),
		Solved:         solved,
		FailedAttempts: e.failed,
		MaxAttempts:    e.ex.MaxAttempts,
		HintsUsed:      e.hints,
		Command:        e.ta.Value(),
		ParChars:       e.ex.ParChars,
	})
	e.solved = solved
	e.phase = editorDone
	e.notice = ""
	e.ta.Blur()
	e.layout()
}

func (e *editorScreen) Close() {
	if e.sb != nil {
		_ = e.sb.Close()
		e.sb = nil
	}
}

func (e *editorScreen) SetSize(width, height int) {
	e.width, e.height = width, height
	e.layout()
}

func (e *editorScreen) top() string {
	w := textWidth(e.width)
	wrap := lipgloss.NewStyle().Width(w)
	extra := styleSubtitle.Render(fmt.Sprintf(strPracticeAttemptsFmt, max(0, e.ex.MaxAttempts-e.failed)))
	var b strings.Builder
	b.WriteString(renderHUD(e.ctx, e.remaining, e.limit(), extra) + "\n\n")
	b.WriteString(wrap.Render(reflow(e.ex.Instructions)) + "\n")
	file := fmt.Sprintf(strEditorFileFmt, "~/"+e.scriptRel())
	if e.sb != nil && !e.sb.Isolated() {
		b.WriteString(styleSubtitle.Render(file) + "\n" + styleStatus.Render(wrap.Render(strPracticeDirWarning)) + "\n")
	} else {
		b.WriteString(styleSubtitle.Render(file) + "\n")
	}
	return b.String()
}

func (e *editorScreen) bottom() string {
	w := textWidth(e.width)
	wrap := lipgloss.NewStyle().Width(w)
	var b strings.Builder
	switch e.phase {
	case editorPreparing:
		b.WriteString(styleSubtitle.Render(strPracticePreparing) + "\n")
		return b.String()
	case editorBroken:
		b.WriteString(styleWrong.Render(wrap.Render(fmt.Sprintf(strPracticeSetupErrFmt, e.err))) + "\n")
		b.WriteString(styleHelp.Render(wrap.Render(strPracticeSetupHelp)))
		return b.String()
	case editorRunning:
		b.WriteString(styleSubtitle.Render(strEditorRunning) + "\n")
		return b.String()
	case editorDone:
		switch {
		case e.solved:
			b.WriteString(styleCorrect.Render(strPracticeSolved) + "  " + styleScore.Render(pointsLabel(e.last)) + "\n")
		case e.timedOut:
			b.WriteString(styleWrong.Render(strTimeout) + "\n")
		default:
			b.WriteString(styleWrong.Render(strPracticeOutOfTries) + "\n")
		}
		if e.ex.Explanation != "" {
			b.WriteString(wrap.Render(reflow(e.ex.Explanation)) + "\n")
		}
		b.WriteString(styleSubtitle.Render(strSource) + "\n")
		b.WriteString(styleSubtitle.Render(hardWrap(e.ex.Source, w)) + "\n")
		b.WriteString(styleHelp.Render(wrap.Render(strHelpNext)))
		return b.String()
	}
	b.WriteString(e.viewResults(w))
	for i := 0; i < e.hints; i++ {
		b.WriteString(styleHint.Render(wrap.Render(fmt.Sprintf(strHintFmt, i+1, e.ex.Hints[i]))) + "\n")
	}
	if e.notice != "" {
		b.WriteString(styleWrong.Render(wrap.Render(e.notice)) + "\n")
	}
	help := fmt.Sprintf(strEditorHelpFmt, externalEditor())
	if e.ctx.lessons && e.hints < len(e.ex.Hints) {
		help += strHelpHint
	}
	b.WriteString(styleHelp.Render(wrap.Render(help + strHelpBack)))
	return b.String()
}

// viewResults resume el último intento: una línea por test y, en los que fallaron, su salida.
func (e *editorScreen) viewResults(w int) string {
	if e.noShebang {
		return styleWrong.Render(lipgloss.NewStyle().Width(w).Render(strEditorNoShebang)) + "\n"
	}
	var b strings.Builder
	for i, r := range e.results {
		label := fmt.Sprintf(strEditorTestFmt, i+1, testLabel(r.Test))
		if r.Passed() {
			b.WriteString(styleCorrect.Render("✔ "+truncate(label, w-2)) + "\n")
			continue
		}
		b.WriteString(styleWrong.Render("✘ "+truncate(label, w-2)) + "\n")
		b.WriteString(styleSubtitle.Render(truncate("  "+failureText(r), w)) + "\n")
		got := strings.TrimSpace(r.Stdout + r.Stderr)
		if got == "" {
			got = strEditorNoOutput
		}
		b.WriteString(styleSubtitle.Render(truncate(fmt.Sprintf(strEditorGotFmt, strings.ReplaceAll(got, "\n", " ⏎ ")), w)) + "\n")
	}
	return b.String()
}

// failureText explica al alumno por qué falló un test, sin mostrarle regex.
func failureText(r checks.TestResult) string {
	switch {
	case r.TimedOut:
		return strEditorTimedOut
	case r.Test.StdoutContains != "" && !strings.Contains(r.Stdout, r.Test.StdoutContains):
		return fmt.Sprintf(strEditorExpectedFmt, r.Test.StdoutContains)
	case r.Test.StdoutRegex != "" && !regexp.MustCompile(r.Test.StdoutRegex).MatchString(r.Stdout):
		return strEditorBadFormat
	case r.Test.ExitCode != nil && r.ExitCode != *r.Test.ExitCode:
		return fmt.Sprintf(strEditorExitFmt, r.ExitCode, *r.Test.ExitCode)
	}
	for _, p := range sortedFiles(r.Test.Files) {
		got, ok := r.Files[p]
		switch {
		case !ok:
			return fmt.Sprintf(strEditorFileMissingFmt, p)
		case got != r.Test.Files[p]:
			return fmt.Sprintf(strEditorFileWrongFmt, p, strings.ReplaceAll(strings.TrimRight(got, "\n"), "\n", " ⏎ "))
		}
	}
	return strEditorBadFormat
}

func sortedFiles(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func testLabel(t content.ScriptTest) string {
	var parts []string
	if len(t.Args) > 0 {
		parts = append(parts, fmt.Sprintf(strEditorArgsFmt, strings.Join(t.Args, " ")))
	}
	if t.Stdin != "" {
		parts = append(parts, fmt.Sprintf(strEditorStdinFmt, strings.ReplaceAll(strings.TrimRight(t.Stdin, "\n"), "\n", " ⏎ ")))
	}
	if len(parts) == 0 {
		return strEditorNoArgs
	}
	return strings.Join(parts, " · ")
}

func truncate(s string, w int) string {
	r := []rune(s)
	if len(r) <= w || w < 2 {
		return s
	}
	return string(r[:w-1]) + "…"
}

func (e *editorScreen) layout() {
	w := textWidth(e.width)
	h := e.height - 2
	if e.height == 0 {
		h = minHeight - 2
	}
	used := lipgloss.Height(e.top()) + lipgloss.Height(e.bottom()) + 1
	e.ta.SetWidth(w)
	e.ta.SetHeight(max(3, h-used))
}

func (e *editorScreen) View() string {
	body := e.ta.View()
	if e.phase == editorDone {
		body = styleSubtitle.Render(strEditorSolution) + "\n" + e.ex.SolutionScript
		lines := strings.Split(body, "\n")
		if room := e.ta.Height() + 1; len(lines) > room {
			body = strings.Join(append(lines[:room-1], "…"), "\n")
		}
	}
	return e.top() + "\n" + body + "\n" + e.bottom()
}
