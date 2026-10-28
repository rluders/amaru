package domain

import (
	"errors"
	"testing"
	"time"
)

func TestGrantAuditEvent(t *testing.T) {
	made := time.Date(2026, 10, 28, 9, 0, 0, 0, time.UTC)
	now := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	owner := Principal{Kind: PrincipalHuman, ID: "u1", Role: RoleOwner}
	g, err := NewGrant(made, owner, Grant{ID: "g1", Level: GrantProject, ProjectID: "p1",
		Subject: Subject{Kind: SubjectWorkflow, ID: "wf"}, Capabilities: []Capability{"mcp.call"}})
	if err != nil {
		t.Fatal(err)
	}

	ev, err := GrantAuditEvent(nil, g, now)
	if err != nil || ev.Kind != AuditGrantCreated || ev.Actor != "u1" || ev.ActorKind != ActorHuman || !ev.At.Equal(made) || ev.ProjectID != "p1" {
		t.Fatalf("created: %+v, %v", ev, err)
	}

	revoked := g
	if err := revoked.Revoke(now, owner, "left the team"); err != nil {
		t.Fatal(err)
	}
	ev, err = GrantAuditEvent(&g, revoked, made)
	if err != nil || ev.Kind != AuditGrantRevoked || ev.Actor != "u1" || ev.Reason != "left the team" || !ev.At.Equal(now) {
		t.Fatalf("revoked: %+v, %v", ev, err)
	}

	lapsed := g
	if err := lapsed.Lapse(); err != nil {
		t.Fatal(err)
	}
	if ev, err = GrantAuditEvent(&g, lapsed, now); err != nil || ev.Kind != AuditGrantLapsed || ev.ActorKind != ActorPlatform || ev.Reason != AuditReasonDefinitionChanged {
		t.Fatalf("lapsed: %+v, %v", ev, err)
	}
	expired := g
	if err := expired.Expire(); err != nil {
		t.Fatal(err)
	}
	if ev, err = GrantAuditEvent(&g, expired, now); err != nil || ev.Kind != AuditGrantExpired || ev.Reason != AuditReasonExpired {
		t.Fatalf("expired: %+v, %v", ev, err)
	}

	// Not lifecycle steps: a no-op, a move out of a final state, and a new Grant that is not active.
	if _, err := GrantAuditEvent(&g, g, now); !errors.Is(err, ErrInvalid) {
		t.Fatalf("active to active: %v", err)
	}
	if _, err := GrantAuditEvent(&revoked, lapsed, now); !errors.Is(err, ErrInvalid) {
		t.Fatalf("revoked to lapsed: %v", err)
	}
	if _, err := GrantAuditEvent(nil, revoked, now); !errors.Is(err, ErrInvalid) {
		t.Fatalf("new grant already revoked: %v", err)
	}
}
