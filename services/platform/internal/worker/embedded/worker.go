// Package embedded is the Worker that runs inside the Platform process in compact installs
// (FEAT-ENV-001, ADR-0013, ADR-0029). It talks to the coordinator only through a ports.WorkerLink,
// exactly as a remote Worker would; it imports no coordinator, store or domain-state package, so
// it cannot reach Platform state by another road (internal/arch enforces this).
//
// It pulls work: it offers its free slots, accepts what it is assigned, heartbeats the Lease while
// a Runner works, and reports the Result. What a Runner does is not decided here; Providers
// (FEAT-CAP-*) and the OS layer arrive with their own issues.
package embedded

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/rluders/amaru/services/platform/internal/ports"
)

// Outcome is what a Runner reports for one attempt.
type Outcome struct {
	Result      ports.ResultOutcome
	ErrorCode   string
	ErrorDetail string
}

// Runner does the work of one Assignment. It must return when ctx ends: the context is cancelled
// when the Platform revokes the Lease or the Worker stops. A Runner that panics fails its attempt.
type Runner interface {
	Run(ctx context.Context, a ports.Assignment) Outcome
}

// RunnerFunc adapts a function to a Runner.
type RunnerFunc func(ctx context.Context, a ports.Assignment) Outcome

func (f RunnerFunc) Run(ctx context.Context, a ports.Assignment) Outcome { return f(ctx, a) }

// Worker is the embedded Worker. The zero value of Slots means one slot and of HeartbeatPeriod ten
// seconds. Runner is required.
type Worker struct {
	Slots           int
	HeartbeatPeriod time.Duration
	Runner          Runner
}

// Run serves link until ctx ends or the link closes (both return nil). On return every running
// attempt has been cancelled and has finished; no Result is sent for a cancelled attempt, because
// the Platform decides what a stopped Worker's Steps become (the restart rule).
func (w *Worker) Run(ctx context.Context, link ports.WorkerLink) error {
	if w.Runner == nil {
		return errors.New("embedded worker: no Runner")
	}
	slots, beat := w.Slots, w.HeartbeatPeriod
	if slots <= 0 {
		slots = 1
	}
	if beat <= 0 {
		beat = 10 * time.Second
	}

	var (
		mu      sync.Mutex
		free    = slots
		running = map[string]context.CancelFunc{}
		wg      sync.WaitGroup
	)
	runCtx, stop := context.WithCancel(ctx)
	defer func() {
		stop()
		wg.Wait()
	}()

	// An Offer and an Accept are sent under mu together with the change to free they report, so the
	// order on the link is the order of the changes. The Platform relies on it to count slots
	// exactly: an Offer made before an Accept does not include that attempt, one after does.
	offer := func() error {
		mu.Lock()
		defer mu.Unlock()
		return link.Send(runCtx, ports.Offer{FreeSlots: uint32(free)})
	}
	if err := offer(); err != nil {
		return ignoreClosed(runCtx, err)
	}

	for {
		m, err := link.Receive(runCtx)
		if err != nil {
			return ignoreClosed(runCtx, err)
		}
		switch m := m.(type) {
		case ports.Assignment:
			actx, cancel := context.WithCancel(runCtx)
			mu.Lock()
			free--
			running[m.AttemptID] = cancel
			err := link.Send(runCtx, ports.AcceptAttempt{AttemptID: m.AttemptID, FencingToken: m.Lease.FencingToken})
			mu.Unlock()
			if err != nil {
				cancel()
				return ignoreClosed(runCtx, err)
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer cancel()
				revoked := w.execute(actx, link, m, beat)
				mu.Lock()
				delete(running, m.AttemptID)
				free++
				mu.Unlock()
				if !revoked {
					_ = offer() // reads free again: later completions are included, and the Platform counts exactly
				}
			}()
		case ports.LeaseRevoked:
			mu.Lock()
			cancel := running[m.AttemptID]
			mu.Unlock()
			if cancel != nil {
				cancel()
			}
		case ports.ResultAck:
			// The slot was freed when the attempt ended; the ack needs no action.
		}
	}
}

// execute runs one attempt: heartbeat while the Runner works, then report. It returns true when
// the attempt was cancelled and so reported nothing.
func (w *Worker) execute(ctx context.Context, link ports.WorkerLink, a ports.Assignment, beat time.Duration) (cancelled bool) {
	hbCtx, stopHeartbeat := context.WithCancel(ctx)
	hbDone := make(chan struct{})
	go func() {
		defer close(hbDone)
		t := time.NewTicker(beat)
		defer t.Stop()
		for {
			select {
			case <-hbCtx.Done():
				return
			case <-t.C:
				if err := link.Send(hbCtx, ports.LeaseRenew{AttemptID: a.AttemptID, FencingToken: a.Lease.FencingToken}); err != nil {
					return
				}
			}
		}
	}()

	out := w.runSafely(ctx, a)
	stopHeartbeat()
	<-hbDone
	if ctx.Err() != nil {
		return true
	}
	_ = link.Send(ctx, ports.Result{
		AttemptID: a.AttemptID, FencingToken: a.Lease.FencingToken,
		Outcome: out.Result, ErrorCode: out.ErrorCode, ErrorDetail: out.ErrorDetail,
	})
	return false
}

// timeoutCode is the error code of a Result for an attempt that ran past its Deadline. It equals
// domain.CodeTimeout; the Worker does not import the domain.
const timeoutCode = "timeout"

func (w *Worker) runSafely(ctx context.Context, a ports.Assignment) (out Outcome) {
	if !a.Deadline.IsZero() {
		// The Runner's context ends at the Deadline. It must stop what it started, the whole process
		// tree, before it returns; this Worker reports the timeout once it has.
		var cancel context.CancelFunc
		ctx, cancel = context.WithDeadline(ctx, a.Deadline)
		defer cancel()
		defer func() {
			// A success that arrives at the Deadline stands; anything else past it is a timeout.
			if ctx.Err() == context.DeadlineExceeded && out.Result != ports.OutcomeSucceeded {
				out = Outcome{Result: ports.OutcomeFailed, ErrorCode: timeoutCode, ErrorDetail: "the step ran past its deadline"}
			}
		}()
	}
	defer func() {
		if r := recover(); r != nil {
			out = Outcome{Result: ports.OutcomeFailed, ErrorCode: "runner_panic", ErrorDetail: fmt.Sprint(r)}
		}
	}()
	return w.Runner.Run(ctx, a)
}

func ignoreClosed(ctx context.Context, err error) error {
	if errors.Is(err, ports.ErrLinkClosed) || ctx.Err() != nil {
		return nil
	}
	return err
}
