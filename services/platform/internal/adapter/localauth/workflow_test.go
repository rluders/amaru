package localauth

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/rluders/amaru/services/platform/internal/coordinator"
	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
	"github.com/rluders/amaru/services/platform/internal/ports/fakes"
)

type workflowSourceFunc func(context.Context, domain.ProjectKey, string, string) (domain.Workflow, error)

func (f workflowSourceFunc) Workflow(ctx context.Context, project domain.ProjectKey, id, revision string) (ports.ProjectWorkflow, error) {
	w, err := f(ctx, project, id, revision)
	return ports.ProjectWorkflow{Workflow: w, Clone: domain.ProjectClone{ProjectKey: project, Root: "test-clone"}}, err
}

func TestWorkflowWithholdsContentAfterRevocation(t *testing.T) {
	for _, lifecycle := range []string{"logout", "expiry", "cancellation", "shutdown"} {
		t.Run(lifecycle, func(t *testing.T) {
			a, _, _ := setup(t)
			now := time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
			a.now = func() time.Time { return now }
			projects := fakes.NewProjectStore()
			project := domain.ProjectKey("ssh://example.com/app#" + strings.Repeat("a", 40))
			if err := projects.Create(context.Background(), domain.Project{Key: project, Name: "App", CreatedAt: now}); err != nil {
				t.Fatal(err)
			}
			access, err := coordinator.NewProjectAccess(coordinator.New(fakes.NewExecutionStore(), time.Now), projects, a.Principal)
			if err != nil {
				t.Fatal(err)
			}
			var captured context.Context
			h := a.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { captured = r.Context(); w.WriteHeader(204) }))
			cookie, csrf := launch(t, a, h)
			if w := call(h, "GET", "/workflow-test", map[string]string{"Cookie": cookie.String()}); w.Code != 204 || captured == nil {
				t.Fatal("guarded dispatch failed")
			}
			ctx, cancel := context.WithCancel(captured)
			defer cancel()
			w := domain.Workflow{ID: "flow", Name: "Private content", Status: domain.WorkflowActive, Steps: []domain.WorkflowStep{{ID: "test", Name: "Test", Type: domain.StepTypeTool, Capability: "code.test"}}}
			var calls int
			source := workflowSourceFunc(func(context.Context, domain.ProjectKey, string, string) (domain.Workflow, error) {
				calls++
				switch lifecycle {
				case "logout":
					if got := call(h, "DELETE", "/v1/auth/session", map[string]string{"Cookie": cookie.String(), "Origin": "http://localhost:7777", "X-Amaru-CSRF": csrf}); got.Code != 204 {
						t.Fatal("logout failed")
					}
				case "expiry":
					now = now.Add(SessionLifetime)
				case "cancellation":
					cancel()
				case "shutdown":
					a.Close()
				}
				return w, nil
			})
			got, err := access.Workflow(ctx, project, "flow", strings.Repeat("a", 40), source)
			if err != domain.ErrNotPermitted || got.ID != "" || calls != 1 {
				t.Fatalf("revoked read: %+v %v calls=%d", got, err, calls)
			}
			if _, err := access.Workflow(ctx, project, "flow", strings.Repeat("a", 40), source); err != domain.ErrNotPermitted || calls != 1 {
				t.Fatalf("revoked lookup: %v calls=%d", err, calls)
			}
		})
	}
}
