---
title: Persist the active Project clone in Platform state
status: accepted
date: 2026-11-12
deciders: Ricardo Lüders
---

# 0066. Persist the active Project clone in Platform state

## Context

**Accepted, not implemented.** PR #347 merged into #18 at `17d7f0dfe66bfaea92355bb917b4e5f0fe2634f8`. Project-scoped access and committed Workflow loading exist, but product Project HTTP composition remains incomplete under [#191](https://github.com/rluders/amaru/issues/191).

Verified in source: [Attacher.Attach](../../../services/platform/internal/attach/attach.go) derives identity from a local repository and registers only `domain.Project` (key, name, creation time). Reattaching another clone returns that existing record unchanged. [ProjectStore](../../../services/platform/internal/ports/project.go) has no clone location. [WorkflowReader](../../../services/platform/internal/attach/workflow.go) therefore requires a trusted `Location` callback; no persisted production callback is provided. It already revalidates clone identity and the requested commit's repository roots before reading bounded regular Git blobs.

[ADR-0043](0043-project-definitions-in-git-teams-and-operational-state-in-platform.md) and [ADR-0046](0046-project-key-is-the-normalized-remote-plus-root-commit.md) separate shared repository identity from local clone paths. They do not select a clone when several exist. The existing public [Project schema](../../../packages/contracts/schemas/Project.json) requires an informational absolute `root`; its list is ordered by name. It also requires config/Git inspection metadata that the current Project record cannot supply. Returning a guessed path or empty required fields would not implement that contract.

The immediate prerequisite is an authoritative clone selection rule, not a new public clone-management API. This decision adds detail to the accepted identity model; it supersedes no ADR and approves no predecessor import.

## Decision

**Accepted by the product owner on 2026-11-12; implementation is pending.**

