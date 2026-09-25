// Package storage guarda el progreso y el ranking en JSON, en ~/.config/lsvp-pupil/.
package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"

	"lsvp-pupil/internal/content"
	"lsvp-pupil/internal/scoring"
)

const (
	progressFile = "progress.json"
	scoresFile   = "scores.json"
	topN         = 10
)

// ErrCorrupt indica que un archivo estaba dañado: se respaldó como .bak y se empezó de cero.
var ErrCorrupt = errors.New("archivo dañado")

// Store lee y escribe los archivos de un directorio. Con dir vacío no guarda nada (solo memoria).
type Store struct{ dir string }

// DefaultDir es ~/.config/lsvp-pupil (o el equivalente de cada sistema).
func DefaultDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("buscar el directorio de configuración: %w", err)
	}
	return filepath.Join(base, "lsvp-pupil"), nil
}

// Open prepara un Store en dir.
func Open(dir string) *Store { return &Store{dir: dir} }

// Dir devuelve el directorio donde se guarda todo.
func (s *Store) Dir() string { return s.dir }

// ModuleProgress es el mejor resultado de un módulo en modo Lecciones.
type ModuleProgress struct {
	BestRank    scoring.Rank `json:"best_rank"`
	BestPercent float64      `json:"best_percent"`
	Completed   int          `json:"completed"` // veces que se terminó
}

// Progress es el progreso del alumno.
type Progress struct {
	Modules  map[int]ModuleProgress `json:"modules"`
	Solved   map[string]bool        `json:"solved"`    // ids de ejercicios resueltos alguna vez
	Misses   map[string]int         `json:"misses"`    // fallos por sección del curso (source)
	ExamRank scoring.Rank           `json:"exam_rank"` // mejor rango «oficial» del examen final
}

func newProgress() Progress {
	return Progress{Modules: map[int]ModuleProgress{}, Solved: map[string]bool{}, Misses: map[string]int{}}
}

// Unlocked dice qué módulos están abiertos: los marcados open y los que siguen a uno terminado con rango C o mejor.
func (p Progress) Unlocked(mods []content.Module) map[int]bool {
	open := map[int]bool{}
	for _, m := range mods {
		prev, ok := p.Modules[m.ID-1]
		open[m.ID] = m.Open || (ok && prev.Completed > 0 && prev.BestRank.AtLeast(scoring.RankC))
	}
	return open
}

// RecordModule guarda un módulo terminado. Devuelve true si su rango es el mejor hasta ahora.
func (p *Progress) RecordModule(id int, rank scoring.Rank, percent float64) bool {
	mp, ok := p.Modules[id]
	better := !ok || mp.Completed == 0 || percent > mp.BestPercent
	if better {
		mp.BestRank, mp.BestPercent = rank, percent
	}
	mp.Completed++
	p.Modules[id] = mp
	return better
}

// RecordExam guarda el rango del examen si es el mejor.
func (p *Progress) RecordExam(rank scoring.Rank) {
	if p.ExamRank == "" || (rank.AtLeast(p.ExamRank) && rank != p.ExamRank) {
		p.ExamRank = rank
	}
}

// Score es una entrada del ranking.
type Score struct {
	Points   int           `json:"points"`
	Rank     scoring.Rank  `json:"rank"`
	Percent  float64       `json:"percent"`
	Duration time.Duration `json:"duration"`
	Date     time.Time     `json:"date"`
}

// Scores es el top 10 de cada modo.
type Scores map[string][]Score

// Add agrega un puntaje al modo y deja solo los 10 mejores. Devuelve la posición (1 = primero; 0 si
// no entró) y si es un nuevo récord del modo.
func (sc Scores) Add(mode string, s Score) (pos int, record bool) {
	list := append(sc[mode], s)
	sort.SliceStable(list, func(i, j int) bool { return list[i].Points > list[j].Points })
	for i, e := range list {
		if e == s {
			pos = i + 1
			break
		}
	}
	if len(list) > topN {
		list = list[:topN]
	}
	if pos > topN {
		pos = 0
	}
	sc[mode] = list
	return pos, pos == 1 && s.Points > 0 && (len(list) == 1 || list[1].Points < s.Points)
}

// LoadProgress lee progress.json. Si no existe, devuelve un progreso vacío. Si está dañado lo
// respalda como .bak, devuelve un progreso vacío y ErrCorrupt.
func (s *Store) LoadProgress() (Progress, error) {
	p := newProgress()
	err := s.load(progressFile, &p)
	if p.Modules == nil || p.Solved == nil || p.Misses == nil {
		fresh := newProgress()
		if p.Modules != nil {
			fresh.Modules = p.Modules
		}
		if p.Solved != nil {
			fresh.Solved = p.Solved
		}
		if p.Misses != nil {
			fresh.Misses = p.Misses
		}
		fresh.ExamRank = p.ExamRank
		p = fresh
	}
	return p, err
}

// SaveProgress escribe progress.json de forma atómica.
func (s *Store) SaveProgress(p Progress) error { return s.save(progressFile, p) }

// LoadScores lee scores.json, con las mismas reglas que LoadProgress.
func (s *Store) LoadScores() (Scores, error) {
	sc := Scores{}
	err := s.load(scoresFile, &sc)
	if sc == nil {
		sc = Scores{}
	}
	return sc, err
}

// SaveScores escribe scores.json de forma atómica.
func (s *Store) SaveScores(sc Scores) error { return s.save(scoresFile, sc) }

func (s *Store) load(name string, v any) error {
	if s.dir == "" {
		return nil
	}
	path := filepath.Join(s.dir, name)
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("leer %s: %w", path, err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		if rerr := os.Rename(path, path+".bak"); rerr != nil {
			return fmt.Errorf("respaldar %s dañado: %w", path, rerr)
		}
		return fmt.Errorf("%s: %w (se guardó una copia en %s.bak)", path, ErrCorrupt, path)
	}
	return nil
}

// save escribe en un temporal del mismo directorio y lo renombra: nunca queda un archivo a medias.
func (s *Store) save(name string, v any) error {
	if s.dir == "" {
		return nil
	}
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return fmt.Errorf("crear %s: %w", s.dir, err)
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("codificar %s: %w", name, err)
	}
	tmp, err := os.CreateTemp(s.dir, "."+name+".*.tmp")
	if err != nil {
		return fmt.Errorf("crear temporal para %s: %w", name, err)
	}
	defer os.Remove(tmp.Name()) // si el rename funcionó, ya no existe
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		return fmt.Errorf("escribir %s: %w", name, err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("escribir %s: %w", name, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("escribir %s: %w", name, err)
	}
	if err := os.Rename(tmp.Name(), filepath.Join(s.dir, name)); err != nil {
		return fmt.Errorf("guardar %s: %w", name, err)
	}
	return nil
}
