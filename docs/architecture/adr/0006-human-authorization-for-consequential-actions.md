---
title: Explicit human authorization for consequential actions
status: accepted
date: 2026-10-24
deciders: Ricardo Lüders (product owner)
---

# 0006. Explicit human authorization for consequential actions

## Context
"The engineer remains the maestro of execution" is the product's core principle. The prototype already forbids auto-approved agent permissions (no `--dangerously-skip-permissions`), starts every execution from an explicit user action, and treats project files as untrusted. It has no Approval step and no enforced capability permissions (prototype ROADMAP Q2, GAP-09), so agents inside a run are limited only by provider defaults.

## Decision
1. Amaru never auto-approves agent or tool permissions on the user's behalf.
2. Consequential actions require an explicit, recorded human authorization before they proceed. At minimum: pushing or publishing to shared systems, deleting or overwriting user data outside a run's own output location, spending money or quota on paid services, sending data to new external destinations, and granting new permissions to Agents, plugins or Workers.
3. Authorization is either a standing **Policy** grant made by a human, or a per-occurrence **Approval** recorded with identity, time, and the evidence shown. Both are Events in the audit trail.
4. No path bypasses this: triggers (webhooks, schedules, other agents) go through the same Policy and Approval checks.

## Consequences
- The Approval Step (PRD FR-13) and capability Policy (FR-24) are MVP requirements, not later features.
- The exact list of consequential actions, Policy shape and approver roles are designed in Stage 2/3 (prototype carry-overs W1, W2 in [OPEN_QUESTIONS](../OPEN_QUESTIONS.md#carried-over-from-the-prototype), later decided in [ADR-0036](0036-approvals-bind-to-evidence-and-gate-work.md) and [ADR-0037](0037-one-policy-evaluates-capabilities-grants-and-approvals.md)); they refine but may not weaken this rule.
- Some fully automated flows will require a human to pre-grant Policy; that is intended.

## Alternatives considered
- *Trust provider-side permission prompts only*: inconsistent across vendors and invisible to the audit trail.
- *Approval only in team deployments*: leaves single users exposed to prompt-injection-driven actions.

## Approval
Proposed by Claude in the bootstrap plan. Approved by Ricardo Lüders on 2026-10-24 (plan approval, Amaru bootstrap session). Grounded in the principle stated in the product brief and the prototype's security boundary.
