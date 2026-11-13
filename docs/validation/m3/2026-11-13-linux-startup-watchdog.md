# Linux aggregate startup watchdog evidence

Date: 2026-11-13. Scope: #18 and accepted ADR-0068, after PR #361 merged at `5ebd4bb5fac47a9016bbecc3f85a1458bb7e77a0`. This is partial lifecycle composition, not production process acceptance.

## Change and architecture review

`services/platform/internal/platform/lifecycle.go` constructs one Lane v2.1.0 with the existing safe diagnostic handoff, caller-supplied concrete stages, one absolute startup deadline, one positive shutdown budget and an atomic readiness latch. Readiness stops the startup timer; expiry cancels startup independently of an initializer obeying its context. Timeout remains failure even when Lane returns nil after cancellation. Intentional caller cancellation is forwarded only after closing admission; caller deadline expiry remains failure. Each helper is single-use, with no retry, restart or restoration path.

Review verdict: sound within ADR-0068. No new dependency, infrastructure, contract, domain mutation or migration. Domain, Coordinator, ports and Worker logic are unchanged. Reusing Lane alone cannot express the independent startup budget: its shutdown timeout starts only after stopping begins. Applying a deadline to healthy lifetime would incorrectly stop an otherwise healthy process. Observers only perform atomic transitions and timer cancellation; no IO, arbitrary error formatting or blocking telemetry handoff is introduced. Production composition must still use the same deadline in resource initializers, order all required stages, map health/admission through existing contracts and exit unsuccessfully on returned failure without logging raw errors.

## Reproduced failure and correction

The first `go test -race -count=10 ./internal/platform ./internal/arch` failed all ten healthy-lifetime cancellation cases. A direct child of the caller context let Start observe cancellation while the readiness latch remained true, before Lane's stopping observer ran. The correction preserves caller values with `context.WithoutCancel`, forwards cancellation after closing the latch, and retains Lane as sole signal owner. A failed runner exit also closes admission before Lane's errgroup propagates failure cancellation. No test timeout was enlarged to hide this failure.

## Local validation

Linux x86_64; Platform Go 1.27.0 and Lane v2.1.0. Local results are not hosted CI.

- `go test -race -count=10 ./internal/platform ./internal/arch`: PASS after correction (Platform 35.024s; architecture 1.479s), exit 0. Log: `/tmp/amaru-startup-watchdog-race-final.log`.
- `go test -race -count=10 -run '^TestLifecycle' ./internal/platform`: PASS, exit 0 (8.571s), including caller-deadline classification. Log: `/tmp/amaru-startup-watchdog-final-focused.log`.
- `go test -race -count=100 -run '^TestLifecycleRunnerFailureClosesAdmissionBeforePeerCancellation$' ./internal/platform`: PASS, exit 0; stresses post-readiness fatal-loop admission closure. Log: `/tmp/amaru-startup-watchdog-failure-race.log`.
- `make check`: PASS, exit 0, including docs, fixture parity, SDK tests, Platform tests and cross-platform vet (compile evidence only). Log: `/tmp/amaru-startup-watchdog-check.log`. Final `make check` after the fatal-loop regression also PASS, exit 0; log `/tmp/amaru-startup-watchdog-check-final.log`.

The fake-backed actual-Lane tests cover uncooperative Start/readiness/Stop, bounded return, no later-stage admission, healthy lifetime beyond startup deadline, cancellation ordering, intentional cancellation versus startup/readiness/panic/cleanup/deadline failure, repeat-use rejection, invalid configuration, already-expired startup without acquisition, and pool availability through dependent rollback. Existing SQL driver-panic subprocesses now use Lifecycle with an uncancelled parent instead of a test-parent timeout; the child outer timeout remains only a test-harness safeguard. They require failure, withheld admission and no panic-canary output. They do not claim that corrupted database/sql internals unwind gracefully.

## Limits

No production entrypoint, final exit-code/diagnostic consumer, essential execution-loop owner, bundled PostgreSQL child owner, NATS/outbox, public admission/HTTP/discovery composition, native non-Linux execution, server/process crash or new PostgreSQL runtime validation. The helper cannot terminate goroutines that ignore cancellation, reap children or protect owned processes after abrupt exit. Lane may invoke late Stops without awaiting them after the shared cleanup budget expires; a failure return cannot be reported as graceful cleanup. Durable reconciliation and Worker cancel-and-join/no-synthetic-Result semantics remain unchanged. #18 and M3 stay open.

## Trail review

`trail check --base origin/feat/18-wp-m3-06-public-api-events` exits 1 with three lexical documentation conflicts in ARCHITECTURE, engineering validation and the Platform README against the SQL-driver-panic learning. These edits retain the learning: SQL panics stay in subprocesses, the independent watchdog bounds a stranded migration, and no graceful unwind or in-process repair is claimed. Driver-panic tests now exercise the required watchdog instead of a parent timeout. Informational auth, no-legacy, storage-path and native-platform matches do not change those boundaries. The new cancellation-ordering learning records a reproduced failure and corrected context forwarding; it does not replace or edit previous learnings.
