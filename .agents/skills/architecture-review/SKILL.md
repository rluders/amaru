---
name: architecture-review
description: Review a proposed design, ADR, new component or cross-boundary change in Amaru for boundary violations, dependency direction, trade-offs, failure modes and security. Use before accepting a design, adding a component, or changing contracts.
---

# Architecture review

## Purpose
Catch boundary violations and unexamined trade-offs before they become code or accepted decisions.

## When to use
- A new or changed proposal in `docs/architecture/` or an ADR.
- A change that adds a component, a dependency, a contract field, or crosses Platform / Worker / Client / Provider boundaries.
- Before a migration ADR that moves Weave code.

Not for: style review, small in-component changes.

## Inputs
The proposal or diff; [ARCHITECTURE.md](../../../docs/architecture/ARCHITECTURE.md); [DECISIONS.md](../../../docs/architecture/DECISIONS.md); relevant [OPEN_QUESTIONS](../../../docs/architecture/OPEN_QUESTIONS.md); Weave evidence if cited ([inventory](../../../docs/migration/weave-inventory.md)).

## Workflow
1. **State the change** in two sentences: what moves where, and why.
2. **Boundaries.** Check each:
   - Clients use only the public API; no client owns execution state (ADR-0003).
   - Platform owns state, Policy, Approvals, Events; Workers own runs, not state.
   - Workflows reference Capabilities, never vendors; a new Provider changes no core code, contract shape or UI architecture.
   - Domain code imports no OS, process, network or vendor packages.
   - Contracts carry no client or framework concepts (ADR-0004).
3. **Dependencies.** List new runtime, library and infrastructure dependencies. Each needs a reason; infrastructure needs an accepted ADR (ADR-0008).
4. **Human control and security.** Does anything consequential happen without Policy or Approval (ADR-0006)? New secrets, egress, process execution, untrusted input paths? Compare with the [security model](../../../docs/security/SECURITY_MODEL.md).
5. **Failure modes.** For each new state or call: crash mid-way, restart, timeout, partial write, network partition, duplicate delivery. What is the defined outcome?
6. **Deployment parity.** Does it work for local, self-hosted and Cloud with only adapter differences (ADR-0002)?
7. **Evidence.** Mark each claim verified (source/test) or assumed. Unverified platform/library claims are findings.
8. **Alternatives.** Name at least one simpler option and why it is not enough.

## Output
A short report:
- Summary (2 sentences) and verdict: *sound* / *sound with changes* / *needs redesign* / *needs human decision*.
- Findings, most severe first, each with location, the rule or risk, and a concrete fix.
- Open questions to add to OPEN_QUESTIONS, if any.

## Verification
- Every finding cites a file/line, ADR or doc section.
- No finding relies on an unverified API or behavior without saying so.
- If the review changes docs, run `make docs-check`.
