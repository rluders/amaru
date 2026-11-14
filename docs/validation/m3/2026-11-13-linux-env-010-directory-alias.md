# ENV-010: reject aliased Work Item directories

| Field | Value |
|---|---|
| Date | 2026-11-13 local (latest GitHub merges are dated 2026-11-14 UTC) |
| Code commit | `44e990822f3314e54eabeec8b0a948dcf6e10a55` |
| Integration baseline | `8e09778e6d1c4893eefd6b41817ca6737962ca7b`, merge of PR #375 into the existing #19 integration branch |
| OS | Fedora Linux 43 Workstation, kernel `7.2.11-100.fc43.x86_64`, x86_64; local host, not hosted CI |
| Tools | Go `1.26.0`, Git `2.56.0`, Node `25.7.0`, pnpm `11.5.0`; kernel Landlock ABI 10 |
| Run by | Codex in the product owner's session, isolated worktree `/tmp/amaru-env154` |

## Queue refresh

Native GitHub relationships, child delivery bodies, remote heads and PR state were read after #375 merged. All six #19 children designate `feat/19-wp-m3-07-worktrees-configuration-scopes` as their PR base. That branch exists remotely at the baseline above. #18's integration branch is a separate queue.

| Issue | State | Dependency / remaining acceptance |
|---|---|---|
| #19 WP-M3-07 | Open | Native #15 closed; full audit-query/Evidence-chain integration still open |
| #85 worktree migration ADR | Closed | Accepted ADR-0056 and migrated provider present |
| #119 ART-001 | Closed | Existing Artifact Ingester retained |
| #124 ART-002 | Open | Native #119/#123 closed; #375 reader identity fix merged, recording/exposure and full Evidence acceptance still open |
| #125 ART-003 | Open | Native #197 still open |
| #154 ENV-010 | Open | Native #85/#120 and inherited #15 closed; production confinement entry and enforced-root reporting still open |
| #178 OPS-003 | Closed | Existing scope resolver retained |

Latest merges: #375 into #19 at the baseline; #374 into #18 at `a28a6e2e8d38d07629bba25ef1a1719bed2a3e22`; #373 into #18 at `55516862288a1dedb194876a6b978dbb9b1e5360`; #372 into #18 at `18f3b0a964737f13bdebdbf27865c72e20af0706`. No open PRs existed at selection time. This change does not import those #18 branch commits or merge/close any issue.

## Bounded change and architecture review

Accepted ADR-0056 requires a distinct directory per Work Item; ADR-0057 grants that directory as the mandatory writable root. Previously a symlink at one Work Item key could point to another prepared worktree. `Prepare` could accept it when its branch matched, and `Confinement` returned the alias, which the OS layer could resolve to the other item's directory. Status, sync and removal also followed the alias into Git operations.

`Repo.Dir` now rejects an existing symlink or file with `domain.ErrInvalid`. Preparation, existing-worktree lookups and removal propagate the error before invoking Git. Nonexistent paths remain valid for preparation and idempotent removal; ordinary prepared worktrees keep their existing behavior. `Confinement` already propagates `Dir` errors. The new regression exercises the symlink case; file rejection follows the same directory-entry check and has no new dedicated test in this slice.

Architecture review: **sound for this bounded change**. The check stays in the existing Worker provider; no new component, dependency, contract, Policy authority or isolation mechanism is introduced. A simpler change to `Confinement` alone would leave preparation and destructive operations accepting the alias. Accepted ADRs already require directory separation, so no new human decision is needed for this fix.

This is a check of the directory entry at call time. It does not protect against concurrent filesystem replacement after validation (TOCTOU), replacement of the configured root or its ancestors, or arbitrary tampering with Git metadata. It does not complete production Worker launch integration.

## Commands and outcomes

| Command | Outcome |
|---|---|
| `trail map`; `trail ask "worktree provider isolation ENV-010 contracts" --source` in shared checkout | Existing graph covered an older spike; no relevant current provider context |
| `trail build`; `trail ask "ConfineExecMain" --source` in isolated checkout, followed by targeted provider queries/skeletons | PASS; current provider/OS-layer spans located before inspecting source |
| `cd services/platform && go test -count=1 -run TestWorktreeDirectoryCannotAliasAnotherWorkItem ./internal/provider/worktree` before fix | FAIL (exit 1): alias accepted; Git invoked and target worktree changed in the disposable fixture. Initial sandbox attempt could not write Go cache; reproduced with cache access |
| `cd services/platform && go test -count=1 -race -v ./internal/provider/worktree ./internal/worker/osproc` | PASS (exit 0), 44 top-level tests, no skips; real confinement tests report ABI 10. An initial implementation attempt failed to compile due to an `err` redeclaration, corrected before this passing run |
| `make check` | PASS (exit 0): format, vet, doccheck, spike checks, contract/protocol checks, 338 TypeScript contract tests, parity of 336 fixtures, 17 SDK tests, Platform tests; no skipped TypeScript tests |
| `git diff --check` | PASS (exit 0) for code change |
| `trail build` after code change | PASS (exit 0); refreshed local graph |
| `make docs-check` after documentation | PASS (exit 0), `doccheck: ok` |
| `trail check --base origin/feat/19-wp-m3-07-worktrees-configuration-scopes` | PASS (exit 0), no conflicts; final staged check reports 11 advisory lexical matches. Reviewed: unrelated Project/SDK/storage/board learnings; no change introduces those rejected behaviors |
| `git diff --check`; `git diff --check origin/feat/19-wp-m3-07-worktrees-configuration-scopes` | PASS (exit 0) |
| `git diff --exit-code 8e09778 -- docs/validation/m3/2026-10-28-linux-env-010.md` | PASS (exit 0); historical record unchanged |

The new regression covers `Dir`, `Confinement`, `Prepare`, `Head`, `Changed`, `Sync` and explicit-discard `Remove`, checking refusal before any Git command and preservation of the target worktree's files and HEAD. Existing provider tests cover normal preparation/idempotency, separate Work Item edits, merge-only refresh, conflict abort and no forced push.

## Evidence levels and remaining obligations

- **Integration-tested on Linux:** alias refusal against real Git/local bare remotes and filesystem symlinks. Existing process tests exercise inside/outside writes, links, descendant confinement and fake-probe refusal. These use a test-binary helper, not a production Worker entry.
- **Not validated:** actual Worker-entry-to-agent launch and recording enforced roots/ABI in Step results/Events. The base has an embedded Worker Runner interface and helpers, but no production Worker executable or concrete agent Runner to wire. Adding an empty entry point would not satisfy acceptance.
- **Not validated:** vendor agent CLIs (#127), full #19 Evidence query, PostgreSQL runtime, native macOS/Windows, concurrent path replacement. Cross-platform vet in `make check` is only a compile/vet signal. Read/network isolation and filesystem metadata restrictions are not claimed; existing OS-layer exclusions remain.
- **Human decisions:** no new architecture choice for this patch. #154 remains open for its owner to assess full acceptance. No issue checkbox, closure, merge or Project-board state was changed. Shared checkout work was preserved.
