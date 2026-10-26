---
title: Approval reuse and validity, uncovered Capabilities and Triggers, and Policy defaults and tightening
status: proposed
date: 2026-10-26
deciders:
---

# 0048. Approval reuse and validity, uncovered Capabilities and Triggers, and Policy defaults and tightening

## Context
[POLICY_AND_APPROVAL_SHAPES](../POLICY_AND_APPROVAL_SHAPES.md) (WP-M1-04) found three points the accepted ADRs leave open and recorded them as [Q56](../OPEN_QUESTIONS.md#q56-what-happens-to-a-decided-approval-and-to-a-retried-step-when-evidence-changes), [Q57](../OPEN_QUESTIONS.md#q57-how-does-policy-treat-a-capability-no-rule-covers-and-who-is-the-requester-of-a-triggered-execution) and [Q58](../OPEN_QUESTIONS.md#q58-policy-defaults-versus-the-mvp-never-merges-rule-and-what-a-project-may-tighten). They block the Approval and Policy contracts of WP-M2-01 (#68). The product owner asked for them to be solved on 2026-10-26; this ADR is the proposed solution and takes effect only when the owner accepts it. The proposal adopts the recommendations already recorded with each question and adds one decision of its own (item 4).

Verified in the repository: [ADR-0036](0036-approvals-bind-to-evidence-and-gate-work.md) (pending Approvals are invalidated, decided ones stay attached to what they saw), [ADR-0037](0037-one-policy-evaluates-capabilities-grants-and-approvals.md) (resolution order, `needs_grant`), [ADR-0038](0038-roles-teams-and-separation-of-duties.md), [ADR-0039](0039-configuration-scopes-and-resolution-order.md), [ADR-0042](0042-durable-execution-with-explicit-restart-semantics.md) and [ADR-0044](0044-restart-semantics-clarifications-from-the-m1-spike.md) (idempotent retry is a new Lease, no attempt spent), [baseline rules 2 and 12](../../security/SECURITY_MODEL.md#baseline-rules). Nothing below is implemented; M3 crash and property tests would validate it.

## Decision
**Q56: decided Approvals and retries**
1. An `approved` or `rejected` Approval is immutable. It is never voided or edited. The wording "is void" in [DOMAIN_MODEL §4](../DOMAIN_MODEL.md#4-key-relationships-and-rules) applies to `requested` Approvals only.
2. Validity is checked at use time: before the gated action starts, the Platform recomputes the digest of the current Evidence. A different digest means the Approval does not authorize the action; the Step returns to `awaiting_approval` with a new `requested` Approval whose `replaces` names the old one.
3. An Approval authorizes one **occurrence**, identified by (Step Execution id, Capability, resource). An idempotent retry of that occurrence (a new Lease, [ADR-0042](0042-durable-execution-with-explicit-restart-semantics.md)) reuses the Approval while the digest is unchanged; it does not ask the human again. A different occurrence, or a changed digest, needs a new Approval. A Step that is not idempotent is never re-run, so it never reuses.
4. First use of a secret: the Approval records the secret **name** (never the value) and the key (Capability, secret name, Project). Later uses with that key need no new Approval while the secret reference is unchanged and no Grant or rule has revoked it; a changed or rotated reference is a first use again.

**Q57: uncovered Capabilities and Triggers**
5. An explicit `deny` rule yields decision `deny` with reason code `policy.explicit_deny`; the Step ends `refused`.
6. A request that no rule covers and that no rule marks approvable yields decision `deny` with reason code `policy.uncovered`. In the delivery loop this parks the Work Item as `needs_grant` and spends no attempt; outside the loop the Step ends `refused`. Clients tell the two cases apart by the reason code.
7. A Trigger (webhook, schedule, delivery loop) is a principal of kind `trigger` with a named human **sponsor**: the human who created or last edited it, or who enabled the loop. The sponsor is the initiator for separation of duties. A Trigger never approves and holds no role.

**Q58: defaults and tightening**
8. The MVP Capability catalog has **no merge Capability**. `scm.pr.merge` is a reserved name: until it is added to the catalog its default is `deny`, and `require_approval` once added. The Platform never merges in the MVP (rule 2 stands).
9. `scm.pr.create` (opening a pull request, draft or not) is an approvable forge write: default `require_approval`.
10. A Project's configuration may only (a) add approval requirements, (b) narrow allowlists, (c) shorten retention and Approval expiry, (d) make a gate stricter (higher role, a named Team, separation of duties on). A Project value that loosens is rejected at validation ([ADR-0039](0039-configuration-scopes-and-resolution-order.md)).

## Consequences
- The Approval, Policy and configuration contracts can carry these rules without an open blocker: `occurrence`, `replaces`, reason codes `policy.explicit_deny` and `policy.uncovered`, and a `trigger` principal with a required `sponsor`.
- After acceptance: close Q56–Q58 in OPEN_QUESTIONS, reword DOMAIN_MODEL §4, mark the resolved points in POLICY_AND_APPROVAL_SHAPES. None of that is done while this ADR is `proposed`.
- Item 4 is the choice most worth the owner's review: it goes beyond the recorded recommendation. A stricter alternative (ask once per Execution) costs more prompts and can be adopted by a superseding ADR without changing the contract fields.
- Revisit when a merge Capability is proposed, or when M3 crash tests show the retry reuse rule is unsound.

## Alternatives considered
- **Void decided Approvals when Evidence changes:** erases audit history (UC-5) and contradicts ADR-0036.
- **Always re-approve after a retry:** asks the human twice for one occurrence after a crash.
- **Fail uncovered requests in the loop:** stalls or fails the loop instead of parking, against ADR-0037.
- **A merge Capability in the MVP:** contradicts baseline rule 2.
- **Let Projects loosen defaults:** breaks the security baseline; only Owner or Admin lower defaults, by Grant (rule 12).

## Approval
Proposed by: Claude (resolution of Q56–Q58, 2026-10-26). Not yet approved: awaiting the product owner's explicit acceptance.
