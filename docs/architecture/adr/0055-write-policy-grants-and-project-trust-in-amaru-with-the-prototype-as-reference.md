---
title: Write Policy evaluation, Grants and Project trust in Amaru, with the prototype grant and trust types as reference
status: accepted
date: 2026-10-28
deciders: Ricardo Lüders (product owner)
---

# 0055. Write Policy evaluation, Grants and Project trust in Amaru, with the prototype grant and trust types as reference

## Context
- [ADR-0007](0007-reuse-of-earlier-internal-code-is-a-separate-activity.md) bars moving code from the earlier internal implementation until a human approves a migration ADR for that component. [ADR-0049](0049-adapt-the-prototype-domain-and-ports-into-services-platform.md) item 4 left `domain/{hook,trust,grant,settings,playbook}.go` as **requires human decision** and promised that the choice would be made when WP-M3-04 needed it. This ADR makes that choice for **`grant.go` and `trust.go` only**. It is the decision gate for WP-M3-04 ([#16](https://github.com/rluders/amaru/issues/16)); no issue exists yet for it, so it is tracked from #16.
- **Source (verified 2026-10-28):** the Runtime repository of the earlier internal implementation, baseline commit **`c8a2f6e`** ([ADR-0032](0032-migration-baseline-is-a-pinned-commit.md)).

  | File | Lines | Blob | Own tests |
  |---|---|---|---|
  | `runtime/internal/domain/grant.go` | 76 | `a986459853b7982d8906551f60b5a2a18bc3e07f` | none |
  | `runtime/internal/domain/trust.go` | 23 | `db6f4025eb4b6cdcbc5b037c802a43211f245f1c` | none |

  Both import only the standard library. Neither has a test file in `domain`, so there is no behavior-preserving suite to move; the semantics below are read from the code, not from tests.
- **What the prototype does.**
  - `AgentGrants` is the user's standing consent, kept in the user's state directory. A cloned project cannot ship a grant. The user allow-list is the ceiling: a grant never adds a server or a tool.
  - Two grant kinds: a **trusted project** (any tool inside the allow-list) and a **stanza grant** bound to a SHA-256 of one Agent's `mcp` stanza. `StanzaHash` is order-independent over servers and tools, and an empty tool list (the whole allow-list of a server) hashes differently from a subset. Editing the stanza drops the grant.
  - `Authority` (`project_trust`, `stanza_grant`) says why a run may use its tools and is recorded in the Step snapshot.
  - `Trust{Kind, Ref, Hash, At}` is a user decision that one exact content (a Playbook or an imported Skill) may reach a provider. A changed hash makes the content untrusted again.
- **Why it is not a straight port.** The accepted Amaru model differs on the points that matter:
  - [ADR-0037](0037-one-policy-evaluates-capabilities-grants-and-approvals.md): one Policy function `(principal, capability, resource scope, context) → allow | require_approval | deny`, with Grants scoped to an **Organization or Project** and kept by the Platform. The prototype grant is per user, per MCP stanza, in a file.
  - [ADR-0048](0048-approval-reuse-uncovered-capabilities-and-policy-defaults.md): reason codes `policy.explicit_deny` and `policy.uncovered`, trigger principals with a sponsor, Approval validity checked by Evidence digest at use time.
  - [FEAT-POL-001](../../product/FEATURE_CATALOG.md#37-observability-identity-policy): grants are "recorded, revocable and attributed". The prototype type has no revocation record, no grantor and no audit Event.
  - [ADR-0006](0006-human-authorization-for-consequential-actions.md) and [baseline rule 12](../../security/SECURITY_MODEL.md#baseline-rules): both are Events in the audit trail.
- **Candidate dependency.** [`rluders/canery`](https://github.com/rluders/canery) v2.0.0 is a subject–action–resource–scope authorization kernel with a standard-library-only dependency set ([WORK_PACKAGES](../WORK_PACKAGES.md#wp-m3-04-policy-grants-and-approval)). GitHub reports no license for the repository. Not verified here: its API, semantics against ADR-0037 resolution order, or how it would carry hash-bound Grants.
- **Domain constraint.** `internal/domain` imports only the standard library and is pure ([AGENTS.md](../../../AGENTS.md#architecture-boundaries-accepted-not-implemented-see-architecture), `internal/arch`). ADR-0037 requires the Policy function to be pure and fast, domain code with no I/O.
- **Unverified:** that the prototype's `Authority` values and the stanza-hash canonical form are what an Amaru Agent definition (WP-M3-01 `domain.Agent`, which holds no `mcp` stanza yet) needs; how a real Agent provider reports a tool call; macOS and Windows (the evaluator is pure and has no OS code).

## Decision
1. **Scope.** This ADR covers `domain/grant.go` and `domain/trust.go` of `c8a2f6e`. `hook.go`, `settings.go` and `playbook.go` stay **requires human decision** and unmoved. WP-M3-04 does not need them: hooks belong to the webhook work (`MADR-webhook`, [#95](https://github.com/rluders/amaru/issues/95)), `settings` is configuration ([ADR-0039](0039-configuration-scopes-and-resolution-order.md)) and the Playbook is the Workflow template already written in Amaru for WP-M3-03.
2. **`grant.go`: Reference only, rewritten in Amaru.** No line is copied. Amaru Grants follow ADR-0037, not the prototype shape: a Grant has an id, a scope (Organization or Project), a principal selector, a Capability and resource scope, an optional **definition hash** it is bound to, the human who made it, a time, and a revocation record. A Grant bound to a definition hash lapses when the definition changes. The prototype contributes four ideas, each re-expressed and covered by new tests:
   - a Grant never widens what configuration allows (the allow-list is the ceiling);
   - a Project cannot ship its own Grant: Grants are written only through the Platform API by an authorized human;
   - the hash is canonical (order-independent over servers and tools; an empty tool list differs from a subset);
   - the authority that allowed a Step is recorded with the Step (an `authority` value naming the Grant id, not a bare tag).
3. **`trust.go`: Adapt narrowly, as Project trust.** The content-bound record `{kind, ref, hash, at}` is the shape of FEAT-PRJ-003: trust is bound to content, a change requires re-trust, and an untrusted Project is read-only to Agents. Renames: `TrustPlaybook` → `workflow` (a Workflow definition), `TrustSkill` → `skill`; the Project is added as the scope and the trusting human as the author. Because the prototype has no tests for it, the Amaru tests are written from FEAT-PRJ-003's acceptance, not "ported".
4. **Policy evaluation: an Amaru-written pure function; no new dependency in M3.** `domain` gets one function implementing the ADR-0037 resolution order: explicit deny wins; an action covered by a Grant is allowed; an uncovered action that a rule marks approvable is `require_approval`; anything else is denied, with the ADR-0048 reason codes. Inputs are plain values: the request, the Grants in scope, and the rules from configuration. It performs no I/O and has no port: the Platform loads the inputs and calls it. Reasons for not adopting `canery` now:
   - no license is reported, and the owner decides on a dependency ([AGENTS.md](../../../AGENTS.md#dependencies));
   - the domain may not import a third-party package, so it would need a port and an adapter, which is more structure than a function of this size justifies;
   - ADR-0037 already names the external-engine option as a later backing for the same function.

   `canery` is **not rejected**. Revisit when its license is resolved and the rule set outgrows the function; swapping it then changes the function's body, not its callers.
5. **Enforcement.** The coordinator evaluates Policy **before a Step can lease**, on every path that starts one (a human Submit, a Trigger, a retry), so ungranted use ends the Step `refused` (or `awaiting_approval`) before any Worker sees an Assignment. `needs_grant` parking in the delivery loop stays an M4 concern. Every evaluation is an Event with its reason code. Approvals and Grants are stored by the Platform (migration in WP-M3-04), and an Approval wait survives a restart because its state is in the store, not in a session.
6. **History option: fresh write, with the source cited.** No `git subtree`, no copy. The two source blob ids above are recorded in `services/platform/PROVENANCE.md` as **reference only, no file transferred**, so a reader can see what informed the Amaru types.
7. **Sequence inside WP-M3-04.** #132 (Policy and Grants) first, then #187, #188, #195, #197, #121. #168 (Egress consent) is not started by this ADR; see *Requires human decision*.
8. **Gate.** No code from `grant.go` or `trust.go` is used until this ADR is `accepted` with a recorded approval. Merging this draft is not acceptance.

## Consequences
- **Easier:** Grants and Policy are written once against the accepted model; the evaluator is a pure function that property tests can drive; no dependency is added or license question forced.
- **Harder:** the prototype's per-user MCP stanza behavior (user state directory, `trusted` list) has no direct counterpart; any per-user standing consent beyond Organization and Project Grants needs its own decision.
- **Security:** a Project still cannot grant itself authority, and a changed definition invalidates a bound Grant. The evaluator defaults to deny when no rule covers a request.
- **Maintenance:** Policy has a single owner (the Platform owner). Drift between the evaluator and the contracts in `packages/contracts` is guarded by the contract fixtures.
- **Follow-up:** WP-M3-04 sub-issues implement items 2 to 5; docs ([DOMAIN_MODEL](../DOMAIN_MODEL.md), [POLICY_AND_APPROVAL_SHAPES](../POLICY_AND_APPROVAL_SHAPES.md)) are updated with each.

## Requires human decision
- ~~Accept this ADR (and with it, item 4: no `canery` in M3).~~ Accepted 2026-10-28 (see Approval).
- **Egress consent (#168).** The feature catalog says FEAT-POL-004 reuses `runtime/internal/transport/webhook/sender.go` (private-host block, per-URL consent). Issue #168 names migration ADR #83 (`transport/httpapi`) as its gate, while the webhook sender falls under #95 (`transport/webhook`). The mismatch is not resolved here; `internal/provider/remotejob` already refuses non-public addresses at connect time ([ADR-0054](0054-adapt-the-prototype-remote-job-http-helpers-into-the-worker-providers.md)). Which ADR gates #168, and whether it is written fresh like the others, needs the owner's call.
- `hook`, `settings` and `playbook` dispositions (item 1) remain open.

## Alternatives considered
- **Adapt `grant.go` as is:** carries a per-user file model and an MCP-only stanza hash that contradict ADR-0037's Organization/Project Grants and the revocation and attribution FEAT-POL-001 requires.
- **Adopt `canery` now:** an unlicensed dependency, a port and an adapter for a function of this size, and an unverified fit with ADR-0037 resolution order.
- **An external policy engine (OPA, Cedar):** already rejected for M3 in ADR-0037.
- **Keep Policy in the coordinator instead of the domain:** mixes I/O with evaluation and loses the property-test surface ADR-0037 asks for.

## Approval
Proposed by: Claude (migration and design ADR draft for #16, 2026-10-28). Approved by: Ricardo Lüders (product owner) on 2026-10-28 ("Accept, and merge #290"), including item 4 (no `canery` in M3). The #168 gate and the `hook`, `settings` and `playbook` dispositions stay open.
