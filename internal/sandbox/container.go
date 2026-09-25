package sandbox

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// ContainerImage es la imagen que usa el backend container (make image la construye).
const ContainerImage = "lsvp-pupil-sandbox:1"

// containerSandbox corre la shell en un contenedor desechable de docker o podman: sin red,
// raíz de solo lectura, sin capacidades y con el home del ejercicio montado.
type containerSandbox struct {
	base
}

func newContainer(opts Options) (*containerSandbox, error) {
	if err := containerAvailable(); err != nil {
		return nil, err
	}
	runtime := containerRuntime
	root, err := newTempRoot()
	if err != nil {
		return nil, err
	}
	home, meta := filepath.Join(root, "home"), filepath.Join(root, "meta")
	s := &containerSandbox{base: base{backend: BackendContainer, isolated: true, root: root, hostRoot: home, home: visibleHome, opts: opts}}
	if err := os.MkdirAll(home, 0o755); err != nil {
		_ = s.Close()
		return nil, fmt.Errorf("crear home del sandbox: %w", err)
	}
	uid, gid := os.Getuid(), os.Getgid()
	if err := writeMeta(meta, uid, gid); err != nil {
		_ = s.Close()
		return nil, err
	}
	podman := filepath.Base(runtime) == "podman"
	s.launch = func(cwd string) launched {
		name := "lsvp-pupil-" + newNonce()[:12]
		cmd := exec.Command(runtime, containerArgs(name, home, meta, cwd, uid, gid, podman)...)
		// El cliente de docker/podman necesita el entorno del usuario (DOCKER_HOST, XDG_RUNTIME_DIR...);
		// el entorno de la shell va con -e.
		cmd.Env = os.Environ()
		// Matar al cliente no detiene el contenedor: hay que borrarlo aparte.
		cleanup := func() { _ = exec.Command(runtime, "rm", "-f", name).Run() }
		attach := func(line, cwd, term string) *exec.Cmd {
			c := exec.Command(runtime, "exec", "-i", "-t", "-w", cwd, "-e", "TERM="+term,
				name, "bash", "--noprofile", "--norc", "-c", line)
			c.Env = os.Environ()
			return c
		}
		return launched{cmd: cmd, cleanup: cleanup, attach: attach}
	}
	if err := s.init(); err != nil {
		_ = s.Close()
		return nil, err
	}
	return s, nil
}

func containerArgs(name, home, meta, cwd string, uid, gid int, podman bool) []string {
	args := []string{
		"run", "--rm", "-i", // -i sin -t: stdout y stderr llegan por separado
		"--name", name,
		"--network", "none",
		"--hostname", Hostname,
		"--user", fmt.Sprintf("%d:%d", uid, gid),
		"--cap-drop", "ALL",
		"--security-opt", "no-new-privileges",
		"--pids-limit", "256", // una fork bomb que escape a guard no tumba el equipo
		"--memory", "512m",
		"--read-only",
		"--tmpfs", "/tmp",
		"-v", home + ":" + visibleHome,
		"-v", filepath.Join(meta, "passwd") + ":/etc/passwd:ro",
		"-v", filepath.Join(meta, "group") + ":/etc/group:ro",
		"-w", cwd,
	}
	// La zona horaria del equipo, para que ls -l y date muestren la hora local.
	if _, err := os.Stat("/etc/localtime"); err == nil {
		args = append(args, "-v", "/etc/localtime:/etc/localtime:ro")
	}
	if podman {
		args = append(args, "--userns=keep-id") // sin root: el uid de dentro coincide con el de fuera
	}
	for _, kv := range shellEnv(visibleHome) {
		args = append(args, "-e", kv)
	}
	return append(args, ContainerImage, "bash", "--noprofile", "--norc")
}
