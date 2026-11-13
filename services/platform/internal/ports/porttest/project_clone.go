package porttest

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
)

// ProjectCloneStore runs atomic selection and isolation checks against both adapters.
func ProjectCloneStore(t *testing.T, create func(*testing.T) ports.ProjectCloneStore, reopen func(ports.ProjectCloneStore) ports.ProjectCloneStore) {
	s := create(t)
	ctx := context.Background()
	a := project(t, "https://github.com/acme/a", commitA, "same")
	b := project(t, "https://github.com/acme/b", commitA, "same")
	missing := project(t, "https://github.com/acme/missing", commitA, "same")
	root1, root2, root3 := t.TempDir(), t.TempDir(), t.TempDir()
	register := func(p domain.Project, root string) (domain.Project, bool, error) {
		return s.RegisterClone(ctx, p, domain.ProjectClone{ProjectKey: p.Key, Root: root})
	}
	check := func(key domain.ProjectKey, root string) {
		t.Helper()
		got, err := s.ActiveClone(ctx, key)
		if err != nil || got.ProjectKey != key || got.Root != root {
			t.Fatalf("active clone = %+v %v, want %s", got, err, root)
		}
	}
	for _, key := range []domain.ProjectKey{a.Key, missing.Key} {
		if _, err := s.ActiveClone(ctx, key); !errors.Is(err, domain.ErrNotFound) {
			t.Fatal(err)
		}
	}
	got, created, err := register(a, root1)
	if err != nil || !created || got != a {
		t.Fatalf("first: %+v %v %v", got, created, err)
	}
	// Reattach preserves shared display metadata, timestamp and settings.
	settings := s.(ports.SettingsStore)
	enabled := true
	if _, err := settings.UpdateSettings(ctx, a.Key, domain.UpdateSettings{ImportClaudeSkills: &enabled}); err != nil {
		t.Fatal(err)
	}
	changed := a
	changed.Name = "ignored"
	changed.CreatedAt = a.CreatedAt.Add(time.Hour)
	for _, root := range []string{root1, root2, root2} {
		got, created, err = register(changed, root)
		if err != nil || created || got != a {
			t.Fatalf("reattach: %+v %v %v", got, created, err)
		}
		check(a.Key, root)
	}
	if setting, err := settings.Settings(ctx, a.Key); err != nil || !setting.ImportClaudeSkills {
		t.Fatalf("settings lost: %+v %v", setting, err)
	}
	// Collision must roll back a new Project as well as leave the old binding intact.
	if _, _, err := register(b, root2); !errors.Is(err, domain.ErrExists) {
		t.Fatalf("collision: %v", err)
	}
	if _, err := s.Get(ctx, b.Key); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("partially created Project: %v", err)
	}
	check(a.Key, root2)
	if _, _, err := register(b, root3); err != nil {
		t.Fatal(err)
	}
	if _, _, err := register(b, root2); !errors.Is(err, domain.ErrExists) {
		t.Fatal(err)
	}
	check(b.Key, root3)
	// Syntax checks do not access the filesystem; identity was checked by the attach adapter.
	for _, root := range []string{"", root1 + "\x00"} {
		if _, _, err := register(a, root); !errors.Is(err, domain.ErrInvalid) {
			t.Fatalf("invalid root %q: %v", root, err)
		}
	}
	if _, _, err := s.RegisterClone(ctx, a, domain.ProjectClone{ProjectKey: b.Key, Root: root1}); !errors.Is(err, domain.ErrInvalid) {
		t.Fatal(err)
	}
	invalid := a
	invalid.Name = ""
	if _, _, err := register(invalid, root1); !errors.Is(err, domain.ErrInvalid) {
		t.Fatal(err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, _, err := s.RegisterClone(cancelled, missing, domain.ProjectClone{ProjectKey: missing.Key, Root: root1}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := s.ActiveClone(cancelled, a.Key); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := s.ActiveClone(ctx, missing.Key); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("cancelled selection persisted", err)
	}
	check(a.Key, root2)
	// Concurrent registrations serialize without replacing the Project or inventing roots.
	var wg sync.WaitGroup
	errs := make(chan error, 16)
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			root := root1
			if i%2 == 0 {
				root = root2
			}
			got, created, err := register(changed, root)
			if err == nil && (created || got != a) {
				err = domain.ErrInvalid
			}
			errs <- err
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	gotClone, err := s.ActiveClone(ctx, a.Key)
	if err != nil || (gotClone.Root != root1 && gotClone.Root != root2) {
		t.Fatalf("concurrent: %+v %v", gotClone, err)
	}
	if _, _, err := register(a, root1); err != nil {
		t.Fatal(err)
	}
	s = reopen(s)
	check(a.Key, root1)
	check(b.Key, root3)
	if got, err := s.Get(ctx, a.Key); err != nil || got.Key != a.Key || got.Name != a.Name || !got.CreatedAt.Equal(a.CreatedAt) {
		t.Fatal("Project changed", got, err)
	}
}
