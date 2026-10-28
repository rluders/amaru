package domain

import (
	"errors"
	"strings"
	"testing"
)

const (
	trustKey  = ProjectKey("ssh://github.com/acme/app#0123456789abcdef0123456789abcdef01234567")
	trustHash = "sha256:0000000000000000000000000000000000000000000000000000000000000001"
	otherHash = "sha256:0000000000000000000000000000000000000000000000000000000000000002"
)

func TestNewTrustWhoMayTrust(t *testing.T) {
	for _, role := range []Role{RoleOwner, RoleAdmin, RoleMaintainer} {
		if _, err := NewTrust(t0, human(role), trustKey, TrustProject, "", ""); err != nil {
			t.Errorf("%s: %v", role, err)
		}
	}
	for _, role := range []Role{RoleContributor, RoleViewer, ""} {
		if _, err := NewTrust(t0, human(role), trustKey, TrustProject, "", ""); !errors.Is(err, ErrNotPermitted) {
			t.Errorf("%q: got %v, want not permitted", role, err)
		}
	}
	// An Agent, trigger or plugin never trusts, even when it names an Owner role.
	for _, k := range []PrincipalKind{PrincipalAgent, PrincipalTrigger, PrincipalPlugin} {
		by := Principal{Kind: k, ID: "x", Sponsor: "u", Role: RoleOwner}
		if _, err := NewTrust(t0, by, trustKey, TrustProject, "", ""); !errors.Is(err, ErrNotPermitted) {
			t.Errorf("%s: got %v, want not permitted", k, err)
		}
	}
}

func TestTrustShape(t *testing.T) {
	ok := func(kind TrustKind, ref, hash string) error {
		_, err := NewTrust(t0, human(RoleMaintainer), trustKey, kind, ref, hash)
		return err
	}
	if err := ok(TrustWorkflow, "build", trustHash); err != nil {
		t.Errorf("workflow: %v", err)
	}
	if err := ok(TrustSkill, "lint", trustHash); err != nil {
		t.Errorf("skill: %v", err)
	}
	for name, err := range map[string]error{
		"project with a ref":      ok(TrustProject, "x", ""),
		"project with a hash":     ok(TrustProject, "", trustHash),
		"workflow without a ref":  ok(TrustWorkflow, "", trustHash),
		"workflow without a hash": ok(TrustWorkflow, "build", ""),
		"workflow, bad hash":      ok(TrustWorkflow, "build", "abc"),
		"unknown kind":            ok(TrustKind("plugin"), "x", trustHash),
		"ref with space around":   ok(TrustWorkflow, " build", trustHash),
		"ref too long":            ok(TrustWorkflow, strings.Repeat("a", MaxTrustRef+1), trustHash),
	} {
		if !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: got %v, want invalid", name, err)
		}
	}
	if _, err := NewTrust(t0, human(RoleOwner), ProjectKey("/home/me/app"), TrustProject, "", ""); !errors.Is(err, ErrInvalid) {
		t.Errorf("a path is not a project key: %v", err)
	}
}

func TestTrustRevoke(t *testing.T) {
	tr, _ := NewTrust(t0, human(RoleMaintainer), trustKey, TrustProject, "", "")
	if err := tr.Revoke(t0, human(RoleViewer), "x"); !errors.Is(err, ErrNotPermitted) {
		t.Errorf("viewer: %v", err)
	}
	if err := tr.Revoke(t0, Principal{Kind: PrincipalAgent, ID: "a", Sponsor: "u", Role: RoleOwner}, "x"); !errors.Is(err, ErrNotPermitted) {
		t.Errorf("agent: %v", err)
	}
	if err := tr.Revoke(t0, human(RoleMaintainer), "  "); !errors.Is(err, ErrReasonRequired) {
		t.Errorf("no reason: %v", err)
	}
	if tr.RevokedAt != nil {
		t.Fatal("a refused revoke changed the trust")
	}
	if err := tr.Revoke(t0, human(RoleMaintainer), "left the team"); err != nil {
		t.Fatal(err)
	}
	if tr.RevokedAt == nil || tr.RevokeReason != "left the team" || tr.Validate() != nil {
		t.Fatalf("%+v", tr)
	}
	if err := tr.Revoke(t0, human(RoleMaintainer), "again"); !errors.Is(err, ErrInvalidTransition) {
		t.Errorf("twice: %v", err)
	}
}

