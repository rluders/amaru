// Package egress enforces the address-level half of egress consent (FEAT-POL-004, #168): a
// connection to a loopback or private address is refused unless its host is allowlisted, and
// link-local, multicast, unspecified and reserved addresses are always refused. The check runs at
// connection time on the address that is dialed, so a host name cannot be pointed at a local
// service after the check (DNS rebinding).
//
// It does not decide consent: whether a URL may be contacted at all is Policy's decision
// (domain.CapabilityNetEgress with a domain.ResourceURL resource). Written fresh in Amaru; no
// prototype code is migrated.
package egress
