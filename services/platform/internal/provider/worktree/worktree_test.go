package worktree_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
	"github.com/rluders/amaru/services/platform/internal/provider/worktree"
	"github.com/rluders/amaru/services/platform/internal/worker/osproc"
)

const branch = "amaru/wi-001-x"

var ident = worktree.Identity{Name: "Amaru Worker", Email: "worker@example.com"}

// recorder wraps the OS runner and records every git argument list, so a test can check what was
// asked of git and not only what happened.
type recorder struct {
	inner ports.ProcessRunner
	mu    sync.Mutex
	argv  [][]string
}

func (r *recorder) Start(ctx context.Context, s ports.ProcSpec) (ports.Proc, error) {
	r.mu.Lock()
	r.argv = append(r.argv, append([]string(nil), s.Args...))
	r.mu.Unlock()
	return r.inner.Start(ctx, s)
}

// env is a bare origin, the user's checkout, a worktrees root outside it, and a second clone
// ("someone else") that moves the base or the branch.
type env struct {
	t      *testing.T
	origin string
	repo   string
	other  string
	root   string
	rec    *recorder
	r      *worktree.Repo
}

func needGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not on PATH; the worktree provider is tested against real git")
	}
}

// git runs git for test setup with a fixed identity and no user or system configuration.
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

func write(t *testing.T, p, s string) {
	t.Helper()
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}

func commitAll(t *testing.T, dir, msg string) string {
	t.Helper()
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-q", "-m", msg)
	return git(t, dir, "rev-parse", "HEAD")
}

func newEnv(t *testing.T) *env {
	t.Helper()
	needGit(t)
	tmp := t.TempDir()
	e := &env{
		t: t, origin: filepath.Join(tmp, "origin.git"), repo: filepath.Join(tmp, "code"),
		other: filepath.Join(tmp, "other"), root: filepath.Join(tmp, "worktrees"),
	}
	git(t, tmp, "init", "-q", "--bare", "-b", "main", e.origin)
	if err := os.MkdirAll(e.repo, 0o755); err != nil {
		t.Fatal(err)
	}
	git(t, e.repo, "init", "-q", "-b", "main")
	write(t, filepath.Join(e.repo, "shared.txt"), "one\ntwo\nthree\n")
	commitAll(t, e.repo, "init")
	git(t, e.repo, "remote", "add", "origin", e.origin)
	git(t, e.repo, "push", "-q", "origin", "main")
	git(t, tmp, "clone", "-q", e.origin, e.other)
	e.rec = &recorder{inner: osproc.Runner{}}
	r, err := worktree.New(worktree.Config{
		Repo: e.repo, Root: e.root, Base: "main", Identity: ident,
		Resolver: osproc.Resolver{}, Runner: e.rec,
	})
	if err != nil {
		t.Fatal(err)
	}
	e.r = r
	return e
}

// prepared returns an env with one worktree whose first commit is pushed.
type prep struct {
	*env
	ws      domain.Workspace
	wt      string
	implSHA string
}

func newPrepared(t *testing.T) *prep {
	t.Helper()
	e := newEnv(t)
	ws, err := e.r.Prepare(context.Background(), "WI-001", branch)
	if err != nil {
		t.Fatal(err)
	}
	wt, err := e.r.Dir(ws)
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(wt, "feature.txt"), "feature\n")
	sha := commitAll(t, wt, "WI-001: feature")
	git(t, wt, "push", "-q", "origin", branch)
	return &prep{env: e, ws: ws, wt: wt, implSHA: sha}
}

// moveBase commits a change of file on main from the other clone and pushes it.
func (e *env) moveBase(file, content string) string {
	e.t.Helper()
	git(e.t, e.other, "checkout", "-q", "main")
	git(e.t, e.other, "pull", "-q", "origin", "main")
	write(e.t, filepath.Join(e.other, file), content)
	commitAll(e.t, e.other, "base: "+file)
	git(e.t, e.other, "push", "-q", "origin", "main")
	return git(e.t, e.other, "rev-parse", "HEAD")
}

