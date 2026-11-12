package coordinator_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/rluders/amaru/services/platform/internal/coordinator"
	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports/fakes"
)

type workflowSource func(context.Context, domain.ProjectKey, string, string) (domain.Workflow, error)

func (f workflowSource) Workflow(ctx context.Context, project domain.ProjectKey, id, revision string) (domain.Workflow, error) {
	return f(ctx, project, id, revision)
}

func TestProjectWorkflowAuthorization(t *testing.T) {
	ctx := context.Background()
	project := domain.ProjectKey("ssh://example.com/app#" + strings.Repeat("a", 40))
	projects := fakes.NewProjectStore()
	if err := projects.Create(ctx, domain.Project{Key: project, Name: "App", CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	store := fakes.NewExecutionStore()
	c := coordinator.New(store, time.Now)
	revoked := false
	api, err := coordinator.NewProjectAccess(c, projects, func(context.Context) (domain.Principal, error) {
		if revoked {
			return domain.Principal{}, domain.ErrNotPermitted
		}
		return domain.Principal{Kind: domain.PrincipalHuman, ID: "owner", Role: domain.RoleOwner}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	req := coordinator.RunWorkflowRequest{Key: "run", WorkflowID: "same-flow", Revision: strings.Repeat("a", 64)}
	if _, _, err := api.RunWorkflow(ctx, domain.ProjectKey("ssh://example.com/other#"+strings.Repeat("a", 40)), req, nil); err != domain.ErrNotFound {
		t.Fatalf("unattached Project reached source: %v", err)
	}
	w := domain.Workflow{ID: "same-flow", Name: "Same", Status: domain.WorkflowActive, Steps: []domain.WorkflowStep{{ID: "test", Name: "Test", Type: domain.StepTypeTool, Capability: "code.test"}}}
	source := workflowSource(func(_ context.Context, p domain.ProjectKey, id, revision string) (domain.Workflow, error) {
		if p != project || id != req.WorkflowID || revision != req.Revision {
			t.Fatal("source selected wrong content")
		}
		return w, nil
	})
	for _, tc := range []struct {
		name   string
		change func()
	}{
		{"different id", func() { w.ID = "other-flow" }},
		{"invalid definition", func() { w.ID = req.WorkflowID; w.Name = "" }},
		{"draft", func() { w.Name = "Same"; w.Status = domain.WorkflowDraft }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.change()
			out, replay, err := api.RunWorkflow(ctx, project, req, source)
			if !errors.Is(err, domain.ErrInvalid) {
				t.Fatalf("bad source: %v", err)
			}
			if err == nil || replay || out.Execution.ID != "" {
				t.Fatalf("bad source created Execution: %+v %v %v", out, replay, err)
			}
		})
	}
	w.Status = domain.WorkflowActive
	source = workflowSource(func(context.Context, domain.ProjectKey, string, string) (domain.Workflow, error) {
		revoked = true
		return w, nil
	})
	if _, _, err := api.RunWorkflow(ctx, project, req, source); err != domain.ErrNotPermitted {
		t.Fatalf("session revoked during load reached Submit: %v", err)
	}
}
