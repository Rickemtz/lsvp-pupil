// Package quiz evalúa las respuestas de las preguntas teóricas. Funciones puras, sin I/O.
package quiz

import (
	"fmt"
	"math/rand/v2"
	"strings"

	"lsvp-pupil/internal/content"
)

// Normalize prepara una respuesta para compararla: sin mayúsculas ni espacios extremos.
func Normalize(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// Choices devuelve las opciones de la pregunta; en true_false, los valores canónicos.
func Choices(ex content.Exercise) []string {
	if ex.Format == content.FormatTrueFalse {
		return []string{content.True, content.False}
	}
	return ex.Options
}

// Check dice si given responde correctamente a ex.
// multiple_choice, true_false y short_answer esperan un solo elemento; multi_select, el conjunto elegido.
func Check(ex content.Exercise, given []string) (bool, error) {
	switch ex.Format {
	case content.FormatMultipleChoice, content.FormatTrueFalse:
		if len(given) != 1 {
			return false, fmt.Errorf("%s: se esperaba una respuesta, llegaron %d", ex.ID, len(given))
		}
		return Normalize(given[0]) == Normalize(ex.Answer[0]), nil
	case content.FormatShortAnswer:
		if len(given) != 1 {
			return false, fmt.Errorf("%s: se esperaba una respuesta, llegaron %d", ex.ID, len(given))
		}
		g := Normalize(given[0])
		for _, a := range append([]string{ex.Answer[0]}, ex.Accepted...) {
			if g == Normalize(a) {
				return true, nil
			}
		}
		return false, nil
	case content.FormatMultiSelect:
		return sameSet(given, ex.Answer), nil
	}
	return false, fmt.Errorf("%s: format %q desconocido", ex.ID, ex.Format)
}

func sameSet(a, b []string) bool {
	set := map[string]bool{}
	for _, s := range b {
		set[Normalize(s)] = true
	}
	got := map[string]bool{}
	for _, s := range a {
		n := Normalize(s)
		if !set[n] {
			return false
		}
		got[n] = true
	}
	return len(got) == len(set)
}

// Shuffle devuelve una copia de las opciones en orden aleatorio.
// true_false conserva el orden verdadero/falso.
func Shuffle(ex content.Exercise, r *rand.Rand) []string {
	c := append([]string(nil), Choices(ex)...)
	if ex.Format != content.FormatTrueFalse {
		r.Shuffle(len(c), func(i, j int) { c[i], c[j] = c[j], c[i] })
	}
	return c
}
