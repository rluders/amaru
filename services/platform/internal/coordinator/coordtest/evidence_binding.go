package coordtest

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/rluders/amaru/services/platform/internal/coordinator"
	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
)

// EvidenceBindingSuite checks that an Approval is good only for the Evidence it was requested for
// (FEAT-REV-003): the Evidence names the Skills and the context, a Step whose Evidence changed
// after the request cannot be approved, and an approved Step whose Evidence changed afterwards does
// not start. newStores returns empty stores each call, with Contexts set.
func EvidenceBindingSuite(t *testing.T, newStores func(t *testing.T) SkillStores, nl NewLink) {
	t.Helper()
	ctx := context.Background()
	project := domain.ProjectKey("ssh://github.com/acme/app#0123456789abcdef0123456789abcdef01234567")
	maintainer := domain.Principal{Kind: domain.PrincipalHuman, ID: "maint-1", Role: domain.RoleMaintainer}

	type env struct {
		SkillStores
		c *coordinator.Coordinator
	}
	build := func(t *testing.T) env {
		t.Helper()
		st := newStores(t)
		if err := st.Projects.Create(ctx, domain.Project{Key: project, Name: "app", CreatedAt: clock()}); err != nil {
			t.Fatal(err)
		}
		var execs, apprs, attempts atomic.Int64
		c := coordinator.New(st.Executions, clock, coordinator.WithTrust(st.Trust), coordinator.WithSkills(st.Skills), coordinator.WithContexts(st.Contexts),
			coordinator.WithLeasePeriod(leasePeriod),
			coordinator.WithAttemptIDs(func() string { return fmt.Sprintf("att-%d", attempts.Add(1)) }),
			coordinator.WithExecutionIDs(func() string { return fmt.Sprintf("e-%d", execs.Add(1)) }),
			coordinator.WithApprovalIDs(func() string { return fmt.Sprintf("appr-%d", apprs.Add(1)) }))
		if _, err := c.TrustContent(ctx, maintainer, project, domain.TrustProject, "", ""); err != nil {
			t.Fatal(err)
		}
		return env{st, c}
	}
	var n atomic.Int64
	push := func(t *testing.T, e env, refs ...domain.SkillRef) coordinator.Submitted {
		t.Helper()
		got, _, err := e.c.Submit(ctx, coordinator.SubmitRequest{By: Requester, Key: fmt.Sprintf("k%d", n.Add(1)), ProjectID: string(project),
			Steps: []coordinator.StepSpec{{Name: "push", Capability: "git.push", Skills: refs}}})
		if err != nil {
			t.Fatal(err)
		}
		if len(got.Approvals) != 1 || got.Steps[0].State != domain.StepAwaitingApproval {
			t.Fatalf("step %s, %d approvals; want awaiting_approval and one", got.Steps[0].State, len(got.Approvals))
		}
		return got
	}
	decide := func(e env, key string, a domain.Approval, outcome domain.ApprovalOutcome, why string) (coordinator.Decided, bool, error) {
		return e.c.DecideApproval(ctx, coordinator.DecideRequest{Key: key, ApprovalID: a.ID, By: maintainer, Outcome: outcome, Rationale: why, EvidenceDigest: a.Evidence.Digest})
	}
	stepOf := func(t *testing.T, e env, exec, step string) domain.StepExecution {
		t.Helper()
		_, steps, err := e.Executions.Get(ctx, exec)
		if err != nil {
			t.Fatal(err)
		}
		i := slices.IndexFunc(steps, func(s domain.StepExecution) bool { return s.ID == step })
		if i < 0 {
			t.Fatalf("no step %s", step)
		}
		return steps[i]
	}
	assemble := func(t *testing.T, e env, got coordinator.Submitted, intent string) {
		t.Helper()
		if _, err := e.c.AssembleContext(ctx, coordinator.ContextRequest{ExecutionID: got.Execution.ID, StepID: got.Steps[0].ID, Intent: intent}); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("the Evidence names the pinned Skills, so a different Skill is a different Approval", func(t *testing.T) {
		e := build(t)
		mk := func(body string) domain.SkillRef {
			s, err := domain.NewSkill(clock(), maintainer, domain.SkillOrganization, "", "review", "1", body)
			if err != nil {
				t.Fatal(err)
			}
			_ = e.Skills.Add(ctx, s)
			return domain.SkillRef{Name: "review", Version: "1", Hash: s.Hash}
		}
		a := push(t, e, mk("one")).Approvals[0]
		var kinds []string
		for _, it := range a.Evidence.Items {
			kinds = append(kinds, it.Kind)
		}
		if !slices.Contains(kinds, "skill") {
			t.Fatalf("evidence kinds %v", kinds)
		}
		e2 := build(t)
		mk2 := func(body string) domain.SkillRef {
			s, _ := domain.NewSkill(clock(), maintainer, domain.SkillOrganization, "", "review", "1", body)
			_ = e2.Skills.Add(ctx, s)
			return domain.SkillRef{Name: "review", Version: "1", Hash: s.Hash}
		}
		if b := push(t, e2, mk2("two")).Approvals[0]; b.Evidence.Digest == a.Evidence.Digest {
			t.Error("two Skill bodies gave the same Evidence digest")
		}
	})

	t.Run("with no context built, the Step is approved and runs as before", func(t *testing.T) {
		e := build(t)
		got := push(t, e)
		out, _, err := decide(e, "d1", got.Approvals[0], domain.OutcomeApproved, "ok")
		if err != nil || out.Step.State != domain.StepQueued {
			t.Fatalf("%v, %+v", err, out.Step)
		}
		s := startSession(t, e.Executions, e.c, nl, "w1")
		s.send(ports.Offer{FreeSlots: 1})
		if as := s.assignment(); as.StepID != got.Steps[0].ID {
			t.Fatalf("assigned %+v", as)
		}
	})

	t.Run("a context built after the request keeps the Step from being approved on the old Evidence", func(t *testing.T) {
		e := build(t)
		got := push(t, e)
		assemble(t, e, got, "do something else")
		_, _, err := decide(e, "d1", got.Approvals[0], domain.OutcomeApproved, "ok")
		if !errors.Is(err, domain.ErrEvidenceChanged) {
			t.Fatalf("approve: %v", err)
		}
		a, _ := e.Executions.Approval(ctx, got.Approvals[0].ID)
		if st := stepOf(t, e, got.Execution.ID, got.Steps[0].ID); a.State != domain.ApprovalRequested || st.State != domain.StepAwaitingApproval {
			t.Fatalf("a refused approval wrote something: approval %s, step %s", a.State, st.State)
		}
		// A person can still reject it.
		out, _, err := decide(e, "d2", got.Approvals[0], domain.OutcomeRejected, "stale")
		if err != nil || out.Step.State != domain.StepFailed {
			t.Fatalf("reject: %v, %+v", err, out.Step)
		}
	})

	t.Run("a context built after the approval holds the Step, with the reason, and it never starts", func(t *testing.T) {
		e := build(t)
		got := push(t, e)
		if _, _, err := decide(e, "d1", got.Approvals[0], domain.OutcomeApproved, "ok"); err != nil {
			t.Fatal(err)
		}
		assemble(t, e, got, "something nobody approved")
		s := startSession(t, e.Executions, e.c, nl, "w1")
		s.send(ports.Offer{FreeSlots: 1})
		s.expectNothing()
		st := stepOf(t, e, got.Execution.ID, got.Steps[0].ID)
		if st.State != domain.StepQueued || !strings.Contains(st.Reason, "other Evidence") || st.AttemptID != "" {
			t.Fatalf("step %s %q attempt %q", st.State, st.Reason, st.AttemptID)
		}
	})
}
