package coordinator

import (
	"context"
	"strings"

	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
)

// Workflow authorizes the Project before selecting its committed definition. Source is a
// trusted Instance dependency using the same attached Projects, never a request-supplied reader.
func (a *ProjectAccess) Workflow(ctx context.Context, project domain.ProjectKey, id, revision string, source ports.ProjectWorkflowSource) (domain.Workflow, error) {
	w, err := a.workflow(ctx, project, id, revision, source)
	return w.Workflow, err
}

func (a *ProjectAccess) workflow(ctx context.Context, project domain.ProjectKey, id, revision string, source ports.ProjectWorkflowSource) (ports.ProjectWorkflow, error) {
	if _, err := a.authorize(ctx, project); err != nil {
		return ports.ProjectWorkflow{}, err
	}
	if source == nil || !domain.ValidWorkflowID(id) || !domain.ValidDefinitionRevision(revision) {
		return ports.ProjectWorkflow{}, domain.ErrInvalid
	}
	w, err := source.Workflow(ctx, project, id, revision)
	if _, authErr := a.authorize(ctx, project); authErr != nil {
		return ports.ProjectWorkflow{}, authErr
	}
	if err != nil {
		return ports.ProjectWorkflow{}, err
	}
	if w.ID != id || w.Clone.ProjectKey != project || w.Clone.Root == "" || strings.ContainsRune(w.Clone.Root, 0) {
		return ports.ProjectWorkflow{}, domain.ErrInvalid
	}
	if err := w.Validate(); err != nil {
		return ports.ProjectWorkflow{}, domain.ErrInvalid
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
	w, err := a.workflow(ctx, project, req.WorkflowID, req.Revision, source)
	if err != nil {
		return Submitted{}, false, err
	}
	return a.Submit(ctx, project, SubmitRequest{Key: req.Key, Workflow: &w.Workflow, Revision: req.Revision, CloneRoot: w.Clone.Root, Variables: req.Variables})
}
