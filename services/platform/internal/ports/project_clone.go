package ports

import (
	"context"

	"github.com/rluders/amaru/services/platform/internal/domain"
)

// ProjectCloneStore atomically registers Projects and selects their local clone (ADR-0066).
// Root must be a canonical absolute native path, inspected by the trusted attach adapter.
// Stores check record shape; native path validation, filesystem identity and authorization
// belong to adapters and application composition.
type ProjectCloneStore interface {
	ProjectStore
	// RegisterClone preserves existing Project metadata and returns whether it created the Project.
	// Last committed registration wins for the same Project. A root owned by another Project
	// returns ErrExists; invalid inputs return ErrInvalid. Failures leave both records unchanged.
	RegisterClone(context.Context, domain.Project, domain.ProjectClone) (domain.Project, bool, error)
	// ActiveClone returns ErrNotFound for Projects without a binding; never guesses a root.
	ActiveClone(context.Context, domain.ProjectKey) (domain.ProjectClone, error)
}
