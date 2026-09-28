// Package content carga los módulos y ejercicios en YAML y valida su esquema.
package content

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// SourcePrefix es el dominio del curso al que debe apuntar todo campo source.
const SourcePrefix = "https://lsvp-uami.github.io/mdbook-curso-linux-0/"

// Tipos de módulo.
const (
	ModuleTheory   = "theory"
	ModuleMixed    = "mixed"
	ModulePractice = "practice"
	ModuleExam     = "exam"
)

// Tipos y formatos de ejercicio.
const (
	KindQuiz     = "quiz"
	KindPractice = "practice"
	KindScript   = "script"

	FormatMultipleChoice = "multiple_choice"
	FormatTrueFalse      = "true_false"
	FormatShortAnswer    = "short_answer"
	FormatMultiSelect    = "multi_select"
)

// Respuestas canónicas de las preguntas true_false.
const (
	True  = "verdadero"
	False = "falso"
)

// Module describe un módulo del curso.
type Module struct {
	ID     int    `yaml:"id"`
	Title  string `yaml:"title"`
	Dir    string `yaml:"dir"` // subdirectorio de exercises/; vacío en el examen
	Kind   string `yaml:"kind"`
	Open   bool   `yaml:"open"` // abierto desde el inicio
	Source string `yaml:"source"`
}

// MinExercises es el mínimo de ejercicios que exige el módulo (0 para el examen).
func (m Module) MinExercises() int {
	switch m.Kind {
	case ModuleTheory:
		return 15
	case ModuleExam:
		return 0
	default:
		return 10
	}
}

// Exercise es un ejercicio de cualquier tipo (quiz o practice).
type Exercise struct {
	ID           string `yaml:"id"`
	Module       int    `yaml:"module"`
	Kind         string `yaml:"kind"`
	Difficulty   int    `yaml:"difficulty"`
	Source       string `yaml:"source"`
	Explanation  string `yaml:"explanation"`
	TimeLimitSec int    `yaml:"time_limit_sec"`
	// LessonTimeLimitSec sustituye a TimeLimitSec solo en Lecciones; 0: usa TimeLimitSec.
	LessonTimeLimitSec int      `yaml:"lesson_time_limit_sec"`
	Hints              []string `yaml:"hints"`

	// Quiz.
	Format   string     `yaml:"format"`
	Question string     `yaml:"question"`
	Options  []string   `yaml:"options"`
	Answer   StringList `yaml:"answer"`
	Accepted []string   `yaml:"accepted"`

	// Práctica.
	Fixture      string   `yaml:"fixture"`
	StartDir     string   `yaml:"start_dir"`
	Setup        []string `yaml:"setup"`
	Instructions string   `yaml:"instructions"`
	Checks       []Check  `yaml:"checks"`
	Solution     string   `yaml:"solution"`
	ParChars     int      `yaml:"par_chars"`    // 0: la longitud de solution
	MaxAttempts  int      `yaml:"max_attempts"` // 0: DefaultMaxAttempts

	// Script (también usa fixture, start_dir, setup, instructions, max_attempts).
	Filename       string       `yaml:"filename"`        // nombre del script, dentro de start_dir
	Tests          []ScriptTest `yaml:"tests"`           // cada prueba corre «bash ./filename args» con stdin
	RequireShebang bool         `yaml:"require_shebang"` // la primera línea debe empezar con #!
	SolutionFile   string       `yaml:"solution_file"`   // relativo al directorio del YAML
	SolutionScript string       `yaml:"-"`               // contenido de solution_file, leído al cargar

	File string `yaml:"-"` // archivo de origen, para mensajes de error
}

// ScriptTest es una prueba de un ejercicio de script. Pasa si se cumplen todas sus expectativas.
type ScriptTest struct {
	Args           []string          `yaml:"args"`
	Stdin          string            `yaml:"stdin"`
	StdoutRegex    string            `yaml:"stdout_regex"`
	StdoutContains string            `yaml:"stdout_contains"`
	ExitCode       *int              `yaml:"exit_code"` // código de salida esperado
	Files          map[string]string `yaml:"files"`     // contenido exacto tras el test; rutas relativas a start_dir
}

