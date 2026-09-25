// Package checks valida el estado de un sandbox contra los checks de un ejercicio práctico.
// No conoce la UI ni el sandbox: recibe un State.
package checks

import (
	"fmt"
	"os"
	"path"
	"regexp"
	"sort"
	"strings"

	"lsvp-pupil/internal/content"
)

// State es lo que hace falta para evaluar los checks después de un comando.
type State struct {
	Command  string // la línea que escribió el usuario (para pattern)
	Stdout   string
	ExitCode int
	Cwd      string // directorio actual como lo ve la shell
	Home     string // home como lo ve la shell
	// HostPath traduce una ruta relativa al home a una ruta en este equipo.
	HostPath func(rel string) string

	// Para process: los procesos que el setup dejó en segundo plano.
	Processes []Process

	// Para same_as_solution: la salida de la solución en un sandbox limpio y el home de ese sandbox.
	SolutionStdout string
	SolutionHome   string
}

// Process es el estado de un proceso lanzado por el setup (copia de sandbox.ProcessInfo, para no
// depender del sandbox).
type Process struct {
	Name    string
	Running bool
	Signal  string
}

// Failure describe un check que no pasó. El mensaje es para quien escribe el contenido
// (make validate), no para el alumno.
type Failure struct {
	Check   int // índice en la lista de checks
	Message string
}

// Run evalúa todos los checks y devuelve los que fallaron. El ejercicio está resuelto si no falla ninguno.
func Run(cs []content.Check, st State) []Failure {
	var fails []Failure
	for i, c := range cs {
		for _, msg := range check(c, st) {
			fails = append(fails, Failure{Check: i, Message: msg})
		}
	}
	return fails
}

func check(c content.Check, st State) []string {
	switch c.Type {
	case content.CheckFS:
		return checkFS(c, st)
	case content.CheckOutput:
		return checkOutput(c, st)
	case content.CheckExitCode:
		if st.ExitCode != *c.Code {
			return []string{fmt.Sprintf("código de salida %d, se esperaba %d", st.ExitCode, *c.Code)}
		}
	case content.CheckProcess:
		return checkProcess(c, st)
	case content.CheckPattern:
		if !MatchPattern(c.Accepted, st.Command) {
			return []string{fmt.Sprintf("el comando %q no coincide con ninguna respuesta aceptada", st.Command)}
		}
	case content.CheckCwd:
		if want := path.Join(st.Home, c.Path); path.Clean(st.Cwd) != want {
			return []string{fmt.Sprintf("directorio actual %s, se esperaba %s", st.Cwd, want)}
		}
	default:
		return []string{fmt.Sprintf("tipo de check desconocido %q", c.Type)}
	}
	return nil
}

func checkFS(c content.Check, st State) []string {
	var fails []string
	fail := func(format string, a ...any) { fails = append(fails, fmt.Sprintf(format, a...)) }
	stat := func(rel string) (os.FileInfo, bool) {
		info, err := os.Stat(st.HostPath(rel))
		return info, err == nil
	}
	for _, p := range c.Exists {
		if _, ok := stat(p); !ok {
			fail("%s no existe", p)
		}
	}
	for _, p := range c.Absent {
		// Lstat: un enlace roto también cuenta como que el archivo sigue ahí.
		if _, err := os.Lstat(st.HostPath(p)); err == nil {
			fail("%s todavía existe", p)
		}
	}
	for _, p := range c.IsDir {
		if info, ok := stat(p); !ok || !info.IsDir() {
			fail("%s no es un directorio", p)
		}
	}
	for _, p := range c.IsFile {
		if info, ok := stat(p); !ok || !info.Mode().IsRegular() {
			fail("%s no es un archivo regular", p)
		}
	}
	for _, p := range c.Executable {
		if info, ok := stat(p); !ok || info.Mode().Perm()&0o111 == 0 {
			fail("%s no tiene permiso de ejecución", p)
		}
	}
	for _, p := range sortedKeys(c.ContentEquals) {
		if got, ok := read(st, p); !ok || got != c.ContentEquals[p] {
			fail("el contenido de %s no es el esperado", p)
		}
	}
	for _, p := range sortedKeys(c.ContentContains) {
		if got, ok := read(st, p); !ok || !strings.Contains(got, c.ContentContains[p]) {
			fail("%s no contiene %q", p, c.ContentContains[p])
		}
	}
	for _, p := range sortedKeys(c.LineCount) {
		// Como wc -l: cuenta saltos de línea.
		if got, ok := read(st, p); !ok || strings.Count(got, "\n") != c.LineCount[p] {
			fail("%s no tiene %d líneas", p, c.LineCount[p])
		}
	}
	return fails
}

func checkOutput(c content.Check, st State) []string {
	ok := false
	switch c.Mode {
	case content.OutputExact:
		ok = st.Stdout == c.Expected
	case content.OutputTrimmed:
		ok = strings.TrimSpace(st.Stdout) == strings.TrimSpace(c.Expected)
	case content.OutputRegex:
		re, err := regexp.Compile(c.Expected)
		if err != nil {
			return []string{fmt.Sprintf("regex inválida: %v", err)}
		}
		ok = re.MatchString(st.Stdout)
	case content.OutputSameAsSolution:
		// Cada sandbox tiene su propio home; se compara con el home escrito como ~.
		got := normalizeHome(st.Stdout, st.Home)
		want := normalizeHome(st.SolutionStdout, st.SolutionHome)
		ok = strings.TrimRight(got, "\n") == strings.TrimRight(want, "\n") && strings.TrimSpace(want) != ""
	default:
		return []string{fmt.Sprintf("mode desconocido %q", c.Mode)}
	}
	if !ok {
		return []string{fmt.Sprintf("la salida no coincide (%s)", c.Mode)}
	}
	return nil
}

func normalizeHome(s, home string) string {
	if home == "" {
		return s
	}
	return strings.ReplaceAll(s, home, "~")
}

func read(st State, rel string) (string, bool) {
	b, err := os.ReadFile(st.HostPath(rel))
	return string(b), err == nil
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// NormalizeCommand quita espacios extremos y deja un solo espacio entre palabras.
func NormalizeCommand(cmd string) string {
	return strings.Join(strings.Fields(cmd), " ")
}

// MatchPattern dice si el comando (normalizado) coincide completo con alguna de las regex.
func MatchPattern(accepted []string, cmd string) bool {
	cmd = NormalizeCommand(cmd)
	for _, a := range accepted {
		re, err := regexp.Compile(`^(?:` + a + `)$`)
		if err == nil && re.MatchString(cmd) {
			return true
		}
	}
	return false
}

func checkProcess(c content.Check, st State) []string {
	found := false
	for _, p := range st.Processes {
		if p.Name != c.Process {
			continue
		}
		found = true
		switch {
		case p.Running:
			return []string{fmt.Sprintf("%s sigue corriendo", c.Process)}
		case c.Signal != "" && p.Signal != c.Signal:
			return []string{fmt.Sprintf("%s terminó con la señal %q, se esperaba %s", c.Process, p.Signal, c.Signal)}
		}
	}
	if !found {
		return []string{fmt.Sprintf("el setup no lanzó ningún proceso %s", c.Process)}
	}
	return nil
}
