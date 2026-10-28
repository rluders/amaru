package coordinator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
)

// CmdCancel is the name of Cancel in command records.
const CmdCancel = "execution.cancel"

// CancelRequest cancels one Execution. Key is the caller's idempotency key and is required. By
// must be a human, or a Policy rule that names itself; anyone else is refused.
type CancelRequest struct {
	Key         string
	ExecutionID string
	Reason      string
	By          domain.Actor
}

// Cancelled is the outcome of Cancel, as recorded for the key.
type Cancelled struct {
	ExecutionID string `json:"executionId"`
	// Ended are the Steps that stopped at once: queued, interrupted or timed-out Steps.
	Ended []string `json:"ended,omitempty"`
	// Stopping are the Steps a Worker holds (leased or running); each Worker was told to end the
	// Step's process tree and the Step ends when the Worker reports. The Execution ends then.
	Stopping []string `json:"stopping,omitempty"`
	// ExecutionCancelled is true when the Execution reached cancelled in this command.
	ExecutionCancelled bool `json:"executionCancelled,omitempty"`
}

// systemActor is the actor for Steps the coordinator ends while it settles a requested cancel.
var systemActor = domain.Actor{Kind: domain.ActorPolicy, Authority: CmdCancel}

// Cancel cancels an Execution (FEAT-EXE-003). In one write it ends every Step nobody holds
// (queued, interrupted, timed out) and records that a cancel was requested. For each Step a
// Worker holds, it sends the Worker a Cancel message; the Worker ends the Step's whole process
// tree within its grace period and reports OutcomeCancelled, and the Execution becomes cancelled
// when its last Step has ended. An Execution with no held Step becomes cancelled at once.
//
// A repeat with the same key returns the recorded outcome and signals nothing; a key used for
// another request is refused (domain.ErrKeyReused). A cancel of an Execution that has ended is
// domain.ErrInvalidTransition. A held Step whose Worker has no session here (the Worker is gone)
// is not reached: it ends by the restart rule, and Reconcile then settles the cancel.
func (c *Coordinator) Cancel(ctx context.Context, req CancelRequest) (Cancelled, bool, error) {
	if req.Key == "" || req.ExecutionID == "" {
		return Cancelled{}, false, fmt.Errorf("%w: cancel needs an idempotency key and an execution id", domain.ErrInvalid)
	}
	if !req.By.MayCancel() {
		return Cancelled{}, false, fmt.Errorf("%w: %q may not cancel an execution", domain.ErrNotPermitted, req.By.Kind)
	}
	fp, err := fingerprint(req)
	if err != nil {
		return Cancelled{}, false, err
	}
	for range maxEditAttempts {
		if out, found, err := replay[Cancelled](ctx, c, CmdCancel, req.Key, fp); err != nil || found {
			return out, found, err
		}
		e, steps, err := c.store.Get(ctx, req.ExecutionID)
		if err != nil {
			return Cancelled{}, false, err
		}
		if e.State.Terminal() {
			return Cancelled{}, false, fmt.Errorf("%w: execution %s is %s", domain.ErrInvalidTransition, e.ID, e.State)
		}
		now := c.now()
		out := Cancelled{ExecutionID: e.ID}
		var held []domain.StepExecution
		var changed []domain.StepExecution
		for i := range steps {
			st := &steps[i]
			switch st.State {
			case domain.StepQueued:
				if err := st.Cancel(now); err != nil {
					return Cancelled{}, false, err
				}
			case domain.StepAwaitingApproval:
				if err := st.CancelAwaiting(now); err != nil {
					return Cancelled{}, false, err
				}
			case domain.StepInterrupted, domain.StepTimedOut:
				if err := st.Abandon(now, req.By); err != nil {
					return Cancelled{}, false, err
				}
			case domain.StepLeased, domain.StepRunning:
				held = append(held, *st)
				out.Stopping = append(out.Stopping, st.ID)
				continue
			default:
				continue
			}
			out.Ended = append(out.Ended, st.ID)
			changed = append(changed, *st)
		}
		events := []domain.ExecutionEvent{{Kind: domain.EventExecCancelRequested, Reason: req.Reason, At: now}}
		for _, id := range out.Ended {
			events = append(events, domain.ExecutionEvent{Kind: domain.EventStepCancelled, StepID: id, Reason: "execution_cancelled", At: now})
		}
		if len(held) == 0 && allEnded(steps) {
			if err := e.Finish(now, domain.ExecCancelled, nil); err != nil {
				return Cancelled{}, false, err
			}
			out.ExecutionCancelled = true
			events = append(events, domain.ExecutionEvent{Kind: domain.EventExecCancelled, At: now})
		}
		result, err := json.Marshal(out)
		if err != nil {
			return Cancelled{}, false, fmt.Errorf("encode outcome: %w", err)
		}
		voided, err := c.voidWaiting(ctx, e.ID, out.Ended, now)
		if err != nil {
			return Cancelled{}, false, err
		}
		ch := ports.ExecutionChange{Steps: changed, Events: events, Approvals: voided,
			Command: &domain.CommandRecord{Name: CmdCancel, Key: req.Key, Fingerprint: fp, Result: result, At: now}}
		if out.ExecutionCancelled || len(changed) == 0 {
			ch.Execution = &e // the change must name its Execution; unchanged, it only moves the version
		}
		_, err = c.store.Commit(ctx, ch)
		if errors.Is(err, domain.ErrConflict) || errors.Is(err, domain.ErrDuplicateCommand) {
			continue // the Execution or the key moved under us; read again
		}
		if err != nil {
			return Cancelled{}, false, err
		}
		for _, st := range held {
			c.signalCancel(st.AttemptID, req.Reason)
		}
		c.Wake()
		return out, false, nil
	}
	return Cancelled{}, false, fmt.Errorf("%w: execution %s kept changing under the cancel", domain.ErrConflict, req.ExecutionID)
}