// theirFix pushes someone else's commit to the branch and returns its id.
func (e *env) theirFix() string {
	e.t.Helper()
	git(e.t, e.other, "fetch", "-q", "origin")
	git(e.t, e.other, "checkout", "-q", "-b", "fix", "origin/"+branch)
	write(e.t, filepath.Join(e.other, "fix.txt"), "fix\n")
	sha := commitAll(e.t, e.other, "a person's fix")
	git(e.t, e.other, "push", "-q", "origin", "fix:"+branch)
	return sha
}

func (e *env) remoteHead() string { return git(e.t, e.origin, "rev-parse", branch) }

// tree lists every file under dir except .git, with its content, to prove nothing was written.
func tree(t *testing.T, dir string) string {
	t.Helper()
	var b strings.Builder
	_ = filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.Name() == ".git" {
			if d != nil && d.IsDir() && d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.IsDir() {
			c, _ := os.ReadFile(p)
			b.WriteString(p + "\x00" + string(c) + "\x00")
		}
		return nil
	})
	return b.String()
}

func TestPrepareCreatesOneWorktreePerWorkItemOutsideTheCheckout(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	before := tree(t, e.repo)
	headBefore := git(t, e.repo, "rev-parse", "HEAD")

	a, err := e.r.Prepare(ctx, "WI-001", "amaru/wi-001")
	if err != nil {
		t.Fatal(err)
	}
	b, err := e.r.Prepare(ctx, "WI-002", "amaru/wi-002")
	if err != nil {
		t.Fatal(err)
	}
	da, _ := e.r.Dir(a)
	db, _ := e.r.Dir(b)
	if da == db || filepath.Dir(da) != e.root || filepath.Dir(db) != e.root {
		t.Fatalf("worktrees at %s and %s, want two directories under %s", da, db, e.root)
	}
	for dir, br := range map[string]string{da: "amaru/wi-001", db: "amaru/wi-002"} {
		if got := git(t, dir, "rev-parse", "--abbrev-ref", "HEAD"); got != br {
			t.Fatalf("%s is on %s, want %s", dir, got, br)
		}
		if got, want := git(t, dir, "rev-parse", "HEAD"), git(t, e.repo, "rev-parse", "origin/main"); got != want {
			t.Fatalf("%s starts at %s, want the base %s", dir, got, want)
		}
		if up := exec.Command("git", "-C", dir, "rev-parse", "--abbrev-ref", "@{upstream}"); up.Run() == nil {
			t.Fatalf("%s tracks an upstream; a new branch must not follow the base", dir)
		}
	}

	// An edit in one worktree is not in the other, and the user's checkout is untouched.
	write(t, filepath.Join(da, "only-a.txt"), "a\n")
	if _, err := os.Stat(filepath.Join(db, "only-a.txt")); !os.IsNotExist(err) {
		t.Fatal("the second worktree sees the first one's file")
	}
	if tree(t, e.repo) != before || git(t, e.repo, "rev-parse", "HEAD") != headBefore {
		t.Fatal("the user's checkout changed")
	}
	if st := git(t, e.repo, "status", "--porcelain"); st != "" {
		t.Fatalf("the user's checkout is dirty: %q", st)
	}
}

func TestPrepareIsIdempotentAndKeepsExistingCommits(t *testing.T) {
	e := newPrepared(t)
	ctx := context.Background()
	again, err := e.r.Prepare(ctx, "WI-001", branch)
	if err != nil || again != e.ws {
		t.Fatalf("second Prepare = %+v, %v", again, err)
	}
	if got := git(t, e.wt, "rev-parse", "HEAD"); got != e.implSHA {
		t.Fatalf("an idempotent Prepare moved the head to %s", got)
	}
	if _, err := e.r.Prepare(ctx, "WI-001", "amaru/other"); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("Prepare on another branch = %v, want ErrConflict", err)
	}

	// The worktree is lost but its branch has a commit: Prepare checks the branch out as it is
	// and does not reset it to the base.
	if err := os.RemoveAll(e.wt); err != nil {
		t.Fatal(err)
	}
	if _, err := e.r.Prepare(ctx, "WI-001", branch); err != nil {
		t.Fatal(err)
	}
	if got := git(t, e.wt, "rev-parse", "HEAD"); got != e.implSHA {
		t.Fatalf("a lost worktree was rebuilt at %s, want its branch at %s", got, e.implSHA)
	}
}

