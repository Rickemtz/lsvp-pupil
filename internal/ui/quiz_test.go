package ui

import (
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"lsvp-pupil/internal/content"
	"lsvp-pupil/internal/scoring"
	"lsvp-pupil/internal/session"
)

type fakeClock struct{ t time.Time }

func (c *fakeClock) now() time.Time { return c.t }

var (
	exMC = content.Exercise{ID: "mc", Kind: content.KindQuiz, Format: content.FormatMultipleChoice, Options: []string{"a", "b", "c"},
		Answer: content.StringList{"b"}, TimeLimitSec: 10, Hints: []string{"h1", "h2"}, Explanation: "e", Difficulty: 1}
	exTF = content.Exercise{ID: "tf", Kind: content.KindQuiz, Format: content.FormatTrueFalse, Answer: content.StringList{content.True},
		TimeLimitSec: 10, Explanation: "e", Difficulty: 1}
	exSA = content.Exercise{ID: "sa", Kind: content.KindQuiz, Format: content.FormatShortAnswer, Answer: content.StringList{"bash"},
		TimeLimitSec: 10, Explanation: "e", Difficulty: 1}
	exMS = content.Exercise{ID: "ms", Kind: content.KindQuiz, Format: content.FormatMultiSelect, Options: []string{"a", "b", "c"},
		Answer: content.StringList{"a", "c"}, TimeLimitSec: 10, Explanation: "e", Difficulty: 1}
)

func testCtx(lessons bool) (exerciseCtx, *fakeClock) {
	clk := &fakeClock{t: time.Unix(0, 0)}
	return exerciseCtx{id: 1, title: "Test", total: 1, lessons: lessons, session: session.New(clk.now),
		now: clk.now, rng: rand.New(rand.NewPCG(1, 1))}, clk
}

func newTestQuiz(lessons bool, ex content.Exercise) (*quizScreen, *fakeClock) {
	ctx, clk := testCtx(lessons)
	return newQuiz(ctx, ex), clk
}

func press(s exerciseScreen, keys ...string) tea.Cmd {
	var cmd tea.Cmd
	for _, k := range keys {
		cmd = s.Update(key(k))
	}
	return cmd
}

// choose mueve el cursor a la opción con ese texto (las opciones se barajan).
func choose(t *testing.T, q *quizScreen, text string) {
	t.Helper()
	i := slices.Index(q.choices, text)
	if i < 0 {
		t.Fatalf("no hay opción %q en %v", text, q.choices)
	}
	press(q, string(rune('1'+i)))
}

func res(q *quizScreen) scoring.Outcome { return q.last.Outcome }

func TestQuizCorrectFirstTry(t *testing.T) {
	q, clk := newTestQuiz(true, exMC)
	clk.t = clk.t.Add(3 * time.Second)
	choose(t, q, "b")
	press(q, "enter")
	if !q.answered || !res(q).Solved || res(q).FailedAttempts != 0 || res(q).TimeUsed != 3*time.Second {
		t.Fatalf("resultado %+v", res(q))
	}
	cmd := press(q, "enter")
	if _, ok := cmd().(nextExerciseMsg); !ok {
		t.Errorf("enter tras responder debe pedir el siguiente ejercicio, llegó %T", cmd())
	}
}

func TestQuizTrueFalse(t *testing.T) {
	q, _ := newTestQuiz(false, exTF)
	press(q, "1", "enter") // Verdadero va primero
	if !res(q).Solved {
		t.Fatal("Verdadero debería ser correcto")
	}
}

func TestQuizLessonsSecondAttempt(t *testing.T) {
	q, _ := newTestQuiz(true, exMC)
	choose(t, q, "a")
	press(q, "enter")
	if q.answered || q.notice == "" {
		t.Fatal("en Lecciones el primer fallo debe permitir otro intento")
	}
	choose(t, q, "b")
	press(q, "enter")
	if r := res(q); !r.Solved || r.FailedAttempts != 1 {
		t.Errorf("resultado %+v, quiero correcto con 1 fallo", r)
	}
}

func TestQuizSingleAttemptOutsideLessons(t *testing.T) {
	q, _ := newTestQuiz(false, exMC)
	choose(t, q, "a")
	press(q, "enter")
	if !q.answered || res(q).Solved {
		t.Fatal("fuera de Lecciones solo hay un intento")
	}
}

func TestQuizHints(t *testing.T) {
	q, _ := newTestQuiz(true, exMC)
	press(q, "f1", "f1", "f1")
	if q.hints != 2 {
		t.Errorf("hints = %d, quiero 2 (no hay más pistas)", q.hints)
	}
	q, _ = newTestQuiz(false, exMC)
	if press(q, "f1"); q.hints != 0 {
		t.Error("fuera de Lecciones no hay pistas")
	}
}

func TestQuizShortAnswer(t *testing.T) {
	q, _ := newTestQuiz(true, exSA)
	press(q, "enter")
	if q.answered || q.failed != 0 {
		t.Fatal("una respuesta vacía no cuenta como intento")
	}
	q.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" BASH ")})
	press(q, "enter")
	if !q.answered || !res(q).Solved {
		t.Fatalf("resultado %+v", res(q))
	}
}

