---
title: Adapt the prototype git worktree provider into the Worker providers, without its commit-scratch and force-push parts
status: proposed
date: 2026-10-28
deciders:
---

# 0056. Adapt the prototype git worktree provider into the Worker providers, without its commit-scratch and force-push parts

## Context
- [ADR-0007](0007-reuse-of-earlier-internal-code-is-a-separate-activity.md) bars moving code from the earlier internal implementation until a human approves a migration ADR for that component. This is the draft migration ADR for the `provider/gitrepo` package of its Runtime (`runtime/internal/provider/gitrepo`; `MADR-provider-gitrepo`, issue #85, sub-issue of WP-M3-07 / #19). Every issue that moves this component is gated on it ([delivery §8](../../engineering/delivery.md#8-human-approval-gates)); in M3 that is FEAT-ENV-010 (#154, worktree isolation per Work Item).
- **Source (read 2026-10-28):** the Runtime repository of the earlier internal implementation (named in the private migration notes, not part of this repository), baseline commit **`c8a2f6e`** ([ADR-0032](0032-migration-baseline-is-a-pinned-commit.md)). The package directory has tree id `2627f8560e3aac78dd9abb1fca048d715800ada0` and four files: `gitrepo.go` (534 lines) and three test files, `gitrepo_test.go` (175), `scratch_test.go` (219) and `sync_test.go` (317). It imports the standard library, the prototype `domain` and `ports` (`Workspace`, `CommitOptions`, `BranchSync`, `ExecutableResolver`, `ProcessRunner`, `CleanRelPath`, `ValidBranch`, `ScratchReason`, `MatchesInclude`).
- **Not verified here:** the package tests were **not run** for this draft. The attempt to run them against the extracted tree was refused by the session's permission classifier (it runs external code), and I did not work around that. The tests use real `git` and a local bare remote, so they are runnable by the owner (`go test -count=1 ./internal/provider/gitrepo/...` in the Runtime at `c8a2f6e`); readiness criterion 2 below is therefore **not met** until someone records that run.
- **Planned disposition** (migration plan, `MADR-provider-gitrepo`): **Adapt** into the Worker providers (worktrees).
- **What the package does** (read from the source):
  - `Prepare` creates one worktree per Work Item under `<project>/<worktrees dir>/<key>` with `git worktree add -B <branch> <dir> origin/<base>` after a fetch, and is idempotent for a worktree that already exists on the branch. The key is checked against `^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$` and the branch with `ValidBranch`. Git runs with argv arrays and an explicit working directory, one operation at a time per repository.
  - `Sync` is the base refresh: it refuses local edits, fetches, adopts commits someone pushed to the branch **fast-forward only**, merges the base with `merge --no-ff` (aborting and reporting the conflicting files on a conflict), pushes **without force**, and undoes its merge if the push lost a race. It reports `Dirty`, `Behind`, `Diverged`, `Conflicts`, `Raced` and `Pushed`. This is the "merges, never rebases or force-pushes" criterion of FEAT-ENV-010, and the strongest tested behavior in the package (nine tests).
  - `Head`, `Changed`, `Remove` (worktree remove, prune, delete the local branch) and `Remote` (the `owner/name` of `origin` on GitHub).
  - `Commit` builds the change set itself: tracked changes are always staged, new files are staged unless a scratch rule (`ScratchReason`) excludes them, scratch the agent already committed is taken out of the tree in this commit, and a fixed author is used. `Push` uses `--force-with-lease`.
- **Things in the source that conflict with Amaru or need a decision before they move.**
  - **`Push` forces.** `--force-with-lease` contradicts "never force-pushes". `Sync` does not force, and only `Sync` is in the acceptance criteria.
  - **`Remove` forces.** `worktree remove --force` and `branch -D` discard uncommitted work in the worktree. Removing a worktree that has unmerged commits or uncommitted changes needs an explicit rule, not a silent force.
  - **Scratch and commit policy.** `Commit`, `ScratchReason` and `MatchesInclude` encode the delivery loop's idea of what an agent may commit (`.weave` paths, `generated/<workflow>/exe_<id>/`). That is delivery-loop behavior (FEAT-WRK-004, M4), tied to prototype paths, and is out of scope for FEAT-ENV-010.
  - **Identity and paths.** The author `Weave Autoloop <autoloop@weave.invalid>` and the worktree location under `.weave/runtime/` are prototype names. Amaru's operational state is not in the Project folder ([ADR-0043](0043-project-definitions-in-git-teams-and-operational-state-in-platform.md)); where a Worker puts a worktree is a decision (below).
  - **Confinement is not here.** "Agents write only inside it" is enforced by the Worker's path and process confinement ([ADR-0053](0053-adapt-the-prototype-process-and-path-code-into-the-worker-os-layer.md)), not by git. The package only decides the directory. A git worktree is not a sandbox: the process can still write elsewhere unless the Worker confines it.
  - **Remote URL handling.** `Remote()` assumes a GitHub origin. Project keys normalize remotes to `ssh://` ([ADR-0046](0046-project-key-is-the-normalized-remote-plus-root-commit.md) amendment, owner decision 2026-10-27); pushes authenticate through the user's ssh agent via passed environment variables, which the Worker must pass on explicitly and never log.
  - **Environment.** The package gets its `git` binary and process runner from the prototype's `ExecutableResolver` and `ProcessRunner`. In Amaru these are the Worker OS layer of ADR-0053, so the adapted code depends on that layer and not on the prototype ports.
- **Placement constraint.** The core imports no OS or process package (`internal/arch`). The worktree provider runs `git`, so it lives in the Worker providers and depends only on `ports` and `domain`. The Platform decides that a Work Item needs a worktree and records its branch and base; the Worker creates and removes it ([ADR-0013](0013-outbound-pull-based-workers-with-leases.md)).
- **Unverified:** behavior on macOS and Windows (Windows path rules and `git worktree` on network drives); that `Sync` handles every history shape (submodules, shallow clones, LFS are not covered by the source tests); a Project whose repository is not at the Project root (the source supports it through `repoRel`).

## Decision
1. **Component and source.** Migrate `gitrepo.go` and its tests at commit `c8a2f6e` (ADR-0032) for the parts listed in item 4. No later commit is used.
2. **Disposition: Adapt, with parts not moved.** The prototype is a behavioral reference and migration source. `services/platform` is canonical from the first commit; the prototype is not kept in sync.
3. **History option: fresh import with provenance** (migration plan default), as in ADR-0049 and ADR-0054: the source commit and tree id are recorded here, in `services/platform/README.md` and in `PROVENANCE.md` with each file's source path and blob id. No `git subtree` or `filter-repo`.
4. **File and function disposition.**

   | Source (`internal/provider/gitrepo/`) | Disposition | Target / note |
   |---|---|---|
   | `Prepare`, key and branch checks, `abs` | Adapt | One worktree per Work Item, idempotent. The location is configured by the Worker (see item 6), not a Project-relative prototype path. `ValidBranch` and the key check are re-expressed in the domain |
   | `Sync` and `fetchBranch`, `rev`, `ancestor` | Adapt | The base refresh. Kept: refuse local edits, fast-forward adoption only, `merge --no-ff`, abort on conflict with the file list, push without force, undo on a lost race. **Added:** a test that no code path rebases or force-pushes |
   | `Head`, `Changed` | Adapt | Read-only status of a worktree |
   | `Remove` | Adapt, **with a fix** | Refuses a worktree with uncommitted changes or commits not on the remote unless the caller passes an explicit discard, which is recorded. No unconditional `--force` or `branch -D` |
   | `Push` | **Not moved** | `--force-with-lease` contradicts the acceptance criterion. A non-force push is already part of `Sync`; pushing for delivery is M4 |
   | `Commit`, `statusEntry`, `status`, scratch and include rules | **Not moved** | Delivery-loop behavior (M4, FEAT-WRK-004) tied to prototype paths. Needs its own decision when the delivery loop starts. `scratch_test.go` stays with it |
   | `Remote` | **Not moved** | GitHub-specific; remote identity is the Project key (ADR-0046) |
   | Author name and email constants | **Not moved** | Replaced by a configured identity; no prototype name carried |
   | `gitrepo_test.go` (2 tests), `sync_test.go` (9 tests) | Adapt | See item 5 |
   | Source ADRs (021, 026, 028), dogfood records | Reference only | cited, not copied |

5. **Tests that move.** `sync_test.go` is the specification of the base refresh and `gitrepo_test.go` of the worktree lifecycle. They run with a real `git` and a local bare remote in a temporary directory, so they need `git` on the PATH and are skipped, with a recorded reason, where it is missing. **New**, listed in `PROVENANCE.md`: `Remove` refuses a dirty worktree and one with unpushed commits; no operation rebases or force-pushes (checked on the recorded argv); a key or branch that tries to leave the worktrees directory is refused; two Work Items get two worktrees that do not share a working directory.
6. **Where worktrees live (requires human decision).** The prototype puts them inside the Project folder. Amaru keeps operational state out of the Project ([ADR-0043](0043-project-definitions-in-git-teams-and-operational-state-in-platform.md)), but a worktree must be reachable by the agent and, for most repositories, near the repository. Proposed: the Worker has a configured worktrees root outside the Project checkout (default under its own state directory), and the Platform records only the Work Item, branch and base, never an absolute path from a Worker. The owner confirms or chooses.
7. **Confinement is a separate acceptance item.** FEAT-ENV-010's "agents write only inside it" is delivered by running the agent under the Worker's confinement with the worktree as its only writable root (ADR-0053), and is tested there. This ADR moves no confinement code.
8. **Renames** follow the [glossary term map](../../product/GLOSSARY.md#2-term-mapping-from-predecessor-repositories); `weave`, `autoloop` and `.weave` identifiers are not carried. No such string remains in `services/platform` except in `PROVENANCE.md`.
9. **Gate.** No file from the source is added to this repository until this ADR is `accepted` with a recorded approval. Only then does #154 start moving code. Merging this draft is not acceptance; issue #85 stays open until the decision. #154 also remains blocked by #120 (PRJ-001), which this ADR does not resolve.

## Readiness criteria (migration plan §5)
| # | Criterion | State | Evidence |
|---|---|---|---|
| 1 | Source side stable at `c8a2f6e` | **Met** | ADR-0032; the commit and the package tree id were read 2026-10-28 |
| 2 | Inventory refreshed with tests run | **Not met** | The package source was read; its tests were **not run** for this draft (see Context). The owner or a later session records the run before the first file moves |
| 3 | Target ADRs accepted | **Met** | ADR-0013, 0043, 0046 (amended), 0049, 0053 accepted |
| 4 | Contracts stable, with fixtures, in Amaru | **Met for this scope** | The Worker providers port and the OS layer exist (WP-M3-03). A `Workspaces` port for the worktree provider is new and is defined by #154 with its fake |
| 5 | Behavior covered by tests that run in Amaru | **Met by plan** | Item 5; becomes evidence in #154 |
| 6 | Verified execution | **Partial** | The behavior is exercised by the prototype's tests against real git; it was used in the prototype's own delivery runs, which are not reproducible evidence here |
| 7 | License resolved | **Met** | The source is MIT, © Ricardo Lüders, and the holder consents to relicensing ([ADR-0026](0026-fair-code-community-license-with-commercial-licensing.md)). No license file is added by this work |
| 8 | Explicit product-owner approval | **Not met** | Awaiting acceptance |

## Consequences
- **Easier:** worktree isolation starts from a base refresh whose merge, conflict and race handling is already tested against real git, and the "never force-push" rule is checked by a test instead of by reading.
- **Harder:** `Commit` and its scratch rules do not move, so the delivery loop (M4) has to decide what an agent may commit. The worktree location is a new decision. Real-`git` tests need `git` in CI.
- **Security:** git runs with argv arrays and explicit directories only; no shell. Keys and branches are validated before they reach a path or an argument. Credentials reach `git` through the user's ssh agent, passed explicitly by the Worker and never logged. Removing a worktree no longer discards work silently. Confinement stays with the Worker (ADR-0053); a worktree alone is not isolation of writes.
- **Maintenance:** the provider has a single owner (the Platform owner); the prototype copy is frozen. No secrets or personal data are transferred (scanned in each PR). Amaru promises no compatibility with the prototype (no importer, owner decision of 2026-10-27).
- **Follow-up:** after acceptance, update the migration inventory in the private notes and mark these functions migrated (separate approval in the source repository). Record the prototype test run (criterion 2). Decide `Commit`, scratch rules and `Push` with the delivery loop.

## Alternatives considered
- **Move the package unchanged:** ships `Push --force-with-lease`, an unconditional `Remove --force` and prototype identity and paths; fails the acceptance criterion.
- **Write worktree handling from the documents only:** loses the tested `Sync` behavior (conflict abort, lost-race undo, interrupted merge); the file is 534 lines and can be reviewed whole.
- **Use a Go git library (go-git or similar):** a new dependency, and `git worktree` is not fully supported by such libraries; shelling out to the `git` binary matches what the prototype tested.
- **Move `Commit` and the scratch rules now:** pulls delivery-loop policy and prototype path names into M3.

## Approval
Proposed by: Claude (migration ADR draft for #85, 2026-10-28). Not yet approved: the product owner accepts, requests changes or rejects. Requires human decision: item 6 (worktree location), the not-moved rows of item 4, and whether criterion 2 must be met before acceptance.
