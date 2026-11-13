package coordinator

import (
	"context"
	"errors"
	"sort"

	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
)

// InspectedProject combines authoritative metadata with one captured clone inspection.
// It is an application result, not a public wire contract or Worker placement instruction.
type InspectedProject struct {
	Project    domain.Project
	Inspection ports.ProjectInspection
}

// AttachProject authorizes before inspection and immediately before atomic registration.
// A committed registration is retained even if authority is lost before returning it.
func (a *ProjectAccess) AttachProject(ctx context.Context, root string, inspector ports.ProjectInspector) (InspectedProject, bool, error) {
	if _, err := a.administrator(ctx); err != nil {
		return InspectedProject{}, false, err
	}
	store, ok := a.projects.(ports.ProjectCloneStore)
	if !ok || inspector == nil {
		return InspectedProject{}, false, domain.ErrInvalid
	}
	inspected, inspectErr := inspector.Inspect(ctx, root)
	if _, err := a.administrator(ctx); err != nil {
		return InspectedProject{}, false, err
	}
	if inspectErr != nil {
		return InspectedProject{}, false, inspectErr
	}
	p := domain.Project{Key: inspected.Key, Name: inspected.Name, CreatedAt: a.coordinator.now()}
	if err := p.Validate(); err != nil {
		return InspectedProject{}, false, err
	}
	if _, err := a.administrator(ctx); err != nil {
		return InspectedProject{}, false, err
	}
	p, created, err := store.RegisterClone(ctx, p, domain.ProjectClone{ProjectKey: p.Key, Root: inspected.Root})
	if _, authErr := a.administrator(ctx); authErr != nil {
		return InspectedProject{}, false, authErr
	}
	if errors.Is(err, domain.ErrExists) {
		err = domain.ErrConflict
	}
	if err != nil {
		return InspectedProject{}, false, err
	}
	return InspectedProject{Project: p, Inspection: inspected}, created, nil
}

// Project captures exactly one binding; a changed or unavailable clone never falls back.
func (a *ProjectAccess) Project(ctx context.Context, key domain.ProjectKey, inspector ports.ProjectInspector) (InspectedProject, error) {
	if _, err := a.administrator(ctx); err != nil {
		return InspectedProject{}, err
	}
	if !key.Valid() {
		return InspectedProject{}, domain.ErrNotFound
	}
	p, err := a.projects.Get(ctx, key)
	if _, authErr := a.administrator(ctx); authErr != nil {
		return InspectedProject{}, authErr
	}
	if err != nil {
		return InspectedProject{}, err
	}
	if p.Key != key {
		return InspectedProject{}, domain.ErrNotFound
	}
	return a.inspectProject(ctx, p, inspector)
}

// Projects returns all attached Projects sorted by stored name, then key, or no results.
func (a *ProjectAccess) Projects(ctx context.Context, inspector ports.ProjectInspector) ([]InspectedProject, error) {
	if _, err := a.administrator(ctx); err != nil {
		return nil, err
	}
	projects, err := a.projects.List(ctx)
	if _, authErr := a.administrator(ctx); authErr != nil {
		return nil, authErr
	}
	if err != nil {
		return nil, err
	}
	result := make([]InspectedProject, 0, len(projects))
	for _, p := range projects {
		inspected, err := a.inspectProject(ctx, p, inspector)
		if err != nil {
			return nil, err
		}
		result = append(result, inspected)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Project.Name == result[j].Project.Name {
			return result[i].Project.Key < result[j].Project.Key
		}
		return result[i].Project.Name < result[j].Project.Name
	})
	if _, err := a.administrator(ctx); err != nil {
		return nil, err
	}
	return result, nil
}

func (a *ProjectAccess) inspectProject(ctx context.Context, p domain.Project, inspector ports.ProjectInspector) (InspectedProject, error) {
	if _, err := a.administrator(ctx); err != nil {
		return InspectedProject{}, err
	}
	store, ok := a.projects.(ports.ProjectCloneStore)
	if !ok || inspector == nil {
		return InspectedProject{}, domain.ErrInvalid
	}
	clone, err := store.ActiveClone(ctx, p.Key)
	if _, authErr := a.administrator(ctx); authErr != nil {
		return InspectedProject{}, authErr
	}
	if errors.Is(err, domain.ErrNotFound) {
		return InspectedProject{}, domain.ErrConflict
	}
	if err != nil {
		return InspectedProject{}, err
	}
	if clone.ProjectKey != p.Key || clone.Root == "" {
		return InspectedProject{}, domain.ErrConflict
	}
	inspected, inspectErr := inspector.Inspect(ctx, clone.Root)
	if _, err := a.administrator(ctx); err != nil {
		return InspectedProject{}, err
	}
	if errors.Is(inspectErr, domain.ErrInvalid) {
		return InspectedProject{}, domain.ErrConflict
	}
	if inspectErr != nil {
		return InspectedProject{}, inspectErr
	}
	if inspected.Key != p.Key || inspected.Root != clone.Root {
		return InspectedProject{}, domain.ErrConflict
	}
	return InspectedProject{Project: p, Inspection: inspected}, nil
}
