package main

import (
	"os"
	"os/exec"
	"path"
	"strings"
	"testing"

	files "lsvp-pupil/content"
	"lsvp-pupil/fixtures"
	"lsvp-pupil/internal/checks"
	"lsvp-pupil/internal/content"
	"lsvp-pupil/internal/sandbox"
)

// Scripts correctos escritos de otra forma. Si alguno deja de pasar, los tests son demasiado estrictos.
var alternativeScripts = map[string][]string{
	"scr-01": {"#!/bin/bash\nprintf '¡Hola, mundo!\\n'\n"},
	"scr-03": {"#!/bin/bash\necho \"Introduce tu nombre:\"\nread nombre\necho \"Edad:\"\nread edad\necho \"Carrera:\"\nread carrera\necho \"¡Hola $nombre, tienes $edad años y estudias $carrera, saludos!\"\n"},
	"scr-04": {"#!/bin/bash\nhostname\necho $USER\ndate +%F\npwd\n"},
	"scr-05": {"#!/bin/bash\necho $0\necho $1\necho $2\necho $#\necho $*\n"},
	"scr-06": {"#!/bin/bash\nif (( $1 > 10 )); then echo mayor; elif (( $1 < 10 )); then echo menor; else echo igual; fi\n"},
	"scr-07": {"#!/bin/bash\nif [ -e \"$1\" ]\nthen\n  ls -l $1\nelse\n  echo \"El archivo $1 no existe\"\nfi\n"},
	"scr-08": {"#!/bin/bash\nfor ((i = $1; i >= 0; i--)); do echo $i; done\n"},
	"scr-09": {"#!/bin/bash\nfor i in {1..10}; do echo \"$1 x $i = $(( $1 * $i ))\"; done\n"},
	"scr-10": {"#!/bin/bash\ns=0\nfor ((i=1;i<=$1;i++)); do s=$((s+i)); done\necho \"La suma es $s\"\n"},
	"scr-11": {"#!/bin/bash\nls $1 &> /dev/null\ncodigo=$?\necho \"Código: $codigo\"\n"},
	"scr-13": {"#!/bin/bash\nif [ ! -e \"$1\" ]; then\n  echo \"no existe\" >&2\n  exit 1\nfi\ncat \"$1\" | wc -l > salida.txt\n"},
	"scr-14": {"#!/bin/bash\n[ $# -eq 0 ] && { echo \"Uso: validar_args.sh ARCHIVO\"; exit 1; }\necho \"Recibí: $1\"\n"},
	"scr-12": {"#!/bin/bash\nif ! cat \"$1\" 2>> error.log; then\n  echo \"Error al mostrar el archivo\"\nfi\n"},
}

func TestAlternativeScripts(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash no está instalado")
	}
	backend, err := sandbox.Resolve(sandbox.BackendAuto)
	if err != nil {
		t.Fatal(err)
	}
	cat, err := content.Load(files.Files)
	if err != nil {
		t.Fatal(err)
	}
	for _, ex := range cat.Exercises(9) {
		for i, script := range alternativeScripts[ex.ID] {
			t.Run(ex.ID, func(t *testing.T) {
				sb, err := sandbox.New(backend, sandbox.Options{Fixtures: fixtures.Files, Fixture: ex.Fixture, StartDir: ex.StartDir})
				if err != nil {
					t.Fatal(err)
				}
				defer sb.Close()
				if err := os.WriteFile(sb.HostPath(path.Join(ex.StartDir, ex.Filename)), []byte(script), 0o644); err != nil {
					t.Fatal(err)
				}
				rs, err := checks.RunScript(ex, script, sandboxRunner(sb), sandboxReader(sb, ex.StartDir))
				if err != nil {
					t.Fatal(err)
				}
				for j, r := range rs {
					if !r.Passed() {
						t.Errorf("alternativa %d, test %d: %s\nsalida: %q", i+1, j+1, strings.Join(r.Fails, "; "), r.Stdout)
					}
				}
			})
		}
	}
}

func TestScriptWithoutShebangDoesNotRun(t *testing.T) {
	ex := content.Exercise{Filename: "a.sh", RequireShebang: true, Tests: []content.ScriptTest{{StdoutContains: "x"}}}
	ran := false
	rs, err := checks.RunScript(ex, "echo x\n", func(string) (string, string, int, bool, error) { ran = true; return "x", "", 0, false, nil }, nil)
	if err != nil || ran || checks.AllPassed(rs) {
		t.Errorf("sin shebang no debe correr ni pasar: ran=%v rs=%v err=%v", ran, rs, err)
	}
}

// Errores típicos que los tests deben detectar.
var wrongScripts = map[string][]string{
	"scr-13": {
		"#!/bin/bash\nwc -l < \"$1\" >> salida.txt\n",                                                // >> acumula en vez de reemplazar
		"#!/bin/bash\nif [ -f \"$1\" ]; then wc -l \"$1\" > salida.txt; else exit 1; fi\n",           // incluye el nombre del archivo
		"#!/bin/bash\nif [ -f \"$1\" ]; then wc -l < \"$1\" > salida.txt; else echo no existe; fi\n", // sin exit 1
	},
	"scr-12": {"#!/bin/bash\ncat \"$1\" || echo \"Error al mostrar el archivo\"\n"}, // el error no va a error.log
	"scr-14": {"#!/bin/bash\nif [ $# -eq 0 ]; then echo \"Uso: validar_args.sh ARCHIVO\"; fi\necho \"Recibí: $1\"\n"},
}

func TestWrongScriptsFail(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash no está instalado")
	}
	backend, err := sandbox.Resolve(sandbox.BackendAuto)
	if err != nil {
		t.Fatal(err)
	}
	cat, err := content.Load(files.Files)
	if err != nil {
		t.Fatal(err)
	}
	for _, ex := range cat.Exercises(9) {
		for i, script := range wrongScripts[ex.ID] {
			sb, err := sandbox.New(backend, sandbox.Options{Fixtures: fixtures.Files, Fixture: ex.Fixture, StartDir: ex.StartDir})
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(sb.HostPath(path.Join(ex.StartDir, ex.Filename)), []byte(script), 0o644); err != nil {
				t.Fatal(err)
			}
			rs, err := checks.RunScript(ex, script, sandboxRunner(sb), sandboxReader(sb, ex.StartDir))
			sb.Close()
			if err != nil {
				t.Fatal(err)
			}
			if checks.AllPassed(rs) {
				t.Errorf("%s: el script incorrecto %d pasó todos los tests:\n%s", ex.ID, i+1, script)
			}
		}
	}
}