func TestPrepareRefusesWhatCouldLeaveTheRootOrReachGitAsAnOption(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	for _, key := range []string{"", "../x", "a/b", "-x", ".", "..", `a\b`, "a b"} {
		if _, err := e.r.Prepare(ctx, key, "ok"); !errors.Is(err, domain.ErrInvalid) {
			t.Errorf("key %q: %v, want ErrInvalid", key, err)
		}
	}
	for _, br := range []string{"", "-b", "--orphan", "a..b", "x.lock", "a b", "a@{1}", "/x"} {
		if _, err := e.r.Prepare(ctx, "WI-9", br); !errors.Is(err, domain.ErrInvalid) {
			t.Errorf("branch %q: %v, want ErrInvalid", br, err)
		}
	}
	entries, _ := os.ReadDir(e.root)
	if len(entries) != 0 {
		t.Fatalf("a refused Prepare left %d entries in the root", len(entries))
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(e.root), "x")); !os.IsNotExist(err) {
		t.Fatal("a key escaped the worktrees root")
	}
}

func TestNewRefusesAnUnsafeConfiguration(t *testing.T) {
	e := newEnv(t)
	base := worktree.Config{Repo: e.repo, Root: filepath.Join(filepath.Dir(e.repo), "wt2"), Identity: ident,
		Resolver: osproc.Resolver{}, Runner: osproc.Runner{}}
	if _, err := worktree.New(base); err != nil {
		t.Fatalf("a valid configuration: %v", err)
	}
	cases := map[string]func(*worktree.Config){
		"root inside the checkout": func(c *worktree.Config) { c.Root = filepath.Join(c.Repo, ".amaru-wt") },
		"checkout inside the root": func(c *worktree.Config) { c.Root = filepath.Dir(c.Repo) },
		"root is the checkout":     func(c *worktree.Config) { c.Root = c.Repo },
		"relative root":            func(c *worktree.Config) { c.Root = "wt" },
		"relative repository":      func(c *worktree.Config) { c.Repo = "code" },
		"not a repository":         func(c *worktree.Config) { c.Repo = t.TempDir() },
		"no merge identity":        func(c *worktree.Config) { c.Identity = worktree.Identity{} },
		"identity with a newline":  func(c *worktree.Config) { c.Identity.Name = "a\nb" },
		"invalid base":             func(c *worktree.Config) { c.Base = "-x" },
		"no runner":                func(c *worktree.Config) { c.Runner = nil },
	}
	for name, mutate := range cases {
		c := base
		mutate(&c)
		if _, err := worktree.New(c); err == nil {
			t.Errorf("%s: New succeeded, want an error", name)
		}
	}
}

func TestSyncMergesAMovedBaseAndKeepsTheBranchCommits(t *testing.T) {
	e := newPrepared(t)
	ctx := context.Background()
	base := e.moveBase("other.txt", "other\n")

	res, err := e.r.Sync(ctx, e.ws, true)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Merged || !res.Pushed || res.Base != base || res.Before != e.implSHA || res.Head == e.implSHA || res.Remote != e.implSHA {
		t.Fatalf("sync = %+v", res)
	}
	if got := e.remoteHead(); got != res.Head {
		t.Fatalf("remote head = %s, want %s", got, res.Head)
	}
	// The implementation commit and the base are both ancestors of the new head: nothing rewritten.
	for _, c := range []string{e.implSHA, base} {
		git(t, e.wt, "merge-base", "--is-ancestor", c, res.Head)
	}
	if parents := git(t, e.wt, "rev-list", "--parents", "-n", "1", res.Head); len(strings.Fields(parents)) != 3 {
		t.Fatalf("head is not a merge commit: %s", parents)
	}
	if author := git(t, e.wt, "log", "-1", "--format=%an <%ae>", res.Head); author != "Amaru Worker <worker@example.com>" {
		t.Fatalf("merge author = %q, want the configured identity", author)
	}

	// Repeating it changes nothing (idempotent): no second merge commit, no push.
	again, err := e.r.Sync(ctx, e.ws, true)
	if err != nil || again.Merged || again.Pushed || again.Head != res.Head {
		t.Fatalf("second sync = %+v %v", again, err)
	}
}

