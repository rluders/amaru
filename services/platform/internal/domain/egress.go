package domain

import (
	"net/netip"
	"net/url"
	"strconv"
	"strings"
)

// Egress consent for FEAT-POL-004. Sending data to an external URL is the Capability net.egress,
// decided by Policy like every other action: without a Grant it needs an Approval, and a Grant or
// an Approval names one URL (ResourceURL) or a prefix of them. A separate, address-level rule
// sits under that: a destination that resolves to a loopback or private address is refused unless
// its host is on the allowlist, and consent alone never lifts it (the Guard in internal/egress
// applies it at connection time).

// CapabilityNetEgress is the Capability of sending data to an external destination. It matches the
// default rule in DefaultRules.
const CapabilityNetEgress Capability = "net.egress"

// ResourceURL is the Resource type of an egress request. Its value is EgressTarget.Key.
const ResourceURL = "url"

// MaxEgressURL bounds the URL a request may name.
const MaxEgressURL = 2048

// EgressTarget is a parsed, normalized destination.
type EgressTarget struct {
	Scheme string
	// Host is lower case, without a trailing dot and without the brackets of an IPv6 literal.
	Host string
	Port int
	// Path starts with "/" and holds no "." or ".." segment.
	Path string
}

// ParseEgressTarget normalizes a URL for consent. It accepts http and https only, refuses
// credentials in the URL, non-ASCII hosts (write the punycode form), control characters and dot
// segments in the path, and drops the query and the fragment: a consent covers an endpoint, and a
// query often carries a token that must not be stored in a Grant or an Approval.
func ParseEgressTarget(raw string) (EgressTarget, error) {
	if raw == "" || len(raw) > MaxEgressURL {
		return EgressTarget{}, invalid("egress: the URL is empty or longer than %d bytes", MaxEgressURL)
	}
	for _, c := range raw {
		if c <= ' ' || c == 0x7f || c >= 0x80 || c == '\\' {
			return EgressTarget{}, invalid("egress: the URL holds a control, space, backslash or non-ASCII character")
		}
	}
	u, err := url.Parse(raw)
	if err != nil {
		return EgressTarget{}, invalid("egress: the URL does not parse")
	}
	t := EgressTarget{Scheme: strings.ToLower(u.Scheme)}
	switch {
	case t.Scheme != "http" && t.Scheme != "https":
		return EgressTarget{}, invalid("egress: scheme %q is not http or https", u.Scheme)
	case u.User != nil:
		return EgressTarget{}, invalid("egress: credentials in the URL are refused")
	case u.Opaque != "":
		return EgressTarget{}, invalid("egress: the URL is opaque")
	}
	t.Host = strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	if t.Host == "" {
		return EgressTarget{}, invalid("egress: the URL has no host")
	}
	t.Port = 80
	if t.Scheme == "https" {
		t.Port = 443
	}
	if ps := u.Port(); ps != "" {
		p, err := strconv.Atoi(ps)
		if err != nil || p < 1 || p > 65535 {
			return EgressTarget{}, invalid("egress: port %q is not valid", ps)
		}
		t.Port = p
	}
	t.Path = u.EscapedPath()
	if t.Path == "" {
		t.Path = "/"
	}
	for _, seg := range strings.Split(t.Path, "/") {
		switch strings.ToLower(seg) {
		case ".", "..", "%2e", "%2e%2e", ".%2e", "%2e.":
			return EgressTarget{}, invalid("egress: the path holds a dot segment")
		}
	}
	return t, nil
}

// Key is the consent key: scheme, host, explicit port and path, e.g. "https://api.example.com:443/v1/x".
// The port is always written so a prefix pattern ending in "*" cannot match another host
// ("https://api.example.com:443/*" does not match "https://api.example.com.evil.test:443/").
func (t EgressTarget) Key() string {
	host := t.Host
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	return t.Scheme + "://" + host + ":" + strconv.Itoa(t.Port) + t.Path
}

// PolicyRequest fills base (principal, subject, scope, trigger, definition hash, project trust)
// with the egress Capability and the URL resource.
func (t EgressTarget) PolicyRequest(base PolicyRequest) PolicyRequest {
	base.Capability = CapabilityNetEgress
	base.Resource = Resource{Type: ResourceURL, Value: t.Key()}
	return base
}