// DefaultMaxAttempts es el número de intentos de un ejercicio práctico que no indica max_attempts.
const DefaultMaxAttempts = 3

// Tipos de check.
const (
	CheckFS       = "fs"
	CheckOutput   = "output"
	CheckExitCode = "exit_code"
	CheckCwd      = "cwd"
	CheckPattern  = "pattern" // compara el texto del comando; no se ejecuta nada
	CheckProcess  = "process" // un proceso que lanzó el setup ya no corre (o murió por cierta señal)
)

// Signals son las señales que acepta un check process.
var Signals = []string{"HUP", "INT", "QUIT", "KILL", "TERM", "USR1", "USR2"}

// Modos del check output.
const (
	OutputExact          = "exact"
	OutputTrimmed        = "trimmed"
	OutputRegex          = "regex"
	OutputSameAsSolution = "same_as_solution"
)

// Check es un validador de un ejercicio práctico. Las rutas son relativas al home del sandbox.
type Check struct {
	Type string `yaml:"type"`

	// fs
	Exists          []string          `yaml:"exists"`
	Absent          []string          `yaml:"absent"`
	IsDir           []string          `yaml:"is_dir"`
	IsFile          []string          `yaml:"is_file"`
	Executable      []string          `yaml:"executable"`
	ContentEquals   map[string]string `yaml:"content_equals"`
	ContentContains map[string]string `yaml:"content_contains"`
	LineCount       map[string]int    `yaml:"line_count"`

	// output
	Mode     string `yaml:"mode"`
	Expected string `yaml:"expected"`

	// exit_code
	Code *int `yaml:"code"`

	// cwd
	Path string `yaml:"path"`

	// pattern: regex que deben coincidir con la línea completa (ya normalizada: sin espacios repetidos)
	Accepted []string `yaml:"accepted"`

	// process: nombre del programa lanzado en segundo plano por el setup, y la señal esperada (opcional)
	Process string `yaml:"process"`
	Signal  string `yaml:"signal"`
}

// TimeLimit es el tiempo para resolver el ejercicio; en Lecciones manda lesson_time_limit_sec si existe.
func (ex Exercise) TimeLimit(lessons bool) time.Duration {
	sec := ex.TimeLimitSec
	if lessons && ex.LessonTimeLimitSec > 0 {
		sec = ex.LessonTimeLimitSec
	}
	return time.Duration(sec) * time.Second
}

// NeedsProcesses dice si algún check revisa procesos lanzados por el setup.
func (ex Exercise) NeedsProcesses() bool {
	for _, c := range ex.Checks {
		if c.Type == CheckProcess {
			return true
		}
	}
	return false
}

// IsPatternOnly dice si el ejercicio solo compara el texto del comando (apt, ssh...): no crea sandbox.
func (ex Exercise) IsPatternOnly() bool {
	for _, c := range ex.Checks {
		if c.Type != CheckPattern {
			return false
		}
	}
	return len(ex.Checks) > 0
}

// NeedsSolutionOutput dice si algún check compara con la salida de la solución.
func (ex Exercise) NeedsSolutionOutput() bool {
	for _, c := range ex.Checks {
		if c.Type == CheckOutput && c.Mode == OutputSameAsSolution {
			return true
		}
	}
	return false
}

// StringList acepta en YAML tanto un escalar como una lista de escalares.
type StringList []string

func (l *StringList) UnmarshalYAML(n *yaml.Node) error {
	switch n.Kind {
	case yaml.ScalarNode:
		*l = StringList{n.Value}
		return nil
	case yaml.SequenceNode:
		var s []string
		if err := n.Decode(&s); err != nil {
			return err
		}
		*l = s
		return nil
	}
	return fmt.Errorf("línea %d: se esperaba texto o lista de textos", n.Line)
}

// Catalog contiene todo el contenido cargado.
type Catalog struct {
	Modules   []Module
	exercises map[int][]Exercise
}

