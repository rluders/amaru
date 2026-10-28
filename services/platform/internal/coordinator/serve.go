package coordinator

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
)

// Wake tells every serving session that Steps may have been queued, so a Worker with free slots
// is offered them. Whoever queues a Step calls it; it never blocks.
func (c *Coordinator) Wake() {
	c.mu.Lock()
	defer c.mu.Unlock()
	close(c.wakeCh)
	c.wakeCh = make(chan struct{})
}

func (c *Coordinator) wake() <-chan struct{} {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.wakeCh
}

// maxEditAttempts bounds how often an edit is re-read and re-applied after losing a write race.
const maxEditAttempts = 4

// Serve runs one Worker session over link until the context ends or the link closes (both return
// nil). It is the Platform's half of the pull protocol: a Worker's Offer gives it credit, Queued
// Steps are leased to it, one attempt per Lease, and each reply of the Worker is applied.
//
// Every state change is committed to the store before the message that announces it is sent, so
// a Worker never acts on a Lease the Platform could forget.
//
// workerID names the Worker in the Lease. The embedded Worker and a remote one differ only in the
// link: identity and authentication of remote Workers (ADR-0024) are not implemented.
func (c *Coordinator) Serve(ctx context.Context, link ports.PlatformLink, workerID string) error {
	msgs := make(chan ports.WorkerMessage)
	errc := make(chan error, 1)
	pumpCtx, stop := context.WithCancel(ctx)
	defer stop()
	go func() {
		for {
			m, err := link.Receive(pumpCtx)
			if err != nil {
				errc <- err
				return
			}
			select {
			case msgs <- m:
			case <-pumpCtx.Done():
				return
			}
		}
	}()

	// credit is how many more Steps this Worker can take. An Offer reports the Worker's free slots
	// as they were when it was sent; attempts sent to it that it has not yet accepted or refused
	// are not in that number, so they are taken off. pending holds those attempts.
	credit := 0
	pending := map[string]bool{}
	// cancelCh carries Cancel messages for this session's attempts from Cancel to this loop, the
	// only writer on the link. mine are the attempts registered for it.
	cancelCh := make(chan ports.Cancel, 64)
	mine := map[string]bool{}
	defer func() {
		for a := range mine {
			c.unregister(a)
		}
	}()
	for {
		// Hand out work first: a Step may have been queued while no message arrived.
		for credit > 0 {
			attempt, err := c.assignOne(ctx, link, workerID)
			if err != nil {
				return err
			}
			if attempt == "" {
				break
			}
			pending[attempt] = true
			mine[attempt] = true
			c.register(attempt, cancelCh)
			credit--
		}
		select {
		case <-ctx.Done():
			return nil
		case err := <-errc:
			if errors.Is(err, ports.ErrLinkClosed) || ctx.Err() != nil {
				return nil
			}
			return err
		case <-c.wake():
		case m := <-cancelCh:
			if err := send(ctx, link, m); err != nil {
				return err
			}
		case m := <-msgs:
			switch m := m.(type) {
			case ports.Offer:
				credit = max(int(m.FreeSlots)-len(pending), 0)
			case ports.AcceptAttempt:
				delete(pending, m.AttemptID)
				if err := c.onAccept(ctx, link, m); err != nil {
					return err
				}
			case ports.RejectAttempt:
				delete(pending, m.AttemptID)
				delete(mine, m.AttemptID)
				c.unregister(m.AttemptID)
				if err := c.onReject(ctx, m); err != nil {
					return err
				}
				credit = 0 // a Worker that refused must Offer again; it is not handed the same Step in a loop
			case ports.LeaseRenew:
				if err := c.onRenew(ctx, link, m); err != nil {
					return err
				}
			case ports.StepEvents:
				if err := c.onStepEvents(ctx, link, m); err != nil {
					return err
				}
			case ports.Result:
				if err := c.onResult(ctx, link, m); err != nil {
					return err
				}
				delete(mine, m.AttemptID)
				c.unregister(m.AttemptID)
			}
		}
	}
}

// assignOne leases the oldest queued Step to workerID and sends the Assignment. It returns the
// attempt id, or "" when no Step could be leased.
func (c *Coordinator) assignOne(ctx context.Context, link ports.PlatformLink, workerID string) (string, error) {
	asg, ok, err := c.leaseOne(ctx, workerID)
	if err != nil || !ok {
		return "", err
	}
	if err := link.Send(ctx, asg); err != nil {
		if errors.Is(err, ports.ErrLinkClosed) || ctx.Err() != nil {
			return "", nil
		}
		return "", err
	}
	return asg.AttemptID, nil
}

