package ui

import (
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"lsvp-pupil/internal/checks"
	"lsvp-pupil/internal/content"
	"lsvp-pupil/internal/sandbox"
)

var exScript = content.Exercise{
	ID: "scr", Kind: content.KindScript, Difficulty: 2, TimeLimitSec: 300, MaxAttempts: 3,
	Fixture: "taller", StartDir: "taller", Filename: "saludo.sh", RequireShebang: true,
	Instructions: "Saluda con el nombre que llega por stdin.",
	Tests: []content.ScriptTest{
		{Stdin: "Ana\n", StdoutContains: "Hola Ana"},
		{Args: []string{"x"}, Stdin: "Luis\n", StdoutContains: "Hola Luis"},
	},
	SolutionScript: "#!/bin/bash\nread n\necho \"Hola $n\"\n", ParChars: 30,
	Hints: []string{"read nombre"},
}

func newTestEditor(t *testing.T, ex content.Exercise) (*editorScreen, *fakeClock) {
	t.Helper()
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash no está instalado")
	}
	ctx, clk := testCtx(true)
	ctx.newSandbox = func(o sandbox.Options) (sandbox.Sandbox, error) {
		o.Fixtures = uiFixtures
		return sandbox.New(sandbox.BackendDir, o)
	}
	e := newEditor(ctx, ex)
	e.SetSize(80, 30)
	e.Update(e.Init()())
	if e.phase != editorEditing {
		t.Fatalf("fase %v, error %v", e.phase, e.err)
	}
	t.Cleanup(e.Close)
	return e, clk
}

// submit guarda el texto, corre los tests (F5) y entrega el resultado.
func submit(t *testing.T, e *editorScreen, script string) {
	t.Helper()
	e.ta.SetValue(script)
	cmd := e.Update(key("f5"))
	if cmd == nil {
		t.Fatal("F5 no corrió los tests")
	}
	e.Update(cmd())
}

func TestEditorSolve(t *testing.T) {
	e, clk := newTestEditor(t, exScript)
	if !strings.Contains(e.View(), "~/taller/saludo.sh") {
		t.Errorf("la vista debe decir qué archivo se edita:\n%s", e.View())
	}
	submit(t, e, "#!/bin/bash\necho Hola\n")
	if e.failed != 1 || e.phase != editorEditing || !strings.Contains(e.View(), "tu script mostró: Hola") ||
		!strings.Contains(e.View(), "se esperaba que mostrara «Hola Ana»") {
		t.Fatalf("script incorrecto: failed %d\n%s", e.failed, e.View())
	}
	submit(t, e, "echo sin shebang\n")
	if e.failed != 2 || !e.noShebang || !strings.Contains(e.View(), "shebang") {
		t.Fatalf("sin shebang: failed %d", e.failed)
	}
	clk.t = clk.t.Add(time.Minute)
	submit(t, e, "#!/bin/bash\nread nombre\necho \"Hola $nombre\"\n")
	if !e.solved || e.phase != editorDone || e.last.Outcome.FailedAttempts != 2 {
		t.Fatalf("resuelto %v, outcome %+v", e.solved, e.last.Outcome)
	}
	data, _ := os.ReadFile(e.sb.HostPath("taller/saludo.sh"))
	if !strings.Contains(string(data), "read nombre") {
		t.Error("F5 debe guardar el script en el sandbox")
	}
	if !strings.Contains(e.View(), "Una solución:") {
		t.Error("al terminar se muestra la solución")
	}
	if cmd := e.Update(key("enter")); cmd == nil || cmd() != (nextExerciseMsg{}) {
		t.Error("enter pasa al siguiente ejercicio")
	}
}

func TestEditorOutOfAttempts(t *testing.T) {
	ex := exScript
	ex.MaxAttempts = 1
	e, _ := newTestEditor(t, ex)
	submit(t, e, "#!/bin/bash\n")
	if e.phase != editorDone || e.solved {
		t.Fatal("con 1 intento, el primer fallo termina el ejercicio")
	}
}

func TestEditorDraftFromSetupAndExternalEditor(t *testing.T) {
	ex := exScript
	ex.Setup = []string{"printf '#!/bin/bash\\n# borrador\\n' > saludo.sh"}
	e, _ := newTestEditor(t, ex)
	if !strings.Contains(e.ta.Value(), "# borrador") {
		t.Fatalf("el borrador del setup debe cargarse: %q", e.ta.Value())
	}
	e.ta.SetValue("#!/bin/bash\n# desde el editor integrado\n")
	cmd := e.Update(key("f4"))
	if cmd == nil || e.phase != editorRunning {
		t.Fatal("F4 debe abrir el editor externo")
	}
	// Mientras nano estaba abierto, el archivo cambió.
	if err := os.WriteFile(e.sb.HostPath("taller/saludo.sh"), []byte("#!/bin/bash\n# editado en nano\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	e.Update(externalEditorDoneMsg{id: e.ctx.id, gen: e.gen})
	if e.phase != editorEditing || !strings.Contains(e.ta.Value(), "editado en nano") {
		t.Errorf("al volver de nano se recarga el archivo: %q", e.ta.Value())
	}
}

func TestEditorTabTimeoutAndHints(t *testing.T) {
	e, clk := newTestEditor(t, exScript)
	e.Update(key("tab"))
	if e.ta.Value() != "    " {
		t.Errorf("tab = %q", e.ta.Value())
	}
	e.Update(key("f1"))
	if !strings.Contains(e.View(), "read nombre") {
		t.Error("F1 muestra la pista")
	}
	clk.t = clk.t.Add(301 * time.Second)
	e.Update(tickMsg{id: e.ctx.id})
	if e.phase != editorDone || !e.timedOut || e.last.Outcome.HintsUsed != 1 {
		t.Fatalf("tiempo agotado: fase %v %+v", e.phase, e.last.Outcome)
	}
}

func TestExternalEditor(t *testing.T) {
	t.Setenv("LSVP_EDITOR", "")
	t.Setenv("EDITOR", "/usr/bin/vim")
	if got := externalEditor(); got != "vim" {
		t.Errorf("EDITOR=vim: %q", got)
	}
	t.Setenv("EDITOR", "code --wait")
	if got := externalEditor(); got != "nano" {
		t.Errorf("un editor que no está en el sandbox cae en nano: %q", got)
	}
}

func TestFailureText(t *testing.T) {
	one := 1
	tests := []struct {
		r    checks.TestResult
		want string
	}{
		{checks.TestResult{TimedOut: true}, "tardó demasiado"},
		{checks.TestResult{Test: content.ScriptTest{StdoutContains: "Hola"}, Stdout: "Adiós"}, "se esperaba que mostrara «Hola»"},
		{checks.TestResult{Test: content.ScriptTest{ExitCode: &one}, ExitCode: 0}, "terminó con código 0, se esperaba 1"},
		{checks.TestResult{Test: content.ScriptTest{Files: map[string]string{"salida.txt": "4\n"}}, Files: map[string]string{}}, "no se creó el archivo salida.txt"},
		{checks.TestResult{Test: content.ScriptTest{Files: map[string]string{"salida.txt": "4\n"}}, Files: map[string]string{"salida.txt": "4\n16\n"}}, "salida.txt quedó con: 4 ⏎ 16"},
		{checks.TestResult{Test: content.ScriptTest{StdoutRegex: `^\d+$`}, Stdout: "x"}, "formato"},
	}
	for _, tt := range tests {
		if got := failureText(tt.r); !strings.Contains(got, tt.want) {
			t.Errorf("failureText = %q, quiero que contenga %q", got, tt.want)
		}
	}
}
