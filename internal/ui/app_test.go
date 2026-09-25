package ui

import (
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	files "lsvp-pupil/content"
	"lsvp-pupil/internal/content"
	"lsvp-pupil/internal/scoring"
	"lsvp-pupil/internal/session"
	"lsvp-pupil/internal/storage"
)

func testApp(t *testing.T, dir string) App {
	t.Helper()
	cat, err := content.Load(files.Files)
	if err != nil {
		t.Fatal(err)
	}
	return NewApp(Config{Catalog: cat, Store: storage.Open(dir)})
}

func update(a App, msg tea.Msg) (App, tea.Cmd) {
	m, cmd := a.Update(msg)
	return m.(App), cmd
}

// fakeResults arma un showResultsMsg con el porcentaje pedido en un módulo.
func fakeResults(mode gameMode, module int, percent float64) showResultsMsg {
	return showResultsMsg{title: "x", mode: mode,
		summary: session.Summary{Total: int(percent * 10), Max: 1000, Percent: percent, Rank: scoring.RankFor(percent), Exercises: 5},
		results: []session.Result{{Exercise: content.Exercise{ID: "intro-001", Module: module, Source: "s#a"},
			Outcome: scoring.Outcome{Solved: true, FailedAttempts: 1}}}}
}

func TestUnlockAndPersist(t *testing.T) {
	dir := t.TempDir()
	a := testApp(t, dir)
	if a.unlocked()[4] {
		t.Fatal("el módulo 4 empieza bloqueado")
	}
	a, _ = update(a, fakeResults(gameMode{kind: modeLessons, module: 3}, 3, 40))
	if a.unlocked()[4] {
		t.Error("rango D no desbloquea")
	}
	a, _ = update(a, fakeResults(gameMode{kind: modeLessons, module: 3}, 3, 60))
	if !a.unlocked()[4] {
		t.Fatal("rango C en el módulo 3 desbloquea el 4")
	}
	view := a.results.View()
	if !strings.Contains(view, "Desbloqueaste: Comandos básicos") || !strings.Contains(view, "mejor resultado") {
		t.Errorf("resultados:\n%s", view)
	}

	// Lo guardado sobrevive a reiniciar la app.
	b := testApp(t, dir)
	if !b.unlocked()[4] || !b.progress.Solved["intro-001"] || b.progress.Misses["s#a"] != 2 {
		t.Errorf("progreso leído: %+v", b.progress)
	}
	b, _ = update(b, openLessonsMsg{})
	if !strings.Contains(b.modules.View(), "mejor: C") {
		t.Errorf("la lista de módulos muestra el mejor rango:\n%s", b.modules.View())
	}
}

func TestExamLockedAndScores(t *testing.T) {
	dir := t.TempDir()
	a := testApp(t, dir)
	a, cmd := update(a, startModeMsg{mode: gameMode{kind: modeExam}})
	if cmd != nil || a.screen != screenMenu || a.menu.status != strExamLocked {
		t.Fatalf("el examen empieza bloqueado: pantalla %v status %q", a.screen, a.menu.status)
	}
	a.unlockAll = true
	a, cmd = update(a, startModeMsg{mode: gameMode{kind: modeExam}})
	if cmd == nil || a.screen != screenLesson || len(a.lesson.exercises) != session.ExamSize {
		t.Fatalf("examen: pantalla %v", a.screen)
	}
	a.Close()

	mode := gameMode{kind: modeExam}
	a, _ = update(a, fakeResults(mode, 1, 90))
	if !strings.Contains(a.results.View(), "¡Nuevo récord!") || !strings.Contains(a.results.View(), "rango oficial") {
		t.Errorf("resultados del examen:\n%s", a.results.View())
	}
	a, _ = update(a, fakeResults(mode, 1, 50))
	if !strings.Contains(a.results.View(), "Lugar 2") {
		t.Errorf("segundo lugar:\n%s", a.results.View())
	}
	if a.progress.ExamRank != scoring.RankA {
		t.Errorf("rango oficial = %s", a.progress.ExamRank)
	}
	sc, _ := storage.Open(dir).LoadScores()
	if len(sc["examen"]) != 2 {
		t.Errorf("scores guardados: %+v", sc)
	}
	a, _ = update(a, openLeaderboardMsg{})
	if v := a.leaderboard.View(); !strings.Contains(v, "900") || !strings.Contains(v, "500") {
		t.Errorf("ranking:\n%s", v)
	}
	a, _ = update(a, key("right"))
	if !strings.Contains(a.leaderboard.View(), "Todavía no hay puntajes") {
		t.Error("el quiz rápido todavía no tiene puntajes")
	}
	// Al terminar un modo que no es Lecciones se vuelve al menú.
	a, _ = update(a, fakeResults(mode, 1, 10))
	_, cmd = a.results.Update(key("enter"))
	if cmd() != (backToMenuMsg{}) {
		t.Error("tras el examen se vuelve al menú")
	}
}

