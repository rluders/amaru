package domain

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

func human(role Role) Principal {
	return Principal{Kind: PrincipalHuman, ID: "u-" + string(role), Role: role}
}

func req(c Capability) PolicyRequest {
	return PolicyRequest{
		Principal: Principal{Kind: PrincipalTrigger, ID: "t1", Sponsor: "u-owner"},
		Subject:   Subject{Kind: SubjectWorkflow, ID: "wf"}, Capability: c,
		Scope: PolicyScope{ProjectID: "p1"}, Trigger: TriggerManual, DefinitionHash: "sha256:a",
	}
}

func grant(t *testing.T, id string, mod func(*Grant)) Grant {
	t.Helper()
	g := Grant{ID: id, Level: GrantProject, ProjectID: "p1", Subject: Subject{Kind: SubjectWorkflow, ID: "wf"}, Capabilities: []Capability{"mcp.call"}}
	if mod != nil {
		mod(&g)
	}
	out, err := NewGrant(t0, human(RoleAdmin), g)
	if err != nil {
		t.Fatalf("NewGrant: %v", err)
	}
	return out
}

func eval(t *testing.T, r PolicyRequest, gs []Grant, rs []PolicyRule) PolicyResult {
	t.Helper()
	res, err := Evaluate(t0, r, gs, rs)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	return res
}

func TestEvaluateResolutionOrder(t *testing.T) {
	deny := PolicyRule{ID: "d", Source: SourceOrganization, Effect: EffectDeny, Capabilities: []string{"mcp.*"}}
	appr := PolicyRule{ID: "a", Source: SourceOrganization, Effect: EffectRequireApproval, Capabilities: []string{"mcp.call"}}
	allow := PolicyRule{ID: "l", Source: SourceDefault, Effect: EffectAllow, Capabilities: []string{"mcp.call"}}
	g := grant(t, "g1", nil)
	cases := []struct {
		name   string
		grants []Grant
		rules  []PolicyRule
		want   Decision
		code   string
	}{
		{"nothing covers it", nil, nil, DecisionDeny, ReasonUncovered},
		{"deny beats a grant", []Grant{g}, []PolicyRule{deny}, DecisionDeny, ReasonExplicitDeny},
		{"deny beats allow and approval", nil, []PolicyRule{allow, appr, deny}, DecisionDeny, ReasonExplicitDeny},
		{"grant allows", []Grant{g}, nil, DecisionAllow, ReasonGrantMatched},
		{"grant lowers an approval rule", []Grant{g}, []PolicyRule{appr}, DecisionAllow, ReasonGrantMatched},
		{"approval rule needs an approval", nil, []PolicyRule{appr}, DecisionRequireApproval, ReasonRuleRequiresAppr},
		{"approval rule beats allow rule", nil, []PolicyRule{allow, appr}, DecisionRequireApproval, ReasonRuleRequiresAppr},
		{"allow rule allows", nil, []PolicyRule{allow}, DecisionAllow, ReasonRuleAllow},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			res := eval(t, req("mcp.call"), c.grants, c.rules)
			if res.Decision != c.want || res.Reasons[0].Code != c.code {
				t.Fatalf("got %s %+v, want %s %s", res.Decision, res.Reasons, c.want, c.code)
			}
		})
	}
}

func TestEvaluateUncoveredIsActionable(t *testing.T) {
	r := req("asset.upload")
	res := eval(t, r, nil, DefaultRules())
	if res.Decision != DecisionDeny || res.Reasons[0].Code != ReasonUncovered {
		t.Fatalf("got %+v", res)
	}
	msg := res.Explain(r)
	for _, want := range []string{ReasonUncovered, "asset.upload", "Owner or Admin", "workflow wf"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("message %q lacks %q", msg, want)
		}
	}
}

