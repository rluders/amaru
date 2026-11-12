package coordinator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
)

const (
	CmdResumeStep  = "step.resume"
	CmdAbandonStep = "step.abandon"
)

// HumanStepRequest resumes or abandons a human wait; transport composition supplies attribution.
// These internal application commands do not install public HTTP routes.
type HumanStepRequest struct {
	Key, ExecutionID, StepID string
	By                       domain.Actor
	ActorID                  string
}

func (c *Coordinator) ResumeStep(ctx context.Context, req HumanStepRequest) (domain.StepExecution, bool, error) {
	return c.humanStep(ctx, req, CmdResumeStep)
}

func (c *Coordinator) AbandonStep(ctx context.Context, req HumanStepRequest) (domain.StepExecution, bool, error) {
	return c.humanStep(ctx, req, CmdAbandonStep)
}

func (c *Coordinator) humanStep(ctx context.Context, req HumanStepRequest, name string) (domain.StepExecution, bool, error) {
	if !validCommandKey(req.Key) || req.ExecutionID == "" || req.StepID == "" {
		return domain.StepExecution{}, false, domain.ErrInvalid
	}
	if !req.By.MayCancel() || (req.By.Kind == domain.ActorHuman && req.ActorID == "") {
		return domain.StepExecution{}, false, domain.ErrNotPermitted
	}
	fp, err := fingerprint(req)
	if err != nil {
		return domain.StepExecution{}, false, err
	}
	for range maxEditAttempts {
		if out, found, err := replay[domain.StepExecution](ctx, c.store, name, req.Key, fp); err != nil || found {
			if err == nil && found {
				err = c.settle(ctx, req.ExecutionID)
			}
			return out, found, err
		}
		e, steps, err := c.store.Get(ctx, req.ExecutionID)
		if err != nil {
			return domain.StepExecution{}, false, err
		}
		if out, found, err := replay[domain.StepExecution](ctx, c.store, name, req.Key, fp); err != nil || found {
			if err == nil && found {
				err = c.settle(ctx, req.ExecutionID)
			}
			return out, found, err
		}
		i := indexOfStep(steps, req.StepID)
		if i < 0 {
			return domain.StepExecution{}, false, domain.ErrNotFound
		}
		if e.State.Terminal() {
			return domain.StepExecution{}, false, domain.ErrInvalidTransition
		}
		st := steps[i]
		now := c.now()
		kind := domain.EventStepCancelled
		if name == CmdResumeStep {
			stopping, err := c.stopping(ctx, e, steps)
			if err != nil {
				return domain.StepExecution{}, false, err
			}
			if stopping {
				return domain.StepExecution{}, false, fmt.Errorf("%w: execution is draining; abandon the Step or cancel the Execution", domain.ErrInvalidTransition)
			}
			err, kind = st.Resume(req.By), domain.EventStepResumed
		} else {
			err = st.Abandon(now, req.By)
		}
		if err != nil {
			return domain.StepExecution{}, false, err
		}
		out := st
		out.Version++
		result, err := json.Marshal(out)
		if err != nil {
			return domain.StepExecution{}, false, err
		}
		_, err = c.store.Commit(ctx, ports.ExecutionChange{Execution: &e, CheckSteps: steps, Steps: []domain.StepExecution{st},
			Events:  []domain.ExecutionEvent{{Kind: kind, StepID: st.ID, At: now}},
			Command: &domain.CommandRecord{Name: name, Key: req.Key, Fingerprint: fp, Result: result, At: now}})
		if errors.Is(err, domain.ErrConflict) || errors.Is(err, domain.ErrDuplicateCommand) {
			continue
		}
		if err != nil {
			return domain.StepExecution{}, false, err
		}
		if err := c.settle(ctx, e.ID); err != nil {
			return domain.StepExecution{}, false, err
		}
		c.Wake()
		return out, false, nil
	}
	return domain.StepExecution{}, false, domain.ErrConflict
}
