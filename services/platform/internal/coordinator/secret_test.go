package coordinator_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync/atomic"
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

func TestExactScopeSecretResolution(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
	a := domain.ProjectKey("ssh://example.com/a#" + strings.Repeat("a", 40))
	b := domain.ProjectKey("ssh://example.com/b#" + strings.Repeat("a", 40))
	missing := domain.ProjectKey("ssh://example.com/missing#" + strings.Repeat("a", 40))
	projects := fakes.NewProjectStore()
	for _, key := range []domain.ProjectKey{a, b} {
		if err := projects.Create(ctx, domain.Project{Key: key, Name: "same-name", CreatedAt: now}); err != nil {
			t.Fatal(err)
		}
	}
	var revoked atomic.Bool
	access, err := coordinator.NewProjectAccess(coordinator.New(fakes.NewExecutionStore(), func() time.Time { return now }), projects, func(ctx context.Context) (domain.Principal, error) {
		if ctx.Err() != nil {
			return domain.Principal{}, ctx.Err()
		}
		if revoked.Load() {
			return domain.Principal{}, domain.ErrNotPermitted
		}
		return domain.Principal{Kind: domain.PrincipalHuman, ID: "owner", Role: domain.RoleOwner}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	identity := func(org string, project domain.ProjectKey, name string) domain.SecretIdentity {
		return domain.SecretIdentity{Organization: org, Project: project, Name: name}
	}
	records := map[domain.SecretIdentity]bool{
		identity("org", a, "TOKEN"):                true,
		identity("org", b, "TOKEN"):                true,
		identity("org", b, "other-only"):           true,
		identity("org", "", "TOKEN"):               true,
		identity("org", "", "org-only"):            true,
		identity("other-org", a, "other-org-only"): true,
	}
	var calls []domain.SecretIdentity
	index := secretIndexFunc(func(_ context.Context, id domain.SecretIdentity) (bool, error) {
		calls = append(calls, id)
		return records[id], nil
	})
	resolver, err := coordinator.NewSecretResolver(access, "org", index)
	if err != nil {
		t.Fatal(err)
	}
	t.Run("exact identity and no fallback", func(t *testing.T) {
		for _, tc := range []struct {
			project domain.ProjectKey
			scope   domain.SecretScope
			name    string
			want    domain.SecretIdentity
			err     error
		}{
			{a, "", "TOKEN", identity("org", a, "TOKEN"), nil},
			{b, domain.SecretProject, "TOKEN", identity("org", b, "TOKEN"), nil},
			{a, domain.SecretOrganization, "TOKEN", identity("org", "", "TOKEN"), nil},
			{a, domain.SecretOrganization, "org-only", identity("org", "", "org-only"), nil},
			{a, "", "org-only", domain.SecretIdentity{}, domain.ErrNotFound},
			{a, "", "other-only", domain.SecretIdentity{}, domain.ErrNotFound},
			{a, "", "other-org-only", domain.SecretIdentity{}, domain.ErrNotFound},
			{a, "", "token", domain.SecretIdentity{}, domain.ErrNotFound},
			{a, "", " TOKEN ", domain.SecretIdentity{}, domain.ErrNotFound},
		} {
			before := len(calls)
			got, err := resolver.Resolve(ctx, tc.project, domain.SecretReference{Scope: tc.scope, Name: tc.name})
			if got != tc.want || err != tc.err {
				t.Fatalf("%s/%s/%q: %+v %v", tc.project, tc.scope, tc.name, got, err)
			}
			project := tc.project
			if tc.scope == domain.SecretOrganization {
				project = ""
			}
			if !reflect.DeepEqual(calls[before:], []domain.SecretIdentity{identity("org", project, tc.name)}) {
				t.Fatalf("lookup fallback: %+v", calls[before:])
			}
		}
	})
	t.Run("invalid or unauthorized input never accesses index", func(t *testing.T) {
		before := len(calls)
		for _, key := range []domain.ProjectKey{"", missing} {
			for _, scope := range []domain.SecretScope{"", domain.SecretOrganization} {
				got, err := resolver.Resolve(ctx, key, domain.SecretReference{Scope: scope, Name: "TOKEN"})
				if got != (domain.SecretIdentity{}) || err != domain.ErrNotFound {
					t.Fatalf("%+v %v", got, err)
				}
			}
		}
		for _, ref := range []domain.SecretReference{{Name: ""}, {Scope: "user", Name: "TOKEN"}, {Name: "bad\x00name"}, {Name: string([]byte{255})}} {
			got, err := resolver.Resolve(ctx, a, ref)
			if got != (domain.SecretIdentity{}) || err != domain.ErrInvalid {
				t.Fatalf("%+v %v", got, err)
			}
		}
		revoked.Store(true)
		for _, scope := range []domain.SecretScope{"", domain.SecretOrganization} {
			got, err := resolver.Resolve(ctx, a, domain.SecretReference{Scope: scope, Name: "TOKEN"})
			if got != (domain.SecretIdentity{}) || err != domain.ErrNotPermitted {
				t.Fatalf("%+v %v", got, err)
			}
		}
		revoked.Store(false)
		cancelled, cancel := context.WithCancel(ctx)
		cancel()
		if _, err := resolver.Resolve(cancelled, a, domain.SecretReference{Name: "TOKEN"}); !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
		if len(calls) != before {
			t.Fatalf("rejected lookup: %+v", calls[before:])
		}
	})
	t.Run("revocation during IO withholds identity", func(t *testing.T) {
		defer revoked.Store(false)
		r, err := coordinator.NewSecretResolver(access, "org", secretIndexFunc(func(context.Context, domain.SecretIdentity) (bool, error) { revoked.Store(true); return true, nil }))
		if err != nil {
			t.Fatal(err)
		}
		got, err := r.Resolve(ctx, a, domain.SecretReference{Name: "TOKEN"})
		if got != (domain.SecretIdentity{}) || err != domain.ErrNotPermitted {
			t.Fatalf("%+v %v", got, err)
		}
	})
	t.Run("backend errors never expose diagnostics or partial success", func(t *testing.T) {
		for _, backendErr := range []error{errors.New("public-fixture-value"), errors.Join(domain.ErrNotFound, errors.New("public-fixture-value"))} {
			r, err := coordinator.NewSecretResolver(access, "org", secretIndexFunc(func(context.Context, domain.SecretIdentity) (bool, error) { return true, backendErr }))
			if err != nil {
				t.Fatal(err)
			}
			got, err := r.Resolve(ctx, a, domain.SecretReference{Name: "TOKEN"})
			if got != (domain.SecretIdentity{}) || err != domain.ErrSecretUnavailable || strings.Contains(err.Error(), "public-fixture-value") {
				t.Fatalf("unsafe lookup result")
			}
		}
	})
	t.Run("trusted dependencies required", func(t *testing.T) {
		if _, err := coordinator.NewSecretResolver(nil, "org", index); err != domain.ErrInvalid {
			t.Fatal(err)
		}
		if _, err := coordinator.NewSecretResolver(access, " ", index); err != domain.ErrInvalid {
			t.Fatal(err)
		}
		if _, err := coordinator.NewSecretResolver(access, "org", nil); err != domain.ErrInvalid {
			t.Fatal(err)
		}
		if got, err := (domain.SecretReference{Name: "TOKEN"}).Bind("", a); err != domain.ErrInvalid || got != (domain.SecretIdentity{}) {
			t.Fatal("invalid organization accepted")
		}
	})
}
