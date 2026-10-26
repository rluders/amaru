# Architecture Decisions

Amaru records significant decisions as Architecture Decision Records (ADRs) in [`adr/`](adr/). This file is the convention and the index. The convention itself is [ADR-0001](adr/0001-record-architecture-decisions.md).

## Convention

- **File:** `adr/NNNN-lowercase-title.md`, numbered sequentially, never renumbered or reused. Start from the [template](adr/0000-template.md).
- **Frontmatter** (single-line `key: value` pairs, checked by `make docs-check`):
  - `title`: the decision, as a short phrase
  - `status`: one of the statuses below
  - `date`: `YYYY-MM-DD` of the last status change
  - `deciders`: who approved it — **required when `accepted`**
  - optional: `supersedes`, `superseded-by` (ADR numbers)
- **Statuses:**
  | Status | Meaning |
  |---|---|
  | `proposed` | Written and recommended; not approved. Nothing may be built *only* on its authority. |
  | `under-investigation` | Evidence is being gathered (spike, research). |
  | `accepted` | Approved by a named human decider. Recorded where and when. |
  | `rejected` | Considered and declined; kept for the record. |
  | `superseded` | Replaced by a later ADR (`superseded-by`). |
  | `deprecated` | No longer applies, not replaced. |
- **Acceptance rule:** an ADR becomes `accepted` only with an explicit, recorded human approval (who, when, where). An agent never marks its own proposal accepted. Asking for an investigation is not approval of its outcome.
- **Change rule:** an accepted ADR is not edited in substance; write a new ADR that supersedes it. Typos and links may be fixed.
- **Index:** every ADR has exactly one row below, whose status matches the file (`make docs-check` enforces this).
- **Scope:** significant, hard-to-reverse choices (boundaries, contracts, dependencies on infrastructure, security model, licensing). Unresolved questions live in [OPEN_QUESTIONS](OPEN_QUESTIONS.md) until there is enough evidence for an ADR. Procedure: the `adr-authoring` Skill (`.agents/skills/adr-authoring/SKILL.md`).

## Index

