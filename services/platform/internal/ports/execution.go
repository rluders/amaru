package ports

import (
	"context"
	"fmt"

	"github.com/rluders/amaru/services/platform/internal/domain"
)

// ExecutionChange is one atomic write: every listed row is changed and every event appended, or
// nothing is. The Version of each Execution and Step is the version it was read at; the store
// refuses the whole change when any of them is stale.
type ExecutionChange struct {
	Execution *domain.Execution       // optional
	Steps     []domain.StepExecution  // optional
	Events    []domain.ExecutionEvent // appended in order; Seq and ExecutionID are set by the store
}

// ExecutionStore keeps Executions, their Steps and their event log in the Platform store
// (ADR-0042). A write returns only after it is durable, so a caller that acknowledges after a
// successful write never acknowledges a transition a crash can lose.
type ExecutionStore interface {
	// Create stores a new Execution with its Steps, all at version 1, in one write. It returns
	// domain.ErrExists when the Execution id is stored, domain.ErrInvalid when a Step does not
	// belong to the Execution.
	Create(ctx context.Context, e domain.Execution, steps []domain.StepExecution) (domain.Execution, []domain.StepExecution, error)
	// Commit applies the change and returns the stored rows with their new versions (the
	// Execution, if any, then the Steps in the order given) and the appended events with their Seq.
	// It returns domain.ErrConflict when any version is stale and domain.ErrNotFound when a row
	// does not exist; in both cases nothing was written.
	Commit(ctx context.Context, c ExecutionChange) (ExecutionChange, error)
	// Get returns the Execution and its Steps in creation order, or domain.ErrNotFound.
	Get(ctx context.Context, id string) (domain.Execution, []domain.StepExecution, error)
	// Open returns the id of every Execution that is not in a terminal state, ordered by id.
	Open(ctx context.Context) ([]string, error)
	// Events returns the Execution's log in Seq order.
	Events(ctx context.Context, id string) ([]domain.ExecutionEvent, error)
	ExecutionQueue
}

// ExecutionID is the one Execution the change belongs to: the Execution's, or its Steps'.
// It returns domain.ErrInvalid when the change is empty or spans Executions, because one write
// must keep one Execution's rows and log consistent.
func (c ExecutionChange) ExecutionID() (string, error) {
	id := ""
	if c.Execution != nil {
		id = c.Execution.ID
	}
	for _, st := range c.Steps {
		if id == "" {
			id = st.ExecutionID
		}
		if st.ExecutionID != id {
			return "", fmt.Errorf("%w: change spans executions %s and %s", domain.ErrInvalid, id, st.ExecutionID)
		}
	}
	if id == "" {
		return "", fmt.Errorf("%w: change names no execution", domain.ErrInvalid)
	}
	return id, nil
}

// StepRef names a Step and its Execution.
type StepRef struct{ ExecutionID, StepID string }

// ExecutionQueue is the part of the store the coordinator uses to hand out work and to find the
// Step a Worker message is about.
type ExecutionQueue interface {
	// Queued returns the Steps in state queued whose Execution is not finished, oldest Execution
	// first, then in Step order, at most limit of them.
	Queued(ctx context.Context, limit int) ([]StepRef, error)
	// ByAttempt returns the Step that was leased as attemptID, even after its Lease ended, or
	// domain.ErrNotFound.
	ByAttempt(ctx context.Context, attemptID string) (StepRef, error)
}