// leaseOne leases the oldest queued Step that no limit holds back. Counting the slots in use,
// checking the limits and committing the Lease happen under one lock, so two Worker sessions
// cannot both take the last slot. (Several Platform processes over one store would need the check
// in the store; one Platform is what exists.)
func (c *Coordinator) leaseOne(ctx context.Context, workerID string) (ports.Assignment, bool, error) {
	c.assignMu.Lock()
	defer c.assignMu.Unlock()
	refs, err := c.store.Queued(ctx, queueScan)
	if err != nil {
		return ports.Assignment{}, false, fmt.Errorf("list queued steps: %w", err)
	}
	if len(refs) == 0 {
		return ports.Assignment{}, false, nil
	}
	active, err := c.store.Active(ctx)
	if err != nil {
		return ports.Assignment{}, false, fmt.Errorf("list active steps: %w", err)
	}
	used := countUsage(active)
	for _, ref := range refs {
		if reason := c.limits.blocked(used, ref); reason != "" {
			if err := c.wait(ctx, ref, reason); err != nil {
				return ports.Assignment{}, false, err
			}
			continue
		}
		var asg ports.Assignment
		err := c.edit(ctx, ports.StepRef{ExecutionID: ref.ExecutionID, StepID: ref.StepID}, func(_ *domain.Execution, st *domain.StepExecution) (*change, error) {
			now := c.now()
			if err := st.Lease(workerID, c.newAttemptID(), now.Add(c.leasePeriod)); err != nil {
				return nil, err
			}
			asg = ports.Assignment{
				ExecutionID: ref.ExecutionID, StepID: st.ID, AttemptID: st.AttemptID,
				Capability: st.Capability, InputsJSON: st.InputsJSON,
				// The Step may run until Deadline; the Worker enforces it and kills what it started.
				Deadline: now.Add(c.timeouts.Effective(st.Agent != nil, st.TimeoutSeconds)),
				Lease:    ports.Lease{LeaseID: st.AttemptID, FencingToken: st.FenceToken, ExpiresAt: st.CurrentLease.ExpiresAt},
				// Stable across attempts, so a re-run side effect is deduplicated.
				IdempotencyKey:       ref.ExecutionID + "/" + st.ID,
				CapabilityIdempotent: st.Idempotent,
			}
			if st.RemoteJob != nil {
				// The Step ran before and left a remote job: the Worker re-attaches to it.
				asg.RemoteJob = &ports.RemoteJob{ID: st.RemoteJob.ID, Resumable: st.RemoteJob.Resumable}
			}
			return &change{events: []domain.ExecutionEvent{{Kind: domain.EventStepLeased, StepID: st.ID, Reason: workerID, At: now}}}, nil
		})
		if errors.Is(err, domain.ErrInvalidTransition) || errors.Is(err, domain.ErrConflict) || errors.Is(err, domain.ErrExists) {
			continue // it moved on
		}
		if err != nil {
			return ports.Assignment{}, false, err
		}
		return asg, true, nil
	}
	return ports.Assignment{}, false, nil
}

func (c *Coordinator) onAccept(ctx context.Context, link ports.PlatformLink, m ports.AcceptAttempt) error {
	ref, err := c.store.ByAttempt(ctx, m.AttemptID)
	if errors.Is(err, domain.ErrNotFound) {
		return revoke(ctx, link, m.AttemptID, "unknown_attempt")
	}
	if err != nil {
		return err
	}
	err = c.edit(ctx, ref, func(e *domain.Execution, st *domain.StepExecution) (*change, error) {
		if st.State != domain.StepLeased || !st.HoldsLease(m.AttemptID, m.FencingToken) {
			return nil, fmt.Errorf("%w: attempt %s", domain.ErrStaleLease, m.AttemptID)
		}
		now := c.now()
		if err := st.Start(now); err != nil {
			return nil, err
		}
		ch := &change{events: []domain.ExecutionEvent{{Kind: domain.EventStepStarted, StepID: st.ID, At: now}}}
		switch e.State {
		case domain.ExecQueued:
			ch.exec = true
			if err := e.Start(now); err != nil {
				return nil, err
			}
		case domain.ExecWaiting:
			ch.exec = true
			if err := e.Resume(); err != nil {
				return nil, err
			}
		}
		return ch, nil
	})
	if errors.Is(err, domain.ErrStaleLease) {
		return revoke(ctx, link, m.AttemptID, "stale_lease")
	}
	return err
}

