package checks

import (
	"fmt"
	"regexp"
	"strings"

	"lsvp-pupil/internal/content"
)

// ScriptCommand arma la línea que corre un test de script en el sandbox, desde start_dir:
// bash ./filename 'arg1' 'arg2' < <(printf '%s' 'stdin'). Sin stdin, lee de /dev/null.
func ScriptCommand(filename string, t content.ScriptTest) string {
	var b strings.Builder
	b.WriteString("bash ./" + filename)
	for _, a := range t.Args {
		b.WriteString(" " + ShellQuote(a))
	}
	if t.Stdin != "" {
		b.WriteString(" < <(printf '%s' " + ShellQuote(t.Stdin) + ")")
	} else {
		b.WriteString(" < /dev/null")
	}
	return b.String()
}

// ShellQuote encierra s entre comillas simples para bash.
func ShellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// ScriptTest revisa un test ya ejecutado. Devuelve las expectativas que no se cumplieron.
func ScriptTest(t content.ScriptTest, r TestResult) []string {
	var fails []string
	if t.StdoutContains != "" && !strings.Contains(r.Stdout, t.StdoutContains) {
		fails = append(fails, fmt.Sprintf("la salida no contiene %q", t.StdoutContains))
	}
	if t.StdoutRegex != "" {
		re, err := regexp.Compile(t.StdoutRegex)
		switch {
		case err != nil:
			fails = append(fails, fmt.Sprintf("regex inválida: %v", err))
		case !re.MatchString(r.Stdout):
			fails = append(fails, fmt.Sprintf("la salida no coincide con %q", t.StdoutRegex))
		}
	}
	if t.ExitCode != nil && r.ExitCode != *t.ExitCode {
		fails = append(fails, fmt.Sprintf("terminó con código %d, se esperaba %d", r.ExitCode, *t.ExitCode))
	}
	for _, p := range sortedKeys(t.Files) {
		if got, ok := r.Files[p]; !ok || got != t.Files[p] {
			fails = append(fails, fmt.Sprintf("%s no tiene el contenido esperado", p))
		}
	}
	return fails
}

// HasShebang dice si el script empieza con #! (sin espacios ni líneas antes).
func HasShebang(script string) bool {
	return strings.HasPrefix(script, "#!")
}

// RunFunc corre una línea en el sandbox (desde start_dir) y devuelve lo que produjo.
type RunFunc func(cmd string) (stdout, stderr string, exitCode int, timedOut bool, err error)

// ReadFunc lee un archivo del sandbox, con ruta relativa a start_dir.
type ReadFunc func(rel string) (content string, ok bool)

// TestResult es el resultado de un test de script.
type TestResult struct {
	Test     content.ScriptTest
	Stdout   string
	Stderr   string
	ExitCode int
	TimedOut bool
	Files    map[string]string // contenido de los archivos de Test.Files que existían tras el test
	Fails    []string
}

// Passed dice si el test pasó.
func (r TestResult) Passed() bool { return len(r.Fails) == 0 }

// RunScript corre todos los tests de un ejercicio de script con run. El script ya debe estar
// escrito en el sandbox. Si falta el shebang que pide el ejercicio, no corre nada.
func RunScript(ex content.Exercise, script string, run RunFunc, read ReadFunc) ([]TestResult, error) {
	if ex.RequireShebang && !HasShebang(script) {
		return nil, nil
	}
	results := make([]TestResult, len(ex.Tests))
	for i, t := range ex.Tests {
		out, errOut, code, timedOut, err := run(ScriptCommand(ex.Filename, t))
		if err != nil {
			return nil, fmt.Errorf("test %d: %w", i+1, err)
		}
		r := TestResult{Test: t, Stdout: out, Stderr: errOut, ExitCode: code, TimedOut: timedOut, Files: map[string]string{}}
		for p := range t.Files {
			if data, ok := read(p); ok {
				r.Files[p] = data
			}
		}
		r.Fails = ScriptTest(t, r)
		results[i] = r
		if timedOut {
			results[i].Fails = append(results[i].Fails, "el script tardó demasiado")
		}
	}
	return results, nil
}

// AllPassed dice si hay resultados y todos pasaron.
func AllPassed(rs []TestResult) bool {
	for _, r := range rs {
		if !r.Passed() {
			return false
		}
	}
	return len(rs) > 0
}
