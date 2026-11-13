package ports

import (
	"context"

	"github.com/rluders/amaru/services/platform/internal/domain"
)

// ProjectWorkflow carries the definition and the clone captured by the same source read.
// Clone is trusted compact-Instance provenance, never a remote Worker instruction.
type ProjectWorkflow struct {
	domain.Workflow
	Clone domain.ProjectClone
}

// ProjectWorkflowSource reads a validated Workflow from the selected attached Project at a
// full Git commit ID. Missing/foreign content returns ErrNotFound; invalid content ErrInvalid.
// Location selection belongs to trusted composition, never a client-supplied path. Readers
// must not execute repository content, apply filters, or fall back to another Project.
type ProjectWorkflowSource interface {
	Workflow(ctx context.Context, project domain.ProjectKey, id, revision string) (ProjectWorkflow, error)
}
