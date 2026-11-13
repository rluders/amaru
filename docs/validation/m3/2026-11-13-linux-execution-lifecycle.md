# Linux execution lifecycle evidence

Date: 2026-11-13. Scope: #18, accepted ADR-0068, after PR #362 merged at `de8ffa02e11a6cc98316bad5153a43691cea6d80`. These are concrete lifecycle helpers, not a composed production Platform.

## Change and architecture review

`services/platform/internal/platform/execution.go` adds a Coordinator `Session` and `EmbeddedWorker` owner. Session obtains the existing recovered Coordinator, creates its owned in-process protocol pair inside Start and completes readiness when the receive pump enters that usable transport. Worker readiness completes after its initial Offer successfully crosses the existing link. Neither waits for a first Assignment. Register Session after recovery/background stages, then Worker, before public admission. Production composition must retain the preceding messaging/outbox stages; tests do not substitute placeholders for them.

Essential loops use existing Coordinator.Serve and Worker.Run; unexpected nil exit with a live context becomes a static lifecycle failure. Within the cleanup budget, Stop joins Worker attempts, then Coordinator/session work and closes the owned pair. No retry/restart, replacement supervisor, synthetic Result, domain Cancel or durable rollback is introduced. One-shot recovery and Provider-attempt failure semantics are unchanged.

Review verdict: sound within ADR-0068. No new dependency, infrastructure, contract, domain interface, authorization bypass or migration. Lane imports remain outside Coordinator/Worker logic. A simpler readiness signal at goroutine launch would not prove transport initialization or the initial Offer; waiting for an Assignment would block a healthy empty process. Resources remain owned until reverse joins, and a stuck adapter is bounded by the existing aggregate shutdown budget. Timeout remains failure requiring final process exit, not an assertion that abandoned work stopped. Public HTTP admission, final exit and Provider selection are not composed here.

## Reproduced receive-pump defect

`Coordinator.Serve` previously deferred only cancellation of its receive pump. `TestServeJoinsReceivePump` held that Receive after observing cancellation; unchanged code returned before release (local `go test -race -count=1 -run '^TestServeJoinsReceivePump$' ./internal/coordinator`: FAIL, exit 1, log `/tmp/amaru-session-pump-before.log`). Session teardown could therefore mistake Serve return for a complete join.

Serve now cancels and joins its receive pump on return. The pump captures its own panic as a static error without formatting its value, and its joined result preserves unrelated transport failure if cancellation wins the loop select. This still requires the existing ports.Link context contract; an uncooperative Receive can strand Start, which Lane bounds at process shutdown. It adds no Lane dependency or execution-state logic to Coordinator.

## Local validation

Linux x86_64, kernel `7.2.11-100.fc43.x86_64`; Platform Go 1.27.0, Lane v2.1.0. Local results are not hosted CI.

- `go test -race -count=20 -run '^TestServe' ./internal/coordinator`: PASS, exit 0, after the receive-pump correction. Log: `/tmp/amaru-session-pump-after.log`.
- `go test -race -count=1 ./internal/platform ./internal/coordinator ./internal/worker/embedded ./internal/arch`: PASS, exit 0. Log: `/tmp/amaru-execution-lifecycle-race.log`.
- Final `go test -race -count=20 -run '^TestExecution|^TestServe' ./internal/platform ./internal/coordinator`: PASS, exit 0 (Platform 1.194s; Coordinator 1.714s). Log: `/tmp/amaru-execution-lifecycle-final-focused.log`.
- `AMARU_TEST_PG=... go test -race -count=1 -tags pg ./...`: PASS, exit 0, against the existing disposable PostgreSQL 17.5 instance on loopback port 56391 (PostgreSQL package 121.414s). Log: `/tmp/amaru-execution-lifecycle-pg.log`. The disposable server was stopped afterward; no operator-owned server was stopped.
- `make check`: PASS, exit 0, including docs, Go/TypeScript fixture parity, SDK, Platform tests, PostgreSQL compile checks and Windows/macOS vet. Log: `/tmp/amaru-execution-lifecycle-check.log`.

The actual Lane/Coordinator/Worker tests use fake durable stores and the actual in-process transport: recovery readiness and WaitReady before Start, empty-process readiness, initial Offer failure, end-to-end success, unexpected closed-link failure, configuration rejection, bounded joins, link retention until attempts finish, and unchanged Execution/Step/Event snapshots after process cancellation. Coordinator tests separately cover pump joining and safe panic/error propagation during cancellation. These are integration-tested helpers; Provider work is fake.

## Limits

No product entrypoint, final failure exit, Linux child-process owner/reaping, NATS/outbox/Event transport, public HTTP/discovery/admission composition or real Provider. The PostgreSQL regression run validates existing durable adapters/Coordinator behavior; it is not a new PostgreSQL-backed composition test for these owners or a process/server crash test. Native macOS/Windows runtime remains unvalidated; cross-platform vet is compilation evidence only. After cleanup expiry, Lane may invoke Session.Stop before an unfinished Worker join completes; the process must exit, and graceful completion is not established. Process cancellation cannot be reported as domain cancellation or successful graceful cleanup after a timeout. #18/#191/M3 stay open.

## Trail review

`trail check --base origin/feat/18-wp-m3-06-public-api-events` exits 1 on seven lexical documentation conflicts: four matches against the cancellation-ordering learning (ARCHITECTURE, engineering validation, Platform README and owner README), two against SQL-driver-panic cleanup limits (engineering validation and Platform README), and one against the owner-approved Linux/native-platform scope (engineering validation). The implementation retains Lifecycle's latch-before-forwarded-cancellation path and exercises it during attempt shutdown; it does not reintroduce a direct caller-context child around Lane. SQL driver-panic subprocesses and bounded process-failure requirements are unchanged, with no graceful SQL-unwind claim. Linux remains required and native non-Linux runtime remains unvalidated/best effort. These are documentation context matches, not changes to those decisions. Informational new-learning, authorization, no-legacy, storage-path and previous-test matches likewise do not change those boundaries. The receive-pump finding is recorded as a new learning; previous learnings are untouched.
