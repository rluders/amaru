package coordinator

import (
	"context"
	"errors"
	"fmt"

	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
)

// Project trust (FEAT-PRJ-003). A Step in a Project starts only when a human has trusted the
// Project and, when the Step runs a pinned Workflow, that exact definition. Policy enforces it
// first and nothing lifts it (domain.Evaluate). Without WithTrust no Project is trusted, so the
// Coordinator fails closed.
//
// The Platform compares the hash of the definition an Execution is pinned to. It does not read
// the Project's files, so the content it can see changed is a definition; a Skill record is
// stored but nothing yet reads one, and a Project-wide content scan is not implemented
// (WP-M3-07).

// WithTrust sets where Trust records are read and written.
func WithTrust(t ports.TrustStore) Option { return func(c *Coordinator) { c.trust = t } }

// trustStanding looks up the trust the request needs.
func (c *Coordinator) trustStanding(ctx context.Context, req domain.PolicyRequest) (domain.TrustStanding, error) {
	if c.trust == nil || req.Scope.ProjectID == "" {
		return domain.TrustUntrusted, nil
	}
	key := domain.ProjectKey(req.Scope.ProjectID)
	project, err := c.trust.Get(ctx, key, domain.TrustProject, "")
	if errors.Is(err, domain.ErrNotFound) {
		return domain.TrustUntrusted, nil
	}
	if err != nil {
		return "", fmt.Errorf("load project trust: %w", err)
	}
	var content *domain.Trust
	if req.Subject.Kind == domain.SubjectWorkflow && req.DefinitionHash != "" {
		t, err := c.trust.Get(ctx, key, domain.TrustWorkflow, req.Subject.ID)
		switch {
		case err == nil:
			content = &t
		case !errors.Is(err, domain.ErrNotFound):
			return "", fmt.Errorf("load workflow trust: %w", err)
		}
	}
	return domain.Standing(project, content, req.DefinitionHash), nil
}

// TrustContent records that a human trusts the Project (kind project, no ref, no hash) or one
// exact content of it. Trusting content again at a new hash replaces the old trust. It wakes
// Serve, so a Step that waited for the trust can start. Only a human Maintainer, Admin or Owner
// may; domain.ErrNotPermitted otherwise.
func (c *Coordinator) TrustContent(ctx context.Context, by domain.Principal, key domain.ProjectKey, kind domain.TrustKind, ref, hash string) (domain.Trust, error) {
	if c.trust == nil {
		return domain.Trust{}, fmt.Errorf("%w: trust is not configured", domain.ErrInvalid)
	}
	t, err := domain.NewTrust(c.now(), by, key, kind, ref, hash)
	if err != nil {
		return domain.Trust{}, err
	}
	if err := c.trust.Save(ctx, t); err != nil {
		return domain.Trust{}, err
	}
	c.Wake()
	return t, nil
}

// RevokeTrust withdraws a trust. A queued Step of the Project then waits at the lease path with
// the reason, as for a revoked Grant.
func (c *Coordinator) RevokeTrust(ctx context.Context, by domain.Principal, key domain.ProjectKey, kind domain.TrustKind, ref, reason string) (domain.Trust, error) {
	if c.trust == nil {
		return domain.Trust{}, fmt.Errorf("%w: trust is not configured", domain.ErrInvalid)
	}
	t, err := c.trust.Get(ctx, key, kind, ref)
	if err != nil {
		return domain.Trust{}, err
	}
	if err := t.Revoke(c.now(), by, reason); err != nil {
		return domain.Trust{}, err
	}
	if err := c.trust.Save(ctx, t); err != nil {
		return domain.Trust{}, err
	}
	return t, nil
}