func TestSyncReportsABehindBranchWithoutMergingWhenNotAsked(t *testing.T) {
	e := newPrepared(t)
	e.moveBase("other.txt", "other\n")
	res, err := e.r.Sync(context.Background(), e.ws, false)
	if err != nil || !res.Behind || res.Merged || res.Pushed || res.Head != e.implSHA {
		t.Fatalf("sync = %+v %v", res, err)
	}
}

func TestSyncWithAnUpToDateBranchDoesNothing(t *testing.T) {
	e := newPrepared(t)
	res, err := e.r.Sync(context.Background(), e.ws, true)
	if err != nil || res.Merged || res.Pushed || res.Behind || res.Head != e.implSHA {
		t.Fatalf("sync = %+v %v", res, err)
	}
}

func TestSyncAbortsAConflictAndLeavesTheBranchAsItWas(t *testing.T) {
	e := newPrepared(t)
	ctx := context.Background()
	write(t, filepath.Join(e.wt, "shared.txt"), "one\nBRANCH\nthree\n")
	sha := commitAll(t, e.wt, "WI-001: edit shared")
	git(t, e.wt, "push", "-q", "origin", branch)
	e.moveBase("shared.txt", "one\nBASE\nthree\n")

	res, err := e.r.Sync(ctx, e.ws, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Conflicts) != 1 || res.Conflicts[0] != "shared.txt" || res.Merged || res.Pushed || res.Head != sha {
		t.Fatalf("sync = %+v", res)
	}
	if got := git(t, e.wt, "rev-parse", "HEAD"); got != sha {
		t.Fatalf("local head moved to %s", got)
	}
	if st := git(t, e.wt, "status", "--porcelain", "--untracked-files=no"); st != "" {
		t.Fatalf("worktree left dirty: %q", st)
	}
	if e.remoteHead() != sha {
		t.Fatal("a conflicting merge was pushed")
	}
	// Deterministic: the same answer again, nothing changed.
	again, err := e.r.Sync(ctx, e.ws, true)
	if err != nil || len(again.Conflicts) != 1 || again.Head != sha {
		t.Fatalf("second sync = %+v %v", again, err)
	}
}

func TestSyncRefusesUncommittedChangesAndKeepsThem(t *testing.T) {
	e := newPrepared(t)
	e.moveBase("other.txt", "other\n")
	write(t, filepath.Join(e.wt, "feature.txt"), "work in progress\n")
	res, err := e.r.Sync(context.Background(), e.ws, true)
	if err != nil || !res.Dirty || res.Merged || res.Pushed {
		t.Fatalf("sync = %+v %v", res, err)
	}
	if b, _ := os.ReadFile(filepath.Join(e.wt, "feature.txt")); string(b) != "work in progress\n" {
		t.Fatalf("uncommitted change lost: %q", b)
	}
}

func TestSyncAdoptsCommitsSomeonePushedToTheBranch(t *testing.T) {
	e := newPrepared(t)
	theirs := e.theirFix()
	res, err := e.r.Sync(context.Background(), e.ws, true)
	if err != nil || !res.Adopted || res.Head != theirs || res.Pushed || res.Merged {
		t.Fatalf("sync = %+v %v", res, err)
	}
	if got := git(t, e.wt, "rev-parse", "HEAD"); got != theirs {
		t.Fatalf("worktree head = %s, want %s", got, theirs)
	}
}

func TestSyncReportsADivergedBranchAndChangesNothing(t *testing.T) {
	e := newPrepared(t)
	theirs := e.theirFix()
	write(t, filepath.Join(e.wt, "local.txt"), "local\n")
	local := commitAll(t, e.wt, "WI-001: local, not pushed")

	res, err := e.r.Sync(context.Background(), e.ws, true)
	if err != nil || !res.Diverged || res.Pushed || res.Head != local {
		t.Fatalf("sync = %+v %v", res, err)
	}
	if e.remoteHead() != theirs {
		t.Fatal("the remote branch was changed")
	}
}

