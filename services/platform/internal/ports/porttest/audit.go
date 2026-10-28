package porttest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
)

// AuditTrail runs the audit-log contract (FEAT-POL-003) against newStore, which must return an
// empty store each call.
func AuditTrail(t *testing.T, newStore func(t *testing.T) ports.GrantAuditStore) {
	ctx := context.Background()
	admin := domain.Principal{Kind: domain.PrincipalHuman, ID: "u2", Role: domain.RoleAdmin}
	later := time.Date(2026, 10, 28, 10, 0, 0, 0, time.UTC)

	t.Run("a grant is created, revoked and each is an event with actor and reason", func(t *testing.T) {
		s := newStore(t)
		g, err := s.Create(ctx, newGrant(t, "g1", nil))
		if err != nil {
			t.Fatal(err)
		}
		if err := g.Revoke(later, admin, "no longer needed"); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Update(ctx, g); err != nil {
			t.Fatal(err)
		}
		evs, err := s.Audit(ctx, 0, 0)
		if err != nil {
			t.Fatal(err)
		}
		if len(evs) != 2 {
			t.Fatalf("got %d events, want 2: %+v", len(evs), evs)
		}
		c, r := evs[0], evs[1]
		if c.Seq != 1 || c.Kind != domain.AuditGrantCreated || c.Actor != "u1" || c.ActorKind != domain.ActorHuman ||
			c.TargetType != "grant" || c.TargetID != "g1" || c.ProjectID != "p1" || !c.At.Equal(g.CreatedAt) {
			t.Fatalf("created event %+v", c)
		}
		if r.Seq != 2 || r.Kind != domain.AuditGrantRevoked || r.Actor != "u2" || r.ActorKind != domain.ActorHuman ||
			r.Reason != "no longer needed" || r.TargetID != "g1" || !r.At.Equal(later) {
			t.Fatalf("revoked event %+v", r)
		}
	})

	t.Run("a lapse and an expiry are the platform's", func(t *testing.T) {
		s := newStore(t)
		a, _ := s.Create(ctx, newGrant(t, "ga", nil))
		b, _ := s.Create(ctx, newGrant(t, "gb", nil))
		if err := a.Lapse(); err != nil {
			t.Fatal(err)
		}
		if err := b.Expire(); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Update(ctx, a); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Update(ctx, b); err != nil {
			t.Fatal(err)
		}
		evs, _ := s.Audit(ctx, 2, 0)
		if len(evs) != 2 || evs[0].Kind != domain.AuditGrantLapsed || evs[1].Kind != domain.AuditGrantExpired {
			t.Fatalf("got %+v", evs)
		}
		for _, ev := range evs {
			if ev.ActorKind != domain.ActorPlatform || ev.Actor != "platform" || ev.Reason == "" || ev.At.IsZero() {
				t.Fatalf("event %+v: want the platform as actor, a reason and a time", ev)
			}
		}
	})

	t.Run("a refused write leaves no event", func(t *testing.T) {
		s := newStore(t)
		g, _ := s.Create(ctx, newGrant(t, "g1", nil))
		if _, err := s.Create(ctx, newGrant(t, "g1", nil)); !errors.Is(err, domain.ErrExists) {
			t.Fatalf("duplicate create: %v", err)
		}
		bad := newGrant(t, "g2", nil)
		bad.Subject.ID = ""
		if _, err := s.Create(ctx, bad); !errors.Is(err, domain.ErrInvalid) {
			t.Fatalf("invalid create: %v", err)
		}
		stale := g
		if err := g.Revoke(later, admin, "why"); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Update(ctx, g); err != nil {
			t.Fatal(err)
		}
		if err := stale.Revoke(later, admin, "again"); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Update(ctx, stale); !errors.Is(err, domain.ErrConflict) {
			t.Fatalf("stale update: %v", err)
		}
		edited := g
		edited.Version = 2
		edited.Capabilities = []domain.Capability{"deploy.run"}
		if _, err := s.Update(ctx, edited); err == nil {
			t.Fatal("editing a grant in place was accepted")
		}
		evs, _ := s.Audit(ctx, 0, 0)
		if len(evs) != 2 {
			t.Fatalf("got %d events, want 2 (created, revoked): %+v", len(evs), evs)
		}
	})

	t.Run("replay from any point and by page", func(t *testing.T) {
		s := newStore(t)
		for _, id := range []string{"g1", "g2", "g3", "g4", "g5"} {
			if _, err := s.Create(ctx, newGrant(t, id, nil)); err != nil {
				t.Fatal(err)
			}
		}
		var seen []int64
		var after int64
		for {
			page, err := s.Audit(ctx, after, 2)
			if err != nil {
				t.Fatal(err)
			}
			if len(page) == 0 {
				break
			}
			if len(page) > 2 {
				t.Fatalf("page of %d, limit 2", len(page))
			}
			for _, ev := range page {
				seen = append(seen, ev.Seq)
				after = ev.Seq
			}
		}
		if len(seen) != 5 {
			t.Fatalf("replayed %v, want seq 1..5", seen)
		}
		for i, seq := range seen {
			if seq != int64(i+1) {
				t.Fatalf("replayed %v: not gapless and ordered", seen)
			}
		}
		if rest, _ := s.Audit(ctx, 3, 0); len(rest) != 2 || rest[0].Seq != 4 {
			t.Fatalf("resume after 3: %+v", rest)
		}
		if none, _ := s.Audit(ctx, 5, 0); len(none) != 0 {
			t.Fatalf("after the last: %+v", none)
		}
	})
}
