//go:build !windows

package osproc

import (
	"errors"
	"os/exec"
	"syscall"
)

func normEnvKey(k string) string { return k }

// configure puts the child in its own process group, so one signal reaches its whole tree. A
// descendant that calls setsid or setpgid leaves the group and is not reached; that is a known
// limit of process groups (ADR-0053), not something this layer hides.
func configure(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} }

func interrupt(cmd *exec.Cmd) { signalGroup(cmd, syscall.SIGTERM) }
func kill(cmd *exec.Cmd)      { signalGroup(cmd, syscall.SIGKILL) }

func signalGroup(cmd *exec.Cmd, sig syscall.Signal) {
	if cmd.Process == nil {
		return
	}
	// A negative pid is the process group; pgid equals the pid because of Setpgid.
	if err := syscall.Kill(-cmd.Process.Pid, sig); err != nil && !errors.Is(err, syscall.ESRCH) {
		_ = cmd.Process.Signal(sig)
	}
}
