//go:build windows

package osproc

import (
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

func normEnvKey(k string) string { return strings.ToUpper(k) } // Windows names are case-insensitive

// configure starts the child in a new process group so console Ctrl events do not reach us.
func configure(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
}

// Windows has no SIGTERM for arbitrary console processes: taskkill without /F asks the tree to
// end (works for windowed programs), with /F it forces it. taskkill /T follows the parent chain at
// the time of the call, so a descendant whose parent already ended can survive. Best-effort and
// not validated (ADR-0053 item 5).
func interrupt(cmd *exec.Cmd) { taskkill(cmd, false) }
func kill(cmd *exec.Cmd)      { taskkill(cmd, true) }

func taskkill(cmd *exec.Cmd, force bool) {
	if cmd.Process == nil {
		return
	}
	args := []string{"/T", "/PID", strconv.Itoa(cmd.Process.Pid)}
	if force {
		args = append([]string{"/F"}, args...)
	}
	_ = exec.Command("taskkill", args...).Run()
}
