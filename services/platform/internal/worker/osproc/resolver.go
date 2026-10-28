package osproc

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/rluders/amaru/services/platform/internal/ports"
)

// Resolver finds executables: an explicit path, then PATH (exec.LookPath, which honors PATHEXT on
// Windows), then ExtraDirs. A Provider that knows where its program installs passes those
// directories; the OS layer names no vendor's directory.
type Resolver struct {
	ExtraDirs []string
}

func (r Resolver) Find(name string) (string, error) {
	if strings.ContainsAny(name, `/\`) { // an explicit path
		if isExecFile(name) {
			return filepath.Abs(name)
		}
		return "", fmt.Errorf("%s: %w", name, ports.ErrExecutableNotFound)
	}
	if p, err := exec.LookPath(name); err == nil {
		return filepath.Abs(p)
	}
	for _, dir := range r.ExtraDirs {
		for _, cand := range candidates(name) {
			if p := filepath.Join(dir, cand); isExecFile(p) {
				return p, nil
			}
		}
	}
	return "", fmt.Errorf("%s: %w", name, ports.ErrExecutableNotFound)
}

func candidates(name string) []string {
	if runtime.GOOS == "windows" {
		return []string{name + ".exe", name + ".cmd", name + ".bat"}
	}
	return []string{name}
}

func isExecFile(p string) bool {
	st, err := os.Stat(p)
	if err != nil || st.IsDir() {
		return false
	}
	if runtime.GOOS == "windows" {
		return true // no exec bit; the extension decides at start
	}
	return st.Mode().Perm()&0o111 != 0
}

var _ ports.ExecutableResolver = Resolver{}
