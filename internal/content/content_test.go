package content

import (
	"strings"
	"testing"
	"testing/fstest"

	files "lsvp-pupil/content"
)

func TestLoadEmbedded(t *testing.T) {
	c, err := Load(files.Files)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Modules) != 10 {
		t.Errorf("hay %d módulos, quiero 10", len(c.Modules))
	}
	m, _ := c.Module(1)
	if n := len(c.Exercises(1)); n < m.MinExercises() {
		t.Errorf("el módulo 1 tiene %d ejercicios, mínimo %d", n, m.MinExercises())
	}
}

const modulesYAML = `modules:
  - {id: 1, title: Intro, dir: 01-intro, kind: theory, open: true, source: "` + SourcePrefix + `a.html"}
`

const goodExercise = `id: x-1
module: 1
kind: quiz
format: multiple_choice
difficulty: 1
source: ` + SourcePrefix + `a.html#b
question: "¿?"
options: [a, b]
answer: a
explanation: "porque sí"
time_limit_sec: 10
`

func load(t *testing.T, exercises string) error {
	t.Helper()
	_, err := Load(fstest.MapFS{
		"modules.yaml":              {Data: []byte(modulesYAML)},
		"exercises/01-intro/a.yaml": {Data: []byte(exercises)},
	})
	return err
}

func TestLoadValid(t *testing.T) {
	if err := load(t, goodExercise+"---\n"+strings.Replace(goodExercise, "x-1", "x-2", 1)); err != nil {
		t.Fatal(err)
	}
}

func TestLoadErrors(t *testing.T) {
	tests := []struct {
		name, yaml, want string
	}{
		{"campo desconocido", goodExercise + "respuesta: a\n", "respuesta"},
		{"id repetido", goodExercise + "---\n" + goodExercise, "repetido"},
		{"source fuera del curso", strings.Replace(goodExercise, SourcePrefix, "https://example.com/", 1), "source"},
		{"answer fuera de options", strings.Replace(goodExercise, "answer: a", "answer: c", 1), "no está en options"},
		{"dificultad", strings.Replace(goodExercise, "difficulty: 1", "difficulty: 6", 1), "difficulty"},
		{"sin explicación", strings.Replace(goodExercise, `explanation: "porque sí"`, "", 1), "explanation"},
		{"módulo equivocado", strings.Replace(goodExercise, "module: 1", "module: 2", 1), "no existe"},
		{"format desconocido", strings.Replace(goodExercise, "multiple_choice", "ensayo", 1), "desconocido"},
		{"true_false con options", strings.Replace(goodExercise, "multiple_choice", "true_false", 1), "no lleva options"},
		{"kind no soportado", strings.Replace(goodExercise, "kind: quiz", "kind: ensayo", 1), "no soportado"},
		{"quiz con campos de práctica", goodExercise + "solution: ls\n", "no lleva campos de práctica"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := load(t, tt.yaml)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %v, quiero que contenga %q", err, tt.want)
			}
		})
	}
}

func TestValidateExerciseFormats(t *testing.T) {
	base := Exercise{ID: "x", Kind: KindQuiz, Difficulty: 1, Source: SourcePrefix, Question: "q", Explanation: "e", TimeLimitSec: 5}
	tests := []struct {
		name string
		mod  func(*Exercise)
		ok   bool
	}{
		{"true_false válido", func(e *Exercise) { e.Format = FormatTrueFalse; e.Answer = StringList{True} }, true},
		{"true_false con respuesta rara", func(e *Exercise) { e.Format = FormatTrueFalse; e.Answer = StringList{"sí"} }, false},
		{"short_answer válido", func(e *Exercise) {
			e.Format = FormatShortAnswer
			e.Answer = StringList{"bash"}
			e.Accepted = []string{"GNU bash"}
		}, true},
		{"short_answer sin answer", func(e *Exercise) { e.Format = FormatShortAnswer }, false},
		{"multi_select válido", func(e *Exercise) {
			e.Format = FormatMultiSelect
			e.Options = []string{"a", "b", "c"}
			e.Answer = StringList{"a", "c"}
		}, true},
		{"multi_select sin answer", func(e *Exercise) { e.Format = FormatMultiSelect; e.Options = []string{"a", "b"} }, false},
		{"options repetidas", func(e *Exercise) {
			e.Format = FormatMultipleChoice
			e.Options = []string{"a", "a"}
			e.Answer = StringList{"a"}
		}, false},
		{"accepted fuera de short_answer", func(e *Exercise) {
			e.Format = FormatMultipleChoice
			e.Options = []string{"a", "b"}
			e.Answer = StringList{"a"}
			e.Accepted = []string{"a"}
		}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ex := base
			tt.mod(&ex)
			errs := ValidateExercise(ex)
			if (len(errs) == 0) != tt.ok {
				t.Errorf("errores = %v, quiero ok=%v", errs, tt.ok)
			}
		})
	}
}

