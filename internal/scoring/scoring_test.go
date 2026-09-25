package scoring

import (
	"testing"
	"time"
)

func theory(d int, used, limit time.Duration) Outcome {
	return Outcome{Kind: Theory, Difficulty: d, TimeUsed: used, TimeLimit: limit, Solved: true}
}

func TestSubtotal(t *testing.T) {
	s := time.Second
	tests := []struct {
		name string
		o    Outcome
		want float64
	}{
		{"teoría perfecta en tiempo 0", theory(1, 0, 20*s), 200},
		{"teoría a la mitad del tiempo", theory(2, 10*s, 20*s), 200 + 50 + 100},
		{"teoría justo en el límite", theory(1, 20*s, 20*s), 150},
		{"tiempo usado mayor que el límite no resta", theory(1, 30*s, 20*s), 150},
		{"límite 0 no divide entre cero", theory(1, 5*s, 0), 150},
		{"segundo intento en Lecciones", func() Outcome { o := theory(1, 20*s, 20*s); o.FailedAttempts = 1; return o }(), 50},
		{"una pista", func() Outcome { o := theory(1, 20*s, 20*s); o.HintsUsed = 1; return o }(), 135},
		{"penalizaciones que darían negativo", func() Outcome {
			o := theory(1, 20*s, 20*s)
			o.FailedAttempts, o.HintsUsed = 1, 5
			return o
		}(), 0},
		{"no resuelto", Outcome{Kind: Theory, Difficulty: 5, Solved: false}, 0},
		{"práctica perfecta", Outcome{Kind: Practice, Difficulty: 2, TimeLimit: 60 * s, Solved: true,
			MaxAttempts: 3, Command: "mv file.txt ../dir3", ParChars: 19}, 200 + 100 + 100 + 20},
		{"práctica con un fallo y comando largo", Outcome{Kind: Practice, Difficulty: 1, TimeUsed: 60 * s, TimeLimit: 60 * s,
			Solved: true, FailedAttempts: 1, MaxAttempts: 2, Command: "mv file.txt ../dir3/file.txt", ParChars: 19}, 100 + 25},
		{"práctica con guard", Outcome{Kind: Practice, Difficulty: 1, TimeUsed: 60 * s, TimeLimit: 60 * s,
			Solved: true, MaxAttempts: 1, GuardPenalty: true}, 100 + 50 - 10},
		{"práctica sin max_attempts no divide entre cero", Outcome{Kind: Practice, Difficulty: 1, TimeUsed: 60 * s, TimeLimit: 60 * s,
			Solved: true, FailedAttempts: 1}, 100},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Subtotal(tt.o); got != tt.want {
				t.Errorf("Subtotal = %v, quiero %v", got, tt.want)
			}
		})
	}
}

func TestCommandLen(t *testing.T) {
	tests := map[string]int{
		"ls":                  2,
		"  ls   -l   /tmp  ":  10,
		"mv file.txt ../dir3": 19,
		"echo ñandú":          10,
		"":                    0,
	}
	for in, want := range tests {
		if got := CommandLen(in); got != want {
			t.Errorf("CommandLen(%q) = %d, quiero %d", in, got, want)
		}
	}
}

func TestElegance(t *testing.T) {
	o := Outcome{Kind: Practice, Difficulty: 1, TimeUsed: time.Second, TimeLimit: time.Second, Solved: true, ParChars: 10}
	o.Command = "123456789012" // 12 = 10 × 1.2
	withBonus := Subtotal(o)
	o.Command = "1234567890123"
	if withBonus-Subtotal(o) != 10 {
		t.Errorf("el bono de elegancia debe aplicar hasta par×1.2 inclusive")
	}
}

func TestCombo(t *testing.T) {
	good := theory(1, 0, time.Second)
	var c Combo
	if c.Multiplier() != 1 {
		t.Fatalf("combo inicial = %v", c.Multiplier())
	}
	for range 15 {
		c = c.Next(good)
	}
	if c.Multiplier() != 2 {
		t.Errorf("tope = %v, quiero 2", c.Multiplier())
	}
	if got := Points(good, c); got != 400 {
		t.Errorf("Points con ×2 = %d, quiero 400", got)
	}

	breakers := map[string]Outcome{
		"pista":          {Kind: Theory, Difficulty: 1, Solved: true, HintsUsed: 1},
		"fallo":          {Kind: Theory, Difficulty: 1, Solved: true, FailedAttempts: 1},
		"tiempo agotado": {Kind: Theory, Difficulty: 1, Solved: false},
	}
	for name, o := range breakers {
		if got := c.Next(o).Multiplier(); got != 1 {
			t.Errorf("%s: combo = %v, quiero 1", name, got)
		}
	}

	c = Combo{}
	for range 3 {
		c = c.Next(good)
	}
	if c.Multiplier() != 1.3 {
		t.Errorf("tres aciertos = %v, quiero 1.3", c.Multiplier())
	}
}

func TestRank(t *testing.T) {
	tests := []struct {
		pct  float64
		want Rank
	}{
		{120, RankS}, {95, RankS}, {94.99, RankA}, {85, RankA}, {70, RankB},
		{69.9, RankC}, {50, RankC}, {49.9, RankD}, {0, RankD},
	}
	for _, tt := range tests {
		if got := RankFor(tt.pct); got != tt.want {
			t.Errorf("RankFor(%v) = %s, quiero %s", tt.pct, got, tt.want)
		}
	}
	if Percent(10, 0) != 0 {
		t.Error("Percent con máximo 0 debe ser 0")
	}
	if !RankC.AtLeast(RankC) || !RankS.AtLeast(RankC) || RankD.AtLeast(RankC) {
		t.Error("AtLeast mal ordenado")
	}
}

func TestMaxPoints(t *testing.T) {
	if MaxPoints(Theory, 3) != 600 || MaxPoints(Practice, 1) != 210 {
		t.Errorf("MaxPoints = %v, %v", MaxPoints(Theory, 3), MaxPoints(Practice, 1))
	}
	// El máximo debe coincidir con el mejor caso real.
	best := Outcome{Kind: Practice, Difficulty: 3, TimeLimit: time.Minute, Solved: true, MaxAttempts: 3, Command: "ls", ParChars: 2}
	if Subtotal(best) != MaxPoints(Practice, 3) {
		t.Errorf("mejor práctica = %v, MaxPoints = %v", Subtotal(best), MaxPoints(Practice, 3))
	}
}
