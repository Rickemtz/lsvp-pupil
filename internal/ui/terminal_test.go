package ui

import (
	"os"
	"os/exec"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"lsvp-pupil/internal/content"
	"lsvp-pupil/internal/sandbox"
)

var uiFixtures = fstest.MapFS{
	"taller/archivo.txt":     {Data: []byte("hola\n")},
	"taller/dia-1/Dewey.txt": {Data: []byte("Dewey\n")},
}

var exMove = content.Exercise{
	ID: "mv", Kind: content.KindPractice, Difficulty: 1, TimeLimitSec: 60, MaxAttempts: 2,
	Fixture: "taller", StartDir: "taller/dir1", Setup: []string{"mkdir -p ../dir3 && touch file.txt"},
	Instructions: "Mueve file.txt a dir3.",
	Checks: []content.Check{{Type: content.CheckFS, Exists: []string{"taller/dir3/file.txt"},
		Absent: []string{"taller/dir1/file.txt"}}},
	Solution: "mv file.txt ../dir3", ParChars: 19, Hints: []string{"mv origen destino"},
}

func newTestPractice(t *testing.T, ex content.Exercise) (*practiceScreen, *fakeClock) {
	t.Helper()
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash no está instalado")
	}
	ctx, clk := testCtx(true)
	ctx.newSandbox = func(o sandbox.Options) (sandbox.Sandbox, error) {
		o.Fixtures = uiFixtures
		return sandbox.New(sandbox.BackendDir, o)
	}
	p := newPractice(ctx, ex)
	p.SetSize(80, 30)
	p.Update(p.Init()()) // sandboxReadyMsg; el tick y el parpadeo no se ejecutan en los tests
	if p.phase != practiceReady {
		t.Fatalf("fase %v, error %v", p.phase, p.err)
	}
	t.Cleanup(p.Close)
	return p, clk
}

// typeLine escribe una línea, la ejecuta y entrega el resultado a la pantalla.
func typeLine(t *testing.T, p *practiceScreen, line string) {
	t.Helper()
	p.input.SetValue(line)
	cmd := p.Update(key("enter"))
	if cmd == nil {
		return // línea vacía o clear
	}
	msg, ok := cmd().(commandDoneMsg)
	if !ok {
		t.Fatalf("esperaba commandDoneMsg")
	}
	p.Update(msg)
}

func TestPracticeSolve(t *testing.T) {
	p, clk := newTestPractice(t, exMove)
	view := p.View()
	for _, want := range []string{"alumno@lsvp-pupil:~/taller/dir1$", "Mueve file.txt a dir3.", "sin aislamiento", "Intentos: 2"} {
		if !strings.Contains(view, want) {
			t.Errorf("la vista no contiene %q:\n%s", want, view)
		}
	}
	typeLine(t, p, "ls")
	if p.failed != 0 || p.phase != practiceReady {
		t.Fatal("ls es exploración: no cuenta como intento")
	}
	if !strings.Contains(p.View(), "file.txt") {
		t.Error("la salida de ls debería verse en la terminal")
	}
	clk.t = clk.t.Add(30 * time.Second)
	typeLine(t, p, "mv file.txt ../dir3")
	if p.phase != practiceDone || !p.solved {
		t.Fatalf("fase %v, resuelto %v", p.phase, p.solved)
	}
	// base 100 + tiempo 25 + precisión 50 + elegancia 10
	if o := p.last.Outcome; p.last.Points != 185 || o.Command != "mv file.txt ../dir3" || o.TimeUsed != 30*time.Second {
		t.Errorf("puntos %d, outcome %+v", p.last.Points, o)
	}
	if cmd := p.Update(key("enter")); cmd == nil || cmd() != (nextExerciseMsg{}) {
		t.Error("enter tras resolver debe pedir el siguiente ejercicio")
	}
}

func TestPracticeFailedAttempts(t *testing.T) {
	p, _ := newTestPractice(t, exMove)
	typeLine(t, p, "cp file.txt ../dir3/otro.txt")
	if p.failed != 1 || p.notice == "" {
		t.Fatalf("failed = %d, notice %q", p.failed, p.notice)
	}
	typeLine(t, p, "touch nada")
	if p.phase != practiceDone || p.solved || p.last.Outcome.FailedAttempts != 2 {
		t.Fatalf("con 2 fallos se acaban los intentos: fase %v %+v", p.phase, p.last.Outcome)
	}
	if !strings.Contains(p.View(), "mv file.txt ../dir3") {
		t.Error("al perder se muestra la solución")
	}
}

