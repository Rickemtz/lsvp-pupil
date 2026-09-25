package session

import (
	"testing"
	"time"

	"lsvp-pupil/internal/content"
	"lsvp-pupil/internal/scoring"
)

type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }

func ex(id string, module int, source string) content.Exercise {
	return content.Exercise{ID: id, Module: module, Source: source}
}

func perfect() scoring.Outcome {
	return scoring.Outcome{Kind: scoring.Theory, Difficulty: 1, TimeLimit: time.Second, Solved: true}
}

func TestRecordAppliesComboBeforeIncrement(t *testing.T) {
	s := New(time.Now)
	r1 := s.Record(ex("a", 1, "x"), perfect())
	r2 := s.Record(ex("b", 1, "x"), perfect())
	if r1.Points != 200 || r2.Points != 220 {
		t.Errorf("puntos = %d, %d; quiero 200, 220", r1.Points, r2.Points)
	}
	if s.Combo() != 1.2 {
		t.Errorf("combo = %v, quiero 1.2", s.Combo())
	}
	miss := perfect()
	miss.Solved = false
	if r := s.Record(ex("c", 1, "x"), miss); r.Points != 0 || s.Combo() != 1 {
		t.Errorf("un fallo da 0 y reinicia: %d pts, combo %v", r.Points, s.Combo())
	}
	if got := s.Summary().MaxCombo; got != 1.2 {
		t.Errorf("combo máximo = %v, quiero 1.2", got)
	}
}

func TestGuardPenaltyOnlyOnce(t *testing.T) {
	s := New(time.Now)
	o := scoring.Outcome{Kind: scoring.Practice, Difficulty: 1, TimeUsed: time.Second, TimeLimit: time.Second,
		Solved: true, MaxAttempts: 1, GuardPenalty: true}
	r1 := s.Record(ex("a", 4, "x"), o)
	r2 := s.Record(ex("b", 4, "x"), o)
	if r1.Outcome.GuardPenalty != true || r2.Outcome.GuardPenalty != false {
		t.Errorf("guard: %v, %v; quiero true, false", r1.Outcome.GuardPenalty, r2.Outcome.GuardPenalty)
	}
}

func TestSummary(t *testing.T) {
	clk := &clock{t: time.Unix(0, 0)}
	s := New(clk.now)
	second := perfect()
	second.FailedAttempts = 1
	lost := perfect()
	lost.Solved = false

	s.Record(ex("a", 1, "s#kernel"), perfect()) // 200 de 200
	s.Record(ex("b", 1, "s#kernel"), lost)      // 0 de 200
	s.Record(ex("c", 1, "s#historia"), second)  // 100 de 200 (combo reiniciado)
	s.Record(ex("d", 2, "t#arbol"), perfect())  // 200 de 200
	s.Record(ex("e", 2, "t#rutas"), lost)       // 0 de 200
	clk.t = clk.t.Add(90 * time.Second)

	sum := s.Summary()
	if sum.Total != 500 || sum.Max != 1000 || sum.Percent != 50 || sum.Rank != scoring.RankC {
		t.Errorf("total %d/%d = %v%% rango %s", sum.Total, sum.Max, sum.Percent, sum.Rank)
	}
	if sum.Duration != 90*time.Second {
		t.Errorf("duración = %v", sum.Duration)
	}
	if sum.FirstTry != 2 || sum.Accuracy() != 0.4 {
		t.Errorf("primer intento = %d, precisión %v", sum.FirstTry, sum.Accuracy())
	}
	if len(sum.ByModule) != 2 || sum.ByModule[0].Module != 1 || sum.ByModule[0].Points != 300 || sum.ByModule[1].Percent != 50 {
		t.Errorf("desglose = %+v", sum.ByModule)
	}
	want := []string{"s#historia", "s#kernel", "t#rutas"} // 1 fallo cada uno, por orden alfabético
	if len(sum.Review) != 3 {
		t.Fatalf("repaso = %+v", sum.Review)
	}
	for i, w := range want {
		if sum.Review[i].Source != w {
			t.Errorf("repaso[%d] = %s, quiero %s", i, sum.Review[i].Source, w)
		}
	}
}

func TestSummaryReviewOrderAndLimit(t *testing.T) {
	s := New(time.Now)
	lost := perfect()
	lost.Solved = false
	for i, src := range []string{"a", "b", "b", "c", "d", "d", "d"} {
		s.Record(ex(string(rune('0'+i)), 1, src), lost)
	}
	r := s.Summary().Review
	if len(r) != 3 || r[0].Source != "d" || r[0].Misses != 3 || r[1].Source != "b" || r[2].Source != "a" {
		t.Errorf("repaso = %+v", r)
	}
}

func TestEmptySummary(t *testing.T) {
	sum := New(time.Now).Summary()
	if sum.Percent != 0 || sum.Rank != scoring.RankD || sum.Accuracy() != 0 || sum.MaxCombo != 1 {
		t.Errorf("resumen vacío = %+v", sum)
	}
}
