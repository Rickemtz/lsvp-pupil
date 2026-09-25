package sandbox

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

var testFixtures = fstest.MapFS{
	"taller/archivo.txt":     {Data: []byte("hola\n")},
	"taller/dia-1/Dewey.txt": {Data: []byte("Dewey\n")},
	"taller/.oculto":         {Data: []byte("secreto\n")},
}

func newTestSandbox(t *testing.T, opts Options) Sandbox {
	t.Helper()
	return newTestSandboxOn(t, BackendDir, opts)
}

func newTestSandboxOn(t *testing.T, b Backend, opts Options) Sandbox {
	t.Helper()
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash no está instalado")
	}
	if opts.Fixtures == nil {
		opts.Fixtures = testFixtures
	}
	if opts.Fixture == "" {
		opts.Fixture = "taller"
	}
	if opts.StartDir == "" {
		opts.StartDir = "taller"
	}
	sb, err := New(b, opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sb.Close() })
	return sb
}

// forEachBackend corre fn con cada backend disponible en este equipo; los que faltan se saltan.
func forEachBackend(t *testing.T, fn func(t *testing.T, b Backend)) {
	for _, b := range []Backend{BackendDir, BackendBwrap, BackendContainer} {
		t.Run(string(b), func(t *testing.T) {
			if _, err := Resolve(b); err != nil {
				t.Skip(err)
			}
			fn(t, b)
		})
	}
}

func run(t *testing.T, sb Sandbox, cmd string) Result {
	t.Helper()
	r, err := sb.Run(cmd)
	if err != nil {
		t.Fatalf("Run(%q): %v", cmd, err)
	}
	return r
}

func TestMarkerProtocol(t *testing.T) {
	forEachBackend(t, func(t *testing.T, b Backend) {
		sb := newTestSandboxOn(t, b, Options{})
		tests := []struct {
			cmd            string
			stdout, stderr string
			code           int
		}{
			{"echo hola", "hola\n", "", 0},
			{"printf 'a\\nb\\n'", "a\nb\n", "", 0},
			{"printf 'sin salto'", "sin salto", "", 0},
			{"printf '\\n\\n'", "\n\n", "", 0},
			{"true", "", "", 0},
			{"false", "", "", 1},
			{"(exit 3)", "", "", 3},
			{"echo error >&2", "", "error\n", 0},
			{"echo out; echo err >&2; printf fin", "out\nfin", "err\n", 0},
			{"ls noexiste", "", "ls: cannot access 'noexiste': No such file or directory\n", 2},
			{"echo ñandú 🐧", "ñandú 🐧\n", "", 0},
			{"cat archivo.txt dia-1/Dewey.txt", "hola\nDewey\n", "", 0},
			{"echo '__LDJ_END__ 0 /'", "__LDJ_END__ 0 /\n", "", 0}, // un marcador falso no confunde al protocolo
			{"echo \"hola", "", "", 2},                             // comilla sin cerrar: error de sintaxis, no se cuelga
			{"cat", "", "", 0},                                     // stdin es /dev/null
			{"read x; echo \"[$x]\"", "[]\n", "", 0},
		}
		for _, tt := range tests {
			r := run(t, sb, tt.cmd)
			if r.Stdout != tt.stdout || r.ExitCode != tt.code {
				t.Errorf("%q: stdout %q código %d; quiero %q código %d (stderr %q)", tt.cmd, r.Stdout, r.ExitCode, tt.stdout, tt.code, r.Stderr)
			}
			if tt.stderr != "" && r.Stderr != tt.stderr {
				t.Errorf("%q: stderr %q; quiero %q", tt.cmd, r.Stderr, tt.stderr)
			}
		}
		if r := run(t, sb, "echo \"hola"); !strings.Contains(r.Stderr, "unexpected EOF") {
			t.Errorf("comilla sin cerrar: stderr %q", r.Stderr)
		}
	})
}