func TestSyncPushesLocalCommitsTheRemoteLacks(t *testing.T) {
	e := newPrepared(t)
	write(t, filepath.Join(e.wt, "more.txt"), "more\n")
	local := commitAll(t, e.wt, "WI-001: committed, push lost in a restart")
	res, err := e.r.Sync(context.Background(), e.ws, true)
	if err != nil || !res.Pushed || res.Head != local || e.remoteHead() != local {
		t.Fatalf("sync = %+v %v", res, err)
	}
}

// A merge left in progress by a Worker that stopped mid-refresh is aborted first.
func TestSyncFinishesAnInterruptedMerge(t *testing.T) {
	e := newPrepared(t)
	base := e.moveBase("other.txt", "other\n")
	git(t, e.wt, "fetch", "-q", "origin", "main")
	git(t, e.wt, "merge", "--no-commit", "--no-ff", "-q", "origin/main")
	if _, err := os.Stat(filepath.Join(git(t, e.wt, "rev-parse", "--absolute-git-dir"), "MERGE_HEAD")); err != nil {
		t.Fatalf("setup: no merge in progress: %v", err)
	}
	res, err := e.r.Sync(context.Background(), e.ws, true)
	if err != nil || !res.Merged || !res.Pushed || res.Base != base {
		t.Fatalf("sync = %+v %v", res, err)
	}
	git(t, e.wt, "merge-base", "--is-ancestor", e.implSHA, res.Head)
}

// The remote branch moves between the fetch and the push: the push is rejected (never forced),
// the local merge commit is undone, and the outcome says so.
func TestSyncUndoesTheMergeWhenThePushRaces(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the race is staged with a shell hook")
	}
	e := newPrepared(t)
	e.moveBase("other.txt", "other\n")
	git(t, e.other, "fetch", "-q", "origin")
	git(t, e.other, "checkout", "-q", "-b", "fix", "origin/"+branch)
	write(t, filepath.Join(e.other, "fix.txt"), "fix\n")
	theirs := commitAll(t, e.other, "a person's fix")
	git(t, e.other, "push", "-q", "origin", "fix:refs/heads/staged")
	// Someone else's commit lands on the branch at the moment of our push.
	hook := filepath.Join(e.origin, "hooks", "pre-receive")
	write(t, hook, "#!/bin/sh\nunset GIT_QUARANTINE_PATH\ngit update-ref refs/heads/"+branch+" "+theirs+"\nexit 1\n")
	if err := os.Chmod(hook, 0o755); err != nil {
		t.Fatal(err)
	}

	res, err := e.r.Sync(context.Background(), e.ws, true)
	if err != nil || !res.Raced || res.Merged || res.Pushed || res.Head != e.implSHA {
		t.Fatalf("sync = %+v %v", res, err)
	}
	if got := git(t, e.wt, "rev-parse", "HEAD"); got != e.implSHA {
		t.Fatalf("local merge not undone: head %s", got)
	}
	if e.remoteHead() != theirs {
		t.Fatal("their commit was overwritten")
	}
	_ = os.Remove(hook)
	// The next wake adopts their commit and merges the base on top: nothing of theirs is lost.
	next, err := e.r.Sync(context.Background(), e.ws, true)
	if err != nil || !next.Adopted || !next.Merged || !next.Pushed {
		t.Fatalf("next sync = %+v %v", next, err)
	}
	git(t, e.wt, "merge-base", "--is-ancestor", theirs, next.Head)
}

func TestHeadAndChanged(t *testing.T) {
	e := newPrepared(t)
	ctx := context.Background()
	write(t, filepath.Join(e.wt, "untracked.txt"), "u\n")
	write(t, filepath.Join(e.wt, "feature.txt"), "edited\n")
	h, err := e.r.Head(ctx, e.ws)
	if err != nil || h.SHA != e.implSHA || !h.TrackedDirty || h.Untracked != 1 {
		t.Fatalf("Head = %+v, %v", h, err)
	}
	files, err := e.r.Changed(ctx, e.ws)
	if err != nil || len(files) != 1 || files[0] != "feature.txt" {
		t.Fatalf("Changed = %v, %v", files, err)
	}
	if _, err := e.r.Head(ctx, domain.Workspace{Key: "WI-404", Branch: "b", Base: "main"}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Head of an unknown worktree = %v, want ErrNotFound", err)
	}
}

