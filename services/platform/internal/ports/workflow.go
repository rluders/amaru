package ports

import (
	"context"

	"github.com/rluders/amaru/services/platform/internal/domain"
)

// ProjectWorkflowSource reads a validated Workflow from the selected attached Project at a
// full Git commit ID. Missing/foreign content returns ErrNotFound; invalid content ErrInvalid.
// Location selection belongs to trusted composition, never a client-supplied path. Readers
// must not execute repository content, apply filters, or fall back to another Project.
type ProjectWorkflowSource interface {
	Workflow(ctx context.Context, project domain.ProjectKey, id, revision string) (domain.Workflow, error)
}
