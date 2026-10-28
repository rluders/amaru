package porttest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
)

func newGrant(t *testing.T, id string, mod func(*domain.Grant)) domain.Grant {
	t.Helper()
	g := domain.Grant{ID: id, Level: domain.GrantProject, ProjectID: "p1",
		Subject: domain.Subject{Kind: domain.SubjectWorkflow, ID: "wf"}, Capabilities: []domain.Capability{"mcp.call", "asset.upload"},
		Resource: domain.Resource{Type: "git_ref", Value: "refs/heads/amaru/*"}, DefinitionHash: "sha256:abc"}
	if mod != nil {
		mod(&g)
	}
	out, err := domain.NewGrant(time.Date(2026, 10, 28, 9, 0, 0, 0, time.UTC), domain.Principal{Kind: domain.PrincipalHuman, ID: "u1", Role: domain.RoleOwner}, g)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// GrantStore runs the ports.GrantStore contract against newStore, which must return an empty
// store each call.
func GrantStore(t *testing.T, newStore func(t *testing.T) ports.GrantStore) {
	ctx := context.Background()
	admin := domain.Principal{Kind: domain.PrincipalHuman, ID: "u2", Role: domain.RoleAdmin}
	later := time.Date(2026, 10, 28, 10, 0, 0, 0, time.UTC)

	t.Run("create then get keeps every field", func(t *testing.T) {
		s := newStore(t)
		exp := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
		g := newGrant(t, "g1", func(g *domain.Grant) { g.ExpiresAt = &exp })
		made, err := s.Create(ctx, g)
		if err != nil {
			t.Fatal(err)
		}
		if made.Version != 1 {
			t.Fatalf("version %d, want 1", made.Version)
		}
		got, err := s.Get(ctx, "g1")
		if err != nil {
			t.Fatal(err)
		}
		g.Version = 1
		if got.ID != g.ID || got.Version != 1 || got.Level != g.Level || got.ProjectID != g.ProjectID || got.Subject != g.Subject ||
			len(got.Capabilities) != 2 || got.Capabilities[0] != "mcp.call" || got.Resource != g.Resource ||
			got.DefinitionHash != g.DefinitionHash || got.GrantedBy != "u1" || !got.CreatedAt.Equal(g.CreatedAt) ||
			got.ExpiresAt == nil || !got.ExpiresAt.Equal(exp) || got.Status != domain.GrantActive || got.RevokedAt != nil {
			t.Fatalf("got %+v, want %+v", got, g)
		}
	})

	t.Run("create twice and invalid", func(t *testing.T) {
		s := newStore(t)
		if _, err := s.Create(ctx, newGrant(t, "g1", nil)); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Create(ctx, newGrant(t, "g1", nil)); !errors.Is(err, domain.ErrExists) {
			t.Fatalf("duplicate: %v", err)
		}
		bad := newGrant(t, "g2", nil)
		bad.Capabilities = nil
		if _, err := s.Create(ctx, bad); !errors.Is(err, domain.ErrInvalid) {
			t.Fatalf("invalid: %v", err)
		}
	})

	t.Run("get unknown", func(t *testing.T) {
		if _, err := newStore(t).Get(ctx, "nope"); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("got %v", err)
		}
	})

	t.Run("revoke is recorded and kept", func(t *testing.T) {
		s := newStore(t)
		g, _ := s.Create(ctx, newGrant(t, "g1", nil))
		if err := g.Revoke(later, admin, "contract ended"); err != nil {
			t.Fatal(err)
		}
		got, err := s.Update(ctx, g)
		if err != nil {
			t.Fatal(err)
		}
		if got.Version != 2 || got.Status != domain.GrantRevoked || got.RevokedBy != "u2" || got.RevokedAt == nil || !got.RevokedAt.Equal(later) || got.RevokeReason != "contract ended" {
			t.Fatalf("got %+v", got)
		}
		again, err := s.Get(ctx, "g1")
		if err != nil || again.Status != domain.GrantRevoked || again.GrantedBy != "u1" {
			t.Fatalf("after revoke: %+v %v", again, err)
		}
	})

	t.Run("a stale version is a conflict and writes nothing", func(t *testing.T) {
		s := newStore(t)
		g, _ := s.Create(ctx, newGrant(t, "g1", nil))
		stale := g
		if err := g.Revoke(later, admin, "r"); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Update(ctx, g); err != nil {
			t.Fatal(err)
		}
		if err := stale.Lapse(); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Update(ctx, stale); !errors.Is(err, domain.ErrConflict) {
			t.Fatalf("got %v, want ErrConflict", err)
		}
		if got, _ := s.Get(ctx, "g1"); got.Status != domain.GrantRevoked {
			t.Fatalf("status %s", got.Status)
		}
	})

	t.Run("update of an unknown grant", func(t *testing.T) {
		g := newGrant(t, "ghost", nil)
		g.Version = 1
		if _, err := newStore(t).Update(ctx, g); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("got %v", err)
		}
	})

	t.Run("what a human granted is never edited in place", func(t *testing.T) {
		s := newStore(t)
		g, _ := s.Create(ctx, newGrant(t, "g1", nil))
		for name, mut := range map[string]func(*domain.Grant){
			"capabilities": func(g *domain.Grant) { g.Capabilities = []domain.Capability{"git.push"} },
			"resource":     func(g *domain.Grant) { g.Resource.Value = "refs/heads/main" },
			"subject":      func(g *domain.Grant) { g.Subject.ID = "other" },
			"granted by":   func(g *domain.Grant) { g.GrantedBy = "someone" },
			"definition":   func(g *domain.Grant) { g.DefinitionHash = "sha256:zzz" },
		} {
			e := g
			mut(&e)
			if _, err := s.Update(ctx, e); !errors.Is(err, domain.ErrInvalid) {
				t.Errorf("%s: got %v, want ErrInvalid", name, err)
			}
		}
		got, _ := s.Get(ctx, "g1")
		if got.Version != 1 || got.Capabilities[0] != "mcp.call" || got.GrantedBy != "u1" {
			t.Fatalf("a refused edit changed the grant: %+v", got)
		}
	})

	t.Run("a grant out of use cannot be brought back", func(t *testing.T) {
		s := newStore(t)
		g, _ := s.Create(ctx, newGrant(t, "g1", nil))
		if err := g.Revoke(later, admin, "r"); err != nil {
			t.Fatal(err)
		}
		g, err := s.Update(ctx, g)
		if err != nil {
			t.Fatal(err)
		}
		back := g
		back.Status, back.RevokedBy, back.RevokedAt, back.RevokeReason = domain.GrantActive, "", nil, ""
		if _, err := s.Update(ctx, back); !errors.Is(err, domain.ErrInvalidTransition) {
			t.Fatalf("got %v, want ErrInvalidTransition", err)
		}
	})

	t.Run("active lists the organization's and the project's own", func(t *testing.T) {
		s := newStore(t)
		must := func(g domain.Grant) domain.Grant {
			t.Helper()
			out, err := s.Create(ctx, g)
			if err != nil {
				t.Fatal(err)
			}
			return out
		}
		must(newGrant(t, "g-b", func(g *domain.Grant) { g.Level, g.ProjectID = domain.GrantOrganization, "" }))
		must(newGrant(t, "g-a", nil))
		must(newGrant(t, "g-other", func(g *domain.Grant) { g.ProjectID = "p2" }))
		gone := must(newGrant(t, "g-gone", nil))
		if err := gone.Revoke(later, admin, "r"); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Update(ctx, gone); err != nil {
			t.Fatal(err)
		}
		got, err := s.Active(ctx, "p1")
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 2 || got[0].ID != "g-a" || got[1].ID != "g-b" {
			t.Fatalf("got %+v, want g-a then g-b", got)
		}
		if other, _ := s.Active(ctx, "p2"); len(other) != 2 || other[0].ID != "g-b" || other[1].ID != "g-other" {
			t.Fatalf("p2: %+v", other)
		}
	})
}
