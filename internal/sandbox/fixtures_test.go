package sandbox

import (
	"strings"
	"testing"

	"lsvp-pupil/fixtures"
)

// Los comandos del curso funcionan sobre los fixtures generados.
func TestCourseCommandsOnFixtures(t *testing.T) {
	forEachBackend(t, func(t *testing.T, b Backend) {
		sb := newTestSandboxOn(t, b, Options{Fixtures: fixtures.Files, Fixture: "taller", StartDir: "taller"})
		r := run(t, sb, "find ~/taller -name 'eje*.txt' | sort")
		want := strings.Join([]string{"ejeDatos.txt", "eje_practica.txt", "ejemplo.txt", "ejes.txt"}, "\n")
		got := strings.ReplaceAll(r.Stdout, sb.Home()+"/taller/", "")
		if strings.TrimSpace(got) != want {
			t.Errorf("find eje*.txt =\n%s\nquiero\n%s", got, want)
		}
		if r := run(t, sb, "ls"); strings.Contains(r.Stdout, ".oculto") {
			t.Error("ls sin -a no debe mostrar .oculto")
		}
		if r := run(t, sb, "ls -a"); !strings.Contains(r.Stdout, ".oculto") {
			t.Error("ls -a debe mostrar .oculto")
		}

		sb2 := newTestSandboxOn(t, b, Options{Fixtures: fixtures.Files, Fixture: "taller2", StartDir: "taller2/dia3/redireccionYPipes/red_pipes"})
		checks := map[string]string{
			"wc -l < datos.csv": "801\n",
			"tail -n 800 datos.csv | cut -d ',' -f 3 | sort | uniq -c | sort -rn | head -n 1 | awk '{print $2}'": "Agua\n",
			"head -n 1 datos.csv | cut -d , -f 2,6": "nombre,hp\n",
			"cat poesia_artificial > poesia_redireccionada && cmp -s poesia_artificial poesia_redireccionada && echo igual": "igual\n",
		}
		for cmd, want := range checks {
			if r := run(t, sb2, cmd); r.Stdout != want {
				t.Errorf("%s = %q (stderr %q), quiero %q", cmd, r.Stdout, r.Stderr, want)
			}
		}
	})
}