func TestPersistence(t *testing.T) {
	forEachBackend(t, func(t *testing.T, b Backend) {
		sb := newTestSandboxOn(t, b, Options{})
		home := sb.Home()
		if got := sb.Cwd(); got != home+"/taller" {
			t.Errorf("cwd inicial = %q", got)
		}
		if r := run(t, sb, "cd dia-1"); r.Cwd != home+"/taller/dia-1" {
			t.Errorf("cd: cwd = %q", r.Cwd)
		}
		if r := run(t, sb, "pwd"); r.Stdout != home+"/taller/dia-1\n" {
			t.Errorf("pwd = %q", r.Stdout)
		}
		run(t, sb, "x=5; export Y=7")
		if r := run(t, sb, "echo $x $Y"); r.Stdout != "5 7\n" {
			t.Errorf("variables = %q", r.Stdout)
		}
		if r := run(t, sb, "cd; pwd; echo $HOME $USER"); r.Stdout != home+"\n"+home+" alumno\n" {
			t.Errorf("home = %q", r.Stdout)
		}
		if r := run(t, sb, "ls -a ~/taller"); !strings.Contains(r.Stdout, ".oculto") {
			t.Errorf("los archivos ocultos deben copiarse: %q", r.Stdout)
		}
	})
}

func TestTimeout(t *testing.T) {
	forEachBackend(t, func(t *testing.T, b Backend) {
		sb := newTestSandboxOn(t, b, Options{Timeout: 300 * time.Millisecond})
		run(t, sb, "cd dia-1")
		start := time.Now()
		r := run(t, sb, "echo antes; sleep 10")
		if !r.TimedOut || !r.Restarted || r.ExitCode != 124 {
			t.Errorf("sleep 10: %+v", r)
		}
		if time.Since(start) > 3*time.Second {
			t.Errorf("el tiempo agotado tardó %v", time.Since(start))
		}
		if r.Stdout != "antes\n" {
			t.Errorf("la salida previa al tiempo agotado se pierde: %q", r.Stdout)
		}
		if r := run(t, sb, "pwd"); r.Stdout != sb.Home()+"/taller/dia-1\n" {
			t.Errorf("tras reiniciar debe seguir en el mismo directorio: %q", r.Stdout)
		}
	})
}

func TestInfiniteOutput(t *testing.T) {
	forEachBackend(t, func(t *testing.T, b Backend) {
		sb := newTestSandboxOn(t, b, Options{Timeout: 300 * time.Millisecond, MaxOutput: 1000})
		r := run(t, sb, "yes")
		if !r.TimedOut || !r.Truncated || len(r.Stdout) > 1000 {
			t.Errorf("yes: timedOut %v truncated %v len %d", r.TimedOut, r.Truncated, len(r.Stdout))
		}
		r = run(t, sb, "head -c 5000 /dev/zero | tr '\\0' a")
		if r.TimedOut || !r.Truncated || len(r.Stdout) != 1000 {
			t.Errorf("5000 bytes: timedOut %v truncated %v len %d", r.TimedOut, r.Truncated, len(r.Stdout))
		}
		if r := run(t, sb, "echo ok"); r.Stdout != "ok\n" || r.Truncated {
			t.Errorf("después de truncar: %+v", r)
		}
	})
}

func TestExitRestartsShell(t *testing.T) {
	forEachBackend(t, func(t *testing.T, b Backend) {
		sb := newTestSandboxOn(t, b, Options{})
		run(t, sb, "cd dia-1; x=1")
		r := run(t, sb, "echo adios; exit 7")
		if !r.Restarted || r.ExitCode != 7 || r.Stdout != "adios\n" {
			t.Errorf("exit 7: %+v", r)
		}
		if r := run(t, sb, "pwd; echo \"[$x]\""); r.Stdout != sb.Home()+"/taller/dia-1\n[]\n" {
			t.Errorf("tras exit: %q", r.Stdout)
		}
		if r := run(t, sb, "set -e; false"); !r.Restarted {
			t.Errorf("set -e; false también termina la shell: %+v", r)
		}
		if r := run(t, sb, "echo sigo"); r.Stdout != "sigo\n" {
			t.Errorf("después de set -e: %q", r.Stdout)
		}
	})
}

