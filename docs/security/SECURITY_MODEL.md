# Security Model

Status: **accepted planning baseline** (2026-10-25; every decision it relies on is an accepted ADR). **Nothing is implemented**: no Amaru component enforces anything yet. Items marked **[Prototype]** are implemented in the earlier internal prototypes. They are the baseline Amaru must not weaken.

## Assets
- User source code and project files.
- Secrets: API keys, tokens, webhook and plugin secrets.
- Execution history, Evidence and audit Events.
- Approvals, Grants and Policies.
- The user's machines (process execution) and Workers' machines.
- In Cloud: other tenants' data.

## Trust boundaries

| Boundary | Trusted side | Untrusted side | Controls |
|---|---|---|---|
| User → Instance | Instance (after authentication) | Any caller until authenticated | Local: loopback, token, Origin check **[Prototype]**; browser: one-time launch code → HttpOnly SameSite=Strict session cookie, Host allowlist, Origin and CSRF checks ([ADR-0034](../architecture/adr/0034-local-web-client-authentication-with-launch-code-and-cookie.md)) [Proposed]. Server/Cloud: local accounts or OIDC, server-side sessions, revocable opaque tokens ([ADR-0028](../architecture/adr/0028-identity-local-accounts-oidc-and-opaque-tokens.md)) [Proposed] |
| Organization ↔ Organization | Platform scoping | Other Organizations' users and data | Every query scoped by Organization; tests prove no cross-Organization reads (FEAT-IAM-003) [Proposed]. **[Prototype]** has workspaces but does not enforce project scoping |
| Project ↔ Project | Platform | Other Projects in the same Instance | Secrets, events and settings scoped per Project (a secret's visibility is Organization or Project; all are encrypted with the per-Organization data key, [ADR-0022](../architecture/adr/0022-cloud-tenancy-row-level-isolation-by-organization.md)); F-10 regression **[Prototype]** |
| Platform → Worker | Platform (state, Policy) | Worker reports (validated, not trusted blindly) | Outbound-connecting Workers with machine identity; assignment-scoped secrets; at-most-once completion ([ADR-0013](../architecture/adr/0013-outbound-pull-based-workers-with-leases.md)) [Proposed] |
| Worker → Provider process | Worker | Provider process and its output | argv, cwd, env allowlist, stdin prompts, process groups **[Prototype]**; refusal when a provider cannot enforce confinement (FEAT-POL-006) [Proposed] |
| Agent ↔ Execution Environment | Platform-assembled instructions and Policy | Project files, untrusted Skills, tool output (prompt injection) | Hermetic runs, tool scope, workspace-command validation and denylist, OS sandbox where available **[Prototype]**; container sandbox Environment **[Prototype]** |
| Plugin ↔ host | Platform Policy and extension hosts | Plugin code, always | Declared permissions, consent, isolated frames for UI, Wasm `logic` in a separate plugin-host process ([ADR-0040](../architecture/adr/0040-plugin-logic-runs-in-a-separate-plugin-host-process-built-with-javy.md)), out-of-process adapters ([EXTENSIBILITY](../architecture/EXTENSIBILITY.md)) [Proposed] |
| Local workstation ↔ remote Platform | User's machine | Remote Platform requests | A Cloud or remote Platform never reaches a laptop directly; local access only through a user-run Worker with declared filesystem roots [Proposed] |
| Cloud tenant ↔ tenant | Vendor isolation | Other tenants | Row-level isolation per Organization with PostgreSQL RLS (FORCE, non-owner role, per-transaction tenant setting), per-Organization envelope keys and artifact prefixes, single-tenant vendor Worker leases ([ADR-0022](../architecture/adr/0022-cloud-tenancy-row-level-isolation-by-organization.md)) [Proposed] |
| Platform ↔ secrets and external providers | Secrets port | Providers, plugins, logs | Secrets are write-only from clients, injected per Step, and never in Events, logs, Artifacts or errors **[Prototype]**; redaction of stored output [Proposed] |
| Inbound triggers | Platform Policy | Webhook senders, schedules | HMAC + replay protection + rate limits **[Prototype]**; every trigger goes through Policy [Proposed] |

## Baseline rules
1. **Local Platform.**
   - Binds to loopback only and authenticates every request except health.
   - Keeps its token in a user-only file and checks `Origin` **[Prototype]**.
   - The browser client uses a one-time launch code exchanged for a session cookie, a Host-header allowlist against DNS rebinding, Origin checks and CSRF tokens ([ADR-0034](../architecture/adr/0034-local-web-client-authentication-with-launch-code-and-cookie.md)).
   - The state file is restricted to the user's SID on Windows; prototype never verified its ACLs, so this is validated in M4.
2. **No auto-approval** of agent permissions. Consequential actions need a Grant or an Approval ([ADR-0006](../architecture/adr/0006-human-authorization-for-consequential-actions.md), accepted). The Platform never merges pull requests in the MVP.
3. **Process execution:**
   - argv arrays, explicit cwd, environment allowlist, prompts over stdin, no shell **[Prototype]**;
   - only `git` and `gh` receive SSH agent and GitHub configuration **[Prototype]**.
4. **Secrets** reach Providers only through a secrets port. They never appear in Events, logs, Artifacts, errors or UI notices **[Prototype]**, and are write-only from clients **[Prototype]**.
5. **Project content is untrusted.** Nothing project-supplied executes implicitly. Skills and linked repositories require explicit, content-bound trust **[Prototype]**. *Status:* the Platform records a human's trust of a Project and of one Workflow definition by hash, and denies every Step of an untrusted Project or an untrusted definition before a Worker sees it (FEAT-PRJ-003; integration-tested on Linux; Skills and linked repositories are not yet covered).
6. **Writes**: nothing the Platform writes at runtime lands in the Project tree; operational state is in PostgreSQL and outputs in the Artifact store ([ADR-0043](../architecture/adr/0043-project-definitions-in-git-teams-and-operational-state-in-platform.md), [ADR-0017](../architecture/adr/0017-content-addressed-artifact-store-filesystem-or-s3.md)) [Proposed]. Agents write only inside their worktree **[Prototype]** (the prototype confined Platform writes to its project definition folder).
7. **Egress:**
   - remote Providers and outbound webhooks are disabled until the user enables them;
   - webhook egress consent is bound to the exact URL;
   - private destinations are blocked **[Prototype]**.
8. **Inbound triggers** (webhooks) authenticate with HMAC plus replay protection **[Prototype]**, and go through Policy like any other start [Proposed].
9. **Workers** never accept inbound connections for assignments. Secrets they receive are scoped to one assignment [Proposed].
10. **Environment-hosted MCP tools** (e.g. the Unreal Editor's MCP server) are reached only by an agent on that Worker, through the Worker-side MCP gateway; each tool call is Policy-checked and audited, and MCP credentials stay on the Environment ([ADR-0035](../architecture/adr/0035-agents-run-beside-environment-hosted-tools.md)) [Proposed].
11. **Plugins** run out of process. Third-party code never runs inside the Platform or Desktop host; Wasm `logic` runs in the `amaru plugin-host` process ([ADR-0040](../architecture/adr/0040-plugin-logic-runs-in-a-separate-plugin-host-process-built-with-javy.md), [ADR-0041](../architecture/adr/0041-isolation-tiers-for-plugins-providers-and-steps.md)) [Proposed].
12. **Default consequential actions** [Proposed, Policy defaults for [ADR-0037](../architecture/adr/0037-one-policy-evaluates-capabilities-grants-and-approvals.md)]. Without a standing Grant, these return `require_approval`: `git.push` to a shared remote, merging a pull request, deploys, writes to external trackers, network egress outside the Organization allowlist, deleting files outside the Step's worktree, installing or upgrading a plugin, and first use of a secret by a Capability. Workflows and plugins cannot lower these defaults; only an Owner or Admin can, by Grant. **Status (2026-10-28):** the defaults except the Organization allowlist for egress and first use of a secret are implemented in `services/platform` (`domain.DefaultRules`) and unit- and integration-tested on Linux against fakes and PostgreSQL; a `require_approval` result makes the Step wait in `awaiting_approval` with a requested Approval (FEAT-REV-001, #195); a person with the Maintainer role or above decides it, and nothing runs before then. Evidence binding beyond the Execution, Step, inputs and definition, and separation of duties are not implemented.

## Known gaps to close before the MVP
- No general capability Policy (prototype GAP-09; grants exist) → FEAT-POL-001.
- Approval not bound to evidence or voided on subject change → FEAT-REV-003.
- Codex and OpenCode providers do not enforce `WorkspaceWrite` → FEAT-POL-006.
- Local Checks run unsandboxed (documented prototype residual risk) → disclosed in the MVP UI; sandboxed in M5 (FEAT-VAL-004).
- Provider output and stderr stored unredacted; no retention (prototype GAP-07) → FEAT-ART-003: per-Lease secret values are redacted on the Worker before emission, plus pattern scanning; retention defaults per data class in [DATA_ARCHITECTURE §5](../architecture/DATA_ARCHITECTURE.md).
- Any process running as the same OS user can obtain a session through the state-file secret and attach any directory as a project (documented prototype residual risk). **Accepted** for compact installs: the OS user is the trust boundary ([ADR-0034](../architecture/adr/0034-local-web-client-authentication-with-launch-code-and-cookie.md)); multi-user Instances require authenticated users with roles ([ADR-0038](../architecture/adr/0038-roles-teams-and-separation-of-duties.md)).
- Single-instance enforcement is a liveness probe, not a lock → start-up lock ([RESILIENCE](../architecture/RESILIENCE.md#3-failure-catalogue)).

## Future boundaries (decided, built at their milestone)
- User identity and roles: [ADR-0028](../architecture/adr/0028-identity-local-accounts-oidc-and-opaque-tokens.md), [ADR-0038](../architecture/adr/0038-roles-teams-and-separation-of-duties.md) (M7).
- Remote Worker enrollment, key identity and per-Lease secret delivery: [ADR-0024](../architecture/adr/0024-remote-worker-registration-identity-and-delivery.md), [ADR-0029](../architecture/adr/0029-worker-protocol-over-websocket-and-protobuf.md) (M5); Worker-side MCP gateway with per-call audit: [ADR-0035](../architecture/adr/0035-agents-run-beside-environment-hosted-tools.md) (M5).
- Plugin isolation tiers, Wasm logic sandbox and cosign signing: [ADR-0041](../architecture/adr/0041-isolation-tiers-for-plugins-providers-and-steps.md), [ADR-0040](../architecture/adr/0040-plugin-logic-runs-in-a-separate-plugin-host-process-built-with-javy.md), [ADR-0021](../architecture/adr/0021-plugin-packaging-signing-permissions-and-compatibility.md) (M6).
- Tenant isolation: [ADR-0022](../architecture/adr/0022-cloud-tenancy-row-level-isolation-by-organization.md) (M9).

## Reporting
[SECURITY.md](../../SECURITY.md) exists since 2026-10-26 (owner-approved; the reporting channel is enabled when the repository goes public; the full process is FEAT-POL-008, not implemented). Decision: a `SECURITY.md` policy with private reporting through GitHub Security Advisories and a contact address, a 90-day coordinated disclosure window, is published before the first public release (roadmap WP-M1-01; FEAT-POL-008).
