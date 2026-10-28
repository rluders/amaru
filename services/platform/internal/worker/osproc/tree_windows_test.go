//go:build windows

package osproc

import "os/exec"

// Windows tree termination is best-effort and not validated (ADR-0053 item 5): only the helpers
// the shared tests need exist here.
func ignoreTerm()            {}
func detach(*exec.Cmd, bool) {}
