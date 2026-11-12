package coordinator

import (
	"context"
	"encoding/json"
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

// TrustContent records a human's trust once for the key, waking Steps waiting for it.
func (c *Coordinator) TrustContent(ctx context.Context, key string, by domain.Principal, project domain.ProjectKey, kind domain.TrustKind, ref, hash string) (domain.Trust, error) {
	if c.trust == nil {
		return domain.Trust{}, fmt.Errorf("%w: trust is not configured", domain.ErrInvalid)
	}
	if !validCommandKey(key) {
		return domain.Trust{}, fmt.Errorf("%w: trust needs an idempotency key", domain.ErrInvalid)
	}
	t, err := domain.NewTrust(c.now(), by, project, kind, ref, hash)
	if err != nil {
		return domain.Trust{}, err
	}
	fp, err := fingerprint(struct {
		By        domain.Principal
		Project   domain.ProjectKey
		Kind      domain.TrustKind
		Ref, Hash string
	}{by, project, kind, ref, hash})
	if err != nil {
		return domain.Trust{}, err
	}
	for range 2 {
		if out, found, err := replay[domain.Trust](ctx, c.trust, CmdTrustContent, key, fp); err != nil || found {
			return out, err
		}
		result, err := json.Marshal(t)
		if err != nil {
			return domain.Trust{}, err
		}
		err = c.trust.SaveOnce(ctx, domain.CommandRecord{Name: CmdTrustContent, Key: key, Fingerprint: fp, Result: result, At: c.now()}, t)
		if errors.Is(err, domain.ErrDuplicateCommand) {
			continue
		}
		if err != nil {
			return domain.Trust{}, err
		}
		c.Wake()
		return t, nil
	}
	return domain.Trust{}, fmt.Errorf("%w: trust key changed", domain.ErrConflict)
}

// RevokeTrust withdraws trust once for the key; it retries if trust changes between the read and the write.
func (c *Coordinator) RevokeTrust(ctx context.Context, key string, by domain.Principal, project domain.ProjectKey, kind domain.TrustKind, ref, reason string) (domain.Trust, error) {
	if c.trust == nil {
		return domain.Trust{}, fmt.Errorf("%w: trust is not configured", domain.ErrInvalid)
	}
	if !validCommandKey(key) {
		return domain.Trust{}, fmt.Errorf("%w: revoke trust needs an idempotency key", domain.ErrInvalid)
	}
	if by.Kind != domain.PrincipalHuman || by.ID == "" || !by.Role.MayTrustProjects() {
		return domain.Trust{}, domain.ErrNotPermitted
	}
	fp, err := fingerprint(struct {
		By          domain.Principal
		Project     domain.ProjectKey
		Kind        domain.TrustKind
		Ref, Reason string
	}{by, project, kind, ref, reason})
	if err != nil {
		return domain.Trust{}, err
	}
	for range maxEditAttempts {
		if out, found, err := replay[domain.Trust](ctx, c.trust, CmdRevokeTrust, key, fp); err != nil || found {
			return out, err
		}
		t, err := c.trust.Get(ctx, project, kind, ref)
		if err != nil {
			return domain.Trust{}, err
		}
		if out, found, err := replay[domain.Trust](ctx, c.trust, CmdRevokeTrust, key, fp); err != nil || found {
			return out, err
		}
		if err := t.Revoke(c.now(), by, reason); err != nil {
			return domain.Trust{}, err
		}
		result, err := json.Marshal(t)
		if err != nil {
			return domain.Trust{}, err
		}
		err = c.trust.SaveOnce(ctx, domain.CommandRecord{Name: CmdRevokeTrust, Key: key, Fingerprint: fp, Result: result, At: c.now()}, t)
		if errors.Is(err, domain.ErrDuplicateCommand) || errors.Is(err, domain.ErrConflict) {
			continue
		}
		if err != nil {
			return domain.Trust{}, err
		}
		return t, nil
	}
	return domain.Trust{}, fmt.Errorf("%w: trust kept changing", domain.ErrConflict)
}

const (
	CmdTrustContent = "trust.content"
	CmdRevokeTrust  = "trust.revoke"
)