func TestPracticeGuard(t *testing.T) {
	p, _ := newTestPractice(t, exMove)
	typeLine(t, p, "sudo mv file.txt ../dir3")
	if p.failed != 0 || !p.guardHit || !strings.Contains(p.View(), "administrador") {
		t.Fatalf("guard: failed %d, guardHit %v", p.failed, p.guardHit)
	}
	typeLine(t, p, "mv file.txt ../dir3")
	if !p.solved || !p.last.Outcome.GuardPenalty {
		t.Errorf("la penalización de guard debe llegar al puntaje: %+v", p.last.Outcome)
	}
}

func TestPracticeSameAsSolution(t *testing.T) {
	ex := content.Exercise{
		ID: "cat", Kind: content.KindPractice, Difficulty: 1, TimeLimitSec: 60, MaxAttempts: 3,
		Fixture: "taller", StartDir: "taller", Instructions: "Muestra archivo.txt.",
		Checks:   []content.Check{{Type: content.CheckOutput, Mode: content.OutputSameAsSolution}},
		Solution: "cat archivo.txt",
	}
	p, _ := newTestPractice(t, ex)
	if p.solStdout != "hola\n" {
		t.Fatalf("salida de la solución = %q", p.solStdout)
	}
	typeLine(t, p, "pwd")
	if p.failed != 0 {
		t.Error("pwd es exploración")
	}
	typeLine(t, p, "cat dia-1/Dewey.txt")
	if p.failed != 1 {
		t.Error("cat está en la solución: un cat equivocado es un intento")
	}
	typeLine(t, p, "cat ~/taller/archivo.txt")
	if !p.solved {
		t.Error("la misma salida con otra ruta también resuelve")
	}
}

func TestPracticeCwd(t *testing.T) {
	ex := content.Exercise{
		ID: "cd", Kind: content.KindPractice, Difficulty: 1, TimeLimitSec: 60, MaxAttempts: 3,
		Fixture: "taller", StartDir: "taller", Instructions: "Entra a dia-1.",
		Checks: []content.Check{{Type: content.CheckCwd, Path: "taller/dia-1"}}, Solution: "cd dia-1",
	}
	p, _ := newTestPractice(t, ex)
	typeLine(t, p, "cd dia-1")
	if !p.solved {
		t.Fatal("cd dia-1 debería resolver")
	}
	if !strings.Contains(p.prompt(), "~/taller/dia-1$") {
		t.Errorf("prompt = %q", p.prompt())
	}
}

func TestPracticeResetAndStaleMessages(t *testing.T) {
	p, _ := newTestPractice(t, exMove)
	typeLine(t, p, "touch nuevo")
	failedBefore := p.failed
	old := p.sb
	oldHost := old.HostPath("")

	// Un comando que termina después del reinicio no debe contarse.
	p.input.SetValue("rm file.txt")
	pending := p.Update(key("enter"))
	cmd := p.Update(key("f2"))
	if _, err := os.Stat(oldHost); !os.IsNotExist(err) {
		t.Error("F2 debe borrar el sandbox anterior")
	}
	if msg := pending(); msg != nil {
		p.Update(msg)
	}
	if p.failed != failedBefore {
		t.Error("el resultado de un comando del sandbox anterior no cuenta")
	}
	p.Update(cmd())
	if p.phase != practiceReady || p.sb == old {
		t.Fatalf("tras F2: fase %v", p.phase)
	}
	if _, err := os.Stat(p.sb.HostPath("taller/dir1/nuevo")); !os.IsNotExist(err) {
		t.Error("tras F2 los archivos vuelven a su estado inicial")
	}
	if _, err := os.Stat(p.sb.HostPath("taller/dir1/file.txt")); err != nil {
		t.Error("tras F2 se vuelve a correr el setup")
	}

	// Un sandbox que llega tarde (de otra generación) se cierra y se ignora.
	late, err := p.ctx.newSandbox(sandbox.Options{})
	if err != nil {
		t.Fatal(err)
	}
	cur := p.sb
	p.Update(sandboxReadyMsg{id: p.ctx.id, gen: p.gen - 1, sb: late})
	if p.sb != cur {
		t.Error("un sandbox viejo no debe reemplazar al actual")
	}
	if _, err := os.Stat(late.HostPath("")); !os.IsNotExist(err) {
		t.Error("un sandbox que llega tarde debe cerrarse")
	}
}