// Module devuelve el módulo con ese id.
func (c *Catalog) Module(id int) (Module, bool) {
	for _, m := range c.Modules {
		if m.ID == id {
			return m, true
		}
	}
	return Module{}, false
}

// Exercises devuelve los ejercicios de un módulo en el orden de sus archivos.
func (c *Catalog) Exercises(module int) []Exercise {
	return c.exercises[module]
}

// Load lee modules.yaml y exercises/<dir>/*.yaml de fsys y valida todo.
// Un archivo puede tener varios ejercicios separados por "---".
func Load(fsys fs.FS) (*Catalog, error) {
	var mf struct {
		Modules []Module `yaml:"modules"`
	}
	if err := decodeFile(fsys, "modules.yaml", func(d *yaml.Decoder) error { return d.Decode(&mf) }); err != nil {
		return nil, err
	}
	c := &Catalog{Modules: mf.Modules, exercises: map[int][]Exercise{}}

	files, err := fs.Glob(fsys, "exercises/*/*.yaml")
	if err != nil {
		return nil, fmt.Errorf("listar ejercicios: %w", err)
	}
	sort.Strings(files)
	for _, f := range files {
		err := decodeFile(fsys, f, func(d *yaml.Decoder) error {
			for {
				var ex Exercise
				err := d.Decode(&ex)
				if errors.Is(err, io.EOF) {
					return nil
				}
				if err != nil {
					return err
				}
				ex.File = f
				if ex.Kind == KindScript && ex.SolutionFile != "" {
					data, err := fs.ReadFile(fsys, path.Join(path.Dir(f), ex.SolutionFile))
					if err != nil {
						return fmt.Errorf("%s: leer solution_file: %w", ex.ID, err)
					}
					ex.SolutionScript = string(data)
				}
				if ex.Kind == KindScript {
					if ex.MaxAttempts == 0 {
						ex.MaxAttempts = DefaultMaxAttempts
					}
					if ex.ParChars == 0 {
						ex.ParChars = len([]rune(strings.Join(strings.Fields(ex.SolutionScript), " ")))
					}
				}
				if ex.Kind == KindPractice {
					if ex.MaxAttempts == 0 {
						ex.MaxAttempts = DefaultMaxAttempts
					}
					if ex.ParChars == 0 {
						ex.ParChars = len([]rune(strings.Join(strings.Fields(ex.Solution), " ")))
					}
				}
				c.exercises[ex.Module] = append(c.exercises[ex.Module], ex)
			}
		})
		if err != nil {
			return nil, err
		}
	}
	if err := c.validate(files); err != nil {
		return nil, err
	}
	return c, nil
}

func decodeFile(fsys fs.FS, name string, fn func(*yaml.Decoder) error) error {
	data, err := fs.ReadFile(fsys, name)
	if err != nil {
		return fmt.Errorf("leer %s: %w", name, err)
	}
	d := yaml.NewDecoder(bytes.NewReader(data))
	d.KnownFields(true)
	if err := fn(d); err != nil {
		return fmt.Errorf("decodificar %s: %w", name, err)
	}
	return nil
}

