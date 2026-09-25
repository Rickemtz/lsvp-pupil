package sandbox

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// visibleHome es el home dentro de bwrap y del contenedor.
const visibleHome = "/home/" + User

// bwrapSandbox aísla con bubblewrap: namespaces propios (usuario, procesos, red, hostname...),
// el sistema del equipo en solo lectura, sin red y con el directorio del ejercicio como único
// lugar escribible (además de un /tmp propio).
type bwrapSandbox struct {
	base
}

func newBwrap(opts Options) (*bwrapSandbox, error) {
	bwrap, err := exec.LookPath("bwrap")
	if err != nil {
		return nil, fmt.Errorf("buscar bwrap: %w", err)
	}
	root, err := newTempRoot()
	if err != nil {
		return nil, err
	}
	home, meta := filepath.Join(root, "home"), filepath.Join(root, "meta")
	s := &bwrapSandbox{base: base{backend: BackendBwrap, isolated: true, root: root, hostRoot: home, home: visibleHome, opts: opts}}
	if err := os.MkdirAll(home, 0o755); err != nil {
		_ = s.Close()
		return nil, fmt.Errorf("crear home del sandbox: %w", err)
	}
	if err := writeMeta(meta, os.Getuid(), os.Getgid()); err != nil {
		_ = s.Close()
		return nil, err
	}
	s.launch = func(cwd string) launched {
		cmd := exec.Command(bwrap, bwrapArgs(home, meta, cwd, "bash", "--noprofile", "--norc")...)
		cmd.Env = shellEnv(visibleHome)
		attach := func(line, cwd, term string) *exec.Cmd {
			env := append(shellEnv(visibleHome), "TERM="+term)
			// Con nsenter el programa entra al mismo sandbox y ve sus procesos (htop, ps).
			if nsenter, err := exec.LookPath("nsenter"); err == nil && cmd.Process != nil {
				if target := firstChild(cmd.Process.Pid); target > 0 {
					// --wd=RUTA se abriría antes de entrar al sandbox; por eso se usa la raíz y el
					// directorio del proceso destino y el cd se hace ya adentro.
					c := exec.Command(nsenter, "--target", strconv.Itoa(target), "--user", "--preserve-credentials",
						"--pid", "--mount", "--uts", "--ipc", "--net", "--root", "--wd",
						"--", "bash", "--noprofile", "--norc", "-c", `cd -- "$1" || exit; eval "$2"`, "bash", cwd, line)
					c.Env = env
					return c
				}
			}
			// Sin nsenter: otro bwrap con los mismos archivos, pero con su propia lista de procesos.
			c := exec.Command(bwrap, bwrapArgs(home, meta, cwd, "bash", "--noprofile", "--norc", "-c", line)...)
			c.Env = env
			return c
		}
		return launched{cmd: cmd, attach: attach}
	}
	if err := s.init(); err != nil {
		_ = s.Close()
		return nil, err
	}
	return s, nil
}

// bwrapArgs arma la línea de bwrap. home y meta son rutas de este equipo; cwd es la ruta dentro.
func bwrapArgs(home, meta, cwd string, command ...string) []string {
	args := []string{
		"--unshare-all", // usuario, procesos, red, IPC, UTS y cgroup propios: sin red
		"--die-with-parent",
		"--new-session", // otra sesión: el sandbox no puede inyectar teclas en la terminal real
		"--cap-drop", "ALL",
		"--hostname", Hostname,
		"--ro-bind", "/usr", "/usr",
	}
	// /bin, /lib... son enlaces a /usr en los sistemas modernos y directorios en los viejos.
	for _, d := range []string{"/bin", "/sbin", "/lib", "/lib32", "/lib64"} {
		info, err := os.Lstat(d)
		switch {
		case err != nil:
		case info.Mode()&os.ModeSymlink != 0:
			if target, err := os.Readlink(d); err == nil {
				args = append(args, "--symlink", target, d)
			}
		case info.IsDir():
			args = append(args, "--ro-bind", d, d)
		}
	}
	args = append(args,
		// /etc en solo lectura (locale, alternativas de Debian...) con passwd y group propios encima.
		"--ro-bind", "/etc", "/etc",
		"--ro-bind", filepath.Join(meta, "passwd"), "/etc/passwd",
		"--ro-bind", filepath.Join(meta, "group"), "/etc/group",
		"--ro-bind", filepath.Join(meta, "hostname"), "/etc/hostname",
		"--proc", "/proc", // solo los procesos del sandbox
		"--dev", "/dev", // null, zero, random, tty... sin discos
		"--tmpfs", "/tmp",
		"--dir", "/var/tmp",
		"--bind", home, visibleHome,
		"--remount-ro", "/", // la raíz que arma bwrap queda de solo lectura
		"--chdir", cwd,
		"--",
	)
	return append(args, command...)
}

// firstChild devuelve el PID (en este equipo) del primer hijo de pid, o 0.
func firstChild(pid int) int {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/task/%d/children", pid, pid))
	if err != nil {
		return 0
	}
	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return 0
	}
	child, _ := strconv.Atoi(fields[0])
	return child
}
