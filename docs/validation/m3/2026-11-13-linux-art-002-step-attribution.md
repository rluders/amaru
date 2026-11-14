# ART-002 Step attribution — Linux, 2026-11-13

## Scope and provenance

Partial progress on #124 (ART-002), parent #19. Isolated worktree based on
`df35f6bd4dd6faf5ab682272228975085122a7a2`, the merge of #376 into
`feat/19-wp-m3-07-worktrees-configuration-scopes`. GitHub returned no open PRs
at selection. #119/#123/#85 are closed per the refreshed queue; #124 and #19
remain open. Shared checkout edits were preserved.

## Change and decision basis

`BuildExecutionEvidence` already rejects foreign Execution IDs and checks that
contexts and Artifacts refer to a Step in the Execution. It now also rejects an
Approval gate or nonempty Event Step ID absent from the bundle's Steps, returning
`ErrInvalid` and no partial bundle. Execution Events with no Step ID remain valid.
The regression tests reproduced both previously accepted invalid references.

This is **unit-tested** attribution validation, not Project/Organization access
isolation or a consistent database snapshot. The existing domain model defines
[Events within an Execution and Approvals on Steps](../../architecture/DOMAIN_MODEL.md),
and [ADR-0036](../../architecture/adr/0036-approvals-bind-to-evidence-and-gate-work.md)
binds Approvals to Execution/Step Evidence. No contract, new recorder, dependency,
Policy rule or architecture decision is introduced. Domain code remains pure;
valid bundles retain their representation and digest. A simpler Execution-ID-only
check misses the reproduced invalid Step references.

## Verification

- Before fix: focused `go test -count=1 ./internal/domain -run 'TestBuildExecutionEvidence(RejectsUnknownStepReferences|AllowsExecutionAndStepEvents)'`
  failed for both unknown-Step cases; the valid Event case passed.
- `GOCACHE=/tmp/amaru-art-002-gocache go test -race -count=1 ./internal/domain ./internal/evidence`
  from `services/platform`: PASS, both packages. An initial attempt using the default
  cache could not open an Evidence build-cache entry due to sandbox permissions;
  the rerun above completed.

- `GOCACHE=/tmp/amaru-art-002-gocache make check`: PASS (exit 0), including
  336 contract fixtures with identical Go/TypeScript outcomes and 17 SDK tests.
  The sandbox attempt stopped at pnpm's store database; the elevated rerun passed.
- `GOCACHE=/tmp/amaru-art-002-gocache make docs-check`: PASS (`doccheck: ok`).
- `git diff --check`: PASS.
- `trail build`, `trail build --check`: PASS; wiring graph current. The optional
  deep semantic layer was not built.
- `trail check --base origin/feat/19-wp-m3-07-worktrees-configuration-scopes`:
  PASS, no conflicts. These are local checks, not CI.

## Remaining gaps and decisions

API/CLI bundle exposure, Project/Organization authorization, missing check/diff/log/
usage/workspace recorders, and snapshot consistency remain open. This slice does
not establish full ART-002 or #19 acceptance. No PostgreSQL runtime, native
macOS/Windows, real Provider, or full audit walkthrough was validated.

Further consistency work must specify the cross-store read boundary and whether
an in-progress query promises one snapshot or explicitly reports a mixed-time
view. This change makes neither promise. Recorder/exposure work must use existing
accepted contracts or obtain owner approval for any new semantics; no new semantics
were needed for this slice.