func TestDefaultRules(t *testing.T) {
	for _, c := range []Capability{"agent.run", "check.run", "git.commit", "code.test"} {
		if d := eval(t, req(c), nil, DefaultRules()).Decision; d != DecisionAllow {
			t.Errorf("%s: %s, want allow", c, d)
		}
	}
	for _, c := range []Capability{"mcp.call", "asset.upload", "unknown.thing"} {
		if d := eval(t, req(c), nil, DefaultRules()).Decision; d != DecisionDeny {
			t.Errorf("%s: %s, want deny (uncovered)", c, d)
		}
	}
	res := eval(t, req("scm.pr.merge"), []Grant{grant(t, "g", func(g *Grant) { g.Capabilities = []Capability{"scm.pr.merge"} })}, DefaultRules())
	if res.Decision != DecisionDeny || res.Reasons[0].Code != ReasonExplicitDeny {
		t.Fatalf("a grant must not lift the merge deny: %+v", res)
	}
	for _, r := range DefaultRules() {
		if err := r.Validate(); err != nil {
			t.Errorf("default rule %s: %v", r.ID, err)
		}
	}
}

func TestGrantScopeAndBinding(t *testing.T) {
	r := req("mcp.call")
	t.Run("a project grant never matches another project", func(t *testing.T) {
		other := r
		other.Scope.ProjectID = "p2"
		if d := eval(t, other, []Grant{grant(t, "g", nil)}, nil).Decision; d != DecisionDeny {
			t.Fatalf("got %s", d)
		}
	})
	t.Run("an organization grant matches every project", func(t *testing.T) {
		g := grant(t, "g", func(g *Grant) { g.Level, g.ProjectID = GrantOrganization, "" })
		other := r
		other.Scope.ProjectID = "p2"
		if d := eval(t, other, []Grant{g}, nil).Decision; d != DecisionAllow {
			t.Fatalf("got %s", d)
		}
	})
	t.Run("a different subject is not covered", func(t *testing.T) {
		other := r
		other.Subject = Subject{Kind: SubjectAgent, ID: "wf"}
		if d := eval(t, other, []Grant{grant(t, "g", nil)}, nil).Decision; d != DecisionDeny {
			t.Fatalf("got %s", d)
		}
	})
	t.Run("a request with no subject is covered by no grant", func(t *testing.T) {
		none := r
		none.Subject = Subject{}
		if d := eval(t, none, []Grant{grant(t, "g", nil)}, nil).Decision; d != DecisionDeny {
			t.Fatalf("got %s", d)
		}
	})
	t.Run("a definition-bound grant lapses when the definition changes", func(t *testing.T) {
		g := grant(t, "g", func(g *Grant) { g.DefinitionHash = "sha256:a" })
		if d := eval(t, r, []Grant{g}, nil).Decision; d != DecisionAllow {
			t.Fatalf("same definition: %s", d)
		}
		changed := r
		changed.DefinitionHash = "sha256:b"
		if d := eval(t, changed, []Grant{g}, nil).Decision; d != DecisionDeny {
			t.Fatalf("changed definition: %s", d)
		}
		if got := g.Standing(t0, "sha256:b"); got != GrantLapsed {
			t.Fatalf("standing %s, want lapsed", got)
		}
	})
	t.Run("an unbound grant ignores the definition", func(t *testing.T) {
		changed := r
		changed.DefinitionHash = "sha256:zzz"
		if d := eval(t, changed, []Grant{grant(t, "g", nil)}, nil).Decision; d != DecisionAllow {
			t.Fatalf("got %s", d)
		}
	})
	t.Run("a resource-limited grant matches by prefix", func(t *testing.T) {
		g := grant(t, "g", func(g *Grant) {
			g.Capabilities = []Capability{"git.push"}
			g.Resource = Resource{Type: "git_ref", Value: "refs/heads/amaru/*"}
		})
		push := req("git.push")
		push.Resource = Resource{Type: "git_ref", Value: "refs/heads/amaru/feat-1"}
		if d := eval(t, push, []Grant{g}, nil).Decision; d != DecisionAllow {
			t.Fatalf("inside: %s", d)
		}
		push.Resource.Value = "refs/heads/main"
		if d := eval(t, push, []Grant{g}, nil).Decision; d != DecisionDeny {
			t.Fatalf("outside: %s", d)
		}
		push.Resource = Resource{}
		if d := eval(t, push, []Grant{g}, nil).Decision; d != DecisionDeny {
			t.Fatalf("no resource: %s", d)
		}
	})
}

