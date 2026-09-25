package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"lsvp-pupil/internal/content"
	"lsvp-pupil/internal/scoring"
	"lsvp-pupil/internal/storage"
)

type moduleEntry struct {
	module content.Module
	count  int
	locked bool
	best   scoring.Rank // mejor rango en Lecciones; vacío si nunca se terminó
}

// moduleListModel es la lista de módulos del modo Lecciones.
type moduleListModel struct {
	entries []moduleEntry
	cursor  int
	status  string
}

func newModuleList(cat *content.Catalog, unlocked map[int]bool, progress storage.Progress) moduleListModel {
	var m moduleListModel
	for _, mod := range cat.Modules {
		if mod.Kind == content.ModuleExam {
			continue // el examen tiene su propia opción en el menú
		}
		m.entries = append(m.entries, moduleEntry{
			module: mod,
			count:  len(cat.Exercises(mod.ID)),
			locked: !unlocked[mod.ID],
			best:   progress.Modules[mod.ID].BestRank,
		})
	}
	return m
}

func (m moduleListModel) Update(msg tea.Msg) (moduleListModel, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch key.String() {
	case "up", "k":
		m.cursor = (m.cursor - 1 + len(m.entries)) % len(m.entries)
		m.status = ""
	case "down", "j":
		m.cursor = (m.cursor + 1) % len(m.entries)
		m.status = ""
	case "esc", "q":
		return m, send(backToMenuMsg{})
	case "enter", " ":
		e := m.entries[m.cursor]
		switch {
		case e.locked:
			m.status = strModuleLocked
		case e.count == 0:
			m.status = strModuleNoItems
		default:
			return m, send(startLessonMsg{module: e.module.ID})
		}
	}
	return m, nil
}

func (m moduleListModel) View() string {
	var b strings.Builder
	b.WriteString(styleTitle.Render(strModulesTitle) + "\n\n")
	for i, e := range m.entries {
		detail := fmt.Sprintf(strModuleCountFmt, e.count)
		if e.count == 0 {
			detail = strModuleEmpty
		}
		line := fmt.Sprintf("%2d. %-26s %s", e.module.ID, e.module.Title, detail)
		if e.locked {
			line += " " + strLockMark
		} else if e.best != "" {
			line += " " + fmt.Sprintf(strModuleBestFmt, e.best)
		}
		switch {
		case i == m.cursor:
			b.WriteString(styleSelected.Render("> "+line) + "\n")
		case e.locked || e.count == 0:
			b.WriteString(styleDisabled.Render(line) + "\n")
		default:
			b.WriteString(styleItem.Render(line) + "\n")
		}
	}
	b.WriteString("\n")
	if m.status != "" {
		b.WriteString(styleStatus.Render(m.status) + "\n")
	}
	b.WriteString(styleHelp.Render(strModulesHelp))
	return b.String()
}
