// Package porttest holds behavior tests that every implementation of a port must pass: the
// in-memory fake and the PostgreSQL adapter run the same suite, so the fake cannot drift.
package porttest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
)

const (
	commitA = "0123456789abcdef0123456789abcdef01234567"
	commitB = "fedcba9876543210fedcba9876543210fedcba98"
)

func project(t *testing.T, remote, commit, name string) domain.Project {
	t.Helper()
	k, err := domain.NewProjectKey(remote, commit)
	if err != nil {
		t.Fatal(err)
	}
	return domain.Project{Key: k, Name: name, CreatedAt: time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)}
}

// ProjectStore runs the ports.ProjectStore contract against newStore, which must return an empty
// store each call.
func ProjectStore(t *testing.T, newStore func(t *testing.T) ports.ProjectStore) {
	ctx := context.Background()

	t.Run("create then get", func(t *testing.T) {
		s := newStore(t)
		p := project(t, "https://github.com/acme/app", commitA, "App")
		if err := s.Create(ctx, p); err != nil {
			t.Fatal(err)
		}
		got, err := s.Get(ctx, p.Key)
		if err != nil {
			t.Fatal(err)
		}
		if got.Key != p.Key || got.Name != p.Name || !got.CreatedAt.Equal(p.CreatedAt) {
			t.Fatalf("got %+v, want %+v", got, p)
		}
	})

	t.Run("get unknown", func(t *testing.T) {
		s := newStore(t)
		_, err := s.Get(ctx, project(t, "https://github.com/acme/app", commitA, "App").Key)
		if !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("got %v, want ErrNotFound", err)
		}
	})

	t.Run("create twice", func(t *testing.T) {
		s := newStore(t)
		p := project(t, "https://github.com/acme/app", commitA, "App")
		if err := s.Create(ctx, p); err != nil {
			t.Fatal(err)
		}
		p.Name = "Other"
		if err := s.Create(ctx, p); !errors.Is(err, domain.ErrExists) {
			t.Fatalf("got %v, want ErrExists", err)
		}
		got, _ := s.Get(ctx, p.Key)
		if got.Name != "App" {
			t.Fatalf("second create changed the stored name to %q", got.Name)
		}
	})

	t.Run("invalid project is refused", func(t *testing.T) {
		s := newStore(t)
		p := project(t, "https://github.com/acme/app", commitA, "App")
		p.Name = ""
		if err := s.Create(ctx, p); !errors.Is(err, domain.ErrInvalid) {
			t.Fatalf("got %v, want ErrInvalid", err)
		}
		if l, _ := s.List(ctx); len(l) != 0 {
			t.Fatalf("invalid project was stored: %+v", l)
		}
	})

	t.Run("same remote with another root commit is another project", func(t *testing.T) {
		s := newStore(t)
		a := project(t, "https://github.com/acme/app", commitA, "A")
		b := project(t, "https://github.com/acme/app", commitB, "B")
		for _, p := range []domain.Project{a, b} {
			if err := s.Create(ctx, p); err != nil {
				t.Fatal(err)
			}
		}
		if l, _ := s.List(ctx); len(l) != 2 {
			t.Fatalf("got %d projects, want 2", len(l))
		}
	})

	t.Run("list is ordered by key", func(t *testing.T) {
		s := newStore(t)
		for _, r := range []string{"https://github.com/acme/zeta", "https://github.com/acme/alpha", "https://github.com/acme/mid"} {
			if err := s.Create(ctx, project(t, r, commitA, "P")); err != nil {
				t.Fatal(err)
			}
		}
		l, err := s.List(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if len(l) != 3 || !(l[0].Key < l[1].Key && l[1].Key < l[2].Key) {
			t.Fatalf("not ordered by key: %+v", l)
		}
	})
}