func (c *Coordinator) onReject(ctx context.Context, m ports.RejectAttempt) error {
	ref, err := c.store.ByAttempt(ctx, m.AttemptID)
	if errors.Is(err, domain.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	err = c.edit(ctx, ref, func(_ *domain.Execution, st *domain.StepExecution) (*change, error) {
		if st.State != domain.StepLeased || st.AttemptID != m.AttemptID {
			return nil, fmt.Errorf("%w: attempt %s", domain.ErrStaleLease, m.AttemptID)
		}
		if err := st.ReleaseLease(domain.ReasonAttemptRejected); err != nil {
			return nil, err
		}
		return &change{events: []domain.ExecutionEvent{{Kind: domain.EventStepRejected, StepID: st.ID, Reason: m.Detail, At: c.now()}}}, nil
	})
	if errors.Is(err, domain.ErrStaleLease) {
		return nil // nothing to undo
	}
	if err == nil {
		c.Wake() // another Worker may take it
	}
	return err
}

func (c *Coordinator) onRenew(ctx context.Context, link ports.PlatformLink, m ports.LeaseRenew) error {
	ref, err := c.store.ByAttempt(ctx, m.AttemptID)
	if errors.Is(err, domain.ErrNotFound) {
		return revoke(ctx, link, m.AttemptID, "unknown_attempt")
	}
	if err != nil {
		return err
	}
	err = c.edit(ctx, ref, func(_ *domain.Execution, st *domain.StepExecution) (*change, error) {
		return &change{}, st.RenewLease(m.AttemptID, m.FencingToken, c.now().Add(c.leasePeriod))
	})
	if errors.Is(err, domain.ErrStaleLease) {
		return revoke(ctx, link, m.AttemptID, "stale_lease")
	}
	return err
}

func (c *Coordinator) onResult(ctx context.Context, link ports.PlatformLink, m ports.Result) error {
	ack := func(d ports.ResultDisposition) error {
		return send(ctx, link, ports.ResultAck{AttemptID: m.AttemptID, Disposition: d})
	}
	ref, err := c.store.ByAttempt(ctx, m.AttemptID)
	if errors.Is(err, domain.ErrNotFound) {
		return ack(ports.ResultRejectedStale)
	}
	if err != nil {
		return err
	}
	disp := ports.ResultAccepted
	err = c.edit(ctx, ref, func(_ *domain.Execution, st *domain.StepExecution) (*change, error) {
		now := c.now()
		switch {
		case st.AttemptID == m.AttemptID && st.FenceToken == m.FencingToken && st.State.Terminal():
			disp = ports.ResultDuplicate
			return nil, nil
		case st.State != domain.StepRunning || !st.HoldsLease(m.AttemptID, m.FencingToken):
			disp = ports.ResultRejectedStale
			return &change{events: []domain.ExecutionEvent{{Kind: domain.EventResultStale, StepID: st.ID, Reason: m.AttemptID, At: now}}}, nil
		}
		var kind string
		var ferr error
		switch m.Outcome {
		case ports.OutcomeSucceeded:
			kind, ferr = domain.EventStepSucceeded, st.Finish(now, domain.StepSucceeded, nil)
		case ports.OutcomeCancelled:
			kind, ferr = domain.EventStepCancelled, st.Finish(now, domain.StepCancelled, nil)
		case ports.OutcomeFailed:
			if m.ErrorCode == domain.CodeTimeout {
				// ADR-0044: a Step past its timeout is timed_out, never retried; a person decides.
				kind, ferr = domain.EventStepTimedOut, st.TimeOut(now)
				st.Error = &domain.ExecError{Code: domain.CodeTimeout, Message: m.ErrorDetail}
				break
			}
			code := m.ErrorCode
			if code == "" {
				code = "step_failed"
			}
			kind, ferr = domain.EventStepFailed, st.Finish(now, domain.StepFailed, &domain.ExecError{Code: code, Message: m.ErrorDetail})
		default:
			return nil, fmt.Errorf("%w: result outcome %d", domain.ErrInvalid, m.Outcome)
		}
		if ferr != nil {
			return nil, ferr
		}
		return &change{events: []domain.ExecutionEvent{{Kind: kind, StepID: st.ID, Reason: st.Reason, At: now}}}, nil
	})
	if err != nil {
		return err
	}
	if err := c.settleCancel(ctx, ref.ExecutionID); err != nil {
		return err
	}
	c.Wake() // a finished Step frees a slot a queued Step may be waiting for
	return ack(disp)
}

func revoke(ctx context.Context, link ports.PlatformLink, attemptID, reason string) error {
	return send(ctx, link, ports.LeaseRevoked{AttemptID: attemptID, Reason: reason})
}

// send treats a closed link as the end of the session, not as a failure of the coordinator.
func send(ctx context.Context, link ports.PlatformLink, m ports.PlatformMessage) error {
	if err := link.Send(ctx, m); err != nil && !errors.Is(err, ports.ErrLinkClosed) && ctx.Err() == nil {
		return err
	}
	return nil
}

// change is what an edit wants committed besides the Step: the Execution (when exec is set) and events.
type change struct {
	exec   bool
	events []domain.ExecutionEvent
}

// edit reads the Step's Execution, lets fn change the Step (and the Execution), and commits the
// result with the events. A nil change commits nothing. It reads and applies again when another
// writer got there first, up to maxEditAttempts times.
func (c *Coordinator) edit(ctx context.Context, ref ports.StepRef, fn func(*domain.Execution, *domain.StepExecution) (*change, error)) error {
	for range maxEditAttempts {
		e, steps, err := c.store.Get(ctx, ref.ExecutionID)
		if err != nil {
			return err
		}
		idx := -1
		for i := range steps {
			if steps[i].ID == ref.StepID {
				idx = i
			}
		}
		if idx < 0 {
			return fmt.Errorf("%w: step %s", domain.ErrNotFound, ref.StepID)
		}
		ch, err := fn(&e, &steps[idx])
		if err != nil {
			return err
		}
		if ch == nil {
			return nil
		}
		w := ports.ExecutionChange{Steps: []domain.StepExecution{steps[idx]}, Events: ch.events}
		if ch.exec {
			w.Execution = &e
		}
		if _, err := c.store.Commit(ctx, w); !errors.Is(err, domain.ErrConflict) {
			return err
		}
	}
	return fmt.Errorf("%w: step %s kept changing under the coordinator", domain.ErrConflict, ref.StepID)
}

// remoteJobData is the DataJSON of a StepEventRemoteJob event.
type remoteJobData struct {
	ID        string `json:"id"`
	Resumable bool   `json:"resumable"`
}

// onStepEvents applies the events of an attempt. Only the remote_job event is acted on: it records
// the Step's remote job, so that a restart can re-attach to it instead of running the work again
// (FEAT-CAP-004). The message is fenced like a Result: it must name the Step's current attempt and
// token, and the Step must be running. Anything else gets the Worker told it no longer holds the
// attempt. A malformed remote_job event is dropped, because a Worker's message must not stop the
// coordinator, and it is not recorded; other event types are dropped until the event stream exists.
func (c *Coordinator) onStepEvents(ctx context.Context, link ports.PlatformLink, m ports.StepEvents) error {
	ref, err := c.store.ByAttempt(ctx, m.AttemptID)
	if errors.Is(err, domain.ErrNotFound) {
		return revoke(ctx, link, m.AttemptID, "unknown_attempt")
	}
	if err != nil {
		return err
	}
	var jobs []remoteJobData
	for _, ev := range m.Events {
		if ev.Type != ports.StepEventRemoteJob {
			continue
		}
		var d remoteJobData
		dec := json.NewDecoder(bytes.NewReader(ev.DataJSON))
		dec.DisallowUnknownFields()
		if dec.Decode(&d) != nil {
			continue
		}
		jobs = append(jobs, d)
	}
	if len(jobs) == 0 {
		return nil
	}
	err = c.edit(ctx, ref, func(_ *domain.Execution, st *domain.StepExecution) (*change, error) {
		if st.State != domain.StepRunning || !st.HoldsLease(m.AttemptID, m.FencingToken) {
			return nil, fmt.Errorf("%w: attempt %s", domain.ErrStaleLease, m.AttemptID)
		}
		var evs []domain.ExecutionEvent
		for _, j := range jobs {
			changed, err := st.RecordRemoteJob(j.ID, j.Resumable)
			if errors.Is(err, domain.ErrInvalid) {
				continue // an id the domain refuses is dropped, not recorded
			}
			if err != nil {
				return nil, err
			}
			if changed {
				evs = append(evs, domain.ExecutionEvent{Kind: domain.EventStepRemoteJob, StepID: st.ID, At: c.now()})
			}
		}
		if len(evs) == 0 {
			return nil, nil
		}
		return &change{events: evs}, nil
	})
	if errors.Is(err, domain.ErrStaleLease) {
		return revoke(ctx, link, m.AttemptID, "stale_lease")
	}
	return err
}
