package coordinator

import (
	"context"
	"errors"
	"fmt"

	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
)

// Skills (FEAT-SKL-001). A Step may pin Skills by name, version and hash (StepSpec.Skills). Before
// the Step can start the Platform resolves every reference: it must be well formed, name a Skill
// that exists, carry the Skill's own hash, and, for a Project Skill, be trusted at that exact
// content (a Trust of kind skill, FEAT-PRJ-003). Any failure ends the Step refused at Submit,
// with the code and message in its Reason, before a Worker sees it. The lease path resolves again,
// so a Skill trust revoked after Submit keeps a queued Step from starting; it waits with the
// reason, like a revoked Grant. Without WithSkills no Skill exists, so a Step that pins one is
// refused: the Coordinator fails closed.
//
// Not implemented: Skills named by a Workflow definition or an Agent (neither carries Skills yet),
// recording the resolved Skills as the Step's context (FEAT-SKL-002), reading Project Skills from
// the Project (a Skill reaches the store through ports.SkillStore.Add).

// WithSkills sets where Skills are read.
func WithSkills(s ports.SkillStore) Option { return func(c *Coordinator) { c.skills = s } }

// resolveSkills resolves every reference of a Step in the Project. It returns the Skills with
// their audit records, in the order of refs, or a *domain.SkillError for the first reference that may not be used, or any other
// error for a store failure.
func (c *Coordinator) resolveSkills(ctx context.Context, projectID string, refs []domain.SkillRef) ([]domain.ContextSkill, error) {
	if len(refs) > domain.MaxStepSkills {
		return nil, &domain.SkillError{Code: domain.ReasonSkillInvalid, Message: fmt.Sprintf("at most %d skills per step", domain.MaxStepSkills)}
	}
	resolved := make([]domain.ContextSkill, 0, len(refs))
	seen := map[string]bool{}
	for _, ref := range refs {
		if seen[ref.Name] {
			return nil, &domain.SkillError{Code: domain.ReasonSkillInvalid, Message: fmt.Sprintf("skill %s is listed twice", ref.Name)}
		}
		seen[ref.Name] = true
		var found *domain.Skill
		if ref.Validate() == nil && c.skills != nil {
			sk, err := c.skills.Get(ctx, domain.ProjectKey(projectID), ref.Name, ref.Version)
			switch {
			case err == nil:
				found = &sk
			case !errors.Is(err, domain.ErrNotFound):
				return nil, fmt.Errorf("load skill %s: %w", ref.Name, err)
			}
		}
		var trust *domain.Trust
		if found != nil && found.Scope == domain.SkillProject && c.trust != nil {
			t, err := c.trust.Get(ctx, found.ProjectKey, domain.TrustSkill, found.Name)
			switch {
			case err == nil:
				trust = &t
			case !errors.Is(err, domain.ErrNotFound):
				return nil, fmt.Errorf("load skill trust %s: %w", ref.Name, err)
			}
		}
		use, err := domain.ResolveSkill(ref, found, trust)
		if err != nil {
			return nil, err
		}
		resolved = append(resolved, domain.ContextSkill{Use: use, Body: found.Body})
	}
	return resolved, nil
}

// skillBlock returns why a Step's Skills may not be used now, or "" when they may. A store failure
// is returned as an error.
func (c *Coordinator) skillBlock(ctx context.Context, projectID string, refs []domain.SkillRef) (string, error) {
	if len(refs) == 0 {
		return "", nil
	}
	_, err := c.resolveSkills(ctx, projectID, refs)
	if se, ok := domain.IsSkillError(err); ok {
		return se.Error(), nil
	}
	return "", err
}
