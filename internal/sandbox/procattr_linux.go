package sandbox

import (
	"os/exec"
	"syscall"
)

// setProcAttr pone la shell en su propio grupo de procesos (para matar también a sus hijos)
// y hace que el kernel la mate si lsvp-pupil muere.
func setProcAttr(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
	cmd.SysProcAttr.Pdeathsig = syscall.SIGKILL
}
