# OBS-001 HTTP response I/O diagnostics — Linux

Scope: bounded #170 implementation on #18 integration base
`1449fbe24355d2f4cc910d6de41d5f954ce4b42b`. Native blockers #83/#84,
#120 and inherited #15 were verified closed before selection. Latest #18,
#92, #170 and #191 notes were reviewed. Shared checkout changes were preserved;
work took place in an isolated worktree.

## Behavior and evidence

`TestHTTPResponseIOFailureDiagnostics` reproduced four false-success records on
the unchanged implementation: response write error, short write, flush error
and failed hijack. All four subtests failed for `outcome:succeeded` despite the
injected response failure (focused command exit 1).

The operational HTTP wrapper now retains failure presence and emits
`outcome:failed` with static `cause:response_io_failed` after normal handler
return. Any observed response I/O failure takes precedence over the HTTP status
classification, even if the handler handles the error or continues. Status
remains the status selected by the handler; it does not prove client receipt.
Panic handling retains its existing separate classification and safe abort.
The wrapper preserves the original write count/error and flush/hijack results,
and does not retry delivery or change command outcomes.

The regression checks one JSON record, correlation, selected status and fixed
cause, unchanged I/O results, and absence of an unregistered credential-bearing
error, response body and query fixture. Existing tests retain real-loopback
success/rejection, upgrade, flush, panic-channel and sink-failure coverage.
This is **unit-tested** failure injection with existing **integration-tested**
adapter controls, not a real network failure or full product validation.

## Commands

Environment: Linux amd64, kernel `7.2.11-100.fc43.x86_64`, Go `1.26.0`,
Node `25.7.0`, pnpm `11.5.0`. Native machine/VM classification is unverified.

- Before fix, from `services/platform`:
  `go test -count=1 -run TestHTTPResponseIOFailureDiagnostics ./internal/adapter/operational`
  — FAIL exit 1, all four subtests report false success.
- After fix, from `services/platform`:
  `go test -race -count=10 ./internal/adapter/operational`
  — PASS exit 0.
- From the repository root: `make check` — PASS exit 0, including 348
  TypeScript contract tests, 346 shared fixture outcomes with Go/TypeScript
  parity, 48 SDK tests and all default Platform checks. Log:
  `/tmp/m3-http-io-check.log` (local evidence, not hosted CI).
- From `services/platform`:
  `go test -race -count=1 ./internal/adapter/httpapi ./internal/adapter/localauth ./internal/arch`
  — PASS exit 0, all three packages.
- From the repository root: `make docs-check` and `git diff --cached --check`
  — PASS exit 0. An initial docs-check invocation from `services/platform`
  failed because that directory has no Makefile target; corrected to root.
- `trail build` — PASS exit 0, 305 files, 2,427 nodes and 6,043 edges.
  `trail check --base 1449fbe24355d2f4cc910d6de41d5f954ce4b42b` — PASS
  exit 0, no conflicts, 12 advisory lexical matches reviewed. The advisories
  concern unrelated OS scope, board fields, lifecycle/coordinator races,
  migration/storage rules and test tooling; none of those behaviors changes.

Initial sandbox attempts could not write the existing Go build cache; validation
used elevated cache/loopback access. An exploratory `go doc` also attempted
module metadata network lookup; it is not test evidence.

## Limits and remaining gates

Only errors observable through wrapped Write, Flush and Hijack calls are
classified. Buffered failures after handler return, direct writes to hijacked
connections and operations through an unwrapped writer are outside this capture.
Known-secret registration/rotation behavior is unchanged. No new contract,
domain Event, startup, Work, replay or secret-delivery semantics are introduced.

Complete running-product instrumentation/channel capture remains open in #170.
#92 startup/CLI and composed acceptance remain open. #191 Work mutation mapping
still needs owner decisions and contract alignment for omitted owners, display
prefix allocation/collisions, reopen reason, and replay/version fields. Other
definition/Provider/secret and #18 Event/product gates remain open. No new
PostgreSQL, real Provider, process restart or native macOS/Windows runtime
validation was performed. #92/#170/#191/#18 and M3 remain open.
