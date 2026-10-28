package domain

import (
	"errors"
	"testing"
	"time"
)

func TestNewGrantAuthority(t *testing.T) {
	base := Grant{ID: "g", Level: GrantOrganization, Subject: Subject{Kind: SubjectAgent, ID: "a"}, Capabilities: []Capability{"mcp.call"}}
	for _, role := range []Role{RoleOwner, RoleAdmin} {
		g, err := NewGrant(t0, human(role), base)
		if err != nil || g.Status != GrantActive || g.GrantedBy != "u-"+string(role) || !g.CreatedAt.Equal(t0) {
			t.Fatalf("%s: %+v %v", role, g, err)
		}
	}
	for _, role := range []Role{RoleMaintainer, RoleContributor, RoleViewer, ""} {
		if _, err := NewGrant(t0, human(role), base); !errors.Is(err, ErrNotPermitted) {
			t.Errorf("role %q: %v", role, err)
		}
	}
	for _, k := range []PrincipalKind{PrincipalAgent, PrincipalTrigger, PrincipalPlugin} {
		by := Principal{Kind: k, ID: "x", Sponsor: "u", Role: RoleOwner}
		if _, err := NewGrant(t0, by, base); !errors.Is(err, ErrNotPermitted) {
			t.Errorf("%s granted: %v", k, err)
		}
	}
}

func TestNewGrantIgnoresCallerSuppliedRecordFields(t *testing.T) {
	now := t0
	g, err := NewGrant(t0, human(RoleOwner), Grant{
		ID: "g", Level: GrantOrganization, Subject: Subject{Kind: SubjectAgent, ID: "a"}, Capabilities: []Capability{"mcp.call"},
		GrantedBy: "someone-else", Status: GrantRevoked, RevokedBy: "x", RevokedAt: &now, Version: 9,
	})
	if err != nil || g.GrantedBy != "u-owner" || g.Status != GrantActive || g.RevokedBy != "" || g.Version != 0 {
		t.Fatalf("%+v %v", g, err)
	}
}

func TestGrantValidate(t *testing.T) {
	ok := Grant{ID: "g", Level: GrantProject, ProjectID: "p", Subject: Subject{Kind: SubjectWorkflow, ID: "w"}, Capabilities: []Capability{"mcp.call"}}
	exp := t0
	for name, mut := range map[string]func(*Grant){
		"no id":                  func(g *Grant) { g.ID = "" },
		"bad level":              func(g *Grant) { g.Level = "user" },
		"project grant no proj":  func(g *Grant) { g.ProjectID = "" },
		"org grant with project": func(g *Grant) { g.Level = GrantOrganization },
		"no subject":             func(g *Grant) { g.Subject = Subject{} },
		"no capabilities":        func(g *Grant) { g.Capabilities = nil },
		"wildcard capability":    func(g *Grant) { g.Capabilities = []Capability{"mcp.*"} },
		"invalid capability":     func(g *Grant) { g.Capabilities = []Capability{"Bad"} },
		"pattern without type":   func(g *Grant) { g.Resource = Resource{Value: "x"} },
		"expiry not after":       func(g *Grant) { g.ExpiresAt = &exp },
	} {
		g := ok
		mut(&g)
		if _, err := NewGrant(t0, human(RoleOwner), g); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestGrantRevoke(t *testing.T) {
	g := grant(t, "g", nil)
	later := t0.Add(time.Hour)
	for name, c := range map[string]struct {
		by     Principal
		reason string
		want   error
	}{
		"maintainer": {human(RoleMaintainer), "why", ErrNotPermitted},
		"agent":      {Principal{Kind: PrincipalAgent, ID: "a", Sponsor: "u", Role: RoleOwner}, "why", ErrNotPermitted},
		"no reason":  {human(RoleAdmin), "  ", ErrReasonRequired},
	} {
		g2 := g
		if err := g2.Revoke(later, c.by, c.reason); !errors.Is(err, c.want) {
			t.Errorf("%s: %v", name, err)
		}
		if g2.Status != GrantActive {
			t.Errorf("%s: a refused revoke changed the grant", name)
		}
	}
	if err := g.Revoke(later, human(RoleAdmin), "contract ended"); err != nil {
		t.Fatal(err)
	}
	if g.Status != GrantRevoked || g.RevokedBy != "u-admin" || g.RevokedAt == nil || !g.RevokedAt.Equal(later) || g.RevokeReason != "contract ended" {
		t.Fatalf("%+v", g)
	}
	if err := g.Validate(); err != nil {
		t.Fatal(err)
	}
	if err := g.Revoke(later, human(RoleAdmin), "again"); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("second revoke: %v", err)
	}
}

func TestGrantLapseAndExpireOnlyFromActive(t *testing.T) {
	g := grant(t, "g", nil)
	if err := g.Expire(); err != nil || g.Status != GrantExpired {
		t.Fatal(err, g.Status)
	}
	if err := g.Lapse(); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("lapse from expired: %v", err)
	}
}

func TestMatchPattern(t *testing.T) {
	for _, c := range []struct {
		p, v string
		want bool
	}{
		{"*", "anything", true}, {"refs/heads/amaru/*", "refs/heads/amaru/x", true},
		{"refs/heads/amaru/*", "refs/heads/main", false}, {"exact", "exact", true},
		{"exact", "exactly", false}, {"", "", true}, {"", "x", false},
	} {
		if got := matchPattern(c.p, c.v); got != c.want {
			t.Errorf("matchPattern(%q,%q)=%v", c.p, c.v, got)
		}
	}
}
