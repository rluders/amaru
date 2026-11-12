---
title: Bind public operations and secret resolution to an authorized Project
status: accepted
date: 2026-11-12
deciders: Ricardo Lüders
---

# 0062. Bind public operations and secret resolution to an authorized Project

## Context

[PRJ-002 #191](https://github.com/rluders/amaru/issues/191), under [#18](https://github.com/rluders/amaru/issues/18), requires isolated queries, Events, settings, definition resolution and same-name secrets, with independent progress for multiple attached Projects. Native prerequisite #120 and inherited #15/#83/#84 are closed and their changes are on the #18 integration base `007f69448f4c2556574205763bfda6447f9f0c0d`.

Verified source boundaries, rather than a claim of a currently exposed vulnerability:

- [ProjectStore](../../../services/platform/internal/ports/project.go) and [Project](../../../services/platform/internal/domain/project.go) hold stable repository identity, not Organization membership or an authorized request scope. [ADR-0046](0046-project-key-is-the-normalized-remote-plus-root-commit.md) remains the identity rule.
- [ExecutionStore](../../../services/platform/internal/ports/execution.go) reads by Execution/Approval ID; [Cancel](../../../services/platform/internal/coordinator/cancel.go) and [Evidence](../../../services/platform/internal/evidence/evidence.go) accept an Execution ID without a requested Project. These internal APIs support trusted coordinator work; they are not safe public authorization boundaries by themselves.
- [EventStream](../../../services/platform/internal/ports/event_stream.go) already filters by Project, with [shared isolation tests](../../../services/platform/internal/ports/porttest/event_stream.go). Its caller still has to authorize that Project.
- [ConfigLayer](../../../services/platform/internal/domain/config.go) carries a precedence scope, not a Project identity. Its pure resolver cannot establish where supplied layers came from. There is no secrets port under `services/platform/internal/ports` at this baseline.
- [ADR-0039](0039-configuration-scopes-and-resolution-order.md) permits names in configuration but no secret values. [Q50](../OPEN_QUESTIONS.md#q50-secrets-scope-versus-encryption-key) permits Organization- and Project-visible secrets; it does not choose ambiguous name lookup or fallback behavior.

The security baseline already requires Project isolation. This decision chooses the application boundary and ambiguous secret-reference behavior needed to enforce it. It neither claims a running public API nor implements an encrypted secret store.

## Decision

**Accepted by the product owner on 2026-11-12; implementation remains pending.**

1. **Authorize a Project before accessing its resources.** Public command/query entry points resolve the requested Project against trusted authenticated Instance/Organization access and carry that resolved scope explicitly. An ID from a path, body, Event filter or client header is a selector, never authority. Compact M3 composition serves one configured Organization; it grants the local administrator access to attached Projects, without pretending to implement multi-Organization membership or RLS. Empty or unattached Project scope is refused before resource lookup or mutation.
2. **Keep internal scheduling distinct from public access.** Retain the trusted coordinator's Instance-wide queue/reconciliation APIs. Add Project-bound application operations for public reads and mutations instead of exposing raw stores. Before reading child data, returning command replay or executing a mutation, verify that its Execution, Approval, Work record or definition belongs to the resolved Project. A foreign and an absent resource have the same public not-found result, with no foreign details, cursor, original command outcome or side effects. Recheck current authorization on replay; preserve atomic commits and version checks. Project identity is immutable for stored Executions. All public callers, including Evidence, definition and Artifact access, must use this boundary.
3. **Preserve the existing command-key contract.** [ADR-0058](0058-command-idempotency-conflicts-and-retention.md)'s `(command name, key)` namespace, normalized request identity and no-expiry policy stay in force. Two Projects deliberately reusing one key for different requests receive a conflict, not an independent namespace or another Project's result. Normal independent work uses distinct keys. Test this exception explicitly; do not silently change key storage to make an isolation test pass. Multi-Organization replay remains prohibited until ADR-0022 isolation is implemented.
4. **Select configuration and definitions by the resolved Project.** Application loaders assemble Deployment and the configured Organization layers plus only the selected Project's definitions/settings; then reuse `ResolveConfig` with ADR-0039 precedence, locks and tightening. User preferences never affect execution. Cache keys include the Project and definition/configuration revision; same-name Project definitions cannot replace one another. Attached roots stay at their original locations, with operational metadata outside the trees under ADR-0043. Cross-Project Work dependencies are refused at the authoritative mutation boundary. Invalid definitions affect only their owning Project.
5. **Make secret visibility explicit at resolution.** A Project reference resolves exactly `(Organization, Project, name)`; a missing Project value never falls back to another Project, a user-level name or an Organization value. An Organization reference resolves exactly `(Organization, name)` and is explicitly identified as such by the selected connection/definition. Existing name-only Project configuration means a Project reference; no new public syntax is introduced by this ADR. Names are case-sensitive and are not normalized into aliases. Policy and first-use Approval apply to the selected secret identity and Capability before delivery; scope resolution never grants permission. Missing/denied values produce safe errors without secret values or foreign existence details. Plaintext reaches only an authorized assignment through a secrets port and its redaction path, never clients, Events or diagnostics. Typed reference/connection wire shapes need shared schema/fixture/SDK changes before public exposure.
6. **Preserve independent progress.** Reuse existing per-Project concurrency limits and persisted Step wait/failure reasons. A Project waiting for Approval, containing an invalid definition or using an unavailable Provider must not abort scheduling of eligible work in another Project. An Instance-wide capacity limit may intentionally constrain both. This does not promise fair scheduling, parallel Workflow branches or retries, which are outside #191.

## Consequences

The implementation changes application contracts across commands, queries and composition; adding a filter to one HTTP handler is insufficient. Workers continue to receive assignments and hold no authoritative Project state. Domain code remains pure; Clients use the public API. No vendor, infrastructure, dependency, predecessor migration or license change is authorized here. Organization encryption and future multi-Organization storage remain governed by ADR-0022; no plaintext production secret backend is an acceptable shortcut.

Implement and review in this order after approval:

1. Inventory every public Project resource operation from OpenAPI and the command inventory. Add the common authorized Project boundary, migrate its callers and check foreign/missing IDs before replay. Run existing coordinator/port suites to preserve internal recovery behavior.
2. Add Project-selected settings/definition loading and an exact-scope secrets port/resolver. Use fake secret delivery for #191 regression tests. Record the missing encrypted production backend and assignment integration as open delivery gates; fake resolution alone cannot close #191.
3. Attach two temporary Git Projects with matching definition and secret names. Verify separate settings, Work/dependency validation, Execution queries, Evidence and Event cursors; mutate both from two clients. Foreign IDs, stale/duplicate requests and denied operations must leave state and Event counts unchanged. A missing Project secret must fail even when the other Project or Organization has that name. Explicit Organization lookup must still require Policy/Approval.
4. Exercise actual coordinator dispatch with fake Providers: one Project fails, one waits for Approval and the healthy Project progresses. Capture every test output channel using public secret fixtures and assert absence. Include restart and concurrent mutation cases for affected durable adapters. Run PostgreSQL integration checks when migrations/store semantics change and finish with `make check` on both the candidate and merged result.
5. Verify the composed #18 authenticated API with two clients and linked contract/SDK fixtures. Record Linux evidence and unrun macOS/Windows behavior under the existing best-effort scope. #191, #18 and M3 remain open until their respective acceptance is demonstrated.

Boundary review verdict: **sound; approved by the product owner** for the public application boundary and explicit secret visibility lookup. Existing Event filtering and pure configuration resolution are reusable, but not authorization evidence. Storage failures must return without publishing a successful change; command mutation/outcome and domain Events retain their existing atomic commit. Missing scope, unknown ownership, secret lookup failure and invalid Project content fail closed for that Project. This source review is not runtime or security-test evidence.

## Alternatives considered

- **Check Project only in individual HTTP handlers:** smallest initial edit, but Evidence, nested IDs, replay and future transports can bypass it. Centralize the public application boundary while retaining trusted internal APIs.
- **Add Project to every internal store method immediately:** changes scheduler/recovery APIs that legitimately traverse Projects, without proving public callers use authorized scope. Keep that wider storage/Organization migration separate and explicitly gated before multi-Organization exposure.
- **Search all secret scopes by name or fall back to Organization:** convenient, but can silently select a different credential after deletion or a spelling error. Require exact visibility; Organization sharing remains available explicitly.
- **Create an Instance or process per Project:** avoids some shared state but duplicates supervision and contradicts the multi-Project Instance outcome. Reuse the Platform's existing scoped state and limits.

## Approval

Approved by Ricardo Lüders in the repository task conversation on 2026-11-12: “PR $330 proposal is approved, update and merge it.” Recorded in [PR #330](https://github.com/rluders/amaru/pull/330). This approval accepts the decision; it does not establish #191 implementation or validation.
