package checks

import (
	"os"
	"path/filepath"
	"testing"

	"lsvp-pupil/internal/content"
)

func setup(t *testing.T) State {
	t.Helper()
	home := t.TempDir()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(os.MkdirAll(filepath.Join(home, "taller", "dir1"), 0o755))
	must(os.WriteFile(filepath.Join(home, "taller", "notas.txt"), []byte("uno\ndos\ntres\n"), 0o644))
	must(os.WriteFile(filepath.Join(home, "taller", "script.sh"), []byte("#!/bin/bash\n"), 0o755))
	must(os.Symlink("no-existe", filepath.Join(home, "taller", "roto")))
	return State{
		Home:     home,
		Cwd:      home + "/taller",
		HostPath: func(rel string) string { return filepath.Join(home, rel) },
	}
}

func TestFS(t *testing.T) {
	st := setup(t)
	tests := []struct {
		name string
		c    content.Check
		ok   bool
	}{
		{"exists", content.Check{Exists: []string{"taller/notas.txt", "taller/dir1"}}, true},
		{"exists falla", content.Check{Exists: []string{"taller/nada"}}, false},
		{"absent", content.Check{Absent: []string{"taller/nada"}}, true},
		{"absent falla", content.Check{Absent: []string{"taller/notas.txt"}}, false},
		{"enlace roto no está ausente", content.Check{Absent: []string{"taller/roto"}}, false},
		{"is_dir", content.Check{IsDir: []string{"taller/dir1"}}, true},
		{"is_dir con archivo", content.Check{IsDir: []string{"taller/notas.txt"}}, false},
		{"is_file", content.Check{IsFile: []string{"taller/notas.txt"}}, true},
		{"is_file con directorio", content.Check{IsFile: []string{"taller/dir1"}}, false},
		{"executable", content.Check{Executable: []string{"taller/script.sh"}}, true},
		{"executable falla", content.Check{Executable: []string{"taller/notas.txt"}}, false},
		{"content_equals", content.Check{ContentEquals: map[string]string{"taller/notas.txt": "uno\ndos\ntres\n"}}, true},
		{"content_equals sin salto final", content.Check{ContentEquals: map[string]string{"taller/notas.txt": "uno\ndos\ntres"}}, false},
		{"content_contains", content.Check{ContentContains: map[string]string{"taller/notas.txt": "dos"}}, true},
		{"content_contains de archivo inexistente", content.Check{ContentContains: map[string]string{"taller/x": ""}}, false},
		{"line_count", content.Check{LineCount: map[string]int{"taller/notas.txt": 3}}, true},
		{"line_count falla", content.Check{LineCount: map[string]int{"taller/notas.txt": 2}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.c.Type = content.CheckFS
			fails := Run([]content.Check{tt.c}, st)
			if (len(fails) == 0) != tt.ok {
				t.Errorf("fallas = %+v, quiero ok=%v", fails, tt.ok)
			}
		})
	}
}

func TestOutput(t *testing.T) {
	tests := []struct {
		name         string
		mode, expect string
		stdout       string
		solution     string
		ok           bool
	}{
		{"exact", content.OutputExact, "hola\n", "hola\n", "", true},
		{"exact sin salto", content.OutputExact, "hola\n", "hola", "", false},
		{"exact vacío", content.OutputExact, "", "", "", true},
		{"trimmed", content.OutputTrimmed, "hola", "  hola\n\n", "", true},
		{"regex", content.OutputRegex, `^\d+$`, "801", "", true},
		{"regex falla", content.OutputRegex, `^\d+$`, "abc", "", false},
		{"same_as_solution", content.OutputSameAsSolution, "", "a\nb\n", "a\nb\n", true},
		{"same_as_solution sin salto final", content.OutputSameAsSolution, "", "a\nb", "a\nb\n", true},
		{"same_as_solution distinto", content.OutputSameAsSolution, "", "a\n", "a\nb\n", false},
		{"same_as_solution con home distinto", content.OutputSameAsSolution, "", "/tmp/h1/taller/x\n", "/tmp/h2/taller/x\n", true},
		{"same_as_solution vacía nunca pasa", content.OutputSameAsSolution, "", "", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := State{Stdout: tt.stdout, Home: "/tmp/h1", SolutionStdout: tt.solution, SolutionHome: "/tmp/h2"}
			fails := Run([]content.Check{{Type: content.CheckOutput, Mode: tt.mode, Expected: tt.expect}}, st)
			if (len(fails) == 0) != tt.ok {
				t.Errorf("fallas = %+v, quiero ok=%v", fails, tt.ok)
			}
		})
	}
}

func TestExitCodeAndCwd(t *testing.T) {
	one := 1
	st := State{ExitCode: 1, Home: "/h", Cwd: "/h/taller/dir1"}
	cs := []content.Check{
		{Type: content.CheckExitCode, Code: &one},
		{Type: content.CheckCwd, Path: "taller/dir1"},
	}
	if fails := Run(cs, st); len(fails) != 0 {
		t.Errorf("fallas = %+v", fails)
	}
	st.Cwd, st.ExitCode = "/h", 0
	if fails := Run(cs, st); len(fails) != 2 {
		t.Errorf("quiero 2 fallas: %+v", fails)
	}
	if fails := Run([]content.Check{{Type: content.CheckCwd, Path: "."}}, st); len(fails) != 0 {
		t.Errorf("cwd . es el home: %+v", fails)
	}
}

func TestPattern(t *testing.T) {
	accepted := []string{`(sudo )?apt install htop`, `(sudo )?apt-get install htop`}
	tests := []struct {
		cmd string
		ok  bool
	}{
		{"sudo apt install htop", true},
		{"apt install htop", true},
		{"  sudo   apt   install  htop  ", true}, // se normalizan los espacios
		{"sudo apt-get install htop", true},
		{"sudo apt install htop && rm -rf /", false}, // la regex debe cubrir la línea completa
		{"echo sudo apt install htop", false},
		{"sudo apt install", false},
		{"", false},
	}
	for _, tt := range tests {
		fails := Run([]content.Check{{Type: content.CheckPattern, Accepted: accepted}}, State{Command: tt.cmd})
		if (len(fails) == 0) != tt.ok {
			t.Errorf("pattern %q: fallas %v, quiero ok=%v", tt.cmd, fails, tt.ok)
		}
	}
}

func TestProcess(t *testing.T) {
	procs := []Process{{Name: "sleep", Running: false, Signal: "KILL"}, {Name: "yes", Running: true}}
	tests := []struct {
		c  content.Check
		ok bool
	}{
		{content.Check{Process: "sleep"}, true},
		{content.Check{Process: "sleep", Signal: "KILL"}, true},
		{content.Check{Process: "sleep", Signal: "TERM"}, false},
		{content.Check{Process: "yes"}, false},
		{content.Check{Process: "watch"}, false}, // no existe: el ejercicio está mal armado
	}
	for _, tt := range tests {
		tt.c.Type = content.CheckProcess
		if fails := Run([]content.Check{tt.c}, State{Processes: procs}); (len(fails) == 0) != tt.ok {
			t.Errorf("%+v: fallas %v, quiero ok=%v", tt.c, fails, tt.ok)
		}
	}
}