func TestRevokedExpiredLapsedGrantsDoNotMatch(t *testing.T) {
	r := req("mcp.call")
	revoked := grant(t, "g1", nil)
	if err := revoked.Revoke(t0, human(RoleOwner), "no longer needed"); err != nil {
		t.Fatal(err)
	}
	exp := t0.Add(time.Hour)
	expiring := grant(t, "g2", func(g *Grant) { g.ExpiresAt = &exp })
	lapsed := grant(t, "g3", nil)
	if err := lapsed.Lapse(); err != nil {
		t.Fatal(err)
	}
	for _, g := range []Grant{revoked, lapsed} {
		if d := eval(t, r, []Grant{g}, nil).Decision; d != DecisionDeny {
			t.Errorf("%s grant matched: %s", g.Status, d)
		}
	}
	if d := eval(t, r, []Grant{expiring}, nil).Decision; d != DecisionAllow {
		t.Fatalf("before expiry: %s", d)
	}
	res, err := Evaluate(exp, r, []Grant{expiring}, nil)
	if err != nil || res.Decision != DecisionDeny {
		t.Fatalf("at expiry: %+v %v", res, err)
	}
}

func TestEvaluateIsDeterministicAndOrderIndependentForGrants(t *testing.T) {
	r := req("mcp.call")
	a, b := grant(t, "g-a", nil), grant(t, "g-b", nil)
	one := eval(t, r, []Grant{a, b}, DefaultRules())
	two := eval(t, r, []Grant{b, a}, DefaultRules())
	if !reflect.DeepEqual(one, two) || one.MatchedGrantID != "g-a" {
		t.Fatalf("not order independent: %+v vs %+v", one, two)
	}
	if again := eval(t, r, []Grant{a, b}, DefaultRules()); !reflect.DeepEqual(one, again) {
		t.Fatal("not deterministic")
	}
}

// Properties from POLICY_AND_APPROVAL_SHAPES §4.2: total, and monotone in denies.
func TestEvaluateTotalAndMonotoneInDenies(t *testing.T) {
	caps := []Capability{"agent.run", "mcp.call", "git.push", "scm.pr.merge", "asset.upload"}
	extra := []PolicyRule{
		{ID: "x1", Source: SourceOrganization, Effect: EffectRequireApproval, Capabilities: []string{"git.push"}},
		{ID: "x2", Source: SourceOrganization, Effect: EffectAllow, Capabilities: []string{"asset.*"}},
	}
	gs := []Grant{grant(t, "g", func(g *Grant) { g.Capabilities = []Capability{"mcp.call", "git.push"} })}
	rank := map[Decision]int{DecisionAllow: 0, DecisionRequireApproval: 1, DecisionDeny: 2}
	for _, c := range caps {
		base := eval(t, req(c), gs, append(DefaultRules(), extra...))
		if _, ok := rank[base.Decision]; !ok || len(base.Reasons) == 0 {
			t.Fatalf("%s: not total: %+v", c, base)
		}
		withDeny := eval(t, req(c), gs, append(append(DefaultRules(), extra...), PolicyRule{ID: "dd", Source: SourceOrganization, Effect: EffectDeny, Capabilities: []string{string(c)}}))
		if withDeny.Decision != DecisionDeny {
			t.Errorf("%s: a matching deny gave %s", c, withDeny.Decision)
		}
		other := eval(t, req(c), gs, append(append(DefaultRules(), extra...), PolicyRule{ID: "dd", Source: SourceOrganization, Effect: EffectDeny, Capabilities: []string{"unrelated.cap"}}))
		if other.Decision != base.Decision {
			t.Errorf("%s: an unrelated deny changed %s to %s", c, base.Decision, other.Decision)
		}
	}
}

