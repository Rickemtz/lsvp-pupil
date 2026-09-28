package ui

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"lsvp-pupil/internal/checks"
	"lsvp-pupil/internal/content"
	"lsvp-pupil/internal/sandbox"
	"lsvp-pupil/internal/scoring"
	"lsvp-pupil/internal/session"
)

type practicePhase int

const (
	practicePreparing practicePhase = iota
	practiceReady
	practiceRunning
	practiceDone
	practiceBroken // no se pudo crear el sandbox
)

// Mensajes de trabajo en segundo plano. gen cambia con cada reinicio del sandbox (F2).
type (
	sandboxReadyMsg struct {
		id, gen   int
		sb        sandbox.Sandbox
		solStdout string
		solHome   string
		err       error
	}
	commandDoneMsg struct {
		id, gen     int
		line        string
		res         sandbox.Result
		procs       []checks.Process
		interactive bool // la línea corrió un programa de pantalla completa (less, nano, htop...)
		err         error
	}
)

type termKind int

const (
	termPrompt termKind = iota
	termStdout
	termStderr
	termInfo
)

type termLine struct {
	kind termKind
	text string
}

// practiceScreen es un ejercicio práctico: una terminal real dentro de un sandbox.
type practiceScreen struct {
	ctx           exerciseCtx
	ex            content.Exercise
	width, height int

	phase     practicePhase
	gen       int
	sb        sandbox.Sandbox
	home, cwd string // copias locales: consultar al sandbox bloquearía mientras corre un comando
	solStdout string
	solHome   string
	err       error

	input   textinput.Model
	history []string
	histPos int // len(history) = línea nueva
	lines   []termLine
	vp      viewport.Model

	failed   int
	hints    int
	guardHit bool
	notice   string

	timerOn   bool
	started   time.Time
	remaining time.Duration

	solved   bool
	timedOut bool
	last     session.Result
}

func newPractice(ctx exerciseCtx, ex content.Exercise) *practiceScreen {
	in := textinput.New()
	in.Prompt = "" // el prompt de la shell se dibuja aparte
	in.Placeholder = strPracticeInputHolder
	in.CharLimit = 500
	p := &practiceScreen{ctx: ctx, ex: ex, input: in, vp: viewport.New(0, 0)}
	p.remaining = p.limit()
	return p
}

func (p *practiceScreen) limit() time.Duration { return p.ex.TimeLimit(p.ctx.lessons) }

func (p *practiceScreen) Init() tea.Cmd { return p.prepare() }

// prepare crea el sandbox en segundo plano. Si algún check compara con la solución, antes corre la
// solución en otro sandbox limpio para guardar su salida.
func (p *practiceScreen) prepare() tea.Cmd {
	p.phase = practicePreparing
	ex, id, gen, newSB := p.ex, p.ctx.id, p.gen, p.ctx.newSandbox
	if ex.IsPatternOnly() { // no se ejecuta nada: no hace falta sandbox
		return send(sandboxReadyMsg{id: id, gen: gen})
	}
	return func() tea.Msg {
		opts := sandbox.Options{Fixture: ex.Fixture, StartDir: ex.StartDir, Setup: ex.Setup}
		msg := sandboxReadyMsg{id: id, gen: gen}
		if ex.NeedsSolutionOutput() {
			ref, err := newSB(opts)
			if err != nil {
				msg.err = err
				return msg
			}
			r, err := ref.Run(ex.Solution)
			msg.solStdout, msg.solHome = r.Stdout, ref.Home()
			_ = ref.Close()
			if err != nil {
				msg.err = fmt.Errorf("ejecutar la solución de referencia: %w", err)
				return msg
			}
		}
		msg.sb, msg.err = newSB(opts)
		return msg
	}
}

func (p *practiceScreen) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case sandboxReadyMsg:
		return p.onReady(msg)
	case commandDoneMsg:
		return p.onCommandDone(msg)
	case tickMsg:
		if msg.id != p.ctx.id || !p.timerOn || p.phase == practiceDone {
			return nil
		}
		p.remaining = p.limit() - p.ctx.now().Sub(p.started)
		if p.remaining <= 0 {
			p.remaining = 0
			p.timedOut = true
			p.finish(false)
			return nil
		}
		return tick(p.ctx.id)
	case tea.KeyMsg:
		return p.onKey(msg)
	}
	var cmd tea.Cmd
	p.input, cmd = p.input.Update(msg)
	return cmd
}

