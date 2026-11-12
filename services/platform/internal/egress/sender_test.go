package egress

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"

	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/provider/remotejob"
)

type unreadBody struct{ t *testing.T }

func (b unreadBody) Read([]byte) (int, error) { b.t.Fatal("denied body read"); return 0, io.EOF }
func (b unreadBody) Close() error             { return nil }

func TestSenderRefusesBeforeDNSOrBody(t *testing.T) {
	for _, consent := range []bool{false, true} {
		t.Run(map[bool]string{false: "missing checker", true: "wrong URL"}[consent], func(t *testing.T) {
			s := Sender{Guard: Guard{Resolver: func(context.Context, string) ([]netip.Addr, error) { t.Fatal("denied DNS lookup"); return nil, nil }}}
			if consent {
				s.Authorize = func(_ context.Context, target domain.EgressTarget) error {
					if target.Key() != "https://api.test:443/other" {
						return errors.New("secret denied")
					}
					return nil
				}
			}
			req, _ := http.NewRequest("POST", "https://api.test/send?token=secret", unreadBody{t})
			_, err := s.Do(req)
			if err == nil || strings.Contains(err.Error(), "secret") {
				t.Fatalf("unsafe refusal: %v", err)
			}
		})
	}
}

func TestSenderChecksDestinationsAndUsesVettedIP(t *testing.T) {
	for _, tc := range []struct {
		name  string
		ips   []string
		allow bool
		want  bool
	}{
		{"public", []string{"93.184.216.34"}, false, true},
		{"loopback", []string{"127.0.0.1"}, false, false},
		{"private", []string{"10.0.0.1"}, false, false},
		{"allowed private", []string{"10.0.0.1"}, true, true},
		{"allowed loopback", []string{"127.0.0.1"}, true, true},
		{"mixed", []string{"93.184.216.34", "10.0.0.1"}, false, false},
		{"metadata always denied", []string{"169.254.169.254"}, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			received := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { received++; w.Write([]byte(`{}`)) }))
			defer srv.Close()
			g := Guard{Resolver: resolver(map[string][]string{"api.test": tc.ips})}
			if tc.allow {
				g.Allow.PrivateHosts = []string{"api.test"}
			}
			dials := 0
			g.Dial = func(ctx context.Context, network, address string) (net.Conn, error) {
				dials++
				if address != net.JoinHostPort(tc.ips[0], "80") {
					t.Errorf("not vetted IP: %s", address)
				}
				return (&net.Dialer{}).DialContext(ctx, network, strings.TrimPrefix(srv.URL, "http://"))
			}
			s := Sender{Guard: g, Authorize: func(_ context.Context, tg domain.EgressTarget) error {
				if tg.Key() != "http://api.test:80/send" {
					t.Errorf("key %s", tg.Key())
				}
				return nil
			}}
			// Exercise the existing Provider caller, rather than only the transport.
			c := remotejob.Client{BaseURL: "http://api.test", HTTP: s}
			err := c.Do(context.Background(), "POST", "/send", map[string]string{"message": "ok"}, nil)
			if (err == nil) != tc.want {
				t.Fatalf("result %v", err)
			}
			if !tc.want && (dials != 0 || received != 0) {
				t.Fatalf("denied request escaped: %d/%d", dials, received)
			}
			if tc.want && received != 1 {
				t.Fatalf("requests %d", received)
			}
		})
	}
}

func TestSenderRechecksDNSAndDoesNotFollowRedirects(t *testing.T) {
	received := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received++
		http.Redirect(w, r, "http://169.254.169.254/secret", 302)
	}))
	defer srv.Close()
	lookups := 0
	s := Sender{Guard: Guard{Resolver: func(context.Context, string) ([]netip.Addr, error) {
		lookups++
		if lookups == 1 {
			return []netip.Addr{netip.MustParseAddr("93.184.216.34")}, nil
		}
		return []netip.Addr{netip.MustParseAddr("127.0.0.1")}, nil
	}, Dial: func(ctx context.Context, n, a string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, n, strings.TrimPrefix(srv.URL, "http://"))
	}}, Authorize: func(context.Context, domain.EgressTarget) error { return nil }}
	req, _ := http.NewRequest("GET", "http://api.test/x?token=secret", nil)
	resp, err := s.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 302 || received != 1 {
		t.Fatal("redirect followed")
	}
	_, err = s.Do(req)
	if !errors.Is(err, ErrBlocked) || received != 1 {
		t.Fatalf("rebinding: %d %v", received, err)
	}
}

func TestSenderDownloadAndErrorsAreSafe(t *testing.T) {
	s := Sender{Guard: Guard{Resolver: resolver(map[string][]string{"api.test": {"93.184.216.34"}}), Dial: func(context.Context, string, string) (net.Conn, error) {
		return nil, errors.New("secret transport credentials")
	}}, Authorize: func(context.Context, domain.EgressTarget) error { return nil }}
	_, err := remotejob.Download(context.Background(), "https://api.test/x?token=secret", 10, remotejob.DownloadOptions{Client: s})
	if err == nil || strings.Contains(err.Error(), "secret") {
		t.Fatalf("unsafe error: %v", err)
	}
	req, _ := http.NewRequest("GET", "http://user:secret@api.test/x", nil)
	_, err = s.Do(req)
	if err == nil || strings.Contains(err.Error(), "secret") {
		t.Fatalf("credentials: %v", err)
	}
	req, _ = http.NewRequest("GET", "http://api.test/x", nil)
	req.Host = "other.test"
	if _, err = s.Do(req); err == nil {
		t.Fatal("host override accepted")
	}
}
