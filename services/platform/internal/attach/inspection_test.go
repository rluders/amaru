package attach_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rluders/amaru/services/platform/internal/attach"
	"github.com/rluders/amaru/services/platform/internal/domain"
)

func TestInspectReadOnly(t *testing.T) {
	needGit(t)
	ctx := context.Background()
	a, st := newAttacher()
	inspector := attach.Attacher{Runner: a.Runner, Resolver: a.Resolver}
	dir := repo(t, "https://github.com/acme/inspect")
	first, err := inspector.Inspect(ctx, dir)
	if err != nil || first.ConfigPresent || first.Key == "" {
		t.Fatalf("absent: %+v %v", first, err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".amaru"), 0755); err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(dir, ".amaru", "config.yaml")
	if err := os.WriteFile(cfg, []byte("project:\n  name: committed\n"), 0644); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "add", ".")
	git(t, dir, "commit", "-qm", "config")
	if err := os.WriteFile(cfg, []byte("[dirty malformed"), 0644); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0755); err != nil {
		t.Fatal(err)
	}
	before := snapshot(t, dir)
	got, err := inspector.Inspect(ctx, sub)
	canonical, canonicalErr := filepath.EvalSymlinks(dir)
	if err != nil || canonicalErr != nil || got.Root != canonical || got.Key != first.Key || got.Name != "committed" || !got.ConfigPresent {
		t.Fatalf("inspection: %+v %v; canonical %q %v", got, err, canonical, canonicalErr)
	}
	if snapshot(t, dir) != before {
		t.Fatal("inspection wrote to repository")
	}
	if all, err := st.List(ctx); err != nil || len(all) != 0 {
		t.Fatalf("registered: %+v %v", all, err)
	}
}

func TestInspectInvalidRoots(t *testing.T) {
	needGit(t)
	a, _ := newAttacher()
	empty := t.TempDir()
	git(t, empty, "init", "-q")
	bare := t.TempDir()
	git(t, bare, "init", "--bare", "-q")
	dir := repo(t, "https://github.com/acme/inspect")
	for _, root := range []string{"", ".", filepath.Join(t.TempDir(), "missing"), filepath.Join(dir, "a.txt"), t.TempDir(), empty, bare} {
		t.Run(root, func(t *testing.T) {
			if _, err := a.Inspect(context.Background(), root); !errors.Is(err, domain.ErrInvalid) {
				t.Fatalf("%q: %v", root, err)
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := a.Inspect(ctx, dir); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel: %v", err)
	}
}

func TestCommittedConfigInspection(t *testing.T) {
	needGit(t)
	for _, kind := range []string{"empty", "executable", "malformed", "multiple-documents", "oversized", "symlink", "tree"} {
		t.Run(kind, func(t *testing.T) {
			ctx := context.Background()
			a, st := newAttacher()
			dir := repo(t, "https://github.com/acme/"+kind)
			original, _, err := a.Attach(ctx, dir)
			if err != nil {
				t.Fatal(err)
			}
			clone := filepath.Join(t.TempDir(), "clone")
			git(t, filepath.Dir(clone), "clone", "-q", dir, clone)
			git(t, clone, "remote", "set-url", "origin", "https://github.com/acme/"+kind)
			if err := os.MkdirAll(filepath.Join(clone, ".amaru"), 0755); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(clone, ".amaru", "config.yaml")
			switch kind {
			case "symlink":
				if err := os.Symlink("../../a.txt", path); err != nil {
					t.Skipf("symlink unavailable: %v", err)
				}
			case "tree":
				if err := os.Mkdir(path, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(path, "entry"), []byte("x"), 0644); err != nil {
					t.Fatal(err)
				}
			default:
				content := ""
				if kind == "malformed" {
					content = "project: ["
				}
				if kind == "oversized" {
					content = strings.Repeat("# padding\n", 120000)
				}
				if kind == "multiple-documents" {
					content = "project: {}\n---\nproject: {}\n"
				}
				if err := os.WriteFile(path, []byte(content), 0644); err != nil {
					t.Fatal(err)
				}
			}
			git(t, clone, "add", ".")
			if kind == "executable" {
				git(t, clone, "update-index", "--chmod=+x", ".amaru/config.yaml")
			}
			git(t, clone, "commit", "-qm", "config")
			got, err := a.Inspect(ctx, clone)
			if kind == "empty" || kind == "executable" {
				if err != nil || !got.ConfigPresent {
					t.Fatalf("regular config: %+v %v", got, err)
				}
			} else {
				if !errors.Is(err, domain.ErrInvalid) {
					t.Fatalf("unsafe inspection: %v", err)
				}
				if _, _, err := a.Attach(ctx, clone); !errors.Is(err, domain.ErrInvalid) {
					t.Fatalf("unsafe attach: %v", err)
				}
			}
			selected, err := st.ActiveClone(ctx, original.Key)
			if err != nil || selected.Root != dir {
				t.Fatalf("selection changed: %+v %v", selected, err)
			}
			if all, err := st.List(ctx); err != nil || len(all) != 1 || all[0] != original {
				t.Fatalf("metadata changed: %+v %v", all, err)
			}
		})
	}
}
