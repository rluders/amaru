---
title: Fixed roles, Teams of humans and Agents, and per-gate separation of duties
status: accepted
date: 2026-10-25
deciders: Ricardo Lüders
---

# 0038. Fixed roles, Teams of humans and Agents, and per-gate separation of duties

## Context
- Question: [W3](../OPEN_QUESTIONS.md#w3-teams) (prototype ADR-019). Resolved under the session's delegation.
- Prototype Teams: `{members:[{agentId, role}], separationOfDuties}`, opt-in, single-user product. earlier prototype: `RequiredReviewerTeam`, self-approval rejected.

## Decision
- **Organization roles** (fixed set): Owner, Admin, Maintainer, Contributor, Viewer. Project-level role assignments may narrow or raise a member's role for that Project.
  - Owner/Admin: membership, Grants, Workers, plugins, settings. Maintainer: approve gates, manage definitions. Contributor: create Work Items, start Executions. Viewer: read.
- **Teams** are named sets of humans and Agents with a team role per member (e.g. implementer, reviewer). Agents are principals for attribution but never hold an Organization role and never approve.
- **Separation of duties**, configurable per gate: the approver must differ from the initiator and from the human sponsor of the Agent that produced the Evidence; a gate may require an approver from a named Team (required reviewer Team).
- Compact single-user installs disable separation of duties by default (one human), and record that it was disabled.

## Consequences
- Simple, predictable permission model; custom roles are not supported.
- SoD becomes enforceable once multiple users exist (M7).

## Alternatives considered
- **Custom roles and permission sets**: flexible but harder to audit; can be added later on the same Policy function.

## Approval
Proposed by: Claude (open-question resolution, 2026-10-25) · Approved by: Ricardo Lüders on 2026-10-25, product-owner planning session (resolution delegated to Claude; accepted in the same session).
