# OBS-001 Platform Lifecycle diagnostic composition

Date: 2026-11-14. Local Linux validation by Codex; not hosted CI.
Base: `feat/18-wp-m3-06-public-api-events` at
`c3f831391f025f1c14236418471276efb64b0a3d` (merged PR #389).
Branch: `test/170-lifecycle-diagnostics`. Tested source is this PR's test file
on that base; no production code changes.
Refs [#170](https://github.com/rluders/amaru/issues/170), parent
[#18](https://github.com/rluders/amaru/issues/18), and the
[selected assessment](2026-11-14-m3-agent-and-logging-assessment.md).

## Scope and evidence

`TestLifecycleDiagnosticComposition` in
[lifecycle_diagnostics_test.go](../../../services/platform/internal/platform/lifecycle_diagnostics_test.go)
exercises the actual Platform `Lifecycle`, Lane v2.1.0, diagnostic handoff and
existing operational JSON logger with fake cooperative stage owners. Eighteen
subtests cross buffered/full/nil queues with Start error, readiness error,
Start panic, Stop error, failure after aggregate readiness and intentional
cancellation. This is **integration-tested** component composition with
**unit-tested** fake failure/admission behavior; no real Provider is involved.

Buffered cases require error severity for failures, static lifecycle operation
and severity only in queued records, and one valid JSON line per drained record.
Every JSON line has exactly the existing five fields, a parseable timestamp,
and trusted drain-owned Instance/correlation identity. A different identity in
the producer context cannot replace that identity. Distinct unregistered stage,
error and panic canaries, a registered synthetic credential and stack data are
absent. Lifecycle data is omitted before serialization; this does not claim
redaction of arbitrary unknown/encoded secrets by the logger.

Errors remain classifiable with `errors.Is`/`errors.As` without formatting raw
returned errors. Failed readiness prevents dependent admission. Runtime failure
and cancellation close admission before peer cancellation; Stop and Start
completion are observed before draining. Every case must return within a
three-second test bound (two-second startup deadline, 250 ms shutdown budget).
No consumer runs during Lifecycle execution. A prefilled one-slot queue retains
its sentinel, and nil/full queues cannot delay completion. Only after Run and
cooperative producers join does the test owner close and drain the queue.
These cases do not authorize closing a queue while abandoned producers exist.

No production bug was established. The initial new-test run failed because the
fake advertised readiness before Start failure/panic settled; its readiness now
waits for cancellation in those cases. This is a test correction, not a runtime
change. The initial existing-suite race run also failed on sandbox-denied
loopback sockets; the rerun used loopback access.

## Environment and commands

Fedora Linux 43 x86_64, kernel 7.2.11-100.fc43.x86_64;
`systemd-detect-virt`: `container-other` (physical/full-VM classification
unverified). Root Go 1.26.0; Platform module selects Go 1.27.0. Node 25.7.0,
pnpm 11.5.0. No new dependencies.

- From `services/platform`, `GOCACHE=/tmp/amaru-170-lifecycle-cache go test -race -count=1 -v ./internal/platform ./internal/adapter/operational`:
  PASS, exit 0, 40 top-level tests, 18 new composition subtests, no skips.
  Log: `/tmp/amaru-170-focused-rerun.log`.
- From `services/platform`, `GOCACHE=/tmp/amaru-170-lifecycle-cache go test -race -count=20 -run '^TestLifecycleDiagnosticComposition$' ./internal/platform`:
  PASS, exit 0 (360 composition subtest executions).
  Log: `/tmp/amaru-170-repeat.log`.

- From the repository root, `GOCACHE=/tmp/amaru-170-lifecycle-cache make check`:
  PASS, exit 0. Contract parity: 346 identical fixture outcomes; TypeScript
  contracts 348/348, SDK 48/48; Platform tests, host/Windows/macOS and
  PostgreSQL-tag vet passed. Log: `/tmp/amaru-170-check.log`.
- `GOCACHE=/tmp/amaru-170-lifecycle-cache make docs-check`: PASS, exit 0,
  `doccheck: ok`.
- `git diff --check`: PASS, exit 0.

## Remaining gates and next slice

Read-only native GitHub queries confirmed #170 OPEN, direct #83/#84 CLOSED,
and parent #18's #15 CLOSED. PR #389 is MERGED at the exact base above.
This bounded verification does not complete #170, #18 or M3. Product startup,
complete command/Worker instrumentation and stdout/stderr capture, process exit,
real Provider diagnostics, PostgreSQL runtime and native macOS/Windows are
**not validated** by this change. Cross-OS vet is not native runtime evidence.

Recommended next M3 slice: assess the remaining #170 product-entrypoint
composition and consumer ownership against the existing lifecycle failure
matrix, then select one bounded instrumentation/channel-capture gap. Do not
invent a product server or infrastructure to satisfy a test. Agent authority
for #219 remains deferred and requires a human decision before the dependent
persistence/API slice; this diagnostic verification makes no such decision.

Shared checkout preserved in an isolated worktree. No merge, issue closure,
Project mutation or issue/PR comment was performed.
