# Linux ART-002 bounded Usage capture — 2026-11-13

## Scope and base

Runner: Codex in the owner's Linux workspace. Base: latest fetched #19 integration head `77800c485f96daf5d5b7de1606b412162cb5b649`; branch `feat/124-usage-capture`, isolated worktree `/tmp/amaru-124-usage`. The record describes this change's tree; its commit is the containing commit. #124 and #19 stay open; no acceptance checkbox, milestone or full ART-002 status changes. PR base is `feat/19-wp-m3-07-worktrees-configuration-scopes`; no issue-closing keyword.

Shared dirty checkout and owner article PDFs were neither changed nor committed. The owner articles are intent context only, using the page citations already recorded in [ADR-0069](../../architecture/adr/0069-durable-evidence-capture-and-authorized-bundle-reads.md#owner-article-context); they do not authorize new infrastructure, public contracts or broader M3 scope.

## Change and architecture review

Existing Worker Result Usage shapes are mirrored into the in-process port and embedded Runner outcome; no Protobuf/schema/OpenAPI route changes. Platform completion now stores a fenced receipt, ordered reports and reference-only publication intent atomically with the existing lifecycle Event before acknowledgement. The internal Evidence Reader composes these authoritative records and preserves explicit unknown/missing Usage. See [implementation and limits](../../../services/platform/README.md#bounded-usage-capture-art-002).

Architecture-review verdict: **sound within the bounded internal slice** under accepted ADR-0069. Platform retains all state/identity authority; Worker imports only ports, domain validation remains pure, no dependency or infrastructure is added. A standalone recorder/table would not deliver reports into Evidence and would permit partial completion, so capture extends the existing ExecutionChange transaction instead. A failed commit writes nothing; committed-but-unacknowledged replay derives duplicate acknowledgement from the stored fingerprint; conflicting content fails without ack. The store independently refuses capture without the running fenced Step and completion Event. Database receipts refuse UPDATE/DELETE. Public exposure requires the separately reviewed contract and access integration; no new decision is selected here.

Review findings addressed: timed-out Steps are waiting states, so accepted timeout Results explicitly allow capture and replay; concurrent completion must first check the Step version so an identical racing delivery retries and reads the committed receipt; empty and absent report lists normalize equally. A terminal Step with no receipt is stale, not permission for late enrichment. Project/Environment scopes come from immutable Execution state. No Organization mapping exists on this base; no Organization identity is fabricated and no public access guarantee is claimed.

`UsageRecorded` is an existing reference-only contract. Production Event/outbox infrastructure is absent on this branch, unlike the separate spike. The new internal pending intent stores stable Usage id, receipt and ordinal; it assigns no public Event id, cursor, envelope or delivery status. This is durable publication intent only, not proof of publication. No spike code is migrated.

## Environment

Linux x86_64, kernel `7.2.11-100.fc43.x86_64`, workspace environment (physical/VM classification unverified). Root Go 1.26.0; Platform auto toolchain Go 1.27.0; Node v25.7.0; pnpm 11.5.0; buf 1.69.0. Disposable PostgreSQL 17.5 from existing local binaries, initialized in `/tmp/amaru-124-usage-pgdata`, listening only on loopback port 55494. Tests use the existing `migratedDB` harness, separate temporary databases, fake Workers/Providers, no paid APIs. This is local evidence, not CI or a real Provider run.

## Commands and outcomes

Final review resumed on 2026-11-14 in the same worktree. The earlier
`/tmp/amaru-124-usage-check-verified.log` and
`/tmp/amaru-124-usage-pg-full-final.log` contain passing output, but no tree
fingerprint or explicit exit status. They are historical supporting evidence,
not exact-final-tree exit-0 evidence. Finalization changes this record, so the
required checks are rerun before commit.

- `GOCACHE=/tmp/amaru-124-usage-cache make check`: exit 0; log
  `/tmp/amaru-124-resume-check.log`. Includes doccheck, contract/SDK checks,
  Platform vet (including PG-tagged code) and uncached non-PG tests.
- `GOCACHE=/tmp/amaru-124-usage-cache make docs-check`: exit 0; log
  `/tmp/amaru-124-resume-docs.log`.
- In `services/platform`, with `AMARU_TEST_PG` targeting the disposable
  loopback cluster as `postgres` and the same Go cache:
  `go test -race -count=1 -tags pg ./internal/store/postgres -run 'TestUsage(Capture|PublicationAtomicity)$'`:
  exit 0; log `/tmp/amaru-124-resume-pg-race.log`. Two top-level tests,
  including 21 shared outcome/fault cases; no PG skip.
- `trail build` and `git diff --check`: exit 0. Generated Trail cache is
  local only and excluded from the commit.
- Disposable PG: stopped with `pg_ctl -D /tmp/amaru-124-usage-pgdata -m fast stop`;
  subsequent status reports no server running (expected exit 3).

An initial restricted `make check` failed on the read-only default Go cache;
the writable-cache run above resolves that environment issue. The first PG
race invocation used a nonexistent `rluders` role and failed before exercising
capture; the corrected `postgres` invocation above is the test evidence.
The sandbox process view hid the already-running PG instance; host-side
inspection identified that exact instance before testing and stopping it.
Full PG-suite passing output was inspected from the earlier log; the full
suite was not rerun during finalization. No CI result is claimed.

## Evidence and remaining work

- **Unit-tested:** measurement source/presence, unknown vs reported zero, decimal cost and price-table requirements, finite/nonnegative credits, payload/count/label bounds; fake-store delivery, fault and replay cases; Evidence composition and read failures.
- **Integration-tested:** actual embedded Worker with fake Runner over in-process and JSON-serialized links, including requested cancellation preserving Usage; PostgreSQL completion/receipt/intent atomicity, replay and concurrent duplicate delivery. The shared Usage suite covers success, failure and cancellation, store failure before commit, lost acknowledgement after commit, empty Usage, stale fences and changed payload/outcome conflicts.
- **Not validated:** actual public UsageRecorded delivery, versioned public Evidence contract/routes/authorization (including Organization mapping), transactionally consistent multi-store bundle reads, other recorders, roll-ups, real Provider Usage, remote Worker transport, native macOS/Windows and full multi-Step audit walkthrough. Existing receipt storage preserves attempt attribution but adds no M5 retries.

Missing capture and accepted empty reports remain distinct unknown reasons; a stored report with absent measurements does not imply free work. Bounds are internal implementation limits, not new public wire constraints. Full #124/#19 acceptance and the remaining ADR-0069 implementation remain open.