const goodPractice = `id: p-1
module: 1
kind: practice
difficulty: 2
source: ` + SourcePrefix + `a.html#b
fixture: taller
start_dir: taller/dir1
setup:
  - mkdir -p ../dir3 && touch file.txt
instructions: "Mueve file.txt a dir3."
checks:
  - type: fs
    exists: [taller/dir3/file.txt]
    absent: [taller/dir1/file.txt]
solution: "mv  file.txt ../dir3"
time_limit_sec: 60
`

func TestLoadPracticeDefaults(t *testing.T) {
	c, err := Load(fstest.MapFS{
		"modules.yaml":              {Data: []byte(modulesYAML)},
		"exercises/01-intro/a.yaml": {Data: []byte(goodPractice)},
	})
	if err != nil {
		t.Fatal(err)
	}
	ex := c.Exercises(1)[0]
	if ex.MaxAttempts != DefaultMaxAttempts {
		t.Errorf("max_attempts = %d, quiero %d", ex.MaxAttempts, DefaultMaxAttempts)
	}
	if ex.ParChars != len("mv file.txt ../dir3") {
		t.Errorf("par_chars = %d (los espacios repetidos no cuentan)", ex.ParChars)
	}
	if ex.NeedsSolutionOutput() {
		t.Error("NeedsSolutionOutput sin checks same_as_solution")
	}
}

func TestPracticeErrors(t *testing.T) {
	tests := []struct {
		name, yaml, want string
	}{
		{"sin solution", strings.Replace(goodPractice, `solution: "mv  file.txt ../dir3"`, "", 1), "falta solution"},
		{"sin instructions", strings.Replace(goodPractice, `instructions: "Mueve file.txt a dir3."`, "", 1), "falta instructions"},
		{"sin checks", strings.Replace(goodPractice, "checks:\n  - type: fs\n    exists: [taller/dir3/file.txt]\n    absent: [taller/dir1/file.txt]\n", "", 1), "al menos un check"},
		{"ruta absoluta", strings.Replace(goodPractice, "exists: [taller/dir3/file.txt]", "exists: [/etc/passwd]", 1), "relativa"},
		{"ruta que sale", strings.Replace(goodPractice, "exists: [taller/dir3/file.txt]", "exists: [taller/../../x]", 1), "relativa"},
		{"start_dir que sale", strings.Replace(goodPractice, "start_dir: taller/dir1", "start_dir: ../x", 1), "start_dir"},
		{"práctica con options", goodPractice + "options: [a, b]\n", "campos de quiz"},
		{"check desconocido", strings.Replace(goodPractice, "type: fs", "type: magia", 1), "desconocido"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := load(t, tt.yaml)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %v, quiero que contenga %q", err, tt.want)
			}
		})
	}
}

