package localauth

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/rluders/amaru/services/platform/internal/coordinator"
	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports/fakes"
)

type secretIndexFunc func(context.Context, domain.SecretIdentity) (bool, error)

func (f secretIndexFunc) Has(ctx context.Context, id domain.SecretIdentity) (bool, error) {
	return f(ctx, id)
}

// Contexts come from real guarded dispatch; this handler is not a product secret endpoint.
func TestAuthenticatedSecretResolution(t *testing.T) {
	for _, lifecycle := range []string{"live", "logout", "expiry", "cancellation", "shutdown"} {
		for _, scope := range []domain.SecretScope{domain.SecretProject, domain.SecretOrganization} {
			t.Run(lifecycle+"/"+string(scope), func(t *testing.T) {
				a, _, _ := setup(t)
				now := time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
				a.now = func() time.Time { return now }
				projects := fakes.NewProjectStore()
				alpha := domain.ProjectKey("ssh://example.com/alpha#" + strings.Repeat("a", 40))
				beta := domain.ProjectKey("ssh://example.com/beta#" + strings.Repeat("b", 40))
				for _, key := range []domain.ProjectKey{alpha, beta} {
					if err := projects.Create(context.Background(), domain.Project{Key: key, Name: "same-name", CreatedAt: now}); err != nil {
						t.Fatal(err)
					}
				}
				access, err := coordinator.NewProjectAccess(coordinator.New(fakes.NewExecutionStore(), time.Now), projects, a.Principal)
				if err != nil {
					t.Fatal(err)
				}
				var captured context.Context
				h := a.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { captured = r.Context(); w.WriteHeader(204) }))
				capture := func(headers map[string]string) context.Context {
					t.Helper()
					captured = nil
					if w := call(h, "GET", "/secret-test", headers); w.Code != 204 || captured == nil {
						t.Fatal("authenticated dispatch failed")
					}
					return captured
				}
				cookie, csrf := launch(t, a, h)
				secondCookie, _ := launch(t, a, h)
				browser := capture(map[string]string{"Cookie": cookie.String()})
				secondBrowser := capture(map[string]string{"Cookie": secondCookie.String()})
				native := capture(nativeHeaders(a))
				ctx, cancel := context.WithCancel(browser)
				defer cancel()
				var calls []domain.SecretIdentity
				var duringLookup func()
				index := secretIndexFunc(func(_ context.Context, id domain.SecretIdentity) (bool, error) {
					calls = append(calls, id)
					if duringLookup != nil {
						f := duringLookup
						duringLookup = nil
						f()
					}
					// No values exist in this fixture: only exact identity existence metadata.
					return id.Organization == "org" && (id.Name == "TOKEN" || (id.Project == beta && id.Name == "beta-only") || (id.Project == "" && id.Name == "org-only")), nil
				})
				resolver, err := coordinator.NewSecretResolver(access, "org", index)
				if err != nil {
					t.Fatal(err)
				}
				resolve := func(ctx context.Context, project domain.ProjectKey, ref domain.SecretReference, want domain.SecretIdentity, wantErr error) {
					t.Helper()
					got, err := resolver.Resolve(ctx, project, ref)
					if got != want || err != wantErr {
						t.Fatalf("identity/result mismatch: %+v %v", got, err)
					}
				}
				ref := domain.SecretReference{Scope: scope, Name: "TOKEN"}
				identity := func(project domain.ProjectKey, name string) domain.SecretIdentity {
					return domain.SecretIdentity{Organization: "org", Project: project, Name: name}
				}
				expected := identity(alpha, "TOKEN")
				if scope == domain.SecretOrganization {
					expected.Project = ""
				}
				if lifecycle == "live" {
					for _, client := range []context.Context{browser, secondBrowser, native} {
						for _, project := range []domain.ProjectKey{alpha, beta} {
							want := identity(project, "TOKEN")
							if scope == domain.SecretOrganization {
								want.Project = ""
							}
							before := len(calls)
							resolve(client, project, ref, want, nil)
							if len(calls) != before+1 || calls[before] != want {
								t.Fatal("lookup changed scope or fell back")
							}
						}
						for _, name := range []string{"beta-only", "org-only"} {
							before := len(calls)
							resolve(client, alpha, domain.SecretReference{Name: name}, domain.SecretIdentity{}, domain.ErrNotFound)
							if len(calls) != before+1 || calls[before] != identity(alpha, name) {
								t.Fatal("missing Project secret fell back")
							}
						}
						resolve(client, alpha, domain.SecretReference{Scope: domain.SecretOrganization, Name: "org-only"}, identity("", "org-only"), nil)
					}
					before := len(calls)
					resolve(context.Background(), alpha, ref, domain.SecretIdentity{}, domain.ErrNotPermitted)
					other, _, _ := setup(t)
					otherHandler := other.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { captured = r.Context(); w.WriteHeader(204) }))
					captured = nil
					if w := call(otherHandler, "GET", "/secret-test", nativeHeaders(other)); w.Code != 204 || captured == nil {
						t.Fatal("other Instance dispatch failed")
					}
					resolve(captured, alpha, ref, domain.SecretIdentity{}, domain.ErrNotPermitted)
					missing := domain.ProjectKey("ssh://example.com/missing#" + strings.Repeat("c", 40))
					resolve(browser, missing, ref, domain.SecretIdentity{}, domain.ErrNotFound)
					if len(calls) != before {
						t.Fatal("unauthenticated lookup reached index")
					}
					return
				}
				duringLookup = func() {
					switch lifecycle {
					case "logout":
						if w := call(h, "DELETE", "/v1/auth/session", map[string]string{"Cookie": cookie.String(), "Origin": "http://localhost:7777", "X-Amaru-CSRF": csrf}); w.Code != 204 {
							t.Fatal("logout failed")
						}
					case "expiry":
						now = now.Add(SessionLifetime)
					case "cancellation":
						cancel()
					case "shutdown":
						a.Close()
					}
				}
				resolve(ctx, alpha, ref, domain.SecretIdentity{}, domain.ErrNotPermitted)
				if len(calls) != 1 || calls[0] != expected {
					t.Fatal("in-flight lookup did not use exact identity")
				}
				resolve(ctx, alpha, ref, domain.SecretIdentity{}, domain.ErrNotPermitted)
				if len(calls) != 1 {
					t.Fatal("revoked request reached index again")
				}
				// Revoking one browser session/request must not revoke another client.
				if lifecycle == "logout" || lifecycle == "cancellation" {
					resolve(secondBrowser, alpha, ref, expected, nil)
					resolve(native, alpha, ref, expected, nil)
				}
				if lifecycle == "expiry" {
					resolve(native, alpha, ref, expected, nil)
				}
				if lifecycle == "shutdown" {
					before := len(calls)
					resolve(native, alpha, ref, domain.SecretIdentity{}, domain.ErrNotPermitted)
					resolve(secondBrowser, alpha, ref, domain.SecretIdentity{}, domain.ErrNotPermitted)
					if len(calls) != before {
						t.Fatal("shutdown client reached index")
					}
				}
			})
		}
	}
}