| ADR | Title | Status | Date |
|---|---|---|---|
| [0001](adr/0001-record-architecture-decisions.md) | Record architecture decisions as ADRs | accepted | 2026-10-24 |
| [0002](adr/0002-one-platform-multiple-deployment-models.md) | One Platform, multiple deployment models | accepted | 2026-10-25 |
| [0003](adr/0003-desktop-is-a-client.md) | Desktop is a client, not the owner of execution state | accepted | 2026-10-25 |
| [0004](adr/0004-shared-versioned-contracts.md) | Shared, versioned contracts across deployments and clients | accepted | 2026-10-25 |
| [0005](adr/0005-plugin-extensibility-independent-of-desktop.md) | Plugin extensibility independent of the desktop framework | accepted | 2026-10-25 |
| [0006](adr/0006-human-authorization-for-consequential-actions.md) | Explicit human authorization for consequential actions | accepted | 2026-10-24 |
| [0007](adr/0007-reuse-of-earlier-internal-code-is-a-separate-activity.md) | Migration of the prototype implementation is a separate, approval-gated activity | accepted | 2026-10-24 |
| [0008](adr/0008-incremental-monorepo-layout.md) | Monorepo layout introduced incrementally, by evidence | accepted | 2026-10-25 |
| [0009](adr/0009-modular-monolith-platform-with-separate-worker-role.md) | The Platform is a modular monolith with a separate Worker role | accepted | 2026-10-25 |
| [0010](adr/0010-local-mvp-client-is-a-web-ui.md) | The Local MVP client is a Web UI served by the Platform | accepted | 2026-10-25 |
| [0011](adr/0011-platform-adapted-from-earlier-internal-runtime.md) | The Platform core is adapted from the prototype Runtime | accepted | 2026-10-25 |
| [0012](adr/0012-plugin-package-with-targeted-components.md) | A plugin is one package of components with declared targets | accepted | 2026-10-25 |
| [0013](adr/0013-outbound-pull-based-workers-with-leases.md) | Workers connect outbound, pull assignments and hold leases | accepted | 2026-10-25 |
| [0014](adr/0014-durable-execution-on-own-state-machine-over-postgres.md) | Durable execution is Amaru's own state machine persisted in PostgreSQL | superseded | 2026-10-25 |
| [0015](adr/0015-postgresql-is-the-only-database-bundled-in-compact-installs.md) | PostgreSQL is the only database; compact installs bundle and supervise it | accepted | 2026-10-25 |
| [0016](adr/0016-nats-jetstream-message-bus-embedded-in-compact-installs.md) | NATS JetStream is the message bus, embedded in compact installs | accepted | 2026-10-25 |
| [0017](adr/0017-content-addressed-artifact-store-filesystem-or-s3.md) | Artifacts live in a content-addressed store on the filesystem or S3-compatible storage | accepted | 2026-10-25 |
| [0018](adr/0018-desktop-shell-is-tauri-v2.md) | Amaru Desktop is a Tauri v2 shell over the Web client | accepted | 2026-10-25 |
| [0019](adr/0019-isolation-tiers-for-plugins-and-providers.md) | Plugins and Providers run in fixed isolation tiers | superseded | 2026-10-25 |
| [0020](adr/0020-plugin-logic-runs-in-quickjs-on-wazero.md) | TypeScript plugin logic runs in QuickJS compiled to WebAssembly on wazero | superseded | 2026-10-25 |
| [0021](adr/0021-plugin-packaging-signing-permissions-and-compatibility.md) | Plugins are signed OCI-distributed packages with declared permissions and engine ranges | accepted | 2026-10-25 |
| [0022](adr/0022-cloud-tenancy-row-level-isolation-by-organization.md) | Cloud tenancy is shared infrastructure with row-level isolation per Organization | accepted | 2026-10-25 |
| [0023](adr/0023-self-hosted-oci-image-atlas-migrations-operator-owned-backup.md) | Self-hosted ships as an OCI image with Atlas-authored migrations and operator-owned backup | accepted | 2026-10-25 |
| [0024](adr/0024-remote-worker-registration-identity-and-delivery.md) | Remote Workers are enrolled per Organization with key-based identity and per-Lease delivery | accepted | 2026-10-25 |
| [0025](adr/0025-api-and-protocol-versioning-policy.md) | APIs and protocols are versioned by major with additive-only changes | accepted | 2026-10-25 |
| [0026](adr/0026-fair-code-community-license-with-commercial-licensing.md) | Amaru uses a fair-code community license with separate commercial licensing | accepted | 2026-10-25 |
| [0027](adr/0027-project-definitions-in-git-operational-state-in-platform.md) | Project definitions live in Git under .amaru/; operational state lives in the Platform | superseded | 2026-10-25 |
| [0028](adr/0028-identity-local-accounts-oidc-and-opaque-tokens.md) | Identity uses local accounts and OIDC, server-side sessions and revocable opaque tokens | accepted | 2026-10-25 |
| [0029](adr/0029-worker-protocol-over-websocket-and-protobuf.md) | The Worker protocol is Protobuf over WebSocket using coder/websocket | accepted | 2026-10-25 |
| [0030](adr/0030-knowledge-is-lexically-indexed-project-documentation.md) | Knowledge is project documentation indexed lexically in PostgreSQL | accepted | 2026-10-25 |
| [0031](adr/0031-obsidian-plugin-is-retired.md) | The prototype Obsidian plugin is retired, with no Amaru Obsidian client | accepted | 2026-10-25 |
| [0032](adr/0032-migration-baseline-is-a-pinned-commit.md) | The migration baseline is a pinned commit of the earlier internal implementation | accepted | 2026-10-25 |
| [0033](adr/0033-usage-and-cost-telemetry-model.md) | Usage and cost are recorded per Step Execution with explicit provenance | accepted | 2026-10-25 |
| [0034](adr/0034-local-web-client-authentication-with-launch-code-and-cookie.md) | The local Web client authenticates with a one-time launch code and a loopback session cookie | accepted | 2026-10-25 |
| [0035](adr/0035-agents-run-beside-environment-hosted-tools.md) | Agents run on the Worker beside Environment-hosted tools, behind a Worker-side MCP gateway | accepted | 2026-10-25 |
| [0036](adr/0036-approvals-bind-to-evidence-and-gate-work.md) | Approvals are human decisions bound to specific Evidence that gate Steps and Work Item acceptance | accepted | 2026-10-25 |
| [0037](adr/0037-one-policy-evaluates-capabilities-grants-and-approvals.md) | One Policy evaluation combines requested Capabilities, Grants and Approvals | accepted | 2026-10-25 |
| [0038](adr/0038-roles-teams-and-separation-of-duties.md) | Fixed roles, Teams of humans and Agents, and per-gate separation of duties | accepted | 2026-10-25 |
| [0039](adr/0039-configuration-scopes-and-resolution-order.md) | Configuration resolves deployment, Organization, Project and user scopes in a fixed order | accepted | 2026-10-25 |
| [0040](adr/0040-plugin-logic-runs-in-a-separate-plugin-host-process-built-with-javy.md) | Plugin logic runs as Javy-built QuickJS Wasm on wazero in a separate plugin-host process | accepted | 2026-10-25 |
| [0041](adr/0041-isolation-tiers-for-plugins-providers-and-steps.md) | Plugins, Providers and Steps run in fixed isolation tiers, restated | accepted | 2026-10-25 |
| [0042](adr/0042-durable-execution-with-explicit-restart-semantics.md) | Durable execution is Amaru's own state machine in PostgreSQL, with explicit restart semantics | accepted | 2026-10-25 |
| [0043](adr/0043-project-definitions-in-git-teams-and-operational-state-in-platform.md) | Project definitions live in Git under .amaru/; Team membership and operational state live in the Platform | accepted | 2026-10-25 |
| [0044](adr/0044-restart-semantics-clarifications-from-the-m1-spike.md) | Clarify ADR-0042 restart semantics for unstarted Steps, embedded Workers, timeouts, fenced results and attempts | accepted | 2026-10-26 |
| [0045](adr/0045-compact-installs-have-a-bundled-and-a-slim-flavor-on-one-engine.md) | Compact installs come in a bundled and a slim flavor on one PostgreSQL engine; heavy work runs on remote Workers | accepted | 2026-10-26 |

