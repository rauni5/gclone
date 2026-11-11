//go:build windows

package proc

import (
	"os/exec"
	"strconv"
)

// SetProcGroup is a no-op on Windows; SignalGroup uses taskkill to stop the whole tree.
func SetProcGroup(cmd *exec.Cmd) {}

// SignalGroup stops the child and everything it started. Windows has no
// graceful group signal for console programs, so both paths use taskkill.
func SignalGroup(cmd *exec.Cmd, force bool) {
	if cmd.Process == nil {
		return
	}
	_ = exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid)).Run()
}