func TestRemoveDeletesACleanPushedWorktree(t *testing.T) {
	e := newPrepared(t)
	res, err := e.r.Remove(context.Background(), e.ws, false)
	if err != nil || !res.Removed || res.Discarded || res.Head != e.implSHA {
		t.Fatalf("Remove = %+v, %v", res, err)
	}
	if _, err := os.Stat(e.wt); !os.IsNotExist(err) {
		t.Fatal("the worktree directory is still there")
	}
	if err := exec.Command("git", "-C", e.repo, "rev-parse", "--verify", "-q", "refs/heads/"+branch).Run(); err == nil {
		t.Fatal("the local branch is still there")
	}
	if e.remoteHead() != e.implSHA {
		t.Fatal("removing the worktree changed the remote branch")
	}
	// Already gone is not an error, and removes nothing.
	again, err := e.r.Remove(context.Background(), e.ws, false)
	if err != nil || again.Removed {
		t.Fatalf("second Remove = %+v, %v", again, err)
	}
}

func TestRemoveRefusesToLoseWork(t *testing.T) {
	ctx := context.Background()
	cases := map[string]func(*prep){
		"a modified tracked file": func(e *prep) { write(t, filepath.Join(e.wt, "feature.txt"), "wip\n") },
		"an untracked file":       func(e *prep) { write(t, filepath.Join(e.wt, "new.txt"), "wip\n") },
		"a commit not pushed": func(e *prep) {
			write(t, filepath.Join(e.wt, "more.txt"), "more\n")
			commitAll(t, e.wt, "not pushed")
		},
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			e := newPrepared(t)
			setup(e)
			headBefore := git(t, e.wt, "rev-parse", "HEAD")
			stBefore := git(t, e.wt, "status", "--porcelain")
			_, err := e.r.Remove(ctx, e.ws, false)
			if !errors.Is(err, worktree.ErrUnsafeRemove) {
				t.Fatalf("Remove = %v, want ErrUnsafeRemove", err)
			}
			if git(t, e.wt, "rev-parse", "HEAD") != headBefore || git(t, e.wt, "status", "--porcelain") != stBefore {
				t.Fatal("a refused Remove changed the worktree")
			}
			if err := exec.Command("git", "-C", e.repo, "rev-parse", "--verify", "-q", "refs/heads/"+branch).Run(); err != nil {
				t.Fatal("a refused Remove deleted the branch")
			}
		})
	}
}

func TestRemoveWithDiscardSaysWhatItThrewAway(t *testing.T) {
	e := newPrepared(t)
	write(t, filepath.Join(e.wt, "more.txt"), "more\n")
	local := commitAll(t, e.wt, "not pushed")
	write(t, filepath.Join(e.wt, "feature.txt"), "wip\n")
	write(t, filepath.Join(e.wt, "new.txt"), "wip\n")

	res, err := e.r.Remove(context.Background(), e.ws, true)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Removed || !res.Discarded || res.UncommittedPaths != 2 || res.UnpushedCommits != 1 || res.Head != local {
		t.Fatalf("Remove = %+v", res)
	}
	if _, err := os.Stat(e.wt); !os.IsNotExist(err) {
		t.Fatal("the worktree directory is still there")
	}
	if e.remoteHead() != e.implSHA {
		t.Fatal("discarding changed the remote branch")
	}
}

