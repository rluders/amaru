---
title: Approvals are human decisions bound to specific Evidence that gate Steps and Work Item acceptance
status: accepted
date: 2026-10-25
deciders: Ricardo Lüders
---

# 0036. Approvals are human decisions bound to specific Evidence that gate Steps and Work Item acceptance

## Context
- Question: [W1](../OPEN_QUESTIONS.md#w1-approval-model) (carried over from Weave Q2). No product-owner answer; resolved under the session's delegation.
- Weave: `approval` steps persist pending decisions that survive restarts, an inbox lists them, inbound hooks cannot approve, the approver is anonymous. Weave prototype: Approvals link `execution_id` and `verification_run_id`, record reviewer and submitter, and reject self-approval.
- [ADR-0006](0006-human-authorization-for-consequential-actions.md) requires explicit human authorization for consequential actions.

## Decision
- An **Approval** is a record with: gate (Step Approval wait, Work Item acceptance, Policy-required action), requester, approver, decision (approved/rejected) with rationale, time, and the **Evidence it binds to**: Execution and Step Execution ids, commit SHA and diff hash, Artifact version hashes, Local Check/Verification results.
- Only **human principals** with the required role ([ADR-0038](0038-roles-teams-and-separation-of-duties.md)) may decide; Agents, hooks and service accounts never approve.
- If bound Evidence changes (new commit, new Artifact version), pending Approvals are **invalidated** and re-requested; decided Approvals stay attached to the Evidence they saw.
- **Work Item status** remains human-set; "awaiting approval" is derived from pending gates; moving a Work Item to accepted requires an approved acceptance gate when its Work Package Policy demands one.
- A missing decision parks only the affected Work Item (Weave autonomy invariant); the delivery loop continues with others.

## Consequences
- Every consequential action has an auditable, evidence-specific human decision.
- Re-approval after changes adds friction by design.

## Alternatives considered
- **Approval of a Work Item regardless of Evidence**: approves changes the human never saw.
- **Agent reviewers as approvers**: violates ADR-0006; Agents produce review verdicts, not Approvals.

## Approval
Proposed by: Claude (open-question resolution, 2026-10-25) · Approved by: Ricardo Lüders on 2026-10-25, product-owner planning session (resolution delegated to Claude; accepted in the same session).
