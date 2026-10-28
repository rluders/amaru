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

// PolicySuite checks Policy through the coordinator and the Worker link (FEAT-POL-001): a Step
// that no rule or Grant covers is refused before any Worker sees it, with an actionable reason; a
// Grant lets it run; revoking the Grant stops a queued Step from starting; and Grants are made and
// revoked only by a human Owner or Admin and are attributed. newStores returns an empty
// Execution store and Grant store each call; they may share a database.
func PolicySuite(t *testing.T, newStores func(t *testing.T) (ports.ExecutionStore, ports.GrantStore), nl NewLink) {
	t.Helper()
	ctx := context.Background()
	owner := domain.Principal{Kind: domain.PrincipalHuman, ID: "owner-1", Role: domain.RoleOwner}
	admin := domain.Principal{Kind: domain.PrincipalHuman, ID: "admin-1", Role: domain.RoleAdmin}
	maintainer := domain.Principal{Kind: domain.PrincipalHuman, ID: "maint-1", Role: domain.RoleMaintainer}

	build := func(t *testing.T, opts ...coordinator.Option) (ports.ExecutionStore, ports.GrantStore, *coordinator.Coordinator) {
		t.Helper()
		store, grants := newStores(t)
		var attempts, execs atomic.Int64
		c := coordinator.New(store, clock, append([]coordinator.Option{
			coordinator.WithGrants(grants), coordinator.WithLeasePeriod(leasePeriod),
			coordinator.WithAttemptIDs(func() string { return fmt.Sprintf("att-%d", attempts.Add(1)) }),
			coordinator.WithExecutionIDs(func() string { return fmt.Sprintf("e-%d", execs.Add(1)) }),
		}, opts...)...)
		return store, grants, c
	}
	submit := func(t *testing.T, c *coordinator.Coordinator, key, project string, caps ...string) coordinator.Submitted {
		t.Helper()
		steps := make([]coordinator.StepSpec, len(caps))
		for i, cp := range caps {
			steps[i] = coordinator.StepSpec{Name: fmt.Sprintf("s%d", i+1), Capability: cp}
		}
		got, _, err := c.Submit(ctx, coordinator.SubmitRequest{By: Requester, Key: key, ProjectID: project, Steps: steps})
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	mcpGrant := func(id, project string) domain.Grant {
		g := domain.Grant{ID: id, Level: domain.GrantProject, ProjectID: project,
			Subject: domain.Subject{Kind: domain.SubjectWorkflow, ID: "tools"}, Capabilities: []domain.Capability{"mcp.call"}}
		if project == "" {
			g.Level = domain.GrantOrganization
		}
		return g
	}
	// toolsFlow runs mcp.call, which no default rule covers.
	toolsFlow := domain.Workflow{ID: "tools", Name: "Tools", Status: domain.WorkflowActive,
		Steps: []domain.WorkflowStep{{ID: "call", Name: "Call", Type: domain.StepTypeTool, Capability: "mcp.call"}}}
	submitFlow := func(t *testing.T, c *coordinator.Coordinator, key, project string, w domain.Workflow) coordinator.Submitted {
		t.Helper()
		got, _, err := c.Submit(ctx, coordinator.SubmitRequest{By: Requester, Key: key, ProjectID: project, Workflow: &w})
		if err != nil {
			t.Fatal(err)
		}
		return got
	}

	t.Run("a Step no rule or Grant covers is refused before any Worker sees it", func(t *testing.T) {
		store, _, c := build(t)
		got := submitFlow(t, c, "k1", "p1", toolsFlow)
		st := got.Steps[0]
		if st.State != domain.StepRefused {
			t.Fatalf("state %s, want refused", st.State)
		}
		for _, want := range []string{domain.ReasonUncovered, "mcp.call", "Owner or Admin", "workflow tools"} {
			if !strings.Contains(st.Reason, want) {
				t.Errorf("reason %q lacks %q", st.Reason, want)
			}
		}
		stored, _, err := store.Get(ctx, got.Execution.ID)
		if err != nil || stored.RequestedBy != Requester || stored.Trigger != domain.TriggerManual {
			t.Fatalf("requester not recorded: %+v %v", stored, err)
		}
		s := startSession(t, store, c, nl, "w1")
		s.send(ports.Offer{FreeSlots: 1})
		s.expectNothing()
		_, after := s.step(got.Execution.ID, st.ID)
		if after.State != domain.StepRefused || after.AttemptID != "" {
			t.Fatalf("a refused Step was touched: %+v", after)
		}
	})

	t.Run("a Grant lets the same Step run, and the reason is on the Step", func(t *testing.T) {
		store, _, c := build(t)
		g, err := c.CreateGrant(ctx, owner, mcpGrant("g1", "p1"))
		if err != nil {
			t.Fatal(err)
		}
		if g.GrantedBy != "owner-1" || g.Status != domain.GrantActive || g.Version != 1 {
			t.Fatalf("grant %+v", g)
		}
		got := submitFlow(t, c, "k1", "p1", toolsFlow)
		if got.Steps[0].State != domain.StepQueued {
			t.Fatalf("state %s (%s), want queued", got.Steps[0].State, got.Steps[0].Reason)
		}
		s := startSession(t, store, c, nl, "w1")
		s.send(ports.Offer{FreeSlots: 1})
		if a := s.assignment(); a.Capability != "mcp.call" {
			t.Fatalf("assigned %+v", a)
		}
	})

	t.Run("a Grant is scoped: another Project, another workflow and an unbound request are refused", func(t *testing.T) {
		_, _, c := build(t)
		if _, err := c.CreateGrant(ctx, admin, mcpGrant("g1", "p1")); err != nil {
			t.Fatal(err)
		}
		if st := submitFlow(t, c, "k1", "p2", toolsFlow).Steps[0]; st.State != domain.StepRefused {
			t.Errorf("another Project: %s", st.State)
		}
		other := toolsFlow
		other.ID = "other"
		if st := submitFlow(t, c, "k2", "p1", other).Steps[0]; st.State != domain.StepRefused {
			t.Errorf("another workflow: %s", st.State)
		}
		if st := submit(t, c, "k3", "p1", "mcp.call").Steps[0]; st.State != domain.StepRefused {
			t.Errorf("a direct submit names no workflow, so no Grant covers it: %s", st.State)
		}
	})

	t.Run("an Organization Grant covers every Project", func(t *testing.T) {
		_, _, c := build(t)
		if _, err := c.CreateGrant(ctx, owner, mcpGrant("g1", "")); err != nil {
			t.Fatal(err)
		}
		for i, p := range []string{"p1", "p2"} {
			if st := submitFlow(t, c, fmt.Sprintf("k%d", i), p, toolsFlow).Steps[0]; st.State != domain.StepQueued {
				t.Errorf("%s: %s", p, st.State)
			}
		}
	})

	t.Run("a Grant bound to a definition lapses when the definition changes", func(t *testing.T) {
		_, _, c := build(t)
		pin, err := domain.Pin(toolsFlow, "")
		if err != nil {
			t.Fatal(err)
		}
		g := mcpGrant("g1", "p1")
		g.DefinitionHash = pin.Hash
		if _, err := c.CreateGrant(ctx, owner, g); err != nil {
			t.Fatal(err)
		}
		if st := submitFlow(t, c, "k1", "p1", toolsFlow).Steps[0]; st.State != domain.StepQueued {
			t.Fatalf("unchanged definition: %s", st.State)
		}
		edited := toolsFlow
		edited.Name = "Tools, edited"
		if st := submitFlow(t, c, "k2", "p1", edited).Steps[0]; st.State != domain.StepRefused {
			t.Fatalf("edited definition: %s", st.State)
		}
	})

	t.Run("revoking a Grant stops a queued Step from starting, and a new Grant lets it start", func(t *testing.T) {
		store, _, c := build(t)
		if _, err := c.CreateGrant(ctx, owner, mcpGrant("g1", "p1")); err != nil {
			t.Fatal(err)
		}
		got := submitFlow(t, c, "k1", "p1", toolsFlow)
		id := got.Execution.ID
		g, err := c.RevokeGrant(ctx, admin, "g1", "contract ended")
		if err != nil {
			t.Fatal(err)
		}
		if g.Status != domain.GrantRevoked || g.RevokedBy != "admin-1" || g.RevokeReason != "contract ended" || g.GrantedBy != "owner-1" || g.Version != 2 {
			t.Fatalf("revoked grant %+v", g)
		}
		s := startSession(t, store, c, nl, "w1")
		s.send(ports.Offer{FreeSlots: 1})
		s.expectNothing()
		_, st := s.step(id, got.Steps[0].ID)
		if st.State != domain.StepQueued || !strings.HasPrefix(st.Reason, "policy: "+domain.ReasonUncovered) {
			t.Fatalf("step %s reason %q, want queued and a policy reason", st.State, st.Reason)
		}
		if n := deferred(t, store, id); n != 1 {
			t.Fatalf("%d step.deferred events, want 1", n)
		}
		if _, err := c.CreateGrant(ctx, owner, mcpGrant("g2", "p1")); err != nil {
			t.Fatal(err)
		}
		if a := s.assignment(); a.StepID != got.Steps[0].ID {
			t.Fatalf("assigned %+v", a)
		}
		if _, st := s.step(id, got.Steps[0].ID); st.State != domain.StepLeased || st.Reason != "" {
			t.Fatalf("after the new Grant: %s %q", st.State, st.Reason)
		}
	})

	t.Run("only a human Owner or Admin makes or revokes a Grant", func(t *testing.T) {
		_, grants, c := build(t)
		for name, by := range map[string]domain.Principal{
			"maintainer":  maintainer,
			"contributor": {Kind: domain.PrincipalHuman, ID: "u", Role: domain.RoleContributor},
			"agent":       {Kind: domain.PrincipalAgent, ID: "a", Sponsor: "owner-1", Role: domain.RoleOwner},
			"trigger":     {Kind: domain.PrincipalTrigger, ID: "t", Sponsor: "owner-1", Role: domain.RoleOwner},
		} {
			if _, err := c.CreateGrant(ctx, by, mcpGrant("g-"+name, "p1")); !errors.Is(err, domain.ErrNotPermitted) {
				t.Errorf("%s created a grant: %v", name, err)
			}
			if _, err := grants.Get(ctx, "g-"+name); !errors.Is(err, domain.ErrNotFound) {
				t.Errorf("%s: a refused grant was stored: %v", name, err)
			}
		}
		if _, err := c.CreateGrant(ctx, owner, mcpGrant("g1", "p1")); err != nil {
			t.Fatal(err)
		}
		if _, err := c.RevokeGrant(ctx, maintainer, "g1", "no"); !errors.Is(err, domain.ErrNotPermitted) {
			t.Fatalf("maintainer revoked: %v", err)
		}
		if _, err := c.RevokeGrant(ctx, admin, "g1", ""); !errors.Is(err, domain.ErrReasonRequired) {
			t.Fatalf("revoke without a reason: %v", err)
		}
		if g, _ := grants.Get(ctx, "g1"); g.Status != domain.GrantActive {
			t.Fatalf("a refused revoke changed the grant: %s", g.Status)
		}
		if _, err := c.RevokeGrant(ctx, admin, "g1", "done"); err != nil {
			t.Fatal(err)
		}
		if _, err := c.RevokeGrant(ctx, admin, "g1", "again"); !errors.Is(err, domain.ErrInvalidTransition) {
			t.Fatalf("second revoke: %v", err)
		}
		if _, err := c.RevokeGrant(ctx, admin, "nope", "x"); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("unknown grant: %v", err)
		}
		if _, err := c.CreateGrant(ctx, owner, mcpGrant("g1", "p1")); !errors.Is(err, domain.ErrExists) {
			t.Fatalf("same id twice: %v", err)
		}
	})

	t.Run("a coordinator with no Grant store fails closed", func(t *testing.T) {
		store, _ := newStores(t)
		c := coordinator.New(store, clock)
		if st := submitFlow(t, c, "k1", "p1", toolsFlow).Steps[0]; st.State != domain.StepRefused {
			t.Fatalf("got %s", st.State)
		}
		if _, err := c.CreateGrant(ctx, owner, mcpGrant("g1", "p1")); !errors.Is(err, domain.ErrInvalid) {
			t.Fatalf("grant without a store: %v", err)
		}
	})

	t.Run("the defaults allow local work, deny the reserved merge Capability, and no Grant lifts that", func(t *testing.T) {
		_, _, c := build(t)
		g := mcpGrant("g1", "")
		g.Capabilities = []domain.Capability{"scm.pr.merge", "mcp.call"}
		if _, err := c.CreateGrant(ctx, owner, g); err != nil {
			t.Fatal(err)
		}
		got := submit(t, c, "k1", "p1", "code.test", "scm.pr.merge", "git.push")
		want := []domain.StepState{domain.StepQueued, domain.StepRefused, domain.StepRefused}
		for i, st := range got.Steps {
			if st.State != want[i] {
				t.Errorf("%s: %s, want %s", st.Capability, st.State, want[i])
			}
		}
		if !strings.Contains(got.Steps[1].Reason, domain.ReasonExplicitDeny) {
			t.Errorf("merge reason %q", got.Steps[1].Reason)
		}
	})

	t.Run("a rule that requires an approval refuses the Step and says approvals do not exist yet", func(t *testing.T) {
		_, _, c := build(t, coordinator.WithRules(domain.PolicyRule{ID: "org.push", Source: domain.SourceOrganization,
			Effect: domain.EffectRequireApproval, Capabilities: []string{"git.push"}, Message: "pushing needs a human"}))
		st := submit(t, c, "k1", "p1", "git.push").Steps[0]
		if st.State != domain.StepRefused || !strings.Contains(st.Reason, "pushing needs a human") || !strings.Contains(st.Reason, "approvals are not implemented") {
			t.Fatalf("%s %q", st.State, st.Reason)
		}
	})

	t.Run("a submit with no requester, or an unsponsored trigger, is refused and creates nothing", func(t *testing.T) {
		store, _, c := build(t)
		steps := []coordinator.StepSpec{{Name: "a", Capability: "code.test"}}
		for name, req := range map[string]coordinator.SubmitRequest{
			"no requester":         {Key: "k1", ProjectID: "p1", Steps: steps},
			"trigger no sponsor":   {Key: "k2", ProjectID: "p1", Steps: steps, By: domain.Principal{Kind: domain.PrincipalTrigger, ID: "t"}, Trigger: domain.TriggerWebhook},
			"unknown trigger kind": {Key: "k3", ProjectID: "p1", Steps: steps, By: Requester, Trigger: "cron"},
		} {
			if _, _, err := c.Submit(ctx, req); !errors.Is(err, domain.ErrInvalid) {
				t.Errorf("%s: %v", name, err)
			}
		}
		if open, _ := store.Open(ctx); len(open) != 0 {
			t.Fatalf("%d Executions created", len(open))
		}
	})

	t.Run("every trigger goes through the same check", func(t *testing.T) {
		_, _, c := build(t)
		by := domain.Principal{Kind: domain.PrincipalTrigger, ID: "hook-1", Sponsor: "owner-1"}
		for i, trig := range []string{domain.TriggerWebhook, domain.TriggerSchedule, domain.TriggerDeliveryLoop, domain.TriggerManual} {
			got, _, err := c.Submit(ctx, coordinator.SubmitRequest{By: by, Trigger: trig, Key: fmt.Sprintf("k%d", i), ProjectID: "p1",
				Steps: []coordinator.StepSpec{{Name: "a", Capability: "mcp.call"}, {Name: "b", Capability: "code.test"}}})
			if err != nil {
				t.Fatal(err)
			}
			if got.Steps[0].State != domain.StepRefused || got.Steps[1].State != domain.StepQueued {
				t.Errorf("%s: %s, %s", trig, got.Steps[0].State, got.Steps[1].State)
			}
			if got.Execution.Trigger != trig || got.Execution.RequestedBy != by {
				t.Errorf("%s: recorded %q %+v", trig, got.Execution.Trigger, got.Execution.RequestedBy)
			}
		}
	})

	t.Run("a replayed Submit returns the stored outcome without asking Policy again", func(t *testing.T) {
		_, _, c := build(t)
		if _, err := c.CreateGrant(ctx, owner, mcpGrant("g1", "p1")); err != nil {
			t.Fatal(err)
		}
		first := submitFlow(t, c, "k1", "p1", toolsFlow)
		if _, err := c.RevokeGrant(ctx, owner, "g1", "r"); err != nil {
			t.Fatal(err)
		}
		again, replayed, err := c.Submit(ctx, coordinator.SubmitRequest{By: Requester, Key: "k1", ProjectID: "p1", Workflow: &toolsFlow})
		if err != nil || !replayed || again.Execution.ID != first.Execution.ID || again.Steps[0].State != domain.StepQueued {
			t.Fatalf("replay: %+v %v %v", again, replayed, err)
		}
	})

	t.Run("a different requester with the same key is a reused key", func(t *testing.T) {
		_, _, c := build(t)
		submit(t, c, "k1", "p1", "code.test")
		other := domain.Principal{Kind: domain.PrincipalHuman, ID: "someone-else", Role: domain.RoleContributor}
		_, _, err := c.Submit(ctx, coordinator.SubmitRequest{By: other, Key: "k1", ProjectID: "p1", Steps: []coordinator.StepSpec{{Name: "s1", Capability: "code.test"}}})
		if !errors.Is(err, domain.ErrKeyReused) {
			t.Fatalf("got %v", err)
		}
	})

	t.Run("an Execution with no recorded requester is not started", func(t *testing.T) {
		store, _, c := build(t)
		// Created straight in the store, as an Execution from before requesters were recorded.
		e := domain.Execution{ID: "old", State: domain.ExecQueued, CreatedAt: t0}
		st := domain.StepExecution{ID: "old.s", ExecutionID: "old", State: domain.StepPending, Capability: "code.test"}
		_, ss, err := store.Create(ctx, e, []domain.StepExecution{st})
		if err != nil {
			t.Fatal(err)
		}
		if err := ss[0].Plan(); err != nil {
			t.Fatal(err)
		}
		if _, err := store.Commit(ctx, ports.ExecutionChange{Steps: ss}); err != nil {
			t.Fatal(err)
		}
		s := startSession(t, store, c, nl, "w1")
		s.send(ports.Offer{FreeSlots: 1})
		s.expectNothing()
		if _, got := s.step("old", "old.s"); got.State != domain.StepQueued || !strings.HasPrefix(got.Reason, "policy: cannot be evaluated") {
			t.Fatalf("%s %q", got.State, got.Reason)
		}
	})

}
