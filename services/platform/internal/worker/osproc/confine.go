package osproc

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/rluders/amaru/services/platform/internal/ports"
)

// Write confinement (ADR-0057). A confined process is started through a launch helper: the Runner
// starts the Worker's own binary with confineEnv set; that binary calls ConfineExecMain first
// thing, applies the restriction to itself and then replaces itself with the agent program, so the
// agent and its descendants inherit it and nothing else does. The restriction is Landlock on
// Linux (confine_linux.go); no other OS has a mechanism, so a confined Step is refused there.

const (
	// MinLandlockABI is the lowest Landlock ABI that covers every write right ADR-0057 names
	// (truncate arrived in ABI 3, renaming and linking across directories in ABI 2).
	MinLandlockABI = 3

	confineEnv = "AMARU_CONFINE_EXEC"
	confineSep = "--"
	// exitConfine is the helper's exit code when it could not confine; it never runs the program.
	exitConfine = 125
)

// ConfineExecMain must be the first thing the Worker's main (and any test binary that starts
// confined processes) does. In a normal run it returns at once. In a launch-helper run it
// confines the process and replaces it with the requested program, and does not return.
func ConfineExecMain() {
	if os.Getenv(confineEnv) == "" {
		return
	}
	roots, prog, args, err := parseConfineArgs(os.Args[1:])
	if err == nil {
		err = enforce(roots)
	}
	var path string
	if err == nil {
		path, err = exec.LookPath(prog)
	}
	if err == nil {
		err = execve(path, append([]string{prog}, args...), withoutKey(os.Environ(), confineEnv))
	}
	fmt.Fprintf(os.Stderr, "amaru confine: %v\n", err)
	os.Exit(exitConfine)
}

func parseConfineArgs(argv []string) (roots []string, prog string, args []string, err error) {
	for i, a := range argv {
		if a != confineSep {
			continue
		}
		roots = argv[:i]
		if len(roots) == 0 || i+1 >= len(argv) {
			break
		}
		for _, r := range roots {
			if !filepath.IsAbs(r) {
				return nil, "", nil, fmt.Errorf("root %q is not absolute", r)
			}
		}
		return roots, argv[i+1], argv[i+2:], nil
	}
	return nil, "", nil, errors.New("usage: <roots...> -- <program> [args...]")
}

// confineArgs builds the helper's argv: the roots, the separator, then the program and its args.
func confineArgs(roots []string, path string, args []string) []string {
	out := append([]string{}, roots...)
	out = append(out, confineSep, path)
	return append(out, args...)
}

func withoutKey(environ []string, key string) []string {
	var out []string
	for _, kv := range environ {
		if k, _, _ := strings.Cut(kv, "="); k != key {
			out = append(out, kv)
		}
	}
	return out
}

// prepareConfinement refuses the Step unless the host can enforce the restriction, and returns
// the validated roots with the ABI that will be enforced.
func (r Runner) prepareConfinement(c ports.Confinement) (ports.Confinement, error) {
	if r.Self == "" {
		return ports.Confinement{}, fmt.Errorf("%w: the launch helper is not configured (Runner.Self)", ports.ErrConfinementUnavailable)
	}
	probe := r.Probe
	if probe == nil {
		probe = landlockABI
	}
	abi, err := probe()
	if err != nil {
		return ports.Confinement{}, fmt.Errorf("%w: %v", ports.ErrConfinementUnavailable, err)
	}
	if abi < MinLandlockABI {
		return ports.Confinement{}, fmt.Errorf("%w: Landlock ABI %d is below the required %d", ports.ErrConfinementUnavailable, abi, MinLandlockABI)
	}
	roots, err := resolveRoots(c.WritableRoots, r.Forbidden)
	if err != nil {
		return ports.Confinement{}, err
	}
	return ports.Confinement{WritableRoots: roots, ABI: abi}, nil
}

// resolveRoots validates the writable roots and returns them with symlinks resolved. A root must
// be an existing directory, must not be "/", the home directory, an ancestor of it or inside
// ~/.ssh, and must not overlap a forbidden path (the Project checkout).
func resolveRoots(roots, forbidden []string) ([]string, error) {
	if len(roots) == 0 {
		return nil, fmt.Errorf("%w: at least one writable root is required", ports.ErrInvalidConfinement)
	}
	var home, ssh string
	if h, err := os.UserHomeDir(); err == nil && h != "" {
		home = realPath(h)
		ssh = filepath.Join(home, ".ssh")
	}
	var deny []string
	for _, f := range forbidden {
		deny = append(deny, realPath(f))
	}
	var out []string
	seen := map[string]bool{}
	for _, raw := range roots {
		if !filepath.IsAbs(raw) {
			return nil, fmt.Errorf("%w: root %q is not absolute", ports.ErrInvalidConfinement, raw)
		}
		for _, part := range strings.Split(filepath.ToSlash(raw), "/") {
			if part == ".." {
				return nil, fmt.Errorf("%w: root %q contains '..'", ports.ErrInvalidConfinement, raw)
			}
		}
		root, err := filepath.EvalSymlinks(raw)
		if err != nil {
			return nil, fmt.Errorf("%w: root %q: %v", ports.ErrInvalidConfinement, raw, err)
		}
		if fi, err := os.Stat(root); err != nil || !fi.IsDir() {
			return nil, fmt.Errorf("%w: root %q is not a directory", ports.ErrInvalidConfinement, raw)
		}
		switch {
		case root == string(filepath.Separator) || filepath.Dir(root) == root:
			return nil, fmt.Errorf("%w: the filesystem root cannot be writable", ports.ErrInvalidConfinement)
		case home != "" && (within(root, home) || within(ssh, root)):
			return nil, fmt.Errorf("%w: root %q is the home directory, above it or inside ~/.ssh", ports.ErrInvalidConfinement, raw)
		}
		for _, d := range deny {
			if within(root, d) || within(d, root) {
				return nil, fmt.Errorf("%w: root %q overlaps %q", ports.ErrInvalidConfinement, raw, d)
			}
		}
		if !seen[root] {
			seen[root] = true
			out = append(out, root)
		}
	}
	return out, nil
}

// within reports whether path is dir or lies under it. Both are clean absolute paths.
func within(dir, path string) bool {
	if dir == path {
		return true
	}
	sep := string(filepath.Separator)
	return strings.HasPrefix(path, strings.TrimSuffix(dir, sep)+sep)
}

func realPath(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return filepath.Clean(p)
}