// AddressClass says what kind of destination an IP address is.
type AddressClass string

const (
	AddressPublic      AddressClass = "public"
	AddressLoopback    AddressClass = "loopback"
	AddressPrivate     AddressClass = "private"    // RFC 1918, unique local, and shared address space (RFC 6598)
	AddressLinkLocal   AddressClass = "link-local" // includes the cloud metadata address
	AddressMulticast   AddressClass = "multicast"
	AddressUnspecified AddressClass = "unspecified"
	AddressReserved    AddressClass = "reserved" // anything else that is not global unicast
)

var sharedAddressSpace = netip.MustParsePrefix("100.64.0.0/10")

// reservedPrefixes are special-purpose ranges that Go's IsGlobalUnicast still accepts. None is a
// destination an agent should reach, and the translation ranges can embed a private IPv4
// address, so they are refused (fail safe; an IPv6-only network that needs 64:ff9b::/96 can add
// its own rule when a use is decided).
var reservedPrefixes = func() []netip.Prefix {
	var out []netip.Prefix
	for _, p := range []string{
		"0.0.0.0/8", "192.0.0.0/24", "192.0.2.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4",
		"64:ff9b::/96", "64:ff9b:1::/48", "100::/64", "2001::/32", "2001:db8::/32", "2002::/16",
	} {
		out = append(out, netip.MustParsePrefix(p))
	}
	return out
}()

// ClassifyAddress classifies ip. An IPv4-mapped IPv6 address is classified as the IPv4 address,
// so "::ffff:127.0.0.1" is loopback.
func ClassifyAddress(ip netip.Addr) AddressClass {
	ip = ip.Unmap().WithZone("")
	switch {
	case !ip.IsValid() || ip.IsUnspecified():
		return AddressUnspecified
	case ip.IsLoopback():
		return AddressLoopback
	case ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsInterfaceLocalMulticast():
		return AddressLinkLocal
	case ip.IsMulticast():
		return AddressMulticast
	case ip.IsPrivate() || sharedAddressSpace.Contains(ip):
		return AddressPrivate
	case !ip.IsGlobalUnicast():
		return AddressReserved
	}
	for _, p := range reservedPrefixes {
		if p.Contains(ip) {
			return AddressReserved
		}
	}
	return AddressPublic
}

// EgressAllowlist names the hosts that may reach loopback or private addresses (a LAN receiver, a
// local service). It is not a consent list: a host that is not on it is still reachable when it is
// public and consented. An entry is a host name, an IP literal, or "*.suffix" for the subdomains
// of a suffix (not the suffix itself).
type EgressAllowlist struct {
	PrivateHosts []string
}

// AllowsPrivate reports whether host is on the allowlist.
func (a EgressAllowlist) AllowsPrivate(host string) bool {
	host = normalizeHost(host)
	if host == "" {
		return false
	}
	for _, e := range a.PrivateHosts {
		e = normalizeHost(e)
		if suffix, ok := strings.CutPrefix(e, "*."); ok {
			if suffix != "" && strings.HasSuffix(host, "."+suffix) {
				return true
			}
			continue
		}
		if e != "" && e == host {
			return true
		}
	}
	return false
}

func normalizeHost(h string) string {
	h = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(h)), ".")
	return strings.TrimSuffix(strings.TrimPrefix(h, "["), "]")
}

// AddressAllowed decides whether a connection to an address of class c may be made for a host
// that is or is not on the allowlist, with the reason. Public addresses are allowed. Loopback and
// private addresses are allowed only for an allowlisted host. Link-local (the cloud metadata
// address), multicast, unspecified and reserved addresses are never allowed, allowlisted or not.
func AddressAllowed(c AddressClass, hostAllowlisted bool) (bool, string) {
	switch c {
	case AddressPublic:
		return true, "public address"
	case AddressLoopback, AddressPrivate:
		if hostAllowlisted {
			return true, string(c) + " address, host allowlisted"
		}
		return false, string(c) + " address, host not allowlisted"
	}
	return false, string(c) + " addresses are never reachable"
}
