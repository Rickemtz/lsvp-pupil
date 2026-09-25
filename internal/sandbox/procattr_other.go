//go:build !linux

package sandbox

import (
	"os/exec"
	"syscall"
)

// setProcAttr pone la shell en su propio grupo de procesos (para matar también a sus hijos).
func setProcAttr(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
}
