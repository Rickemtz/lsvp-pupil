package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type choiceItem struct {
	label string
	msg   tea.Msg
}

// choiceModel es una lista corta de opciones (p. ej. la duración de Contrarreloj).
type choiceModel struct {
	title  string
	items  []choiceItem
	cursor int
}

func (c choiceModel) Update(msg tea.Msg) (choiceModel, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return c, nil
	}
	switch k := key.String(); k {
	case "up", "k":
		c.cursor = (c.cursor - 1 + len(c.items)) % len(c.items)
	case "down", "j":
		c.cursor = (c.cursor + 1) % len(c.items)
	case "enter", " ":
		return c, send(c.items[c.cursor].msg)
	case "esc", "q":
		return c, send(backToMenuMsg{})
	default:
		if len(k) == 1 && k[0] >= '1' && int(k[0]-'1') < len(c.items) {
			c.cursor = int(k[0] - '1')
			return c, send(c.items[c.cursor].msg)
		}
	}
	return c, nil
}

func (c choiceModel) View() string {
	var b strings.Builder
	b.WriteString(styleTitle.Render(c.title) + "\n\n")
	for i, it := range c.items {
		line := fmt.Sprintf("%d. %s", i+1, it.label)
		if i == c.cursor {
			b.WriteString(styleSelected.Render("> "+line) + "\n")
		} else {
			b.WriteString(styleItem.Render(line) + "\n")
		}
	}
	b.WriteString("\n" + styleHelp.Render(strChoiceHelp))
	return b.String()
}
