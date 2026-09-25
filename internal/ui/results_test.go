package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"testing"
	"time"

	"lsvp-pupil/internal/content"
	"lsvp-pupil/internal/scoring"
	"lsvp-pupil/internal/session"
)

func TestSectionName(t *testing.T) {
	tests := map[string]string{
		"https://x/a.html#breve-historia-del-so-gnulinux": "Breve historia del so gnulinux",
		"https://x/a.html#_tipos_de_archivos":             "Tipos de archivos",
		"https://x/a.html#Árbol-del-sistema":              "Árbol del sistema",
		"https://x/a.html#%C3%A1rbol":                     "Árbol",
		"https://x/a.html":                                "",
		"https://x/a.html#-":                              "",
	}
	for in, want := range tests {
		if got := sectionName(in); got != want {
			t.Errorf("sectionName(%q) = %q, quiero %q", in, got, want)
		}
	}
}

func TestBar(t *testing.T) {
	count := func(s, r string) int { return strings.Count(s, r) }
	for _, tt := range []struct {
		pct    float64
		filled int
	}{{0, 0}, {50, 10}, {100, 20}, {150, 20}, {-5, 0}} {
		b := bar(tt.pct)
		if count(b, "█") != tt.filled || count(b, "█")+count(b, "░") != barWidth {
			t.Errorf("bar(%v) = %q", tt.pct, b)
		}
	}
}

func TestFormatDuration(t *testing.T) {
	if got := formatDuration(252 * time.Second); got != "4:12" {
		t.Errorf("formatDuration = %q", got)
	}
	if got := formatDuration(59600 * time.Millisecond); got != "1:00" {
		t.Errorf("formatDuration redondeado = %q", got)
	}
}

func TestResultsView(t *testing.T) {
	cat := &content.Catalog{Modules: []content.Module{{ID: 1, Title: "Introducción"}}}
	s := session.New(time.Now)
	ok := scoring.Outcome{Kind: scoring.Theory, Difficulty: 1, TimeLimit: time.Second, Solved: true}
	s.Record(content.Exercise{Module: 1, Source: "https://x/a.html#kernel"}, ok)
	s.Record(content.Exercise{Module: 1, Source: "https://x/a.html#kernel"}, scoring.Outcome{Kind: scoring.Theory, Difficulty: 1})
	v := resultsModel{title: "Introducción", summary: s.Summary(), catalog: cat}.View()
	for _, want := range []string{"Rango", "C", "200 de 400", "1 de 2 al primer intento", "1. Introducción", "Kernel", "1 fallo"} {
		if !strings.Contains(v, want) {
			t.Errorf("la vista no contiene %q:\n%s", want, v)
		}
	}
}

func TestResultsScroll(t *testing.T) {
	cat := &content.Catalog{Modules: []content.Module{{ID: 1, Title: "Uno"}, {ID: 2, Title: "Dos"}, {ID: 3, Title: "Tres"}, {ID: 4, Title: "Cuatro"}}}
	s := session.New(time.Now)
	miss := scoring.Outcome{Kind: scoring.Theory, Difficulty: 1}
	for m := 1; m <= 4; m++ {
		for i := 0; i < 3; i++ {
			s.Record(content.Exercise{Module: m, Source: "https://lsvp-uami.github.io/mdbook-curso-linux-0/dia_1/03_sistemas_archivos.html#tema-" + string(rune('a'+i))}, miss)
		}
	}
	r := resultsModel{title: "Examen", summary: s.Summary(), catalog: cat, width: 80, height: 24,
		notes: []string{"nota 1", "nota 2"}}
	v := r.View()
	if n := strings.Count(v, "\n") + 1; n > 22 {
		t.Fatalf("la vista tiene %d líneas; con el marco no cabe en 24:\n%s", n, v)
	}
	if !strings.Contains(v, "Resultados · Examen") || !strings.Contains(v, "ver más") {
		t.Errorf("arriba se ve el título y el indicador:\n%s", v)
	}
	for i := 0; i < 50; i++ {
		r, _ = r.Update(key("down"))
	}
	if v := r.View(); !strings.Contains(v, "enter: volver") || strings.Contains(v, "Resultados · Examen") {
		t.Errorf("al bajar se llega al final:\n%s", v)
	}
	r, _ = r.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	r, _ = r.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	if r.offset != 0 {
		t.Errorf("PgUp regresa al inicio: offset %d", r.offset)
	}
}