// The acceptance criterion "merges, never rebases or force-pushes", checked on the arguments the
// provider gave git across the whole lifecycle, including a conflict and a lost race.
func TestNoOperationRebasesOrForcePushes(t *testing.T) {
	e := newPrepared(t)
	ctx := context.Background()
	e.moveBase("other.txt", "other\n")
	if _, err := e.r.Sync(ctx, e.ws, true); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(e.wt, "shared.txt"), "one\nBRANCH\nthree\n")
	commitAll(t, e.wt, "edit shared")
	e.moveBase("shared.txt", "one\nBASE\nthree\n")
	if res, err := e.r.Sync(ctx, e.ws, true); err != nil || len(res.Conflicts) == 0 {
		t.Fatalf("sync = %+v %v", res, err)
	}
	if _, err := e.r.Remove(ctx, e.ws, true); err != nil {
		t.Fatal(err)
	}

	e.rec.mu.Lock()
	defer e.rec.mu.Unlock()
	var pushes int
	for _, args := range e.rec.argv {
		line := strings.Join(args, " ")
		cmd := subcommand(args)
		if cmd == "rebase" || strings.Contains(line, "--rebase") || strings.Contains(line, "pull") {
			t.Errorf("rebase or pull: git %s", line)
		}
		if cmd == "push" {
			pushes++
			for _, a := range args {
				if a == "-f" || strings.HasPrefix(a, "--force") || a == "--mirror" || a == "--delete" || strings.HasPrefix(a, "+") {
					t.Errorf("forced push: git %s", line)
				}
			}
		}
		if cmd == "reset" && !strings.Contains(line, "--keep") {
			t.Errorf("reset without --keep: git %s", line)
		}
	}
	if pushes == 0 {
		t.Fatal("no push was recorded; the check proves nothing")
	}
}

// subcommand returns the git subcommand: the first argument that is not a -c option pair.
func subcommand(args []string) string {
	for i := 0; i < len(args); i++ {
		if args[i] == "-c" {
			i++
			continue
		}
		return args[i]
	}
	return ""
}

// Hooks of the repository must not run with the Worker's authority (fetch, merge and push all
// have hooks), whether the hook lives in the repository or in a worktree.
func TestHooksDoNotRun(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the hooks are shell scripts")
	}
	e := newEnv(t)
	marker := filepath.Join(t.TempDir(), "hook-ran")
	for _, name := range []string{"pre-push", "post-merge", "pre-merge-commit", "post-checkout", "reference-transaction"} {
		hook := filepath.Join(e.repo, ".git", "hooks", name)
		write(t, hook, "#!/bin/sh\necho "+name+" >> "+marker+"\n")
		if err := os.Chmod(hook, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	ctx := context.Background()
	ws, err := e.r.Prepare(ctx, "WI-001", branch)
	if err != nil {
		t.Fatal(err)
	}
	wt, _ := e.r.Dir(ws)
	write(t, filepath.Join(wt, "feature.txt"), "feature\n")
	commitAllNoHooks(t, wt)
	e.moveBase("other.txt", "other\n")
	if res, err := e.r.Sync(ctx, ws, true); err != nil || !res.Pushed {
		t.Fatalf("sync = %+v %v", res, err)
	}
	if _, err := e.r.Remove(ctx, ws, false); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(marker); err == nil {
		t.Fatalf("a hook ran:\n%s", b)
	}
}

// commitAllNoHooks commits as test setup without letting the hooks planted by the test run.
func commitAllNoHooks(t *testing.T, dir string) {
	t.Helper()
	git(t, dir, "add", "-A")
	git(t, dir, "-c", "core.hooksPath="+os.DevNull, "commit", "-q", "-m", "feature")
}

func TestWithoutGit(t *testing.T) {
	e := newEnv(t)
	r, err := worktree.New(worktree.Config{Repo: e.repo, Root: filepath.Join(filepath.Dir(e.repo), "wt3"), Identity: ident,
		Resolver: noGit{}, Runner: osproc.Runner{}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Prepare(context.Background(), "WI-1", "b"); err == nil || !strings.Contains(err.Error(), "git is required") {
		t.Fatalf("Prepare without git = %v", err)
	}
}

type noGit struct{}

func (noGit) Find(string) (string, error) { return "", ports.ErrExecutableNotFound }

// Dir is the writable root handed to the agent, so it must refuse a key that leaves the root.
func TestDirStaysUnderTheRoot(t *testing.T) {
	e := newEnv(t)
	for _, key := range []string{"../x", "a/b", "", "/abs"} {
		if d, err := e.r.Dir(domain.Workspace{Key: key}); !errors.Is(err, domain.ErrInvalid) {
			t.Errorf("Dir(%q) = %q, %v; want ErrInvalid", key, d, err)
		}
	}
	d, err := e.r.Dir(domain.Workspace{Key: "WI-1"})
	if err != nil || filepath.Dir(d) != e.root {
		t.Fatalf("Dir = %q, %v", d, err)
	}
}
