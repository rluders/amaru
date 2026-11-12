# Linux Worker session shutdown validation

Date: 2026-11-12. Runner: Codex on the owner's Linux development host, not hosted CI. Scope: [#339](https://github.com/rluders/amaru/issues/339), based on #18 integration commit `d7d519cf983ae36a4135e655f0e0e33077f69429` after merged PR #340. The reviewed PR head identifies the resulting implementation and evidence together.

Environment: Linux 7.2.11-100.fc43.x86_64, x86_64; Go host 1.26.0 (Platform module toolchain 1.27), Node v25.7.0, pnpm 11.5.0, disposable PostgreSQL 17.5 on loopback port 55435. Physical/full-VM classification is unverified. Native macOS/Windows were not available; Linux is required for M3 and the other OSes remain best effort.

## Change and regression

`Coordinator.Serve` now normalizes a returned error only when it wraps the session context's own cancellation/deadline error. This applies to assignment and every Worker message handler, including store calls already executing when shutdown starts. Receive and send paths preserve unrelated errors even if the session has been cancelled. Closed-link handling, durable transitions, result acknowledgements and recovery rules are unchanged. A nil shutdown return describes the session ending; it does not assert that an interrupted write committed.

Before the fix, `go test -count=1 -run TestServeShutdown ./internal/coordinator` failed: all six store cancellation cases returned errors, and an unrelated send error was hidden. Deterministic store callbacks signal entry and block on the context; no sleeps or PostgreSQL are needed for these regressions. Tests cover Offer, AcceptAttempt, RejectAttempt, LeaseRenew, StepEvents and Result. Send cases cover assignments and result acknowledgements, checking normal cancellation, unrelated errors and commit-before-assignment-send. Cancellation from another context remains an error while the session context is live.

## Commands and outcomes

Commands below ran locally on Linux. PostgreSQL commands used `AMARU_TEST_PG` pointing to the disposable loopback server; temporary logs are local artifacts, not published CI evidence.

- From `services/platform`: `go test -race -count=1 ./internal/coordinator ./internal/worker/embedded`: PASS, exit 0, both complete packages. Log `/tmp/amaru-339-race.log`.
- From `services/platform`: `go test -race -count=20 -run TestServeShutdown ./internal/coordinator`: PASS, exit 0; 22 regression subtests per repetition.
- From `services/platform`, with the test DSN: `go test -tags pg -count=20 -run 'TestLimitsWithPostgres/a_failed_result_and_a_refused_attempt_free_their_slots' ./internal/store/postgres`: PASS, exit 0, twenty repetitions. Log `/tmp/amaru-339-pg-repeat.log`. The unchanged earlier integration had failed four of twenty repetitions, recorded in the [settings validation](2026-11-12-linux-prj-002-settings.md#commands-and-outcomes).
- With the test DSN: `make platform-test-pg`: PASS, exit 0, all 19 packages with tests; four packages have no test files. Includes shared protocol durability, terminal aggregation, recovery and PostgreSQL suites. Log `/tmp/amaru-339-pg-full.log`.
- `make check`: PASS, exit 0. Includes formatting, vet (with compatibility vet for macOS/Windows), architecture tests, documentation checks, 346 shared contract fixtures with identical Go/TypeScript outcomes and 40 SDK tests. Log `/tmp/amaru-339-check.log`.

## Evidence and limits

**Unit-tested:** cancellation at the store/send boundary, unrelated-error preservation and foreign-context cancellation. **Integration-tested:** real PostgreSQL-backed scheduling, protocol durability and recovery suites on Linux. No new dependency, infrastructure, contract or architecture decision is introduced.

**Not validated:** native macOS/Windows, a composed product HTTP server, remote Worker authentication, real Providers or database power loss. This fix does not complete #18, #191 or M3. #339 stays open until its reviewed PR is merged into the designated integration branch.
