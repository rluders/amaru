---
title: Durable execution is Amaru's own state machine in PostgreSQL, with explicit restart semantics
status: accepted
date: 2026-10-25
deciders: Ricardo Lüders
supersedes: 0014
---

# 0042. Durable execution is Amaru's own state machine in PostgreSQL, with explicit restart semantics

## Context
- [ADR-0014](0014-durable-execution-on-own-state-machine-over-postgres.md) chose Amaru's own durable state machine persisted in PostgreSQL. The product-owner answer to [Q1](../OPEN_QUESTIONS.md#q1-durable-workflow-technology) was "Temporal is way too overpower, let's use Postgres". That choice is kept unchanged.
- Its restart rule said non-resumable Steps "follow their retry Policy". The [resilience view](../RESILIENCE.md), the [architecture](../ARCHITECTURE.md) and FEAT-EXE-002 instead say a non-resumable Step ends `interrupted` and is never silently re-run. A retry Policy alone does not make a re-run safe: an agent Step that already pushed a branch would push again ([Q26](../OPEN_QUESTIONS.md#q26-restart-semantics-for-non-resumable-steps)).
- PRD principle: "never silently re-run" side effects; NFR-9 (no lost or duplicated work across restarts).
- Delegated in the 2026-10-25 planning prompt.

## Decision
Everything in ADR-0014's decision stays, restated here:
- The Platform runs Workflows with **its own durable state machine**, adapted from the prototype coordinator. There is no external workflow engine.
- All execution state lives in PostgreSQL, written transactionally:
  - Execution and Step Execution rows with explicit states and a monotonic version for optimistic concurrency.
  - The ordered Event log per Execution (an append-only table, `seq` per Execution).
  - Timers (retry back-off, Lease expiry, Approval deadlines, schedules) as rows with a due time, claimed with `FOR UPDATE SKIP LOCKED`.
  - Approval waits as persisted gates. A waiting Step holds no process or goroutine.
  - Idempotency keys for every externally visible side effect.
- Every state transition and the messages it causes are committed together through a **transactional outbox**, relayed to the message bus ([ADR-0016](0016-nats-jetstream-message-bus-embedded-in-compact-installs.md)).

Restart semantics are replaced by this explicit rule. On start, the Platform reconciles every non-terminal Step Execution from the database:
1. A Step **waiting** on an Approval, a timer or a Lease that is still valid keeps waiting. Nothing ran, so nothing is lost.
2. A Step with a recorded remote job id whose Provider declares `Resumable` is **resumed** (prototype semantics).
3. A Step whose Capability is declared **idempotent** (its side effects are guarded by idempotency keys) is retried under its retry Policy, as a new Lease.
4. Every other running Step ends **`interrupted`** with reason `platform_restarted`. It is never re-run automatically. An interruption does not consume an attempt (FR-17). A human, or a Policy rule that names the Step, resumes it explicitly.

The same rule applies when a Lease expires because a Worker was lost.

## Consequences
- One rule covers restarts and lost Workers. No side effect runs twice without an idempotency key or a human decision.
- Capability declarations carry an `idempotent` flag; the Capability catalog and the provider protocol include it (M2 contracts).
- Validation (M1 spike): crash injection in every Step state on bundled PostgreSQL on three OSes. Pass means: no lost or duplicated side effects, idempotent Steps resume within one reconcile cycle, all other running Steps end `interrupted`.
- Revisit if: timer or queue contention cannot be tuned, or Workflow features (sagas, child workflows at scale) cost more than adopting an engine.

## Alternatives considered
- **Retry Policy alone decides (ADR-0014 as written)**: simpler to state, but it can duplicate non-idempotent side effects.
- **Fail every non-resumable Step (prototype `runtime_restarted`)**: safe, but it wastes idempotent work and counts the interruption as a failure.
- **Temporal, Restate, Hatchet, DBOS, Conductor**: see ADR-0014.

## Approval
Proposed by: Claude (second-pass open-question resolution, 2026-10-25) · Approved by: Ricardo Lüders on 2026-10-25, planning prompt that delegated every remaining decision (status "accepted, delegated" confirmed in the same session).