func (c *Catalog) validate(files []string) error {
	var errs []error
	fail := func(format string, a ...any) { errs = append(errs, fmt.Errorf(format, a...)) }

	dirs := map[string]int{}
	for i, m := range c.Modules {
		if m.ID != i+1 {
			fail("modules.yaml: el módulo %d tiene id %d; los ids deben ser 1, 2, 3...", i+1, m.ID)
		}
		if m.Title == "" {
			fail("modules.yaml: el módulo %d no tiene title", m.ID)
		}
		switch m.Kind {
		case ModuleTheory, ModuleMixed, ModulePractice:
			if m.Dir == "" {
				fail("modules.yaml: el módulo %d no tiene dir", m.ID)
			}
			dirs[m.Dir] = m.ID
		case ModuleExam:
		default:
			fail("modules.yaml: el módulo %d tiene kind desconocido %q", m.ID, m.Kind)
		}
		if !strings.HasPrefix(m.Source, SourcePrefix) {
			fail("modules.yaml: el source del módulo %d no apunta al curso", m.ID)
		}
	}
	for _, f := range files {
		if _, ok := dirs[path.Base(path.Dir(f))]; !ok {
			fail("%s: el directorio no corresponde a ningún módulo", f)
		}
	}

	ids := map[string]string{}
	mods := make([]int, 0, len(c.exercises))
	for mod := range c.exercises {
		mods = append(mods, mod)
	}
	sort.Ints(mods)
	for _, mod := range mods {
		for _, ex := range c.exercises[mod] {
			if prev, dup := ids[ex.ID]; dup {
				fail("%s: id %q repetido (también en %s)", ex.File, ex.ID, prev)
			}
			ids[ex.ID] = ex.File
			m, ok := c.Module(mod)
			if !ok {
				fail("%s: %s: el módulo %d no existe", ex.File, ex.ID, mod)
				continue
			}
			if dirs[path.Base(path.Dir(ex.File))] != m.ID {
				fail("%s: %s: dice module %d pero está en el directorio de otro módulo", ex.File, ex.ID, mod)
			}
			for _, err := range ValidateExercise(ex) {
				fail("%s: %s: %w", ex.File, ex.ID, err)
			}
		}
	}
	return errors.Join(errs...)
}

// ValidateExercise revisa el esquema de un ejercicio y devuelve todos los problemas encontrados.
func ValidateExercise(ex Exercise) []error {
	var errs []error
	fail := func(format string, a ...any) { errs = append(errs, fmt.Errorf(format, a...)) }

	if ex.ID == "" {
		fail("falta id")
	}
	if ex.Difficulty < 1 || ex.Difficulty > 5 {
		fail("difficulty debe estar entre 1 y 5")
	}
	if !strings.HasPrefix(ex.Source, SourcePrefix) {
		fail("source debe empezar con %s", SourcePrefix)
	}
	if ex.TimeLimitSec <= 0 {
		fail("time_limit_sec debe ser mayor que 0")
	}
	if ex.LessonTimeLimitSec < 0 {
		fail("lesson_time_limit_sec no puede ser negativo")
	}
	switch ex.Kind {
	case KindQuiz:
		errs = append(errs, validateQuiz(ex)...)
	case KindPractice:
		errs = append(errs, validatePractice(ex)...)
	case KindScript:
		errs = append(errs, validateScript(ex)...)
	default:
		fail("kind %q no soportado", ex.Kind)
	}
	return errs
}

func validateQuiz(ex Exercise) []error {
	var errs []error
	fail := func(format string, a ...any) { errs = append(errs, fmt.Errorf(format, a...)) }
	if strings.TrimSpace(ex.Question) == "" {
		fail("falta question")
	}
	if strings.TrimSpace(ex.Explanation) == "" {
		fail("falta explanation")
	}
	if ex.Instructions != "" || ex.Solution != "" || len(ex.Checks) > 0 || ex.Fixture != "" || len(ex.Setup) > 0 {
		fail("un quiz no lleva campos de práctica")
	}

	inOptions := func(a string) bool {
		for _, o := range ex.Options {
			if o == a {
				return true
			}
		}
		return false
	}
	switch ex.Format {
	case FormatMultipleChoice, FormatMultiSelect:
		if len(ex.Options) < 2 {
			fail("%s necesita al menos 2 options", ex.Format)
		}
		seen := map[string]bool{}
		for _, o := range ex.Options {
			if seen[o] {
				fail("option repetida %q", o)
			}
			seen[o] = true
		}
		if ex.Format == FormatMultipleChoice && len(ex.Answer) != 1 {
			fail("multiple_choice necesita exactamente una answer")
		}
		if ex.Format == FormatMultiSelect && len(ex.Answer) == 0 {
			fail("multi_select necesita al menos una answer")
		}
		for _, a := range ex.Answer {
			if !inOptions(a) {
				fail("answer %q no está en options", a)
			}
		}
	case FormatTrueFalse:
		if len(ex.Options) > 0 {
			fail("true_false no lleva options")
		}
		if len(ex.Answer) != 1 || (ex.Answer[0] != True && ex.Answer[0] != False) {
			fail("true_false necesita answer %q o %q", True, False)
		}
	case FormatShortAnswer:
		if len(ex.Options) > 0 {
			fail("short_answer no lleva options")
		}
		if len(ex.Answer) != 1 {
			fail("short_answer necesita exactamente una answer (la que se muestra como correcta)")
		}
	default:
		fail("format %q desconocido", ex.Format)
	}
	if len(ex.Accepted) > 0 && ex.Format != FormatShortAnswer {
		fail("accepted solo se usa en short_answer")
	}
	return errs
}

