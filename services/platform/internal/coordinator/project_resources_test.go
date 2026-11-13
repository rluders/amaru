package coordinator_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/rluders/amaru/services/platform/internal/coordinator"
	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
	"github.com/rluders/amaru/services/platform/internal/ports/fakes"
)

type inspectFunc func(context.Context, string) (ports.ProjectInspection, error)

func (f inspectFunc) Inspect(ctx context.Context, root string) (ports.ProjectInspection, error) {
	return f(ctx, root)
}

type resourceStore struct {
	*fakes.ProjectStore
	read, committed         func()
	failure                 error
	reads, bindings, writes int
}

func (s *resourceStore) Get(ctx context.Context, k domain.ProjectKey) (domain.Project, error) {
	s.reads++
	if s.read != nil {
		s.read()
	}
	return s.ProjectStore.Get(ctx, k)
}
func (s *resourceStore) List(ctx context.Context) ([]domain.Project, error) {
	s.reads++
	if s.read != nil {
		s.read()
	}
	return s.ProjectStore.List(ctx)
}
func (s *resourceStore) ActiveClone(ctx context.Context, k domain.ProjectKey) (domain.ProjectClone, error) {
	s.bindings++
	if s.read != nil {
		s.read()
	}
	return s.ProjectStore.ActiveClone(ctx, k)
}
func (s *resourceStore) RegisterClone(ctx context.Context, p domain.Project, c domain.ProjectClone) (domain.Project, bool, error) {
	s.writes++
	if s.failure != nil {
		return domain.Project{}, false, s.failure
	}
	got, created, err := s.ProjectStore.RegisterClone(ctx, p, c)
	if s.committed != nil {
		s.committed()
	}
	return got, created, err
}
func TestProjectResources(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 11, 13, 0, 0, 0, 0, time.UTC)
	keyA := domain.ProjectKey("ssh://github.com/acme/a#0123456789abcdef0123456789abcdef01234567")
	keyB := domain.ProjectKey("ssh://github.com/acme/b#0123456789abcdef0123456789abcdef01234567")
	keyC := domain.ProjectKey("ssh://github.com/acme/c#0123456789abcdef0123456789abcdef01234567")
	setup := func(t *testing.T) (*coordinator.ProjectAccess, *resourceStore, *bool) {
		t.Helper()
		revoked := false
		store := &resourceStore{ProjectStore: fakes.NewProjectStore()}
		api, err := coordinator.NewProjectAccess(coordinator.New(fakes.NewExecutionStore(), func() time.Time { return now }), store, func(context.Context) (domain.Principal, error) {
			if revoked {
				return domain.Principal{}, domain.ErrNotPermitted
			}
			return domain.Principal{Kind: domain.PrincipalHuman, ID: "admin", Role: domain.RoleOwner}, nil
		})
		if err != nil {
			t.Fatal(err)
		}
		return api, store, &revoked
	}
	input := ports.ProjectInspection{Key: keyA, Name: "suggested", Root: "/captured", ConfigPresent: true}
	inspect := inspectFunc(func(context.Context, string) (ports.ProjectInspection, error) { return input, nil })
	t.Run("authority before I/O", func(t *testing.T) {
		api, s, revoked := setup(t)
		*revoked = true
		never := inspectFunc(func(context.Context, string) (ports.ProjectInspection, error) {
			t.Fatal("unauthorized inspection")
			return input, nil
		})
		if _, _, err := api.AttachProject(ctx, input.Root, never); err != domain.ErrNotPermitted {
			t.Fatal(err)
		}
		if _, err := api.Project(ctx, keyA, never); err != domain.ErrNotPermitted {
			t.Fatal(err)
		}
		if _, err := api.Projects(ctx, never); err != domain.ErrNotPermitted {
			t.Fatal(err)
		}
		if s.reads+s.bindings+s.writes != 0 {
			t.Fatal("unauthorized storage")
		}
	})
	t.Run("revoked and canceled inspection", func(t *testing.T) {
		for _, canceled := range []bool{false, true} {
			api, s, revoked := setup(t)
			request, cancel := context.WithCancel(ctx)
			blocked := inspectFunc(func(got context.Context, _ string) (ports.ProjectInspection, error) {
				if got != request {
					t.Fatal("context replaced")
				}
				if canceled {
					cancel()
				} else {
					*revoked = true
				}
				return input, nil
			})
			_, _, err := api.AttachProject(request, input.Root, blocked)
			cancel()
			want := domain.ErrNotPermitted
			if canceled {
				want = context.Canceled
			}
			if !errors.Is(err, want) || s.writes != 0 {
				t.Fatal(err, s.writes)
			}
		}
	})
	t.Run("metadata preserved and committed mutation retained", func(t *testing.T) {
		api, s, revoked := setup(t)
		p := domain.Project{Key: keyA, Name: "stored", CreatedAt: now.Add(-time.Hour)}
		if err := s.Create(ctx, p); err != nil {
			t.Fatal(err)
		}
		s.committed = func() { *revoked = true }
		if _, _, err := api.AttachProject(ctx, input.Root, inspect); err != domain.ErrNotPermitted {
			t.Fatal(err)
		}
		*revoked = false
		s.committed = nil
		clone, err := s.ActiveClone(ctx, keyA)
		if err != nil || clone.Root != input.Root {
			t.Fatal(clone, err)
		}
		out, created, err := api.AttachProject(ctx, input.Root, inspect)
		if err != nil || created || out.Project != p {
			t.Fatal(out, created, err)
		}
	})
	t.Run("inspection store and ownership failures", func(t *testing.T) {
		api, s, _ := setup(t)
		if _, _, err := api.AttachProject(ctx, input.Root, inspect); err != nil {
			t.Fatal(err)
		}
		bad := inspectFunc(func(context.Context, string) (ports.ProjectInspection, error) {
			return ports.ProjectInspection{}, domain.ErrInvalid
		})
		if _, _, err := api.AttachProject(ctx, "/other", bad); err != domain.ErrInvalid || s.writes != 1 {
			t.Fatal(err, s.writes)
		}
		failure := errors.New("database unavailable")
		s.failure = failure
		if _, _, err := api.AttachProject(ctx, input.Root, inspect); !errors.Is(err, failure) {
			t.Fatal(err)
		}
		s.failure = nil
		foreign := input
		foreign.Key = keyB
		if _, _, err := api.AttachProject(ctx, input.Root, inspectFunc(func(context.Context, string) (ports.ProjectInspection, error) { return foreign, nil })); err != domain.ErrConflict {
			t.Fatal(err)
		}
		if _, err := s.Get(ctx, keyB); !errors.Is(err, domain.ErrNotFound) {
			t.Fatal(err)
		}
		clone, _ := s.ActiveClone(ctx, keyA)
		if clone.Root != input.Root {
			t.Fatal(clone)
		}
	})
	t.Run("ordering ties single capture and all or error", func(t *testing.T) {
		api, s, _ := setup(t)
		for i, p := range []domain.Project{{Key: keyA, Name: "z", CreatedAt: now}, {Key: keyC, Name: "a", CreatedAt: now}, {Key: keyB, Name: "a", CreatedAt: now}} {
			if _, _, err := s.RegisterClone(ctx, p, domain.ProjectClone{ProjectKey: p.Key, Root: []string{"/a", "/c", "/b"}[i]}); err != nil {
				t.Fatal(err)
			}
		}
		roots := map[string]domain.ProjectKey{}
		for _, k := range []domain.ProjectKey{keyA, keyB, keyC} {
			c, _ := s.ProjectStore.ActiveClone(ctx, k)
			roots[c.Root] = k
		}
		calls := map[string]int{}
		reader := inspectFunc(func(_ context.Context, root string) (ports.ProjectInspection, error) {
			calls[root]++
			return ports.ProjectInspection{Key: roots[root], Root: root, Name: "ignored"}, nil
		})
		out, err := api.Projects(ctx, reader)
		if err != nil || len(out) != 3 || out[0].Project.Key != keyB || out[1].Project.Key != keyC || out[2].Project.Key != keyA {
			t.Fatal(out, err)
		}
		if s.bindings != 3 || !reflect.DeepEqual(calls, map[string]int{"/a": 1, "/b": 1, "/c": 1}) {
			t.Fatal(s.bindings, calls)
		}
		broken := inspectFunc(func(ctx context.Context, root string) (ports.ProjectInspection, error) {
			if roots[root] == keyC {
				return ports.ProjectInspection{}, domain.ErrInvalid
			}
			return reader.Inspect(ctx, root)
		})
		if got, err := api.Projects(ctx, broken); got != nil || err != domain.ErrConflict {
			t.Fatal(got, err)
		}
	})
	t.Run("capture survives reattach without second lookup", func(t *testing.T) {
		api, s, _ := setup(t)
		out, _, err := api.AttachProject(ctx, input.Root, inspect)
		if err != nil {
			t.Fatal(err)
		}
		reader := inspectFunc(func(_ context.Context, root string) (ports.ProjectInspection, error) {
			if root != input.Root {
				t.Fatal(root)
			}
			if _, _, err := s.RegisterClone(ctx, out.Project, domain.ProjectClone{ProjectKey: keyA, Root: "/new"}); err != nil {
				t.Fatal(err)
			}
			return input, nil
		})
		got, err := api.Project(ctx, keyA, reader)
		if err != nil || got.Inspection.Root != input.Root || s.bindings != 1 {
			t.Fatal(got, err, s.bindings)
		}
	})
	t.Run("unavailable mismatched and revoked reads", func(t *testing.T) {
		api, s, revoked := setup(t)
		if err := s.Create(ctx, domain.Project{Key: keyA, Name: "stored", CreatedAt: now}); err != nil {
			t.Fatal(err)
		}
		if _, err := api.Project(ctx, keyA, inspect); err != domain.ErrConflict {
			t.Fatal(err)
		}
		if _, _, err := api.AttachProject(ctx, input.Root, inspect); err != nil {
			t.Fatal(err)
		}
		for _, bad := range []ports.ProjectInspection{{Key: keyB, Root: input.Root}, {Key: keyA, Root: "/parent"}} {
			if _, err := api.Project(ctx, keyA, inspectFunc(func(context.Context, string) (ports.ProjectInspection, error) { return bad, nil })); err != domain.ErrConflict {
				t.Fatal(err)
			}
		}

		for _, failure := range []error{domain.ErrInvalid, errors.New("process unavailable")} {
			reader := inspectFunc(func(context.Context, string) (ports.ProjectInspection, error) {
				return ports.ProjectInspection{}, failure
			})
			_, err := api.Project(ctx, keyA, reader)
			want := failure
			if errors.Is(failure, domain.ErrInvalid) {
				want = domain.ErrConflict
			}
			if !errors.Is(err, want) {
				t.Fatal(err)
			}
			if got, err := api.Projects(ctx, reader); got != nil || !errors.Is(err, want) {
				t.Fatal(got, err)
			}
		}
		reader := inspectFunc(func(context.Context, string) (ports.ProjectInspection, error) { *revoked = true; return input, nil })
		if _, err := api.Project(ctx, keyA, reader); err != domain.ErrNotPermitted {
			t.Fatal(err)
		}
		*revoked = false
		s.read = func() { *revoked = true }
		if _, err := api.Projects(ctx, inspect); err != domain.ErrNotPermitted {
			t.Fatal(err)
		}
	})
	t.Run("unknown malformed empty", func(t *testing.T) {
		api, _, _ := setup(t)
		for _, k := range []domain.ProjectKey{keyA, "invalid"} {
			if _, err := api.Project(ctx, k, inspect); !errors.Is(err, domain.ErrNotFound) {
				t.Fatal(err)
			}
		}
		if got, err := api.Projects(ctx, inspect); err != nil || got == nil || len(got) != 0 {
			t.Fatal(got, err)
		}
	})
}

