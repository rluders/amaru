package ports

import (
	"context"

	"github.com/rluders/amaru/services/platform/internal/domain"
)

// GrantStore keeps Grants in the Platform store. A Grant is never deleted: revoking, lapsing and
// expiring change its status and keep the row.
type GrantStore interface {
	// Create stores a new Grant at version 1. It returns domain.ErrInvalid for an invalid Grant
	// and domain.ErrExists when the id is stored.
	Create(ctx context.Context, g domain.Grant) (domain.Grant, error)
	// Get returns the Grant, or domain.ErrNotFound.
	Get(ctx context.Context, id string) (domain.Grant, error)
	// Update stores a changed status and revocation record. g.Version is the version it was read
	// at: a stale one returns domain.ErrConflict and writes nothing, and a missing Grant returns
	// domain.ErrNotFound. Any other field differing from the stored Grant is domain.ErrInvalid:
	// what a human granted is never edited in place. It returns the stored Grant.
	Update(ctx context.Context, g domain.Grant) (domain.Grant, error)
	// Active returns every Grant whose stored status is active and that applies to the Project:
	// the Organization's and the Project's own, ordered by id. Expiry and definition binding are
	// decided by domain.Evaluate.
	Active(ctx context.Context, projectID string) ([]domain.Grant, error)
}
