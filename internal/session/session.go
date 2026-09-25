// Package session orquesta una partida: registra resultados, lleva el combo y arma el resumen final.
package session

import (
	"sort"
	"time"

	"lsvp-pupil/internal/content"
	"lsvp-pupil/internal/scoring"
)

// maxReview es cuántos temas a repasar se muestran al final.
const maxReview = 3

// Result es el resultado registrado de un ejercicio.
type Result struct {
	Exercise content.Exercise
	Outcome  scoring.Outcome
	Points   int
	Max      int // máximo sin combo
	Combo    float64
}

// Session es una partida en curso.
type Session struct {
	now      func() time.Time
	started  time.Time
	combo    scoring.Combo
	maxCombo float64
	guarded  bool
	results  []Result
}

// New empieza una partida.
func New(now func() time.Time) *Session {
	return &Session{now: now, started: now(), maxCombo: 1}
}

// Combo devuelve el multiplicador actual.
func (s *Session) Combo() float64 { return s.combo.Multiplier() }

// Total devuelve los puntos acumulados.
func (s *Session) Total() int {
	t := 0
	for _, r := range s.results {
		t += r.Points
	}
	return t
}

// Record registra un ejercicio. Los puntos usan el combo vigente antes del ejercicio;
// después el combo sube o se reinicia. La penalización de guard solo cuenta la primera vez.
func (s *Session) Record(ex content.Exercise, o scoring.Outcome) Result {
	if o.GuardPenalty {
		o.GuardPenalty = !s.guarded
		s.guarded = true
	}
	r := Result{
		Exercise: ex,
		Outcome:  o,
		Points:   scoring.Points(o, s.combo),
		Max:      int(scoring.MaxPoints(o.Kind, o.Difficulty)),
		Combo:    s.combo.Multiplier(),
	}
	s.combo = s.combo.Next(o)
	s.maxCombo = max(s.maxCombo, s.combo.Multiplier())
	s.results = append(s.results, r)
	return r
}

// Results devuelve los resultados registrados.
func (s *Session) Results() []Result { return s.results }

// ModuleSummary es el desglose de un módulo.
type ModuleSummary struct {
	Module  int
	Points  int
	Max     int
	Percent float64
}

// ReviewTopic es una sección del curso donde hubo fallos.
type ReviewTopic struct {
	Module int
	Source string
	Misses int
}

// Summary es el resumen final de la partida.
type Summary struct {
	Total     int
	Max       int
	Percent   float64
	Rank      scoring.Rank
	Duration  time.Duration
	Exercises int
	FirstTry  int // resueltos al primer intento
	MaxCombo  float64
	ByModule  []ModuleSummary
	Review    []ReviewTopic
}

// Accuracy es la fracción de ejercicios resueltos al primer intento (0 a 1).
func (s Summary) Accuracy() float64 {
	if s.Exercises == 0 {
		return 0
	}
	return float64(s.FirstTry) / float64(s.Exercises)
}

// Summary calcula el resumen con lo registrado hasta ahora.
// Un fallo es no resolver el ejercicio o no hacerlo al primer intento; los temas se agrupan por source.
func (s *Session) Summary() Summary {
	sum := Summary{Duration: s.now().Sub(s.started), Exercises: len(s.results), MaxCombo: s.maxCombo}
	mods := map[int]*ModuleSummary{}
	misses := map[string]*ReviewTopic{}
	for _, r := range s.results {
		sum.Total += r.Points
		sum.Max += r.Max
		if r.Outcome.Solved && r.Outcome.FailedAttempts == 0 {
			sum.FirstTry++
		} else {
			t, ok := misses[r.Exercise.Source]
			if !ok {
				t = &ReviewTopic{Module: r.Exercise.Module, Source: r.Exercise.Source}
				misses[r.Exercise.Source] = t
			}
			t.Misses++
		}
		m, ok := mods[r.Exercise.Module]
		if !ok {
			m = &ModuleSummary{Module: r.Exercise.Module}
			mods[r.Exercise.Module] = m
		}
		m.Points += r.Points
		m.Max += r.Max
	}
	sum.Percent = scoring.Percent(sum.Total, sum.Max)
	sum.Rank = scoring.RankFor(sum.Percent)

	for _, m := range mods {
		m.Percent = scoring.Percent(m.Points, m.Max)
		sum.ByModule = append(sum.ByModule, *m)
	}
	sort.Slice(sum.ByModule, func(i, j int) bool { return sum.ByModule[i].Module < sum.ByModule[j].Module })

	for _, t := range misses {
		sum.Review = append(sum.Review, *t)
	}
	sort.Slice(sum.Review, func(i, j int) bool {
		a, b := sum.Review[i], sum.Review[j]
		if a.Misses != b.Misses {
			return a.Misses > b.Misses
		}
		return a.Source < b.Source
	})
	if len(sum.Review) > maxReview {
		sum.Review = sum.Review[:maxReview]
	}
	return sum
}
