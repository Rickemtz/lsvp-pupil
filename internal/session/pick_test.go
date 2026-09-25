package session

import (
	"fmt"
	"math/rand/v2"
	"testing"

	"lsvp-pupil/internal/content"
)

func pool(quiz, practice, scripts int) []content.Exercise {
	var exs []content.Exercise
	add := func(kind string, n int) {
		for i := range n {
			exs = append(exs, content.Exercise{ID: fmt.Sprintf("%s-%d", kind, i), Kind: kind})
		}
	}
	add(content.KindQuiz, quiz)
	add(content.KindPractice, practice)
	add(content.KindScript, scripts)
	return exs
}

func count(exs []content.Exercise) map[string]int {
	c := map[string]int{}
	seen := map[string]bool{}
	for _, e := range exs {
		c[e.Kind]++
		if seen[e.ID] {
			c["repetidos"]++
		}
		seen[e.ID] = true
	}
	return c
}

func TestPickQuickQuiz(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	if c := count(PickQuickQuiz(pool(50, 30, 5), r)); c[content.KindQuiz] != 20 || c[content.KindPractice] != 0 || c["repetidos"] != 0 {
		t.Errorf("quiz rápido: %v", c)
	}
	if got := PickQuickQuiz(pool(7, 30, 0), r); len(got) != 7 {
		t.Errorf("con pocas preguntas se usan todas: %d", len(got))
	}
}

func TestPickExam(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	c := count(PickExam(pool(60, 40, 12), r))
	if c[content.KindQuiz] != 10 || c[content.KindPractice]+c[content.KindScript] != 15 || c[content.KindScript] > 2 || c["repetidos"] != 0 {
		t.Errorf("examen: %v", c)
	}
	// Con poca práctica se completa con teoría, y al revés.
	if c := count(PickExam(pool(60, 5, 0), r)); c[content.KindPractice] != 5 || c[content.KindQuiz] != 20 {
		t.Errorf("poca práctica: %v", c)
	}
	if c := count(PickExam(pool(3, 40, 0), r)); c[content.KindQuiz] != 3 || c[content.KindPractice] != 22 {
		t.Errorf("poca teoría: %v", c)
	}
	if got := PickExam(nil, r); len(got) != 0 {
		t.Errorf("sin ejercicios: %d", len(got))
	}
}

func TestTimeAttackPool(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	if c := count(TimeAttackPool(pool(10, 10, 5), r)); c[content.KindScript] != 0 || c[content.KindQuiz]+c[content.KindPractice] != 20 {
		t.Errorf("contrarreloj: %v", c)
	}
}
