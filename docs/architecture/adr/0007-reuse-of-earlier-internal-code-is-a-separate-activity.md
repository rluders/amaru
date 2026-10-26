---
title: Reuse of earlier internal code is a separate, approval-gated activity
status: accepted
date: 2026-10-24
deciders: Ricardo Lüders (product owner)
---

# 0007. Reuse of earlier internal code is a separate, approval-gated activity

## Context
An earlier internal implementation exists outside this repository: a working Go Runtime, an Obsidian plugin, protocol contracts, its own decision records and validation records, and it still has outstanding development cycles. Amaru is a new product repository. Copying code now would freeze an in-flux implementation, fork its history and mix two product identities.

## Decision
1. The Amaru bootstrap does not copy, move or rewrite code, history, branches or data from the earlier implementation, and does not change it to suit Amaru.
2. The earlier implementation continues its own development cycles in its own repository.
3. Reuse of selected code and behavior happens later, component by component, only after its readiness criteria are met **and** a human explicitly approves each migration step.
4. Until then, Amaru documents may cite the earlier implementation as evidence but must not claim its behavior as Amaru behavior.

## Consequences
- Amaru has no executable product code for now; its docs label prototype evidence explicitly.
- The evidence inventory of the earlier implementation must be refreshed before migration decisions.
- Licensing (the earlier code is MIT; Amaru's license was later decided in [ADR-0026](0026-fair-code-community-license-with-commercial-licensing.md)) must be resolved before code moves.

## Alternatives considered
- *Import the earlier implementation as the starting point now*: fastest path to running code, but locks in decisions the architecture study should test and interrupts its cycles.
- *Rewrite from scratch*: discards validated behavior and tests.

## Approval
Required by the product brief, which said not to migrate the earlier implementation into the Amaru repository in this initiative. Recorded as accepted by Ricardo Lüders on 2026-10-24 (plan approval, Amaru bootstrap session).
