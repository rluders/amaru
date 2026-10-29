package domain

import (
	"regexp"
	"strings"
)

// Worktree vocabulary for FEAT-ENV-010. The Platform records which Work Item needs a worktree,
// its branch and its base; the Worker creates it and decides where it lives. The Platform never
// stores a Worker's absolute path (ADR-0056 item 6).

// MaxBranchLen bounds a branch name.
const MaxBranchLen = 200

var worktreeKeyRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)

// ValidWorktreeKey reports whether key may name a worktree directory: it starts with a letter or
// digit and holds only letters, digits, "_" and "-", so it cannot leave the worktrees root.
func ValidWorktreeKey(key string) bool { return worktreeKeyRe.MatchString(key) }

// ValidBranch reports whether name is a branch name that is safe to hand to git as an argument.
// It follows `git check-ref-format` and is stricter where an argument could be mistaken for an
// option: a leading "-" is refused. It holds only printable ASCII, so a lookalike or control
// character never reaches git or a log.
func ValidBranch(name string) bool {
	if name == "" || len(name) > MaxBranchLen || name == "@" {
		return false
	}
	if strings.HasPrefix(name, "-") || strings.HasPrefix(name, "/") || strings.HasSuffix(name, "/") ||
		strings.HasSuffix(name, ".") || strings.HasSuffix(name, ".lock") {
		return false
	}
	if strings.Contains(name, "..") || strings.Contains(name, "//") || strings.Contains(name, "@{") {
		return false
	}
	for _, c := range name {
		if c <= ' ' || c >= 0x7f || strings.ContainsRune(`~^:?*[\`, c) {
			return false
		}
	}
	for _, part := range strings.Split(name, "/") {
		if strings.HasPrefix(part, ".") || strings.HasSuffix(part, ".lock") {
			return false
		}
	}
	return true
}

// Workspace identifies the worktree of one Work Item: a key for its directory, the branch it has
// checked out and the base branch it started from. It carries no path.
type Workspace struct {
	Key    string `json:"key"`
	Branch string `json:"branch"`
	Base   string `json:"base"`
}

// Validate checks the key, the branch and the base.
func (w Workspace) Validate() error {
	switch {
	case !ValidWorktreeKey(w.Key):
		return invalid("worktree key %q is not letters, digits, '_' or '-' (up to 64, starting with a letter or digit)", w.Key)
	case !ValidBranch(w.Branch):
		return invalid("branch %q is not a valid branch name", w.Branch)
	case !ValidBranch(w.Base):
		return invalid("base branch %q is not a valid branch name", w.Base)
	}
	return nil
}

// BranchSync is the outcome of a base refresh (ADR-0056, FEAT-ENV-010). The refresh merges the
// base into the branch and pushes without force; it never rebases. Exactly one of Dirty, Diverged,
// Behind, Conflicts, Raced describes why nothing was published, and Pushed says a push happened.
type BranchSync struct {
	// Dirty: the worktree has uncommitted changes to tracked files; nothing was touched.
	Dirty bool `json:"dirty,omitempty"`
	// Adopted: commits someone pushed to the branch were fast-forwarded into the worktree.
	Adopted bool `json:"adopted,omitempty"`
	// Diverged: local and remote branch each have commits the other lacks; nothing changed.
	Diverged bool `json:"diverged,omitempty"`
	// Behind: the base moved and the caller did not ask to merge it.
	Behind bool `json:"behind,omitempty"`
	// Merged: the base was merged into the branch with a merge commit.
	Merged bool `json:"merged,omitempty"`
	// Conflicts lists the files of a merge that was aborted; the branch is as it was.
	Conflicts []string `json:"conflicts,omitempty"`
	// Raced: the remote branch moved between fetch and push; a local merge was undone.
	Raced bool `json:"raced,omitempty"`
	// Pushed: the branch was pushed (never forced).
	Pushed bool `json:"pushed,omitempty"`
	// Before, Base, Remote and Head are commit ids: the branch before the refresh, the fetched
	// base, the fetched remote branch ("" if it has none) and the branch afterwards.
	Before string `json:"before,omitempty"`
	Base   string `json:"base,omitempty"`
	Remote string `json:"remote,omitempty"`
	Head   string `json:"head,omitempty"`
}

// WorktreeHead reports where a worktree stands.
type WorktreeHead struct {
	SHA          string `json:"sha"`
	TrackedDirty bool   `json:"trackedDirty"`
	Untracked    int    `json:"untracked"`
}

// Removal is what removing a worktree did. A removal that discarded work says so, so the caller
// can record it (ADR-0056 item 4).
type Removal struct {
	// Removed is false when there was nothing to remove.
	Removed bool `json:"removed"`
	// Discarded is true when uncommitted changes or unpushed commits were thrown away on purpose.
	Discarded bool `json:"discarded,omitempty"`
	// UncommittedPaths counts changed and untracked paths that were discarded.
	UncommittedPaths int `json:"uncommittedPaths,omitempty"`
	// UnpushedCommits counts commits no remote branch had that were discarded.
	UnpushedCommits int `json:"unpushedCommits,omitempty"`
	// Head is the commit the worktree was at, so discarded commits can be found in the reflog.
	Head string `json:"head,omitempty"`
}
