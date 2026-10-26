---
title: One Policy evaluation combines requested Capabilities, Grants and Approvals
status: accepted
date: 2026-10-25
deciders: Ricardo Lüders
---

# 0037. One Policy evaluation combines requested Capabilities, Grants and Approvals

## Context
- Question: [W2](../OPEN_QUESTIONS.md#w2-capability-permission-model) (prototype GAP-09). Resolved under the session's delegation.
- the prototype: `requestedCapabilities` grants nothing (GAP-09); ADR-023 tool scope narrows MCP tools; ADR-024 grants are per user, per stanza hash or trusted project, and a missing grant parks the item as `needs_grant` without spending an attempt.

## Decision
- Every action that uses a Capability or resource (Step run, MCP tool, secret, network egress, Git push, plugin API call) is checked by **one Policy function**: `(principal, capability, resource scope, context) → allow | require_approval | deny`, with the reasons.
- Inputs: the capabilities **requested** by the Agent, Workflow or plugin manifest (a request is never authority); **Grants** — standing human decisions scoped to Organization or Project, optionally bound to a definition hash; **rules** from configuration ([ADR-0039](0039-configuration-scopes-and-resolution-order.md)).
- Resolution: explicit deny wins; an action covered by a Grant is allowed; an uncovered action that a rule marks approvable becomes `require_approval` ([ADR-0036](0036-approvals-bind-to-evidence-and-gate-work.md)); anything else is denied.
- A Grant bound to a definition hash lapses when the definition changes.
- Every evaluation is recorded as an Event; missing authority parks the item (prototype `needs_grant` semantics) instead of failing the loop.

## Consequences
- One place to audit and test authorization; plugins, Agents and Workflows use the same model.
- Policy evaluation is on the hot path and must stay pure and fast (domain code, no I/O).

## Alternatives considered
- **Separate mechanisms per extension kind**: inconsistent and hard to audit.
- **External policy engine (OPA/Cedar)**: extra dependency; can back the same function later if rules grow.

## Approval
Proposed by: Claude (open-question resolution, 2026-10-25) · Approved by: Ricardo Lüders on 2026-10-25, product-owner planning session (resolution delegated to Claude; accepted in the same session).
