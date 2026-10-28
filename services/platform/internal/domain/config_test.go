package domain

import (
	"errors"
	"testing"
)

var testSchema = ConfigSchema{
	"model":        {Name: "model", Kind: ConfigString},
	"theme":        {Name: "theme", Kind: ConfigString, UserSetting: true},
	"egress.open":  {Name: "egress.open", Kind: ConfigBool, Tighten: TightenToFalse},
	"review.must":  {Name: "review.must", Kind: ConfigBool, Tighten: TightenToTrue},
	"retention.d":  {Name: "retention.d", Kind: ConfigInt, Tighten: TightenLower},
	"approvals.mn": {Name: "approvals.mn", Kind: ConfigInt, Tighten: TightenHigher},
}

func layer(s ConfigScope, kv map[string]ConfigValue, locks ...string) ConfigLayer {
	return ConfigLayer{Scope: s, Values: kv, Locks: locks}
}

func resolve(t *testing.T, ls ...ConfigLayer) EffectiveConfig {
	t.Helper()
	c, err := ResolveConfig(testSchema, ls...)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func refused(c EffectiveConfig, s ConfigScope, key, reason string) bool {
	for _, r := range c.Refused {
		if r.Scope == s && r.Key == key && r.Reason == reason {
			return true
		}
	}
	return false
}

func TestMostSpecificScopeWinsWhateverTheOrderGiven(t *testing.T) {
	d := layer(ConfigDeployment, map[string]ConfigValue{"model": StringValue("d")})
	o := layer(ConfigOrganization, map[string]ConfigValue{"model": StringValue("o")})
	p := layer(ConfigProject, map[string]ConfigValue{"model": StringValue("p")})
	for _, ls := range [][]ConfigLayer{{d, o, p}, {p, o, d}, {o, p, d}} {
		c := resolve(t, ls...)
		if v := c.Values["model"]; v.Value.Str != "p" || v.Scope != ConfigProject {
			t.Errorf("got %+v", v)
		}
	}
	if v := resolve(t, d, o).Values["model"]; v.Value.Str != "o" || v.Scope != ConfigOrganization {
		t.Errorf("without a project layer: %+v", v)
	}
}

func TestALockedKeyKeepsTheLockingScopesValue(t *testing.T) {
	o := layer(ConfigOrganization, map[string]ConfigValue{"model": StringValue("o")}, "model")
	p := layer(ConfigProject, map[string]ConfigValue{"model": StringValue("p")})
	c := resolve(t, o, p)
	if v := c.Values["model"]; v.Value.Str != "o" || v.Locked != ConfigOrganization {
		t.Errorf("got %+v", v)
	}
	if !refused(c, ConfigProject, "model", ConfigRefusedLocked) {
		t.Errorf("refused: %+v", c.Refused)
	}
}

func TestADeploymentLockHoldsAgainstTheOrganization(t *testing.T) {
	d := layer(ConfigDeployment, map[string]ConfigValue{"model": StringValue("d")}, "model")
	o := layer(ConfigOrganization, map[string]ConfigValue{"model": StringValue("o")}, "model")
	c := resolve(t, d, o)
	if v := c.Values["model"]; v.Value.Str != "d" || v.Locked != ConfigDeployment {
		t.Errorf("got %+v", v)
	}
}

func TestALockOnAnUnsetKeyStopsLowerScopesSettingIt(t *testing.T) {
	c := resolve(t, layer(ConfigOrganization, nil, "model"), layer(ConfigProject, map[string]ConfigValue{"model": StringValue("p")}))
	if _, ok := c.Values["model"]; ok || !refused(c, ConfigProject, "model", ConfigRefusedLocked) {
		t.Errorf("%+v", c)
	}
}

func TestOnlyDeploymentAndOrganizationMayLock(t *testing.T) {
	for _, s := range []ConfigScope{ConfigProject, ConfigUser} {
		ls := []ConfigLayer{layer(s, map[string]ConfigValue{"theme": StringValue("x")}, "theme")}
		if s != ConfigUser {
			ls = append(ls, layer(ConfigUser, map[string]ConfigValue{"theme": StringValue("y")}))
		}
		c := resolve(t, ls...)
		if !refused(c, s, "theme", ConfigRefusedLockNotAble) {
			t.Errorf("%s: %+v", s, c.Refused)
		}
	}
	// A refused lock locks nothing.
	c := resolve(t, layer(ConfigProject, map[string]ConfigValue{"model": StringValue("p")}, "model"), layer(ConfigUser, map[string]ConfigValue{"theme": StringValue("y")}))
	if c.Values["model"].Locked != "" {
		t.Errorf("project lock took effect: %+v", c.Values["model"])
	}
}

func TestSecurityKeysCanOnlyBeTightenedBelow(t *testing.T) {
	cases := []struct {
		name, key string
		org, proj ConfigValue
		ok        bool
	}{
		{"bool to false, tighter", "egress.open", BoolValue(true), BoolValue(false), true},
		{"bool to false, looser", "egress.open", BoolValue(false), BoolValue(true), false},
		{"bool to true, tighter", "review.must", BoolValue(false), BoolValue(true), true},
		{"bool to true, looser", "review.must", BoolValue(true), BoolValue(false), false},
		{"lower int, tighter", "retention.d", IntValue(90), IntValue(30), true},
		{"lower int, looser", "retention.d", IntValue(30), IntValue(90), false},
		{"lower int, equal", "retention.d", IntValue(30), IntValue(30), true},
		{"higher int, tighter", "approvals.mn", IntValue(1), IntValue(2), true},
		{"higher int, looser", "approvals.mn", IntValue(2), IntValue(1), false},
	}
	for _, tc := range cases {
		c := resolve(t, layer(ConfigOrganization, map[string]ConfigValue{tc.key: tc.org}), layer(ConfigProject, map[string]ConfigValue{tc.key: tc.proj}))
		got := c.Values[tc.key]
		if tc.ok && (got.Value != tc.proj || got.Scope != ConfigProject) {
			t.Errorf("%s: %+v, refused %+v", tc.name, got, c.Refused)
		}
		if !tc.ok && (got.Value != tc.org || !refused(c, ConfigProject, tc.key, ConfigRefusedLoosens)) {
			t.Errorf("%s: %+v, refused %+v", tc.name, got, c.Refused)
		}
	}
}

func TestASecurityKeyNoScopeAboveSetIsFreeToSet(t *testing.T) {
	c := resolve(t, layer(ConfigProject, map[string]ConfigValue{"retention.d": IntValue(999)}))
	if c.Values["retention.d"].Value.Int != 999 || len(c.Refused) != 0 {
		t.Errorf("%+v", c)
	}
}

func TestAUserLayerSetsPreferencesOnly(t *testing.T) {
	c := resolve(t, layer(ConfigOrganization, map[string]ConfigValue{"model": StringValue("o")}),
		layer(ConfigUser, map[string]ConfigValue{"model": StringValue("u"), "theme": StringValue("dark"), "egress.open": BoolValue(true)}))
	if c.Values["model"].Value.Str != "o" || c.Values["theme"].Value.Str != "dark" || c.Values["theme"].Scope != ConfigUser {
		t.Errorf("%+v", c.Values)
	}
	if !refused(c, ConfigUser, "model", ConfigRefusedNotUser) || !refused(c, ConfigUser, "egress.open", ConfigRefusedNotUser) {
		t.Errorf("refused: %+v", c.Refused)
	}
	if _, ok := c.Values["egress.open"]; ok {
		t.Error("a user layer set a security key")
	}
}

func TestBadEntriesAreRefusedWithoutStoppingTheRest(t *testing.T) {
	c := resolve(t, layer(ConfigProject, map[string]ConfigValue{"nope": StringValue("x"), "model": IntValue(3), "theme": StringValue("t")}, "ghost"))
	if !refused(c, ConfigProject, "nope", ConfigRefusedUnknownKey) || !refused(c, ConfigProject, "model", ConfigRefusedWrongKind) || !refused(c, ConfigProject, "ghost", ConfigRefusedUnknownKey) {
		t.Errorf("refused: %+v", c.Refused)
	}
	if _, ok := c.Values["model"]; ok || c.Values["theme"].Value.Str != "t" {
		t.Errorf("values: %+v", c.Values)
	}
}

func TestAWrongLayerSetIsAProgramError(t *testing.T) {
	if _, err := ResolveConfig(testSchema, layer("tenant", nil)); !errors.Is(err, ErrInvalid) {
		t.Errorf("unknown scope: %v", err)
	}
	if _, err := ResolveConfig(testSchema, layer(ConfigProject, nil), layer(ConfigProject, nil)); !errors.Is(err, ErrInvalid) {
		t.Errorf("repeated scope: %v", err)
	}
	for name, s := range map[string]ConfigSchema{
		"renamed":          {"a": {Name: "b", Kind: ConfigBool}},
		"bad kind":         {"a": {Name: "a", Kind: "float"}},
		"tighten on text":  {"a": {Name: "a", Kind: ConfigString, Tighten: TightenLower}},
		"wrong direction":  {"a": {Name: "a", Kind: ConfigBool, Tighten: TightenLower}},
		"secure user pref": {"a": {Name: "a", Kind: ConfigBool, Tighten: TightenToFalse, UserSetting: true}},
	} {
		if _, err := ResolveConfig(s); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestDigestIsStableAndSeesValuesScopesAndRefusals(t *testing.T) {
	mk := func(proj string, userKey string) EffectiveConfig {
		return resolve(t, layer(ConfigOrganization, map[string]ConfigValue{"model": StringValue("o")}),
			layer(ConfigProject, map[string]ConfigValue{"model": StringValue(proj)}), layer(ConfigUser, map[string]ConfigValue{userKey: StringValue("x")}))
	}
	base := mk("p", "theme")
	if base.Digest == "" || base.Digest != mk("p", "theme").Digest {
		t.Fatal("digest is not stable")
	}
	if base.Digest == mk("q", "theme").Digest {
		t.Error("a changed value left the digest")
	}
	if base.Digest == mk("p", "model").Digest {
		t.Error("a refusal left the digest")
	}
	same := resolve(t, layer(ConfigProject, map[string]ConfigValue{"model": StringValue("p")}))
	moved := resolve(t, layer(ConfigOrganization, map[string]ConfigValue{"model": StringValue("p")}))
	if same.Digest == moved.Digest {
		t.Error("the scope a value came from left the digest")
	}
}
