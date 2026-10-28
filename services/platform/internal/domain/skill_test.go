package domain

import (
	"errors"
	"strings"
	"testing"
)

func orgSkill(t *testing.T, name, version, body string) Skill {
	t.Helper()
	s, err := NewSkill(t0, human(RoleMaintainer), SkillOrganization, "", name, version, body)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func projSkill(t *testing.T, name, version, body string) Skill {
	t.Helper()
	s, err := NewSkill(t0, human(RoleMaintainer), SkillProject, trustKey, name, version, body)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestNewSkillComputesTheHashItself(t *testing.T) {
	s := orgSkill(t, "review", "1", "Check the diff.")
	if s.Hash != HashBody("Check the diff.") || s.AddedBy != "u-maintainer" {
		t.Fatalf("%+v", s)
	}
	// A Skill read back with a hash that is not its body's is invalid.
	s.Hash = strings.Repeat("0", 64)
	if err := s.Validate(); !errors.Is(err, ErrInvalid) {
		t.Fatalf("hash not of the body: %v", err)
	}
}

func TestNewSkillWhoMayAdd(t *testing.T) {
	for _, role := range []Role{RoleContributor, RoleViewer, ""} {
		if _, err := NewSkill(t0, human(role), SkillOrganization, "", "review", "1", "x"); !errors.Is(err, ErrNotPermitted) {
			t.Errorf("%q: %v", role, err)
		}
	}
	if _, err := NewSkill(t0, Principal{Kind: PrincipalAgent, ID: "a", Sponsor: "u", Role: RoleOwner}, SkillOrganization, "", "review", "1", "x"); !errors.Is(err, ErrNotPermitted) {
		t.Errorf("agent: %v", err)
	}
}

func TestSkillShape(t *testing.T) {
	m := human(RoleMaintainer)
	bad := map[string]error{}
	_, bad["bad name"] = NewSkill(t0, m, SkillOrganization, "", "Bad Name", "1", "x")
	_, bad["no version"] = NewSkill(t0, m, SkillOrganization, "", "review", "", "x")
	_, bad["empty body"] = NewSkill(t0, m, SkillOrganization, "", "review", "1", "")
	_, bad["huge body"] = NewSkill(t0, m, SkillOrganization, "", "review", "1", strings.Repeat("a", MaxSkillBody+1))
	_, bad["unknown scope"] = NewSkill(t0, m, "team", "", "review", "1", "x")
	_, bad["project skill with a path as key"] = NewSkill(t0, m, SkillProject, "/home/me/app", "review", "1", "x")
	_, bad["org skill naming a project"] = NewSkill(t0, m, SkillOrganization, trustKey, "review", "1", "x")
	for name, err := range bad {
		if !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: got %v, want invalid", name, err)
		}
	}
}

func TestResolveSkill(t *testing.T) {
	org := orgSkill(t, "review", "1", "Check the diff.")
	proj := projSkill(t, "lint", "2", "Run the linter.")
	ref := func(s Skill) SkillRef { return SkillRef{Name: s.Name, Version: s.Version, Hash: s.Hash} }
	trusted, _ := NewTrust(t0, human(RoleMaintainer), trustKey, TrustSkill, "lint", SkillTrustHash(proj))
	otherHash, _ := NewTrust(t0, human(RoleMaintainer), trustKey, TrustSkill, "lint", "sha256:"+strings.Repeat("a", 64))
	revoked := trusted
	_ = revoked.Revoke(t0, human(RoleMaintainer), "x")
	code := func(err error) string {
		if se, ok := IsSkillError(err); ok {
			return se.Code
		}
		if err != nil {
			return "other: " + err.Error()
		}
		return ""
	}
	cases := []struct {
		name  string
		ref   SkillRef
		found *Skill
		trust *Trust
		want  string
	}{
		{"organization skill at its hash", ref(org), &org, nil, ""},
		{"project skill, trusted at its hash", ref(proj), &proj, &trusted, ""},
		{"project skill, never trusted", ref(proj), &proj, nil, ReasonSkillUntrusted},
		{"project skill, trusted at another content", ref(proj), &proj, &otherHash, ReasonSkillUntrusted},
		{"project skill, trust revoked", ref(proj), &proj, &revoked, ReasonSkillUntrusted},
		{"project skill, trust of another kind", ref(proj), &proj, &Trust{Kind: TrustWorkflow, Hash: trusted.Hash}, ReasonSkillUntrusted},
		{"unknown skill", ref(org), nil, nil, ReasonSkillUnknown},
		{"pinned hash is not the skill's", SkillRef{Name: "review", Version: "1", Hash: strings.Repeat("0", 64)}, &org, nil, ReasonSkillHashMismatch},
		{"bad name", SkillRef{Name: "Bad", Version: "1", Hash: org.Hash}, &org, nil, ReasonSkillInvalid},
		{"no version", SkillRef{Name: "review", Hash: org.Hash}, &org, nil, ReasonSkillInvalid},
		{"hash not hex", SkillRef{Name: "review", Version: "1", Hash: "sha256:abc"}, &org, nil, ReasonSkillInvalid},
		{"no hash", SkillRef{Name: "review", Version: "1"}, &org, nil, ReasonSkillInvalid},
	}
	for _, c := range cases {
		use, err := ResolveSkill(c.ref, c.found, c.trust)
		if got := code(err); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
		if c.want == "" && (use.Name != c.ref.Name || use.Hash != c.ref.Hash || use.Version != c.ref.Version) {
			t.Errorf("%s: use %+v", c.name, use)
		}
	}
}