func TestProjectAttachRechecksImmediatelyBeforeWrite(t *testing.T) {
	revoked := false
	store := &resourceStore{ProjectStore: fakes.NewProjectStore()}
	api, err := coordinator.NewProjectAccess(coordinator.New(fakes.NewExecutionStore(), func() time.Time { revoked = true; return time.Now() }), store, func(context.Context) (domain.Principal, error) {
		if revoked {
			return domain.Principal{}, domain.ErrNotPermitted
		}
		return domain.Principal{Kind: domain.PrincipalHuman, ID: "admin", Role: domain.RoleOwner}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	inspector := inspectFunc(func(context.Context, string) (ports.ProjectInspection, error) {
		return ports.ProjectInspection{Key: domain.ProjectKey("ssh://github.com/acme/a#0123456789abcdef0123456789abcdef01234567"), Name: "a", Root: "/a"}, nil
	})
	if _, _, err := api.AttachProject(context.Background(), "/a", inspector); err != domain.ErrNotPermitted || store.writes != 0 {
		t.Fatal(err, store.writes)
	}
}

func TestProjectResourcesRejectNonAdministrator(t *testing.T) {
	for _, by := range []domain.Principal{
		{}, {Kind: domain.PrincipalHuman, ID: "admin", Role: domain.RoleContributor},
		{Kind: domain.PrincipalHuman, Role: domain.RoleOwner},
		{Kind: domain.PrincipalHuman, ID: "admin", Role: domain.RoleOwner, Sponsor: "other"},
	} {
		s := &resourceStore{ProjectStore: fakes.NewProjectStore()}
		api, err := coordinator.NewProjectAccess(coordinator.New(fakes.NewExecutionStore(), time.Now), s, func(context.Context) (domain.Principal, error) { return by, nil })
		if err != nil {
			t.Fatal(err)
		}
		never := inspectFunc(func(context.Context, string) (ports.ProjectInspection, error) {
			t.Fatal("unauthorized inspection")
			return ports.ProjectInspection{}, nil
		})
		if _, _, err := api.AttachProject(context.Background(), "/a", never); err != domain.ErrNotPermitted {
			t.Fatal(by, err)
		}
		if _, err := api.Projects(context.Background(), never); err != domain.ErrNotPermitted {
			t.Fatal(by, err)
		}
		if s.reads+s.writes != 0 {
			t.Fatal("unauthorized store")
		}
	}
}
