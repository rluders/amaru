package attach_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/rluders/amaru/services/platform/internal/attach"
	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports/fakes"
	"github.com/rluders/amaru/services/platform/internal/worker/osproc"
)

var t0 = time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)

func needGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not on PATH; attach is tested against real git")
	}
}

func newAttacher() (attach.Attacher, *fakes.ProjectStore) {
	st := fakes.NewProjectStore()
	return attach.Attacher{Runner: osproc.Runner{}, Resolver: osproc.Resolver{}, Projects: st, Now: func() time.Time { return t0 }}, st
}

// git runs git in dir for test setup, with a fixed identity and no user config.
func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func repo(t *testing.T, remote string) string {
	t.Helper()
	dir := t.TempDir()
	git(t, dir, "init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "add", ".")
	git(t, dir, "commit", "-q", "-m", "first")
	if remote != "" {
		git(t, dir, "remote", "add", "origin", remote)
	}
	return dir
}

func TestAttachDerivesTheADR0046Key(t *testing.T) {
	needGit(t)
	dir := repo(t, "https://User:tok@GitHub.com/acme/app.git")
	root := git(t, dir, "rev-list", "--max-parents=0", "HEAD")
	a, st := newAttacher()
	p, created, err := a.Attach(context.Background(), dir)
	if err != nil || !created {
		t.Fatalf("attach: %v, created %v", err, created)
	}
	if want := domain.ProjectKey("ssh://github.com/acme/app#" + root); p.Key != want {
		t.Errorf("key = %q, want %q", p.Key, want)
	}
	if got, err := st.Get(context.Background(), p.Key); err != nil || got != p {
		t.Errorf("stored %+v, %v", got, err)
	}
	if p.Name != filepath.Base(dir) {
		t.Errorf("name = %q", p.Name)
	}
}

// The key is stable across path spelling, a subdirectory, a second clone and a different transport.
func TestAttachIsStableAcrossSpellingAndClones(t *testing.T) {
	needGit(t)
	dir := repo(t, "git@github.com:acme/app.git")
	a, st := newAttacher()
	first, created, err := a.Attach(context.Background(), dir)
	if err != nil || !created {
		t.Fatal(err, created)
	}
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	spellings := []string{dir + string(filepath.Separator), filepath.Join(dir, ".", "sub", ".."), sub}
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(dir, link); err == nil {
		spellings = append(spellings, link)
	}
	for _, s := range spellings {
		p, created, err := a.Attach(context.Background(), s)
		if err != nil || created || p.Key != first.Key {
			t.Errorf("attach(%q) = %+v, created %v, %v", s, p, created, err)
		}
	}
	// another clone, over https
	clone := filepath.Join(t.TempDir(), "clone")
	git(t, filepath.Dir(clone), "clone", "-q", dir, clone)
	git(t, clone, "remote", "set-url", "origin", "https://github.com/acme/app")
	p, created, err := a.Attach(context.Background(), clone)
	if err != nil || created || p.Key != first.Key {
		t.Errorf("clone: %+v, created %v, %v", p, created, err)
	}
	if all, _ := st.List(context.Background()); len(all) != 1 {
		t.Errorf("%d projects stored, want 1", len(all))
	}
	// "restart": a new Attacher over the same store
	b := attach.Attacher{Runner: osproc.Runner{}, Resolver: osproc.Resolver{}, Projects: st, Now: func() time.Time { return t0.Add(time.Hour) }}
	if p, created, err := b.Attach(context.Background(), dir); err != nil || created || p.Key != first.Key || !p.CreatedAt.Equal(t0) {
		t.Errorf("after restart: %+v, %v, %v", p, created, err)
	}
}

// snapshot hashes every file and directory entry of dir, .git included, with its mode and mtime.
func snapshot(t *testing.T, dir string) string {
	t.Helper()
	h := sha256.New()
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		fmt.Fprintf(h, "%s %v %d %d\n", p, info.Mode(), info.Size(), info.ModTime().UnixNano())
		if info.Mode().IsRegular() {
			b, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			h.Write(b)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

func TestAttachWritesNothingIntoTheProject(t *testing.T) {
	needGit(t)
	dir := repo(t, "https://github.com/acme/app")
	before := snapshot(t, dir)
	a, _ := newAttacher()
	if _, _, err := a.Attach(context.Background(), dir); err != nil {
		t.Fatal(err)
	}
	if after := snapshot(t, dir); after != before {
		t.Error("attach changed a file, a mode or a time under the Project, .git included")
	}
}

// Attaching an untrusted repository must not start a program the repository names.
func TestAttachDoesNotExecuteProjectContent(t *testing.T) {
	needGit(t)
	if runtime.GOOS == "windows" {
		t.Skip("the probe script is a POSIX shell script")
	}
	dir := repo(t, "https://github.com/acme/app")
	marker := filepath.Join(t.TempDir(), "ran")
	probe := filepath.Join(t.TempDir(), "probe.sh")
	if err := os.WriteFile(probe, []byte("#!/bin/sh\necho ran >> "+marker+"\ncat\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, kv := range [][2]string{
		{"core.fsmonitor", probe},
		{"core.pager", probe},
		{"core.sshCommand", probe},
		{"core.editor", probe},
		{"diff.external", probe},
		{"diff.x.textconv", probe},
		{"filter.x.clean", probe},
		{"filter.x.smudge", probe},
		{"credential.helper", probe},
	} {
		git(t, dir, "config", kv[0], kv[1])
	}
	if err := os.WriteFile(filepath.Join(dir, ".gitattributes"), []byte("* filter=x diff=x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, hook := range []string{"post-checkout", "post-commit", "post-merge", "pre-commit", "reference-transaction", "fsmonitor-watchman"} {
		if err := os.WriteFile(filepath.Join(dir, ".git", "hooks", hook), []byte("#!/bin/sh\necho ran >> "+marker+"\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// a committed config the attach reads as data
	if err := os.MkdirAll(filepath.Join(dir, ".amaru"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := "project:\n  name: probed\n"
	if err := os.WriteFile(filepath.Join(dir, ".amaru", "config.yaml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "-c", "core.fsmonitor=false", "-c", "filter.x.clean=cat", "-c", "filter.x.smudge=cat", "add", ".amaru", ".gitattributes")
	git(t, dir, "-c", "core.fsmonitor=false", "-c", "filter.x.clean=cat", "-c", "filter.x.smudge=cat", "commit", "-q", "--no-verify", "-m", "config")
	_ = os.Remove(marker)

	a, _ := newAttacher()
	p, _, err := a.Attach(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "probed" {
		t.Errorf("committed name not read: %q", p.Name)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("attach started a program named by the repository")
	}
}

func TestAttachWithoutRemoteUsesTheCommittedID(t *testing.T) {
	needGit(t)
	dir := repo(t, "")
	a, _ := newAttacher()
	_, _, err := a.Attach(context.Background(), dir)
	if !errors.Is(err, domain.ErrInvalid) || !strings.Contains(err.Error(), "project.id") {
		t.Fatalf("no remote, no id: %v", err)
	}
	// attach did not write one
	if _, err := os.Stat(filepath.Join(dir, ".amaru")); err == nil {
		t.Fatal("attach wrote into the Project")
	}
	const id = "01J8Z6V2KQ3X9R5T7M1N4P8WAB"
	if err := os.MkdirAll(filepath.Join(dir, ".amaru"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".amaru", "config.yaml"), []byte("project:\n  id: "+id+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// not committed yet: the working tree file is not read
	if _, _, err := a.Attach(context.Background(), dir); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("uncommitted id was used: %v", err)
	}
	git(t, dir, "add", ".amaru")
	git(t, dir, "commit", "-q", "-m", "id")
	p, created, err := a.Attach(context.Background(), dir)
	if err != nil || !created || p.Key != domain.ProjectKey("id:"+id) {
		t.Fatalf("got %+v, %v, %v", p, created, err)
	}
}

func TestAttachRefusals(t *testing.T) {
	needGit(t)
	a, _ := newAttacher()
	ctx := context.Background()

	if _, _, err := a.Attach(ctx, ""); !errors.Is(err, domain.ErrInvalid) {
		t.Errorf("empty path: %v", err)
	}
	if _, _, err := a.Attach(ctx, t.TempDir()); !errors.Is(err, domain.ErrInvalid) {
		t.Errorf("not a repository: %v", err)
	}
	empty := t.TempDir()
	git(t, empty, "init", "-q")
	git(t, empty, "remote", "add", "origin", "https://github.com/acme/app")
	if _, _, err := a.Attach(ctx, empty); !errors.Is(err, domain.ErrInvalid) {
		t.Errorf("no commit: %v", err)
	}
	two := repo(t, "https://github.com/acme/one")
	git(t, two, "remote", "rename", "origin", "first")
	git(t, two, "remote", "add", "second", "https://github.com/acme/two")
	if _, _, err := a.Attach(ctx, two); !errors.Is(err, domain.ErrInvalid) || !strings.Contains(err.Error(), "origin") {
		t.Errorf("two remotes, no origin: %v", err)
	}
	// one remote not named origin is used
	one := repo(t, "https://github.com/acme/one")
	git(t, one, "remote", "rename", "origin", "upstream")
	if _, _, err := a.Attach(ctx, one); err != nil {
		t.Errorf("single non-origin remote: %v", err)
	}
	// a shallow clone cannot know its root commit
	src := repo(t, "https://github.com/acme/app")
	if err := os.WriteFile(filepath.Join(src, "b.txt"), []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, src, "add", ".")
	git(t, src, "commit", "-q", "-m", "second")
	shallow := filepath.Join(t.TempDir(), "s")
	git(t, filepath.Dir(shallow), "clone", "-q", "--depth", "1", "file://"+src, shallow)
	git(t, shallow, "remote", "set-url", "origin", "https://github.com/acme/app")
	if _, _, err := a.Attach(ctx, shallow); !errors.Is(err, domain.ErrInvalid) || !strings.Contains(err.Error(), "shallow") {
		t.Errorf("shallow clone: %v", err)
	}
}

func TestAttachSeveralRootCommitsUsesTheSmallest(t *testing.T) {
	needGit(t)
	dir := repo(t, "https://github.com/acme/app")
	git(t, dir, "checkout", "-q", "--orphan", "other")
	if err := os.WriteFile(filepath.Join(dir, "o.txt"), []byte("o"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "add", "o.txt")
	git(t, dir, "commit", "-q", "-m", "orphan")
	git(t, dir, "checkout", "-q", "main")
	git(t, dir, "merge", "-q", "--allow-unrelated-histories", "-m", "join", "other")
	roots := strings.Fields(git(t, dir, "rev-list", "--max-parents=0", "HEAD"))
	if len(roots) != 2 {
		t.Fatalf("roots = %v", roots)
	}
	want := min(roots[0], roots[1])
	a, _ := newAttacher()
	p, _, err := a.Attach(context.Background(), dir)
	if err != nil || !strings.HasSuffix(string(p.Key), "#"+want) {
		t.Fatalf("key %q, err %v, want root %s", p.Key, err, want)
	}
}

func TestAttachWithoutGit(t *testing.T) {
	a, _ := newAttacher()
	a.Resolver = osproc.Resolver{ExtraDirs: nil}
	t.Setenv("PATH", t.TempDir())
	if _, _, err := a.Attach(context.Background(), t.TempDir()); err == nil || !strings.Contains(err.Error(), "git is required") {
		t.Errorf("err = %v", err)
	}
}
