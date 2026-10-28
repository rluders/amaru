package ports

import (
	"context"
	"errors"
	"io"
	"time"
)

// This file is the process port of the Worker OS layer (ADR-0053, FEAT-EXE-003). A Provider that
// runs a local program starts it through a ProcessRunner and never imports os/exec itself; the OS
// layer (internal/worker/osproc) implements it. Only interfaces and plain data are here, so the
// core and the coordinator stay free of the OS.

// ErrExecutableNotFound is returned by an ExecutableResolver when no executable matches.
var ErrExecutableNotFound = errors.New("executable not found")

// ProcSpec describes one process to start. Path is the executable and Args its arguments: the
// process is started directly, never through a shell. Dir is the working directory.
type ProcSpec struct {
	Path string
	Args []string
	Dir  string
	// Env is added to the OS layer's allow-listed environment. It is where a Provider declares
	// what its program needs; nothing else of the parent environment reaches the child.
	Env []string
	// Stdin is written to the process and closed, unless Interactive is set.
	Stdin string
	// Interactive keeps stdin open for the caller to write (see InteractiveProc).
	Interactive bool
}

// Proc is a started process. Stdout and Stderr must be read until EOF before Wait returns the
// exit code.
type Proc interface {
	Stdout() io.Reader
	Stderr() io.Reader
	// Wait waits for the process to end and returns its exit code. A process that was ended by a
	// signal or by Terminate returns a non-zero code and a nil error.
	Wait() (exitCode int, err error)
	// Terminate asks the whole process tree to stop, then forces it to after grace. It returns
	// at once and is safe to call more than once; Wait returns when the root process has ended.
	Terminate(grace time.Duration)
}

// InteractiveProc is a Proc started with Interactive set.
type InteractiveProc interface {
	Proc
	// Stdin is the process's standard input; the caller closes it.
	Stdin() io.WriteCloser
}

// ProcessRunner starts processes. The context ends the process tree when it is cancelled.
type ProcessRunner interface {
	Start(ctx context.Context, spec ProcSpec) (Proc, error)
}

// ExecutableResolver finds an executable by name or path.
type ExecutableResolver interface {
	Find(name string) (string, error)
}
