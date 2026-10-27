package ports

import (
	"context"

	"github.com/rluders/amaru/services/platform/internal/domain"
)

// ProjectStore keeps Project records in the Platform store.
type ProjectStore interface {
	// Create stores a new Project. It returns domain.ErrInvalid for an invalid Project and
	// domain.ErrExists when the key is already stored.
	Create(ctx context.Context, p domain.Project) error
	// Get returns the Project with the key, or domain.ErrNotFound.
	Get(ctx context.Context, key domain.ProjectKey) (domain.Project, error)
	// List returns every Project, ordered by key.
	List(ctx context.Context) ([]domain.Project, error)
}