func validatePractice(ex Exercise) []error {
	var errs []error
	fail := func(format string, a ...any) { errs = append(errs, fmt.Errorf(format, a...)) }
	if ex.Format != "" || ex.Question != "" || len(ex.Options) > 0 || len(ex.Answer) > 0 || len(ex.Accepted) > 0 {
		fail("una práctica no lleva campos de quiz")
	}
	if strings.TrimSpace(ex.Instructions) == "" {
		fail("falta instructions")
	}
	if strings.TrimSpace(ex.Solution) == "" {
		fail("falta solution")
	}
	if ex.StartDir != "" && !validRel(ex.StartDir) {
		fail("start_dir %q debe ser relativo al home y no salir de él", ex.StartDir)
	}
	if ex.MaxAttempts < 0 || ex.ParChars < 0 {
		fail("max_attempts y par_chars no pueden ser negativos")
	}
	if len(ex.Checks) == 0 {
		fail("una práctica necesita al menos un check")
	}
	patterns := 0
	for i, c := range ex.Checks {
		if c.Type == CheckPattern {
			patterns++
		}
		for _, err := range validateCheck(c) {
			fail("check %d (%s): %w", i+1, c.Type, err)
		}
	}
	if patterns > 0 && patterns < len(ex.Checks) {
		fail("un ejercicio con checks pattern no ejecuta comandos: no puede tener checks de otro tipo")
	}
	if patterns > 0 && (len(ex.Setup) > 0 || ex.Fixture != "") {
		fail("un ejercicio pattern no ejecuta nada: no lleva fixture ni setup")
	}
	if ex.NeedsProcesses() && len(ex.Setup) == 0 {
		fail("un check process necesita un setup que lance el proceso en segundo plano (con &)")
	}
	return errs
}

func validateCheck(c Check) []error {
	var errs []error
	fail := func(format string, a ...any) { errs = append(errs, fmt.Errorf(format, a...)) }
	var paths []string
	for _, l := range [][]string{c.Exists, c.Absent, c.IsDir, c.IsFile, c.Executable} {
		paths = append(paths, l...)
	}
	for p := range c.ContentEquals {
		paths = append(paths, p)
	}
	for p := range c.ContentContains {
		paths = append(paths, p)
	}
	for p := range c.LineCount {
		paths = append(paths, p)
	}
	hasFS := len(paths) > 0
	hasOutput := c.Mode != "" || c.Expected != ""
	if c.Type != CheckPattern && len(c.Accepted) > 0 {
		fail("accepted solo se usa en checks pattern")
	}
	if c.Type != CheckProcess && (c.Process != "" || c.Signal != "") {
		fail("process y signal solo se usan en checks process")
	}

	switch c.Type {
	case CheckFS:
		if !hasFS {
			fail("fs necesita al menos una comprobación")
		}
		for _, p := range paths {
			if !validRel(p) {
				fail("ruta %q debe ser relativa al home y no salir de él", p)
			}
		}
		if hasOutput || c.Code != nil || c.Path != "" {
			fail("fs solo lleva comprobaciones de archivos")
		}
	case CheckOutput:
		switch c.Mode {
		case OutputExact, OutputTrimmed:
		case OutputRegex:
			if _, err := regexp.Compile(c.Expected); err != nil {
				fail("expected no es una regex válida: %v", err)
			}
		case OutputSameAsSolution:
			if c.Expected != "" {
				fail("same_as_solution no lleva expected")
			}
		default:
			fail("mode %q desconocido", c.Mode)
		}
		if c.Mode != OutputSameAsSolution && c.Mode != OutputExact && c.Expected == "" {
			fail("%s necesita expected", c.Mode)
		}
		if hasFS || c.Code != nil || c.Path != "" {
			fail("output solo lleva mode y expected")
		}
	case CheckExitCode:
		if c.Code == nil {
			fail("exit_code necesita code")
		}
		if hasFS || hasOutput || c.Path != "" {
			fail("exit_code solo lleva code")
		}
	case CheckCwd:
		if c.Path == "" || !validRel(c.Path) {
			fail("cwd necesita path relativo al home (usa \".\" para el home)")
		}
		if hasFS || hasOutput || c.Code != nil {
			fail("cwd solo lleva path")
		}
	case CheckProcess:
		if c.Process == "" {
			fail("process necesita el nombre del programa")
		}
		if c.Signal != "" && !slices.Contains(Signals, c.Signal) {
			fail("signal %q desconocida (opciones: %s)", c.Signal, strings.Join(Signals, ", "))
		}
		if hasFS || hasOutput || c.Code != nil || c.Path != "" {
			fail("process solo lleva process y signal")
		}
	case CheckPattern:
		if len(c.Accepted) == 0 {
			fail("pattern necesita al menos una regex en accepted")
		}
		for _, re := range c.Accepted {
			if _, err := regexp.Compile(re); err != nil {
				fail("regex %q inválida: %v", re, err)
			}
		}
		if hasFS || hasOutput || c.Code != nil || c.Path != "" {
			fail("pattern solo lleva accepted")
		}
	default:
		fail("tipo de check desconocido")
	}
	return errs
}

