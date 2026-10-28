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
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
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
		running = map[string]*attempt{}
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
			at := &attempt{cancel: cancel}
			mu.Lock()
			free--
			running[m.AttemptID] = at
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
				revoked := w.execute(actx, runCtx, link, m, beat, at)
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
			at := running[m.AttemptID]
			mu.Unlock()
			if at != nil {
				at.cancel()
			}
		case ports.Cancel:
			// Stop the attempt and, once the Runner has ended what it started, report it cancelled.
			mu.Lock()
			at := running[m.AttemptID]
			mu.Unlock()
			if at != nil {
				at.asked.Store(true)
				at.cancel()
			}
		case ports.ResultAck:
			// The slot was freed when the attempt ended; the ack needs no action.
		}
	}
}

// attempt is a running attempt: cancel ends its context; asked is set when the Platform sent a
// Cancel, as opposed to revoking the Lease.
type attempt struct {
	cancel context.CancelFunc
	asked  atomic.Bool
}

// execute runs one attempt: heartbeat while the Runner works, then report. It returns true when
// the attempt was revoked or the Worker stopped, and so reported nothing. An attempt the Platform
// asked to cancel reports OutcomeCancelled once the Runner has returned, whatever the Runner said:
// the Runner returns after it has ended the process tree it started (osproc does so when its
// context ends).
func (w *Worker) execute(ctx, linkCtx context.Context, link ports.WorkerLink, a ports.Assignment, beat time.Duration, at *attempt) (cancelled bool) {
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

	out := w.runSafely(ctx, link, a)
	stopHeartbeat()
	<-hbDone
	if ctx.Err() != nil {
		if !at.asked.Load() || linkCtx.Err() != nil {
			return true
		}
		_ = link.Send(linkCtx, ports.Result{AttemptID: a.AttemptID, FencingToken: a.Lease.FencingToken, Outcome: ports.OutcomeCancelled})
		return false
	}
	_ = link.Send(ctx, ports.Result{
		AttemptID: a.AttemptID, FencingToken: a.Lease.FencingToken,
		Outcome: out.Result, ErrorCode: out.ErrorCode, ErrorDetail: out.ErrorDetail,
	})
	return false
}

type reporterKey struct{}

// reporter sends the events of one attempt to the Platform.
type reporter struct {
	link ports.WorkerLink
	a    ports.Assignment
}

// ReportRemoteJob records, with the Platform, the remote job the attempt just started
// (FEAT-CAP-004). A Runner calls it with the context it was given, as soon as the remote service
// has accepted the work and before it waits for the result: if the Platform restarts while the
// job runs, a resumable job is re-attached through Assignment.RemoteJob instead of being
// submitted again, and a job that is not resumable ends the Step interrupted. The id is the
// service's own and must not be a secret or a signed URL. It may be called again for a
// multi-phase job.
func ReportRemoteJob(ctx context.Context, id string, resumable bool) error {
	r, ok := ctx.Value(reporterKey{}).(reporter)
	if !ok {
		return errors.New("embedded worker: ReportRemoteJob needs the context a Runner was given")
	}
	data, err := json.Marshal(struct {
		ID        string `json:"id"`
		Resumable bool   `json:"resumable"`
	}{id, resumable})
	if err != nil {
		return err
	}
	return r.link.Send(ctx, ports.StepEvents{AttemptID: r.a.AttemptID, FencingToken: r.a.Lease.FencingToken,
		Events: []ports.StepEvent{{Type: ports.StepEventRemoteJob, DataJSON: data}}})
}

// timeoutCode is the error code of a Result for an attempt that ran past its Deadline. It equals
// domain.CodeTimeout; the Worker does not import the domain.
const timeoutCode = "timeout"

func (w *Worker) runSafely(ctx context.Context, link ports.WorkerLink, a ports.Assignment) (out Outcome) {
	ctx = context.WithValue(ctx, reporterKey{}, reporter{link: link, a: a})
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