func (p *practiceScreen) onReady(msg sandboxReadyMsg) tea.Cmd {
	if msg.id != p.ctx.id || msg.gen != p.gen || p.phase == practiceDone {
		if msg.sb != nil {
			_ = msg.sb.Close() // llegó tarde: el ejercicio ya cambió o se reinició otra vez
		}
		return nil
	}
	if msg.err != nil {
		p.phase, p.err = practiceBroken, msg.err
		return nil
	}
	p.sb, p.solStdout, p.solHome = msg.sb, msg.solStdout, msg.solHome
	if msg.sb != nil {
		p.home, p.cwd = msg.sb.Home(), msg.sb.Cwd()
	}
	p.phase = practiceReady
	p.input.Focus()
	cmds := []tea.Cmd{textinput.Blink}
	if !p.timerOn { // el tiempo corre desde el primer sandbox; F2 no lo reinicia
		p.timerOn, p.started = true, p.ctx.now()
		cmds = append(cmds, tick(p.ctx.id))
	}
	p.layout()
	return tea.Batch(cmds...)
}

func (p *practiceScreen) onKey(msg tea.KeyMsg) tea.Cmd {
	k := msg.String()
	if k == "esc" {
		return send(backToLessonMsg{})
	}
	switch k {
	case "pgup", "pgdown":
		var cmd tea.Cmd
		p.vp, cmd = p.vp.Update(msg)
		return cmd
	}
	switch p.phase {
	case practiceDone:
		if k == "enter" {
			return send(nextExerciseMsg{})
		}
		return nil
	case practiceBroken:
		if k == "enter" {
			return send(nextExerciseMsg{})
		}
		return nil
	case practiceRunning:
		if k == "f2" && p.sb != nil { // reiniciar también sirve para salir de un comando que se quedó pegado
			return p.reset()
		}
		return nil
	case practiceReady:
	default:
		return nil // preparando
	}

	switch k {
	case "enter":
		return p.submit()
	case "up":
		if p.histPos > 0 {
			p.histPos--
			p.input.SetValue(p.history[p.histPos])
			p.input.CursorEnd()
		}
		return nil
	case "down":
		if p.histPos < len(p.history) {
			p.histPos++
			if p.histPos == len(p.history) {
				p.input.SetValue("")
			} else {
				p.input.SetValue(p.history[p.histPos])
			}
			p.input.CursorEnd()
		}
		return nil
	case "ctrl+l":
		p.lines = nil
		p.layout()
		return nil
	case "f1":
		if p.ctx.lessons && p.hints < len(p.ex.Hints) {
			p.hints++
			p.layout()
		}
		return nil
	case "f2":
		if p.sb == nil {
			return nil
		}
		return p.reset()
	case "f3":
		if p.sb == nil {
			return nil
		}
		for _, l := range strings.Split(sandbox.Tree(p.sb), "\n") {
			p.lines = append(p.lines, termLine{termInfo, l})
		}
		p.layout()
		return nil
	}
	var cmd tea.Cmd
	p.input, cmd = p.input.Update(msg)
	return cmd
}

func (p *practiceScreen) submit() tea.Cmd {
	line := p.input.Value()
	p.input.SetValue("")
	p.lines = append(p.lines, termLine{termPrompt, p.prompt() + line})
	if strings.TrimSpace(line) == "" {
		p.layout()
		return nil
	}
	if len(p.history) == 0 || p.history[len(p.history)-1] != line {
		p.history = append(p.history, line)
	}
	p.histPos = len(p.history)
	if strings.TrimSpace(line) == "clear" { // como en el curso; con TERM=dumb clear no haría nada
		p.lines = nil
		p.layout()
		return nil
	}
	if p.ex.IsPatternOnly() {
		p.checkPattern(line)
		return nil
	}
	if sandbox.IsInteractive(line) {
		return p.runInteractive(line)
	}
	p.phase = practiceRunning
	p.notice = ""
	p.layout()
	sb, id, gen, needProcs := p.sb, p.ctx.id, p.gen, p.ex.NeedsProcesses()
	return func() tea.Msg {
		res, err := sb.Run(line)
		msg := commandDoneMsg{id: id, gen: gen, line: line, res: res, err: err}
		if needProcs && err == nil {
			msg.procs = processes(sb)
		}
		return msg
	}
}

