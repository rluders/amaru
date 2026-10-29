//go:build !linux

package osproc

import (
	"errors"
	"fmt"
	"runtime"
)

// No OS other than Linux has an enforcing mechanism in this layer (ADR-0057 item 3): a Step that
// requests write confinement is refused there.

func landlockABI() (int, error) {
	return 0, fmt.Errorf("no write-confinement mechanism on %s", runtime.GOOS)
}

func enforce([]string) error { return errors.New("no write-confinement mechanism on this OS") }

func execve(string, []string, []string) error { return errors.New("not supported on this OS") }
