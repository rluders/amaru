package ports

import (
	"context"

	"github.com/rluders/amaru/services/platform/internal/domain"
)

// SkillStore keeps Skills (FEAT-SKL-001). A Skill is immutable per scope, Project and name and
// version; it is never deleted.
type SkillStore interface {
	// Add stores s. Adding the same Skill again (same body) is a no-op; the same scope, Project,
	// name and version with a different body returns domain.ErrConflict, so a version never changes
	// under a run that pinned it. An invalid Skill returns domain.ErrInvalid.
	Add(ctx context.Context, s domain.Skill) error
	// Get returns the Skill a run in the Project means by name and version: the Project's own
	// when it has one, else the Organization's (ADR-0039), or domain.ErrNotFound. An empty
	// project means the Organization library only.
	Get(ctx context.Context, project domain.ProjectKey, name, version string) (domain.Skill, error)
}
