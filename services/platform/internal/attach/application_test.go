package attach_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rluders/amaru/services/platform/internal/coordinator"
	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
	"github.com/rluders/amaru/services/platform/internal/ports/fakes"
)

func TestProjectApplicationWithGit(t *testing.T) {
	needGit(t)
	ctx := context.Background()
	a, store := newAttacher()
	api, err := coordinator.NewProjectAccess(coordinator.New(fakes.NewExecutionStore(), func() time.Time { return t0 }), store, func(context.Context) (domain.Principal, error) {
		return domain.Principal{Kind: domain.PrincipalHuman, ID: "admin", Role: domain.RoleOwner}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	first := repo(t, "https://github.com/acme/a.git")
	second := repo(t, "https://github.com/acme/b.git")
	for _, root := range []string{"", ".", filepath.Join(t.TempDir(), "missing"), filepath.Join(first, "a.txt"), t.TempDir()} {
		if _, _, err := api.AttachProject(ctx, root, a); !errors.Is(err, domain.ErrInvalid) {
			t.Fatalf("invalid root %q: %v", root, err)
		}
	}
	pa, created, err := api.AttachProject(ctx, first, a)
	if err != nil || !created {
		t.Fatal(pa, created, err)
	}
	pb, created, err := api.AttachProject(ctx, second, a)
	if err != nil || !created || pb.Project.Key == pa.Project.Key {
		t.Fatal(pb, created, err)
	}
	enabled := true
	if _, err := store.UpdateSettings(ctx, pa.Project.Key, domain.UpdateSettings{ImportClaudeSkills: &enabled}); err != nil {
		t.Fatal(err)
	}
	clone := filepath.Join(t.TempDir(), "clone")
	git(t, "", "clone", "-q", first, clone)
	git(t, clone, "remote", "set-url", "origin", "https://github.com/acme/a.git")
	again, created, err := api.AttachProject(ctx, clone, a)
	if err != nil || created || again.Project != pa.Project || again.Inspection.Root != clone {
		t.Fatal(again, created, err)
	}
	if settings, err := store.Settings(ctx, pa.Project.Key); err != nil || !settings.ImportClaudeSkills {
		t.Fatal(settings, err)
	}
	got, err := api.Project(ctx, pa.Project.Key, a)
	if err != nil || got.Project != pa.Project || got.Inspection.Root != clone {
		t.Fatal(got, err)
	}
	before := snapshot(t, clone)
	if got, err := api.Projects(ctx, a); err != nil || len(got) != 2 {
		t.Fatal(got, err)
	}
	if snapshot(t, clone) != before {
		t.Fatal("read wrote repository")
	}
	// Replaced identity fails closed without substituting the still-available original clone.
	git(t, clone, "remote", "set-url", "origin", "https://github.com/acme/foreign.git")
	if _, err := api.Project(ctx, pa.Project.Key, a); err != domain.ErrConflict {
		t.Fatal(err)
	}
	if out, err := api.Projects(ctx, a); out != nil || err != domain.ErrConflict {
		t.Fatal(out, err)
	}
	if _, _, err := api.AttachProject(ctx, clone, a); err != domain.ErrConflict {
		t.Fatal(err)
	}
	binding, err := store.ActiveClone(ctx, pa.Project.Key)
	if err != nil || binding.Root != clone {
		t.Fatal(binding, err)
	}
	if err := os.RemoveAll(clone); err != nil {
		t.Fatal(err)
	}
	if _, err := api.Project(ctx, pa.Project.Key, a); err != domain.ErrConflict {
		t.Fatal(err)
	}
	if _, _, err := api.AttachProject(ctx, first, a); err != nil {
		t.Fatal(err)
	}
	if _, err := api.Project(ctx, pa.Project.Key, a); err != nil {
		t.Fatal(err)
	}
}

type unavailableResolver struct {
	err           error
	delegate      ports.ExecutableResolver
	calls, failAt int
}

func (r *unavailableResolver) Find(name string) (string, error) {
	r.calls++
	if r.calls == r.failAt {
		return "", r.err
	}
	return r.delegate.Find(name)
}
func TestInspectionPreservesOperationalFailure(t *testing.T) {
	needGit(t)
	dir := repo(t, "https://github.com/acme/a.git")
	for _, failAt := range []int{1, 3, 6} {
		a, _ := newAttacher()
		failure := errors.New("git unavailable")
		a.Resolver = &unavailableResolver{err: failure, delegate: a.Resolver, failAt: failAt}
		if _, err := a.Inspect(context.Background(), dir); !errors.Is(err, failure) || errors.Is(err, domain.ErrInvalid) {
			t.Fatalf("failure at command %d: %v", failAt, err)
		}
		var _ ports.ProjectInspector = a
	}
}
