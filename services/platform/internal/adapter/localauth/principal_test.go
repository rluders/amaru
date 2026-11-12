package localauth

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/rluders/amaru/services/platform/internal/coordinator"
	"github.com/rluders/amaru/services/platform/internal/coordinator/coordtest"
	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports/fakes"
)

func TestPrincipalRequiresLiveAuthenticatedContext(t *testing.T) {
	a, _, _ := setup(t)
	other, _, _ := setup(t)
	var captured context.Context
	h := a.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { captured = r.Context(); w.WriteHeader(204) }))
	allowed := func(ctx context.Context) {
		t.Helper()
		by, err := a.Principal(ctx)
		if err != nil || by.ID != "local-administrator" || by.Kind != domain.PrincipalHuman || by.Role != domain.RoleOwner {
			t.Fatalf("missing identity: %+v %v", by, err)
		}
	}
	denied := func(ctx context.Context) {
		t.Helper()
		if by, err := a.Principal(ctx); err != domain.ErrNotPermitted || by != (domain.Principal{}) {
			t.Fatalf("unexpected identity: %+v %v", by, err)
		}
	}
	denied(context.Background())
	call(h, "GET", "/v1/health", nativeHeaders(a))
	denied(captured)
	call(h, "GET", "/command", map[string]string{"X-Actor-ID": "local-administrator", "X-Role": "owner"})
	denied(captured)
	call(h, "GET", "/command", nativeHeaders(a))
	native := captured
	allowed(native)
	if _, err := other.Principal(native); err != domain.ErrNotPermitted {
		t.Fatal("another Instance accepted identity")
	}
	cancelled, stop := context.WithCancel(native)
	stop()
	denied(cancelled)
	cookie, csrf := launch(t, a, h)
	call(h, "GET", "/command", map[string]string{"Cookie": cookie.String()})
	browser := captured
	allowed(browser)
	call(h, "DELETE", "/v1/auth/session", map[string]string{"Cookie": cookie.String(), "Origin": "http://localhost:7777", "X-Amaru-CSRF": csrf})
	denied(browser)
	allowed(native)
	cookie, _ = launch(t, a, h)
	call(h, "GET", "/command", map[string]string{"Cookie": cookie.String()})
	expiring := captured
	allowed(expiring)
	clock := a.now()
	a.now = func() time.Time { return clock.Add(SessionLifetime + time.Second) }
	denied(expiring)
	a.Close()
	denied(native)
}

// This composes the real guards and application boundary in a test handler, not a product route.
func TestAuthenticatedProjectAccessRechecksLogoutBeforeReplay(t *testing.T) {
	a, _, _ := setup(t)
	ctx := context.Background()
	key := domain.ProjectKey("ssh://github.com/acme/app#0123456789abcdef0123456789abcdef01234567")
	projects, executions := fakes.NewProjectStore(), fakes.NewExecutionStore()
	if err := projects.Create(ctx, domain.Project{Key: key, Name: "app", CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	c := coordinator.New(executions, time.Now, coordtest.Permissive)
	api, err := coordinator.NewProjectAccess(c, projects, a.Principal)
	if err != nil {
		t.Fatal(err)
	}
	var captured context.Context
	var submitted coordinator.Submitted
	h := a.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = r.Context()
		out, _, err := api.Submit(r.Context(), key, coordinator.SubmitRequest{Key: "submit", Steps: []coordinator.StepSpec{{Name: "test", Capability: "code.test"}}})
		if err != nil {
			t.Errorf("authenticated Submit: %v", err)
			w.WriteHeader(500)
			return
		}
		submitted = out
		w.WriteHeader(204)
	}))
	cookie, csrf := launch(t, a, h)
	if w := call(h, "POST", "/command", map[string]string{"Cookie": cookie.String(), "Origin": "http://localhost:7777", "X-Amaru-CSRF": csrf}); w.Code != 204 {
		t.Fatalf("submit status: %d", w.Code)
	}
	if submitted.Execution.RequestedBy.ID != "local-administrator" {
		t.Fatal("missing trusted attribution")
	}
	req := coordinator.CancelRequest{Key: "cancel", ExecutionID: submitted.Execution.ID}
	if _, _, err := api.Cancel(captured, key, req); err != nil {
		t.Fatal(err)
	}
	before, _, err := executions.Get(ctx, submitted.Execution.ID)
	if err != nil {
		t.Fatal(err)
	}
	call(h, "DELETE", "/v1/auth/session", map[string]string{"Cookie": cookie.String(), "Origin": "http://localhost:7777", "X-Amaru-CSRF": csrf})
	if out, replay, err := api.Cancel(captured, key, req); err != domain.ErrNotPermitted || replay || out.ExecutionID != "" {
		t.Fatalf("logged-out replay: %+v %v %v", out, replay, err)
	}
	after, _, err := executions.Get(ctx, submitted.Execution.ID)
	if err != nil || after.Version != before.Version || after.State != before.State {
		t.Fatal("logged-out replay mutated state")
	}
}
