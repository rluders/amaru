package porttest

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
)

// SettingsStore verifies the same isolation and atomic patch contract for every adapter.
func SettingsStore(t *testing.T, create func(*testing.T) (ports.SettingsStore, ports.ProjectStore), reopen func(ports.SettingsStore) ports.SettingsStore) {
	s, projects := create(t)
	ctx := context.Background()
	a := project(t, "https://github.com/acme/a", commitA, "same")
	b := project(t, "https://github.com/acme/b", commitA, "same")
	missing := project(t, "https://github.com/acme/missing", commitA, "same")
	for _, p := range []domain.Project{a, b} {
		if err := projects.Create(ctx, p); err != nil {
			t.Fatal(err)
		}
	}
	check := func(key domain.ProjectKey, want bool) {
		t.Helper()
		got, err := s.Settings(ctx, key)
		if err != nil || got.ImportClaudeSkills != want {
			t.Fatalf("settings: %+v %v want %v", got, err, want)
		}
	}
	patch := func(value bool) domain.UpdateSettings { return domain.UpdateSettings{ImportClaudeSkills: &value} }
	check(a.Key, false)
	check(b.Key, false)
	for range 2 {
		got, err := s.UpdateSettings(ctx, a.Key, patch(true))
		if err != nil || !got.ImportClaudeSkills {
			t.Fatalf("update: %+v %v", got, err)
		}
	}
	check(a.Key, true)
	check(b.Key, false)
	if _, err := s.Settings(ctx, missing.Key); err != domain.ErrNotFound {
		t.Fatalf("missing read: %v", err)
	}
	if _, err := s.UpdateSettings(ctx, missing.Key, patch(true)); err != domain.ErrNotFound {
		t.Fatalf("missing write: %v", err)
	}
	if _, err := s.UpdateSettings(ctx, a.Key, domain.UpdateSettings{}); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("empty patch: %v", err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := s.UpdateSettings(cancelled, a.Key, patch(false)); err == nil {
		t.Fatal("cancelled update succeeded")
	}
	check(a.Key, true)
	var wg sync.WaitGroup
	errs := make(chan error, 32)
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(value bool) {
			defer wg.Done()
			got, err := s.UpdateSettings(ctx, b.Key, patch(value))
			if err == nil && got.ImportClaudeSkills != value {
				err = domain.ErrInvalid
			}
			errs <- err
		}(i%2 == 0)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.UpdateSettings(ctx, b.Key, patch(false)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpdateSettings(ctx, a.Key, patch(false)); err != nil {
		t.Fatal(err)
	}
	s = reopen(s)
	check(a.Key, false)
	check(b.Key, false)
	if _, err := s.UpdateSettings(ctx, a.Key, patch(true)); err != nil {
		t.Fatal(err)
	}
	s = reopen(s)
	check(a.Key, true)
	check(b.Key, false)
}
