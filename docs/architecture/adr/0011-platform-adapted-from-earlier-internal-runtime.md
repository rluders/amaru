---
title: The Platform core is adapted from an earlier internal Go Runtime
status: accepted
date: 2026-10-25
deciders: Ricardo Lüders
---

# 0011. The Platform core is adapted from an earlier internal Go Runtime

## Context
- [ADR-0007](0007-reuse-of-earlier-internal-code-is-a-separate-activity.md) (accepted) makes reuse a separate, approval-gated activity but does not say whether the Platform starts from the earlier internal code or from scratch.
- Verified at the pinned baseline commit ([ADR-0032](0032-migration-baseline-is-a-pinned-commit.md)):
  - About 27k lines of non-test Go and 19k lines of tests (546 `Test*` functions).
  - Hexagonal boundaries enforced by an architecture test.
  - Execution recovery: restore, resume of interrupted executions, idempotency keys.
  - A delivery loop exercised in seven dogfood cycles.
  - Contracts with 271 fixtures validated in Go and TypeScript.
- Verified gaps:
  - File-only persistence with no migrations (GAP-07).
  - Sequential Steps and no per-Step timeouts.
  - No leases.
  - Providers that ignore `WorkspaceWrite`.
  - OpenAPI drift.
- Verified: a second, earlier prototype has a smaller, less tested Go backend (171 `Test*` functions; executions inside HTTP requests), so it is not a better core.
- The product owner chose this direction on 2026-10-25; this ADR records it for formal acceptance.

## Decision
- The Amaru Platform core is built by **adapting** components of the earlier internal Runtime, not by a clean-room rewrite:
  - `domain`, `ports`, `app` (coordinator, steps, context assembly, delivery loop)
  - `provider/*`, `platform`
  - `protocol/` as the contract seed
- Each component moves only under a migration ADR that meets the readiness criteria, starting from the baseline commit named in that ADR.
- Behavior-preserving tests move first.
- Components whose design contradicts accepted Amaru decisions are reimplemented with the earlier tests as the specification. Examples: its file storage (replaced by PostgreSQL, [ADR-0015](0015-postgresql-is-the-only-database-bundled-in-compact-installs.md)); its Obsidian UI ([ADR-0031](0031-obsidian-plugin-is-retired.md)).

## Consequences
- Proven recovery and delivery-loop behavior is kept; the migration starts from a fixed baseline commit ([ADR-0032](0032-migration-baseline-is-a-pinned-commit.md)) without waiting for further work on the earlier implementation; licensing of its MIT-licensed code is settled by [ADR-0026](0026-fair-code-community-license-with-commercial-licensing.md).
- Identifiers and the provider protocol namespace are renamed per the [glossary](../../product/GLOSSARY.md#2-term-mapping-from-predecessor-repositories).
- Revisit if: the M1 spikes show the storage or execution model must change so much that adaptation costs more than reimplementation for the coordinator.

## Alternatives considered
- **Reimplement with the earlier code as specification**: cleaner boundaries but slower, and re-learns recovery and delivery-loop lessons.
- **Start from the earlier prototype backend**: Postgres and multi-user concepts exist, but execution durability and tests are weaker.

## Approval
Proposed by: Claude (master roadmap planning, 2026-10-25), recording product-owner direction given the same day · Approved by: Ricardo Lüders on 2026-10-25, product-owner planning session that resolved all open questions (explicit acceptance of every pending ADR).
