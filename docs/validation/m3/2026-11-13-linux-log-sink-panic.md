# Linux M3 continuation: operational sink panic containment

Date: 2026-11-13. Base: `18f3b0a964737f13bdebdbf27865c72e20af0706`
on `feat/18-wp-m3-06-public-api-events`. Local results, not hosted CI.

## Selection and refreshed queues

GitHub open issues, native `dependencies/blocked_by`, recent merged PRs and fetched
remote refs were refreshed. No PR was open at selection. #372 merged at the base
above; #371 merged at `d1b75c2`, #370 at `ad5f1bd`, #369 at `a0e379b`,
#368 at `9ba18b7`, #367 at `bf3c9f8`, #366 at `f87733a`, #365 at `4ebcef1`.
Shared checkout changes were preserved; implementation uses an isolated worktree.

| Queue | Open children / native prerequisites | Designated base and remaining gates |
|---|---|---|
| #18 | #92 (#83/#84 closed), #170 (#83/#84 closed), #191 (#120 closed), #219 (#135 closed); inherited #15 closed | `feat/18-wp-m3-06-public-api-events` at `18f3b0a`. #372 adds session-bound CSRF evidence; #371 adds SDK rollback evidence. Full product composition and child acceptance remain open. |
| #16 | #188 (#135 closed); #197 (#124 open, #195 closed) | `feat/16-wp-m3-04-policy-grants-and` at `d2c393a`; #135 implementation `fa8bb47` is still not an ancestor. Refresh prerequisites on this base before integration. |
| #17 | #80/#81/#82 migration decisions open (#8 closed); #112/#113/#127/#128/#131/#163 blocked by those; #189 blocked by #113; #190 by #127/#113 | `feat/17-wp-m3-05-providers-and-context` absent from fetched remote refs. Resolve owner migration gates and establish the designated base before implementation. |
| #19 | #124 (#119/#123 closed), #154 (#85/#120 closed), #125 (#197 open) | `feat/19-wp-m3-07-worktrees-configuration-scopes` absent from fetched remote refs. Establish the designated base and verify prerequisite code before implementation. |
| #20 | #16/#17/#18/#19 open | Exit validation remains blocked. All four parent queues have native #15 closed. |

## #219 comparison and unresolved semantics

The closed #135 prerequisite is on the actual #18 base. Ordered replay alone
does not establish the following WRK-003 requirements:

| Required case | Existing behavior / evidence limit |
|---|---|
| Typed committed mutations with Project/cursor identity | `WorkChanged.json` names `work.package.changed` and `work.item.changed`; `CatalogChanged.json` names `agent.changed`/`team.changed`, describing external file edits. There is no Workflow-changed event variant. No Work/Agent authoritative mutation adapter emitting these events exists in the inspected Platform source. Schemas alone are not committed mutation coverage. |
| External definition edits and invalid external definitions | Workflow Git loading validates selected definitions and pins Execution content; it is not a live external-file ingestion/revalidation publisher. No watcher/committed ingestion path demonstrates this acceptance case. |
| Two clients, duplicate Event, conflict during open edit, unrelated input preserved | `AgentHistory.guard` rejects stale expected revisions; Update/Replace operate on that domain history. This does not demonstrate a public two-client mutation/refetch path, duplicate change-event handling or file/API races. The historical F-7 incident remains unproven. |

Human product/contract decisions remain necessary: the Workflow change payload and
identity; authoritative file-change acceptance/rejection and its notification;
and the public mutation/version/refetch contract connecting file and API edits.
No event name, reconciliation policy or file ingestion ownership is invented here.
These #219 cases remain **not validated** end to end. This record does not change
contracts or claim #219 acceptance. Select the safe unblocked #170 child instead.

## Bounded implementation

`services/platform/internal/adapter/operational/log.go` already converts sink errors
and short writes into `operational log write failed`. It did not contain a sink
panic: a credential-bearing panic value could escape the HTTP logging defer and
reach `net/http` diagnostics. The new narrow writer boundary returns the same
fixed error for a panic without formatting its value. The outer logger lock is
released normally; repeated writes remain possible. No retry or recovery of a
partially written record is promised, and sink failures still reach the existing
error callback. No dependency, contract, infrastructure or architecture decision
is added. A sink's own output or a blocked writer is outside this fix.

## Checks and evidence

- Before the fix: `go test -count=1 -run TestSinkPanicReturnsSafeError ./internal/adapter/operational`
  from `services/platform` **FAIL**, exit 1, with the synthetic sink panic escaping.
  An earlier sandbox invocation failed during setup because the Go cache was read-only;
  it provided no behavioral evidence.
- After the fix: `go test -race -count=1 -v ./internal/adapter/operational`
  **PASS**, exit 0, 16 top-level tests, no skips. Repeated sink panic/error return is
  **unit-tested**. Success/rejection/handler panic over real loopback HTTP with a
  panicking sink, safe callback and empty server ErrorLog is **integration-tested**.
- Final `make check`: **PASS**, exit 0 on Linux. Includes formatting, doccheck,
  contract Go/TypeScript fixture parity, SDK generated-type checks and all 48 SDK
  tests, Platform tests and PostgreSQL-tagged vet. PostgreSQL runtime tests are not
  part of this command.
- `git diff --check`: **PASS**, exit 0. `trail check --base
  origin/feat/18-wp-m3-06-public-api-events`: **PASS**, exit 0, no conflicts.
  Its one advisory matched SQL connection-panic learning text against the HTTP
  test; this fix neither changes SQL ownership nor asserts SQL panic cleanup.

No native macOS/Windows runtime, actual Provider, PostgreSQL, full product startup
or complete product channel capture was run for this slice. Those are **not
validated** here. Linux is the required M3 platform; other OSes remain best effort.
#170, #219, #18 and M3 remain open. No merge, issue closure or board mutation.
