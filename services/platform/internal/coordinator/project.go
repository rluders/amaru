package coordinator

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
)

// ProjectAccess is the compact, single-Organization application boundary (ADR-0062).
// Authenticate must recheck the trusted transport identity on every call; request fields
// never establish authority. Internal scheduling continues to use Coordinator directly.
type ProjectAccess struct {
	coordinator  *Coordinator
	projects     ports.ProjectStore
	authenticate func(context.Context) (domain.Principal, error)
}

// NewProjectAccess binds public operations to the coordinator's authoritative store.
// Compact composition grants its authenticated local administrator all attached Projects;
// multi-Organization membership and non-owner access require a different authorization policy.
func NewProjectAccess(c *Coordinator, projects ports.ProjectStore, authenticate func(context.Context) (domain.Principal, error)) (*ProjectAccess, error) {
	if c == nil || projects == nil || authenticate == nil {
		return nil, fmt.Errorf("%w: Project access needs coordinator, Projects and authentication", domain.ErrInvalid)
	}
	return &ProjectAccess{c, projects, authenticate}, nil
}

func (a *ProjectAccess) authorize(ctx context.Context, project domain.ProjectKey) (domain.Principal, error) {
	by, err := a.authenticate(ctx)
	if err != nil {
		return domain.Principal{}, err
	}
	if by.Kind != domain.PrincipalHuman || by.ID == "" || by.Role != domain.RoleOwner || by.Sponsor != "" {
		return domain.Principal{}, domain.ErrNotPermitted
	}
	if !project.Valid() {
		return domain.Principal{}, domain.ErrNotFound
	}
	p, err := a.projects.Get(ctx, project)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.Principal{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Principal{}, err
	}
	if p.Key != project {
		return domain.Principal{}, domain.ErrNotFound
	}
	return by, nil
}

func (a *ProjectAccess) execution(ctx context.Context, project domain.ProjectKey, id string) (domain.Execution, []domain.StepExecution, error) {
	e, steps, err := a.coordinator.store.Get(ctx, id)
	if errors.Is(err, domain.ErrNotFound) || (err == nil && e.ProjectID != string(project)) {
		return domain.Execution{}, nil, domain.ErrNotFound
	}
	return e, steps, err
}

// Execution refuses missing and foreign IDs identically, before returning child records.
func (a *ProjectAccess) Execution(ctx context.Context, project domain.ProjectKey, id string) (domain.Execution, []domain.StepExecution, error) {
	if _, err := a.authorize(ctx, project); err != nil {
		return domain.Execution{}, nil, err
	}
	return a.execution(ctx, project, id)
}

// Submit derives attribution and Project identity from authorized context, not the body.
// Client-supplied Workflow/Steps remain trusted internal inputs until Project definition loading
// is composed; this method is not an HTTP create/run handler.
func (a *ProjectAccess) Submit(ctx context.Context, project domain.ProjectKey, req SubmitRequest) (Submitted, bool, error) {
	by, err := a.authorize(ctx, project)
	if err != nil {
		return Submitted{}, false, err
	}
	req.ProjectID, req.By = string(project), by
	req.Trigger = domain.TriggerManual
	return a.coordinator.Submit(ctx, req)
}

// Cancel checks ownership before the coordinator can read a recorded outcome or mutate state.
func (a *ProjectAccess) Cancel(ctx context.Context, project domain.ProjectKey, req CancelRequest) (Cancelled, bool, error) {
	by, err := a.authorize(ctx, project)
	if err != nil {
		return Cancelled{}, false, err
	}
	if _, _, err := a.execution(ctx, project, req.ExecutionID); err != nil {
		return Cancelled{}, false, err
	}
	req.By, req.ActorID = domain.Actor{Kind: domain.ActorHuman}, by.ID
	return a.coordinator.Cancel(ctx, req)
}

func (a *ProjectAccess) approval(ctx context.Context, project domain.ProjectKey, id string) (domain.Approval, error) {
	approval, err := a.coordinator.store.Approval(ctx, id)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.Approval{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Approval{}, err
	}
	if approval.ProjectID != string(project) {
		return domain.Approval{}, domain.ErrNotFound
	}
	if _, _, err := a.execution(ctx, project, approval.Gate.ExecutionID); err != nil {
		return domain.Approval{}, err
	}
	return approval, nil
}

func (a *ProjectAccess) Approval(ctx context.Context, project domain.ProjectKey, id string) (domain.Approval, error) {
	if _, err := a.authorize(ctx, project); err != nil {
		return domain.Approval{}, err
	}
	return a.approval(ctx, project, id)
}

func (a *ProjectAccess) DecideApproval(ctx context.Context, project domain.ProjectKey, req DecideRequest) (Decided, bool, error) {
	by, err := a.authorize(ctx, project)
	if err != nil {
		return Decided{}, false, err
	}
	if _, err := a.approval(ctx, project, req.ApprovalID); err != nil {
		return Decided{}, false, err
	}
	req.By = by
	return a.coordinator.DecideApproval(ctx, req)
}

// Definition returns only the selected Project's pinned Execution definition.
func (a *ProjectAccess) Definition(ctx context.Context, project domain.ProjectKey, id string) (domain.Workflow, error) {
	e, _, err := a.Execution(ctx, project, id)
	if err != nil {
		return domain.Workflow{}, err
	}
	if e.Definition == nil {
		return domain.Workflow{}, domain.ErrNotFound
	}
	return e.Definition.Workflow()
}

// Evidence checks ownership before reading contexts, artifacts, Events or Approval audit.
// The reader is a trusted composition dependency, never supplied by a client.
func (a *ProjectAccess) Evidence(ctx context.Context, project domain.ProjectKey, id string, reader ports.EvidenceReader) (domain.ExecutionEvidence, error) {
	if _, _, err := a.Execution(ctx, project, id); err != nil {
		return domain.ExecutionEvidence{}, err
	}
	if reader == nil {
		return domain.ExecutionEvidence{}, domain.ErrInvalid
	}
	return reader.Bundle(ctx, id)
}

// Events authorizes the Project and any Execution filter before exposing even a resync cursor.
// stream and window are trusted deployment dependencies; now comes from the coordinator.
func (a *ProjectAccess) Events(ctx context.Context, q domain.EventQuery, stream ports.EventStream, window time.Duration) (domain.EventPage, error) {
	if _, err := a.authorize(ctx, domain.ProjectKey(q.ProjectID)); err != nil {
		return domain.EventPage{}, err
	}
	if q.ExecutionID != "" {
		if _, _, err := a.execution(ctx, domain.ProjectKey(q.ProjectID), q.ExecutionID); err != nil {
			return domain.EventPage{}, err
		}
	}
	if stream == nil {
		return domain.EventPage{}, domain.ErrInvalid
	}
	return stream.ReadEvents(ctx, q, a.coordinator.now(), window)
}
