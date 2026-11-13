---
title: Gate Platform process readiness and bound shutdown with one lifecycle owner
status: accepted
date: 2026-11-13
deciders: Ricardo Lüders
---

# 0068. Gate Platform process readiness and bound shutdown with one lifecycle owner

## Context

[#357](https://github.com/rluders/amaru/issues/357), a child of [#18](https://github.com/rluders/amaru/issues/18), resolves the Lane candidate before the first production composition. At integration commit `3255a7c` (merged PR #356), Amaru has PostgreSQL adapters/migrations, Coordinator restart reconciliation, embedded Worker sessions and authenticated Project/settings HTTP adapters. It has **no production entrypoint**. The SDK `authfixture` server uses fake stores; the M1 PostgreSQL/NATS spike is experimental evidence, not the product infrastructure owner. Public Event transport, bundled PostgreSQL ownership and Platform NATS/outbox composition remain required work under accepted ADRs [0015](0015-postgresql-is-the-only-database-bundled-in-compact-installs.md), [0016](0016-nats-jetstream-message-bus-embedded-in-compact-installs.md) and [0045](0045-compact-installs-have-a-bundled-and-a-slim-flavor-on-one-engine.md).

The verified candidate is `github.com/rluders/lane/v2` **v2.1.0**, commit `4c139e73cecf86aeddc7dde554f40b5e6cfe54fc`, Go 1.26, MIT, with `golang.org/x/sync` v0.20.0. Amaru uses Go 1.27 and selects x/sync v0.24.0. The [isolated evidence](../../validation/m3/2026-11-13-platform-lifecycle-proposal.md) tests that selection without adding either dependency to Amaru. Source verifies stage barriers, optional completion readiness, reverse Stop, panic-to-error handling and one shared cleanup deadline. Lane cancels **all Start contexts before any Stop**; it does not keep dependencies alive for dependents automatically. Its HTTP runner binds its own listener, unlike Auth/discovery's requirement for one already-bound actual address.

## Decision

**Accepted by the product owner on 2026-11-13.** Use one Lane v2.1.0 instance at the Platform composition root for process startup, readiness and shutdown. Keep Lane imports outside domain, Coordinator, ports and Worker execution logic. Do not use Lane Supervisor/restart wrappers, WorkerRunner, Scheduler or a second lifecycle supervisor. The library does not own execution state, retries, timers, leases, fencing, recovery, Policy, Approvals or domain cancellation.

### Startup and resource ownership

The composition root wires concrete adapters directly. Sequential stages express prerequisites; concurrent starts are limited to genuinely independent components. No dependency injection framework or general dependency graph is introduced. Every initialization operation has completion-based readiness, including a one-shot migration/reconciliation function: returning from Start or launching its goroutine is insufficient. WaitReady may be called before Start and must wait for its completion/error channel.

| Stage, registration order | Resource owner and completion barrier | Cancellation and Stop ownership |
|---|---|---|
| 1. Storage | Compact bundled PostgreSQL process owner, or external connection configuration; open pool and verify connectivity. Never stop an operator-owned server. Bundled first-run initialization follows ADR-0015/0045, including startup lock and pinned binaries. Product child owner is still missing. | Cancel initialization on Start cancellation. Once acquired, keep pool/child usable until storage Stop after dependents join. Close pool, then terminate/reap only the owned child; Linux owner-death protection remains in the child owner. |
| 2. Schema | Run existing migration Apply under its lock; publish completion only on success. | Own and release partial locks on failure; never undo committed migrations or delete state. No continuing loop. |
| 3. Messaging | Embedded/external NATS connection and required stream setup; completion requires usable configured messaging. Product adapter is still missing. | Keep connections/server alive until messaging Stop; close client resources then owned embedded server, never an external broker. |
| 4. Recovery | Run Coordinator.Reconcile once against durable storage, before any lease, Worker Offer or public command admission. | Complete or cancel the transaction under existing rules. No alternate recovery implementation in Lane. |
| 5. Background services | Actual outbox/Event consumers when implemented; each ready after subscription/initialization, before Worker traffic. Do not instantiate placeholders. | Stop fetching on cancellation, settle or abandon transactions/deliveries according to durable outbox/idempotency rules, and join on Stop while pool/broker remain usable. |
| 6. Execution services | Register Coordinator session owner, then embedded Worker owner. Readiness confirms the wired session/transport is ready; no circular wait for a first assignment. | Start cancellation stops leasing and cancels attempts. Reverse Stop joins Worker first, then Coordinator/session work. Keep actual links open as required until joins complete; a bounded receive must obey context. |
| 7. HTTP and private discovery | One owner binds one loopback listener, constructs Auth from its actual address and wires authorized handlers plus existing safe middleware. Keep admission closed through initialization. Publish private discovery only after prerequisite barriers and HTTP initialization succeed; open admission as the final action. | Immediately close admission on stopping; Stop closes long-lived transport/watch sessions, drains bounded ordinary requests, closes HTTP/listener, then closes Auth and finally removes publication/releases discovery lock. Retain publication until serving has stopped. |

The table specifies the required end state, **not existing composition**. Missing accepted infrastructure is implemented under #18 before claiming a production-ready compact process; this proposal does not defer NATS/outbox or bundled PostgreSQL to a later milestone. Server/Cloud use the same ordering with external resource adapters. The SDK fixture keeps its test-only purpose; do not copy its one-second shutdown or treat it as a production implementation.

Acquire each resource inside the responsible runner Start, and release partial acquisitions even if initialization fails. Stop tolerates partially initialized owners, runs once for every started runner, and leaves persisted data intact. Unstarted stages own nothing. Avoid opening resources before Lane can track the owner; otherwise the constructor must unwind them itself. Hold pool/broker resources until Stop, not in a defer that closes them when the Start context ends.

### Readiness, deadlines and failure policy

Proposed defaults are **60 seconds for aggregate startup/readiness** and **30 seconds for aggregate shutdown**. Validate positive configured durations at bootstrap; these are composition settings, not new public flags or contracts. The startup watchdog ends when all barriers complete; its timeout must not remain attached to healthy service lifetime. Expiry reports a startup failure and initiates bounded rollback. Each initializer uses a context bounded by the remaining startup deadline. Lane's WithShutdownTimeout bounds cleanup only and cannot replace that watchdog.

The readiness latch becomes false before Start cancellation and remains false throughout shutdown. Use a fast atomic state update in the lifecycle stopping observer, with no IO or waiting. Public admission consults that latch even if a previously accepted connection still exists. `/v1/health` keeps its OpenAPI shape, authentication and compatibility rules; map internal readiness through the existing contract, never serve Lane's health JSON as Amaru's health response. Internal process readiness does not imply Provider availability or a healthy business Execution.

Any startup/readiness error, essential long-lived loop exit (including an unexpected nil return), or recovered runner panic fails the process and rolls back started stages. The concrete owner classifies an essential loop's unexpected normal return as failure; no general restart wrapper. A completed migration/reconciliation is an expected one-shot exit. Child goroutines need explicit error propagation and local panic handling; Lane only recovers panics in methods it invokes. Existing safe HTTP recovery still contains request panics without killing the process. Do not print Lane's returned error or panic value/stack at the entrypoint.

On intentional signal/cancellation, return success only after cleanup completes without failure. Startup, essential-loop, cleanup or timeout failure yields a nonzero process exit and safe diagnostic. Lane is the sole signal owner; the entrypoint owns the final exit code after Run returns. No deferred resource cleanup may rely on os.Exit running defers.

### Shutdown while work is in progress

1. Atomically close admission/readiness as shutdown begins. Lane then cancels every Start context. Stop accepting new commands/leases/Offers and new background deliveries; already committed work remains durable.
2. HTTP Stop explicitly closes Event/WebSocket/Watch connections and their subscriptions: net/http Shutdown does not close hijacked connections. Allow already-admitted ordinary handlers at most **5 seconds**, capped by the remaining global cleanup budget. Their request contexts are not children of the cancelled Lane Start context. On expiry cancel them, force-close connections and join handler/subscription goroutines within the remaining budget. Keep Auth live through authorization/settlement, then close Auth and discovery after serving stops. Never report a transaction rolled back merely because its client response was lost; idempotent replay resolves committed outcomes.
3. Embedded Worker Stop joins attempts after process cancellation. Preserve its existing cancel-and-join behavior and **no synthetic Result** for shutdown, including its distinction from explicit domain Cancel. Coordinator/session Stop joins protocol operations. Worker work is not drained to business completion; durable Steps/leases and restart reconciliation remain governed by [ADR-0042](0042-durable-execution-with-explicit-restart-semantics.md)/[0044](0044-restart-semantics-clarifications-from-the-m1-spike.md). An in-flight command may commit after dispatch stops; it is recovered next startup, not assigned during stopping.
4. Join background consumers/relay, then stop messaging; release schema resources and stop storage last. Preserve transactional outbox records rather than requiring all business Events to be published before exit. No domain cancellation request or terminal Execution mutation is invented by shutdown.

Stop contexts share one 30-second budget; they are fresh cleanup contexts, not children of cancelled Start contexts. Lane invokes late Stops even after expiry without waiting: actual completion order is guaranteed **only while joins finish within budget**. On expiry, do not claim graceful completion, retry commands or start a replacement supervisor. Return failure and terminate the process; OS/child-owner protections must prevent orphaned owned PostgreSQL/process trees even if a goroutine ignores cancellation. Lane cannot kill them. Production Linux tests must prove this; external database/broker processes are never killed. Preserve durable restart semantics on the next start.

### Safe observability and refactoring

Supply a safe slog handler that drops raw errors, panic values, stacks and arbitrary attributes **before** output; allow only trusted static component/phase labels, severity, bounded durations and known outcome classes. Avoid formatting unknown LogValuer/error values. Lane observers do only non-blocking bounded handoff/atomic updates; a full telemetry queue drops diagnostics, never blocks lifecycle. Keep slow logging/export IO out of lifecycle callbacks. Adapt the bounded handoff to #170 operational identity/redaction; process lifecycle observations are not durable domain Events. Safe diagnostic/exit classification must also cover Run's returned errors and child failures, not only slog output. Do not replace existing operational HTTP middleware with Lane RecoverMiddleware.

Remove redundant signal/error/cleanup coordination when creating the production root, rather than layering Lane over another supervisor. Retain Worker.Run, Coordinator.Serve/Reconcile, transactions and OS child ownership. No compatibility bridges for internal implementations before Amaru product v1.0. No HTTP contract or domain interface change solely to accommodate Lane; HTTPSuite is not adopted by this decision.

## Consequences

One outer owner supplies clear startup barriers and bounded reverse teardown as the process grows. The cost is a pinned dependency and a small number of resource-specific runners, an initialization watchdog, an admission latch and safe diagnostics. A custom pre-bound-listener HTTP runner is necessary; Lane HTTPRunner is insufficient for Auth/discovery. These pieces must express actual resource ownership, not become a second framework.

Before imports, the owner must approve this ADR. Before production acceptance, implement missing owners and run the failure matrix in the [evidence record](../../validation/m3/2026-11-13-platform-lifecycle-proposal.md), including real PostgreSQL process restart/crash and outbox cut points. #357 remains open through the recorded human decision; #18/#191/#20 retain their independent acceptance. Linux is required; native macOS/Windows remain best effort under the existing M3 owner decision.

Revisit Lane if adapter/watchdog/diagnostic machinery becomes a second supervisor, readiness cannot be demonstrated without circular dependencies, or dependency cancellation cannot preserve resource ownership through joins. Choose direct standard-library composition in that case; do not force adoption to reuse a library. Future version upgrades repeat source/license, selected-graph, panic/logging and lifecycle checks.

## Alternatives considered

- **Direct context, signal and error coordination:** no new dependency and sufficient for the present SDK fixture. For the planned multi-component production process it duplicates stage/readiness/rollback/Stop error collection. Retain this smaller alternative if the proposed runners cease to be thin; ownership and business shutdown semantics above still apply.
- **Lane only around HTTP, nested supervisors or automatic restarts:** splits signal/deadline ownership and can restart components against stale sessions/resources. No demonstrated need.
- **Treat startup as unready fire-and-forget tasks, or wait for Executions to finish on shutdown:** permits leasing before reconciliation or changes durable execution semantics. Neither is acceptable.

## Approval

Proposed by Codex on 2026-11-13. Accepted by Ricardo Lüders on 2026-11-13 in the repository task conversation: “Proposal at PR #358 is approved.” The approval authorizes implementation of this decision; no dependency import or product lifecycle implementation is included in this PR. Production failure-matrix validation remains required before production acceptance.
