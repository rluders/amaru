package porttest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
)

// TrustStore runs the Trust contract (FEAT-PRJ-003) against newStore, which returns an empty Trust
// store and the Project store it shares a database with, each call. Trust refers to a Project, so
// the suite creates the Project first.
func TrustStore(t *testing.T, newStore func(t *testing.T) (ports.TrustStore, ports.ProjectStore)) {
	ctx := context.Background()
	now := time.Date(2026, 10, 28, 9, 0, 0, 0, time.UTC)
	later := now.Add(time.Hour)
	key := domain.ProjectKey("ssh://github.com/acme/app#0123456789abcdef0123456789abcdef01234567")
	maint := domain.Principal{Kind: domain.PrincipalHuman, ID: "maint-1", Role: domain.RoleMaintainer}
	admin := domain.Principal{Kind: domain.PrincipalHuman, ID: "admin-1", Role: domain.RoleAdmin}
	h1 := "sha256:" + string(repeat('1', 64))
	h2 := "sha256:" + string(repeat('2', 64))

	open := func(t *testing.T) ports.TrustStore {
		t.Helper()
		s, p := newStore(t)
		if err := p.Create(ctx, domain.Project{Key: key, Name: "app", CreatedAt: now}); err != nil {
			t.Fatal(err)
		}
		return s
	}
	trust := func(t *testing.T, by domain.Principal, at time.Time, kind domain.TrustKind, ref, hash string) domain.Trust {
		t.Helper()
		tr, err := domain.NewTrust(at, by, key, kind, ref, hash)
		if err != nil {
			t.Fatal(err)
		}
		return tr
	}

	t.Run("a trust is stored and read back whole", func(t *testing.T) {
		s := open(t)
		want := trust(t, maint, now, domain.TrustWorkflow, "build", h1)
		if err := s.Save(ctx, want); err != nil {
			t.Fatal(err)
		}
		got, err := s.Get(ctx, key, domain.TrustWorkflow, "build")
		if err != nil || got.Hash != h1 || got.TrustedBy != "maint-1" || !got.TrustedAt.Equal(now) || got.RevokedAt != nil {
			t.Fatalf("%+v %v", got, err)
		}
		if _, err := s.Get(ctx, key, domain.TrustProject, ""); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("missing: %v", err)
		}
	})

	t.Run("trusting again at a new hash replaces the old trust, and each is an event", func(t *testing.T) {
		s := open(t)
		_ = s.Save(ctx, trust(t, maint, now, domain.TrustWorkflow, "build", h1))
		if err := s.Save(ctx, trust(t, admin, later, domain.TrustWorkflow, "build", h2)); err != nil {
			t.Fatal(err)
		}
		got, _ := s.Get(ctx, key, domain.TrustWorkflow, "build")
		if got.Hash != h2 || got.TrustedBy != "admin-1" {
			t.Fatalf("%+v", got)
		}
		evs, err := s.Audit(ctx, 0, 0)
		if err != nil || len(evs) != 2 || evs[0].Kind != domain.AuditTrustGranted || evs[1].Kind != domain.AuditTrustGranted ||
			evs[1].Actor != "admin-1" || evs[1].TargetID != "workflow:build" || evs[1].ProjectID != string(key) {
			t.Fatalf("%+v %v", evs, err)
		}
	})

	t.Run("a revoked trust stays, with who and why, and can be trusted again", func(t *testing.T) {
		s := open(t)
		tr := trust(t, maint, now, domain.TrustProject, "", "")
		_ = s.Save(ctx, tr)
		if err := tr.Revoke(later, admin, "left the team"); err != nil {
			t.Fatal(err)
		}
		if err := s.Save(ctx, tr); err != nil {
			t.Fatal(err)
		}
		got, err := s.Get(ctx, key, domain.TrustProject, "")
		if err != nil || got.RevokedAt == nil || got.RevokedBy != "admin-1" || got.RevokeReason != "left the team" {
			t.Fatalf("%+v %v", got, err)
		}
		if err := s.Save(ctx, trust(t, maint, later.Add(time.Hour), domain.TrustProject, "", "")); err != nil {
			t.Fatal(err)
		}
		got, _ = s.Get(ctx, key, domain.TrustProject, "")
		if got.RevokedAt != nil || got.RevokedBy != "" || got.RevokeReason != "" {
			t.Fatalf("trusting again left the revoke: %+v", got)
		}
		evs, _ := s.Audit(ctx, 0, 0)
		if len(evs) != 3 || evs[1].Kind != domain.AuditTrustRevoked || evs[1].Reason != "left the team" || evs[1].Actor != "admin-1" {
			t.Fatalf("%+v", evs)
		}
	})

	t.Run("a revoke with no trust in force is refused and writes nothing", func(t *testing.T) {
		s := open(t)
		tr := trust(t, maint, now, domain.TrustProject, "", "")
		_ = tr.Revoke(later, admin, "x")
		if err := s.Save(ctx, tr); !errors.Is(err, domain.ErrInvalid) {
			t.Fatalf("revoke of nothing: %v", err)
		}
		fresh := trust(t, maint, now, domain.TrustProject, "", "")
		_ = s.Save(ctx, fresh)
		_ = fresh.Revoke(later, admin, "x")
		_ = s.Save(ctx, fresh)
		if err := s.Save(ctx, fresh); !errors.Is(err, domain.ErrInvalid) {
			t.Fatalf("revoke twice: %v", err)
		}
		if evs, _ := s.Audit(ctx, 0, 0); len(evs) != 2 {
			t.Fatalf("%d events, want 2: %+v", len(evs), evs)
		}
	})

	t.Run("an invalid trust and an unknown project are refused", func(t *testing.T) {
		s := open(t)
		bad := trust(t, maint, now, domain.TrustProject, "", "")
		bad.Kind = "plugin"
		if err := s.Save(ctx, bad); !errors.Is(err, domain.ErrInvalid) {
			t.Errorf("invalid: %v", err)
		}
		other := domain.ProjectKey("ssh://github.com/acme/other#0123456789abcdef0123456789abcdef01234567")
		tr, _ := domain.NewTrust(now, maint, other, domain.TrustProject, "", "")
		if err := s.Save(ctx, tr); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("unknown project: %v", err)
		}
	})

	t.Run("a Project's trusts are listed in order and kept apart from another Project's", func(t *testing.T) {
		s := open(t)
		_ = s.Save(ctx, trust(t, maint, now, domain.TrustWorkflow, "b", h1))
		_ = s.Save(ctx, trust(t, maint, now, domain.TrustProject, "", ""))
		_ = s.Save(ctx, trust(t, maint, now, domain.TrustWorkflow, "a", h1))
		got, err := s.List(ctx, key)
		if err != nil || len(got) != 3 || got[0].Kind != domain.TrustProject || got[1].Ref != "a" || got[2].Ref != "b" {
			t.Fatalf("%+v %v", got, err)
		}
		if none, _ := s.List(ctx, "id:01ARZ3NDEKTSV4RRFFQ69G5FAV"); len(none) != 0 {
			t.Errorf("other project: %+v", none)
		}
	})
}

func repeat(b byte, n int) []byte {
	out := make([]byte, n)
	for i := range out {
		out[i] = b
	}
	return out
}
