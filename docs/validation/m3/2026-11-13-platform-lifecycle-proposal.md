# Platform lifecycle proposal: isolated evidence and acceptance plan

Date: 2026-11-13. Scope: [#357](https://github.com/rluders/amaru/issues/357), [accepted ADR-0068](../../architecture/adr/0068-platform-process-readiness-and-bounded-shutdown.md). Baseline: #18 integration `3255a7c4e16d4d43bacb05596aba65e6977974a4`, after merged PR #356. Owner approval is recorded in the ADR; this record is **not** production Platform lifecycle validation or production composition.

## Source-backed findings

- [Lane v2.1.0 source](https://github.com/rluders/lane/tree/4c139e73cecf86aeddc7dde554f40b5e6cfe54fc): `lane.go` cancels all Start contexts before reverse Stop, uses one cleanup deadline, does not await late Stops and treats an early nil Start as completion. `runner.go` permits WaitReady before Start. `events.go` delivers observers synchronously; `recovery.go` logs raw panic values/stacks and returns unsafe-to-print PanicError. `runners/http.go` owns its own bind. These are constraints, not guarantees supplied by Amaru.
- The source LICENSE is MIT; module Go requirement is 1.26 with x/sync v0.20.0. The isolated module pins Lane v2.1.0 and selects x/sync v0.24.0, matching Amaru's existing selection; Go 1.27 satisfies the requirement. No Platform module/lockfile changes.
- Current `services/platform/internal/coordinator/reconcile.go` requires Reconcile before leasing. `internal/worker/embedded/worker.go` cancels and joins attempts and suppresses shutdown Results; domain Cancel differs. Existing parity/recovery tests exercise fake-backed restart semantics.
- `internal/adapter/discovery/discovery.go` Publish requires a bound listener; Publication.Close follows server stop. `localauth.New` takes that actual address; Auth.Close invalidates sessions/watches. The isolated HTTP proof below uses net/http and Lane, **not** these Auth/discovery adapters together. Their existing tests are checked separately; product composition is unvalidated.
- The only server entrypoint, `internal/adapter/localauth/testserver/main.go`, has the `authfixture` build tag and fake stores. Accepted bundled PostgreSQL/NATS/outbox owners are not production-composed. Do not infer product startup from passing HTTP adapter tests.

## Reproducible isolated proof

These text artifacts are an experiment, not a new package, validation target or application implementation:

- [Go test source](357-lifecycle-proof_test.go.txt)
- [Pinned external module](357-lifecycle-proof.go.mod.txt)
- [Module checksums](357-lifecycle-proof.go.sum.txt)

From the repository root, copy them to a fresh temporary directory and run the external module. Requires Go 1.27 and cached modules or network access for pinned module download; no agents, paid APIs, database or broker. Network use during tests is loopback only.

```sh
proof_dir="$(mktemp -d /tmp/amaru-lifecycle-proof.XXXXXX)"
cp docs/validation/m3/357-lifecycle-proof_test.go.txt "$proof_dir/lifecycle_test.go"
cp docs/validation/m3/357-lifecycle-proof.go.mod.txt "$proof_dir/go.mod"
cp docs/validation/m3/357-lifecycle-proof.go.sum.txt "$proof_dir/go.sum"
(cd "$proof_dir" && GOWORK=off go test -mod=readonly -race -count=10 -v ./...)
(cd "$proof_dir" && GOWORK=off go list -m all)
```

The four tests demonstrate:

1. All service cancellation can occur before Stop while a fake storage resource stays open until the dependent has joined. The storage owner closes only in its Stop. This is not a real PostgreSQL lifecycle test.
2. A completion barrier prevents a later stage from starting; failed readiness stops the started stage without starting the later stage.
3. A custom owner serves and closes one pre-bound loopback listener; its actual address remains unchanged. No second bind or Lane HTTPRunner is used.
4. A fail-closed slog handler emits a fixed message without forwarding raw attributes/panic values/stacks; Lane still returns a typed PanicError which must not be printed. The buffer sink is local/in-memory. Production non-blocking telemetry, safe identity labels and sink-failure handling remain implementation work under #170/#18.

## Local results

Local Linux evidence, never hosted CI:

- `make docs-check` — passed (`doccheck: ok`).
- `make check` — passed in the isolated #18 worktree after the sandbox blocked the TypeScript fixture subprocess; rerun with approval outside the sandbox. It includes doccheck, Go vet/tests (including Windows and macOS cross-vet), contract lint/format/breaking checks, TypeScript typecheck and tests (44 passed), Platform vet/tests, and the remaining repository checks.
- The reproducible isolated proof commands above — `go test -mod=readonly -race -count=10 -v ./...` passed (4 tests × 10 runs; includes loopback listener), and `go list -m all` resolved exactly Lane v2.1.0 and x/sync v0.24.0. The listener test was blocked by sandbox network policy on the first attempt and passed on the approved rerun.

These results validate the documentation and isolated compatibility proof only. They do not validate production Platform composition or the failure matrix below.

## Production verification required after approval

| Scenario | Required observation before #18 production acceptance |
|---|---|
| Each stage fails or panics after a partial acquisition | No subsequent stage/admission; all acquired resources cleaned once, durable data retained, safe nonzero exit. |
| Startup readiness never completes or fails | Aggregate startup deadline triggers rollback; stopped process never becomes ready; watchdog disarmed after successful startup. |
| Migration/Reconcile held at a barrier | No Worker Offer/lease or public command; after completion startup proceeds once, without duplicate recovery. |
| Actual pre-bound listener/Auth/discovery | One address, exact Host/Origin/session/CSRF controls, private lock/permissions; publication failure keeps admission closed; remove publication after serving stops. |
| Signal/cancellation during a real running attempt | Stop admission/leasing, cancel/join real process tree, no fabricated Result or domain Cancel; persisted restart behavior matches ADR-0042/0044. |
| In-flight command commits while shutdown begins | No premature DB close, no double command effect, idempotent replay returns committed outcome after restart even if response was lost. |
| Hung handler, Worker, consumer or Stop; cleanup deadline | Admission remains closed; deadline causes nonzero exit; no claim of ordered completion after timeout; no orphaned owned child/process tree on Linux. |
| Essential loop returns nil or error; storage fails | Fail the process safely; no restart supervisor, stale session or continued admission. Expected one-shot initialization remains distinct. |
| Error/panic/credential canaries and blocked/full log sink | Raw messages, values, stack, paths and credentials absent from output; lifecycle observers and cleanup do not block on diagnostics. |
| Long-lived Event/WebSocket/watch sessions | Explicit transport/subscription close and join, no untracked hijacked connection; Auth shutdown follows bounded request settlement. |
| Real PostgreSQL owner death/restart and NATS/outbox cut points | Owned child reaped; no external service killed; durable state/leases/definitions recover; commit/publish/ack crashes cause no lost or duplicated side effects. |
| External services and partial startup rollback | Disconnect only owned clients; never stop external PostgreSQL/NATS; no state deletion or migration rollback. |

Use fake Providers and deterministic barriers for normal tests. Real PostgreSQL/NATS/process crash evidence is separate from fake-backed lifecycle tests. Linux is required for M3; native Windows/macOS are best effort under the recorded owner decision, not implied by cross-vet. No new claim of remote Worker, Provider execution or complete public Event transport.

## Architecture review and remaining validation

Verdict before owner approval: **needs human decision**. One outer supervisor respects dependency direction and leaves durable business state with the Platform; explicit ownership handles Lane's cancellation semantics. No new domain contract, infrastructure choice, trust grant, wire format or migration from the predecessor. Accepted storage/messaging infrastructure requirements remain unchanged. Direct standard-library composition is the fallback if adapters become a second supervisor. A custom HTTP owner, admission latch, startup watchdog and safe diagnostics are concrete costs, not library features supplied automatically. The owner approved ADR-0068 on 2026-11-13; the implementation and production validation gates below remain open.

The isolated proof supports the approved ownership pattern only. Full Lane-in-Amaru runtime composition, PostgreSQL restart/crash, NATS/outbox and actual Auth/discovery plus Lane integration were **not validated**. The approved decision and numeric default budgets do not claim production behavior. #357 remains open for implementation evidence; #18/#191/#20 retain their independent acceptance gates.