**Pre-release scope clarification (PR #349):** the owner subsequently confirmed that legacy compatibility is unnecessary until Amaru v1.0 is released; see [the repository rule](../../engineering/CONVENTIONS.md#versioning). The upgrade/reversible-migration requirement in item 6 and old-unbound-row evidence below are waived for this implementation. Create the binding table in the initial schema; provide no legacy upgrade, backfill or downgrade path. Normal missing-binding failures, atomic registration and reattachment semantics still apply.

1. **Keep one active local clone binding per Project in each compact Instance.** Store the binding in the Platform's existing PostgreSQL, separately from the shared Project identity and settings. A binding contains the Project key and the canonical absolute repository root. A root belongs to at most one Project in that Instance. It is operational metadata, never a committed Project file, credential, trust decision or authorization grant. Self-hosted/Cloud execution locations remain adapter-owned; do not use this local absolute path as a remote Worker instruction.
2. **Select by explicit authenticated attach.** A successful attach of a second clone of the same Project selects that clone for subsequent operations. Repeating attach at the active root is idempotent. Preserve the existing Project name, creation time, settings and operational state. Concurrent attaches use the last committed transaction, rather than request arrival time. Reject binding a root already registered to a different Project; a changed repository identity requires a future explicit administrative reconciliation, not automatic reassignment.
3. **Commit registration atomically.** The Project registration and active binding update share one transaction. Validation happens before that transaction; failed validation, cancellation before commit and store failures leave both records unchanged. A crash after commit can lose the response, but a retry has the same registration/selection outcome. No filesystem write, move, copy, Git mutation, trust grant or process execution beyond the existing bounded read-only Git inspection is authorized by attach. Public attach still requires an absolute existing root; internal normalization must not weaken that contract.
4. **Use trusted lookup and revalidate.** Public requests select a Project key, never a replacement root for definition reads. After current authentication and attached-Project authorization, resolve its active root through the Platform store. Canonicalize the inspected root with symlinks resolved at attach; revalidate the expected Project identity at use, preserving WorkflowReader's existing commit/regular-blob checks. A missing, moved, replaced, inaccessible or identity-mismatched root fails that Project's operation without fallback to another clone, Project or current working directory. Reattachment is the explicit recovery path. Canonicalization and revalidation do not themselves prove immunity to concurrent filesystem replacement; readers must preserve existing safe Git invocation and confinement, and validation must state race limits honestly.
5. **Capture the selected clone for each operation.** An operation reads one binding; a concurrent reattach affects later operations, not its captured root. Execution creation records the selected clone alongside the existing immutable commit/hash/definition snapshot required by ADR-0043. Reattachment never changes an existing Execution's provenance, permissions or pinned definition. Restart/recovery uses that provenance; it must not silently substitute the current active clone. If the original location is unavailable, use only already durable inputs or fail/wait under the existing lifecycle; do not reload a definition from another root.
6. **Upgrade without inventing paths.** Existing Projects without a binding remain stored and keep their settings/history. An operation requiring a local root fails safely until explicit reattachment. Do not infer paths from keys, scan directories, or choose the daemon's current directory. Add the binding through a reversible additive migration in the existing store; no new database, broker, runtime dependency or background reconciliation service.
7. **Keep wire mapping as an explicit follow-up.** This binding can supply `Project.root` but does not supply config/Git metadata. Before serving list/detail/attach, review and implement those fields against the existing public schemas, including ordering and unavailable-root/error behavior. Never return fabricated `config.present`/`git.present` values or raw Git/filesystem diagnostics. No schema, route, linked-repository grammar or inspector is approved by this proposal. Tool-native Skill discovery remains M7 under ADR-0065. Every other #191/#18 acceptance gate remains in force.

## Consequences

The product has a deterministic, persistent source for the trusted Workflow location callback and eventual Project resource composition. Two clones retain one shared identity and operational state; selecting a clone becomes an explicit human action. The compact Instance chooses one active clone at a time, so clients cannot independently select different clones through the current Project API. Revisit this restriction before multi-host/Remote Worker placement or simultaneous per-client clone selection.

Implementation after approval: domain binding value and store port; fake/PostgreSQL conformance tests and additive migration; atomic attach registration; authorized lookup composed with WorkflowReader; immutable Execution provenance; affected docs and public mapping review. Do not create empty packages or implement proposal-only interfaces before approval. Existing Project-only internal callers remain usable; production attach must use atomic registration rather than split create/bind writes.

Required evidence before claiming this boundary complete: two real temporary Git clones sharing one key and settings/history; same-root repeat; explicit second-root selection; restart lookup from PostgreSQL; concurrent attach commit order; failure/rollback atomicity; old unbound rows; root ownership collision; moved/replaced/symlink roots; foreign key/revision rejection; authentication loss during lookup; no cross-Project fallback; and unchanged Execution provenance after reattachment. Use fakes for automated failure cases and record actual Git/PostgreSQL integration separately. Run `make check`; Linux is required for M3 and other OSes are best effort. None of these runtime cases is validated by this docs-only proposal.

**Architecture review: sound; explicitly approved by the product owner.** Platform retains state and authorization; Clients use the public API; Workers hold no authoritative binding and Workflows remain vendor-neutral. Pure domain code gains no OS imports; canonicalization/Git remain in adapters. Existing PostgreSQL is reused. Atomic writes define crash/retry behavior; read failures stay Project-local. A process-local map is simpler but cannot recover after restart, and choosing any available clone silently changes execution inputs. Local location selection does not establish remote/Cloud placement parity; those adapters must enforce the same identity/provenance boundary with their own location representation.

## Alternatives considered

- **Always keep the first attached clone:** deterministic but a moved/deleted initial clone has no recovery through the current attach operation. Explicit reattachment is the smallest existing human action that can select a usable replacement.
- **Last observed clone or automatic fallback:** lets background reads change execution inputs and makes broken roots hide behind other clones. Selection requires authenticated attach.
- **Store roots on Project identity or in Git:** mixes per-Instance location with repository identity and makes two clones disagree about shared state. Keep a separate operational binding.
- **Process-local map or startup configuration only:** useful test composition, but loses successful attachment after restart or creates a second registration authority.
- **Full clone registry and selection endpoint now:** supports multiple simultaneous choices but expands contracts and placement policy beyond the compact M3 prerequisite. Revisit when concurrent clone selection is required.

## Approval

Accepted by Ricardo Lüders on 2026-11-12 in the delivery conversation for [PR #348](https://github.com/rluders/amaru/pull/348): “Proposal on PR #348 approved. Update, merge, sync, then next task.” This approval accepts the decision; runtime implementation and validation remain pending under #191/#18.


Owner clarification for [PR #349](https://github.com/rluders/amaru/pull/349): “There is no need to worry about legacy. The project wasn't released yet. Adjust the PR #349 accordly.” This narrows pre-release compatibility scope without changing clone identity, selection, authorization or Execution provenance requirements.

The owner made this repository-wide until the Amaru v1.0 release: “Make non-legacy rule, until we release v1.0”. [AGENTS.md hard rule 10](../../../AGENTS.md#hard-rules) and the engineering versioning convention record the rule.