func TestEvaluateRejectsInvalidInput(t *testing.T) {
	bad := []struct {
		name string
		mut  func(*PolicyRequest)
	}{
		{"capability", func(r *PolicyRequest) { r.Capability = "NoDot" }},
		{"no principal id", func(r *PolicyRequest) { r.Principal.ID = "" }},
		{"trigger without sponsor", func(r *PolicyRequest) { r.Principal.Sponsor = "" }},
		{"unknown trigger", func(r *PolicyRequest) { r.Trigger = "cron" }},
		{"unknown principal", func(r *PolicyRequest) { r.Principal.Kind = "robot" }},
		{"half subject", func(r *PolicyRequest) { r.Subject = Subject{Kind: SubjectAgent} }},
	}
	for _, c := range bad {
		t.Run(c.name, func(t *testing.T) {
			r := req("agent.run")
			c.mut(&r)
			if _, err := Evaluate(t0, r, nil, nil); !errors.Is(err, ErrInvalid) {
				t.Fatalf("got %v, want ErrInvalid", err)
			}
		})
	}
	if _, err := Evaluate(t0, req("agent.run"), nil, []PolicyRule{{ID: "p", Source: SourceProject, Effect: EffectAllow, Capabilities: []string{"x.y"}}}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("a project allow rule: %v", err)
	}
}

