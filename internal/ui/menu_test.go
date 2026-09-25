package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func key(s string) tea.KeyMsg {
	switch s {
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "f1":
		return tea.KeyMsg{Type: tea.KeyF1}
	case " ":
		return tea.KeyMsg{Type: tea.KeySpace, Runes: []rune(" ")}
	case "f2":
		return tea.KeyMsg{Type: tea.KeyF2}
	case "f3":
		return tea.KeyMsg{Type: tea.KeyF3}
	case "f4":
		return tea.KeyMsg{Type: tea.KeyF4}
	case "f5":
		return tea.KeyMsg{Type: tea.KeyF5}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func TestMenuNavigation(t *testing.T) {
	tests := []struct {
		name string
		keys []string
		want int
	}{
		{"inicio", nil, 0},
		{"abajo", []string{"down"}, 1},
		{"arriba da la vuelta", []string{"up"}, 6},
		{"abajo da la vuelta", []string{"up", "down"}, 0},
		{"vim", []string{"j", "j", "k"}, 1},
		{"número", []string{"4"}, 3},
		{"número fuera de rango", []string{"9"}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newMenu()
			for _, k := range tt.keys {
				m, _ = m.Update(key(k))
			}
			if m.cursor != tt.want {
				t.Errorf("cursor = %d, quiero %d", m.cursor, tt.want)
			}
		})
	}
}

func TestMenuActivate(t *testing.T) {
	_, cmd := newMenu().Update(key("enter"))
	if cmd == nil {
		t.Fatal("Lecciones debería abrir la lista de módulos")
	}
	if _, ok := cmd().(openLessonsMsg); !ok {
		t.Errorf("Lecciones devolvió %T, quiero openLessonsMsg", cmd())
	}

	for k, want := range map[string]tea.Msg{
		"6": openAboutMsg{},
		"2": openTimeAttackMsg{},
		"3": startModeMsg{mode: gameMode{kind: modeQuickQuiz}},
		"4": startModeMsg{mode: gameMode{kind: modeExam}},
		"5": openLeaderboardMsg{},
	} {
		if _, cmd := newMenu().Update(key(k)); cmd == nil || cmd() != want {
			t.Errorf("opción %s: quiero %#v", k, want)
		}
	}

	_, cmd = newMenu().Update(key("7"))
	if cmd == nil {
		t.Fatal("Salir debería devolver tea.Quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Errorf("Salir devolvió %T, quiero tea.QuitMsg", cmd())
	}
}

func TestAppTooSmall(t *testing.T) {
	tests := []struct {
		w, h int
		want bool
	}{
		{0, 0, false}, // tamaño aún desconocido
		{80, 24, false},
		{79, 24, true},
		{80, 23, true},
	}
	for _, tt := range tests {
		a := App{width: tt.w, height: tt.h}
		if got := a.tooSmall(); got != tt.want {
			t.Errorf("tooSmall(%d×%d) = %v, quiero %v", tt.w, tt.h, got, tt.want)
		}
	}
}
