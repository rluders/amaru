---
title: The Worker protocol is Protobuf over WebSocket using coder/websocket
status: accepted
date: 2026-10-25
deciders: Ricardo Lüders
---

# 0029. The Worker protocol is Protobuf over WebSocket using coder/websocket

## Context
- Question: [Q16](../OPEN_QUESTIONS.md#q16-worker-protocol-transport). Product-owner answer: "Strong preference to use WebSocket with Protobuf (don't use gorilla/websocket)."
- Verified 2026-10-25: `github.com/coder/websocket` is ISC, v1.8.15 (2026-06-15), actively maintained, `context.Context`-aware with concurrent writes; prototype already depends on it. `gorilla/websocket` last released v1.5.3 (2024-06) and is low-activity.
- Workers run behind NAT on three OSes; Leases, heartbeats, Event streaming and resumption must survive unreliable networks ([ADR-0013](0013-outbound-pull-based-workers-with-leases.md)). Durable execution is Amaru's own ([ADR-0014](0014-durable-execution-on-own-state-machine-over-postgres.md)), so no engine-native worker protocol applies.

## Decision
- Workers dial the Platform over **WebSocket (TLS; plain loopback only for the embedded Worker path when it is not in-process)** using **`github.com/coder/websocket`**. `gorilla/websocket` is not used.
- Frames are binary **Protobuf** messages of package `amaru.worker.v1` ([ADR-0025](0025-api-and-protocol-versioning-policy.md)), wrapped in an envelope with message id, ack id and type.
- Message families: handshake (version negotiation, key challenge), capacity/offer, assignment, lease renew/release, Step Events and logs, result, cancel, secret delivery.
- **Reliability**: application-level acks; on reconnect the Worker presents its session and last acknowledged id and the Platform resends unacked messages; heartbeats renew Leases; Lease expiry is decided in PostgreSQL.
- **Large payloads** (Artifacts, logs over a threshold) go over separate resumable HTTP uploads ([ADR-0017](0017-content-addressed-artifact-store-filesystem-or-s3.md)), not over the socket.
- The embedded Worker in compact installs uses the same Protobuf messages through an in-process transport.

## Consequences
- Firewall- and proxy-friendly (HTTPS port), one bidirectional connection per Worker.
- Protobuf gives compact frames and `buf breaking` checks; debugging needs tooling (a decoder in the CLI).
- Validation (M5): a Worker on a second machine with forced disconnects, NAT and an HTTP proxy.

## Alternatives considered
- **gRPC bidi streaming**: HTTP/2 end to end is often broken by proxies; heavier dependency.
- **HTTP long-poll**: higher latency for cancel and lease traffic.
- **gorilla/websocket**: excluded by the product owner; low activity.

## Approval
Proposed by: Claude (open-question resolution, 2026-10-25) · Approved by: Ricardo Lüders on 2026-10-25, product-owner planning session (answer recorded in OPEN_QUESTIONS Q16).

## Amendment 2026-10-27: the embedded Worker's in-process transport carries Go messages that mirror the Protobuf ones
"The same Protobuf messages through an in-process transport" is implemented as Go structs in the Platform's Worker port (`services/platform/internal/ports/worker.go`) that mirror `amaru.worker.v1` field for field, so the Platform core takes no Protobuf dependency. A test reads `worker.proto` and fails when a mirrored message or enum drifts, or when a field is neither carried nor listed as not carried yet. The WebSocket adapter maps between the Protobuf messages and the same structs. Chosen by the product owner in the WP-M3-03 session, 2026-10-27 (option "Domain-level port", [#145](https://github.com/rluders/amaru/issues/145)). Wire behavior is unchanged.
