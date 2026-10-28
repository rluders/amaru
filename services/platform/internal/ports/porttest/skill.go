package porttest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
)

// SkillStore runs the Skill contract (FEAT-SKL-001) against newStore, which returns an empty store
// each call: a Skill is immutable per scope, Project, name and version; a Project's own Skill
// beats the Organization's; and nothing leaks between Projects.
func SkillStore(t *testing.T, newStore func(t *testing.T) ports.SkillStore) {
	ctx := context.Background()
	now := time.Date(2026, 10, 28, 9, 0, 0, 0, time.UTC)
	maint := domain.Principal{Kind: domain.PrincipalHuman, ID: "maint-1", Role: domain.RoleMaintainer}
	pa := domain.ProjectKey("ssh://github.com/acme/a#0123456789abcdef0123456789abcdef01234567")
	pb := domain.ProjectKey("ssh://github.com/acme/b#0123456789abcdef0123456789abcdef01234567")
	mk := func(t *testing.T, scope domain.SkillScope, project domain.ProjectKey, name, version, body string) domain.Skill {
		t.Helper()
		s, err := domain.NewSkill(now, maint, scope, project, name, version, body)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}

	t.Run("a Skill is stored and read back with the hash of its body", func(t *testing.T) {
		s := newStore(t)
		want := mk(t, domain.SkillOrganization, "", "review", "1", "Check the diff.")
		if err := s.Add(ctx, want); err != nil {
			t.Fatal(err)
		}
		got, err := s.Get(ctx, "", "review", "1")
		if err != nil || got.Body != want.Body || got.Hash != domain.HashBody("Check the diff.") || got.AddedBy != "maint-1" || !got.AddedAt.Equal(now) || got.Scope != domain.SkillOrganization {
			t.Fatalf("%+v %v", got, err)
		}
		if _, err := s.Get(ctx, "", "review", "2"); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("other version: %v", err)
		}
		if _, err := s.Get(ctx, "", "other", "1"); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("other name: %v", err)
		}
	})

	t.Run("a version never changes: the same body is a no-op, another body is a conflict", func(t *testing.T) {
		s := newStore(t)
		a := mk(t, domain.SkillOrganization, "", "review", "1", "v1 body")
		_ = s.Add(ctx, a)
		if err := s.Add(ctx, a); err != nil {
			t.Fatalf("again: %v", err)
		}
		if err := s.Add(ctx, mk(t, domain.SkillOrganization, "", "review", "1", "edited body")); !errors.Is(err, domain.ErrConflict) {
			t.Fatalf("edited: %v", err)
		}
		got, _ := s.Get(ctx, "", "review", "1")
		if got.Body != "v1 body" {
			t.Fatalf("the Skill changed: %q", got.Body)
		}
		if err := s.Add(ctx, mk(t, domain.SkillOrganization, "", "review", "2", "edited body")); err != nil {
			t.Fatalf("a new version: %v", err)
		}
	})

	t.Run("a Project's own Skill beats the Organization's, and one Project cannot see another's", func(t *testing.T) {
		s := newStore(t)
		_ = s.Add(ctx, mk(t, domain.SkillOrganization, "", "review", "1", "org body"))
		_ = s.Add(ctx, mk(t, domain.SkillProject, pa, "review", "1", "project A body"))
		_ = s.Add(ctx, mk(t, domain.SkillProject, pb, "only-b", "1", "project B body"))
		if got, _ := s.Get(ctx, pa, "review", "1"); got.Body != "project A body" || got.Scope != domain.SkillProject || got.ProjectKey != pa {
			t.Errorf("project A: %+v", got)
		}
		if got, _ := s.Get(ctx, pb, "review", "1"); got.Body != "org body" {
			t.Errorf("project B falls back to the organization: %+v", got)
		}
		if got, _ := s.Get(ctx, "", "review", "1"); got.Body != "org body" {
			t.Errorf("no project: %+v", got)
		}
		if _, err := s.Get(ctx, pa, "only-b", "1"); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("project A read project B's skill: %v", err)
		}
		if _, err := s.Get(ctx, "", "only-b", "1"); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("the organization read a project's skill: %v", err)
		}
	})

	t.Run("an invalid Skill is refused", func(t *testing.T) {
		s := newStore(t)
		bad := mk(t, domain.SkillOrganization, "", "review", "1", "body")
		bad.Hash = domain.HashBody("something else")
		if err := s.Add(ctx, bad); !errors.Is(err, domain.ErrInvalid) {
			t.Fatalf("got %v", err)
		}
	})
}
