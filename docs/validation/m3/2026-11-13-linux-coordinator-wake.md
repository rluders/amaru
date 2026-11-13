# Coordinator wake subscription — Linux, 2026-11-13

Scope: a bounded scheduling correction under PRJ-002 (#191) and WP-M3-06 (#18), based on integration commit `b490ed6`. No new architecture, protocol, contract, storage, dependency or infrastructure decision.

## Reproduction and correction

`Coordinator.Serve` previously read the queue before capturing the channel returned by `c.wake()`. `Wake` closes and replaces that channel. A submission committed after an empty queue snapshot but before subscription could therefore leave queued work waiting for unrelated traffic despite available Worker credit.

`TestServeWakeDuringEmptyQueueRead` uses a fake Execution store wrapper to capture an empty snapshot, submit through the real coordinator, and return the earlier snapshot. The link sends one Offer and no further messages. The regression requires an Assignment with its Lease already committed. Its two-second context bounds failure; no sleep or increased existing timeout is used.

Before the production change, `go test -count=1 -run TestServeWakeDuringEmptyQueueRead -v ./internal/coordinator` failed, exit 1, after two seconds: queued Step not assigned. Capturing the channel at the beginning of each Serve loop, before attempting assignment, preserves the wake and makes the regression pass.

The PR #352 independent-Project provider-failure timeout motivated investigation. This regression proves the scheduling window, but does not prove it caused that particular timeout. Passing reruns alone were not treated as evidence against the bug.

## Local verification

Environment: Linux x86_64, Go 1.26.0, Git 2.56.0. Commands below run from `services/platform` unless stated otherwise.

- `go test -race -count=100 -run 'TestServe(WakeDuringEmptyQueueRead|Shutdown)' ./internal/coordinator` — PASS, exit 0. Covers the deterministic scheduling window and existing shutdown/error preservation regressions.
- `go test -race -count=100 -run TestProjectsProgressIndependently ./internal/worker/embedded` — PASS, exit 0. Both in-process and serialization-backed links; fake Providers and storage.
- Repository-root `make check` — PASS twice, exit 0, including after adding this validation record and its engineering-guide link. Logs: `/tmp/amaru-191-wake-check.log` and `/tmp/amaru-191-wake-final-check.log`.
- With `AMARU_TEST_PG` set to the existing disposable loopback server, `go test -race -count=1 -tags pg ./...` — PASS, exit 0. The full Platform suite includes real PostgreSQL assignment, concurrent capacity limits, shutdown, protocol durability and process-crash recovery; PostgreSQL package completed in 122.036 seconds. Log: `/tmp/amaru-191-wake-pg.log`. The disposable PostgreSQL 17.5 server was stopped afterward; tests created and dropped their own databases.

The deterministic regression and independent-Project checks are fake-backed integration evidence. The PostgreSQL run adds real database/process evidence for the existing suites, rather than a PostgreSQL-specific reproduction of the lost-wake window. These are local results, not hosted CI or full product validation.

## Limits and remaining work

No native macOS/Windows run, real Provider run, or composed product HTTP/SDK validation. Linux remains the required M3 target; other platforms are best effort. This does not introduce distributed scheduling across Platform processes. Public Project mapping under accepted ADR-0067 and all other #191/#18/M3 acceptance gates remain pending.
