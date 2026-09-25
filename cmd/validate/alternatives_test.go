package main

import (
	"os/exec"
	"testing"

	files "lsvp-pupil/content"
	"lsvp-pupil/fixtures"
	"lsvp-pupil/internal/checks"
	"lsvp-pupil/internal/content"
	"lsvp-pupil/internal/sandbox"
)

// Respuestas correctas distintas a la solución de referencia que un alumno escribiría. Si alguna
// deja de resolver su ejercicio, el check es demasiado estricto.
var alternatives = map[string][]string{
	"basicos-009": {"mv file.txt ~/taller/dir3", "mv file.txt ../dir3/"},
	"basicos-013": {"rm -rf proyecto", "rm -r ./proyecto/"},
	"basicos-014": {"cat ~/taller/dia-1/Dewey.txt frases.txt"},
	"basicos-015": {"find ~/taller -name 'eje*.txt'", `find . -name "eje*.txt"`},
	"fs-p03":      {"cd ~", "cd $HOME", "cd ../.."},
	"fs-p05":      {"cat ~/taller/ejemplo.txt"},
	"red-p01":     {"cat < poesia_artificial > poesia_redireccionada"},
	"red-p05":     {"cat poesia_artificial | wc -l"},
	"red-p09":     {"head -n 5 /proc/cpuinfo", "cat /proc/cpuinfo | head -5"},
	"fil-p01":     {"wc -l -w -c datos.csv", "wc -lwc datos.csv"},
	"fil-p03":     {"grep -ic agua datos.csv", "cat datos.csv | grep -i AGUA | wc -l"},
	"fil-p04":     {"grep -vc Verdadero datos.csv"},
	"fil-p05":     {"sed -n 5,10p datos.csv", "head -10 datos.csv | tail -6"},
	"fil-p06":     {"tail -2 datos.csv | head -1"},
	"fil-p07":     {"head datos.csv | cut -d, -f2,6", "head -n 10 datos.csv | cut -d ',' -f 2,6"},
	"fil-p08":     {"sort -t, -k2 datos.csv"},
	"fil-p09":     {"cut -d, -f5 datos.csv | sort -nr | head -5", "tail -n +2 datos.csv | cut -d, -f5 | sort -rn | head -n 5"},
	"fil-p10":     {"grep -c Verdadero datos.csv", "cut -d, -f13 datos.csv | grep Verdadero | wc -l"},
	"fil-p11":     {"grep -vc '^id,' datos.csv", "tail -n 800 datos.csv | wc -l"},
	"fil-p12":     {"cut -d, -f2,12 datos.csv"},
	"fil-p13":     {"cut -d, -f3 datos.csv | sort | uniq -c | sort -rn | head -1"},
	"fil-p14":     {"cut -d, -f3 datos.csv | tail -n +2 | sort -u", "tail -n +2 datos.csv | cut -d, -f3 | sort -u"},
	"proc-p03":    {"kill -s TERM %1", "kill -15 %1", "kill $(pgrep sleep)"},
	"proc-p04":    {"kill -s KILL %1", "kill -KILL %1"},
	"proc-p05":    {"kill -INT %1", "kill -2 %1"},
}

func TestAlternativeSolutions(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash no está instalado")
	}
	backend, err := sandbox.Resolve(sandbox.BackendAuto)
	if err != nil {
		t.Fatal(err)
	}
	cat, err := content.Load(files.Files)
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]content.Exercise{}
	for _, m := range cat.Modules {
		for _, ex := range cat.Exercises(m.ID) {
			byID[ex.ID] = ex
		}
	}
	for id, alts := range alternatives {
		ex, ok := byID[id]
		if !ok {
			t.Errorf("no existe el ejercicio %s", id)
			continue
		}
		for _, alt := range alts {
			t.Run(id+"/"+alt, func(t *testing.T) {
				if backend == sandbox.BackendDir && alt == "kill $(pgrep sleep)" {
					t.Skip("en el backend dir guard bloquea kill con una expansión")
				}
				opts := sandbox.Options{Fixtures: fixtures.Files, Fixture: ex.Fixture, StartDir: ex.StartDir, Setup: ex.Setup}
				var sol sandbox.Result
				var solHome string
				if ex.NeedsSolutionOutput() {
					ref, err := sandbox.New(backend, opts)
					if err != nil {
						t.Fatal(err)
					}
					sol, _ = ref.Run(ex.Solution)
					solHome = ref.Home()
					ref.Close()
				}
				sb, err := sandbox.New(backend, opts)
				if err != nil {
					t.Fatal(err)
				}
				defer sb.Close()
				r, err := sb.Run(alt)
				if err != nil || r.Blocked != nil {
					t.Fatalf("Run: %v %+v", err, r.Blocked)
				}
				st := checks.State{Command: alt, Stdout: r.Stdout, ExitCode: r.ExitCode, Cwd: r.Cwd, Home: sb.Home(),
					HostPath: sb.HostPath, SolutionStdout: sol.Stdout, SolutionHome: solHome}
				if ex.NeedsProcesses() {
					if st.Processes, err = processes(sb); err != nil {
						t.Fatal(err)
					}
				}
				if fails := checks.Run(ex.Checks, st); len(fails) > 0 {
					t.Errorf("no resuelve: %s\nsalida: %q", describe(fails), r.Stdout)
				}
			})
		}
	}
}
