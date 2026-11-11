//go:build !windows

package proc

import (
	"os/exec"
	"syscall"
)

// SetProcGroup puts the child in its own process group so the whole tree
// (npm, node, the dev server) can be stopped together.
func SetProcGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// SignalGroup asks the child's process group to stop (SIGTERM), or forces it (SIGKILL).
func SignalGroup(cmd *exec.Cmd, force bool) {
	if cmd.Process == nil {
		return
	}
	sig := syscall.SIGTERM
	if force {
		sig = syscall.SIGKILL
	}
	_ = syscall.Kill(-cmd.Process.Pid, sig)
}