func TestQuizMultiSelect(t *testing.T) {
	q, _ := newTestQuiz(false, exMS)
	press(q, "enter")
	if q.answered {
		t.Fatal("enviar sin marcar nada no cuenta como intento")
	}
	choose(t, q, "a")
	choose(t, q, "c")
	press(q, "enter")
	if !res(q).Solved {
		t.Errorf("a y c marcadas deberían ser correctas")
	}
}

func TestQuizTimeout(t *testing.T) {
	q, clk := newTestQuiz(true, exMC)
	clk.t = clk.t.Add(9 * time.Second)
	if cmd := q.Update(tickMsg{id: 1}); cmd == nil || q.answered {
		t.Fatal("con tiempo restante el tick debe seguir")
	}
	if !strings.Contains(renderTimer(q.remaining, q.limit()), "1s") {
		t.Errorf("temporizador = %v", q.remaining)
	}
	clk.t = clk.t.Add(2 * time.Second)
	q.Update(tickMsg{id: 99}) // tick de otra pantalla
	if q.answered {
		t.Fatal("un tick de otra pantalla no debe afectar")
	}
	q.Update(tickMsg{id: 1})
	if !q.answered || !q.timedOut || res(q).Solved {
		t.Fatal("debería agotarse el tiempo")
	}
	if n := len(q.ctx.session.Results()); n != 1 {
		t.Errorf("resultados registrados = %d", n)
	}
}

func TestQuizScoring(t *testing.T) {
	q, _ := newTestQuiz(true, exMC)
	choose(t, q, "b")
	press(q, "enter") // tiempo 0, primer intento: 100 + 50 + 50
	if q.last.Points != 200 || q.ctx.session.Combo() != 1.1 {
		t.Fatalf("puntos %d, combo %v", q.last.Points, q.ctx.session.Combo())
	}
	if !strings.Contains(q.View(), "+200 pts") || !strings.Contains(q.View(), "200 pts") {
		t.Errorf("la vista debería mostrar los puntos:\n%s", q.View())
	}
}

func TestQuizEscGoesBack(t *testing.T) {
	q, _ := newTestQuiz(true, exMC)
	cmd := q.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if _, ok := cmd().(backToLessonMsg); !ok {
		t.Errorf("esc devolvió %T", cmd())
	}
}

func TestLessonRunner(t *testing.T) {
	clk := &fakeClock{t: time.Unix(0, 0)}
	id := 0
	l := newLesson("Test", gameMode{kind: modeLessons, module: 1}, []content.Exercise{exMC, exTF}, clk.now, rand.New(rand.NewPCG(1, 1)), nil, &id, 80, 24)
	q := l.cur.(*quizScreen)
	choose(t, q, "b")
	l.Update(key("enter"))
	cmd := l.Update(key("enter"))
	if msg := cmd(); msg != (nextExerciseMsg{}) {
		t.Fatalf("quiero nextExerciseMsg, llegó %T", msg)
	}
	l.Update(nextExerciseMsg{})
	q2, ok := l.cur.(*quizScreen)
	if !ok || q2.ex.ID != "tf" || q2.ctx.id == q.ctx.id || q2.ctx.index != 1 {
		t.Fatalf("segunda pantalla: %+v", l.cur)
	}
	l.Update(key("1"))
	l.Update(key("enter"))
	cmd = l.Update(nextExerciseMsg{})
	msg, ok := cmd().(showResultsMsg)
	if !ok || msg.summary.Exercises != 2 || msg.summary.FirstTry != 2 {
		t.Fatalf("resultados: %+v", msg)
	}
}

func TestDisplayAnswer(t *testing.T) {
	if got := displayAnswer(exTF); got != strQuizTrue {
		t.Errorf("displayAnswer(tf) = %q", got)
	}
	if got := displayAnswer(exMS); got != "a, c" {
		t.Errorf("displayAnswer(ms) = %q", got)
	}
}

func TestHardWrap(t *testing.T) {
	tests := []struct {
		in    string
		width int
		want  string
	}{
		{"abcdef", 10, "abcdef"},
		{"abcdef", 3, "abc\ndef"},
		{"abcdefg", 3, "abc\ndef\ng"},
		{"añoñoñ", 2, "añ\noñ\noñ"},
	}
	for _, tt := range tests {
		if got := hardWrap(tt.in, tt.width); got != tt.want {
			t.Errorf("hardWrap(%q, %d) = %q, quiero %q", tt.in, tt.width, got, tt.want)
		}
	}
}

func TestReflow(t *testing.T) {
	tests := map[string]string{
		"una\nsola línea\n":           "una sola línea",
		"párrafo uno\n\npárrafo\ndos": "párrafo uno\n\npárrafo dos",
		"  espacios   de   más ":      "espacios de más",
	}
	for in, want := range tests {
		if got := reflow(in); got != want {
			t.Errorf("reflow(%q) = %q, quiero %q", in, got, want)
		}
	}
}
