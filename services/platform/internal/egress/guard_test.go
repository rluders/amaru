package egress

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/rluders/amaru/services/platform/internal/domain"
)

func resolver(m map[string][]string) Resolver {
	return func(_ context.Context, host string) ([]netip.Addr, error) {
		var out []netip.Addr
		for _, s := range m[host] {
			out = append(out, netip.MustParseAddr(s))
		}
		if len(out) == 0 {
			return nil, errors.New("no such host")
		}
		return out, nil
	}
}

func server(t *testing.T) (host, port string, url string) {
	t.Helper()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, "http://169.254.169.254/latest/meta-data", http.StatusFound)
			return
		}
		w.Write([]byte("hello"))
	}))
	t.Cleanup(s.Close)
	host, port, _ = net.SplitHostPort(strings.TrimPrefix(s.URL, "http://"))
	return host, port, s.URL
}

func get(g Guard, url string) (int, error) {
	resp, err := g.Client(5 * time.Second).Get(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	return resp.StatusCode, nil
}

func TestLoopbackIsBlockedUnlessAllowlisted(t *testing.T) {
	host, _, url := server(t)
	if _, err := get(Guard{}, url); !errors.Is(err, ErrBlocked) {
		t.Fatalf("a loopback server must be blocked by default: %v", err)
	}
	if _, err := get(Guard{Allow: domain.EgressAllowlist{PrivateHosts: []string{"other"}}}, url); !errors.Is(err, ErrBlocked) {
		t.Fatalf("another host's allowlist entry must not unblock it: %v", err)
	}
	if code, err := get(Guard{Allow: domain.EgressAllowlist{PrivateHosts: []string{host}}}, url); err != nil || code != 200 {
		t.Fatalf("an allowlisted loopback host: %d %v", code, err)
	}
}

func TestAHostNameCannotBePointedAtALocalService(t *testing.T) {
	_, port, _ := server(t)
	url := "http://svc.example.test:" + port + "/"
	cases := map[string]struct {
		answers []string
		allow   []string
		wantErr bool
	}{
		"rebinds to loopback":              {[]string{"127.0.0.1"}, nil, true},
		"rebinds to a private address":     {[]string{"10.1.2.3"}, nil, true},
		"one bad answer among good ones":   {[]string{"93.184.216.34", "127.0.0.1"}, nil, true},
		"mapped loopback":                  {[]string{"::ffff:127.0.0.1"}, nil, true},
		"allowlisted name to loopback":     {[]string{"127.0.0.1"}, []string{"svc.example.test"}, false},
		"allowlisted wildcard to loopback": {[]string{"127.0.0.1"}, []string{"*.example.test"}, false},
		"allowlisted name to metadata":     {[]string{"169.254.169.254"}, []string{"svc.example.test"}, true},
		"allowlisted name to unspecified":  {[]string{"0.0.0.0"}, []string{"svc.example.test"}, true},
		"allowlisted but one is metadata":  {[]string{"127.0.0.1", "169.254.169.254"}, []string{"svc.example.test"}, true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			g := Guard{Allow: domain.EgressAllowlist{PrivateHosts: tc.allow}, Resolver: resolver(map[string][]string{"svc.example.test": tc.answers})}
			code, err := get(g, url)
			if tc.wantErr && !errors.Is(err, ErrBlocked) {
				t.Fatalf("must be blocked: %d %v", code, err)
			}
			if !tc.wantErr && (err != nil || code != 200) {
				t.Fatalf("must be reachable: %d %v", code, err)
			}
		})
	}
}

func TestUnresolvableAndMalformedDestinationsFail(t *testing.T) {
	g := Guard{Resolver: resolver(nil)}
	if _, err := g.DialContext(context.Background(), "tcp", "nope.test:80"); err == nil || errors.Is(err, ErrBlocked) {
		t.Fatalf("an unresolvable host is an error, not a policy block: %v", err)
	}
	for _, addr := range []string{"nocolon", "host:notaport", "host:70000"} {
		if _, err := g.DialContext(context.Background(), "tcp", addr); !errors.Is(err, ErrBlocked) {
			t.Errorf("%q must be refused: %v", addr, err)
		}
	}
}

func TestRedirectsAreNotFollowed(t *testing.T) {
	host, _, url := server(t)
	g := Guard{Allow: domain.EgressAllowlist{PrivateHosts: []string{host}}}
	resp, err := g.Client(5 * time.Second).Get(url + "/redirect")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusFound || !strings.Contains(resp.Header.Get("Location"), "169.254.169.254") {
		t.Fatalf("the redirect must be returned, not followed: %d %q", resp.StatusCode, resp.Header.Get("Location"))
	}
}

// The standard library reads the proxy variables once per process, so a behavior test would depend
// on test order; the configuration is checked instead.
func TestClientUsesNoProxyAndKeepsNoConnection(t *testing.T) {
	c := Guard{}.Client(time.Second)
	tr, ok := c.Transport.(*http.Transport)
	if !ok || tr.Proxy != nil || !tr.DisableKeepAlives || tr.DialContext == nil {
		t.Fatalf("transport %+v", c.Transport)
	}
}

func TestCheckRefusesBeforeConnecting(t *testing.T) {
	g := Guard{Resolver: resolver(map[string][]string{"ok.test": {"93.184.216.34"}, "bad.test": {"192.168.0.10"}})}
	ok, _ := domain.ParseEgressTarget("https://ok.test/x")
	bad, _ := domain.ParseEgressTarget("https://bad.test/x")
	lit, _ := domain.ParseEgressTarget("http://127.0.0.1:8080/x")
	if err := g.Check(context.Background(), ok); err != nil {
		t.Fatalf("a public host: %v", err)
	}
	for _, tg := range []domain.EgressTarget{bad, lit} {
		if err := g.Check(context.Background(), tg); !errors.Is(err, ErrBlocked) {
			t.Errorf("%s must be blocked: %v", tg.Key(), err)
		}
	}
}

func TestBlockedErrorNamesNoURL(t *testing.T) {
	g := Guard{Resolver: resolver(map[string][]string{"secret-name.test": {"10.0.0.9"}})}
	_, err := g.DialContext(context.Background(), "tcp", "secret-name.test:443")
	if !errors.Is(err, ErrBlocked) || strings.Contains(err.Error(), "secret-name") {
		t.Fatalf("the reason must not repeat the host: %v", err)
	}
}
