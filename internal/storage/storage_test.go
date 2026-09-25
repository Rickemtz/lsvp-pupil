package storage

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"lsvp-pupil/internal/content"
	"lsvp-pupil/internal/scoring"
)

func TestProgressRoundTrip(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "no", "existe", "aun")
	s := Open(dir)
	p, err := s.LoadProgress()
	if err != nil || len(p.Modules) != 0 {
		t.Fatalf("sin archivo: %+v %v", p, err)
	}
	p.RecordModule(1, scoring.RankB, 72)
	p.Solved["intro-001"] = true
	p.Misses["https://x#kernel"] = 2
	p.RecordExam(scoring.RankA)
	if err := s.SaveProgress(p); err != nil {
		t.Fatal(err)
	}
	got, err := s.LoadProgress()
	if err != nil {
		t.Fatal(err)
	}
	if got.Modules[1].BestRank != scoring.RankB || !got.Solved["intro-001"] || got.Misses["https://x#kernel"] != 2 || got.ExamRank != scoring.RankA {
		t.Errorf("leído: %+v", got)
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Errorf("quedó un temporal: %s", e.Name())
		}
	}
}

func TestCorruptFileIsBackedUp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, progressFile)
	if err := os.WriteFile(path, []byte("{esto no es json"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := Open(dir).LoadProgress()
	if !errors.Is(err, ErrCorrupt) {
		t.Fatalf("error = %v, quiero ErrCorrupt", err)
	}
	if p.Modules == nil || len(p.Modules) != 0 {
		t.Errorf("tras un archivo dañado se empieza de cero: %+v", p)
	}
	if data, _ := os.ReadFile(path + ".bak"); string(data) != "{esto no es json" {
		t.Errorf(".bak = %q", data)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("el archivo dañado debe moverse a .bak")
	}
	// Un JSON válido pero incompleto (sin mapas) no debe dejar mapas nil.
	os.WriteFile(filepath.Join(dir, progressFile), []byte(`{"exam_rank":"B"}`), 0o644)
	p, err = Open(dir).LoadProgress()
	if err != nil || p.Solved == nil || p.Misses == nil || p.Modules == nil || p.ExamRank != scoring.RankB {
		t.Errorf("JSON parcial: %+v %v", p, err)
	}
}

func TestMemoryOnlyStore(t *testing.T) {
	s := Open("")
	if err := s.SaveProgress(newProgress()); err != nil {
		t.Error(err)
	}
	if _, err := s.LoadScores(); err != nil {
		t.Error(err)
	}
}

func TestUnlocked(t *testing.T) {
	mods := []content.Module{{ID: 1, Open: true}, {ID: 2, Open: true}, {ID: 3, Open: true}, {ID: 4}, {ID: 5}, {ID: 6}}
	p := newProgress()
	u := p.Unlocked(mods)
	if !u[1] || !u[3] || u[4] {
		t.Errorf("inicio: %v", u)
	}
	p.RecordModule(3, scoring.RankD, 30)
	if p.Unlocked(mods)[4] {
		t.Error("rango D no desbloquea")
	}
	p.RecordModule(3, scoring.RankC, 55)
	p.RecordModule(4, scoring.RankS, 99)
	u = p.Unlocked(mods)
	if !u[4] || !u[5] || u[6] {
		t.Errorf("tras C en 3 y S en 4: %v", u)
	}
}

func TestRecordModuleKeepsBest(t *testing.T) {
	p := newProgress()
	if !p.RecordModule(2, scoring.RankB, 75) {
		t.Error("el primer resultado es el mejor")
	}
	if p.RecordModule(2, scoring.RankD, 20) {
		t.Error("un resultado peor no es récord")
	}
	if m := p.Modules[2]; m.BestRank != scoring.RankB || m.Completed != 2 {
		t.Errorf("módulo 2: %+v", m)
	}
	p.RecordExam(scoring.RankC)
	p.RecordExam(scoring.RankD)
	p.RecordExam(scoring.RankA)
	if p.ExamRank != scoring.RankA {
		t.Errorf("examen: %s", p.ExamRank)
	}
}

func TestScoresTopTen(t *testing.T) {
	sc := Scores{}
	now := time.Unix(0, 0)
	pos, rec := sc.Add("quiz", Score{Points: 500, Date: now})
	if pos != 1 || !rec {
		t.Errorf("primer puntaje: pos %d récord %v", pos, rec)
	}
	for i := 1; i <= 12; i++ {
		sc.Add("quiz", Score{Points: i * 10, Date: now.Add(time.Duration(i))})
	}
	if len(sc["quiz"]) != 10 || sc["quiz"][0].Points != 500 {
		t.Errorf("top: %+v", sc["quiz"])
	}
	if pos, rec := sc.Add("quiz", Score{Points: 1, Date: now}); pos != 0 || rec {
		t.Errorf("un puntaje que no entra: pos %d récord %v", pos, rec)
	}
	if pos, rec := sc.Add("quiz", Score{Points: 500, Date: now.Add(time.Hour)}); pos != 2 || rec {
		t.Errorf("empatar el récord no es nuevo récord: pos %d récord %v", pos, rec)
	}
	if pos, rec := sc.Add("quiz", Score{Points: 900, Date: now}); pos != 1 || !rec {
		t.Errorf("nuevo récord: pos %d récord %v", pos, rec)
	}
	if _, rec := sc.Add("otro", Score{Points: 0}); rec {
		t.Error("0 puntos nunca es récord")
	}
}

func TestScoresRoundTrip(t *testing.T) {
	s := Open(t.TempDir())
	sc := Scores{}
	sc.Add("examen", Score{Points: 1234, Rank: scoring.RankA, Percent: 88.5, Duration: 95 * time.Second, Date: time.Date(2026, 9, 23, 10, 0, 0, 0, time.UTC)})
	if err := s.SaveScores(sc); err != nil {
		t.Fatal(err)
	}
	got, err := s.LoadScores()
	if err != nil || len(got["examen"]) != 1 || got["examen"][0] != sc["examen"][0] {
		t.Errorf("leído: %+v %v", got, err)
	}
}
