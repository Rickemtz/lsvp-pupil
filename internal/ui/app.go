// Package ui contiene la interfaz de terminal (Bubble Tea).
package ui

import (
	"fmt"
	"io/fs"
	"math/rand/v2"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"lsvp-pupil/internal/content"
	"lsvp-pupil/internal/sandbox"
	"lsvp-pupil/internal/session"
	"lsvp-pupil/internal/storage"
)

const (
	minWidth  = 80
	minHeight = 24
)

type screen int

const (
	screenMenu screen = iota
	screenModules
	screenLesson
	screenResults
	screenTimeAttack
	screenLeaderboard
	screenAbout
)

// Config son las dependencias de la app.
type Config struct {
	Catalog   *content.Catalog
	Backend   sandbox.Backend
	Fixtures  fs.FS
	UnlockAll bool
	Store     *storage.Store // nil: no se guarda nada
	Version   string
}

// App es el modelo raíz: guarda el tamaño de la terminal y delega en la pantalla activa.
type App struct {
	catalog       *content.Catalog
	newSandbox    sandboxFactory
	rng           *rand.Rand
	now           func() time.Time
	nextID        *int
	unlockAll     bool
	store         *storage.Store
	progress      *storage.Progress
	scores        storage.Scores
	width, height int

	screen      screen
	menu        menuModel
	modules     moduleListModel
	lesson      *lessonModel
	results     resultsModel
	timeAttack  choiceModel
	leaderboard leaderboardModel
	about       aboutModel
}

// NewApp crea la aplicación con el menú principal como pantalla inicial. Si el progreso guardado no
// se puede leer, la app funciona igual y el menú muestra un aviso.
func NewApp(cfg Config) App {
	seed := uint64(time.Now().UnixNano())
	store := cfg.Store
	if store == nil {
		store = storage.Open("")
	}
	a := App{
		catalog: cfg.Catalog,
		newSandbox: func(o sandbox.Options) (sandbox.Sandbox, error) {
			o.Fixtures = cfg.Fixtures
			return sandbox.New(cfg.Backend, o)
		},
		rng:       rand.New(rand.NewPCG(seed, seed>>32)),
		now:       time.Now,
		nextID:    new(int),
		unlockAll: cfg.UnlockAll,
		store:     store,
		menu:      newMenu(),
		about:     aboutModel{version: cfg.Version, backend: string(cfg.Backend), configDir: store.Dir()},
	}
	p, err := store.LoadProgress()
	a.progress = &p
	if err != nil {
		a.menu.status = fmt.Sprintf(strStorageWarnFmt, err)
	}
	if a.scores, err = store.LoadScores(); err != nil {
		a.menu.status = fmt.Sprintf(strStorageWarnFmt, err)
	}
	return a
}

func (a App) Init() tea.Cmd { return nil }

// unlocked dice qué módulos están abiertos según el progreso (o todos con --unlock-all).
func (a App) unlocked() map[int]bool {
	u := a.progress.Unlocked(a.catalog.Modules)
	if a.unlockAll {
		for _, m := range a.catalog.Modules {
			u[m.ID] = true
		}
	}
	return u
}

// availableExercises junta los ejercicios de los módulos abiertos (o de todos, para el examen).
func (a App) availableExercises(all bool) []content.Exercise {
	u := a.unlocked()
	var exs []content.Exercise
	for _, m := range a.catalog.Modules {
		if all || u[m.ID] {
			exs = append(exs, a.catalog.Exercises(m.ID)...)
		}
	}
	return exs
}

func (a App) examModule() (content.Module, bool) {
	for _, m := range a.catalog.Modules {
		if m.Kind == content.ModuleExam {
			return m, true
		}
	}
	return content.Module{}, false
}

func (a App) startGame(title string, mode gameMode, exs []content.Exercise) (App, tea.Cmd) {
	a.Close()
	a.lesson = newLesson(title, mode, exs, a.now, a.rng, a.newSandbox, a.nextID, a.width, a.height)
	a.screen = screenLesson
	return a, a.lesson.Init()
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width, a.height = msg.Width, msg.Height
		a.results.width, a.results.height = msg.Width, msg.Height
		if a.lesson != nil {
			a.lesson.SetSize(msg.Width, msg.Height)
		}
		return a, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "f10":
			a.Close()
			return a, tea.Quit
		}
	case openLessonsMsg:
		a.modules = newModuleList(a.catalog, a.unlocked(), *a.progress)
		a.screen = screenModules
		return a, nil
	case openTimeAttackMsg:
		a.timeAttack = choiceModel{title: strTimeAttackTitle}
		for _, s := range session.TimeAttackDurations {
			a.timeAttack.items = append(a.timeAttack.items, choiceItem{
				label: fmt.Sprintf(strTimeAttackItemFmt, s),
				msg:   startModeMsg{mode: gameMode{kind: modeTimeAttack, seconds: s}},
			})
		}
		a.screen = screenTimeAttack
		return a, nil
	case openAboutMsg:
		a.about.width = a.width
		a.screen = screenAbout
		return a, nil
	case openLeaderboardMsg:
		a.leaderboard = leaderboardModel{scores: a.scores}
		a.screen = screenLeaderboard
		return a, nil
	case backToMenuMsg:
		a.Close()
		a.screen = screenMenu
		return a, nil
	case backToLessonMsg:
		a.Close()
		if a.lesson != nil && a.lesson.mode.kind != modeLessons {
			a.screen = screenMenu
			return a, nil
		}
		a.modules = newModuleList(a.catalog, a.unlocked(), *a.progress)
		a.screen = screenModules
		return a, nil
	case showResultsMsg:
		a.Close()
		return a.showResults(msg), nil
	case startLessonMsg:
		m, _ := a.catalog.Module(msg.module)
		return a.startGame(m.Title, gameMode{kind: modeLessons, module: m.ID}, a.catalog.Exercises(m.ID))
	case startModeMsg:
		return a.startMode(msg.mode)
	}

	var cmd tea.Cmd
	switch a.screen {
	case screenMenu:
		a.menu, cmd = a.menu.Update(msg)
	case screenModules:
		a.modules, cmd = a.modules.Update(msg)
	case screenLesson:
		if a.lesson != nil {
			cmd = a.lesson.Update(msg)
		}
	case screenResults:
		a.results, cmd = a.results.Update(msg)
	case screenTimeAttack:
		a.timeAttack, cmd = a.timeAttack.Update(msg)
	case screenLeaderboard:
		a.leaderboard, cmd = a.leaderboard.Update(msg)
	case screenAbout:
		a.about, cmd = a.about.Update(msg)
	}
	return a, cmd
}

