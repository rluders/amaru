package egress

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"time"

	"github.com/rluders/amaru/services/platform/internal/domain"
)

// ErrBlocked is returned, wrapped with the reason and without the URL, when a destination is
// refused.
var ErrBlocked = errors.New("egress destination blocked")

// Resolver resolves a host name to addresses. Tests inject one.
type Resolver func(ctx context.Context, host string) ([]netip.Addr, error)

// Guard dials only destinations the address rule allows.
type Guard struct {
	Allow domain.EgressAllowlist
	// Resolver is how names are resolved; nil means the system resolver.
	Resolver Resolver
	// Timeout bounds one dial; zero means 10 s.
	Timeout time.Duration
}

func (g Guard) resolve(ctx context.Context, host string) ([]netip.Addr, error) {
	if ip, err := netip.ParseAddr(host); err == nil {
		return []netip.Addr{ip}, nil
	}
	if g.Resolver != nil {
		return g.Resolver(ctx, host)
	}
	return net.DefaultResolver.LookupNetIP(ctx, "ip", host)
}

// vet resolves host and refuses it unless every answer is allowed. One refused answer refuses the
// host: the Guard never picks around a bad record.
func (g Guard) vet(ctx context.Context, host string) ([]netip.Addr, error) {
	addrs, err := g.resolve(ctx, host)
	if err != nil || len(addrs) == 0 {
		return nil, errors.New("egress: the host did not resolve")
	}
	allowlisted := g.Allow.AllowsPrivate(host)
	for _, a := range addrs {
		if ok, why := domain.AddressAllowed(domain.ClassifyAddress(a), allowlisted); !ok {
			return nil, fmt.Errorf("%w: %s", ErrBlocked, why)
		}
	}
	return addrs, nil
}

// Check vets the host of t without connecting, for a caller that wants to refuse before it asks
// for consent. The connection is vetted again when it is made.
func (g Guard) Check(ctx context.Context, t domain.EgressTarget) error {
	_, err := g.vet(ctx, t.Host)
	return err
}

// DialContext is an http.Transport dialer: it vets the host and dials the vetted address itself,
// so the address that was checked is the address that is used.
func (g Guard) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, fmt.Errorf("%w: malformed address", ErrBlocked)
	}
	if _, err := strconv.ParseUint(port, 10, 16); err != nil {
		return nil, fmt.Errorf("%w: malformed port", ErrBlocked)
	}
	addrs, err := g.vet(ctx, host)
	if err != nil {
		return nil, err
	}
	timeout := g.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	d := net.Dialer{Timeout: timeout}
	var last error
	for _, a := range addrs {
		c, err := d.DialContext(ctx, network, net.JoinHostPort(a.String(), port))
		if err == nil {
			return c, nil
		}
		last = err
	}
	return nil, last
}

// Client returns an HTTP client that connects only through the Guard. It follows no redirect (a
// redirect target is a new URL and needs its own consent: the caller reads the Location and asks
// Policy), uses no proxy from the environment and keeps no idle connection.
func (g Guard) Client(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout:       timeout,
		Transport:     &http.Transport{DialContext: g.DialContext, Proxy: nil, DisableKeepAlives: true},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}