// runInteractive suspende la TUI y corre la línea con la terminal real. El tiempo sigue corriendo.
func (p *practiceScreen) runInteractive(line string) tea.Cmd {
	cmd, blk, err := p.sb.Interactive(line, os.Getenv("TERM"))
	id, gen, sb, needProcs := p.ctx.id, p.gen, p.sb, p.ex.NeedsProcesses()
	if err != nil || blk != nil {
		return send(commandDoneMsg{id: id, gen: gen, line: line, res: sandbox.Result{Cwd: p.cwd, Blocked: blk}, err: err})
	}
	p.phase = practiceRunning
	p.notice = ""
	p.layout()
	cwd := p.cwd
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		res := sandbox.Result{Cwd: cwd}
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			res.ExitCode = exit.ExitCode()
		} else if err != nil {
			return commandDoneMsg{id: id, gen: gen, line: line, res: res, err: err}
		}
		msg := commandDoneMsg{id: id, gen: gen, line: line, res: res, interactive: true}
		if needProcs {
			msg.procs = processes(sb)
		}
		return msg
	})
}

func processes(sb sandbox.Sandbox) []checks.Process {
	infos, err := sb.Processes()
	if err != nil {
		return nil
	}
	ps := make([]checks.Process, len(infos))
	for i, in := range infos {
		ps[i] = checks.Process{Name: in.Name, Running: in.Running, Signal: in.Signal}
	}
	return ps
}

func (p *practiceScreen) onCommandDone(msg commandDoneMsg) tea.Cmd {
	if msg.id != p.ctx.id || msg.gen != p.gen || p.phase != practiceRunning {
		return nil
	}
	p.phase = practiceReady
	defer p.layout()
	if msg.err != nil {
		p.lines = append(p.lines, termLine{termStderr, fmt.Sprintf(strPracticeRunErrFmt, msg.err)})
		return nil
	}
	r := msg.res
	p.cwd = r.Cwd
	if r.Blocked != nil {
		text := guardMessage(*r.Blocked)
		if !p.guardHit {
			text += strGuardPenaltyNote
		}
		p.guardHit = true
		p.lines = append(p.lines, termLine{termInfo, text})
		return nil // no se ejecutó: no cuenta como intento
	}
	p.appendOutput(termStdout, r.Stdout)
	p.appendOutput(termStderr, r.Stderr)
	if msg.interactive {
		p.lines = append(p.lines, termLine{termInfo, strInteractiveDone})
	}
	switch {
	case r.Truncated:
		p.lines = append(p.lines, termLine{termInfo, strPracticeTruncated})
	}
	switch {
	case r.TimedOut:
		p.lines = append(p.lines, termLine{termInfo, fmt.Sprintf(strPracticeTimedOutFmt, sandbox.DefaultTimeout)})
	case r.Restarted:
		p.lines = append(p.lines, termLine{termInfo, strPracticeRestarted})
	}

	st := checks.State{
		Command: msg.line, Stdout: r.Stdout, ExitCode: r.ExitCode, Cwd: r.Cwd, Home: p.home, HostPath: p.sb.HostPath,
		Processes:      msg.procs,
		SolutionStdout: p.solStdout, SolutionHome: p.solHome,
	}
	if len(checks.Run(p.ex.Checks, st)) == 0 {
		p.finishWith(msg.line, true)
		return nil
	}
	if scoring.IsExploration(msg.line, p.ex.Solution) {
		return nil
	}
	p.failed++
	left := p.ex.MaxAttempts - p.failed
	switch {
	case left <= 0:
		p.finishWith(msg.line, false)
	case left == 1:
		p.notice = strPracticeNotYetOne
	default:
		p.notice = fmt.Sprintf(strPracticeNotYetFmt, left)
	}
	return nil
}

// checkPattern evalúa una línea de un ejercicio pattern: se compara con las respuestas, no se ejecuta.
func (p *practiceScreen) checkPattern(line string) {
	defer p.layout()
	p.notice = ""
	if len(checks.Run(p.ex.Checks, checks.State{Command: line})) == 0 {
		p.finishWith(line, true)
		return
	}
	if scoring.IsExploration(line, p.ex.Solution) {
		p.lines = append(p.lines, termLine{termInfo, strPatternNotRun})
		return
	}
	p.failed++
	left := p.ex.MaxAttempts - p.failed
	switch {
	case left <= 0:
		p.finishWith(line, false)
	case left == 1:
		p.notice = strPracticeNotYetOne
	default:
		p.notice = fmt.Sprintf(strPracticeNotYetFmt, left)
	}
}

