//go:build !windows

package pgsup

import "syscall"

// Alive reports whether a process with this pid exists.
func Alive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || err == syscall.EPERM
}
