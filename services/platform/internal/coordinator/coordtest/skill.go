package coordtest

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/rluders/amaru/services/platform/internal/coordinator"
	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
)

// SkillStores are the stores SkillSuite needs; they may share a database.
type SkillStores struct {
	Executions ports.ExecutionStore
	Trust      ports.TrustStore
	Projects   ports.ProjectStore
	Skills     ports.SkillStore
}

// SkillSuite checks trusted, hash-pinned Skills through the Coordinator and the Worker link
// (FEAT-SKL-001): a Step that pins a Skill starts only when every reference is well formed, names
// a Skill that exists, carries the Skill's own hash and, for a Project Skill, is trusted at that
// content; any failure refuses the Step before a Worker sees it, with a code; and a trust revoked
// after Submit keeps a queued Step from starting. newStores returns empty stores each call.
func SkillSuite(t *testing.T, newStores func(t *testing.T) SkillStores, nl NewLink) {
	t.Helper()
	ctx := context.Background()
	project := domain.ProjectKey("ssh://github.com/acme/app#0123456789abcdef0123456789abcdef01234567")
	maintainer := domain.Principal{Kind: domain.PrincipalHuman, ID: "maint-1", Role: domain.RoleMaintainer}

	// env is a Coordinator over fresh stores with the Project trusted, so a refusal is the Skill's.
	type env struct {
		SkillStores
		c *coordinator.Coordinator
	}
	build := func(t *testing.T, withSkills bool) env {
		t.Helper()
		st := newStores(t)
		if err := st.Projects.Create(ctx, domain.Project{Key: project, Name: "app", CreatedAt: clock()}); err != nil {
			t.Fatal(err)
		}
		var attempts, execs atomic.Int64
		opts := []coordinator.Option{coordinator.WithTrust(st.Trust), coordinator.WithLeasePeriod(leasePeriod),
			coordinator.WithAttemptIDs(func() string { return fmt.Sprintf("att-%d", attempts.Add(1)) }),
			coordinator.WithExecutionIDs(func() string { return fmt.Sprintf("e-%d", execs.Add(1)) })}
		if withSkills {
			opts = append(opts, coordinator.WithSkills(st.Skills))
		}
		c := coordinator.New(st.Executions, clock, opts...)
		if _, err := c.TrustContent(ctx, maintainer, project, domain.TrustProject, "", ""); err != nil {
			t.Fatal(err)
		}
		return env{st, c}
	}
	skill := func(t *testing.T, e env, scope domain.SkillScope, name, version, body string) domain.Skill {
		t.Helper()
		var key domain.ProjectKey
		if scope == domain.SkillProject {
			key = project
		}
		s, err := domain.NewSkill(clock(), maintainer, scope, key, name, version, body)
		if err != nil {
			t.Fatal(err)
		}
		if err := e.Skills.Add(ctx, s); err != nil {
			t.Fatal(err)
		}
		return s
	}
	ref := func(s domain.Skill) domain.SkillRef {
		return domain.SkillRef{Name: s.Name, Version: s.Version, Hash: s.Hash}
	}
	// submit runs one code.test Step (allowed by the default rules) that pins refs.
	var n atomic.Int64
	submit := func(t *testing.T, e env, refs ...domain.SkillRef) coordinator.Submitted {
		t.Helper()
		got, _, err := e.c.Submit(ctx, coordinator.SubmitRequest{By: Requester, Key: fmt.Sprintf("k%d", n.Add(1)), ProjectID: string(project),
			Steps: []coordinator.StepSpec{{Name: "test", Capability: "code.test", Skills: refs}}})
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	refused := func(t *testing.T, got coordinator.Submitted, code string) {
		t.Helper()
		st := got.Steps[0]
		if st.State != domain.StepRefused || !strings.HasPrefix(st.Reason, code+":") {
			t.Fatalf("state %s reason %q, want refused with %s", st.State, st.Reason, code)
		}
	}

	t.Run("a Step with no Skill is not affected, and one with an Organization Skill at its hash starts", func(t *testing.T) {
		e := build(t, true)
		if st := submit(t, e).Steps[0]; st.State != domain.StepQueued {
			t.Fatalf("no skill: %s %q", st.State, st.Reason)
		}
		org := skill(t, e, domain.SkillOrganization, "review", "1", "Check the diff.")
		got := submit(t, e, ref(org))
		if st := got.Steps[0]; st.State != domain.StepQueued || len(st.Skills) != 1 || st.Skills[0] != ref(org) {
			t.Fatalf("%s %q %+v", st.State, st.Reason, st.Skills)
		}
		s := startSession(t, e.Executions, e.c, nl, "w1")
		s.send(ports.Offer{FreeSlots: 2})
		_ = s.assignment()
		_ = s.assignment()
	})

	t.Run("a bad reference refuses the Step before any Worker sees it, with a code for each cause", func(t *testing.T) {
		e := build(t, true)
		org := skill(t, e, domain.SkillOrganization, "review", "1", "Check the diff.")
		refused(t, submit(t, e, domain.SkillRef{Name: "Not A Slug", Version: "1", Hash: org.Hash}), domain.ReasonSkillInvalid)
		refused(t, submit(t, e, domain.SkillRef{Name: "review", Version: "1", Hash: "sha256:abc"}), domain.ReasonSkillInvalid)
		refused(t, submit(t, e, ref(org), ref(org)), domain.ReasonSkillInvalid)
		refused(t, submit(t, e, domain.SkillRef{Name: "missing", Version: "1", Hash: org.Hash}), domain.ReasonSkillUnknown)
		refused(t, submit(t, e, domain.SkillRef{Name: "review", Version: "9", Hash: org.Hash}), domain.ReasonSkillUnknown)
		s := startSession(t, e.Executions, e.c, nl, "w1")
		s.send(ports.Offer{FreeSlots: 1})
		s.expectNothing()
	})

	t.Run("the pinned hash is enforced: another hash is refused, and a version cannot be edited to fit", func(t *testing.T) {
		e := build(t, true)
		org := skill(t, e, domain.SkillOrganization, "review", "1", "Check the diff.")
		wrong := ref(org)
		wrong.Hash = domain.HashBody("Check the diff, and be quick.")
		refused(t, submit(t, e, wrong), domain.ReasonSkillHashMismatch)
		edited, _ := domain.NewSkill(clock(), maintainer, domain.SkillOrganization, "", "review", "1", "Check the diff, and be quick.")
		if err := e.Skills.Add(ctx, edited); err == nil {
			t.Fatal("a Skill version was changed under a pin")
		}
		if st := submit(t, e, ref(org)).Steps[0]; st.State != domain.StepQueued {
			t.Fatalf("the original pin: %s %q", st.State, st.Reason)
		}
	})

	t.Run("a Project Skill needs a trust at its exact content", func(t *testing.T) {
		e := build(t, true)
		proj := skill(t, e, domain.SkillProject, "lint", "1", "Run the linter.")
		refused(t, submit(t, e, ref(proj)), domain.ReasonSkillUntrusted)
		stale := "sha256:" + strings.Repeat("a", 64)
		if _, err := e.c.TrustContent(ctx, maintainer, project, domain.TrustSkill, "lint", stale); err != nil {
			t.Fatal(err)
		}
		refused(t, submit(t, e, ref(proj)), domain.ReasonSkillUntrusted)
		if _, err := e.c.TrustContent(ctx, maintainer, project, domain.TrustSkill, "lint", domain.SkillTrustHash(proj)); err != nil {
			t.Fatal(err)
		}
		if st := submit(t, e, ref(proj)).Steps[0]; st.State != domain.StepQueued {
			t.Fatalf("trusted: %s %q", st.State, st.Reason)
		}
		// A new version has a new body, so the old trust does not cover it.
		next := skill(t, e, domain.SkillProject, "lint", "2", "Run the linter, then the formatter.")
		refused(t, submit(t, e, ref(next)), domain.ReasonSkillUntrusted)
	})

	t.Run("a trust revoked after Submit keeps a queued Step from starting until it is trusted again", func(t *testing.T) {
		e := build(t, true)
		proj := skill(t, e, domain.SkillProject, "lint", "1", "Run the linter.")
		if _, err := e.c.TrustContent(ctx, maintainer, project, domain.TrustSkill, "lint", domain.SkillTrustHash(proj)); err != nil {
			t.Fatal(err)
		}
		got := submit(t, e, ref(proj))
		if got.Steps[0].State != domain.StepQueued {
			t.Fatalf("%s %q", got.Steps[0].State, got.Steps[0].Reason)
		}
		if _, err := e.c.RevokeTrust(ctx, maintainer, project, domain.TrustSkill, "lint", "skill rewritten upstream"); err != nil {
			t.Fatal(err)
		}
		s := startSession(t, e.Executions, e.c, nl, "w1")
		s.send(ports.Offer{FreeSlots: 1})
		s.expectNothing()
		if _, st := s.step(got.Execution.ID, got.Steps[0].ID); st.State != domain.StepQueued || !strings.HasPrefix(st.Reason, domain.ReasonSkillUntrusted+":") {
			t.Fatalf("step %s reason %q, want queued with the skill reason", st.State, st.Reason)
		}
		if _, err := e.c.TrustContent(ctx, maintainer, project, domain.TrustSkill, "lint", domain.SkillTrustHash(proj)); err != nil {
			t.Fatal(err)
		}
		if a := s.assignment(); a.StepID != got.Steps[0].ID {
			t.Fatalf("assigned %+v", a)
		}
	})

	t.Run("without a Skill store a Step that pins a Skill is refused", func(t *testing.T) {
		e := build(t, false)
		refused(t, submit(t, e, domain.SkillRef{Name: "review", Version: "1", Hash: domain.HashBody("x")}), domain.ReasonSkillUnknown)
		if st := submit(t, e).Steps[0]; st.State != domain.StepQueued {
			t.Fatalf("a Step with no Skill: %s", st.State)
		}
	})
}
