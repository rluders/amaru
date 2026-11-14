# ART-002 Evidence audit-page validation — Linux

Date: 2026-11-14. Local Linux validation, not hosted CI.
Branch: `feat/124-scoped-evidence-composition`, based on #19 integration head
`2a0136ba75fd554e5b94f9437f06eb8fee6998ed` (merged PR #387).
Refs #124 and #19; both remain open.

## Behavior

The `AuditLog` port requires up to `limit` events with `Seq > after`, in
sequence order. ADR-0069 requires the Evidence Reader to preserve ordered Audit
history across its internal pages. `approvalAudit` now rejects a page larger
than its requested 1,000 events, or any event whose sequence does not strictly
advance from the cursor and prior event. It returns `domain.ErrInvalid` and no
bundle for malformed store pages. This prevents duplicate, descending or stale
pages from being accepted and prevents a non-advancing cursor from looping.

The focused fake-backed regression retains four malformed-page cases:
duplicate, descending, stale cursor and oversized. The first page is exactly
1,000 valid events; each malformed second page must fail after two reads. This
does not cap total page count, total bundle memory, or establish the full
multi-Step audit walkthrough. No schema, API, audit writer, database behavior,
or public contract changed.

## Validation

- Before the guard, the focused regression failed because a stale-cursor page
  returned no error. The former executor's run also showed `error = <nil>, want
  invalid definition` in that case.
- From `services/platform`, `GOCACHE=/tmp/amaru-124-api-cache go test -race -count=1 ./internal/evidence` passed (exit 0).
- `GOCACHE=/tmp/amaru-124-api-cache make docs-check` passed (exit 0,
  `doccheck: ok`).
- A sandboxed `GOCACHE=/tmp/amaru-124-api-cache make check` attempt failed in
  `packages/contracts/test/fixtures.test.ts` (exit 2). The full
  `GOCACHE=/tmp/amaru-124-api-cache make check` rerun with existing dependency,
  child-process and loopback access passed (exit 0).
- `git diff --check` passed (exit 0).
- `trail build` and `trail check --base 2a0136ba75fd554e5b94f9437f06eb8fee6998ed`
  passed with no conflicts; one unrelated lexical advisory was reviewed.

Evidence level: **unit-tested** with a fake AuditLog. No PostgreSQL runtime,
public authorization/API, or full ART-002 acceptance is claimed. Shared checkout,
articles and other worktrees are preserved.