func TestSetup(t *testing.T) {
	sb := newTestSandbox(t, Options{StartDir: "taller/dir1", Setup: []string{"mkdir -p ../dir3 && touch file.txt"}})
	if _, err := os.Stat(sb.HostPath("taller/dir1/file.txt")); err != nil {
		t.Errorf("setup no creó file.txt: %v", err)
	}
	if _, err := os.Stat(sb.HostPath("taller/dir3")); err != nil {
		t.Errorf("setup no creó dir3: %v", err)
	}
	if _, err := New(BackendDir, Options{Fixtures: testFixtures, Fixture: "taller", Setup: []string{"false"}}); err == nil {
		t.Error("un setup que falla debe dar error")
	}
	if _, err := New(BackendDir, Options{Fixtures: testFixtures, StartDir: "../.."}); err == nil {
		t.Error("start_dir fuera del home debe dar error")
	}
}

func TestGuardBlocksBeforeRunning(t *testing.T) {
	forEachBackend(t, func(t *testing.T, b Backend) {
		sb := newTestSandboxOn(t, b, Options{})
		r := run(t, sb, "touch creado; sudo ls")
		if r.Blocked == nil || r.Blocked.Rule != RulePrivilege {
			t.Fatalf("sudo no se bloqueó: %+v", r)
		}
		if _, err := os.Stat(sb.HostPath("taller/creado")); !errors.Is(err, os.ErrNotExist) {
			t.Error("una línea bloqueada no debe ejecutarse ni en parte")
		}
		if r := run(t, sb, "rm -rf ../.."); r.Blocked == nil {
			t.Error("rm -rf ../.. debería bloquearse")
		}
		// guard usa el directorio actual real de la shell.
		run(t, sb, "cd ..")
		if r := run(t, sb, "rm -rf taller/dia-1"); r.Blocked != nil {
			t.Errorf("rm dentro del sandbox bloqueado: %+v", r.Blocked)
		}
	})
}

func TestHostPath(t *testing.T) {
	forEachBackend(t, func(t *testing.T, b Backend) {
		sb := newTestSandboxOn(t, b, Options{})
		root := sb.HostPath("")
		if got := sb.HostPath("taller/archivo.txt"); got != filepath.Join(root, "taller", "archivo.txt") {
			t.Errorf("HostPath = %q", got)
		}
		if got := sb.HostPath("../../etc/passwd"); !strings.HasPrefix(got, root+"/") {
			t.Errorf("HostPath no debe salir del home: %q", got)
		}
		// Lo que se crea dentro aparece en HostPath, del lado del equipo.
		run(t, sb, "echo hola > nuevo.txt")
		if data, err := os.ReadFile(sb.HostPath("taller/nuevo.txt")); err != nil || string(data) != "hola\n" {
			t.Errorf("nuevo.txt visto desde fuera: %q %v", data, err)
		}
	})
}

