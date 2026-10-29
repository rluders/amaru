package domain

import (
	"errors"
	"net/netip"
	"strings"
	"testing"
)

func TestParseEgressTargetNormalizes(t *testing.T) {
	for raw, want := range map[string]string{
		"https://API.Example.com/v1/x?token=secret#frag": "https://api.example.com:443/v1/x",
		"http://example.com":                             "http://example.com:80/",
		"https://example.com.:8443/a%20b":                "https://example.com:8443/a%20b",
		"https://[2001:db8::1]/p":                        "https://[2001:db8::1]:443/p",
		"HTTPS://example.com/":                           "https://example.com:443/",
	} {
		tg, err := ParseEgressTarget(raw)
		if err != nil || tg.Key() != want {
			t.Errorf("%q: key %q, err %v; want %q", raw, tg.Key(), err, want)
		}
	}
}

func TestParseEgressTargetRefusals(t *testing.T) {
	for name, raw := range map[string]string{
		"empty":             "",
		"too long":          "https://example.com/" + strings.Repeat("a", MaxEgressURL),
		"ftp":               "ftp://example.com/",
		"file":              "file:///etc/passwd",
		"no scheme":         "example.com/x",
		"credentials":       "https://user:pw@example.com/",
		"user only":         "https://user@example.com/",
		"no host":           "https:///x",
		"opaque":            "https:example.com",
		"port zero":         "https://example.com:0/",
		"port too big":      "https://example.com:99999/",
		"port text":         "https://example.com:http/",
		"space":             "https://example.com/a b",
		"control":           "https://example.com/a\x00b",
		"backslash":         "https://example.com\\@evil.test/",
		"non-ASCII host":    "https://exаmple.com/", // Cyrillic a
		"dot segment":       "https://example.com/a/../b",
		"single dot":        "https://example.com/./b",
		"encoded dot":       "https://example.com/a/%2e%2e/b",
		"encoded dot upper": "https://example.com/%2E%2E/b",
	} {
		if _, err := ParseEgressTarget(raw); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: %q must be refused, got %v", name, raw, err)
		}
	}
}

func TestEgressKeyCannotBeConfusedByAPrefixGrant(t *testing.T) {
	good, _ := ParseEgressTarget("https://api.example.com/v1/")
	evil, _ := ParseEgressTarget("https://api.example.com.evil.test/v1/")
	pattern := "https://api.example.com:443/*"
	if !matchPattern(pattern, good.Key()) || matchPattern(pattern, evil.Key()) {
		t.Fatalf("pattern %s: good %q, evil %q", pattern, good.Key(), evil.Key())
	}
}

func TestClassifyAddress(t *testing.T) {
	for ip, want := range map[string]AddressClass{
		"8.8.8.8":                AddressPublic,
		"2606:4700:4700::1111":   AddressPublic,
		"127.0.0.1":              AddressLoopback,
		"127.255.255.254":        AddressLoopback,
		"::1":                    AddressLoopback,
		"::ffff:127.0.0.1":       AddressLoopback,
		"10.0.0.1":               AddressPrivate,
		"172.16.0.1":             AddressPrivate,
		"172.31.255.255":         AddressPrivate,
		"192.168.1.1":            AddressPrivate,
		"fd00::1":                AddressPrivate,
		"100.64.0.1":             AddressPrivate,
		"100.127.255.255":        AddressPrivate,
		"::ffff:10.0.0.1":        AddressPrivate,
		"::ffff:100.64.0.1":      AddressPrivate,
		"::ffff:240.0.0.1":       AddressReserved,
		"::ffff:8.8.8.8":         AddressPublic,
		"169.254.169.254":        AddressLinkLocal,
		"fe80::1":                AddressLinkLocal,
		"fe80::1%eth0":           AddressLinkLocal,
		"::ffff:169.254.169.254": AddressLinkLocal,
		"224.0.0.1":              AddressLinkLocal,
		"239.1.1.1":              AddressMulticast,
		"ff02::1":                AddressLinkLocal,
		"ff0e::1":                AddressMulticast,
		"0.0.0.0":                AddressUnspecified,
		"::":                     AddressUnspecified,
		"240.0.0.1":              AddressReserved,
		"255.255.255.255":        AddressReserved,
		"0.1.2.3":                AddressReserved,
		"198.18.0.1":             AddressReserved,
		"203.0.113.7":            AddressReserved,
		"2001:db8::1":            AddressReserved,
		"64:ff9b::a00:1":         AddressReserved, // NAT64 form of 10.0.0.1
		"2002:a00:1::":           AddressReserved, // 6to4 form of 10.0.0.1
		"172.32.0.1":             AddressPublic,   // just outside 172.16/12
		"100.128.0.1":            AddressPublic,   // just outside 100.64/10
	} {
		if got := ClassifyAddress(netip.MustParseAddr(ip)); got != want {
			t.Errorf("%s: %s, want %s", ip, got, want)
		}
	}
	if got := ClassifyAddress(netip.Addr{}); got != AddressUnspecified {
		t.Errorf("the zero address: %s", got)
	}
}