func (p *practiceScreen) appendOutput(kind termKind, out string) {
	if out == "" {
		return
	}
	for _, l := range strings.Split(strings.TrimSuffix(out, "\n"), "\n") {
		p.lines = append(p.lines, termLine{kind, l})
	}
}

func (p *practiceScreen) reset() tea.Cmd {
	if p.sb != nil {
		_ = p.sb.Close()
		p.sb = nil
	}
	p.gen++
	p.lines = append(p.lines, termLine{termInfo, strPracticeReset})
	p.input.Blur()
	return p.prepare()
}

func (p *practiceScreen) finish(solved bool) { p.finishWith("", solved) }

func (p *practiceScreen) finishWith(command string, solved bool) {
	p.last = p.ctx.session.Record(p.ex, scoring.Outcome{
		Kind:           scoring.Practice,
		Difficulty:     p.ex.Difficulty,
		TimeUsed:       p.ctx.now().Sub(p.started),
		TimeLimit:      p.limit(),
		Solved:         solved,
		FailedAttempts: p.failed,
		MaxAttempts:    p.ex.MaxAttempts,
		HintsUsed:      p.hints,
		Command:        command,
		ParChars:       p.ex.ParChars,
		GuardPenalty:   p.guardHit,
	})
	p.solved = solved
	p.phase = practiceDone
	p.notice = ""
	p.input.Blur()
	p.layout()
}

func (p *practiceScreen) Close() {
	if p.sb != nil {
		_ = p.sb.Close()
		p.sb = nil
	}
}

func (p *practiceScreen) SetSize(width, height int) {
	p.width, p.height = width, height
	p.layout()
}

func (p *practiceScreen) prompt() string {
	cwd := "~"
	if p.ex.StartDir != "" {
		cwd += "/" + p.ex.StartDir
	}
	switch {
	case p.cwd == "":
	case p.cwd == p.home:
		cwd = "~"
	case strings.HasPrefix(p.cwd, p.home+"/"):
		cwd = "~" + strings.TrimPrefix(p.cwd, p.home)
	default:
		cwd = p.cwd
	}
	return fmt.Sprintf("%s@%s:%s$ ", sandbox.User, sandbox.Hostname, cwd)
}

// Partes fijas de la pantalla, que la terminal rodea.
func (p *practiceScreen) top() string {
	w := textWidth(p.width)
	wrap := lipgloss.NewStyle().Width(w)
	extra := styleSubtitle.Render(fmt.Sprintf(strPracticeAttemptsFmt, max(0, p.ex.MaxAttempts-p.failed)))
	var b strings.Builder
	b.WriteString(renderHUD(p.ctx, p.remaining, p.limit(), extra) + "\n\n")
	b.WriteString(wrap.Render(reflow(p.ex.Instructions)) + "\n")
	if p.ex.IsPatternOnly() {
		b.WriteString(styleSubtitle.Render(wrap.Render(strPatternNote)) + "\n")
	} else if p.sb != nil && !p.sb.Isolated() {
		b.WriteString(styleStatus.Render(wrap.Render(strPracticeDirWarning)) + "\n")
	} else if p.sb != nil {
		b.WriteString(styleSubtitle.Render(fmt.Sprintf(strPracticeBackendFmt, p.sb.Backend())) + "\n")
	}
	return b.String()
}