ADRs 0002–0005 and 0008–0039 were accepted by the product owner on 2026-10-25 in the planning session that closed every question in [OPEN_QUESTIONS](OPEN_QUESTIONS.md). Where the owner delegated a question ("you can decide"), the ADR's Approval section says so. ADRs 0040–0043 were accepted the same day under the owner's delegation in a second planning pass that closed the residual questions Q23–Q54; they supersede 0020, 0019, 0014 and 0027 respectively, which stay in the log as `superseded`. ADRs 0044 and 0045 were drafted by an agent from the WP-M1-02 spike and accepted by the product owner on 2026-10-26.

## ADR backlog

No decision is pending. Expected future ADRs are **migration ADRs**, one per prototype component (M3–M4), each naming the baseline commit of [ADR-0032](adr/0032-migration-baseline-is-a-pinned-commit.md), and **superseding ADRs** if a validation check in the [validation register](OPEN_QUESTIONS.md#validation-register) fails. Implementation work that follows from the accepted ADRs is tracked in the [ROADMAP](ROADMAP.md).

## Relationship to the prototype ADRs

The earlier internal implementation has its own ADR-001…029 (two files numbered 009). They govern that codebase and are **not** Amaru decisions. They are evidence: an Amaru ADR that keeps, changes or drops a prototype decision cites it.
