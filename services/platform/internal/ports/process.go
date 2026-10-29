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

// ErrConfinementUnavailable is returned by a ProcessRunner that was asked to confine a process and
// cannot enforce it on this host (no mechanism, a kernel that is too old, or the helper is not
// configured). The process is not started: a Step never runs unconfined when confinement was
// requested (ADR-0057, FEAT-POL-006).
var ErrConfinementUnavailable = errors.New("write confinement cannot be enforced")

// ErrInvalidConfinement is returned when the requested writable roots are not acceptable.
var ErrInvalidConfinement = errors.New("invalid write confinement")

// Confinement restricts where a process and everything it starts may write (ADR-0057). Reads,
// the network and signals are not restricted by it.
type Confinement struct {
	// WritableRoots are the directories the process may write under: absolute, existing, no ".."
	// element. At least one is required. The Work Item's worktree is the one a Step always has.
	WritableRoots []string
	// ABI is set by the OS layer on the value a ConfinedProc reports: the enforcement level the
	// host gave. It is ignored on a request.
	ABI int
}

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
	// Confine, when non-nil, confines writes of the process and its descendants to the roots. A
	// nil Confine is an explicit "not confined", for a program the Worker runs itself; an agent
	// is started with a non-nil value or not at all (ADR-0057 item 5). If the runner cannot
	// enforce it, Start fails with ErrConfinementUnavailable.
	Confine *Confinement
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

// ConfinedProc is a Proc that was started with a Confinement. It reports what was enforced, so a
// Step can record a fact rather than a claim (ADR-0057 item 6).
type ConfinedProc interface {
	Proc
	Confinement() Confinement
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