func TestCloseKillsProcessesAndRemovesDir(t *testing.T) {
	forEachBackend(t, func(t *testing.T, b Backend) {
		sb := newTestSandboxOn(t, b, Options{})
		// Los PID de dentro de bwrap o del contenedor no son los del equipo: se busca el proceso
		// por un argumento único.
		marker := fmt.Sprintf("%d.%d", 60+time.Now().Nanosecond()%1000, os.Getpid())
		run(t, sb, "sleep "+marker+" &")
		if !sleepAlive(marker) {
			t.Fatal("el proceso en segundo plano no arrancó")
		}
		// Un directorio y un archivo sin permisos no deben impedir el borrado.
		run(t, sb, "mkdir -p cerrado/sub && touch cerrado/sub/x && chmod 000 cerrado/sub cerrado")
		home := sb.HostPath("")
		if err := sb.Close(); err != nil {
			t.Fatal(err)
		}
		deadline := time.Now().Add(5 * time.Second)
		for sleepAlive(marker) {
			if time.Now().After(deadline) {
				t.Fatalf("sleep %s sigue vivo después de Close", marker)
			}
			time.Sleep(50 * time.Millisecond)
		}
		if _, err := os.Stat(home); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("el directorio del sandbox sigue existiendo: %v", err)
		}
		if _, err := sb.Run("echo"); err == nil {
			t.Error("Run después de Close debe dar error")
		}
		if err := sb.Close(); err != nil {
			t.Errorf("cerrar dos veces: %v", err)
		}
	})
}

func TestCloseDoesNotFollowSymlinks(t *testing.T) {
	outside := t.TempDir()
	if err := os.Chmod(outside, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(outside, 0o700) })
	sb := newTestSandbox(t, Options{})
	// guard bloquea «ln -s /ruta»; el enlace se crea desde Go para probar solo la limpieza.
	if err := os.Symlink(outside, sb.HostPath("taller/afuera")); err != nil {
		t.Fatal(err)
	}
	if err := sb.Close(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(outside)
	if err != nil {
		t.Fatalf("el destino del enlace desapareció: %v", err)
	}
	if info.Mode().Perm() != 0o500 {
		t.Errorf("Close cambió los permisos de un directorio fuera del sandbox: %v", info.Mode().Perm())
	}
}

func TestRemoveSandboxDirRefusesForeignPaths(t *testing.T) {
	for _, p := range []string{"/", "", "relativo", os.TempDir(), filepath.Join(os.TempDir(), "otra-cosa"), "/home"} {
		if err := removeSandboxDir(p); err == nil {
			t.Errorf("removeSandboxDir(%q) no se negó", p)
		}
	}
}

func TestUnknownBackend(t *testing.T) {
	if _, err := New("magia", Options{}); err == nil {
		t.Error("un backend desconocido debe dar error")
	}
}

func TestTree(t *testing.T) {
	forEachBackend(t, func(t *testing.T, b Backend) {
		sb := newTestSandboxOn(t, b, Options{})
		run(t, sb, "ln -s archivo.txt enlace")
		want := "~\n└── taller/\n    ├── .oculto\n    ├── archivo.txt\n    ├── dia-1/\n    │   └── Dewey.txt\n    └── enlace -> archivo.txt"
		if got := Tree(sb); got != want {
			t.Errorf("Tree =\n%s\nquiero\n%s", got, want)
		}
	})
}

func TestCloseDuringRun(t *testing.T) {
	forEachBackend(t, func(t *testing.T, b Backend) {
		sb := newTestSandboxOn(t, b, Options{Timeout: 10 * time.Second})
		done := make(chan error, 1)
		go func() {
			_, err := sb.Run("sleep 8")
			done <- err
		}()
		time.Sleep(200 * time.Millisecond)
		start := time.Now()
		if err := sb.Close(); err != nil {
			t.Fatal(err)
		}
		if d := time.Since(start); d > 2*time.Second {
			t.Errorf("Close esperó %v a que terminara el comando", d)
		}
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Fatal("Run no terminó después de Close")
		}
	})
}

// sleepAlive busca en el equipo un proceso «sleep <marker>».
func sleepAlive(marker string) bool {
	entries, _ := os.ReadDir("/proc")
	for _, e := range entries {
		if _, err := strconv.Atoi(e.Name()); err != nil {
			continue
		}
		cmdline, err := os.ReadFile("/proc/" + e.Name() + "/cmdline")
		if err == nil && string(cmdline) == "sleep\x00"+marker+"\x00" {
			return true
		}
	}
	return false
}