func (a App) startMode(mode gameMode) (tea.Model, tea.Cmd) {
	var exs []content.Exercise
	switch mode.kind {
	case modeTimeAttack:
		exs = session.TimeAttackPool(a.availableExercises(false), a.rng)
	case modeQuickQuiz:
		exs = session.PickQuickQuiz(a.availableExercises(false), a.rng)
	case modeExam:
		if m, ok := a.examModule(); ok && !a.unlocked()[m.ID] {
			a.menu.status = strExamLocked
			a.screen = screenMenu
			return a, nil
		}
		exs = session.PickExam(a.availableExercises(true), a.rng)
	}
	if len(exs) == 0 {
		a.menu.status = strModeEmpty
		a.screen = screenMenu
		return a, nil
	}
	a.menu.status = ""
	return a.startGame(mode.title(), mode, exs)
}

// showResults guarda el progreso y el ranking y arma la pantalla de resultados.
func (a App) showResults(msg showResultsMsg) App {
	sum := msg.summary
	var notes []string
	before := a.unlocked()

	for _, r := range msg.results {
		if r.Outcome.Solved {
			a.progress.Solved[r.Exercise.ID] = true
		}
		if !r.Outcome.Solved || r.Outcome.FailedAttempts > 0 {
			a.progress.Misses[r.Exercise.Source]++
		}
	}
	switch msg.mode.kind {
	case modeLessons:
		if a.progress.RecordModule(msg.mode.module, sum.Rank, sum.Percent) {
			notes = append(notes, strResultsModuleBest)
		}
		after := a.unlocked()
		for _, m := range a.catalog.Modules {
			if after[m.ID] && !before[m.ID] {
				notes = append(notes, fmt.Sprintf(strResultsUnlockFmt, m.Title))
			}
		}
	case modeExam:
		a.progress.RecordExam(sum.Rank)
		notes = append(notes, strResultsOfficial)
	}
	if key := msg.mode.scoreKey(); key != "" && sum.Exercises > 0 {
		pos, record := a.scores.Add(key, storage.Score{Points: sum.Total, Rank: sum.Rank, Percent: sum.Percent,
			Duration: sum.Duration, Date: a.now()})
		switch {
		case record:
			notes = append(notes, strResultsNewRecord)
		case pos > 0:
			notes = append(notes, fmt.Sprintf(strResultsPosFmt, pos))
		}
		if err := a.store.SaveScores(a.scores); err != nil {
			notes = append(notes, fmt.Sprintf(strResultsSaveErrFmt, err))
		}
	}
	if err := a.store.SaveProgress(*a.progress); err != nil {
		notes = append(notes, fmt.Sprintf(strResultsSaveErrFmt, err))
	}

	var back tea.Msg = backToLessonMsg{}
	if msg.mode.kind != modeLessons {
		back = backToMenuMsg{}
	}
	a.results = resultsModel{title: msg.title, summary: sum, catalog: a.catalog, width: a.width, height: a.height,
		notes: notes, back: back}
	a.screen = screenResults
	return a
}

func (a App) View() string {
	if a.tooSmall() {
		return styleFrame.Render(styleStatus.Render(
			fmt.Sprintf(strTooSmallFmt, a.width, a.height, minWidth, minHeight)))
	}
	switch a.screen {
	case screenModules:
		return styleFrame.Render(a.modules.View())
	case screenLesson:
		return styleFrame.Render(a.lesson.View())
	case screenResults:
		return styleFrame.Render(a.results.View())
	case screenTimeAttack:
		return styleFrame.Render(a.timeAttack.View())
	case screenLeaderboard:
		return styleFrame.Render(a.leaderboard.View())
	case screenAbout:
		return styleFrame.Render(a.about.View())
	}
	return styleFrame.Render(a.menu.View())
}

// Close libera el sandbox de la partida en curso, si hay una. Se puede llamar varias veces.
func (a App) Close() {
	if a.lesson != nil {
		a.lesson.Close()
	}
}

// tooSmall solo es verdadero cuando ya se conoce el tamaño y es menor al mínimo.
func (a App) tooSmall() bool {
	return a.width > 0 && (a.width < minWidth || a.height < minHeight)
}