func TestPracticeTimeoutAndHints(t *testing.T) {
	p, clk := newTestPractice(t, exMove)
	p.Update(key("f1"))
	if p.hints != 1 || !strings.Contains(p.View(), "mv origen destino") {
		t.Error("F1 debe mostrar la pista")
	}
	clk.t = clk.t.Add(61 * time.Second)
	p.Update(tickMsg{id: p.ctx.id})
	if p.phase != practiceDone || !p.timedOut || p.solved || p.last.Outcome.HintsUsed != 1 {
		t.Fatalf("tiempo agotado: fase %v %+v", p.phase, p.last.Outcome)
	}
}

func TestPracticeHistoryAndClear(t *testing.T) {
	p, _ := newTestPractice(t, exMove)
	typeLine(t, p, "pwd")
	typeLine(t, p, "ls")
	p.Update(key("up"))
	if p.input.Value() != "ls" {
		t.Errorf("↑ = %q", p.input.Value())
	}
	p.Update(key("up"))
	p.Update(key("up"))
	if p.input.Value() != "pwd" {
		t.Errorf("↑↑↑ = %q", p.input.Value())
	}
	p.Update(key("down"))
	p.Update(key("down"))
	if p.input.Value() != "" {
		t.Errorf("↓ al final = %q", p.input.Value())
	}
	p.Update(tea.KeyMsg{Type: tea.KeyCtrlL})
	if len(p.lines) != 0 {
		t.Error("Ctrl-L limpia la terminal")
	}
	typeLine(t, p, "clear")
	if len(p.lines) != 0 {
		t.Error("clear limpia la terminal")
	}
}

func TestPracticeBrokenSandbox(t *testing.T) {
	ctx, _ := testCtx(true)
	ctx.newSandbox = func(o sandbox.Options) (sandbox.Sandbox, error) { return nil, os.ErrPermission }
	p := newPractice(ctx, exMove)
	p.Update(p.Init()())
	if p.phase != practiceBroken || !strings.Contains(p.View(), "No se pudo preparar") {
		t.Fatalf("fase %v", p.phase)
	}
	if cmd := p.Update(key("enter")); cmd == nil || cmd() != (nextExerciseMsg{}) {
		t.Error("enter salta el ejercicio")
	}
}

func TestGuardMessagesCoverAllRules(t *testing.T) {
	for r := sandbox.RulePrivilege; r <= sandbox.RuleForeignProcess; r++ {
		if msg := guardMessage(sandbox.Block{Rule: r, Detail: "x"}); msg == "" || strings.Contains(msg, "%!") {
			t.Errorf("regla %d: mensaje %q", r, msg)
		}
	}
}

var exPattern = content.Exercise{
	ID: "apt", Kind: content.KindPractice, Difficulty: 1, TimeLimitSec: 60, MaxAttempts: 2,
	Instructions: "Instala htop con apt.",
	Checks:       []content.Check{{Type: content.CheckPattern, Accepted: []string{`(sudo )?apt(-get)? install htop`}}},
	Solution:     "sudo apt install htop",
}

