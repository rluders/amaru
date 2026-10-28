package coordtest

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync/atomic"
	"testing"

	"github.com/rluders/amaru/services/platform/internal/coordinator"
	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
)

// ApprovalSuite checks the durable Approval Step (FEAT-REV-001) through the coordinator and the
// Worker link: a Step Policy sends to a person waits, nothing runs until a person with the right
// role decides, the decision records who, when and the Evidence shown, a rejection ends the Step,
// and the whole state survives a restart. newStore returns an empty store each call; reopen
// returns a new store over the data the old one committed (nil means the same store).
func ApprovalSuite(t *testing.T, newStore func(t *testing.T) ports.ExecutionStore, nl NewLink, reopen Restart) {
	t.Helper()
	if reopen == nil {
		reopen = func(_ *testing.T, s ports.ExecutionStore) ports.ExecutionStore { return s }
	}
	ctx := context.Background()
	maintainer := domain.Principal{Kind: domain.PrincipalHuman, ID: "maint-1", Role: domain.RoleMaintainer}

	build := func(t *testing.T, store ports.ExecutionStore) *coordinator.Coordinator {
		t.Helper()
		var attempts, execs, apprs atomic.Int64
		return coordinator.New(store, clock, NewOpenTrust().Option(),
			coordinator.WithLeasePeriod(leasePeriod),
			coordinator.WithAttemptIDs(func() string { return fmt.Sprintf("att-%d", attempts.Add(1)) }),
			coordinator.WithExecutionIDs(func() string { return fmt.Sprintf("e-%d", execs.Add(1)) }),
			coordinator.WithApprovalIDs(func() string { return fmt.Sprintf("appr-%d", apprs.Add(1)) }))
	}
	// push submits one git.push Step, which the defaults send to a person.
	push := func(t *testing.T, c *coordinator.Coordinator, key string) coordinator.Submitted {
		t.Helper()
		got, _, err := c.Submit(ctx, coordinator.SubmitRequest{By: Requester, Key: key, ProjectID: "p1",
			Steps: []coordinator.StepSpec{{Name: "push", Capability: "git.push"}}})
		if err != nil {
			t.Fatal(err)
		}
		if len(got.Approvals) != 1 || got.Steps[0].State != domain.StepAwaitingApproval {
			t.Fatalf("step %s, %d approvals; want awaiting_approval and one", got.Steps[0].State, len(got.Approvals))
		}
		return got
	}
	decide := func(c *coordinator.Coordinator, key string, a domain.Approval, by domain.Principal, outcome domain.ApprovalOutcome, why string) (coordinator.Decided, bool, error) {
		return c.DecideApproval(ctx, coordinator.DecideRequest{Key: key, ApprovalID: a.ID, By: by, Outcome: outcome, Rationale: why, EvidenceDigest: a.Evidence.Digest})
	}
	stepOf := func(t *testing.T, s ports.ExecutionStore, exec, step string) domain.StepExecution {
		t.Helper()
		_, steps, err := s.Get(ctx, exec)
		if err != nil {
			t.Fatal(err)
		}
		for _, st := range steps {
			if st.ID == step {
				return st
			}
		}
		t.Fatalf("no step %s", step)
		return domain.StepExecution{}
	}
	audit := func(t *testing.T, s ports.ExecutionStore) []domain.AuditEvent {
		t.Helper()
		evs, err := s.Audit(ctx, 0, 0)
		if err != nil {
			t.Fatal(err)
		}
		return evs
	}
	kinds := func(evs []domain.AuditEvent) []string {
		var out []string
		for _, e := range evs {
			out = append(out, e.Kind)
		}
		return out
	}

	t.Run("a Step Policy sends to a person waits with a requested Approval and no Worker sees it", func(t *testing.T) {
		store := newStore(t)
		c := build(t, store)
		got := push(t, c, "k1")
		a, st := got.Approvals[0], got.Steps[0]
		if a.State != domain.ApprovalRequested || a.Gate.StepID != st.ID || a.Gate.ExecutionID != got.Execution.ID ||
			a.Gate.Capability != "git.push" || a.Gate.Kind != domain.GatePolicyRequired || a.Requester != Requester ||
			a.MinRole != domain.RoleMaintainer || a.ProjectID != "p1" || a.Version != 1 || a.Decision != nil {
			t.Fatalf("approval %+v", a)
		}
		var have []string
		for _, it := range a.Evidence.Items {
			have = append(have, it.Kind)
		}
		if !slices.Equal(have, []string{"execution", "step_execution", "step_inputs"}) || a.Evidence.Digest == "" {
			t.Fatalf("evidence %+v", a.Evidence)
		}
		s := startSession(t, store, c, nl, "w1")
		s.send(ports.Offer{FreeSlots: 1})
		s.expectNothing()
		if after := stepOf(t, store, got.Execution.ID, st.ID); after.State != domain.StepAwaitingApproval || after.AttemptID != "" {
			t.Fatalf("a waiting Step was touched: %+v", after)
		}
		if k := kinds(audit(t, store)); !slices.Equal(k, []string{domain.AuditApprovalRequested}) {
			t.Fatalf("audit %v", k)
		}
	})

	t.Run("approving records who, when and the Evidence shown, and lets the Step run", func(t *testing.T) {
		store := newStore(t)
		c := build(t, store)
		got := push(t, c, "k1")
		a := got.Approvals[0]
		out, replayed, err := decide(c, "d1", a, maintainer, domain.OutcomeApproved, "diff reviewed")
		if err != nil || replayed {
			t.Fatalf("decide: %v (replayed %v)", err, replayed)
		}
		d := out.Approval.Decision
		if out.Approval.State != domain.ApprovalApproved || d == nil || d.Approver != "maint-1" || d.ApproverRole != domain.RoleMaintainer ||
			d.Rationale != "diff reviewed" || d.EvidenceDigest != a.Evidence.Digest || !d.DecidedAt.Equal(clock()) {
			t.Fatalf("approval %+v decision %+v", out.Approval, d)
		}
		if out.Step.State != domain.StepQueued {
			t.Fatalf("step %s, want queued", out.Step.State)
		}
		s := startSession(t, store, c, nl, "w1")
		s.send(ports.Offer{FreeSlots: 1})
		if as := s.assignment(); as.StepID != got.Steps[0].ID {
			t.Fatalf("assigned %+v", as)
		}
		stored, err := store.Approval(ctx, a.ID)
		if err != nil || stored.State != domain.ApprovalApproved || stored.Version != 2 {
			t.Fatalf("stored %+v, %v", stored, err)
		}
		last := audit(t, store)[1]
		if last.Kind != domain.AuditApprovalApproved || last.Actor != "maint-1" || last.ActorKind != domain.ActorHuman || last.Reason != "diff reviewed" || last.TargetID != a.ID {
			t.Fatalf("audit %+v", last)
		}
		evs, _ := store.Events(ctx, got.Execution.ID)
		if !slices.ContainsFunc(evs, func(e domain.ExecutionEvent) bool {
			return e.Kind == domain.EventApprovalApproved && e.StepID == got.Steps[0].ID
		}) {
			t.Fatalf("no approval.approved event in %+v", evs)
		}
	})

	t.Run("rejecting needs a rationale, ends the Step failed and it never runs", func(t *testing.T) {
		store := newStore(t)
		c := build(t, store)
		got := push(t, c, "k1")
		a := got.Approvals[0]
		if _, _, err := decide(c, "d0", a, maintainer, domain.OutcomeRejected, ""); !errors.Is(err, domain.ErrReasonRequired) {
			t.Fatalf("reject without a rationale: %v", err)
		}
		if st := stepOf(t, store, got.Execution.ID, got.Steps[0].ID); st.State != domain.StepAwaitingApproval {
			t.Fatalf("a refused decision changed the step: %s", st.State)
		}
		out, _, err := decide(c, "d1", a, maintainer, domain.OutcomeRejected, "wrong branch")
		if err != nil {
			t.Fatal(err)
		}
		if out.Approval.State != domain.ApprovalRejected || out.Step.State != domain.StepFailed || out.Step.Error == nil ||
			out.Step.Error.Code != coordinator.ErrCodeApprovalRejected || out.Step.Error.Message != "wrong branch" {
			t.Fatalf("approval %s step %+v", out.Approval.State, out.Step)
		}
		s := startSession(t, store, c, nl, "w1")
		s.send(ports.Offer{FreeSlots: 1})
		s.expectNothing()
		if k := kinds(audit(t, store)); !slices.Equal(k, []string{domain.AuditApprovalRequested, domain.AuditApprovalRejected}) {
			t.Fatalf("audit %v", k)
		}
	})

	t.Run("only a human with the role may decide, and a refused decision changes nothing", func(t *testing.T) {
		store := newStore(t)
		c := build(t, store)
		got := push(t, c, "k1")
		a := got.Approvals[0]
		for name, by := range map[string]domain.Principal{
			"contributor": {Kind: domain.PrincipalHuman, ID: "u", Role: domain.RoleContributor},
			"viewer":      {Kind: domain.PrincipalHuman, ID: "u", Role: domain.RoleViewer},
			"no role":     {Kind: domain.PrincipalHuman, ID: "u"},
			"agent":       {Kind: domain.PrincipalAgent, ID: "a", Sponsor: "owner-1", Role: domain.RoleOwner},
			"trigger":     {Kind: domain.PrincipalTrigger, ID: "t", Sponsor: "owner-1", Role: domain.RoleOwner},
			"plugin":      {Kind: domain.PrincipalPlugin, ID: "p", Sponsor: "owner-1", Role: domain.RoleOwner},
		} {
			if _, _, err := decide(c, "d-"+name, a, by, domain.OutcomeApproved, "ok"); !errors.Is(err, domain.ErrNotPermitted) {
				t.Errorf("%s decided: %v", name, err)
			}
		}
		if stored, _ := store.Approval(ctx, a.ID); stored.State != domain.ApprovalRequested || stored.Version != 1 {
			t.Fatalf("a refused decision changed the approval: %+v", stored)
		}
		if k := kinds(audit(t, store)); len(k) != 1 {
			t.Fatalf("a refused decision was audited as a change: %v", k)
		}
		for _, role := range []domain.Role{domain.RoleOwner, domain.RoleAdmin} {
			g := push(t, c, "k-"+string(role))
			by := domain.Principal{Kind: domain.PrincipalHuman, ID: "u-" + string(role), Role: role}
			if _, _, err := decide(c, "dd-"+string(role), g.Approvals[0], by, domain.OutcomeApproved, ""); err != nil {
				t.Errorf("%s: %v", role, err)
			}
		}
	})

	t.Run("a decision on Evidence other than what the Approval holds is refused", func(t *testing.T) {
		store := newStore(t)
		c := build(t, store)
		a := push(t, c, "k1").Approvals[0]
		for _, digest := range []string{"", "sha256:" + "0000", a.Evidence.Digest + "x"} {
			_, _, err := c.DecideApproval(ctx, coordinator.DecideRequest{Key: "d" + digest, ApprovalID: a.ID, By: maintainer, Outcome: domain.OutcomeApproved, EvidenceDigest: digest})
			if !errors.Is(err, domain.ErrEvidenceChanged) {
				t.Errorf("digest %q: %v", digest, err)
			}
		}
		if stored, _ := store.Approval(ctx, a.ID); stored.State != domain.ApprovalRequested {
			t.Fatalf("state %s", stored.State)
		}
	})

	t.Run("an Approval is decided once, and a repeated key returns the recorded outcome", func(t *testing.T) {
		store := newStore(t)
		c := build(t, store)
		a := push(t, c, "k1").Approvals[0]
		first, _, err := decide(c, "d1", a, maintainer, domain.OutcomeApproved, "ok")
		if err != nil {
			t.Fatal(err)
		}
		again, replayed, err := decide(c, "d1", a, maintainer, domain.OutcomeApproved, "ok")
		if err != nil || !replayed || again.Approval.Version != first.Approval.Version || again.Approval.State != domain.ApprovalApproved {
			t.Fatalf("replay: %+v replayed=%v err=%v", again, replayed, err)
		}
		if _, _, err := decide(c, "d1", a, maintainer, domain.OutcomeRejected, "no"); !errors.Is(err, domain.ErrKeyReused) {
			t.Fatalf("same key, different decision: %v", err)
		}
		if _, _, err := decide(c, "d2", a, maintainer, domain.OutcomeRejected, "changed my mind"); !errors.Is(err, domain.ErrInvalidTransition) {
			t.Fatalf("second decision: %v", err)
		}
		if k := kinds(audit(t, store)); len(k) != 2 {
			t.Fatalf("audit %v, want requested and approved once each", k)
		}
	})

	t.Run("a waiting Step, its Approval and a decision survive a restart", func(t *testing.T) {
		store := newStore(t)
		c := build(t, store)
		got := push(t, c, "k1")
		a := got.Approvals[0]

		// The Platform restarts: a new coordinator over a new store that has only the committed data.
		store2 := reopen(t, store)
		c2 := build(t, store2)
		if _, err := c2.Reconcile(ctx); err != nil {
			t.Fatal(err)
		}
		if st := stepOf(t, store2, got.Execution.ID, got.Steps[0].ID); st.State != domain.StepAwaitingApproval {
			t.Fatalf("after the restart the step is %s", st.State)
		}
		kept, err := store2.Approval(ctx, a.ID)
		if err != nil || kept.State != domain.ApprovalRequested || kept.Evidence.Digest != a.Evidence.Digest || kept.Requester != Requester {
			t.Fatalf("approval after the restart: %+v, %v", kept, err)
		}
		s := startSession(t, store2, c2, nl, "w1")
		s.send(ports.Offer{FreeSlots: 1})
		s.expectNothing() // still waiting: a restart is not a decision

		if _, _, err := decide(c2, "d1", kept, maintainer, domain.OutcomeApproved, "ok"); err != nil {
			t.Fatal(err)
		}
		if as := s.assignment(); as.StepID != got.Steps[0].ID {
			t.Fatalf("assigned %+v", as)
		}

		// Restart again after the decision: the approval stays approved, and a second decision is refused.
		store3 := reopen(t, store2)
		c3 := build(t, store3)
		if _, err := c3.Reconcile(ctx); err != nil {
			t.Fatal(err)
		}
		done, err := store3.Approval(ctx, a.ID)
		if err != nil || done.State != domain.ApprovalApproved || done.Decision == nil || done.Decision.Approver != "maint-1" {
			t.Fatalf("decision after the restart: %+v, %v", done, err)
		}
		if _, _, err := decide(c3, "d2", done, maintainer, domain.OutcomeRejected, "no"); !errors.Is(err, domain.ErrInvalidTransition) {
			t.Fatalf("decision after the restart: %v", err)
		}
	})

	t.Run("cancelling the Execution withdraws a waiting Step and voids its Approval", func(t *testing.T) {
		store := newStore(t)
		c := build(t, store)
		got := push(t, c, "k1")
		a := got.Approvals[0]
		out, _, err := c.Cancel(ctx, coordinator.CancelRequest{Key: "c1", ExecutionID: got.Execution.ID, Reason: "not needed", By: human})
		if err != nil || !out.ExecutionCancelled {
			t.Fatalf("cancel: %+v, %v", out, err)
		}
		if st := stepOf(t, store, got.Execution.ID, got.Steps[0].ID); st.State != domain.StepCancelled {
			t.Fatalf("step %s", st.State)
		}
		v, _ := store.Approval(ctx, a.ID)
		if v.State != domain.ApprovalVoided || v.VoidReason != "execution_cancelled" || v.VoidedAt == nil {
			t.Fatalf("approval %+v", v)
		}
		if _, _, err := decide(c, "d1", a, maintainer, domain.OutcomeApproved, "ok"); !errors.Is(err, domain.ErrInvalidTransition) {
			t.Fatalf("deciding a voided approval: %v", err)
		}
		if k := kinds(audit(t, store)); !slices.Equal(k, []string{domain.AuditApprovalRequested, domain.AuditApprovalVoided}) {
			t.Fatalf("audit %v", k)
		}
	})

	t.Run("an approval covers its own Step only", func(t *testing.T) {
		store := newStore(t)
		c := build(t, store)
		got, _, err := c.Submit(ctx, coordinator.SubmitRequest{By: Requester, Key: "k1", ProjectID: "p1",
			Steps: []coordinator.StepSpec{{Name: "a", Capability: "git.push"}, {Name: "b", Capability: "git.push"}}})
		if err != nil || len(got.Approvals) != 2 {
			t.Fatalf("submit: %v (%d approvals)", err, len(got.Approvals))
		}
		if _, _, err := decide(c, "d1", got.Approvals[0], maintainer, domain.OutcomeApproved, ""); err != nil {
			t.Fatal(err)
		}
		s := startSession(t, store, c, nl, "w1")
		s.send(ports.Offer{FreeSlots: 2})
		if as := s.assignment(); as.StepID != got.Steps[0].ID {
			t.Fatalf("assigned %+v, want the approved step", as)
		}
		s.expectNothing()
		if st := stepOf(t, store, got.Execution.ID, got.Steps[1].ID); st.State != domain.StepAwaitingApproval {
			t.Fatalf("the other step is %s", st.State)
		}
	})
}