// validRel dice si p es una ruta relativa que no sale del directorio base.
func validRel(p string) bool {
	if p == "" || path.IsAbs(p) || strings.HasPrefix(p, "~") {
		return false
	}
	c := path.Clean(p)
	return c != ".." && !strings.HasPrefix(c, "../")
}

func validateScript(ex Exercise) []error {
	var errs []error
	fail := func(format string, a ...any) { errs = append(errs, fmt.Errorf(format, a...)) }
	if ex.Format != "" || ex.Question != "" || len(ex.Options) > 0 || len(ex.Answer) > 0 || ex.Solution != "" || len(ex.Checks) > 0 {
		fail("un script no lleva campos de quiz ni solution/checks (usa solution_file y tests)")
	}
	if strings.TrimSpace(ex.Instructions) == "" {
		fail("falta instructions")
	}
	if ex.Filename == "" || strings.ContainsAny(ex.Filename, "/ '\"$`") || strings.HasPrefix(ex.Filename, "-") {
		fail("filename %q debe ser un nombre simple, sin rutas ni espacios", ex.Filename)
	}
	if ex.StartDir != "" && !validRel(ex.StartDir) {
		fail("start_dir %q debe ser relativo al home y no salir de él", ex.StartDir)
	}
	if ex.SolutionFile == "" || strings.TrimSpace(ex.SolutionScript) == "" {
		fail("falta solution_file (o está vacío)")
	}
	if ex.RequireShebang && !strings.HasPrefix(ex.SolutionScript, "#!") {
		fail("require_shebang: la solución debe empezar con #!")
	}
	if len(ex.Tests) == 0 {
		fail("un script necesita al menos un test")
	}
	for i, t := range ex.Tests {
		if t.StdoutRegex == "" && t.StdoutContains == "" && t.ExitCode == nil && len(t.Files) == 0 {
			fail("test %d: necesita stdout_regex, stdout_contains, exit_code o files", i+1)
		}
		for p := range t.Files {
			if !validRel(p) || (ex.StartDir != "" && !validRel(path.Join(ex.StartDir, p))) {
				fail("test %d: la ruta %q de files debe ser relativa a start_dir y no salir del home", i+1, p)
			}
		}
		if _, err := regexp.Compile(t.StdoutRegex); err != nil {
			fail("test %d: stdout_regex inválida: %v", i+1, err)
		}
	}
	return errs
}
