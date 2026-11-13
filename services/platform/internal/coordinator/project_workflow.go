package coordinator

import (
	"context"

	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
)

// Workflow authorizes the Project before selecting its committed definition. Source is a
// trusted Instance dependency using the same attached Projects, never a request-supplied reader.
func (a *ProjectAccess) Workflow(ctx context.Context, project domain.ProjectKey, id, revision string, source ports.ProjectWorkflowSource) (domain.Workflow, error) {
	if _, err := a.authorize(ctx, project); err != nil {
		return domain.Workflow{}, err
	}
	if source == nil || !domain.ValidWorkflowID(id) || !domain.ValidDefinitionRevision(revision) {
		return domain.Workflow{}, domain.ErrInvalid
	}
	w, err := source.Workflow(ctx, project, id, revision)
	if _, authErr := a.authorize(ctx, project); authErr != nil {
		return domain.Workflow{}, authErr
	}
	if err != nil {
		return domain.Workflow{}, err
	}
	if w.ID != id {
		return domain.Workflow{}, domain.ErrInvalid
	}
	if err := w.Validate(); err != nil {
		return domain.Workflow{}, domain.ErrInvalid
	}
	return w, nil
}

// RunWorkflowRequest names committed content; it carries no path, definition, Steps or actor.
// Revision is explicit so a retry cannot silently select a later HEAD with the same file name.
type RunWorkflowRequest struct {
	Key        string
	WorkflowID string
	Revision   string
	Variables  map[string]string
}

// RunWorkflow pins only the selected Project's definition and reuses Submit's Policy,
// attribution and atomic command outcome. A changed revision with an old key conflicts.
func (a *ProjectAccess) RunWorkflow(ctx context.Context, project domain.ProjectKey, req RunWorkflowRequest, source ports.ProjectWorkflowSource) (Submitted, bool, error) {
	w, err := a.Workflow(ctx, project, req.WorkflowID, req.Revision, source)
	if err != nil {
		return Submitted{}, false, err
	}
	return a.Submit(ctx, project, SubmitRequest{Key: req.Key, Workflow: &w, Revision: req.Revision, Variables: req.Variables})
}
