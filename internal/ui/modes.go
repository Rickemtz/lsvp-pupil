package ui

import (
	"fmt"
	"time"
)

type modeKind int

const (
	modeLessons modeKind = iota
	modeTimeAttack
	modeQuickQuiz
	modeExam
)

// gameMode es la forma de jugar una partida.
type gameMode struct {
	kind    modeKind
	module  int // Lecciones: el módulo
	seconds int // Contrarreloj: la duración total
}

// scoreKey es la llave del ranking; Lecciones no entra al ranking (su récord es el del módulo).
func (m gameMode) scoreKey() string {
	switch m.kind {
	case modeTimeAttack:
		return fmt.Sprintf("contrarreloj-%d", m.seconds)
	case modeQuickQuiz:
		return "quiz"
	case modeExam:
		return "examen"
	}
	return ""
}

func (m gameMode) title() string {
	switch m.kind {
	case modeTimeAttack:
		return fmt.Sprintf(strModeTimeAttackFmt, m.seconds)
	case modeQuickQuiz:
		return strMenuQuickQuiz
	case modeExam:
		return strMenuFinalExam
	}
	return ""
}

func (m gameMode) duration() time.Duration { return time.Duration(m.seconds) * time.Second }

// leaderboardModes son los tableros del ranking, en el orden en que se muestran.
var leaderboardModes = []gameMode{
	{kind: modeExam},
	{kind: modeQuickQuiz},
	{kind: modeTimeAttack, seconds: 60},
	{kind: modeTimeAttack, seconds: 120},
	{kind: modeTimeAttack, seconds: 300},
}
