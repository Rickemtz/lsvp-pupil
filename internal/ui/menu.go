package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type menuAction int

const (
	actionLessons menuAction = iota
	actionTimeAttack
	actionQuickQuiz
	actionFinalExam
	actionLeaderboard
	actionAbout
	actionQuit
)

type menuItem struct {
	label  string
	action menuAction
}

type menuModel struct {
	items  []menuItem
	cursor int
	status string
}

func newMenu() menuModel {
	return menuModel{items: []menuItem{
		{strMenuLessons, actionLessons},
		{strMenuTimeAttack, actionTimeAttack},
		{strMenuQuickQuiz, actionQuickQuiz},
		{strMenuFinalExam, actionFinalExam},
		{strMenuLeaderboard, actionLeaderboard},
		{strMenuAbout, actionAbout},
		{strMenuQuit, actionQuit},
	}}
}

func (m menuModel) Update(msg tea.Msg) (menuModel, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch s := key.String(); s {
	case "up", "k":
		m.cursor = (m.cursor - 1 + len(m.items)) % len(m.items)
		m.status = ""
	case "down", "j":
		m.cursor = (m.cursor + 1) % len(m.items)
		m.status = ""
	case "enter", " ":
		return m.activate()
	case "q":
		return m, tea.Quit
	default:
		if len(s) == 1 && s[0] >= '1' && int(s[0]-'1') < len(m.items) {
			m.cursor = int(s[0] - '1')
			return m.activate()
		}
	}
	return m, nil
}

func (m menuModel) activate() (menuModel, tea.Cmd) {
	item := m.items[m.cursor]
	switch item.action {
	case actionQuit:
		return m, tea.Quit
	case actionLessons:
		return m, send(openLessonsMsg{})
	case actionTimeAttack:
		return m, send(openTimeAttackMsg{})
	case actionQuickQuiz:
		return m, send(startModeMsg{mode: gameMode{kind: modeQuickQuiz}})
	case actionFinalExam:
		return m, send(startModeMsg{mode: gameMode{kind: modeExam}})
	case actionLeaderboard:
		return m, send(openLeaderboardMsg{})
	case actionAbout:
		return m, send(openAboutMsg{})
	}
	return m, nil
}

func (m menuModel) View() string {
	var b strings.Builder
	b.WriteString(styleTitle.Render(strAppTitle) + "\n")
	b.WriteString(styleSubtitle.Render(strAppSubtitle) + "\n\n")
	for i, it := range m.items {
		line := fmt.Sprintf("%d. %s", i+1, it.label)
		if i == m.cursor {
			b.WriteString(styleSelected.Render("> "+line) + "\n")
		} else {
			b.WriteString(styleItem.Render(line) + "\n")
		}
	}
	b.WriteString("\n")
	if m.status != "" {
		b.WriteString(styleStatus.Render(m.status) + "\n")
	}
	b.WriteString(styleHelp.Render(strMenuHelp))
	return b.String()
}
