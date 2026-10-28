package remotejob

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"syscall"
	"time"
)

// ErrTooLarge is returned when a body exceeds its limit.
var ErrTooLarge = errors.New("body exceeds the size limit")

// ErrForbiddenAddress is returned when a download would connect to an address that is not public.
var ErrForbiddenAddress = errors.New("download address is not public")

// maxRedirects bounds the redirects a download follows.
const maxRedirects = 5

// DownloadOptions control Download. The zero value is the safe one.
type DownloadOptions struct {
	// AllowHTTP accepts http URLs. For tests only.
	AllowHTTP bool
	// AllowPrivate accepts loopback, private, link-local and other non-public addresses. For tests
	// only.
	AllowPrivate bool
	// Client replaces the default one. The caller then owns the redirect and address rules.
	Client *http.Client
	// Timeout bounds the whole download when Client is nil; zero means ten minutes.
	Timeout time.Duration
}

// Download fetches a signed result URL without any credentials. The URL must be https. Every
// connection, including each redirect hop and whatever the name resolves to at connect time, must
// reach a public address: a service that returns a URL which redirects to a loopback, private or
// link-local address (for example a cloud metadata endpoint) is refused. At most maxRedirects
// redirects are followed. The body is read up to limit bytes and a longer one is ErrTooLarge,
// never truncated. Proxies from the environment are not used: the check is on the address the
// connection is made to.
func Download(ctx context.Context, raw string, limit int64, opt DownloadOptions) ([]byte, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "https" && !(opt.AllowHTTP && u.Scheme == "http")) {
		return nil, errors.New("result URL must be https")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	client := opt.Client
	if client == nil {
		client = safeClient(opt)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, &HTTPError{Status: resp.StatusCode, RetryAfter: RetryAfter(resp.Header)}
	}
	return readBounded(resp.Body, limit)
}

func safeClient(opt DownloadOptions) *http.Client {
	timeout := opt.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Minute
	}
	dialer := &net.Dialer{Timeout: 30 * time.Second, Control: func(_, address string, _ syscall.RawConn) error {
		if opt.AllowPrivate {
			return nil
		}
		return checkAddress(address)
	}}
	return &http.Client{
		Timeout:   timeout,
		Transport: &http.Transport{Proxy: nil, DialContext: dialer.DialContext, TLSHandshakeTimeout: 30 * time.Second},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return fmt.Errorf("stopped after %d redirects", maxRedirects)
			}
			if req.URL.Scheme != "https" && !(opt.AllowHTTP && req.URL.Scheme == "http") {
				return errors.New("redirect to a non-https URL")
			}
			return nil
		},
	}
}

// checkAddress refuses an "ip:port" that is not a public address.
func checkAddress(address string) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return fmt.Errorf("%w: %q", ErrForbiddenAddress, host)
	}
	ip = ip.Unmap()
	if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsMulticast() || cgnat.Contains(ip) {
		return fmt.Errorf("%w: %s", ErrForbiddenAddress, ip)
	}
	return nil
}

// cgnat is the shared address space of RFC 6598, which netip does not count as private.
var cgnat = netip.MustParsePrefix("100.64.0.0/10")
