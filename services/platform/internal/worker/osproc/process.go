package osproc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/rluders/amaru/services/platform/internal/ports"
)

// DefaultGrace is how long a process tree gets to stop after the polite signal, before it is
// killed, when the context ends.
const DefaultGrace = 3 * time.Second

// envAllow lists exact variable names passed to children; envPrefix lists name prefixes.
// Everything else (cloud credentials, tokens of other tools) is dropped. No name here belongs to a
// vendor: what a Provider's program needs is declared in ProcSpec.Env by that Provider.
var (
	envAllow = map[string]bool{
		"PATH": true, "HOME": true, "USER": true, "LOGNAME": true, "SHELL": true, "TERM": true,
		"LANG": true, "TMPDIR": true, "TMP": true, "TEMP": true, "TZ": true,
		"USERPROFILE": true, "APPDATA": true, "LOCALAPPDATA": true, "SYSTEMROOT": true, "SYSTEMDRIVE": true,
		"PATHEXT": true, "COMSPEC": true, "PROGRAMDATA": true, "PROGRAMFILES": true,
		"HTTP_PROXY": true, "HTTPS_PROXY": true, "NO_PROXY": true, "SSL_CERT_FILE": true, "SSL_CERT_DIR": true,
		"DISPLAY": true, "WAYLAND_DISPLAY": true,
	}
	envPrefix = []string{"LC_", "XDG_"}
)

func allowedEnv(environ []string) []string {
	var out []string
	for _, kv := range environ {
		k, _, ok := strings.Cut(kv, "=")
		if ok && allowedKey(k) {
			out = append(out, kv)
		}
	}
	return out
}

func allowedKey(k string) bool {
	k = normEnvKey(k)
	if envAllow[k] {
		return true
	}
	for _, p := range envPrefix {
		if strings.HasPrefix(k, p) {
			return true
		}
	}
	return false
}

// Runner is the OS-backed ports.ProcessRunner: argv only (never a shell), an explicit working
// directory, an allow-listed environment, and termination of the whole process tree.
type Runner struct {
	// Environ returns the parent environment; nil means os.Environ. Tests inject it.
	Environ func() []string
	// Grace is the time between the polite signal and the kill when the context ends; zero means
	// DefaultGrace.
	Grace time.Duration
}

// Start starts the process. When ctx ends, the whole process tree is terminated.
func (r Runner) Start(ctx context.Context, spec ports.ProcSpec) (ports.Proc, error) {
	if spec.Path == "" || spec.Dir == "" {
		return nil, errors.New("process: Path and Dir are required")
	}
	environ := os.Environ
	if r.Environ != nil {
		environ = r.Environ
	}
	grace := r.Grace
	if grace <= 0 {
		grace = DefaultGrace
	}
	cmd := exec.Command(spec.Path, spec.Args...) // not CommandContext: the tree is ended by Terminate
	cmd.Dir = spec.Dir
	cmd.Env = append(allowedEnv(environ()), spec.Env...)
	var stdin io.WriteCloser
	if spec.Interactive {
		w, err := cmd.StdinPipe()
		if err != nil {
			return nil, err
		}
		stdin = w
	} else {
		cmd.Stdin = strings.NewReader(spec.Stdin)
	}
	configure(cmd)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start %s: %w", spec.Path, err)
	}
	p := &proc{cmd: cmd, stdout: stdout, stderr: stderr, stdin: stdin, done: make(chan struct{})}
	go func() { // the context ends the tree too
		select {
		case <-ctx.Done():
			p.Terminate(grace)
		case <-p.done:
		}
	}()
	if spec.Interactive {
		return interactive{p}, nil
	}
	return p, nil
}

type proc struct {
	cmd      *exec.Cmd
	stdout   io.Reader
	stderr   io.Reader
	stdin    io.WriteCloser // nil unless Interactive
	done     chan struct{}  // closed when Wait has returned
	waitOnce sync.Once
	code     int
	werr     error
	termOnce sync.Once
}

func (p *proc) Stdout() io.Reader { return p.stdout }
func (p *proc) Stderr() io.Reader { return p.stderr }

func (p *proc) Wait() (int, error) {
	p.waitOnce.Do(func() {
		err := p.cmd.Wait()
		close(p.done)
		var ee *exec.ExitError
		switch {
		case err == nil:
		case errors.As(err, &ee):
			p.code = ee.ExitCode()
		default:
			p.code, p.werr = -1, err
		}
	})
	return p.code, p.werr
}

// Terminate signals the process tree politely, then kills it after grace. The kill is sent even
// when the root process has already ended: a descendant that ignored the polite signal is in the
// same group and is still running, and the root ending says nothing about it.
func (p *proc) Terminate(grace time.Duration) {
	p.termOnce.Do(func() {
		interrupt(p.cmd)
		time.AfterFunc(grace, func() { kill(p.cmd) })
	})
}

// interactive adds the stdin pipe to a proc.
type interactive struct{ *proc }

func (i interactive) Stdin() io.WriteCloser { return i.proc.stdin }

var (
	_ ports.ProcessRunner   = Runner{}
	_ ports.Proc            = (*proc)(nil)
	_ ports.InteractiveProc = interactive{}
)
