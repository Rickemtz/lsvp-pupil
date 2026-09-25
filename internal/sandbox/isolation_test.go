package sandbox

import (
	"os"
	"strings"
	"testing"
)

// Pruebas del aislamiento real de bwrap y container. Varias rutas van en una variable para que
// guard (que en un backend aislado no bloquea lo que no puede comprobar) deje pasar el comando
// y sea el backend el que lo impida.
func TestIsolation(t *testing.T) {
	for _, b := range []Backend{BackendBwrap, BackendContainer} {
		t.Run(string(b), func(t *testing.T) {
			if _, err := Resolve(b); err != nil {
				t.Skip(err)
			}
			sb := newTestSandboxOn(t, b, Options{})
			if !sb.Isolated() || sb.Home() != "/home/alumno" {
				t.Fatalf("Isolated %v, Home %q", sb.Isolated(), sb.Home())
			}
			expect := func(cmd, want string) {
				t.Helper()
				if r := run(t, sb, cmd); strings.TrimSpace(r.Stdout) != want {
					t.Errorf("%s: stdout %q stderr %q, quiero %q", cmd, r.Stdout, r.Stderr, want)
				}
			}
			fails := func(cmd string) {
				t.Helper()
				if r := run(t, sb, cmd); r.Blocked != nil || r.ExitCode == 0 {
					t.Errorf("%s debería fallar dentro del sandbox: %+v", cmd, r)
				}
			}

			expect("whoami", "alumno")
			expect("hostname", "lsvp-pupil")
			expect("echo $HOME", "/home/alumno")
			expect("pwd", "/home/alumno/taller")
			expect("stat -c %U archivo.txt", "alumno")
			expect("ls /home", "alumno")

			fails("d=/usr; touch $d/x")
			fails("d=/etc; touch $d/x")
			fails("d=/; mkdir $d/raiz")
			expect("d=/tmp; touch $d/x && ls $d", "x") // /tmp propio y escribible

			// Sin red: ni siquiera se puede abrir un socket hacia fuera.
			fails("timeout 3 bash -c 'exec 3<>/dev/tcp/1.1.1.1/53'")

			// Solo se ven los procesos del sandbox.
			if r := run(t, sb, "ps -e --no-headers | wc -l"); strings.TrimSpace(r.Stdout) == "" || len(strings.TrimSpace(r.Stdout)) > 1 {
				t.Errorf("ps -e ve %q procesos; quiero menos de 10", strings.TrimSpace(r.Stdout))
			}

			// El home real de quien ejecuta la app no existe dentro.
			if realHome, err := os.UserHomeDir(); err == nil && realHome != "/home/alumno" {
				fails("test -e " + realHome)
			}

			// guard sigue enseñando aunque el backend esté aislado.
			if r := run(t, sb, "rm -rf /"); r.Blocked == nil {
				t.Error("rm -rf / debe bloquearse también con aislamiento")
			}
		})
	}
}

func TestBwrapArgs(t *testing.T) {
	args := strings.Join(bwrapArgs("/tmp/lsvp-pupil-1/home", "/tmp/lsvp-pupil-1/meta", "/home/alumno/taller", "bash", "--noprofile", "--norc"), " ")
	for _, want := range []string{
		"--unshare-all", "--die-with-parent", "--new-session", "--cap-drop ALL",
		"--ro-bind /usr /usr", "--bind /tmp/lsvp-pupil-1/home /home/alumno",
		"--ro-bind /tmp/lsvp-pupil-1/meta/passwd /etc/passwd", "--remount-ro /",
		"--chdir /home/alumno/taller", "-- bash --noprofile --norc",
	} {
		if !strings.Contains(args, want) {
			t.Errorf("faltan %q en: %s", want, args)
		}
	}
	if strings.Contains(args, "--share-net") || strings.Contains(args, "--bind / ") {
		t.Errorf("bwrap no debe compartir la red ni montar / escribible: %s", args)
	}
}

func TestContainerArgs(t *testing.T) {
	args := strings.Join(containerArgs("n", "/tmp/h", "/tmp/m", "/home/alumno/taller", 1000, 1000, false), " ")
	for _, want := range []string{
		"--network none", "--read-only", "--cap-drop ALL", "--security-opt no-new-privileges",
		"--pids-limit 256", "--user 1000:1000", "-v /tmp/h:/home/alumno", "-w /home/alumno/taller",
		"-e HOME=/home/alumno", ContainerImage + " bash --noprofile --norc",
	} {
		if !strings.Contains(args, want) {
			t.Errorf("faltan %q en: %s", want, args)
		}
	}
	if strings.Contains(args, "--userns=keep-id") {
		t.Error("--userns=keep-id es solo para podman")
	}
	if p := strings.Join(containerArgs("n", "/tmp/h", "/tmp/m", "/w", 1, 1, true), " "); !strings.Contains(p, "--userns=keep-id") {
		t.Error("con podman falta --userns=keep-id")
	}
}

func TestParseBackend(t *testing.T) {
	for _, name := range []string{"auto", "bwrap", "container", "dir"} {
		if _, err := ParseBackend(name); err != nil {
			t.Errorf("ParseBackend(%q): %v", name, err)
		}
	}
	if _, err := ParseBackend("magia"); err == nil {
		t.Error("ParseBackend debe rechazar nombres desconocidos")
	}
	b, err := Resolve(BackendAuto)
	if err != nil || b == BackendAuto {
		t.Errorf("Resolve(auto) = %q, %v", b, err)
	}
	if bwrapAvailable() == nil && b != BackendBwrap {
		t.Errorf("con bwrap disponible, auto debería elegirlo; eligió %s", b)
	}
}
