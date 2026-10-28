package coordinator

import (
	"context"
	"errors"
	"fmt"

	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
)

// Policy is evaluated for every Step before it can start (FEAT-POL-001, ADR-0037, ADR-0055).
// Submit asks once per Step: an allowed Step is queued, any other Step ends refused, with the
// reason code and an actionable message in its Reason, before any Worker sees it. The lease
// path asks again, so a Grant revoked after Submit keeps a queued Step from starting; that Step
// waits with the reason and starts when a Grant covers it again.
//
// A require_approval result makes the Step wait for a person (see approval.go). Not implemented
// yet: rules come only from DefaultRules and WithRules, not from configuration scopes (ADR-0039);
// the evaluation itself is not written to the audit log (the Grant and Approval changes are,
// FEAT-POL-003). A refusal is the Step's own Reason and, at the lease path, a step.deferred event.

// WithGrants sets where Grants are read and written. Without it no Grant exists, so only the
// rules can allow a Step: the Coordinator fails closed.
func WithGrants(g ports.GrantStore) Option { return func(c *Coordinator) { c.grants = g } }

// WithRules adds rules to domain.DefaultRules, which always apply. A rule that fails
// domain.PolicyRule.Validate makes every evaluation return an error, so a bad rule stops work
// instead of being skipped.
func WithRules(rules ...domain.PolicyRule) Option {
	return func(c *Coordinator) { c.rules = append(c.rules, rules...) }
}

// policyRequest builds the request for one Step of an Execution.
func policyRequest(by domain.Principal, trigger, projectID string, pin *domain.DefinitionPin, capability string) domain.PolicyRequest {
	r := domain.PolicyRequest{
		Principal: by, Capability: domain.Capability(capability),
		Scope: domain.PolicyScope{ProjectID: projectID}, Trigger: trigger,
	}
	if pin != nil {
		r.Subject = domain.Subject{Kind: domain.SubjectWorkflow, ID: pin.WorkflowID}
		r.DefinitionHash = pin.Hash
	}
	return r
}

// decide evaluates one request against the stored Grants and the rules.
func (c *Coordinator) decide(ctx context.Context, req domain.PolicyRequest) (domain.PolicyResult, error) {
	var grants []domain.Grant
	if c.grants != nil {
		var err error
		if grants, err = c.grants.Active(ctx, req.Scope.ProjectID); err != nil {
			return domain.PolicyResult{}, fmt.Errorf("load grants: %w", err)
		}
	}
	standing, err := c.trustStanding(ctx, req)
	if err != nil {
		return domain.PolicyResult{}, err
	}
	req.ProjectTrust = standing
	return domain.Evaluate(c.now(), req, grants, c.rules)
}

// refusal is the Step Reason for a request that was not allowed.
func refusal(req domain.PolicyRequest, res domain.PolicyResult) string {
	return res.Explain(req)
}

// policyBlock returns why a queued Step may not start now, or "" when Policy allows it. It
// fails closed: an Execution that cannot be evaluated, for example one with no recorded
// requester, is blocked with the reason.
func (c *Coordinator) policyBlock(ctx context.Context, ref ports.StepRef) (string, error) {
	e, steps, err := c.store.Get(ctx, ref.ExecutionID)
	if err != nil {
		return "", err
	}
	for _, st := range steps {
		if st.ID != ref.StepID {
			continue
		}
		if reason, err := c.skillBlock(ctx, e.ProjectID, st.Skills); err != nil || reason != "" {
			return reason, err
		}
		req := policyRequest(e.RequestedBy, e.Trigger, e.ProjectID, e.Definition, st.Capability)
		res, err := c.decide(ctx, req)
		switch {
		case errors.Is(err, domain.ErrInvalid):
			return "policy: cannot be evaluated: " + err.Error(), nil
		case err != nil:
			return "", err
		case res.Decision == domain.DecisionRequireApproval:
			// An approved Approval for this Step covers the requirement: it authorizes this
			// occurrence. It does not lift a deny.
			approvals, err := c.store.Approvals(ctx, e.ID)
			if err != nil {
				return "", err
			}
			if a, ok := approvedFor(approvals, st.ID); ok {
				// The Approval is good for the Evidence it was requested for (FEAT-REV-003,
				// ADR-0048): a Step whose Evidence changed since, for example a context built
				// after the decision, is held with the reason like any other block. Nothing
				// re-requests an Approval, so it stays held until it is cancelled.
				cur, err := c.currentEvidence(ctx, e, st)
				if err != nil {
					return "", err
				}
				if cur.Digest != a.Evidence.Digest {
					return "approval: " + a.ID + " was given for other Evidence than the step has now", nil
				}
				return "", nil
			}
			return "policy: " + refusal(req, res), nil
		case res.Decision != domain.DecisionAllow:
			return "policy: " + refusal(req, res), nil
		}
		return "", nil
	}
	return "", fmt.Errorf("%w: step %s", domain.ErrNotFound, ref.StepID)
}

// CreateGrant stores a Grant made by a human Owner or Admin and wakes Serve, so a Step that waited
// for it can start. The Grant id is the caller's; the same id twice is domain.ErrExists. Making a
// Grant is not an idempotent command with a key: the id is the key.
func (c *Coordinator) CreateGrant(ctx context.Context, by domain.Principal, g domain.Grant) (domain.Grant, error) {
	if c.grants == nil {
		return domain.Grant{}, fmt.Errorf("%w: grants are not configured", domain.ErrInvalid)
	}
	g, err := domain.NewGrant(c.now(), by, g)
	if err != nil {
		return domain.Grant{}, err
	}
	out, err := c.grants.Create(ctx, g)
	if err != nil {
		return domain.Grant{}, err
	}
	c.Wake()
	return out, nil
}

// RevokeGrant revokes an active Grant, recording who and why. A queued Step that relied on it
// stops being leased on the next pass. Revoking a Grant that is not active is
// domain.ErrInvalidTransition.
func (c *Coordinator) RevokeGrant(ctx context.Context, by domain.Principal, id, reason string) (domain.Grant, error) {
	if c.grants == nil {
		return domain.Grant{}, fmt.Errorf("%w: grants are not configured", domain.ErrInvalid)
	}
	for range maxEditAttempts {
		g, err := c.grants.Get(ctx, id)
		if err != nil {
			return domain.Grant{}, err
		}
		if err := g.Revoke(c.now(), by, reason); err != nil {
			return domain.Grant{}, err
		}
		out, err := c.grants.Update(ctx, g)
		if errors.Is(err, domain.ErrConflict) {
			continue // it changed between the read and the write; read it again
		}
		return out, err
	}
	return domain.Grant{}, fmt.Errorf("%w: grant %s kept changing", domain.ErrConflict, id)
}
