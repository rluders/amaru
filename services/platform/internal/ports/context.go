package ports

import (
	"context"

	"github.com/rluders/amaru/services/platform/internal/domain"
)

// ContextStore keeps the context assembled for each Step: the Evidence of what an Agent was told
// (FEAT-SKL-002). A record is written once and never changes.
type ContextStore interface {
	// Record stores c. Recording the same Step again with the same document is a no-op that
	// returns the stored record; another document returns domain.ErrConflict, so the Evidence of a
	// Step never changes. An invalid record returns domain.ErrInvalid.
	Record(ctx context.Context, c domain.StepContext) (domain.StepContext, error)
	// Get returns the context of the Step, or domain.ErrNotFound.
	Get(ctx context.Context, executionID, stepID string) (domain.StepContext, error)
}