func (p *practiceScreen) bottom() string {
	w := textWidth(p.width)
	wrap := lipgloss.NewStyle().Width(w)
	var b strings.Builder
	switch p.phase {
	case practicePreparing:
		b.WriteString(styleSubtitle.Render(strPracticePreparing) + "\n")
	case practiceBroken:
		b.WriteString(styleWrong.Render(wrap.Render(fmt.Sprintf(strPracticeSetupErrFmt, p.err))) + "\n")
		b.WriteString(styleHelp.Render(wrap.Render(strPracticeSetupHelp)))
		return b.String()
	case practiceRunning:
		b.WriteString(styleSubtitle.Render(p.prompt()+strPracticeRunning) + "\n")
	case practiceReady:
		sep := ""
		if p.promptOwnLine() {
			sep = "\n"
		}
		b.WriteString(styleSelected.Render(p.prompt()) + sep + p.input.View() + "\n")
	case practiceDone:
		switch {
		case p.solved:
			b.WriteString(styleCorrect.Render(strPracticeSolved) + "  " + styleScore.Render(pointsLabel(p.last)) + "\n")
		case p.timedOut:
			b.WriteString(styleWrong.Render(strTimeout) + "\n")
		default:
			b.WriteString(styleWrong.Render(strPracticeOutOfTries) + "\n")
		}
		b.WriteString(wrap.Render(fmt.Sprintf(strPracticeSolutionFmt, p.ex.Solution)) + "\n")
		if p.ex.Explanation != "" {
			b.WriteString(wrap.Render(reflow(p.ex.Explanation)) + "\n")
		}
		b.WriteString(styleSubtitle.Render(strSource) + "\n")
		b.WriteString(styleSubtitle.Render(hardWrap(p.ex.Source, w)) + "\n")
		b.WriteString(styleHelp.Render(wrap.Render(strHelpNext)))
		return b.String()
	}
	for i := 0; i < p.hints; i++ {
		b.WriteString(styleHint.Render(wrap.Render(fmt.Sprintf(strHintFmt, i+1, p.ex.Hints[i]))) + "\n")
	}
	if p.notice != "" {
		b.WriteString(styleWrong.Render(p.notice) + "\n")
	}
	help := strPracticeHelp
	if p.ex.IsPatternOnly() {
		help = strPatternHelp
	}
	if p.ctx.lessons && p.hints < len(p.ex.Hints) {
		help += strHelpHint
	}
	b.WriteString(styleHelp.Render(wrap.Render(help + strHelpBack)))
	return b.String()
}

// minInputWidth es el espacio mínimo para escribir junto al prompt; con menos, la entrada
// va en su propia línea (pasa con directorios profundos como taller2/dia3/...).
const minInputWidth = 25

func (p *practiceScreen) promptOwnLine() bool {
	return textWidth(p.width)-lipgloss.Width(p.prompt()) < minInputWidth
}

// layout recalcula el tamaño de la terminal y su contenido.
func (p *practiceScreen) layout() {
	w := textWidth(p.width)
	// La entrada se desplaza horizontalmente en vez de salirse de la pantalla.
	if p.promptOwnLine() {
		p.input.Width = w - 1
	} else {
		p.input.Width = w - lipgloss.Width(p.prompt()) - 1
	}
	h := p.height - 2 // styleFrame agrega una línea arriba y otra abajo
	if p.height == 0 {
		h = minHeight - 2
	}
	used := lipgloss.Height(p.top()) + lipgloss.Height(p.bottom()) + 1
	p.vp.Width = w
	p.vp.Height = max(3, h-used)

	wrap := lipgloss.NewStyle().Width(w)
	rendered := make([]string, len(p.lines))
	for i, l := range p.lines {
		switch l.kind {
		case termPrompt:
			rendered[i] = styleSelected.Render(wrap.Render(l.text))
		case termStderr:
			rendered[i] = styleWrong.UnsetBold().Render(wrap.Render(l.text))
		case termInfo:
			rendered[i] = styleStatus.Render(wrap.Render(l.text))
		default:
			rendered[i] = hardWrap(l.text, w)
		}
	}
	p.vp.SetContent(strings.Join(rendered, "\n"))
	p.vp.GotoBottom()
}

func (p *practiceScreen) View() string {
	return p.top() + "\n" + p.vp.View() + "\n" + p.bottom()
}

func guardMessage(b sandbox.Block) string {
	switch b.Rule {
	case sandbox.RulePrivilege:
		return fmt.Sprintf(strGuardPrivilege, b.Detail)
	case sandbox.RulePower:
		return fmt.Sprintf(strGuardPower, b.Detail)
	case sandbox.RuleMkfs:
		return fmt.Sprintf(strGuardMkfs, b.Detail)
	case sandbox.RuleDeviceWrite:
		return fmt.Sprintf(strGuardDeviceWrite, b.Detail)
	case sandbox.RuleForkBomb:
		return strGuardForkBomb
	case sandbox.RuleOutsidePath:
		return fmt.Sprintf(strGuardOutsidePath, b.Detail)
	case sandbox.RuleHomeWipe:
		return fmt.Sprintf(strGuardHomeWipe, b.Detail)
	case sandbox.RuleForeignProcess:
		return fmt.Sprintf(strGuardForeignProcess, b.Detail)
	}
	return fmt.Sprintf(strGuardUnverifiable, b.Detail)
}