func TestPolicyRuleValidate(t *testing.T) {
	ok := PolicyRule{ID: "r", Source: SourceProject, Effect: EffectRequireApproval, Capabilities: []string{"git.push"}}
	if err := ok.Validate(); err != nil {
		t.Fatal(err)
	}
	for name, mut := range map[string]func(*PolicyRule){
		"no id":            func(r *PolicyRule) { r.ID = "" },
		"no capabilities":  func(r *PolicyRule) { r.Capabilities = nil },
		"unknown effect":   func(r *PolicyRule) { r.Effect = "maybe" },
		"unknown source":   func(r *PolicyRule) { r.Source = "user" },
		"match without ty": func(r *PolicyRule) { r.ResourceMatch = "x" },
	} {
		r := ok
		mut(&r)
		if err := r.Validate(); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// consequential is rule 12 and the ADR-0006 list: what each action is, without a Grant.
var consequential = []struct {
	capability Capability
	resource   Resource
	class      ActionClass
}{
	{"git.push", Resource{Type: "git_ref", Value: "refs/heads/amaru/x"}, ClassPublish},
	{"deploy.run", Resource{Type: "deploy_target", Value: "staging"}, ClassDeploy},
	{"tracker.write", Resource{Type: "tracker_item", Value: "ISSUE-1"}, ClassExternalWrite},
	{"scm.pr.create", Resource{Type: "repository", Value: "origin"}, ClassExternalWrite},
	{"net.egress", Resource{Type: "host", Value: "api.example.com"}, ClassEgress},
	{"fs.delete", Resource{Type: ResourcePath, Value: "/home/u/other"}, ClassDelete},
	{"plugin.install", Resource{Type: "plugin", Value: "acme/pandoc@1.2.0"}, ClassPermission},
	{"plugin.upgrade", Resource{Type: "plugin", Value: "acme/pandoc@1.3.0"}, ClassPermission},
	{"spend.paid", Resource{Type: "service", Value: "render-farm"}, ClassSpend},
}

func TestConsequentialActionsNeedAnApprovalOrAGrant(t *testing.T) {
	for _, c := range consequential {
		r := req(c.capability)
		r.Resource = c.resource
		res := eval(t, r, nil, DefaultRules())
		if res.Decision != DecisionRequireApproval || res.Reasons[0].Code != ReasonRuleRequiresAppr || res.Reasons[0].Class != c.class || res.Reasons[0].Source != "default" {
			t.Errorf("%s: %+v, want require_approval of class %s from a default", c.capability, res, c.class)
		}
		// An Owner's or Admin's Grant lowers the default, and only for what it names.
		g := grant(t, "g", func(g *Grant) { g.Capabilities = []Capability{c.capability} })
		if got := eval(t, r, []Grant{g}, DefaultRules()); got.Decision != DecisionAllow || got.MatchedGrantID != "g" {
			t.Errorf("%s with a grant: %+v", c.capability, got)
		}
		other := r
		other.Capability = "mcp.call"
		if got := eval(t, other, []Grant{g}, DefaultRules()); got.Decision != DecisionDeny {
			t.Errorf("a grant for %s covered mcp.call: %+v", c.capability, got)
		}
	}
}

func TestConsequentialActionsAreTheSameForEveryTriggerAndPrincipal(t *testing.T) {
	principals := []Principal{
		human(RoleOwner), human(RoleContributor),
		{Kind: PrincipalAgent, ID: "a", Sponsor: "u-owner"}, {Kind: PrincipalTrigger, ID: "t", Sponsor: "u-owner"}, {Kind: PrincipalPlugin, ID: "p", Sponsor: "u-owner"},
	}
	for _, c := range consequential {
		for _, trig := range []string{TriggerManual, TriggerWebhook, TriggerSchedule, TriggerDeliveryLoop} {
			for _, p := range principals {
				r := req(c.capability)
				r.Resource, r.Trigger, r.Principal = c.resource, trig, p
				if d := eval(t, r, nil, DefaultRules()).Decision; d != DecisionRequireApproval {
					t.Errorf("%s by %s/%s via %s: %s, want require_approval", c.capability, p.Kind, p.Role, trig, d)
				}
			}
		}
	}
}

func TestDeleteIsAllowedOnlyInsideTheWorktree(t *testing.T) {
	for _, c := range []struct {
		name string
		res  Resource
		want Decision
	}{
		{"inside the worktree", Resource{Type: ResourceWorktreePath, Value: "build/out.o"}, DecisionAllow},
		{"outside the worktree", Resource{Type: ResourcePath, Value: "/etc/hosts"}, DecisionRequireApproval},
		{"no resource named", Resource{}, DecisionDeny},
		{"unknown resource type", Resource{Type: "bucket", Value: "x"}, DecisionDeny},
	} {
		r := req("fs.delete")
		r.Resource = c.res
		if got := eval(t, r, nil, DefaultRules()).Decision; got != c.want {
			t.Errorf("%s: %s, want %s", c.name, got, c.want)
		}
	}
}

func TestNoOneBelowTheOrganizationLowersADefault(t *testing.T) {
	r := req("git.push")
	r.Resource = Resource{Type: "git_ref", Value: "refs/heads/amaru/x"}
	// A Project rule that allows is invalid, so it never reaches the function.
	loosen := PolicyRule{ID: "proj.push", Source: SourceProject, Effect: EffectAllow, Capabilities: []string{"git.push"}}
	if _, err := Evaluate(t0, r, nil, append(DefaultRules(), loosen)); !errors.Is(err, ErrInvalid) {
		t.Fatalf("a project allow: %v", err)
	}
	// A Project rule may tighten: deny, or require an approval for what the defaults allow.
	tighten := PolicyRule{ID: "proj.test", Source: SourceProject, Effect: EffectRequireApproval, Capabilities: []string{"code.test"}}
	if d := eval(t, req("code.test"), nil, append(DefaultRules(), tighten)).Decision; d != DecisionRequireApproval {
		t.Fatalf("a project requirement: %s", d)
	}
	// An allow rule, even the Organization's, does not lower a default requirement: only a Grant does.
	if d := eval(t, r, nil, append(DefaultRules(), PolicyRule{ID: "org.push", Source: SourceOrganization, Effect: EffectAllow, Capabilities: []string{"git.push"}})).Decision; d != DecisionRequireApproval {
		t.Fatalf("an allow must not beat the stricter default requirement: %s", d)
	}
	// The merge Capability stays denied for every principal, with or without a Grant.
	m := req("scm.pr.merge")
	g := grant(t, "g", func(g *Grant) {
		g.Capabilities = []Capability{"scm.pr.merge"}
		g.Level, g.ProjectID = GrantOrganization, ""
	})
	if res := eval(t, m, []Grant{g}, DefaultRules()); res.Decision != DecisionDeny || res.Reasons[0].Class != ClassIrreversibleSCM {
		t.Fatalf("merge: %+v", res)
	}
}

func TestDefaultRuleIDsAreUniqueAndSourcedAsDefault(t *testing.T) {
	seen := map[string]bool{}
	for _, r := range DefaultRules() {
		if seen[r.ID] || r.Source != SourceDefault {
			t.Errorf("rule %s: duplicate or not a default", r.ID)
		}
		seen[r.ID] = true
	}
}