func TestPracticePattern(t *testing.T) {
	ctx, _ := testCtx(true)
	ctx.newSandbox = func(sandbox.Options) (sandbox.Sandbox, error) {
		t.Fatal("un ejercicio pattern no debe crear sandbox")
		return nil, nil
	}
	p := newPractice(ctx, exPattern)
	p.SetSize(80, 30)
	p.Update(p.Init()())
	if p.phase != practiceReady || p.sb != nil {
		t.Fatalf("fase %v", p.phase)
	}
	if !strings.Contains(p.View(), "alumno@lsvp-pupil:~$") || !strings.Contains(p.View(), "no se ejecuta") {
		t.Errorf("vista:\n%s", p.View())
	}
	p.Update(key("f2")) // sin sandbox: no hace nada
	p.Update(key("f3"))

	for _, line := range []string{"ls", "pwd"} {
		p.input.SetValue(line)
		if cmd := p.Update(key("enter")); cmd != nil {
			t.Fatalf("%s: un ejercicio pattern no ejecuta nada en segundo plano", line)
		}
	}
	if p.failed != 0 {
		t.Error("la exploración no cuenta como intento")
	}
	p.input.SetValue("sudo apt install htp")
	p.Update(key("enter"))
	if p.failed != 1 || p.notice == "" {
		t.Fatalf("failed %d", p.failed)
	}
	p.input.SetValue("sudo  apt-get install htop")
	p.Update(key("enter"))
	if !p.solved || p.last.Outcome.Command != "sudo  apt-get install htop" || p.last.Outcome.GuardPenalty {
		t.Errorf("resuelto %v, outcome %+v", p.solved, p.last.Outcome)
	}
}

var exKill = content.Exercise{
	ID: "kill", Kind: content.KindPractice, Difficulty: 2, TimeLimitSec: 60, MaxAttempts: 3,
	Fixture: "taller", StartDir: "taller", Setup: []string{"sleep 100000 &"},
	Instructions: "Termina sleep con la señal KILL.",
	Checks:       []content.Check{{Type: content.CheckProcess, Process: "sleep", Signal: "KILL"}},
	Solution:     "kill -9 %1",
}

func TestPracticeProcess(t *testing.T) {
	p, _ := newTestPractice(t, exKill)
	typeLine(t, p, "ps")
	if p.failed != 0 || p.phase != practiceReady {
		t.Fatal("ps es exploración")
	}
	typeLine(t, p, "kill 1")
	if !strings.Contains(p.View(), "fuera de la práctica") || p.failed != 0 {
		t.Errorf("kill a un PID ajeno debe bloquearse sin contar intento; failed=%d", p.failed)
	}
	typeLine(t, p, "kill %1") // TERM, no KILL
	if p.solved || p.failed != 1 {
		t.Fatalf("TERM no es la señal pedida: solved %v failed %d", p.solved, p.failed)
	}
	p2, _ := newTestPractice(t, exKill)
	typeLine(t, p2, "kill -9 %1")
	if !p2.solved {
		t.Error("kill -9 %1 debería resolver")
	}
}

func TestPracticeInteractive(t *testing.T) {
	p, _ := newTestPractice(t, exMove)
	p.input.SetValue("less ../archivo.txt")
	if cmd := p.Update(key("enter")); cmd == nil || p.phase != practiceRunning {
		t.Fatalf("less debe correr con la terminal real (fase %v)", p.phase)
	}
	// Lo que llega cuando el programa interactivo termina:
	p.Update(commandDoneMsg{id: p.ctx.id, gen: p.gen, line: "less ../archivo.txt", interactive: true,
		res: sandbox.Result{Cwd: p.cwd}})
	if p.phase != practiceReady || p.failed != 0 || !strings.Contains(p.View(), "interactivo terminado") {
		t.Errorf("tras less: fase %v, failed %d", p.phase, p.failed)
	}

	p.input.SetValue("sudo nano /etc/hosts")
	cmd := p.Update(key("enter"))
	p.Update(cmd())
	if !p.guardHit || p.phase != practiceReady {
		t.Error("un programa interactivo también pasa por guard")
	}
}

func TestPracticeLongPromptKeepsInputVisible(t *testing.T) {
	ex := exMove
	ex.StartDir = "taller/un/directorio/bastante/profundo/de/verdad"
	ex.Setup = []string{"touch file.txt"}
	p, _ := newTestPractice(t, ex)
	p.input.SetValue("tail -n +2 datos.csv | cut -d , -f 3 | sort | uniq -c | sort -n -r | head -n 1")
	for _, line := range strings.Split(p.View(), "\n") {
		if w := lipgloss.Width(line); w > textWidth(80) {
			t.Errorf("línea de %d columnas (máximo %d): %q", w, textWidth(80), line)
		}
	}
	if !p.promptOwnLine() {
		t.Error("con un prompt tan largo la entrada debe ir en su propia línea")
	}
}
