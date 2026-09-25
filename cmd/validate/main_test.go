package main

import (
	"os/exec"
	"strings"
	"testing"

	"lsvp-pupil/internal/content"
	"lsvp-pupil/internal/sandbox"
)

func TestValidatePattern(t *testing.T) {
	ex := content.Exercise{ID: "p", Kind: content.KindPractice, Solution: "sudo apt install htop",
		Checks: []content.Check{{Type: content.CheckPattern, Accepted: []string{`(sudo )?apt install htop`}}}}
	if err := validatePractice(sandbox.BackendDir, ex); err != nil {
		t.Errorf("válido: %v", err)
	}
	ex.Solution = "apt-get install htop"
	if err := validatePractice(sandbox.BackendDir, ex); err == nil || !strings.Contains(err.Error(), "no coincide") {
		t.Errorf("solución que no coincide: %v", err)
	}
	ex.Solution, ex.Checks[0].Accepted = "x", []string{`x?`}
	if err := validatePractice(sandbox.BackendDir, ex); err == nil || !strings.Contains(err.Error(), "vacío") {
		t.Errorf("patrón que acepta vacío: %v", err)
	}
}

func TestValidatePractice(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash no está instalado")
	}
	base := content.Exercise{ID: "x", Kind: content.KindPractice, Fixture: "taller", StartDir: "taller",
		Checks:   []content.Check{{Type: content.CheckFS, IsDir: []string{"taller/nuevo"}}},
		Solution: "mkdir nuevo"}
	tests := []struct {
		name string
		mod  func(*content.Exercise)
		want string // "" = válido
	}{
		{"válido", func(*content.Exercise) {}, ""},
		{"solución equivocada", func(e *content.Exercise) { e.Solution = "touch nuevo" }, "no pasa sus checks"},
		{"ya resuelto", func(e *content.Exercise) { e.Checks[0].IsDir = []string{"taller/dia-1"} }, "ya pasan"},
		{"bloqueada por guard", func(e *content.Exercise) { e.Solution = "mkdir /tmp/x" }, "guard"},
		{"setup roto", func(e *content.Exercise) { e.Setup = []string{"false"} }, "crear sandbox"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ex := base
			ex.Checks = []content.Check{{Type: content.CheckFS, IsDir: []string{"taller/nuevo"}}}
			tt.mod(&ex)
			err := validatePractice(sandbox.BackendDir, ex)
			switch {
			case tt.want == "" && err != nil:
				t.Errorf("error inesperado: %v", err)
			case tt.want != "" && (err == nil || !strings.Contains(err.Error(), tt.want)):
				t.Errorf("error = %v, quiero que contenga %q", err, tt.want)
			}
		})
	}
}
