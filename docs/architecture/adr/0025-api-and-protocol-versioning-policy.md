---
title: APIs and protocols are versioned by major with additive-only changes
status: accepted
date: 2026-10-25
deciders: Ricardo Lüders
---

# 0025. APIs and protocols are versioned by major with additive-only changes

## Context
- Question: [Q11](../OPEN_QUESTIONS.md#q11-api-compatibility-and-versioning). Product-owner answer: "Additive-only changes, breaking changes introduce a new version. Propose other versioning for the rest."
- Weave uses path-versioned `/v1` (HTTP JSON commands + WebSocket Event stream with `?since=` replay; SSE was offered and declined, ADR-001) and `weave.provider/v1`, changed in place before any release.
- Clients, plugins and Workers upgrade independently; Cloud and self-hosted run different versions ([ADR-0004](0004-shared-versioned-contracts.md)).

## Decision
- **Public HTTP API**: path major `/v1`. Within a major, changes are additive only (new endpoints, optional fields, new enum values that clients must tolerate). A breaking change creates `/v2`; the previous major stays supported for at least 12 months after the next major ships.
- **Named schemas** for every other contract: `amaru.<kind>/v<major>` — `amaru.provider/v1` (successor of `weave.provider/v1`), `amaru.worker/v1`, `amaru.plugin/v1`, `amaru.event/v1`. Same additive rule.
- **Protobuf** contracts use packages `amaru.<kind>.v<major>`; `buf breaking` runs in CI against the last released contracts.
- **Negotiation**: Workers and Providers announce the majors they support at handshake; the Platform picks the highest common one and supports the current and previous major.
- **SDKs** (TypeScript client, plugin SDK, Go Worker SDK) use semver; an SDK major targets one API major; deprecations are announced one minor ahead.
- **Client event stream**: WebSocket carrying `amaru.event/v1` envelopes with a per-stream sequence cursor; clients resume with the last cursor and fall back to HTTP history when the cursor has expired.
- Deprecations are signalled by `Deprecation`/`Sunset` response headers and the changelog.

## Consequences
- Every contract change is reviewable against one rule; fixtures shared by all consumers prove compatibility.
- Two majors of each protocol may run at once; the cost is bounded to 12 months.
- Validation (M2): contract fixtures and `buf breaking` gate CI.

## Alternatives considered
- **Header/media-type versioning**: harder to route and debug.
- **Date-based API versions**: suited to large public APIs with many minor breaks; unnecessary here.
- **SSE for client events**: one-way only and declined in Weave; WebSocket is already required for Workers.

## Approval
Proposed by: Claude (open-question resolution, 2026-10-25) · Approved by: Ricardo Lüders on 2026-10-25, product-owner planning session (answer recorded in OPEN_QUESTIONS Q11; versioning of the remaining contracts delegated).
