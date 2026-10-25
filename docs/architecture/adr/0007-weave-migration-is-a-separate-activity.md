---
title: Migration of the Weave implementation is a separate, approval-gated activity
status: accepted
date: 2026-10-24
deciders: Ricardo Lüders (product owner)
---

# 0007. Migration of the Weave implementation is a separate, approval-gated activity

## Context
`rluders/weave-obsidian` holds a working Go Runtime, an Obsidian plugin, protocol contracts, ADR-001…020 and validation records, and still has outstanding cycles. Amaru is a new product repository. Copying code now would freeze an in-flux implementation, fork its history and mix two product identities.

## Decision
1. The Amaru bootstrap does not copy, move or rewrite Weave code, history, branches or data, and does not change Weave to suit Amaru.
2. Weave continues its own development cycles in its own repository.
3. Migration of selected code and behavior happens later, component by component, following [MIGRATION_PLAN](../../migration/MIGRATION_PLAN.md), only after its readiness criteria are met **and** a human explicitly approves each migration step.
4. Until then, Amaru documents may cite Weave as evidence but must not claim Weave behavior as Amaru behavior.

## Consequences
- Amaru has no executable product code for now; its docs label Weave evidence explicitly.
- The Weave inventory must be refreshed before migration decisions.
- Licensing (Weave is MIT; Amaru's license was later decided in [ADR-0026](0026-fair-code-community-license-with-commercial-licensing.md)) must be resolved before code moves.

## Alternatives considered
- *Import Weave as the starting point now*: fastest path to running code, but locks in decisions the architecture study should test and interrupts Weave's cycles.
- *Rewrite from scratch*: discards validated behavior and tests.

## Approval
Required by the product brief ("Do not migrate weave-obsidian into the Amaru repository in this initiative"). Recorded as accepted by Ricardo Lüders on 2026-10-24 (plan approval, Amaru bootstrap session).
