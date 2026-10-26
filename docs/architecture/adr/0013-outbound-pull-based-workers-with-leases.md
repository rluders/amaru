---
title: Workers connect outbound, pull assignments and hold leases
status: accepted
date: 2026-10-25
deciders: Ricardo Lüders
---

# 0013. Workers connect outbound, pull assignments and hold leases

## Context
- Remote Execution Environments (PRD UC-3, FR-43) include workstations behind NAT or corporate firewalls, Windows machines with proprietary tools, and customer machines connected to Amaru Cloud.
- Verified: the prototype has no Worker separation, no leases and no heartbeat; recovery relies on in-process restart semantics and `Resumable` remote jobs.
- Verified: the earlier prototype's executor investigation proposes a local Unix-socket executor, not a networked one.
- Requirements: lost Workers must be detected (NFR-10); duplicated messages must not duplicate consequential actions (NFR-11).

## Decision
- **Connection**: Workers initiate the connection to the Platform and authenticate with a machine identity. The Platform never needs inbound access to a Worker.
- **Assignment**: Workers advertise a capability and health manifest and **pull** Step assignments they are eligible for.
- **Lease**: each assignment is a **lease** renewed by heartbeat. On lease expiry the Platform fails or reassigns the Step according to the Step's declared idempotency. Completion reports carry the assignment ID and are accepted at most once.
- **Embedded Worker**: it uses the same assignment, lease and report semantics in process.
- **Transport**: Protobuf over WebSocket ([ADR-0029](0029-worker-protocol-over-websocket-and-protobuf.md)); identity and enrollment: [ADR-0024](0024-remote-worker-registration-identity-and-delivery.md).

## Consequences
- Works through NAT and firewalls and for customer-run Workers in Cloud. Loss and duplication handling become explicit and testable.
- Requires a machine identity model (FEAT-IAM-006) and a lease store with clock rules.
- Interactive tools that need a desktop session are served by running the Worker inside the logged-in user session ([ADR-0024](0024-remote-worker-registration-identity-and-delivery.md)); validated in the M5 UE5 pilot.
- Leases are stored in PostgreSQL by Amaru's own execution engine ([ADR-0014](0014-durable-execution-on-own-state-machine-over-postgres.md)); assignments reach the Worker gateway through NATS ([ADR-0016](0016-nats-jetstream-message-bus-embedded-in-compact-installs.md)).
- Revisit if: lease timing cannot be met with PostgreSQL-stored leases.

## Alternatives considered
- **Platform pushes to Workers over inbound connections**: fails behind NAT and widens the attack surface.
- **Workers consuming the broker directly**: exposes the internal bus to remote machines and couples them to NATS; the bus stays internal behind the Worker gateway ([ADR-0016](0016-nats-jetstream-message-bus-embedded-in-compact-installs.md)).

## Approval
Proposed by: Claude (master roadmap planning, 2026-10-25) · Approved by: Ricardo Lüders on 2026-10-25, product-owner planning session that resolved all open questions (explicit acceptance of every pending ADR).
