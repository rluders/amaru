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
)

// ContextSuite checks context assembly through the Coordinator (FEAT-SKL-002): the same inputs give
// a byte-identical context, the context is recorded as the Step's Evidence and never changes, and a
// Skill that may not be used keeps the context from being built. newStores returns empty stores each
// call, with Contexts set.
func ContextSuite(t *testing.T, newStores func(t *testing.T) SkillStores) {
	t.Helper()
	ctx := context.Background()
	project := domain.ProjectKey("ssh://github.com/acme/app#0123456789abcdef0123456789abcdef01234567")
	maintainer := domain.Principal{Kind: domain.PrincipalHuman, ID: "maint-1", Role: domain.RoleMaintainer}

	type env struct {
		SkillStores
		c *coordinator.Coordinator
	}
	build := func(t *testing.T, withContexts bool) env {
		t.Helper()
		st := newStores(t)
		if err := st.Projects.Create(ctx, domain.Project{Key: project, Name: "app", CreatedAt: clock()}); err != nil {
			t.Fatal(err)
		}
		var execs atomic.Int64
		opts := []coordinator.Option{coordinator.WithTrust(st.Trust), coordinator.WithSkills(st.Skills),
			coordinator.WithExecutionIDs(func() string { return fmt.Sprintf("e-%d", execs.Add(1)) })}
		if withContexts {
			opts = append(opts, coordinator.WithContexts(st.Contexts))
		}
		c := coordinator.New(st.Executions, clock, opts...)
		if _, err := c.TrustContent(ctx, maintainer, project, domain.TrustProject, "", ""); err != nil {
			t.Fatal(err)
		}
		return env{st, c}
	}
	addSkill := func(t *testing.T, e env, scope domain.SkillScope, name, version, body string) domain.SkillRef {
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
		return domain.SkillRef{Name: name, Version: version, Hash: s.Hash}
	}
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
	knowledge := []domain.KnowledgeItem{{Path: "docs/login.md", Text: "Login uses a session cookie.\n"}}

	t.Run("the same inputs give a byte-identical context, recorded as the Step's Evidence", func(t *testing.T) {
		e := build(t, true)
		a := addSkill(t, e, domain.SkillOrganization, "go-style", "2", "Prefer small functions.\nReturn errors.")
		b := addSkill(t, e, domain.SkillOrganization, "testing", "1", "Table-driven tests.")
		one, two := submit(t, e, a, b), submit(t, e, a, b)
		req := func(s coordinator.Submitted) coordinator.ContextRequest {
			return coordinator.ContextRequest{ExecutionID: s.Execution.ID, StepID: s.Steps[0].ID, Intent: "Fix the flaky login test.", Knowledge: knowledge}
		}
		c1, err := e.c.AssembleContext(ctx, req(one))
		if err != nil {
			t.Fatal(err)
		}
		c2, err := e.c.AssembleContext(ctx, req(two))
		if err != nil {
			t.Fatal(err)
		}
		if c1.Document != c2.Document || c1.Hash != c2.Hash {
			t.Fatalf("two Steps with the same inputs got different contexts:\n%s\n---\n%s", c1.Document, c2.Document)
		}
		if c1.StepID == c2.StepID {
			t.Fatal("the two contexts are for the same Step")
		}
		var names []string
		for _, it := range c1.Items {
			names = append(names, it.Kind+":"+it.Name)
		}
		if got := strings.Join(names, ","); got != "intent:intent,skill:go-style,skill:testing,knowledge:docs/login.md" {
			t.Fatalf("items %s", got)
		}
		stored, err := e.Contexts.Get(ctx, one.Execution.ID, one.Steps[0].ID)
		if err != nil || stored.Document != c1.Document || stored.Hash != c1.Hash || stored.Format != domain.ContextFormat {
			t.Fatalf("stored %+v %v", stored, err)
		}
	})

	t.Run("a Step's context is written once: the same inputs return it, other inputs conflict", func(t *testing.T) {
		e := build(t, true)
		a := addSkill(t, e, domain.SkillOrganization, "go-style", "2", "Prefer small functions.")
		got := submit(t, e, a)
		req := coordinator.ContextRequest{ExecutionID: got.Execution.ID, StepID: got.Steps[0].ID, Intent: "one"}
		first, err := e.c.AssembleContext(ctx, req)
		if err != nil {
			t.Fatal(err)
		}
		again, err := e.c.AssembleContext(ctx, req)
		if err != nil || again.Hash != first.Hash {
			t.Fatalf("again: %+v %v", again, err)
		}
		req.Intent = "two"
		if _, err := e.c.AssembleContext(ctx, req); !errors.Is(err, domain.ErrConflict) {
			t.Fatalf("other inputs: %v", err)
		}
		if stored, _ := e.Contexts.Get(ctx, req.ExecutionID, req.StepID); stored.Hash != first.Hash {
			t.Fatal("the recorded context changed")
		}
	})

	t.Run("a Skill that may not be used keeps the context from being built", func(t *testing.T) {
		e := build(t, true)
		proj := addSkill(t, e, domain.SkillProject, "lint", "1", "Run the linter.")
		s, _ := e.Skills.Get(ctx, project, "lint", "1")
		if _, err := e.c.TrustContent(ctx, maintainer, project, domain.TrustSkill, "lint", domain.SkillTrustHash(s)); err != nil {
			t.Fatal(err)
		}
		got := submit(t, e, proj)
		if got.Steps[0].State != domain.StepQueued {
			t.Fatalf("%s %q", got.Steps[0].State, got.Steps[0].Reason)
		}
		if _, err := e.c.RevokeTrust(ctx, maintainer, project, domain.TrustSkill, "lint", "rewritten"); err != nil {
			t.Fatal(err)
		}
		_, err := e.c.AssembleContext(ctx, coordinator.ContextRequest{ExecutionID: got.Execution.ID, StepID: got.Steps[0].ID, Intent: "x"})
		if se, ok := domain.IsSkillError(err); !ok || se.Code != domain.ReasonSkillUntrusted {
			t.Fatalf("got %v, want skill.untrusted", err)
		}
		if _, err := e.Contexts.Get(ctx, got.Execution.ID, got.Steps[0].ID); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("a context was recorded: %v", err)
		}
	})

	t.Run("a Step that cannot start, an unknown Step and a missing store are refused", func(t *testing.T) {
		e := build(t, true)
		refused := submit(t, e, domain.SkillRef{Name: "missing", Version: "1", Hash: domain.HashBody("x")})
		if refused.Steps[0].State != domain.StepRefused {
			t.Fatalf("setup: %s", refused.Steps[0].State)
		}
		if _, err := e.c.AssembleContext(ctx, coordinator.ContextRequest{ExecutionID: refused.Execution.ID, StepID: refused.Steps[0].ID}); !errors.Is(err, domain.ErrInvalidTransition) {
			t.Errorf("refused Step: %v", err)
		}
		if _, err := e.c.AssembleContext(ctx, coordinator.ContextRequest{ExecutionID: refused.Execution.ID, StepID: "nope"}); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("unknown Step: %v", err)
		}
		no := build(t, false)
		ok := submit(t, no)
		if _, err := no.c.AssembleContext(ctx, coordinator.ContextRequest{ExecutionID: ok.Execution.ID, StepID: ok.Steps[0].ID}); !errors.Is(err, domain.ErrInvalid) {
			t.Errorf("no store: %v", err)
		}
	})
}
