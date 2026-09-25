// Package scoring calcula puntos, combos y rangos. Funciones puras.
package scoring

import (
	"math"
	"strings"
	"time"
)

// Kind distingue las fórmulas de teoría y de práctica.
type Kind int

const (
	Theory Kind = iota
	Practice
)

const (
	GuardPenaltyPoints = 10
	HintPenalty        = 0.15 // fracción de base por pista
	SecondTryPenalty   = 0.5  // fracción de base al acertar en el segundo intento (quiz en Lecciones)
	EleganceSlack      = 1.2  // el comando puede medir hasta par_chars × 1.2
)

// Outcome es lo que pasó en un ejercicio.
type Outcome struct {
	Kind       Kind
	Difficulty int
	TimeUsed   time.Duration
	TimeLimit  time.Duration
	Solved     bool // falso si se agotó el tiempo o los intentos, o si se saltó

	FailedAttempts int
	MaxAttempts    int // práctica: max_attempts del ejercicio
	HintsUsed      int

	Command  string // práctica: comando con el que se resolvió
	ParChars int    // práctica: longitud de la solución de referencia

	GuardPenalty bool // la primera vez en la partida que guard bloqueó un comando
}

// Base es 100 × dificultad.
func Base(difficulty int) float64 { return 100 * float64(difficulty) }

// FirstTry dice si el ejercicio se resolvió al primer intento y sin pistas (lo que alimenta el combo).
func (o Outcome) FirstTry() bool {
	return o.Solved && o.FailedAttempts == 0 && o.HintsUsed == 0
}

// Subtotal son los puntos del ejercicio antes de aplicar el combo. Nunca es negativo.
func Subtotal(o Outcome) float64 {
	if !o.Solved {
		return 0
	}
	base := Base(o.Difficulty)
	pts := base + timeBonus(base, o.TimeUsed, o.TimeLimit)

	switch o.Kind {
	case Theory:
		if o.FailedAttempts == 0 {
			pts += base * 0.5
		} else {
			pts -= base * SecondTryPenalty
		}
	case Practice:
		if o.MaxAttempts > 0 {
			pts += base * 0.5 * (1 - float64(o.FailedAttempts)/float64(o.MaxAttempts))
		}
		if o.ParChars > 0 && float64(CommandLen(o.Command)) <= float64(o.ParChars)*EleganceSlack {
			pts += base * 0.1
		}
	}

	pts -= base * HintPenalty * float64(o.HintsUsed)
	if o.GuardPenalty {
		pts -= GuardPenaltyPoints
	}
	return math.Max(0, pts)
}

func timeBonus(base float64, used, limit time.Duration) float64 {
	if limit <= 0 {
		return 0
	}
	return base * 0.5 * math.Max(0, 1-float64(used)/float64(limit))
}

// MaxPoints es el máximo teórico de un ejercicio sin combo: tiempo 0, primer intento, elegante.
func MaxPoints(kind Kind, difficulty int) float64 {
	base := Base(difficulty)
	if kind == Practice {
		return base * 2.1
	}
	return base * 2
}

// CommandLen mide un comando sin espacios extremos y contando cada racha de espacios como uno.
func CommandLen(cmd string) int {
	return len([]rune(strings.Join(strings.Fields(cmd), " ")))
}

// Combo lleva el multiplicador en décimas para evitar errores de redondeo.
type Combo struct{ steps int }

const maxComboSteps = 10 // ×2.0

// Multiplier devuelve el multiplicador actual (1.0 a 2.0).
func (c Combo) Multiplier() float64 { return 1 + float64(c.steps)/10 }

// Next devuelve el combo tras un ejercicio: +0.1 si fue al primer intento y sin pistas; si no, se reinicia.
func (c Combo) Next(o Outcome) Combo {
	if !o.FirstTry() {
		return Combo{}
	}
	return Combo{steps: min(c.steps+1, maxComboSteps)}
}

// Points aplica el multiplicador al subtotal y redondea.
func Points(o Outcome, c Combo) int {
	return int(math.Round(Subtotal(o) * c.Multiplier()))
}

// Rank es el rango final.
type Rank string

const (
	RankS Rank = "S"
	RankA Rank = "A"
	RankB Rank = "B"
	RankC Rank = "C"
	RankD Rank = "D"
)

// Percent es puntaje / máximo teórico sin combo × 100. Puede pasar de 100 gracias al combo.
func Percent(total, max int) float64 {
	if max <= 0 {
		return 0
	}
	return float64(total) / float64(max) * 100
}

// RankFor devuelve el rango para un porcentaje.
func RankFor(percent float64) Rank {
	switch {
	case percent >= 95:
		return RankS
	case percent >= 85:
		return RankA
	case percent >= 70:
		return RankB
	case percent >= 50:
		return RankC
	}
	return RankD
}

// AtLeast dice si r es igual o mejor que other (S > A > B > C > D).
func (r Rank) AtLeast(other Rank) bool {
	order := "SABCD"
	return strings.Index(order, string(r)) <= strings.Index(order, string(other))
}
