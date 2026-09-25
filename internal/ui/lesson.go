package ui

import (
	"math/rand/v2"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"lsvp-pupil/internal/content"
	"lsvp-pupil/internal/session"
)

// globalTickMsg revisa el tiempo total de Contrarreloj.
type globalTickMsg struct{ run int }

// lessonModel lleva una partida: una serie de ejercicios de cualquier tipo con una sola sesión.
// En Contrarreloj la serie se repite hasta que se acaba el tiempo total.
type lessonModel struct {
	title         string
	mode          gameMode
	exercises     []content.Exercise
	session       *session.Session
	now           func() time.Time
	rng           *rand.Rand
	newSandbox    sandboxFactory
	nextID        *int // contador compartido de ids de pantalla
	run           int  // id de esta partida, para sus ticks globales
	deadline      time.Time
	idx           int
	done          bool
	cur           exerciseScreen
	width, height int
}

func newLesson(title string, mode gameMode, exs []content.Exercise, now func() time.Time, rng *rand.Rand,
	newSandbox sandboxFactory, nextID *int, width, height int) *lessonModel {
	*nextID++
	l := &lessonModel{title: title, mode: mode, exercises: exs, session: session.New(now), now: now,
		rng: rng, newSandbox: newSandbox, nextID: nextID, run: *nextID, width: width, height: height}
	if mode.kind == modeTimeAttack {
		l.deadline = now().Add(mode.duration())
	}
	l.cur = l.screen()
	return l
}

func (l *lessonModel) screen() exerciseScreen {
	*l.nextID++
	total := len(l.exercises)
	if l.mode.kind == modeTimeAttack {
		total = 0 // no hay un número fijo de ejercicios
	}
	ctx := exerciseCtx{id: *l.nextID, title: l.title, index: l.idx, total: total, lessons: l.mode.kind == modeLessons,
		session: l.session, deadline: l.deadline, now: l.now, rng: l.rng, newSandbox: l.newSandbox}
	var s exerciseScreen
	switch ex := l.exercises[l.idx%len(l.exercises)]; ex.Kind {
	case content.KindPractice:
		s = newPractice(ctx, ex)
	case content.KindScript:
		s = newEditor(ctx, ex)
	default:
		s = newQuiz(ctx, ex)
	}
	s.SetSize(l.width, l.height)
	return s
}

func (l *lessonModel) Init() tea.Cmd {
	if l.deadline.IsZero() {
		return l.cur.Init()
	}
	return tea.Batch(l.cur.Init(), l.globalTick())
}

func (l *lessonModel) globalTick() tea.Cmd {
	run := l.run
	return tea.Tick(tickEvery, func(time.Time) tea.Msg { return globalTickMsg{run: run} })
}

func (l *lessonModel) finish() tea.Cmd {
	l.done = true
	l.cur.Close()
	return send(showResultsMsg{title: l.title, mode: l.mode, summary: l.session.Summary(), results: l.session.Results()})
}

func (l *lessonModel) Update(msg tea.Msg) tea.Cmd {
	if l.done {
		return nil
	}
	switch msg := msg.(type) {
	case globalTickMsg:
		if msg.run != l.run {
			return nil
		}
		if !l.now().Before(l.deadline) {
			return l.finish() // el ejercicio en curso no cuenta
		}
		return l.globalTick()
	case nextExerciseMsg:
		l.cur.Close()
		if l.mode.kind != modeTimeAttack && l.idx+1 == len(l.exercises) {
			return l.finish()
		}
		l.idx++
		l.cur = l.screen()
		return l.cur.Init()
	}
	return l.cur.Update(msg)
}

func (l *lessonModel) View() string { return l.cur.View() }

func (l *lessonModel) SetSize(w, h int) {
	l.width, l.height = w, h
	l.cur.SetSize(w, h)
}

// Close libera el sandbox del ejercicio actual. Se puede llamar varias veces.
func (l *lessonModel) Close() { l.cur.Close() }