func TestStanding(t *testing.T) {
	project, _ := NewTrust(t0, human(RoleMaintainer), trustKey, TrustProject, "", "")
	wf, _ := NewTrust(t0, human(RoleMaintainer), trustKey, TrustWorkflow, "build", trustHash)
	revoked := wf
	_ = revoked.Revoke(t0, human(RoleMaintainer), "x")
	noProject := Trust{}
	revokedProject := project
	_ = revokedProject.Revoke(t0, human(RoleMaintainer), "x")
	cases := []struct {
		name    string
		project Trust
		content *Trust
		hash    string
		want    TrustStanding
	}{
		{"no trust at all", noProject, nil, "", TrustUntrusted},
		{"project trusted, runs no named content", project, nil, "", TrustTrusted},
		{"project revoked", revokedProject, &wf, trustHash, TrustUntrusted},
		{"content at the trusted hash", project, &wf, trustHash, TrustTrusted},
		{"content changed", project, &wf, otherHash, TrustChanged},
		{"content never trusted", project, nil, trustHash, TrustChanged},
		{"content trust revoked", project, &revoked, trustHash, TrustChanged},
		{"content trusted, project is not", noProject, &wf, trustHash, TrustUntrusted},
	}
	for _, c := range cases {
		if got := Standing(c.project, c.content, c.hash); got != c.want {
			t.Errorf("%s: got %s, want %s", c.name, got, c.want)
		}
	}
}

func TestTrustAuditEvent(t *testing.T) {
	wf, _ := NewTrust(t0, human(RoleMaintainer), trustKey, TrustWorkflow, "build", trustHash)
	ev, err := TrustAuditEvent(nil, wf)
	if err != nil {
		t.Fatal(err)
	}
	if ev.Kind != AuditTrustGranted || ev.Actor != "u-maintainer" || ev.ActorKind != ActorHuman || ev.TargetType != "project_trust" ||
		ev.TargetID != "workflow:build" || ev.ProjectID != string(trustKey) || !ev.At.Equal(t0) {
		t.Fatalf("%+v", ev)
	}
	rev := wf
	_ = rev.Revoke(t0, human(RoleAdmin), "why")
	ev, err = TrustAuditEvent(&wf, rev)
	if err != nil || ev.Kind != AuditTrustRevoked || ev.Reason != "why" {
		t.Fatalf("%+v %v", ev, err)
	}
	if _, err := TrustAuditEvent(nil, rev); !errors.Is(err, ErrInvalid) {
		t.Errorf("revoking nothing: %v", err)
	}
	if _, err := TrustAuditEvent(&rev, rev); !errors.Is(err, ErrInvalid) {
		t.Errorf("revoking twice: %v", err)
	}
}

func TestUntrustedProjectIsClosedToEveryRequest(t *testing.T) {
	allow := PolicyRule{ID: "o.allow", Source: SourceOrganization, Effect: EffectAllow, Capabilities: []string{"*"}}
	for _, standing := range []TrustStanding{"", TrustUntrusted, TrustChanged} {
		r := req("mcp.call")
		r.ProjectTrust = standing
		// An allow rule and a covering Grant do not lift it.
		g := grant(t, "g1", nil)
		res := eval(t, r, []Grant{g}, []PolicyRule{allow})
		want := ReasonProjectUntrusted
		if standing == TrustChanged {
			want = ReasonTrustChanged
		}
		if res.Decision != DecisionDeny || res.Reasons[0].Code != want {
			t.Errorf("%q: got %s %+v, want deny %s", standing, res.Decision, res.Reasons, want)
		}
		for _, k := range []PrincipalKind{PrincipalAgent, PrincipalHuman, PrincipalTrigger, PrincipalPlugin} {
			rr := r
			rr.Principal = Principal{Kind: k, ID: "x", Sponsor: "u"}
			if res := eval(t, rr, nil, []PolicyRule{allow}); res.Decision != DecisionDeny {
				t.Errorf("%q %s: got %s", standing, k, res.Decision)
			}
		}
	}
	// A request outside any Project is not a Project's content.
	r := req("mcp.call")
	r.Scope, r.ProjectTrust = PolicyScope{}, ""
	if res := eval(t, r, nil, []PolicyRule{allow}); res.Decision != DecisionAllow {
		t.Errorf("no project: %s", res.Decision)
	}
}
