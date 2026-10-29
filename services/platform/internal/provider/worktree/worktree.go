// Package worktree isolates each Work Item in its own git worktree (FEAT-ENV-010). It runs the git
// CLI through the process port of the Worker OS layer (ADR-0053), with argument arrays and an
// explicit working directory, never through a shell.
//
// A Repo manages the worktrees of one repository under a Worker-configured root that lies outside
// the repository's checkout (ADR-0056 item 6): the Project folder stays free of operational state
// (ADR-0043), and the Platform records only the Work Item, branch and base, never a path.
//
// The base refresh (Sync) merges the base into the branch and pushes without force. No operation
// here rebases, and none pushes a forced ref. Removing a worktree refuses to discard uncommitted
// changes or commits no remote branch holds unless the caller says so, and then reports what was
// discarded.
//
// This package decides where a worktree is; it does not confine the agent that runs in it. A git
// worktree is not a sandbox. Confining writes to Dir is the Worker's job (ADR-0053).
//
// Trust: git reads the repository's own configuration, so a Project must be trusted
// (FEAT-PRJ-003) before a Repo is opened on it. Hooks are switched off for every command here.
//
// Adapted from the earlier internal implementation's provider/gitrepo at commit c8a2f6e (ADR-0056);
// see PROVENANCE.md. Push, Commit, the scratch rules and Remote were not moved.
package worktree

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
)

const remote = "origin"

// ErrUnsafeRemove is returned by Remove when the worktree holds work that removal would lose.
var ErrUnsafeRemove = errors.New("worktree holds work that is not on a remote branch")

// Identity authors the merge commits of a base refresh. It is configured, never defaulted.
type Identity struct {
	Name  string
	Email string
}

// Config opens a Repo.
type Config struct {
	// Repo is the absolute top level of the repository (a Project checkout).
	Repo string
	// Root is the absolute directory that holds the worktrees, one subdirectory per key. It must
	// lie outside Repo and Repo outside it. It is created if missing.
	Root string
	// Base is the branch worktrees start from and refresh from. Empty means "main".
	Base     string
	Identity Identity
	// Env is added to the OS layer's allow-listed environment, for example SSH_AUTH_SOCK so a
	// push can authenticate. It is never logged.
	Env      []string
	Resolver ports.ExecutableResolver
	Runner   ports.ProcessRunner
}

// Repo manages the worktrees of one repository. It runs one git operation at a time.
type Repo struct {
	dir      string
	root     string
	base     string
	identity Identity
	env      []string
	res      ports.ExecutableResolver
	run      ports.ProcessRunner

	mu sync.Mutex
}

// New validates the configuration and opens the repository. It runs no git command.
func New(cfg Config) (*Repo, error) {
	if cfg.Resolver == nil || cfg.Runner == nil {
		return nil, errors.New("worktree: a resolver and a runner are required")
	}
	base := cfg.Base
	if base == "" {
		base = "main"
	}
	if !domain.ValidBranch(base) {
		return nil, fmt.Errorf("worktree: invalid base branch %q", base)
	}
	if cfg.Identity.Name == "" || cfg.Identity.Email == "" ||
		strings.ContainsAny(cfg.Identity.Name+cfg.Identity.Email, "\r\n\x00") {
		return nil, errors.New("worktree: a merge identity (name and email) is required")
	}
	if !filepath.IsAbs(cfg.Repo) || !filepath.IsAbs(cfg.Root) {
		return nil, errors.New("worktree: the repository and the worktrees root must be absolute paths")
	}
	if _, err := os.Stat(filepath.Join(cfg.Repo, ".git")); err != nil {
		return nil, fmt.Errorf("worktree: %s is not the top level of a git repository", cfg.Repo)
	}
	if err := os.MkdirAll(cfg.Root, 0o700); err != nil {
		return nil, fmt.Errorf("worktree: worktrees root: %w", err)
	}
	repo, err := filepath.EvalSymlinks(cfg.Repo)
	if err != nil {
		return nil, fmt.Errorf("worktree: repository: %w", err)
	}
	root, err := filepath.EvalSymlinks(cfg.Root)
	if err != nil {
		return nil, fmt.Errorf("worktree: worktrees root: %w", err)
	}
	if within(repo, root) || within(root, repo) {
		return nil, fmt.Errorf("worktree: the worktrees root %s and the repository %s must not contain one another", root, repo)
	}
	return &Repo{
		dir: repo, root: root, base: base, identity: cfg.Identity,
		env: slices.Clone(cfg.Env), res: cfg.Resolver, run: cfg.Runner,
	}, nil
}

