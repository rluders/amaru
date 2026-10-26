---
title: Durable execution is Amaru's own state machine persisted in PostgreSQL
status: superseded
date: 2026-10-25
deciders: Ricardo Lüders
superseded-by: 0042
---

# 0014. Durable execution is Amaru's own state machine persisted in PostgreSQL

## Context
- Question: [Q1](../OPEN_QUESTIONS.md#q1-durable-workflow-technology). Product-owner answer: "Temporal is way too overpower, let's use Postgres."
- The prototype already implements a durable Step state machine: persisted Step states, `Manager.Restore`, `ResumeInterrupted`, idempotency keys, the `Resumable`/`Detachable` provider contract and Approval waits that survive restart. Its gaps are storage gaps, not model gaps: files only, non-terminal events not fsynced, no schema migrations (prototype GAP-07), no leases.
- Durable engines (Temporal, Restate, Hatchet, DBOS, Conductor) each add a server or a programming model: Temporal needs its own cluster and deterministic workflow code that clashes with declarative YAML graphs; Hatchet and DBOS are Postgres-based but impose their own schema and SDK model; Restate is a second process ([TECHNOLOGY_BENCHMARK §2](../TECHNOLOGY_BENCHMARK.md)).
- [ADR-0015](0015-postgresql-is-the-only-database-bundled-in-compact-installs.md) makes PostgreSQL the single store; [ADR-0011](0011-platform-adapted-from-earlier-internal-runtime.md) adapts the prototype coordinator.

## Decision
- The Platform runs Workflows with **its own durable state machine**, adapted from the prototype coordinator. No external workflow engine.
- All execution state lives in PostgreSQL, written transactionally:
  - Execution and Step Execution rows with explicit states and a monotonic version (optimistic concurrency).
  - The ordered Event log per Execution (append-only table, `seq` per Execution).
  - Timers (retry back-off, Lease expiry, Approval deadlines, schedules) as rows with a due time, polled and claimed with `FOR UPDATE SKIP LOCKED`.
  - Approval waits as persisted gates; a waiting Step holds no process or goroutine.
  - Idempotency keys for every externally visible side effect (Step submission, remote job, Git push, pull request).
- Every state transition and the messages it causes are committed together through a **transactional outbox** table; a relay publishes the outbox to the message bus ([ADR-0016](0016-nats-jetstream-message-bus-embedded-in-compact-installs.md)).
- Restart semantics: on start the Platform reconciles non-terminal Executions from the database; Steps with a recorded remote job id are resumed (prototype `Resumable` semantics), others follow their retry Policy instead of the prototype's blanket `runtime_restarted` failure.

## Consequences
- One durable substrate (PostgreSQL) for state, timers and history; queries for audit and search are plain SQL.
- Amaru owns correctness of the engine: the crash-recovery test suite is mandatory and grows with each Step kind.
- Multi-day human waits cost nothing at runtime.
- Validation: M1 spike runs the representative Workflow ([ARCHITECTURE §15](../ARCHITECTURE.md#15-representative-workflow-execution)) with a forced crash in every Step state on bundled PostgreSQL on Windows, macOS and Linux. Pass = no lost or duplicated side effects, resume within one reconcile cycle.
- Revisit if: the spike or production shows timer or queue contention that tuning cannot fix, or Workflow features (sagas, child workflows at scale) cost more than adopting an engine.

## Alternatives considered
- **Temporal**: strong guarantees, but a cluster to operate in every install and deterministic-code constraints; rejected by the product owner as too heavy.
- **Restate / Hatchet / DBOS / Conductor**: lighter or Postgres-based, but each imposes its own schema, SDK and process model on a domain prototype already modeled.
- **Keep the prototype's file store**: no transactions, no multi-user, GAP-07.

## Approval
Proposed by: Claude (open-question resolution, 2026-10-25) · Approved by: Ricardo Lüders on 2026-10-25, product-owner planning session (answer recorded in OPEN_QUESTIONS Q1; explicit acceptance of all ADRs from this session).