func TestAddressAllowed(t *testing.T) {
	for _, tc := range []struct {
		class     AddressClass
		allowlist bool
		want      bool
	}{
		{AddressPublic, false, true}, {AddressPublic, true, true},
		{AddressLoopback, false, false}, {AddressLoopback, true, true},
		{AddressPrivate, false, false}, {AddressPrivate, true, true},
		{AddressLinkLocal, false, false}, {AddressLinkLocal, true, false},
		{AddressMulticast, true, false}, {AddressUnspecified, true, false}, {AddressReserved, true, false},
		{AddressClass("surprise"), true, false},
	} {
		if got, why := AddressAllowed(tc.class, tc.allowlist); got != tc.want || why == "" {
			t.Errorf("%s allowlisted=%v: %v %q, want %v", tc.class, tc.allowlist, got, why, tc.want)
		}
	}
}

func TestEgressAllowlist(t *testing.T) {
	a := EgressAllowlist{PrivateHosts: []string{"Hooks.Internal.", "*.lan.test", "127.0.0.1", "[::1]", "", "*."}}
	for host, want := range map[string]bool{
		"hooks.internal":  true,
		"HOOKS.INTERNAL.": true,
		"a.lan.test":      true,
		"a.b.lan.test":    true,
		"lan.test":        false, // the wildcard is for subdomains only
		"evil-lan.test":   false,
		"x.lan.test.evil": false,
		"127.0.0.1":       true,
		"::1":             true,
		"[::1]":           true,
		"10.0.0.1":        false,
		"":                false,
		"x":               false,
	} {
		if got := a.AllowsPrivate(host); got != want {
			t.Errorf("%q: %v, want %v", host, got, want)
		}
	}
	if (EgressAllowlist{}).AllowsPrivate("anything") {
		t.Error("an empty allowlist allows nothing")
	}
}

// Per-URL consent through the one Policy function (ADR-0037, ADR-0055).
func TestEgressConsentIsPerURL(t *testing.T) {
	target := func(raw string) EgressTarget {
		tg, err := ParseEgressTarget(raw)
		if err != nil {
			t.Fatal(err)
		}
		return tg
	}
	consented := target("https://api.example.com/v1/send")
	g := grant(t, "g-url", func(g *Grant) {
		g.Capabilities = []Capability{CapabilityNetEgress}
		g.Resource = Resource{Type: ResourceURL, Value: consented.Key()}
	})
	rules := DefaultRules()
	ask := func(raw string, gs ...Grant) PolicyResult {
		return eval(t, target(raw).PolicyRequest(req("x.y")), gs, rules)
	}

	if r := ask("https://api.example.com/v1/send"); r.Decision != DecisionRequireApproval {
		t.Fatalf("no consent yet: %s", r.Decision)
	}
	if r := ask("https://api.example.com/v1/send?token=abc", g); r.Decision != DecisionAllow || r.MatchedGrantID != "g-url" {
		t.Fatalf("the consented URL (the query is not part of it): %+v", r)
	}
	for _, other := range []string{
		"https://api.example.com/v1/other",
		"http://api.example.com/v1/send", // another scheme and port
		"https://api.example.com:8443/v1/send",
		"https://evil.example.com/v1/send",
		"https://api.example.com.evil.test/v1/send",
	} {
		if r := ask(other, g); r.Decision != DecisionRequireApproval {
			t.Errorf("consent for one URL must not cover %s: %s", other, r.Decision)
		}
	}

	prefix := grant(t, "g-prefix", func(g *Grant) {
		g.Capabilities = []Capability{CapabilityNetEgress}
		g.Resource = Resource{Type: ResourceURL, Value: "https://api.example.com:443/v1/*"}
	})
	if r := ask("https://api.example.com/v1/anything", prefix); r.Decision != DecisionAllow {
		t.Fatalf("a prefix consent: %s", r.Decision)
	}
	if r := ask("https://api.example.com.evil.test/v1/anything", prefix); r.Decision != DecisionRequireApproval {
		t.Fatalf("a prefix consent must not cover another host: %s", r.Decision)
	}
	if got := target("https://api.example.com/v1/send?token=abc").PolicyRequest(req("x.y")).Resource.Value; strings.Contains(got, "token") {
		t.Fatalf("a query must not reach the resource: %q", got)
	}
}