// within reports whether path is dir or lies under it.
func within(dir, path string) bool {
	rel, err := filepath.Rel(dir, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

// Base is the base branch worktrees start from.
func (r *Repo) Base() string { return r.base }

// Dir is the directory of a Work Item's worktree: the one writable root to give the agent.
func (r *Repo) Dir(ws domain.Workspace) (string, error) {
	if !domain.ValidWorktreeKey(ws.Key) {
		return "", fmt.Errorf("%w: worktree key %q is not valid", domain.ErrInvalid, ws.Key)
	}
	return filepath.Join(r.root, ws.Key), nil
}

// Checkout is the Project checkout. A process runner is told to keep it out of every writable
// root (osproc.Runner.Forbidden), so no agent can be given write access to it.
func (r *Repo) Checkout() string { return r.dir }

// Confinement is the write restriction for an agent working in ws: its worktree is the only
// writable root (ADR-0056 item 7, ADR-0057). It does not let the agent commit: git writes to the
// shared repository under the Project checkout, which stays outside the root, so the Worker
// commits and pushes on the agent's behalf.
func (r *Repo) Confinement(ws domain.Workspace) (ports.Confinement, error) {
	dir, err := r.Dir(ws)
	if err != nil {
		return ports.Confinement{}, err
	}
	return ports.Confinement{WritableRoots: []string{dir}}, nil
}

// ExitError is a git command that exited non-zero.
type ExitError struct {
	Code   int
	Cmd    string
	Stderr string
}

func (e *ExitError) Error() string {
	return fmt.Sprintf("git %s exited %d: %s", e.Cmd, e.Code, e.Stderr)
}

const (
	maxStdout = 4 << 20
	maxStderr = 64 << 10
)

func (r *Repo) hardened() []string {
	// Hooks, the pager and the file-system monitor are off for every command: a worktree holds
	// what an agent wrote, and its hooks must not run with the Worker's authority.
	return []string{"-c", "core.hooksPath=" + os.DevNull, "-c", "core.fsmonitor=false", "-c", "core.pager=cat", "-c", "protocol.ext.allow=never"}
}

func (r *Repo) git(ctx context.Context, dir string, args ...string) (string, error) {
	exe, err := r.res.Find("git")
	if err != nil {
		return "", fmt.Errorf("git is required for worktrees: %w", err)
	}
	env := append([]string{"GIT_TERMINAL_PROMPT=0", "GIT_CONFIG_NOSYSTEM=1", "LC_ALL=C"}, r.env...)
	proc, err := r.run.Start(ctx, ports.ProcSpec{Path: exe, Args: append(r.hardened(), args...), Dir: dir, Env: env})
	if err != nil {
		return "", fmt.Errorf("git %s: %w", args[0], err)
	}
	var out, errb bytes.Buffer
	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(&errb, io.LimitReader(proc.Stderr(), maxStderr))
		_, _ = io.Copy(io.Discard, proc.Stderr())
		close(done)
	}()
	_, _ = io.Copy(&out, io.LimitReader(proc.Stdout(), maxStdout))
	_, _ = io.Copy(io.Discard, proc.Stdout())
	<-done
	code, err := proc.Wait()
	if err != nil {
		return "", fmt.Errorf("git %s: %w", args[0], err)
	}
	if code != 0 {
		return out.String(), &ExitError{Code: code, Cmd: args[0], Stderr: strings.TrimSpace(errb.String())}
	}
	return out.String(), nil
}

func exitCode(err error) (int, bool) {
	var ee *ExitError
	if errors.As(err, &ee) {
		return ee.Code, true
	}
	return 0, false
}

// Prepare creates the worktree of a Work Item on branch, started from the remote base, or returns
// the existing one. It is idempotent, so a restart or a replayed command does the same thing. A
// local branch that already exists is checked out as it is, never reset to the base, so commits
// made before a worktree was lost are not overwritten.
func (r *Repo) Prepare(ctx context.Context, key, branch string) (domain.Workspace, error) {
	ws := domain.Workspace{Key: key, Branch: branch, Base: r.base}
	if err := ws.Validate(); err != nil {
		return domain.Workspace{}, err
	}
	dir, _ := r.Dir(ws)
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, err := os.Lstat(dir); err == nil {
		if _, err := os.Lstat(filepath.Join(dir, ".git")); err != nil {
			return domain.Workspace{}, fmt.Errorf("%w: %s exists and is not a worktree", domain.ErrConflict, key)
		}
		head, err := r.git(ctx, dir, "rev-parse", "--abbrev-ref", "HEAD")
		if err == nil && strings.TrimSpace(head) == branch {
			return ws, nil
		}
		return domain.Workspace{}, fmt.Errorf("%w: worktree %s exists but is not on branch %s", domain.ErrConflict, key, branch)
	}
	if _, err := r.git(ctx, r.dir, "fetch", remote, "+refs/heads/"+r.base+":refs/remotes/"+remote+"/"+r.base); err != nil {
		return domain.Workspace{}, err
	}
	_, _ = r.git(ctx, r.dir, "worktree", "prune")
	_, err := r.git(ctx, r.dir, "rev-parse", "--verify", "-q", "refs/heads/"+branch)
	switch code, isExit := exitCode(err); {
	case err == nil:
		_, err = r.git(ctx, r.dir, "worktree", "add", "--", dir, branch)
	case isExit && code == 1:
		_, err = r.git(ctx, r.dir, "worktree", "add", "--no-track", "-b", branch, "--", dir, remote+"/"+r.base)
	}
	if err != nil {
		return domain.Workspace{}, err
	}
	return ws, nil
}

// existing returns the directory of a prepared worktree.
func (r *Repo) existing(ws domain.Workspace) (string, error) {
	if err := ws.Validate(); err != nil {
		return "", err
	}
	dir, _ := r.Dir(ws)
	if _, err := os.Lstat(filepath.Join(dir, ".git")); err != nil {
		return "", fmt.Errorf("%w: worktree %s", domain.ErrNotFound, ws.Key)
	}
	return dir, nil
}

type statusEntry struct {
	x, y byte
	path string
}

// status lists a worktree's changes, untracked files one by one. Ignored files are not listed:
// .gitignore decides those.
func (r *Repo) status(ctx context.Context, dir string) ([]statusEntry, error) {
	out, err := r.git(ctx, dir, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if err != nil {
		return nil, err
	}
	var es []statusEntry
	parts := strings.Split(out, "\x00")
	for i := 0; i < len(parts); i++ {
		p := parts[i]
		if len(p) < 4 {
			continue
		}
		e := statusEntry{x: p[0], y: p[1], path: p[3:]}
		if e.x == 'R' || e.x == 'C' || e.y == 'R' || e.y == 'C' {
			i++ // the next field is the original path
		}
		es = append(es, e)
	}
	return es, nil
}

// Head reports where a worktree stands: its commit, whether tracked files have uncommitted
// changes, and how many untracked files it holds.
func (r *Repo) Head(ctx context.Context, ws domain.Workspace) (domain.WorktreeHead, error) {
	var h domain.WorktreeHead
	dir, err := r.existing(ws)
	if err != nil {
		return h, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	sha, err := r.git(ctx, dir, "rev-parse", "HEAD")
	if err != nil {
		return h, err
	}
	es, err := r.status(ctx, dir)
	if err != nil {
		return h, err
	}
	for _, e := range es {
		if e.x == '?' {
			h.Untracked++
		} else {
			h.TrackedDirty = true
		}
	}
	h.SHA = strings.TrimSpace(sha)
	return h, nil
}

// Changed lists the files the branch changes against the base.
func (r *Repo) Changed(ctx context.Context, ws domain.Workspace) ([]string, error) {
	dir, err := r.existing(ws)
	if err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	out, err := r.git(ctx, dir, "diff", "--name-only", remote+"/"+r.base+"...HEAD")
	if err != nil {
		return nil, err
	}
	return lines(out), nil
}

func lines(s string) []string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	return out
}

// Remove deletes a worktree and its local branch. It refuses, with ErrUnsafeRemove, a worktree
// with uncommitted changes or with commits no remote branch holds, unless discard is true. With
// discard it removes anyway and the Removal says what was thrown away, for the caller to record.
// Ignored files are not counted as work. A worktree that is already gone is not an error.
func (r *Repo) Remove(ctx context.Context, ws domain.Workspace, discard bool) (domain.Removal, error) {
	if err := ws.Validate(); err != nil {
		return domain.Removal{}, err
	}
	dir, _ := r.Dir(ws)
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, err := os.Lstat(dir); err != nil {
		_, _ = r.git(ctx, r.dir, "worktree", "prune")
		return domain.Removal{}, nil
	}
	if _, err := os.Lstat(filepath.Join(dir, ".git")); err != nil {
		return domain.Removal{}, fmt.Errorf("%w: %s exists and is not a worktree", domain.ErrConflict, ws.Key)
	}
	head, err := r.git(ctx, dir, "rev-parse", "HEAD")
	if err != nil {
		return domain.Removal{}, err
	}
	es, err := r.status(ctx, dir)
	if err != nil {
		return domain.Removal{}, err
	}
	n, err := r.git(ctx, dir, "rev-list", "--count", "HEAD", "--not", "--remotes="+remote)
	if err != nil {
		return domain.Removal{}, err
	}
	var unpushed int
	if _, err := fmt.Sscanf(strings.TrimSpace(n), "%d", &unpushed); err != nil {
		return domain.Removal{}, fmt.Errorf("git rev-list: unexpected count %q", n)
	}
	res := domain.Removal{Head: strings.TrimSpace(head)}
	if len(es) > 0 || unpushed > 0 {
		if !discard {
			return domain.Removal{}, fmt.Errorf("%w: %d uncommitted path(s), %d commit(s) not pushed; pass discard to remove it anyway",
				ErrUnsafeRemove, len(es), unpushed)
		}
		res.Discarded, res.UncommittedPaths, res.UnpushedCommits = true, len(es), unpushed
	}
	args := []string{"worktree", "remove"}
	if len(es) > 0 {
		args = append(args, "--force")
	}
	if _, err := r.git(ctx, r.dir, append(args, "--", dir)...); err != nil {
		return domain.Removal{}, err
	}
	_, _ = r.git(ctx, r.dir, "worktree", "prune")
	// The branch holds nothing that is not on a remote, or the caller chose to discard it.
	_, _ = r.git(ctx, r.dir, "branch", "-D", "--", ws.Branch)
	res.Removed = true
	return res, nil
}

// Sync is the base refresh (FEAT-ENV-010, WO ADR-028): it brings the branch in line with its
// remote branch and, with mergeBase, with the base. Order: finish nothing half done, refuse local
// edits, fetch, adopt remote commits (fast-forward only), merge the base with a merge commit, push
// without force. It never rebases. A conflicting merge is aborted and the branch is as it was.
func (r *Repo) Sync(ctx context.Context, ws domain.Workspace, mergeBase bool) (domain.BranchSync, error) {
	var res domain.BranchSync
	dir, err := r.existing(ws)
	if err != nil {
		return res, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	// A merge left in progress (the Worker stopped between merge and abort) is ours: the worktree
	// had no local edits when it started, so aborting it loses nothing.
	if _, err := r.git(ctx, dir, "rev-parse", "-q", "--verify", "MERGE_HEAD"); err == nil {
		if _, err := r.git(ctx, dir, "merge", "--abort"); err != nil {
			return res, err
		}
	}
	st, err := r.git(ctx, dir, "status", "--porcelain", "--untracked-files=no")
	if err != nil {
		return res, err
	}
	if strings.TrimSpace(st) != "" {
		res.Dirty = true
		return res, nil
	}
	if _, err := r.git(ctx, dir, "fetch", remote, "+refs/heads/"+r.base+":refs/remotes/"+remote+"/"+r.base); err != nil {
		return res, err
	}
	if res.Remote, err = r.fetchBranch(ctx, dir, ws.Branch); err != nil {
		return res, err
	}
	if res.Before, err = r.rev(ctx, dir, "HEAD"); err != nil {
		return res, err
	}
	if res.Base, err = r.rev(ctx, dir, remote+"/"+r.base); err != nil {
		return res, err
	}
	if res.Remote != "" && res.Remote != res.Before {
		behind, err := r.ancestor(ctx, dir, res.Before, res.Remote)
		if err != nil {
			return res, err
		}
		ahead, err := r.ancestor(ctx, dir, res.Remote, res.Before)
		if err != nil {
			return res, err
		}
		switch {
		case behind:
			if _, err := r.git(ctx, dir, "merge", "--ff-only", "-q", res.Remote); err != nil {
				return res, err
			}
			res.Adopted = true
		case !ahead:
			res.Diverged, res.Head = true, res.Before
			return res, nil
		}
	}
	preMerge, err := r.rev(ctx, dir, "HEAD")
	if err != nil {
		return res, err
	}
	upToDate, err := r.ancestor(ctx, dir, res.Base, preMerge)
	if err != nil {
		return res, err
	}
	if !upToDate && !mergeBase {
		res.Behind = true
	}
	if !upToDate && mergeBase {
		msg := fmt.Sprintf("Merge %s into %s\n\nBase refresh (base %s).", r.base, ws.Branch, res.Base)
		if _, err := r.git(ctx, dir, "-c", "user.name="+r.identity.Name, "-c", "user.email="+r.identity.Email,
			"merge", "--no-ff", "--no-verify", "--no-edit", "-q", "-m", msg, res.Base); err != nil {
			out, _ := r.git(ctx, dir, "diff", "--name-only", "--diff-filter=U")
			if _, aerr := r.git(ctx, dir, "merge", "--abort"); aerr != nil {
				return res, fmt.Errorf("%w; abort: %v", err, aerr)
			}
			res.Conflicts = lines(out)
			if len(res.Conflicts) == 0 {
				return res, err
			}
			res.Head = preMerge
			return res, nil
		}
		res.Merged = true
	}
	if res.Head, err = r.rev(ctx, dir, "HEAD"); err != nil {
		return res, err
	}
	if res.Head == res.Remote {
		return res, nil
	}
	// Never forced: the head descends from the fetched remote branch, so only a concurrent push
	// to that branch can make this fail.
	if _, perr := r.git(ctx, dir, "push", "-q", remote, "HEAD:refs/heads/"+ws.Branch); perr != nil {
		if res.Merged {
			if _, err := r.git(ctx, dir, "reset", "-q", "--keep", preMerge); err != nil {
				return res, fmt.Errorf("%w; undo merge: %v", perr, err)
			}
			res.Merged, res.Head = false, preMerge
		}
		now, err := r.fetchBranch(ctx, dir, ws.Branch)
		if err == nil && now != res.Remote {
			res.Raced = true
			return res, nil
		}
		return res, perr
	}
	res.Pushed = true
	return res, nil
}

// fetchBranch updates origin/<branch> and returns its commit, or "" when the remote has no such
// branch.
func (r *Repo) fetchBranch(ctx context.Context, dir, branch string) (string, error) {
	ref := "refs/remotes/" + remote + "/" + branch
	if _, err := r.git(ctx, dir, "fetch", remote, "+refs/heads/"+branch+":"+ref); err != nil {
		var ee *ExitError
		if errors.As(err, &ee) && strings.Contains(ee.Stderr, "couldn't find remote ref") {
			return "", nil
		}
		return "", err
	}
	return r.rev(ctx, dir, ref)
}

func (r *Repo) rev(ctx context.Context, dir, ref string) (string, error) {
	out, err := r.git(ctx, dir, "rev-parse", "--verify", "-q", ref+"^{commit}")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// ancestor reports whether commit a is an ancestor of (or equal to) commit b.
func (r *Repo) ancestor(ctx context.Context, dir, a, b string) (bool, error) {
	_, err := r.git(ctx, dir, "merge-base", "--is-ancestor", a, b)
	if err == nil {
		return true, nil
	}
	if code, ok := exitCode(err); ok && code == 1 {
		return false, nil
	}
	return false, err
}
