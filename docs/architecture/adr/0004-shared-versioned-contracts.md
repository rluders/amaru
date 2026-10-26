---
title: Shared, versioned contracts across deployments and clients
status: accepted
date: 2026-10-25
deciders: Ricardo Lüders
---

# 0004. Shared, versioned contracts across deployments and clients

## Context
The prototype keeps its protocol in `protocol/` (OpenAPI 3.1, JSON Schema 2020-12, golden fixtures) and validates the same fixtures in Go and TypeScript tests; additive changes only within `/v1`, clients ignore unknown fields. This prevented client/Runtime drift across many changes.

## Decision
Amaru's public API, event schema, provider protocol and (future) Worker protocol are defined once, in a contracts package, as machine-readable schemas with golden fixtures. Every consumer (Platform, SDK, clients, plugins) is tested against the same fixtures. Contracts are versioned; within a major version changes are additive. No contract contains client- or framework-specific concepts.

## Consequences
- A contracts package is the first code package to introduce (Stage 3), before clients.
- Breaking changes need a new major version and a migration note.
- The versioning scheme and event transport are set by [ADR-0025](0025-api-and-protocol-versioning-policy.md).

## Alternatives considered
- *Code-first contracts generated from the Go server*: faster initially, but makes the server language the source of truth for TS clients and plugins.
- *gRPC/protobuf*: strong tooling, but weaker browser story and a departure from the prototype's working HTTP+JSON contracts; adopted for the Worker protocol only ([ADR-0029](0029-worker-protocol-over-websocket-and-protobuf.md)).

## Approval
Proposed by: Claude (bootstrap, 2026-10-24) · Approved by: Ricardo Lüders on 2026-10-25, product-owner planning session that resolved all open questions (explicit acceptance of every pending ADR).