func TestQuickQuizUsesUnlockedModules(t *testing.T) {
	a := testApp(t, t.TempDir())
	a, _ = update(a, startModeMsg{mode: gameMode{kind: modeQuickQuiz}})
	defer a.Close()
	if len(a.lesson.exercises) != session.QuickQuizSize {
		t.Fatalf("quiz rápido: %d preguntas", len(a.lesson.exercises))
	}
	for _, ex := range a.lesson.exercises {
		if ex.Kind != content.KindQuiz || ex.Module > 3 {
			t.Errorf("%s (módulo %d, %s) no debería estar en el quiz rápido inicial", ex.ID, ex.Module, ex.Kind)
		}
	}
	if a.lesson.cur.(*quizScreen).ctx.lessons {
		t.Error("fuera de Lecciones no hay pistas ni segundo intento")
	}
}

func TestTimeAttackGlobalDeadline(t *testing.T) {
	clk := &fakeClock{t: time.Unix(0, 0)}
	id := 0
	exs := []content.Exercise{exMC, exTF}
	l := newLesson("Contrarreloj 60 s", gameMode{kind: modeTimeAttack, seconds: 60}, exs, clk.now, rand.New(rand.NewPCG(1, 1)), nil, &id, 80, 24)
	if !strings.Contains(l.View(), "#1") || !strings.Contains(l.View(), "quedan 1:00") {
		t.Errorf("HUD de contrarreloj:\n%s", l.View())
	}
	// La serie se repite: después del último ejercicio sigue el primero.
	for i := 0; i < 3; i++ {
		l.cur.(*quizScreen).finish(true, false)
		l.Update(nextExerciseMsg{})
	}
	if l.idx != 3 || l.cur.(*quizScreen).ex.ID != exTF.ID {
		t.Errorf("idx %d, ejercicio %s", l.idx, l.cur.(*quizScreen).ex.ID)
	}
	if cmd := l.Update(globalTickMsg{run: l.run}); cmd == nil {
		t.Fatal("antes del límite el tick global continúa")
	}
	clk.t = clk.t.Add(61 * time.Second)
	if cmd := l.Update(globalTickMsg{run: l.run + 99}); cmd != nil {
		t.Error("un tick de otra partida se ignora")
	}
	cmd := l.Update(globalTickMsg{run: l.run})
	msg, ok := cmd().(showResultsMsg)
	if !ok || msg.summary.Exercises != 3 || msg.mode.seconds != 60 {
		t.Fatalf("fin de contrarreloj: %+v", msg)
	}
	if l.Update(key("1")) != nil {
		t.Error("después de terminar la partida no se procesan teclas")
	}
}

func TestCorruptProgressWarns(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "progress.json"), []byte("{roto"), 0o644)
	a := testApp(t, dir)
	if !strings.Contains(a.menu.View(), "progress.json.bak") {
		t.Errorf("el menú avisa del respaldo:\n%s", a.menu.View())
	}
	if !a.unlocked()[1] || a.unlocked()[4] {
		t.Error("tras un progreso dañado se empieza de cero")
	}
}

func TestAbout(t *testing.T) {
	cat, err := content.Load(files.Files)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	a := NewApp(Config{Catalog: cat, Store: storage.Open(dir), Backend: "bwrap", Version: "v1.2.3"})
	a, _ = update(a, openAboutMsg{})
	v := a.View()
	for _, want := range []string{"v1.2.3", "LSVP", "UAM Iztapalapa", "https://lsvp-uami.github.io/mdbook-curso-linux-0/", "Apache-2.0", "bwrap", dir} {
		if !strings.Contains(v, want) {
			t.Errorf("«Acerca de» no menciona %q:\n%s", want, v)
		}
	}
	if _, cmd := update(a, key("esc")); cmd == nil || cmd() != (backToMenuMsg{}) {
		t.Error("esc vuelve al menú")
	}
}