func TestValidateCheck(t *testing.T) {
	zero := 0
	tests := []struct {
		name string
		c    Check
		ok   bool
	}{
		{"fs vacío", Check{Type: CheckFS}, false},
		{"fs content_equals", Check{Type: CheckFS, ContentEquals: map[string]string{"a.txt": "x"}}, true},
		{"fs con mode", Check{Type: CheckFS, Exists: []string{"a"}, Mode: OutputExact}, false},
		{"output exact vacío es válido", Check{Type: CheckOutput, Mode: OutputExact}, true},
		{"output trimmed sin expected", Check{Type: CheckOutput, Mode: OutputTrimmed}, false},
		{"output regex inválida", Check{Type: CheckOutput, Mode: OutputRegex, Expected: "("}, false},
		{"output regex", Check{Type: CheckOutput, Mode: OutputRegex, Expected: "^a+$"}, true},
		{"same_as_solution con expected", Check{Type: CheckOutput, Mode: OutputSameAsSolution, Expected: "x"}, false},
		{"same_as_solution", Check{Type: CheckOutput, Mode: OutputSameAsSolution}, true},
		{"output sin mode", Check{Type: CheckOutput, Expected: "x"}, false},
		{"exit_code sin code", Check{Type: CheckExitCode}, false},
		{"exit_code 0", Check{Type: CheckExitCode, Code: &zero}, true},
		{"cwd sin path", Check{Type: CheckCwd}, false},
		{"cwd home", Check{Type: CheckCwd, Path: "."}, true},
		{"cwd absoluto", Check{Type: CheckCwd, Path: "/tmp"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := validateCheck(tt.c)
			if (len(errs) == 0) != tt.ok {
				t.Errorf("errores = %v, quiero ok=%v", errs, tt.ok)
			}
		})
	}
}

func TestPatternExercise(t *testing.T) {
	ok := Exercise{ID: "p", Kind: KindPractice, Difficulty: 1, Source: SourcePrefix, TimeLimitSec: 5,
		Instructions: "Instala htop.", Solution: "sudo apt install htop",
		Checks: []Check{{Type: CheckPattern, Accepted: []string{`(sudo )?apt install htop`}}}}
	if errs := ValidateExercise(ok); len(errs) > 0 {
		t.Fatalf("errores: %v", errs)
	}
	if !ok.IsPatternOnly() {
		t.Error("IsPatternOnly")
	}
	bad := map[string]func(*Exercise){
		"mezclado con fs": func(e *Exercise) { e.Checks = append(e.Checks, Check{Type: CheckFS, Exists: []string{"a"}}) },
		"con setup":       func(e *Exercise) { e.Setup = []string{"touch a"} },
		"con fixture":     func(e *Exercise) { e.Fixture = "taller" },
		"sin accepted":    func(e *Exercise) { e.Checks = []Check{{Type: CheckPattern}} },
		"regex inválida":  func(e *Exercise) { e.Checks = []Check{{Type: CheckPattern, Accepted: []string{"("}}} },
		"accepted en fs":  func(e *Exercise) { e.Checks = []Check{{Type: CheckFS, Exists: []string{"a"}, Accepted: []string{"x"}}} },
	}
	for name, mod := range bad {
		ex := ok
		ex.Checks = append([]Check(nil), ok.Checks...)
		mod(&ex)
		if len(ValidateExercise(ex)) == 0 {
			t.Errorf("%s: debería ser inválido", name)
		}
	}
}

func TestProcessCheckValidation(t *testing.T) {
	base := Exercise{ID: "p", Kind: KindPractice, Difficulty: 1, Source: SourcePrefix, TimeLimitSec: 5,
		Instructions: "Termina sleep.", Solution: "kill %1", Setup: []string{"sleep 1000 &"},
		Checks: []Check{{Type: CheckProcess, Process: "sleep", Signal: "KILL"}}}
	if errs := ValidateExercise(base); len(errs) > 0 {
		t.Fatalf("errores: %v", errs)
	}
	bad := map[string]func(*Exercise){
		"sin setup":         func(e *Exercise) { e.Setup = nil },
		"sin nombre":        func(e *Exercise) { e.Checks[0].Process = "" },
		"señal desconocida": func(e *Exercise) { e.Checks[0].Signal = "MATAR" },
		"signal en fs":      func(e *Exercise) { e.Checks[0] = Check{Type: CheckFS, Exists: []string{"a"}, Signal: "KILL"} },
	}
	for name, mod := range bad {
		ex := base
		ex.Checks = []Check{base.Checks[0]}
		mod(&ex)
		if len(ValidateExercise(ex)) == 0 {
			t.Errorf("%s: debería ser inválido", name)
		}
	}
}
