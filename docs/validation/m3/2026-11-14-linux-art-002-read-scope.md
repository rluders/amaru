# Linux ART-002 internal metadata read scope

Date: 2026-11-14. Runner: Codex on the owner's Linux x86_64 workstation
(kernel `7.2.11-100.fc43.x86_64`; physical/VM classification not independently verified).
Branch: `feat/124-evidence-read-scope`, based on #19 integration commit
`5bf1d3d` (merged #386). This record accompanies the implementation commit;
its immutable head is recorded in the PR. Local evidence, not hosted CI.

## Queue and authorization

GitHub was refreshed after #386: merged at `2026-11-14T04:54:07Z`, no open
PRs at selection. Native #124 blockers #119/#123 and inherited #19 blocker
#15 are closed. #124 and #19 remain open, with unmet acceptance untouched.
ADR-0069 is accepted by Ricardo Lüders and explicitly approves repeatable-read
metadata composition. No new significant architecture decision is needed for
this bounded internal adapter slice.

M3 remains the headless Platform milestone, with Linux required and native
macOS/Windows best effort. Public authorized Evidence reads and the full
WP-M3-07 test-Execution query remain required, not silently deferred to M4.
M4 delivery-loop/UI and M5 retries/remote Workers are outside this slice.

## Architecture assessment and change

Verdict: **sound within the accepted internal read boundary**. All metadata
currently used by `internal/evidence.Reader` is in one PostgreSQL database:
Execution/Steps, Events, Approvals, audit, contexts, Artifact versions and Usage
receipts. Existing `Get`, `Events` and `Approvals` used independent transactions;
other reads used the pool directly. Those calls alone cannot share a snapshot.

The new `postgres.EvidenceReader` owns one read-only repeatable-read transaction,
passes its query interface to reused adapter helpers and ends the scope before
returning. The assembler now requires narrow internal read interfaces rather
than writable ports. No transaction enters the domain or ports; no arbitrary
cross-store callback, write access, context-carried transaction or fallback
connection is exposed. Existing store methods retain their prior behavior.
No new dependency, migration, infrastructure, public contract or capture,
Approval, retention, completeness or error-mapping semantics were introduced.

A simpler Execution-version recheck cannot detect independent context, Artifact
or audit commits. A new projection or sealed export would add unnecessary state
and different semantics. This implementation is limited to the existing single
database and does not promise a snapshot across independent stores or blob bytes.

The snapshot begins with the first metadata query, not transaction allocation.
The complete scope has a 30-second maximum and honors earlier caller deadlines.
All internally fetched audit pages share it. Query failures, cancellation,
assembly errors and unsuccessful transaction completion return no successful
partial bundle. Later calls may see later commits; neither Final nor Digest seals
Evidence. No latency, memory or scale claim: audit pages remain 1,000 rows but
number of pages and total bundle size have no new count bound.

Official behavior checked on this date:
[PostgreSQL isolation](https://www.postgresql.org/docs/17/transaction-iso.html#XACT-REPEATABLE-READ)
and [Go database/sql BeginTx](https://pkg.go.dev/database/sql#DB.BeginTx).
The former defines the first-query snapshot; the latter documents transaction
context cancellation and rollback. Tests exercise the installed driver/server.

## Verification

Tool versions: Go launcher `go1.26.0 linux/amd64` (Platform module toolchain `go1.27.0`, selected by
`go.mod`), Node `v25.7.0`, pnpm `11.5.0`, local PostgreSQL `17.5`.
The PostgreSQL instance is a disposable isolated test server on loopback port
55482 with a test-only trust role. Its trust configuration is not a product
security configuration or security validation. Each test creates/drops its own
database through the existing `migratedDB` helper.

Focused coverage in `evidence_test.go` and `evidence_pg_test.go`:

- Unit lifecycle: read-only/repeatable-read transaction options, read failure and
  cancellation while querying, zero returned bundle and exactly one rollback.
- Concurrent commit: PostgreSQL table-lock wait establishes that the reader has
  read its root snapshot before a writer commits changes to all metadata
  families. The in-flight result equals the complete pre-commit bundle; a later
  call observes the new Execution/Step, Event, Approval/audit, context, Artifact
  and Usage capture. Raw fixture SQL is test orchestration, not a production
  capture API or full valid workflow walkthrough.
- Audit pagination: insert a committed matching audit entry between the first
  1,000-row page and the second query through a query decorator over a real
  transaction; it is excluded from this snapshot and included by the next call.
- Cancellation of a real blocked query returns no bundle. Missing Execution and
  a later Artifact table read failure both release scopes; a one-connection pool
  remains usable afterwards. No missing recorder is fabricated as an error gap.

Commands (repository root unless noted):

- `GOCACHE=/tmp/amaru-124-read-cache make check`: initial run failed, exit 2,
  because docs linked this record before it existed. After adding it,
  `GOCACHE=/tmp/amaru-124-read-cache make check > /tmp/amaru-124-read-check-final.log 2>&1`
  passed, exit 0. Go/TypeScript parity: 336 fixtures; TypeScript contracts:
  338/338; SDK: 17/17. Platform tests and host/Windows/macOS/PG-tag vet passed.
- From `services/platform`:
  `AMARU_TEST_PG=postgres://evidence_test@127.0.0.1:55482/postgres GOCACHE=/tmp/amaru-124-read-cache go test -race -count=1 -v -tags pg ./internal/store/postgres -run TestEvidenceReadScope`
  passed, exit 0: four PostgreSQL cases plus lifecycle unit test with two
  subcases, no skips. Log: `/tmp/amaru-124-read-focused.log`.
- `AMARU_TEST_PG=postgres://evidence_test@127.0.0.1:55482/postgres GOCACHE=/tmp/amaru-124-read-cache make platform-test-pg`
  passed, exit 0: full Platform PostgreSQL-tagged regression run, including the
  existing store, coordinator and migration suites. Log:
  `/tmp/amaru-124-read-pg.log`.
- `GOCACHE=/tmp/amaru-124-read-cache make docs-check`: passed, exit 0.
- `git diff --check`: passed, exit 0.
- `trail build` and `trail check --base 5bf1d3d`: passed, exit 0, no conflicts.
  Final staged check: eight advisory lexical matches reviewed. They concern
  historical Project timestamps, a Node fixture failure and superseded native
  OS deferral wording; none conflicts with this slice or its passing results. Wiring graph
  refreshed; optional deep layer not built.
- `/tmp/amaru-335-postgres/bin/pg_ctl -D /tmp/amaru-124-read-pgdata -m fast -w stop`:
  passed, exit 0; disposable server stopped after tests.

An initial focused command used repository-relative paths from the Platform
module and the default read-only Go cache; it failed before testing. Corrected
paths and the writable cache above were used for the passing runs. Temporary
logs and PostgreSQL data are local artifacts, not committed product files.

## Limits and refreshed next task

**Unit-tested**: lifecycle failure and cancellation with a minimal SQL driver
fake; existing Reader/domain validation remains covered by local checks.
**Integration-tested**: the focused PostgreSQL cases above by the passing recorded
commands. This is not a real Provider or complete Evidence audit walkthrough.
**Not validated**: public authorization, cross-Project/Organization access,
API/CLI exposure, blob availability/expiry, remaining recorder producers,
public Usage Event delivery, native macOS/Windows and full ART-002 acceptance.
This reader performs unscoped audit queries internally; it must not be exposed
as an authorization boundary. No Organization mapping is invented.

Next for #124: review this slice, then design/review the versioned Evidence API
contract and scoped application composition with WP-M3-06 before exposure.
Remaining check/diff/log/workspace recorder inputs need their own bounded
producer/identity/payload review under ADR-0069. #197 remains blocked by #124;
#125 is not made ready by this slice.

An independently ready bounded M3 candidate is #219: native #135, inherited
#15 and supporting #83/#84 are closed, with ordered replay already on its #18
base. Recommend a focused two-client stale-edit/conflict acceptance gap
assessment against existing versioned mutation contracts before adding Event
or watcher behavior. Its PR base is #18, not #19. Prerequisite readiness is not
full #219 acceptance. No second implementation slice was started here.

Shared dirty work and article inputs were left untouched. No merge, parent or
child issue closure, acceptance checkbox or Project-board mutation was performed.
