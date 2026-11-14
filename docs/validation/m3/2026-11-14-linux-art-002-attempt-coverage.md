# Linux ART-002 current-attempt Usage coverage — 2026-11-14

## Scope and queue

Runner: Codex in the owner's Linux workspace (container environment; host machine kind unverified). Base: fetched #19 integration head `39ee71196109fbfb62d4775dd18a99fb69799003`, after merged PR #384. Branch: `feat/124-evidence-usage-query`; isolated worktree: `/tmp/amaru-124-query`. This record describes its containing commit's tree. PR base: `feat/19-wp-m3-07-worktrees-configuration-scopes`; Refs #124 and #19 only. Both issues and all outstanding acceptance criteria remain open.

GitHub refreshed on 2026-11-14: native #124 blockers #119/#123 and inherited #19 blocker #15 are closed. No open PRs existed at selection. #154 native #85/#120 are closed; production Worker launch/reporting remains unfinished. #125 still waits for open #197. The current M3 open queue also includes #16/#17/#18/#20, #80/#81/#82, #92/#112/#113, #127/#128/#131, #163/#170/#188/#189/#190/#191/#219. This is an observed queue snapshot, not a readiness claim for those other issues. Historical issue-body statements that #15 remains open or that Usage is not recorded are superseded by current native state and #384. No issue body, dependency, checkbox or Project board was mutated.

The next bounded #124 gap was chosen because the merged capture path now supplies historical receipts to the existing Reader. A receipt for an earlier attempt suppressed the Usage gap even when the Step's current attempt had no accepted capture. The fix requires current-attempt coverage within that Step while retaining all historical receipts and existing empty-report gaps. It adds no retry engine, recorder, contract, endpoint or storage behavior.

## Authority and review

Accepted [ADR-0069](../../architecture/adr/0069-durable-evidence-capture-and-authorized-bundle-reads.md#evidence-identity-and-completeness-accepted-direction) requires completeness per applicable attempt and preserves missing Usage as unknown. [ADR-0033](../../architecture/adr/0033-usage-and-cost-telemetry-model.md) retains all-attempt attribution. The M3 roadmap still requires the full audit query; this correction does not complete it.

Owner PDFs were read from the shared checkout without modification or commit. *Why I'm Building an Engineering Execution Platform*, pp. 6–8, assigns persistent identities and enforced invariants to the Platform. *From Vision to Architecture*, pp. 7–8, separates intelligence from authoritative execution control. These support retaining attributable history and explicit coverage; broader distributed/Brain vision authorizes no new M3 scope. See [recorded filenames and context](../../architecture/adr/0069-durable-evidence-capture-and-authorized-bundle-reads.md#owner-article-context).

Architecture-review verdict: sound within the accepted internal boundary. Pure domain composition remains authoritative, with no new dependencies, infrastructure or Worker/Client changes. The simpler previous test (any receipt for the Step) falsely treated historical presence as current coverage. Store failures still fail the query; no write, acknowledgement, Event publication or snapshot semantics change. Historical receipts are fixtures, not evidence of an implemented M5 retry path.

## Local evidence

Environment: Linux `7.2.11-100.fc43.x86_64`, x86_64; Go `go1.26.0 linux/amd64`; Node `v25.7.0`; pnpm `11.5.0`; disposable PostgreSQL `17.5`. Local results only, not hosted CI.

- Before fix: from `services/platform`, `GOCACHE=/tmp/amaru-124-query-cache go test -count=1 ./internal/domain -run TestUsageEvidenceCurrentAttemptCoverage` failed (exit 1), reproducing the missing-current-attempt gap in running, succeeded, failed, cancelled and timed_out states.
- After fix: `GOCACHE=/tmp/amaru-124-query-cache go test -race -count=1 ./internal/domain ./internal/evidence` passed (exit 0, both packages). Regression matrix: 15 state/capture cases, one cross-Step coverage test and two Reader composition cases; checks retain historical receipts, current empty/missing/recorded distinctions, bundle gaps and stable digest.
- `GOCACHE=/tmp/amaru-124-query-cache make check`: initial sandbox run failed (exit 2) because pnpm could not open its SQLite store; rerun with store/loopback access passed (exit 0), including 336 Go/TypeScript fixture parity, TypeScript contracts 338/338 and SDK 17/17.
- With disposable loopback server on port 55479, from `services/platform`: `AMARU_TEST_PG=postgres://query_test@127.0.0.1:55479/postgres GOCACHE=/tmp/amaru-124-query-cache go test -race -count=1 -v -tags pg ./internal/store/postgres -run 'TestUsage(Capture|PublicationAtomicity)$'` passed (exit 0): two top-level tests, 21 shared capture cases, no skips. This verifies existing persistence/replay/atomicity compatibility; the new coverage regression is domain/Reader evidence, not a new PostgreSQL scenario.
- Disposable server stopped with `/tmp/amaru-335-postgres/bin/pg_ctl -D /tmp/amaru-124-query-pgdata -m fast -w stop` (exit 0). `pg_ctl ... status` returned expected exit 3, no server running.

- Final `GOCACHE=/tmp/amaru-124-query-cache make docs-check` passed (exit 0, `doccheck: ok`). `trail build`, `trail build --check`, `trail check --base 39ee71196109fbfb62d4775dd18a99fb69799003`, `git diff --check` and `git diff --cached --check` passed (exit 0). Trail reported no conflicts; lexical follow/advisory matches were reviewed against this bounded change. Wiring graph current; optional deep layer not built.

## Limits and next work

**Unit-tested:** current-attempt coverage and retained history; fake-backed Reader composition. **Integration-tested:** existing PostgreSQL capture/replay/atomicity suite. **Not validated:** full PostgreSQL suite, transactionally consistent multi-store reads, authorized public API/contract and Organization boundary, actual UsageRecorded publication, remaining check/diff/log/workspace recorders, real Provider Usage, native macOS/Windows, or full multi-Step audit walkthrough. No new owner decision is required for this correction; missing recorder semantics and public contract review remain gates for subsequent slices under ADR-0069.

Shared dirty checkout, owner articles and unrelated branches are preserved. Generated Trail cache is not part of the change. No merge or issue closure was performed.
