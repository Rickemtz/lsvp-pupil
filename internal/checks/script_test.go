package checks

import (
	"os"
	"os/exec"
	"testing"

	"lsvp-pupil/internal/content"
)

func TestScriptCommand(t *testing.T) {
	tests := []struct {
		test content.ScriptTest
		want string
	}{
		{content.ScriptTest{}, "bash ./a.sh < /dev/null"},
		{content.ScriptTest{Args: []string{"hola", "dos palabras", "it's"}}, `bash ./a.sh 'hola' 'dos palabras' 'it'\''s' < /dev/null`},
		{content.ScriptTest{Stdin: "Ana\n20\n"}, "bash ./a.sh < <(printf '%s' 'Ana\n20\n')"},
	}
	for _, tt := range tests {
		if got := ScriptCommand("a.sh", tt.test); got != tt.want {
			t.Errorf("ScriptCommand = %q, quiero %q", got, tt.want)
		}
	}
}

// Lo que arma ScriptCommand, bash lo entiende igual: argumentos y stdin llegan intactos.
func TestScriptCommandRoundTrip(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash no está instalado")
	}
	dir := t.TempDir()
	script := `printf '[%s]' "$@"; echo; cat`
	if err := writeFile(dir+"/eco.sh", script); err != nil {
		t.Fatal(err)
	}
	test := content.ScriptTest{Args: []string{"a b", "it's", "$HOME", "*"}, Stdin: "línea 1\nlínea 'dos'\n"}
	cmd := exec.Command("bash", "-c", ScriptCommand("eco.sh", test))
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	want := "[a b][it's][$HOME][*]\nlínea 1\nlínea 'dos'\n"
	if string(out) != want {
		t.Errorf("salida %q, quiero %q", out, want)
	}
}

func TestScriptTestExpectations(t *testing.T) {
	tt := content.ScriptTest{StdoutContains: "Hola", StdoutRegex: `\d+ años`}
	if f := ScriptTest(tt, TestResult{Stdout: "Hola Ana, tienes 20 años"}); len(f) != 0 {
		t.Errorf("fallas %v", f)
	}
	if f := ScriptTest(tt, TestResult{Stdout: "hola Ana"}); len(f) != 2 {
		t.Errorf("quiero 2 fallas: %v", f)
	}
	one := 1
	ft := content.ScriptTest{ExitCode: &one, Files: map[string]string{"salida.txt": "4\n", "error.log": ""}}
	ok := TestResult{ExitCode: 1, Files: map[string]string{"salida.txt": "4\n", "error.log": ""}}
	if f := ScriptTest(ft, ok); len(f) != 0 {
		t.Errorf("fallas %v", f)
	}
	bad := TestResult{ExitCode: 0, Files: map[string]string{"salida.txt": "4 ejemplo.txt\n"}}
	if f := ScriptTest(ft, bad); len(f) != 3 { // código, contenido distinto y error.log que no existe
		t.Errorf("quiero 3 fallas: %v", f)
	}
	if !HasShebang("#!/bin/bash\necho") || HasShebang("\n#!/bin/bash") || HasShebang("echo") {
		t.Error("HasShebang")
	}
}

func writeFile(p, s string) error { return os.WriteFile(p, []byte(s), 0o644) }
