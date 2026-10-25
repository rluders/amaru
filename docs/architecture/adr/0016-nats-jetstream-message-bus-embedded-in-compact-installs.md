---
title: NATS JetStream is the message bus, embedded in compact installs
status: accepted
date: 2026-10-25
deciders: Ricardo Lüders
---

# 0016. NATS JetStream is the message bus, embedded in compact installs

## Context
- Question: [Q2](../OPEN_QUESTIONS.md#q2-queue-and-messaging-strategy). Product-owner answer: "NATS, 'cause it can handle idempotency easily and have nice performance and scalability." In this session the product owner chose NATS from day one, embedded in compact installs, over a PostgreSQL-only queue until Cloud.
- Verified 2026-10-25:
  - `nats-server` is Apache-2.0, latest v2.15.1 (2026-10-09), and can run inside a Go process: `server.NewServer(opts)`, `Options.DontListen`, `(*Server).InProcessConn()`, client option `nats.InProcessServer` (https://github.com/nats-io/nats-server).
  - JetStream deduplicates on the `Nats-Msg-Id` header inside a duplicate window (default 2 minutes); work-queue retention delivers each message once and removes it on ack; explicit ack, `MaxDeliver`, `BackOff` (https://docs.nats.io/nats-concepts/jetstream/streams, …/consumers).
- PostgreSQL is the source of truth ([ADR-0014](0014-durable-execution-on-own-state-machine-over-postgres.md), [ADR-0015](0015-postgresql-is-the-only-database-bundled-in-compact-installs.md)). Window-based dedup is not end-to-end idempotency, so the bus cannot be the only guard.

## Decision
- **NATS JetStream** is the Platform's message bus in every deployment:
  - **Compact and single-node self-hosted**: `nats-server` with JetStream is embedded in the Platform process (`DontListen`, in-process connection, store under the data directory). No port is opened.
  - **Multi-instance self-hosted and Cloud**: an external NATS cluster; the Platform connects as a client.
- **Roles of the bus**:
  - Step assignment: one work-queue stream per Worker pool (subject `amaru.<org>.assign.<pool>`); the Worker gateway consumes and offers assignments to connected Workers ([ADR-0029](0029-worker-protocol-over-websocket-and-protobuf.md)).
  - Event fan-out: Execution and platform Events to client and Worker gateways and to plugin `logic` hosts (`amaru.<org>.events.>`).
  - Internal module notifications inside the modular monolith.
- **Consistency rules**:
  - Every publish originates from the transactional outbox; the relay sets `Nats-Msg-Id` to the outbox row id and marks the row sent after the JetStream ack.
  - Every consumer is idempotent using keys stored in PostgreSQL; redelivery is expected.
  - **Leases, timers and Step state are authoritative in PostgreSQL**, never in NATS; losing the NATS store loses no state, the relay republishes unsent outbox rows and reconciliation re-enqueues assignments.
- A `Bus` port hides NATS from domain code; tests use an in-memory fake.

## Consequences
- Same messaging model from laptop to Cloud; scaling out the Platform is configuration, not redesign.
- The outbox, relay and reconciliation are required from M3, not later.
- One more embedded component in the compact binary (size and memory measured in M1).
- Validation (M3): kill the Platform between commit and publish, and between publish and ack, for assignments and Events; no lost or duplicated side effects.
- Revisit if: embedded JetStream causes unacceptable footprint or Windows file-locking issues in compact installs.

## Alternatives considered
- **PostgreSQL-only queue (`SKIP LOCKED`, `LISTEN/NOTIFY`) until Cloud**: fewer moving parts early; declined by the product owner in favor of one model from day one.
- **RabbitMQ / cloud queues**: not embeddable; separate service in every install.
- **NATS as source of truth**: dedup window and stream limits make it unsuitable as the authoritative record.

## Approval
Proposed by: Claude (open-question resolution, 2026-10-25) · Approved by: Ricardo Lüders on 2026-10-25, product-owner planning session (answer recorded in OPEN_QUESTIONS Q2; "embedded from day one" chosen in the same session).

## Notes
Cross-reference added 2026-10-25. It leaves the decision above unchanged: the compact footprint budget (≤ 120 MB installer increment, ≤ 300 MB idle memory) is validation A15 in [OPEN_QUESTIONS](../OPEN_QUESTIONS.md#validation-register).
