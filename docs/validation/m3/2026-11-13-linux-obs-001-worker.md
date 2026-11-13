# Linux OBS-001 Worker Runner instrumentation

Date: 2026-11-13. Runner: Codex. Base: #18 integration `7c75abf07b2433f17f39ca0ef44429e18c52ff7f`, including merged PR #363. Candidate: the commit containing this record on `feat/170-obs-001-worker-instrumentation`. Refs [#170](https://github.com/rluders/amaru/issues/170), [initial logging evidence](2026-11-12-linux-obs-001.md).

## Scope and prerequisites

GitHub readback verified #170 open; native dependencies #83/#84 and inherited #18 dependency #15 closed. #92 adapters are already merged and its remaining startup/CLI/product composition belongs to #18. #219 was inspected but stopped before edits because catalog mutation/conflict semantics need human design review. Neither issue is implemented by this change. Existing branches/worktrees and the dirty shared checkout were preserved.

`operational.ObserveRunner` wraps an existing embedded Runner and hands off correlated static diagnostics without adding logging imports to Worker logic. The bounded channel is supplied and drained by composition. Nil/full queues drop records; keep the channel open until Worker attempts join. Drain through `Logger.Write` with `WithIdentity`; logger IO and sink-error handling belong to that consumer, outside the attempt goroutine. The attempt ID is the correlation ID, with trusted Instance identity and Assignment Execution/Step IDs. Project ID is absent from Assignments: composition may enrich it from authoritative Execution state, never from Provider input. No request-correlation propagation or Project lookup is invented here.

Records use operation `worker.runner`, with `started` then a static local outcome: `succeeded`, `failed`, `cancelled`, `context_cancelled`, `timed_out`, `unknown_outcome` or `panicked`. They describe Runner activity, not a committed domain result or cancellation cause. A context cancellation alone cannot distinguish process shutdown, Lease revocation and domain Cancel. A success returned at the deadline retains the Worker's existing success classification. Inputs, remote-job IDs, idempotency keys, Provider error code/detail and panic values are omitted, without invoking their formatting methods. Outcomes, contexts, Assignments and panic propagation remain unchanged.

Architecture review: **sound** for this bounded adapter slice. The wrapper imports the existing Runner/ports interfaces outside domain, Worker and Coordinator; those packages gain no dependency. No contract, schema, infrastructure, vendor or permission change. The simpler synchronous Logger call could block Worker completion on sink IO; the bounded handoff avoids that coupling. Queue loss is an operational diagnostic limitation, never a substitute for durable Events. Partial diagnostics on process crash are expected. No restart, execution retry or domain mutation is added.

## Environment

Linux x86_64, kernel `7.2.11-100.fc43.x86_64`; `systemd-detect-virt` reports `container-other`, so physical/full-VM classification is unverified. Go 1.27.0, Node v25.7.0, pnpm 11.5.0. Local results are not hosted CI or a claim of the native physical/full-VM milestone matrix.

## Checks

All commands below ran locally on Linux. Go commands run from `services/platform`; Make commands run from the repository root.

| Command | Outcome |
|---|---|
| `go test -race -count=1 ./internal/adapter/operational ./internal/worker/embedded ./internal/arch` | PASS, exit 0; all three packages. Initial sandbox attempt failed on prohibited loopback sockets; rerun with socket access passed. An earlier invocation also failed on a read-only default Go cache; neither sandbox failure is a product test pass. |
| `go test -race -count=20 -run '^TestRunnerDiagnostics' ./internal/adapter/operational` | PASS, exit 0; four new top-level tests and ten subtests per repetition, no skipped cases. |
| `make check` | PASS, exit 0; Go format/vet/tests, docs, spike cross-vet, contracts/proto, 346 matching Go/TypeScript fixture outcomes, 44 SDK tests (zero failed/skipped), Platform host/cross-vet and PostgreSQL-tag compile check. Log: `/tmp/amaru-170-worker-check.log`. |
| `make docs-check` (after final evidence update) | PASS, exit 0. |
| `git diff --check` | PASS, exit 0. |
| `trail build` | PASS, exit 0; refreshed local graph (not committed). |
| `trail check --base origin/feat/18-wp-m3-06-public-api-events` | Exit 1, one lexical conflict on Platform README wording against the earlier finding that Lane cannot unwind SQL internals. This wrapper makes no SQL cleanup claim, adds no SQL calls and leaves existing bounded lifecycle/driver-panic behavior unchanged. Informational matches do not change cancellation, storage paths, clone provenance or authorization. |

No tests were disabled, skipped or changed to hide a failure.

## Evidence and remaining gates

- **Unit-tested:** exact passthrough, static outcome classification, missing identity, nil/unbuffered/full queue behavior and panic propagation without formatting. Provider payload fixture values are absent from parsed JSON with no secret registration; the existing logger tests separately verify known-value/pattern redaction.
- **Integration-tested:** actual embedded Worker and in-process transport with fake Provider success/failure/panic, explicit Cancel, process-context cancellation and deadline. Tests assert unchanged fencing/result identity, timeout classification, cancel-and-join and no synthetic shutdown Result; queued records drain through the actual JSON logger. Existing HTTP/auth tests cover rejected requests.
- **Not validated:** production wiring/consumer, Project identity enrichment, request-to-attempt correlation, full command instrumentation, product stdout/stderr capture, real Provider diagnostics and native macOS/Windows. No new PostgreSQL runtime check: no store/transaction/schema change; PostgreSQL tests are compile-checked by `make check`. Node 22 was not used.

The existing Worker panic boundary may still place a raw panic string into its protocol Result; this wrapper does not inspect or emit that string. No claim is made that arbitrary Provider Result delivery or all process output is secret-safe. Product composition must retain existing redaction boundaries and capture every output channel before #170/#18 acceptance. #170, #18 and M3 stay open; no merge or issue closure is authorized by this record.
