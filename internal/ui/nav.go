package ui

import (
	tea "github.com/charmbracelet/bubbletea"

	"lsvp-pupil/internal/session"
)

// Mensajes de navegación entre pantallas; los maneja App.
type (
	openLessonsMsg     struct{}
	backToMenuMsg      struct{}
	startLessonMsg     struct{ module int }
	startModeMsg       struct{ mode gameMode }
	openTimeAttackMsg  struct{}
	openLeaderboardMsg struct{}
	openAboutMsg       struct{}
	backToLessonMsg    struct{}
	nextExerciseMsg    struct{}
	showResultsMsg     struct {
		title   string
		mode    gameMode
		summary session.Summary
		results []session.Result
	}
)

func send(msg tea.Msg) tea.Cmd {
	return func() tea.Msg { return msg }
}
