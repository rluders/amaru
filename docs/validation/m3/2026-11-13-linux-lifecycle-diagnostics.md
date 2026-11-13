# Linux lifecycle diagnostic handoff (#18, #170)

Date: 2026-11-13. Runner: Codex, local Linux x86_64 workspace (machine kind unverified), kernel 7.2.11-100.fc43.x86_64. Base: #18 integration `0b10575` after accepted ADR-0068; implementation branch `feat/18-lifecycle-safe-diagnostics`. Toolchain: Go 1.27.0 in the Platform module (root launcher 1.26.0), Node v25.7.0, pnpm 11.5.0, buf 1.69.0.

## Implemented scope

`internal/adapter/operational.LifecycleHandler` implements the slog output boundary required by accepted [ADR-0068](../../architecture/adr/0068-platform-process-readiness-and-bounded-shutdown.md). It uses the existing operational Record/Logger and the standard library, with no new service or lifecycle supervisor. Only a static operation and severity classification reach a caller-owned bounded channel. All raw messages, source locations, names, errors, panic values, stacks, groups and attributes are omitted without resolving values. A full or nil queue drops diagnostics; the callback never calls a sink or acquires its lock. The queue must remain open until producers join.

Lane v2.1.0 is pinned for the actual failure-path tests under ADR-0068 (MIT, verified in the downloaded module's LICENSE; source commit `4c139e73cecf86aeddc7dde554f40b5e6cfe54fc`). `lane.go`/`recovery.go` verify the WithLogger integration and raw panic/error emission. Its only selected transitive dependency, x/sync, remains v0.24.0. Production handler code does not import Lane; domain, Coordinator, ports and Worker logic are unchanged.

## Checks

- `go mod tidy`: PASS, exit 0; adds Lane v2.1.0 checksums without changing x/sync.
- From `services/platform`, `go test -race -count=1 ./internal/adapter/operational`: PASS, exit 0, including three new top-level tests. They cover malicious LogValuer/Stringer/error values, attached/grouped attributes, static severity mapping, known-unregistered secret omission through the existing JSON writer, concurrent producers with absent/stalled consumers and real Lane runner error/panic/cleanup failures.
- `go list -m github.com/rluders/lane/v2 golang.org/x/sync`: PASS, exit 0; v2.1.0 and v0.24.0 respectively.
- From `services/platform`, `go test -race -count=20 ./internal/adapter/operational`: PASS, exit 0, no skips.
- From the repository root, `make check`: PASS, exit 0; includes docs-check, 346 matching Go/TypeScript fixture outcomes, 44 SDK tests and all default Platform suites. Linux local output: `/tmp/amaru-lifecycle-diagnostics-check.log`. Cross-vet is a compatibility signal, not native macOS/Windows evidence. PostgreSQL-tagged code is vetted, not runtime-tested.
- Final `make docs-check` and `git diff --check`: PASS, exit 0. `trail check --base origin/feat/18-wp-m3-06-public-api-events`: PASS, exit 0, no conflicts; informational lexical matches do not change authorization, clone provenance, subprocess cleanup, schema or Linux-first rules. An earlier default-base check included the whole integration branch and was truncated; the designated PR-base check covers this eight-file slice.

The first sandboxed attempt could not write the existing Go build cache and did not run tests; rerunning with cache access produced the passing result above.

## Boundary review and remaining validation

Verdict: sound as a diagnostic handoff. It preserves existing JSON redaction and identity handling, introduces no public contract/state mutation or infrastructure, and keeps IO outside lifecycle callbacks. The simpler direct slog-to-writer option would let a stalled writer block lifecycle failure/cleanup and could format unknown values. A general attribute sanitizer or new asynchronous logging service is unnecessary here.

Evidence level: **unit-tested** handoff/security behavior and **integration-tested** real Lane failure diagnostics with the existing operational JSON writer. This is local evidence, not hosted CI or real product validation. Deliberate drops lose diagnostic detail under saturation; this is preferable to blocking lifecycle progress under ADR-0068. Severity labels describe log levels, not authoritative process outcomes.

The future composition owner must drain with trusted Instance/correlation identity, own consumer shutdown, close admission before cancellation, and classify Run's returned error without printing it. The handler cannot prevent Lane from formatting a panic before calling slog; it prevents that data reaching the output/queue. It does not sanitize observer payloads, wrap child goroutines, or protect a caller that independently logs raw returned errors. Lane imports here are test-only; there is no production consumer, bootstrap, signal owner, readiness latch, HTTP shutdown, bundled PostgreSQL owner, messaging/outbox or Event transport. Those gates and the ADR-0068 production failure matrix remain under #18, with #92/#170/#191 and M3 open. PostgreSQL runtime, process restart/crash, real Provider and native macOS/Windows behavior were **not validated** by this slice; no persistence behavior changes.
