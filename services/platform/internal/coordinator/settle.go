package coordinator

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
)

// failedStep chooses a stable failure independently of the store's Step order (ADR-0063).
func failedStep(steps []domain.StepExecution) *domain.StepExecution {
	var failed *domain.StepExecution
	for i := range steps {
		st := &steps[i]
		if (st.State == domain.StepFailed || st.State == domain.StepRefused) && (failed == nil || st.ID < failed.ID) {
			failed = st
		}
	}
	return failed
}

func (c *Coordinator) cancelRequested(ctx context.Context, id string) (bool, error) {
	evs, err := c.store.Events(ctx, id)
	return slices.ContainsFunc(evs, func(ev domain.ExecutionEvent) bool { return ev.Kind == domain.EventExecCancelRequested }), err
}

// stopping is checked again under the parent/Step versions when admitting work.
func (c *Coordinator) stopping(ctx context.Context, e domain.Execution, steps []domain.StepExecution) (bool, error) {
	if e.State.Terminal() || failedStep(steps) != nil {
		return true, nil
	}
	return c.cancelRequested(ctx, e.ID)
}

// settle repairs the Step-to-parent crash window and drains from durable state. Human waits
// survive ordinary failure; only an explicit cancel can abandon them (ADR-0063).
func (c *Coordinator) settle(ctx context.Context, id string) error {
	for range maxEditAttempts {
		e, steps, err := c.store.Get(ctx, id)
		if err != nil {
			return err
		}
		if e.State.Terminal() {
			return nil
		}
		cancel, err := c.cancelRequested(ctx, id)
		if err != nil {
			return err
		}
		failed := failedStep(steps)
		now := c.now()
		ch := ports.ExecutionChange{Execution: &e, CheckSteps: slices.Clone(steps)}
		reason := "execution_failed"
		if cancel {
			reason = "execution_cancelled"
		}
		var ended []string
		var held []domain.StepExecution
		waiting := false
		for i := range steps {
			st := &steps[i]
			if cancel || failed != nil {
				switch st.State {
				case domain.StepPending, domain.StepQueued:
					err = st.Cancel(now)
				case domain.StepAwaitingApproval:
					err = st.CancelAwaiting(now)
				case domain.StepInterrupted, domain.StepTimedOut:
					if !cancel {
						waiting = true
						continue
					}
					err = st.Abandon(now, systemActor)
				case domain.StepLeased, domain.StepRunning:
					held = append(held, *st)
					continue
				default:
					continue
				}
				if err != nil {
					return err
				}
				ch.Steps = append(ch.Steps, *st)
				ended = append(ended, st.ID)
				ch.Events = append(ch.Events, domain.ExecutionEvent{Kind: domain.EventStepCancelled, StepID: st.ID, Reason: reason, At: now})
			} else if st.State == domain.StepInterrupted || st.State == domain.StepTimedOut {
				waiting = true
			}
		}
		if len(ended) > 0 {
			ch.Approvals, err = c.voidWaiting(ctx, id, ended, now, reason)
			if err != nil {
				return err
			}
		}
		parentChanged := false
		if len(steps) > 0 && allEnded(steps) {
			state, kind := domain.ExecSucceeded, domain.EventExecCompleted
			var detail *domain.ExecError
			switch {
			case cancel:
				state, kind = domain.ExecCancelled, domain.EventExecCancelled
			case failed != nil:
				state, kind = domain.ExecFailed, domain.EventExecFailed
				detail = &domain.ExecError{Code: "step_failed", Message: "A Step failed."}
				if failed.State == domain.StepRefused {
					detail = &domain.ExecError{Code: "step_refused", Message: "A Step was refused."}
				} else if failed.Error != nil && failed.Error.Code != ErrCodeApprovalRejected {
					code, _ := c.redactor.Redact(failed.Error.Code)
					message, _ := c.redactor.Redact(failed.Error.Message)
					if code == "" {
						code = "step_failed"
					}
					if message == "" {
						message = "A Step failed."
					}
					detail = &domain.ExecError{Code: code, Message: message}
				} else if failed.Error != nil {
					detail = &domain.ExecError{Code: ErrCodeApprovalRejected, Message: "An Approval was rejected."}
				}
			case slices.ContainsFunc(steps, func(st domain.StepExecution) bool { return st.State == domain.StepCancelled }):
				state, kind = domain.ExecCancelled, domain.EventExecCancelled
			}
			if err := e.Finish(now, state, detail); err != nil {
				return err
			}
			ch.Events = append(ch.Events, domain.ExecutionEvent{Kind: kind, At: now})
			parentChanged = true
		} else if waiting && e.State == domain.ExecRunning {
			if err := e.Wait(); err != nil {
				return err
			}
			ch.Events = append(ch.Events, domain.ExecutionEvent{Kind: domain.EventExecWaiting, At: now})
			parentChanged = true
		}
		if parentChanged || len(ch.Steps) > 0 {
			_, err = c.store.Commit(ctx, ch)
			if errors.Is(err, domain.ErrConflict) {
				continue
			}
			if err != nil {
				return err
			}
		}
		if !cancel {
			for _, st := range held {
				c.signalCancel(st.AttemptID, reason)
			}
		}
		return nil
	}
	return fmt.Errorf("%w: execution %s kept changing under settlement", domain.ErrConflict, id)
}
