package coordinator

import (
	"context"
	"fmt"

	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
)

// AuthorizeEgress is the consent callback for an egress.Sender bound to one Worker attempt.
// It uses the Execution's stored identity, scope, definition and trust, never caller-supplied
// authority. Each request rechecks Grants. A Step-wide Approval is not consent for a new URL.
// The normalized target is reconstructed to reject forged EgressTarget fields. A refusal or an
// unavailable decision log sends no request. This records authorization, not successful delivery.
func (c *Coordinator) AuthorizeEgress(ctx context.Context, attempt string, fence uint64, target domain.EgressTarget) error {
	parsed, err := domain.ParseEgressTarget(target.Key())
	if err != nil || parsed != target {
		return fmt.Errorf("%w: invalid egress target", domain.ErrInvalid)
	}
	ref, err := c.store.ByAttempt(ctx, attempt)
	if err != nil {
		return err
	}
	var allowed bool
	err = c.edit(ctx, ref, func(e *domain.Execution, st *domain.StepExecution) (*change, error) {
		if e.State != domain.ExecRunning || st.State != domain.StepRunning || !st.HoldsLease(attempt, fence) || !st.CurrentLease.ExpiresAt.After(c.now()) {
			return nil, domain.ErrStaleLease
		}
		req := target.PolicyRequest(policyRequest(e.RequestedBy, e.Trigger, e.ProjectID, e.Definition, ""))
		res, err := c.decide(ctx, req)
		if err != nil {
			return nil, err
		}
		allowed = res.Decision == domain.DecisionAllow
		// No raw URL, query, headers, inputs, or transport error belongs in the decision event.
		return &change{exec: true, events: []domain.ExecutionEvent{{Kind: domain.EventEgressDecision, StepID: st.ID, Reason: string(res.Decision), At: c.now()}}}, nil
	})
	if err != nil {
		return err
	}
	if !allowed {
		return domain.ErrNotPermitted
	}
	return nil
}

// EgressConsent binds a sender callback to the leased attempt; Providers receive the sender,
// never a Grant store or the authority to choose another principal.
func (c *Coordinator) EgressConsent(a ports.Assignment) func(context.Context, domain.EgressTarget) error {
	return func(ctx context.Context, t domain.EgressTarget) error {
		return c.AuthorizeEgress(ctx, a.AttemptID, a.Lease.FencingToken, t)
	}
}
