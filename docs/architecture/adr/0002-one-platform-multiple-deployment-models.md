---
title: One Platform, multiple deployment models
status: accepted
date: 2026-10-25
deciders: Ricardo Lüders
---

# 0002. One Platform, multiple deployment models

## Context
The product direction names Amaru Cloud (managed) and Amaru Self-hosted (user-operated), plus the local single-user use that prototype serves today ([PRD §6](../../product/PRD.md#6-product-surfaces-and-deployment-models)). Separate products per deployment would fork the domain model and contracts.

## Decision
There is one Amaru Platform with one domain model and one set of public contracts. Local, self-hosted and Cloud are **deployments** of it that differ only through adapters (storage, identity provider, Worker placement, observability backends) and configuration — never through divergent domain logic or contract shapes.

## Consequences
- Cloud-only or self-hosted-only features are expressed as adapters or Policy, not forks.
- Storage and identity need ports from the start (see [ARCHITECTURE §10](../ARCHITECTURE.md#10-persistence-and-artifact-storage)).
- The local deployment must stay simple; heavy infrastructure is an adapter choice of server deployments, not a requirement of the Platform ([ADR-0008](0008-incremental-monorepo-layout.md)).
- Revisit if a deployment's needs cannot be met by adapters without distorting the domain.

## Alternatives considered
- *Separate Cloud product*: faster Cloud iteration, but two domain models and duplicated contracts.
- *Cloud-only*: conflicts with local/self-hosted direction and with the prototype's local-first users.

## Approval
Proposed by: Claude (bootstrap, 2026-10-24) · Approved by: Ricardo Lüders on 2026-10-25, product-owner planning session that resolved all open questions (explicit acceptance of every pending ADR).