// allEnded reports whether every Step has reached a final state, counting the ones in steps as
// they are now (callers have already moved the Steps they end).
func allEnded(steps []domain.StepExecution) bool {
	return !slices.ContainsFunc(steps, func(s domain.StepExecution) bool { return !s.State.Terminal() })
}

// settleCancel finishes a requested cancel after a Step ended or after a restart: it ends the
// Steps nobody holds and, when every Step has ended, moves the Execution to cancelled. It does
// nothing for an Execution that has no cancel request or has ended.
func (c *Coordinator) settleCancel(ctx context.Context, executionID string) error {
	for range maxEditAttempts {
		e, steps, err := c.store.Get(ctx, executionID)
		if err != nil {
			return err
		}
		if e.State.Terminal() {
			return nil
		}
		events, err := c.store.Events(ctx, executionID)
		if err != nil {
			return err
		}
		if !slices.ContainsFunc(events, func(ev domain.ExecutionEvent) bool { return ev.Kind == domain.EventExecCancelRequested }) {
			return nil
		}
		now := c.now()
		var changed []domain.StepExecution
		var endedIDs []string
		var evs []domain.ExecutionEvent
		for i := range steps {
			st := &steps[i]
			var err error
			switch st.State {
			case domain.StepQueued:
				err = st.Cancel(now)
			case domain.StepAwaitingApproval:
				err = st.CancelAwaiting(now)
			case domain.StepInterrupted, domain.StepTimedOut:
				err = st.Abandon(now, systemActor)
			default:
				continue
			}
			if err != nil {
				return err
			}
			changed = append(changed, *st)
			endedIDs = append(endedIDs, st.ID)
			evs = append(evs, domain.ExecutionEvent{Kind: domain.EventStepCancelled, StepID: st.ID, Reason: "execution_cancelled", At: now})
		}
		voided, err := c.voidWaiting(ctx, executionID, endedIDs, now)
		if err != nil {
			return err
		}
		ch := ports.ExecutionChange{Steps: changed, Events: evs, Approvals: voided}
		if allEnded(steps) {
			if err := e.Finish(now, domain.ExecCancelled, nil); err != nil {
				return err
			}
			ch.Execution = &e
			ch.Events = append(ch.Events, domain.ExecutionEvent{Kind: domain.EventExecCancelled, At: now})
		}
		if len(ch.Steps) == 0 && ch.Execution == nil {
			return nil // nothing to settle
		}
		if _, err := c.store.Commit(ctx, ch); !errors.Is(err, domain.ErrConflict) {
			return err
		}
	}
	return fmt.Errorf("%w: execution %s kept changing under the cancel", domain.ErrConflict, executionID)
}

func (c *Coordinator) register(attempt string, ch chan<- ports.Cancel) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cancels[attempt] = ch
}

func (c *Coordinator) unregister(attempt string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.cancels, attempt)
}

// signalCancel hands a Cancel message to the session that holds the attempt. It reports whether
// a session took it.
func (c *Coordinator) signalCancel(attempt, reason string) bool {
	c.mu.Lock()
	ch := c.cancels[attempt]
	c.mu.Unlock()
	if ch == nil {
		return false
	}
	select {
	case ch <- ports.Cancel{AttemptID: attempt, Reason: reason}:
		return true
	default:
		return false
	}
}
