---
title: Aggregate terminal Executions and drain failed runs without discarding human waits
status: accepted
date: 2026-11-12
deciders: Ricardo Lüders (product owner)
---

# 0063. Aggregate terminal Executions and drain failed runs without discarding human waits

## Context

[Bug #335](https://github.com/rluders/amaru/issues/335), discovered during [#334](https://github.com/rluders/amaru/pull/334), blocks composed lifecycle acceptance in [#18](https://github.com/rluders/amaru/issues/18). At integration commit `d546445`, `coordinator/serve.go` commits terminal Steps but only calls `settleCancel`; `Reconcile` also only settles requested cancellation. A failed single-Step Execution remains `running` after restart. The temporary reproduction and its limits are recorded in the issue. No runtime fix is included here.

[ADR-0052](0052-adapt-the-prototype-execution-manager-into-the-execution-coordinator.md) names the predecessor as a behavioral reference. Verified read-only against baseline `c8a2f6e983beba59edc446f36df2f442edaa9855`: `runtime/internal/app/steps.go`, `sequence.Wait`, stops at the first failed Step; `manager.go`, `Manager.run`, gives an explicit cancel request precedence over the result. Their Git blobs are respectively `cbc056afaf8f47630a32faa7ab1aebf5f78fe0ce` and `1670a2bda8dc0982744c7353c02c16c1e71dccef`. No predecessor code is copied or modified.

Amaru has multiple Worker-held Steps rather than that sequential goroutine. Its accepted [ADR-0042](0042-durable-execution-with-explicit-restart-semantics.md), [ADR-0044](0044-restart-semantics-clarifications-from-the-m1-spike.md) and [domain rules](../DOMAIN_MODEL.md#52-step-execution) retain interrupted and timed-out Steps for a human or named Policy rule. Simply failing the parent would strand live work or make those waits unrecoverable. Simply waiting for all Steps while continuing dispatch can start consequential work after a failure. Neither adaptation is specified by the accepted documents. This decision supplies the missing rules; it does not supersede their human-control requirements.

## Decision

**Accepted; runtime implementation and verification pending.** The Platform coordinator owns aggregation and failure drain. Workers continue reporting Step outcomes through the existing port. No new engine, retry mechanism, Provider dependency or client-owned state is introduced.

1. **Choose the parent outcome from durable state.** Do nothing to an already terminal Execution. A committed explicit Execution cancellation request takes precedence over ordinary failure until the parent is terminal; the existing cancellation behavior remains authoritative. Without that request, any `failed` or `refused` Step makes the eventual outcome `failed`. Otherwise any `cancelled` Step makes it `cancelled`; otherwise all Steps must be `succeeded` or `skipped` for `succeeded`. Empty Step sets do not auto-complete. Concurrent cancellation versus finalization is resolved by the first valid commit against the parent version; cancellation cannot overwrite an already terminal parent.
2. **Stop a failed run safely.** A persisted failed/refused Step is the durable drain condition. Prevent new leases for that Execution. Cancel its queued and pending Steps, withdraw awaiting-Approval Steps and void their requested Approvals without recording a human decision. Signal cancellation to Worker-held Steps through the existing cancellation port; preserve results that won the race. Never mark the parent terminal until every Step is terminal. Duplicate drain passes must not duplicate cancellation Events or Approval voids. Do not add automatic retries or Workflow rerouting.
3. **Preserve human waits.** Ordinary failure drain never abandons an `interrupted` or `timed_out` Step. The parent remains `waiting` while either exists, even alongside failure. A human or named Policy rule can abandon it; an explicit Execution cancellation retains its existing authority. A resume request for a failed run is refused without changing the Step, so it cannot appear accepted and then be silently cancelled by drain. The person can abandon the Step or cancel the Execution instead. Healthy Executions retain their existing resume behavior.
4. **Represent completion without fabricated execution.** Permit `queued → failed` for a rejected Approval/refusal before any Step starts and `queued → succeeded` for a nonempty all-skipped run. Permit `waiting → succeeded` when all Steps have actually succeeded or been skipped. Add `pending → cancelled` only to support cancellation/drain before planning finishes. Preserve `StartedAt == nil` when no Step ever ran; set `EndedAt` when the parent finishes. Update the domain diagram and transition tests with the implementation. These accepted extensions are not implemented by this ADR change.
5. **Keep one safe failure reason.** Use the failure from the lexicographically smallest failed/refused Step ID for deterministic aggregation across store ordering. Redact the chosen error again before persistence. Use a fixed safe code/message for a refusal or missing error; never copy raw Provider output, Approval rationale or arbitrary refusal detail into the parent Event. A terminal Event identifies the outcome without exposing that text.
6. **Serialize lifecycle decisions durably.** Lease admission, explicit cancellation, Approval decisions, human resume/abandon and aggregation must validate the parent and relevant Step versions in the same store transaction. A lease committed before failure may need cancellation; a lease after the durable drain condition must be refused. A stale all-terminal snapshot cannot finish a parent after a concurrent resume. Commit parent state, timestamp and exactly one terminal Execution Event atomically; retry optimistic conflicts by reading fresh state. Do not rely on a process mutex for correctness across sessions or restarts. Update store implementations only where their current contract cannot enforce these checks, with isolated PostgreSQL verification.
7. **Repair crash windows.** Settle after terminal Worker results and every application transition that can end a Step, including Approval rejection and human abandonment. Startup reconciliation repairs committed final Steps whose parent was not aggregated, and resumes drain from persisted failure. Apply ADR-0042/0044 recovery rules without re-running work just to aggregate it; cancellation of held work retains the existing Worker/remote-job limitations. Repeated recovery, duplicate results and stale deliveries must not duplicate terminal Events or overwrite outcomes. Acknowledgement follows the required durable writes; a retry repairs a partial drain.

## Consequences

Owner approval unblocks implementation of #335, not its closure. No runtime behavior is changed by merging this decision. #18 and #191 remain open, as does M3 exit.

Implementation must cover single-Step success/failure, all-skipped and pre-start rejection/refusal, multi-Step failure with queued/Approval/held work, failure mixed with cancelled Steps, timeout/interruption plus failure, resume versus drain, explicit cancellation races, duplicate/stale results, concurrent final results and crashes between final Step and parent commits. Run over in-process and serialized links with fake Workers and against an isolated PostgreSQL store for transaction/concurrency behavior. Keep #334's independent-Project cases green. Run focused race tests and `make check` on Linux; record unrun Providers and other OSes honestly.

This adds lifecycle coordination, but no public fields, configurable failure modes, retries or parallel-branch feature. If implementation needs a new public Event kind, contract field or store guarantee, review its consumers and fixtures before exposing it. Remote process termination is not proved by a terminal store row. New rerouting/retry behavior or different failure precedence requires a later explicit decision.

## Alternatives considered

- **Finish only single-Step parents:** repairs the reproduction but leaves #335's multi-Step and restart acceptance incomplete.
- **Wait for all Steps while dispatching the rest:** smaller code, but can perform consequential work after an ordinary failure and diverges from the predecessor's stop-on-failure behavior.
- **Fail the parent immediately:** strands Worker-held work and violates recoverable human waits.
- **Reuse explicit Execution cancellation wholesale:** incorrectly turns failure into cancellation and automatically abandons interrupted/timed-out Steps.
- **Keep the whole predecessor sequence runner:** conflicts with durable Worker coordination and is unnecessary code migration.

## Approval

Proposed by Codex on 2026-11-12 for #335. Approved by Ricardo Lüders (product owner) on 2026-11-12 in the repository task conversation: "Accept the proposed. Update and merge." Recorded in [PR #336](https://github.com/rluders/amaru/pull/336). Acceptance authorizes implementation; it is not evidence of a fixed runtime.
