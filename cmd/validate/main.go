// Command validate carga todo el contenido, revisa su esquema y los mínimos por módulo, y ejecuta
// la solución de cada ejercicio práctico en un sandbox para comprobar que sus checks pasan.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path"
	"strings"

	files "lsvp-pupil/content"
	"lsvp-pupil/fixtures"
	"lsvp-pupil/internal/checks"
	"lsvp-pupil/internal/content"
	"lsvp-pupil/internal/sandbox"
)

func main() {
	backendName := flag.String("sandbox", string(sandbox.BackendAuto), "backend del sandbox: auto, bwrap, container o dir")
	flag.Parse()
	backend, err := sandbox.ParseBackend(*backendName)
	if err == nil {
		backend, err = sandbox.Resolve(backend)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	fmt.Printf("sandbox: %s\n", backend)

	c, err := content.Load(files.Files)
	if err != nil {
		fmt.Fprintf(os.Stderr, "contenido inválido:\n%v\n", err)
		os.Exit(1)
	}
	failed := false
	for _, m := range c.Modules {
		exs := c.Exercises(m.ID)
		n, min := len(exs), m.MinExercises()
		practices := 0
		var errs []string
		for _, ex := range exs {
			if ex.Kind != content.KindPractice && ex.Kind != content.KindScript {
				continue
			}
			practices++
			if err := validatePractice(backend, ex); err != nil {
				errs = append(errs, fmt.Sprintf("    %s: %v", ex.ID, err))
			}
		}
		status := "ok"
		switch {
		case m.Kind == content.ModuleExam:
			status = "usa ejercicios de todos los módulos"
		case n == 0:
			status = "pendiente"
		case n < min:
			status = fmt.Sprintf("ERROR: mínimo %d", min)
			failed = true
		case len(errs) > 0:
			status = "ERROR"
		}
		fmt.Printf("%2d. %-26s %3d ejercicios (%d prácticos)  %s\n", m.ID, m.Title, n, practices, status)
		if len(errs) > 0 {
			fmt.Println(strings.Join(errs, "\n"))
			failed = true
		}
	}
	if failed {
		os.Exit(1)
	}
}

func validatePractice(backend sandbox.Backend, ex content.Exercise) error {
	if ex.Kind == content.KindScript {
		return validateScript(backend, ex)
	}
	if ex.IsPatternOnly() {
		if fails := checks.Run(ex.Checks, checks.State{Command: ex.Solution}); len(fails) > 0 {
			return fmt.Errorf("la solución no coincide con sus patrones: %s", describe(fails))
		}
		if len(checks.Run(ex.Checks, checks.State{Command: ""})) == 0 {
			return errors.New("un comando vacío ya coincide con los patrones")
		}
		return nil
	}
	opts := sandbox.Options{Fixtures: fixtures.Files, Fixture: ex.Fixture, StartDir: ex.StartDir, Setup: ex.Setup}

	ref, err := sandbox.New(backend, opts)
	if err != nil {
		return fmt.Errorf("crear sandbox: %w", err)
	}
	defer ref.Close()
	r, err := ref.Run(ex.Solution)
	switch {
	case err != nil:
		return fmt.Errorf("ejecutar la solución: %w", err)
	case r.Blocked != nil:
		return fmt.Errorf("guard bloquea la solución (regla %d, %q)", r.Blocked.Rule, r.Blocked.Detail)
	case r.TimedOut:
		return errors.New("la solución agota el tiempo")
	}
	solved := checks.State{Command: ex.Solution, Stdout: r.Stdout, ExitCode: r.ExitCode, Cwd: r.Cwd, Home: ref.Home(),
		HostPath: ref.HostPath, SolutionStdout: r.Stdout, SolutionHome: ref.Home()}
	if ex.NeedsProcesses() {
		if solved.Processes, err = processes(ref); err != nil {
			return err
		}
	}
	if fails := checks.Run(ex.Checks, solved); len(fails) > 0 {
		return fmt.Errorf("la solución no pasa sus checks: %s", describe(fails))
	}

	// Sin hacer nada, el ejercicio no debe estar resuelto.
	fresh, err := sandbox.New(backend, opts)
	if err != nil {
		return fmt.Errorf("crear sandbox: %w", err)
	}
	defer fresh.Close()
	initial := checks.State{Cwd: fresh.Cwd(), Home: fresh.Home(), HostPath: fresh.HostPath,
		SolutionStdout: r.Stdout, SolutionHome: ref.Home()}
	if ex.NeedsProcesses() {
		if initial.Processes, err = processes(fresh); err != nil {
			return err
		}
	}
	if len(checks.Run(ex.Checks, initial)) == 0 {
		return errors.New("los checks ya pasan antes de escribir cualquier comando")
	}
	return nil
}

// validateScript comprueba que la solución pase todos los tests y que un script vacío no.
func validateScript(backend sandbox.Backend, ex content.Exercise) error {
	try := func(script string) ([]checks.TestResult, error) {
		sb, err := sandbox.New(backend, sandbox.Options{Fixtures: fixtures.Files, Fixture: ex.Fixture, StartDir: ex.StartDir, Setup: ex.Setup})
		if err != nil {
			return nil, fmt.Errorf("crear sandbox: %w", err)
		}
		defer sb.Close()
		if err := os.WriteFile(sb.HostPath(path.Join(ex.StartDir, ex.Filename)), []byte(script), 0o644); err != nil {
			return nil, fmt.Errorf("escribir el script: %w", err)
		}
		return checks.RunScript(ex, script, sandboxRunner(sb), sandboxReader(sb, ex.StartDir))
	}
	rs, err := try(ex.SolutionScript)
	if err != nil {
		return err
	}
	for i, r := range rs {
		if !r.Passed() {
			return fmt.Errorf("la solución no pasa el test %d: %s (salida %q, errores %q)", i+1, strings.Join(r.Fails, "; "), r.Stdout, r.Stderr)
		}
	}
	if !checks.AllPassed(rs) {
		return errors.New("la solución no corrió ningún test")
	}
	empty, err := try("#!/bin/bash\n")
	if err != nil {
		return err
	}
	if checks.AllPassed(empty) {
		return errors.New("un script vacío ya pasa todos los tests")
	}
	return nil
}

// sandboxRunner adapta un sandbox a checks.RunFunc.
func sandboxRunner(sb sandbox.Sandbox) checks.RunFunc {
	return func(cmd string) (string, string, int, bool, error) {
		r, err := sb.Run(cmd)
		if err == nil && r.Blocked != nil {
			err = fmt.Errorf("guard bloqueó %q (regla %d)", cmd, r.Blocked.Rule)
		}
		return r.Stdout, r.Stderr, r.ExitCode, r.TimedOut, err
	}
}

// sandboxReader adapta un sandbox a checks.ReadFunc, con rutas relativas a startDir.
func sandboxReader(sb sandbox.Sandbox, startDir string) checks.ReadFunc {
	return func(rel string) (string, bool) {
		data, err := os.ReadFile(sb.HostPath(path.Join(startDir, rel)))
		return string(data), err == nil
	}
}

func processes(sb sandbox.Sandbox) ([]checks.Process, error) {
	infos, err := sb.Processes()
	if err != nil {
		return nil, fmt.Errorf("revisar procesos: %w", err)
	}
	ps := make([]checks.Process, len(infos))
	for i, in := range infos {
		ps[i] = checks.Process{Name: in.Name, Running: in.Running, Signal: in.Signal}
	}
	return ps, nil
}

func describe(fails []checks.Failure) string {
	msgs := make([]string, len(fails))
	for i, f := range fails {
		msgs[i] = fmt.Sprintf("check %d: %s", f.Check+1, f.Message)
	}
	return strings.Join(msgs, "; ")
}
