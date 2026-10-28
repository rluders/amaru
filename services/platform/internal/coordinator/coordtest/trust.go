package coordtest

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/rluders/amaru/services/platform/internal/coordinator"
	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
)

// TrustSuite checks Project trust through the Coordinator and the Worker link (FEAT-PRJ-003):
// nothing in a Project starts until a human trusts it; the trust is bound to the content of the
// Workflow definition the Execution is pinned to, so a changed definition needs a new trust; and
// only a human Maintainer, Admin or Owner trusts or untrusts. newStores returns an empty
// Execution, Trust and Project store each call; they may share a database.
func TrustSuite(t *testing.T, newStores func(t *testing.T) (ports.ExecutionStore, ports.TrustStore, ports.ProjectStore), nl NewLink) {
	t.Helper()
	ctx := context.Background()
	project := domain.ProjectKey("ssh://github.com/acme/app#0123456789abcdef0123456789abcdef01234567")
	maintainer := domain.Principal{Kind: domain.PrincipalHuman, ID: "maint-1", Role: domain.RoleMaintainer}

	build := func(t *testing.T) (ports.ExecutionStore, ports.TrustStore, *coordinator.Coordinator) {
		t.Helper()
		store, trust, projects := newStores(t)
		if err := projects.Create(ctx, domain.Project{Key: project, Name: "app", CreatedAt: clock()}); err != nil {
			t.Fatal(err)
		}
		var attempts, execs atomic.Int64
		c := coordinator.New(store, clock, coordinator.WithTrust(trust), coordinator.WithLeasePeriod(leasePeriod),
			coordinator.WithAttemptIDs(func() string { return fmt.Sprintf("att-%d", attempts.Add(1)) }),
			coordinator.WithExecutionIDs(func() string { return fmt.Sprintf("e-%d", execs.Add(1)) }))
		return store, trust, c
	}
	// test submits one code.test Step, which the default rules allow.
	test := func(t *testing.T, c *coordinator.Coordinator, key string) coordinator.Submitted {
		t.Helper()
		got, _, err := c.Submit(ctx, coordinator.SubmitRequest{By: Requester, Key: key, ProjectID: string(project),
			Steps: []coordinator.StepSpec{{Name: "test", Capability: "code.test"}}})
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	flow := func(name string) domain.Workflow {
		return domain.Workflow{ID: "build", Name: name, Status: domain.WorkflowActive,
			Steps: []domain.WorkflowStep{{ID: "x", Name: "X", Type: domain.StepTypeTool, Capability: "code.test"}}}
	}
	submitFlow := func(t *testing.T, c *coordinator.Coordinator, key string, w domain.Workflow) domain.StepExecution {
		t.Helper()
		got, _, err := c.Submit(ctx, coordinator.SubmitRequest{By: Requester, Key: key, ProjectID: string(project), Workflow: &w})
		if err != nil {
			t.Fatal(err)
		}
		return got.Steps[0]
	}
	hashOf := func(t *testing.T, w domain.Workflow) string {
		t.Helper()
		pin, err := domain.Pin(w, "")
		if err != nil {
			t.Fatal(err)
		}
		return pin.Hash
	}

	t.Run("nothing in an untrusted Project starts: the Step is refused before any Worker sees it", func(t *testing.T) {
		store, _, c := build(t)
		got := test(t, c, "k1")
		st := got.Steps[0]
		if st.State != domain.StepRefused || !strings.Contains(st.Reason, domain.ReasonProjectUntrusted) || !strings.Contains(st.Reason, "maintainer") {
			t.Fatalf("state %s reason %q", st.State, st.Reason)
		}
		s := startSession(t, store, c, nl, "w1")
		s.send(ports.Offer{FreeSlots: 1})
		s.expectNothing()
	})

	t.Run("a Coordinator with no trust store trusts nothing", func(t *testing.T) {
		store, _, _ := newStores(t)
		c := coordinator.New(store, clock)
		got, _, err := c.Submit(ctx, coordinator.SubmitRequest{By: Requester, Key: "k1", ProjectID: string(project),
			Steps: []coordinator.StepSpec{{Name: "test", Capability: "code.test"}}})
		if err != nil || got.Steps[0].State != domain.StepRefused {
			t.Fatalf("%v %+v", err, got.Steps)
		}
		if _, err := c.TrustContent(ctx, maintainer, project, domain.TrustProject, "", ""); !errors.Is(err, domain.ErrInvalid) {
			t.Fatalf("trust without a store: %v", err)
		}
	})

	t.Run("only a human maintainer, admin or owner trusts, and a refused attempt changes nothing", func(t *testing.T) {
		_, trust, c := build(t)
		for _, by := range []domain.Principal{
			{Kind: domain.PrincipalHuman, ID: "c1", Role: domain.RoleContributor},
			{Kind: domain.PrincipalHuman, ID: "v1", Role: domain.RoleViewer},
			{Kind: domain.PrincipalAgent, ID: "a1", Sponsor: "maint-1", Role: domain.RoleOwner},
			{Kind: domain.PrincipalTrigger, ID: "t1", Sponsor: "maint-1", Role: domain.RoleOwner},
		} {
			if _, err := c.TrustContent(ctx, by, project, domain.TrustProject, "", ""); !errors.Is(err, domain.ErrNotPermitted) {
				t.Errorf("%s %s: %v", by.Kind, by.Role, err)
			}
		}
		if got, _ := trust.List(ctx, project); len(got) != 0 {
			t.Fatalf("a refused trust stored %+v", got)
		}
		if evs, _ := trust.Audit(ctx, 0, 0); len(evs) != 0 {
			t.Fatalf("a refused trust was audited: %+v", evs)
		}
	})

	t.Run("a trusted Project runs its Step, and untrusting it stops a queued Step until it is trusted again", func(t *testing.T) {
		store, trust, c := build(t)
		if _, err := c.TrustContent(ctx, maintainer, project, domain.TrustProject, "", ""); err != nil {
			t.Fatal(err)
		}
		got := test(t, c, "k1")
		if got.Steps[0].State != domain.StepQueued {
			t.Fatalf("state %s (%s)", got.Steps[0].State, got.Steps[0].Reason)
		}
		id := got.Execution.ID
		if _, err := c.RevokeTrust(ctx, maintainer, project, domain.TrustProject, "", ""); !errors.Is(err, domain.ErrReasonRequired) {
			t.Fatalf("revoke without a reason: %v", err)
		}
		if _, err := c.RevokeTrust(ctx, maintainer, project, domain.TrustProject, "", "repository changed hands"); err != nil {
			t.Fatal(err)
		}
		s := startSession(t, store, c, nl, "w1")
		s.send(ports.Offer{FreeSlots: 1})
		s.expectNothing()
		if _, st := s.step(id, got.Steps[0].ID); st.State != domain.StepQueued || !strings.Contains(st.Reason, domain.ReasonProjectUntrusted) {
			t.Fatalf("step %s reason %q, want queued and the trust reason", st.State, st.Reason)
		}
		if _, err := c.TrustContent(ctx, maintainer, project, domain.TrustProject, "", ""); err != nil {
			t.Fatal(err)
		}
		if a := s.assignment(); a.StepID != got.Steps[0].ID {
			t.Fatalf("assigned %+v", a)
		}
		evs, _ := trust.Audit(ctx, 0, 0)
		var kinds []string
		for _, e := range evs {
			kinds = append(kinds, e.Kind)
		}
		if strings.Join(kinds, ",") != "trust.granted,trust.revoked,trust.granted" || evs[1].Actor != "maint-1" ||
			evs[1].Reason != "repository changed hands" || evs[1].ProjectID != string(project) {
			t.Fatalf("audit %v %+v", kinds, evs)
		}
	})

	t.Run("trust is bound to the definition's content: a changed definition needs a new trust", func(t *testing.T) {
		_, _, c := build(t)
		if _, err := c.TrustContent(ctx, maintainer, project, domain.TrustProject, "", ""); err != nil {
			t.Fatal(err)
		}
		w := flow("Build")
		if st := submitFlow(t, c, "k1", w); st.State != domain.StepRefused || !strings.Contains(st.Reason, domain.ReasonTrustChanged) {
			t.Fatalf("definition never trusted: %s %q", st.State, st.Reason)
		}
		if _, err := c.TrustContent(ctx, maintainer, project, domain.TrustWorkflow, "build", hashOf(t, w)); err != nil {
			t.Fatal(err)
		}
		if st := submitFlow(t, c, "k2", w); st.State != domain.StepQueued {
			t.Fatalf("trusted definition: %s %q", st.State, st.Reason)
		}
		edited := flow("Build, edited")
		if st := submitFlow(t, c, "k3", edited); st.State != domain.StepRefused || !strings.Contains(st.Reason, domain.ReasonTrustChanged) {
			t.Fatalf("edited definition: %s %q", st.State, st.Reason)
		}
		if _, err := c.TrustContent(ctx, maintainer, project, domain.TrustWorkflow, "build", hashOf(t, edited)); err != nil {
			t.Fatal(err)
		}
		if st := submitFlow(t, c, "k4", edited); st.State != domain.StepQueued {
			t.Fatalf("re-trusted definition: %s %q", st.State, st.Reason)
		}
		// The new trust replaced the old one, so the first definition is no longer trusted.
		if st := submitFlow(t, c, "k5", w); st.State != domain.StepRefused {
			t.Fatalf("the old definition is still trusted: %s", st.State)
		}
	})
}
