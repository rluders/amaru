package porttest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
)

func newApproval(t *testing.T, id, exec, step string) domain.Approval {
	t.Helper()
	ev, err := domain.NewEvidence(domain.EvidenceItem{Kind: "execution", ID: exec}, domain.EvidenceItem{Kind: "step_execution", ID: step})
	if err != nil {
		t.Fatal(err)
	}
	a, err := domain.NewApproval(t0, id, "p1", domain.ApprovalGate{Kind: domain.GatePolicyRequired, ExecutionID: exec, StepID: step, Capability: "git.push"},
		domain.Principal{Kind: domain.PrincipalHuman, ID: "u1", Role: domain.RoleContributor}, ev)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

// ApprovalStore runs the Approval part of the ports.ExecutionStore contract (FEAT-REV-001)
// against newStore, which must return an empty store each call. reopen returns a new store over
// the same data, as a restarted Platform sees it.
func ApprovalStore(t *testing.T, newStore func(t *testing.T) ports.ExecutionStore, reopen func(t *testing.T, s ports.ExecutionStore) ports.ExecutionStore) {
	ctx := context.Background()
	if reopen == nil {
		reopen = func(_ *testing.T, s ports.ExecutionStore) ports.ExecutionStore { return s }
	}
	maintainer := domain.Principal{Kind: domain.PrincipalHuman, ID: "m1", Role: domain.RoleMaintainer}
	rec := func(key string) domain.CommandRecord {
		return domain.CommandRecord{Name: "execution.submit", Key: key, Fingerprint: "fp", Result: []byte(`{}`), At: t0}
	}
	// waiting creates an Execution with one Step awaiting approval and its requested Approval.
	waiting := func(t *testing.T, s ports.ExecutionStore, exec string) (domain.Approval, domain.StepExecution) {
		t.Helper()
		e, steps := newExec(exec, exec+".s")
		if err := steps[0].RequireApproval(); err != nil {
			t.Fatal(err)
		}
		a := newApproval(t, exec+".a", exec, steps[0].ID)
		_, cs, err := s.CreateOnce(ctx, rec("k-"+exec), e, steps, a)
		if err != nil {
			t.Fatal(err)
		}
		a.Version = 1
		return a, cs[0]
	}

	t.Run("created with the Execution, at version 1, with its audit event, and kept across a restart", func(t *testing.T) {
		s := newStore(t)
		a, _ := waiting(t, s, "e1")
		got, err := reopen(t, s).Approval(ctx, a.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got.Version != 1 || got.State != domain.ApprovalRequested || got.Gate != a.Gate || got.Requester != a.Requester ||
			got.Evidence.Digest != a.Evidence.Digest || len(got.Evidence.Items) != 2 || got.MinRole != domain.RoleMaintainer ||
			got.ProjectID != "p1" || !got.RequestedAt.Equal(t0) {
			t.Fatalf("got %+v, want %+v", got, a)
		}
		list, err := reopen(t, s).Approvals(ctx, "e1")
		if err != nil || len(list) != 1 || list[0].ID != a.ID {
			t.Fatalf("approvals %+v, %v", list, err)
		}
		evs, _ := s.Audit(ctx, 0, 0)
		if len(evs) != 1 || evs[0].Kind != domain.AuditApprovalRequested || evs[0].TargetID != a.ID || evs[0].Seq != 1 {
			t.Fatalf("audit %+v", evs)
		}
	})

	t.Run("unknown approval and execution", func(t *testing.T) {
		s := newStore(t)
		if _, err := s.Approval(ctx, "nope"); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("approval: %v", err)
		}
		if _, err := s.Approvals(ctx, "nope"); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("approvals: %v", err)
		}
		e, steps := newExec("e1", "s1")
		if _, _, err := s.Create(ctx, e, steps); err != nil {
			t.Fatal(err)
		}
		if list, err := s.Approvals(ctx, "e1"); err != nil || len(list) != 0 {
			t.Fatalf("an execution with none: %+v, %v", list, err)
		}
	})

	t.Run("an Approval that gates no Step of the Execution, or is stored twice, creates nothing", func(t *testing.T) {
		s := newStore(t)
		e, steps := newExec("e1", "s1")
		elsewhere := newApproval(t, "a1", "e1", "other")
		if _, _, err := s.CreateOnce(ctx, rec("k1"), e, steps, elsewhere); !errors.Is(err, domain.ErrInvalid) {
			t.Fatalf("wrong step: %v", err)
		}
		decided := newApproval(t, "a2", "e1", "s1")
		if err := decided.Decide(t0, maintainer, domain.OutcomeApproved, "", decided.Evidence.Digest); err != nil {
			t.Fatal(err)
		}
		if _, _, err := s.CreateOnce(ctx, rec("k2"), e, steps, decided); !errors.Is(err, domain.ErrInvalid) {
			t.Fatalf("already decided: %v", err)
		}
		if _, _, err := s.Get(ctx, "e1"); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("a refused create left the execution: %v", err)
		}
		if _, err := s.Command(ctx, "execution.submit", "k1"); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("a refused create left its command: %v", err)
		}
		if evs, _ := s.Audit(ctx, 0, 0); len(evs) != 0 {
			t.Fatalf("a refused create left audit events: %+v", evs)
		}
		w, wsteps := newExec("e2", "s2")
		_ = wsteps
		ok := newApproval(t, "dup", "e2", "s2")
		if _, _, err := s.CreateOnce(ctx, rec("k3"), w, wsteps, ok); err != nil {
			t.Fatal(err)
		}
		e3, steps3 := newExec("e3", "s3")
		if _, _, err := s.CreateOnce(ctx, rec("k4"), e3, steps3, newApproval(t, "dup", "e3", "s3")); !errors.Is(err, domain.ErrExists) {
			t.Fatalf("duplicate approval id: %v", err)
		}
		if _, _, err := s.Get(ctx, "e3"); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("a duplicate approval left its execution: %v", err)
		}
	})

	t.Run("a decision and the Step it moves are one write, with the audit event", func(t *testing.T) {
		s := newStore(t)
		a, st := waiting(t, s, "e1")
		if err := a.Decide(t0.Add(time.Minute), maintainer, domain.OutcomeApproved, "ok", a.Evidence.Digest); err != nil {
			t.Fatal(err)
		}
		if err := st.Approve(); err != nil {
			t.Fatal(err)
		}
		out, err := s.Commit(ctx, ports.ExecutionChange{Steps: []domain.StepExecution{st}, Approvals: []domain.Approval{a}})
		if err != nil {
			t.Fatal(err)
		}
		if len(out.Approvals) != 1 || out.Approvals[0].Version != 2 || out.Steps[0].Version != 2 {
			t.Fatalf("out %+v", out)
		}
		got, _ := reopen(t, s).Approval(ctx, a.ID)
		if got.State != domain.ApprovalApproved || got.Decision == nil || got.Decision.Approver != "m1" || got.Decision.ApproverRole != domain.RoleMaintainer ||
			got.Decision.Rationale != "ok" || got.Decision.EvidenceDigest != a.Evidence.Digest || !got.Decision.DecidedAt.Equal(t0.Add(time.Minute)) {
			t.Fatalf("got %+v", got)
		}
		_, steps, _ := s.Get(ctx, "e1")
		if steps[0].State != domain.StepQueued {
			t.Fatalf("step %s", steps[0].State)
		}
		evs, _ := s.Audit(ctx, 0, 0)
		if len(evs) != 2 || evs[1].Kind != domain.AuditApprovalApproved || evs[1].Actor != "m1" || evs[1].ActorKind != domain.ActorHuman || evs[1].Seq != 2 {
			t.Fatalf("audit %+v", evs)
		}
	})

	t.Run("a stale Approval version writes nothing, not even the Step", func(t *testing.T) {
		s := newStore(t)
		a, st := waiting(t, s, "e1")
		a.Version = 7
		if err := a.Decide(t0, maintainer, domain.OutcomeApproved, "", a.Evidence.Digest); err != nil {
			t.Fatal(err)
		}
		if err := st.Approve(); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Commit(ctx, ports.ExecutionChange{Steps: []domain.StepExecution{st}, Approvals: []domain.Approval{a}}); !errors.Is(err, domain.ErrConflict) {
			t.Fatalf("stale approval: %v", err)
		}
		_, steps, _ := s.Get(ctx, "e1")
		cur, _ := s.Approval(ctx, a.ID)
		if steps[0].State != domain.StepAwaitingApproval || cur.State != domain.ApprovalRequested {
			t.Fatalf("a refused write changed the step (%s) or the approval (%s)", steps[0].State, cur.State)
		}
		if evs, _ := s.Audit(ctx, 0, 0); len(evs) != 1 {
			t.Fatalf("a refused write left audit events: %+v", evs)
		}
	})

	t.Run("what identifies an Approval is never edited, and a decided one is final", func(t *testing.T) {
		s := newStore(t)
		a, st := waiting(t, s, "e1")
		forged := a
		forged.Requester = maintainer
		if err := forged.Decide(t0, maintainer, domain.OutcomeApproved, "", forged.Evidence.Digest); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Commit(ctx, ports.ExecutionChange{Steps: []domain.StepExecution{st}, Approvals: []domain.Approval{forged}}); !errors.Is(err, domain.ErrInvalid) {
			t.Fatalf("edited requester: %v", err)
		}
		other := a
		other.Evidence = domain.Evidence{Digest: "sha256:other", Items: a.Evidence.Items}
		if err := other.Decide(t0, maintainer, domain.OutcomeApproved, "", "sha256:other"); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Commit(ctx, ports.ExecutionChange{Steps: []domain.StepExecution{st}, Approvals: []domain.Approval{other}}); !errors.Is(err, domain.ErrInvalid) {
			t.Fatalf("edited evidence: %v", err)
		}
		if err := a.Decide(t0, maintainer, domain.OutcomeRejected, "no", a.Evidence.Digest); err != nil {
			t.Fatal(err)
		}
		if err := st.Reject(t0, &domain.ExecError{Code: "approval_rejected", Message: "no"}); err != nil {
			t.Fatal(err)
		}
		out, err := s.Commit(ctx, ports.ExecutionChange{Steps: []domain.StepExecution{st}, Approvals: []domain.Approval{a}})
		if err != nil {
			t.Fatal(err)
		}
		// A decided Approval cannot be decided or voided again.
		again := out.Approvals[0]
		again.State, again.Decision = domain.ApprovalApproved, &domain.ApprovalDecision{Outcome: domain.OutcomeApproved, Approver: "m1", DecidedAt: t0, EvidenceDigest: again.Evidence.Digest}
		if _, err := s.Commit(ctx, ports.ExecutionChange{Steps: []domain.StepExecution{out.Steps[0]}, Approvals: []domain.Approval{again}}); err == nil {
			t.Fatal("a rejected approval was changed to approved")
		}
		if cur, _ := s.Approval(ctx, a.ID); cur.State != domain.ApprovalRejected {
			t.Fatalf("state %s", cur.State)
		}
	})

	t.Run("a requested Approval can be voided with a reason", func(t *testing.T) {
		s := newStore(t)
		a, st := waiting(t, s, "e1")
		if err := a.Void(t0.Add(time.Hour), "execution_cancelled"); err != nil {
			t.Fatal(err)
		}
		if err := st.CancelAwaiting(t0.Add(time.Hour)); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Commit(ctx, ports.ExecutionChange{Steps: []domain.StepExecution{st}, Approvals: []domain.Approval{a}}); err != nil {
			t.Fatal(err)
		}
		got, _ := reopen(t, s).Approval(ctx, a.ID)
		if got.State != domain.ApprovalVoided || got.VoidReason != "execution_cancelled" || got.VoidedAt == nil {
			t.Fatalf("got %+v", got)
		}
		evs, _ := s.Audit(ctx, 0, 0)
		if len(evs) != 2 || evs[1].Kind != domain.AuditApprovalVoided || evs[1].ActorKind != domain.ActorPlatform || evs[1].Reason != "execution_cancelled" {
			t.Fatalf("audit %+v", evs)
		}
	})
}
