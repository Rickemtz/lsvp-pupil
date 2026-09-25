package ui

import (
	"fmt"
	"math"
	"math/rand/v2"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"lsvp-pupil/internal/sandbox"
	"lsvp-pupil/internal/session"
)

const tickEvery = 250 * time.Millisecond

// tickMsg lleva el id de la pantalla de ejercicio para descartar ticks de ejercicios anteriores.
type tickMsg struct{ id int }

func tick(id int) tea.Cmd {
	return tea.Tick(tickEvery, func(time.Time) tea.Msg { return tickMsg{id: id} })
}

// sandboxFactory crea un sandbox con el backend y los fixtures elegidos al iniciar la app.
type sandboxFactory func(sandbox.Options) (sandbox.Sandbox, error)

// exerciseCtx es lo que cada pantalla de ejercicio recibe del runner de la partida.
type exerciseCtx struct {
	id         int // único en toda la ejecución
	title      string
	index      int // 0-based
	total      int
	lessons    bool
	session    *session.Session
	deadline   time.Time // fin de la partida en Contrarreloj; cero si no hay
	now        func() time.Time
	rng        *rand.Rand
	newSandbox sandboxFactory
}

// exerciseScreen es una pantalla de un ejercicio (quiz o práctica).
type exerciseScreen interface {
	Init() tea.Cmd
	Update(tea.Msg) tea.Cmd
	View() string
	SetSize(width, height int)
	Close()
}

// renderHUD dibuja la línea superior: ejercicio, tiempo, puntos, combo y algo extra (intentos).
func renderHUD(ctx exerciseCtx, remaining, limit time.Duration, extra string) string {
	var s string
	if ctx.total > 0 {
		s = styleTitle.Render(fmt.Sprintf(strHUDExerciseFmt, ctx.title, ctx.index+1, ctx.total))
	} else {
		s = styleTitle.Render(fmt.Sprintf(strHUDExerciseOpenFmt, ctx.title, ctx.index+1))
	}
	s += "   " + renderTimer(remaining, limit)
	if !ctx.deadline.IsZero() {
		left := max(0, ctx.deadline.Sub(ctx.now()))
		s += "  " + styleSubtitle.Render(fmt.Sprintf(strHUDTotalFmt, formatDuration(left)))
	}
	s += "   " + styleScore.Render(fmt.Sprintf(strHUDScoreFmt, ctx.session.Total()))
	s += "  " + styleCombo.Render(fmt.Sprintf(strHUDComboFmt, ctx.session.Combo()))
	if extra != "" {
		s += "   " + extra
	}
	return s
}

func renderTimer(remaining, limit time.Duration) string {
	secs := int(math.Ceil(remaining.Seconds()))
	text := fmt.Sprintf(strHUDTimerFmt, secs)
	frac := float64(remaining) / float64(limit)
	switch {
	case frac < 0.1:
		return styleTimerDanger.Render(text)
	case frac < 0.3:
		return styleTimerWarn.Render(text)
	}
	return styleTimer.Render(text)
}

func pointsLabel(r session.Result) string {
	if r.Combo > 1 {
		return fmt.Sprintf(strPointsComboFmt, r.Points, r.Combo)
	}
	return fmt.Sprintf(strPointsFmt, r.Points)
}

// textWidth es el ancho útil dentro del marco (styleFrame agrega 2 columnas por lado).
func textWidth(width int) int {
	if width == 0 {
		return minWidth - 4
	}
	return max(20, width-4)
}
