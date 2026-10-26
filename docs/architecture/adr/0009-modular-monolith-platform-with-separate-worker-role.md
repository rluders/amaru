---
title: The Platform is a modular monolith with a separate Worker role
status: accepted
date: 2026-10-25
deciders: Ricardo Lüders
---

# 0009. The Platform is a modular monolith with a separate Worker role

## Context
- The product needs one Platform for compact local, self-hosted and Cloud deployments ([ADR-0002](0002-one-platform-multiple-deployment-models.md)) and Steps that run on other machines (PRD UC-3, FR-43).
- Verified: the prototype Runtime is one Go process per user combining coordination and execution, with hexagonal packages (`domain`, `app`, `ports`, `provider/*`) and an architecture test forbidding core → provider imports.
- Verified: the earlier prototype is a Go modular monolith whose synchronous in-request execution could not run safely in its self-hosted container, and whose investigation recommends a separate executor process.
- Assumed: early deployments do not need independent scaling of control-plane modules.

## Decision
- The Amaru Platform control plane is **one deployable Go service** composed of internal modules with enforced boundaries: API, Identity & Policy, Domain (Projects, Work, Workflows, Agents, Skills), Execution coordinator, Event log, Capability catalog and provider registry, Plugin registry, storage adapters.
- **Execution is a separate role, the Worker**, defined by the Worker protocol. In compact mode a Worker runs **embedded** in the Platform process but uses the same interface as a remote Worker. In server and Cloud deployments Workers are **separate processes**, possibly on other machines.
- **Plugin hosts and external Providers are always separate processes.**
- Modules become separate services only through a later ADR backed by a measured need.

## Consequences
- Compact install stays one Platform process (NATS embedded, [ADR-0016](0016-nats-jetstream-message-bus-embedded-in-compact-installs.md)) plus its supervised bundled PostgreSQL ([ADR-0015](0015-postgresql-is-the-only-database-bundled-in-compact-installs.md)); the remote Worker path is exercised from the start by running the same assignment tests against embedded and separate Workers.
- Module boundaries need an enforced import rule (the prototype's architecture test generalized).
- The Worker protocol becomes a contract (ADR-0004) and a prerequisite for M5.
- Revisit if: a module needs independent scaling or availability in Cloud, or the embedded Worker cannot share the remote interface without unacceptable cost.

## Alternatives considered
- **Microservices from the start**: rejected; operational weight for self-hosters (ADR-0008 principle), no measured need.
- **Keep coordinator and executor fused (prototype model)**: rejected; blocks remote environments and safe sandboxing (prototype investigation).
- **Embed the Platform in the Desktop**: rejected by [ADR-0003](0003-desktop-is-a-client.md).

## Approval
Proposed by: Claude (master roadmap planning, 2026-10-25) · Approved by: Ricardo Lüders on 2026-10-25, product-owner planning session that resolved all open questions (explicit acceptance of every pending ADR).
