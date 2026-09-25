package sandbox

import (
	"fmt"
	"os/exec"
)

// dirSandbox usa un directorio temporal como home. No aísla nada: los comandos corren con los
// permisos del usuario y ven todo su sistema. Solo guard lo protege; es el último recurso.
type dirSandbox struct {
	base
}

func newDir(opts Options) (*dirSandbox, error) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		return nil, fmt.Errorf("buscar bash: %w", err)
	}
	root, err := newTempRoot()
	if err != nil {
		return nil, err
	}
	s := &dirSandbox{base: base{backend: BackendDir, root: root, hostRoot: root, home: root, opts: opts}}
	s.launch = func(cwd string) launched {
		cmd := exec.Command(bash, "--noprofile", "--norc")
		cmd.Dir = cwd
		cmd.Env = shellEnv(root)
		attach := func(line, cwd, term string) *exec.Cmd {
			c := exec.Command(bash, "--noprofile", "--norc", "-c", line)
			c.Dir = cwd
			c.Env = append(shellEnv(root), "TERM="+term)
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
