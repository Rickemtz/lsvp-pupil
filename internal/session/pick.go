package session

import (
	"math/rand/v2"

	"lsvp-pupil/internal/content"
)

// Tamaños de los modos (CLAUDE.md).
const (
	QuickQuizSize  = 20
	ExamSize       = 25
	ExamTheory     = 10 // ≈40 % teoría
	ExamMaxScripts = 2  // los scripts tardan minutos: pocos en el examen
)

// TimeAttackDurations son las duraciones de Contrarreloj, en segundos.
var TimeAttackDurations = []int{60, 120, 300}

func shuffled(exs []content.Exercise, r *rand.Rand) []content.Exercise {
	out := append([]content.Exercise(nil), exs...)
	r.Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
	return out
}

func filter(exs []content.Exercise, keep func(content.Exercise) bool) []content.Exercise {
	var out []content.Exercise
	for _, ex := range exs {
		if keep(ex) {
			out = append(out, ex)
		}
	}
	return out
}

// PickQuickQuiz elige hasta QuickQuizSize preguntas teóricas al azar.
func PickQuickQuiz(exs []content.Exercise, r *rand.Rand) []content.Exercise {
	quiz := shuffled(filter(exs, func(e content.Exercise) bool { return e.Kind == content.KindQuiz }), r)
	return quiz[:min(QuickQuizSize, len(quiz))]
}

// PickExam elige ExamSize ejercicios: ExamTheory de teoría y el resto de práctica (con a lo más
// ExamMaxScripts scripts), en orden aleatorio. Si faltan de un tipo, completa con el otro.
func PickExam(exs []content.Exercise, r *rand.Rand) []content.Exercise {
	quiz := shuffled(filter(exs, func(e content.Exercise) bool { return e.Kind == content.KindQuiz }), r)
	var practice, scripts []content.Exercise
	for _, e := range shuffled(exs, r) {
		switch e.Kind {
		case content.KindPractice:
			practice = append(practice, e)
		case content.KindScript:
			scripts = append(scripts, e)
		}
	}
	scripts = scripts[:min(ExamMaxScripts, len(scripts))]
	rest := shuffled(append(practice, scripts...), r)

	nTheory := min(ExamTheory, len(quiz))
	nPractice := min(ExamSize-nTheory, len(rest))
	nTheory = min(ExamSize-nPractice, len(quiz)) // si faltó práctica, más teoría
	return shuffled(append(append([]content.Exercise(nil), quiz[:nTheory]...), rest[:nPractice]...), r)
}

// TimeAttackPool devuelve los ejercicios de Contrarreloj en orden aleatorio, sin scripts.
func TimeAttackPool(exs []content.Exercise, r *rand.Rand) []content.Exercise {
	return shuffled(filter(exs, func(e content.Exercise) bool { return e.Kind != content.KindScript }), r)
}
