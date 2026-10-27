# Amaru — Product Requirements

Status: **accepted planning baseline** (2026-10-24; reconciled with the master roadmap and reviewed by the product owner 2026-10-25). Nothing described here is implemented in this repository. Where behavior already exists in a predecessor, it is marked *(the prototype)* for the earlier internal Runtime or *(earlier prototype)* for a second, earlier internal prototype. Terms are defined in the [glossary](GLOSSARY.md). Features with stable IDs, priorities and acceptance criteria are in the [feature catalog](FEATURE_CATALOG.md); quality attributes in the [non-functional requirements](NON_FUNCTIONAL_REQUIREMENTS.md).

## 1. Vision

Amaru is an engineering execution platform that coordinates humans, AI agents, tools, knowledge, workflows and execution environments across the software development lifecycle.

The engineer remains the maestro of execution. Humans set intent, scope, priorities, quality criteria and approval boundaries; agents carry out delegated work inside explicit policies; workflows coordinate multi-step activities; and every result, decision and piece of evidence stays observable and auditable.

Amaru does not replace Git, GitHub, issue trackers, IDEs, CI or specialized engineering tools. It integrates with them and orchestrates work across them.

## 2. Problem

Engineers increasingly delegate work to AI agents and automated tools, but that work is scattered across terminals, chat windows, CI logs and ad-hoc scripts:

- **No shared record.** What an agent was asked, with which context, what it changed, and why the result was accepted is rarely captured in one place.
- **No consistent control.** Permissions, approvals and boundaries differ per tool; consequential actions can happen without a deliberate human decision.
- **No coordination.** Multi-step work (investigate → implement → validate → review) is stitched together by hand, and a crash or restart loses its state.
- **Tool and machine lock-in.** Specialized tools (a GPU workstation, a game engine on Windows, a licensed CAD tool) live on specific machines; work cannot easily be routed to them.
- **Vendor lock-in.** Workflows are written against one agent vendor and break when the user switches.
- **Wasted runs.** Agent runs that repeat, review without evidence or produce nothing cost money and attention; the cost of an *accepted* change is invisible. *(prototype dogfood cycle 7: one accepted change took 2 useful and 6 wasted LLM runs.)*

## 3. Target users

| User | Needs | First served at |
|---|---|---|
| **Individual engineer** working locally with AI agents and tools | Delegate work with clear boundaries, keep evidence, recover from failures, switch agent vendors without rewriting workflows. | Local MVP (M4) |
| **Technical lead of a small team** | Shared workflows and Skills, review and approval of agent work, visibility of what ran, why and at what cost. | Self-hosted (M7) |
| **Self-host operator** (platform/infra engineer) | Install, upgrade, back up and secure an Amaru deployment inside their own network; attach Workers. | Self-hosted (M7) |
| **Specialized-environment owner** (game, graphics, hardware engineer) | Expose a Windows/GPU/licensed-tool machine as an Execution Environment without opening inbound ports. | Remote execution (M5) |
| **Plugin / integration author** | Stable contracts and a TypeScript SDK to add Capabilities, Providers, integrations and UI. | Plugin platform (M6) |
| **Cloud customer** | Executions that continue while their workstation is off; managed infrastructure. | Cloud (M9) |

### Use cases

| ID | Use case | Evidence |
|---|---|---|
| UC-1 | **Delegated change with approval.** An engineer defines a Work Item, runs a Workflow where an agent implements a change in an isolated worktree, local checks validate it, an agent review cites the evidence, and the engineer approves before a draft pull request is opened; the human merges. | *(the prototype: delivery loop, local checks, review step, approval step, draft PRs — Cycle 7 supervised pilot.)* |
| UC-2 | **Recoverable multi-step run.** A long Workflow survives a Platform or machine restart and resumes or fails explicitly, never silently. | *(the prototype: restart semantics, Resumable/Detachable remote jobs, restart recoveries per attempt.)* |
| UC-3 | **Route work to a specialized machine.** A Step needing Unreal Engine 5 runs on a Windows VM where the editor and its MCP server are available; an agent uses the editor's MCP tools under Policy while the engineer works on a laptop. The VM starts on demand and stops when idle. | *(New.)* |
| UC-4 | **Vendor-neutral agent work.** The same Workflow runs with different agent providers chosen per Agent. | *(the prototype: Claude, Codex, OpenCode behind `agent.run`.)* |
| UC-5 | **Audit a past result.** Months later, someone sees which context, Skills, provider, inputs, outputs, checks and cost produced an Artifact, and who approved it. | *(the prototype: context/Skill snapshots and hashes; earlier prototype: approval linked to execution and verification run.)* |
| UC-6 | **Work continues without my laptop.** A self-hosted or Cloud Platform keeps executing and waiting for approvals while the engineer's Desktop is closed; the engineer approves from any client later. | *(New.)* |
| UC-7 | **Extend Amaru for a domain.** A plugin author ships a package that adds a Capability, a Provider for a remote Worker, and a UI panel to view its Artifacts, installed with declared permissions. | *(the prototype: external provider protocol only.)* |
| UC-8 | **Understand the cost of accepted work.** A lead sees attempts, runs, tokens and cost per accepted Work Item and why runs were wasted. | *(New; prototype finding F-41: usage is dropped.)* |

## 4. Product principles

1. **Human authority.** Consequential actions require an explicit, recorded human decision ([ADR-0006](../architecture/adr/0006-human-authorization-for-consequential-actions.md)). No auto-approved agent permissions.
2. **Evidence over claims.** Results carry the evidence that produced them. The product never presents planned or simulated behavior as real.
3. **Orchestrate, don't replace.** The user's repositories and tools stay authoritative. Amaru never moves or takes ownership of user files.
4. **Vendor and tool neutrality.** Workflows reference Capabilities, not vendors (Capability → Provider → Adapter, proven in the prototype).
5. **One platform, many deployments.** Cloud and self-hosted share one domain model and one set of contracts ([ADR-0002](../architecture/adr/0002-one-platform-multiple-deployment-models.md)).
6. **Clients are clients.** Web, Desktop and the command-line client present and command; they do not own execution state ([ADR-0003](../architecture/adr/0003-desktop-is-a-client.md)).
7. **Incremental by evidence.** Infrastructure (queues, databases, durable engines, distributed workers) is introduced only by a recorded decision that states its need and how compact installs carry it without user operation (PostgreSQL and NATS: [ADR-0015](../architecture/adr/0015-postgresql-is-the-only-database-bundled-in-compact-installs.md), [ADR-0016](../architecture/adr/0016-nats-jetstream-message-bus-embedded-in-compact-installs.md)).
8. **Secure by default.** Least privilege, explicit trust for project-supplied content, no secrets in logs or events.
9. **Optimize for accepted work.** Measure and reduce the total cost of a correct, reviewed change, not the cost of one model call.

## 5. Core domain concepts

Organization, Instance, Project, Work Package, Work Item, Workflow, Step, Execution, Step Execution, Event, Artifact, Evidence, Local Check, Agent, Team, Skill, Capability, Provider, Execution Environment, Worker, Lease, Approval, Grant, Policy, Plugin, Plugin Component. Definitions: [glossary](GLOSSARY.md). Model, relationships and lifecycles: [domain model](../architecture/DOMAIN_MODEL.md).

## 6. Product surfaces and deployment models

| Surface | Role | Status |
|---|---|---|
| **Amaru Platform** | Owns domain state, contracts, policies and execution coordination. | Proposed. the prototype's Go Runtime is the closest existing implementation and the intended code source ([ADR-0011](../architecture/adr/0011-platform-adapted-from-earlier-internal-runtime.md)). |
| **Workers / Execution Environments** | Run Steps through Providers on a given machine. | Proposed ([execution environments](../architecture/EXECUTION_ENVIRONMENTS.md)). The prototype runs everything on the local machine. |
| **Amaru Web** | Browser client; **the first-party client of the Local MVP**, served by the Platform. | Proposed ([ADR-0010](../architecture/adr/0010-local-mvp-client-is-a-web-ui.md)). earlier prototype dashboard is the starting asset. |
| **Amaru Desktop** | Native client wrapping the same UI; Tauri v2 thin Rust host with the Platform as sidecar ([ADR-0018](../architecture/adr/0018-desktop-shell-is-tauri-v2.md)). | Proposed for M8. |
| **Amaru Self-hosted** | The Platform operated by the user or their organization: compact, small-organization or distributed. | Proposed for M7 ([deployment](../architecture/DEPLOYMENT.md)). |
| **Amaru Cloud** | The Platform operated as a managed service. | Proposed for M9. |
| **Plugins** | Packages of UI, sandboxed TypeScript logic, services and Worker/local adapters. | Proposed for M6 ([extensibility](../architecture/EXTENSIBILITY.md)). The prototype has an external provider protocol. |
| **Command-line client** | Scripting and automation over the public API. | Proposed for M7 (FEAT-CLI-012). Local operator subcommands of the Platform binary (`amaru open`, `migrate`, `backup`, `admin check`, `worker`, `plugin-host`) ship with the Platform from M3/M4 ([DEPLOYMENT](../architecture/DEPLOYMENT.md)). |
| **Editor integrations** | Clients over the same contracts. | The prototype Obsidian plugin is retired and has no Amaru successor ([ADR-0031](../architecture/adr/0031-obsidian-plugin-is-retired.md)); other editor integrations would be Plugins. |

A local, single-user deployment (Platform + embedded Worker + Web client on the engineer's machine) is the first target.

## 7. Functional requirements

IDs are stable and never reused; gaps in numbering are intentional room for additions. **Phase** gives the [roadmap milestone](../architecture/ROADMAP.md) at which the requirement must hold. Each requirement is realized by the features listed in the [feature catalog](FEATURE_CATALOG.md#4-requirement-traceability).

### 7.1 Projects and work
| ID | Requirement | Phase |
|---|---|---|
| FR-1 | Attach an existing Project without moving, rewriting or taking ownership of its files; Amaru metadata is stored separately. | M3 *(the prototype)* |
| FR-2 | Create and track Work Packages and Work Items with human-set status, dependencies, and links to Executions. | M3 *(the prototype)* |
| FR-3 | Link Work Items to external trackers (GitHub issues, etc.) without duplicating them as the source of truth. | M7 *(the prototype: GitHub issue intake)* |
| FR-4 | Declare linked repositories per Project and deliver changes to them as draft pull requests that humans merge. | M4 *(the prototype: ADR-026)* |

### 7.2 Workflows and execution
| ID | Requirement | Phase |
|---|---|---|
| FR-10 | Define Workflows as versioned graphs of Steps with ordering and data edges, stored in a reviewable text format; every Execution records the definition version it ran. | M3 *(the prototype: YAML DAG)* |
| FR-11 | Run a Workflow or a single Step as an Execution with a defined lifecycle, cancellation and an ordered, replayable Event stream. | M3 *(the prototype)* |
| FR-12 | Persist Execution state so a Platform restart never silently loses or duplicates work; interrupted Steps keep waiting, resume, retry when idempotent, or end `interrupted` ([ADR-0042](../architecture/adr/0042-durable-execution-with-explicit-restart-semantics.md)). | M3 *(the prototype)* |
| FR-13 | **Approval Step**: pause an Execution until an authorized human approves or rejects, recording the decision and the Evidence shown. Waits survive restarts. | M3 *(the prototype: approval steps; earlier prototype: evidence binding)* |
| FR-14 | Retries with backoff, parallel branches, per-Step timeouts, conditional branching. No saga-style compensation: failure edges route to explicit cleanup Steps. | Timeouts M3; retries and parallel branches M5; branching P2 (FEAT-WFL-011) |
| FR-15 | Reusable Workflow templates instantiated into Projects. | M7 *(the prototype: ADR-018)* |
| FR-16 | **Supervised delivery loop**: an opt-in per-Work-Package reconciler that drives Work Items through implement → checks → review → draft PR within attempt and repair budgets, never merging without a human. | M4 *(the prototype: Autoloop)* |
| FR-17 | Attempt accounting: a run that produced no change, or an interruption by restart, does not consume an attempt; unblocking resumes from the right phase. | M4 *(the prototype: partly; F-39, F-40 open)* |

### 7.3 Agents, Skills, Capabilities, Providers
| ID | Requirement | Phase |
|---|---|---|
| FR-20 | Define Agents with instructions, model/provider preference, Skill references, requested Capabilities and tool scope. | M3 *(the prototype)* |
| FR-21 | Skills are user-owned, explicitly trusted, optionally hash-pinned; context assembly is deterministic and recorded per Execution. | M3 *(the prototype)* |
| FR-22 | Capabilities have typed specs; Workflows reference Capabilities, never vendors. Adding a Provider changes no core code, contract shape or UI architecture. | M3 *(the prototype)* |
| FR-23 | Third parties add Providers without rebuilding the Platform (external provider protocol). | M3 *(the prototype has one)* |
| FR-24 | **Capability policy**: an Agent, Workflow or Plugin can use only Capabilities its Policy grants; ungranted use is refused before start. A Provider that cannot enforce a requested confinement refuses the Step. | M3 *(New; prototype GAP-09; prototype grants ADR-024)* |
| FR-25 | Agent review Steps produce a structured verdict that cites the validation Evidence of the reviewed revision. | M4 *(the prototype: F-12, F-38)* |

### 7.4 Artifacts, evidence and cost
| ID | Requirement | Phase |
|---|---|---|
| FR-30 | Record outputs as Artifacts in the content-addressed store with role, logical name and content hash computed by the Platform; generated output reaches the Project tree only through an explicit Step ([ADR-0017](../architecture/adr/0017-content-addressed-artifact-store-filesystem-or-s3.md)). | M3 *(the prototype)* |
| FR-31 | Each Execution keeps its Evidence: assembled context, Agent/Skill snapshots, inputs, outputs, diffs, logs and validation results. | M3 *(the prototype, partial)* |
| FR-32 | Large Artifacts in object storage with retention policies. | M7 |
| FR-33 | Record token usage and cost per Step Execution and aggregate per Work Item, Work Package and Project. | Per-step M4; aggregation M7 *(New; prototype F-41)* |
| FR-34 | Local Checks run against a specific commit in an isolated worktree, and their results are bound to that commit and check configuration. | M4 *(the prototype: ADR-029)* |

### 7.5 Clients, plugins, environments, identity
| ID | Requirement | Phase |
|---|---|---|
| FR-40 | All clients use the same versioned public contracts; no client-only backdoors. | M2 |
| FR-41 | One first-party client for the Local MVP: **Amaru Web served by the local Platform** ([ADR-0010](../architecture/adr/0010-local-mvp-client-is-a-web-ui.md), accepted; product-owner direction 2026-10-25). | M4 |
| FR-42 | Plugins declare their components, contributions and permissions; the Platform and clients enforce them. | M6 |
| FR-43 | Register remote Execution Environments (e.g. a Windows VM with Unreal Engine 5) and route Steps to them by required Capability and labels, including tools that need an interactive session and MCP servers hosted on the Environment. | M5 |
| FR-47 | Provision Execution Environments from templates on demand (start, stop when idle, reset to a snapshot) through provisioner plugins. | M7 |
| FR-44 | Multi-user projects, roles, separation of duties configurable per gate: the approver differs from the initiator and the Agent's human sponsor; Agents never approve; disabled (and recorded) in single-user compact installs ([ADR-0038](../architecture/adr/0038-roles-teams-and-separation-of-duties.md)). | M7 *(the prototype: Teams ADR-019; earlier prototype: required reviewer team)* |
| FR-45 | A native Desktop client manages several Instances, bundles the Platform as a sidecar and supervises it when a local Instance is used, and exposes native capabilities only through an explicit host API. | M8 |
| FR-46 | Organizations, user accounts, sessions and API tokens for self-hosted and Cloud deployments. | M7 *(earlier prototype: users, JWT, workspaces — incomplete)* |

## 8. Non-functional requirements

Canonical list, targets and deployment differences: [NON_FUNCTIONAL_REQUIREMENTS](NON_FUNCTIONAL_REQUIREMENTS.md). Summary of the original IDs, which keep their meaning there:

| ID | Requirement | Phase |
|---|---|---|
| NFR-1 | Local deployment runs on Windows, macOS and Linux without requiring Docker/Podman. | M4 |
| NFR-2 | Versioned, additive public contracts with fixtures tested in every consuming language. | M2 |
| NFR-3 | No acknowledged state change lost on restart; crash behavior defined and tested. | M3 |
| NFR-4 | Observability sufficient to reconstruct who did what, when, with which inputs. | M3 |
| NFR-5 | Core tests use fakes; real-tool validation recorded separately. | M3 |
| NFR-6 | Retention, redaction of sensitive output, schema migrations of persisted data. | M3 |
| NFR-7 | Self-hosted install, upgrade, backup and restore documented and tested. | M7 |
| NFR-8 | Cloud tenant isolation, availability targets, horizontal scaling. | M9 |

## 9. Security, privacy and human control

- **SEC-1** The local Platform binds to loopback only and authenticates every request except health; the browser client is protected against cross-origin and DNS-rebinding access (one-time launch code exchanged for a session cookie, Host and Origin checks; [ADR-0034](../architecture/adr/0034-local-web-client-authentication-with-launch-code-and-cookie.md)). *(the prototype: bearer token, 0600 discovery file, Origin allowlist.)*
- **SEC-2** Secrets reach Providers only through a secrets port; they never appear in events, logs, artifacts or UI notices. *(the prototype)*
- **SEC-3** Project files, Skills and tool output are untrusted input (prompt injection). Nothing project-supplied executes implicitly. *(the prototype)*
- **SEC-4** No auto-approved agent permissions. Write access, network egress and other consequential actions require Policy grants and, where the Policy says so, an Approval. *(Partly prototype; general Policy is new.)*
- **SEC-5** Child processes get an explicit argv, cwd and environment allowlist; no shell invocation. *(the prototype)*
- **SEC-6** Every Approval and Policy change is attributable to an identity and recorded as an Event.
- **SEC-7** Remote Execution Environments and plugins are authenticated and limited to what they declared and were granted.
- **PRIV-1** Local deployments send no data anywhere unless the user enables a remote Provider or Integration; enabled egress is visible and revocable.
- **PRIV-2** Users can export and delete their Organization's data from self-hosted and Cloud deployments.

Details: [security model](../security/SECURITY_MODEL.md).

## 10. Scope

**In scope for the Local MVP (M4)**: single-user local deployment on three OSes; Projects, linked repositories, Work Packages, Work Items; graph Workflows with sequential execution and timeouts; Executions with durable state and Events; Agents, Skills, Capabilities, Providers (built in and external); worktree isolation; Local Checks; agent review; Approval Step; capability Policy; draft pull request delivery through a supervised delivery loop; Artifacts, Evidence, per-step cost; Amaru Web served by the Platform; versioned contracts with shared fixtures and a generated TypeScript SDK.

**Planned after the MVP** (milestones in the [roadmap](../architecture/ROADMAP.md)): remote Execution Environments and execution hardening (M5); plugin platform and TypeScript SDK (M6); self-hosted deployments with identity and organizations (M7); Desktop (M8); Cloud (M9); plugin registry (M10).

**Out of scope until a decision says otherwise**: billing; mobile clients; autonomous agents that create Agents, Skills or Workflows without a human; automatic merging; in-process third-party plugins; a requirement for Docker in local installs; real-time collaborative editing.

**Not the product**: replacing Git, GitHub, IDEs, CI, issue trackers or engines; reorganizing user files; hiding agent actions from the user.

The complete in/out list with IDs: [feature catalog §5](FEATURE_CATALOG.md#5-out-of-scope).

## 11. Phases

The phases are the milestones of the [master roadmap](../architecture/ROADMAP.md); there are no dates. Summary:

| Milestone | Outcome |
|---|---|
| M0 Foundation reconciled | Done 2026-10-25: documents consistent; every question closed (Q1–Q54); ADRs accepted. |
| M1 Decisions and spikes | Decisions were recorded on 2026-10-25 (ADRs 0014–0043); M1 validates them with spikes (bundled PostgreSQL, embedded NATS, crash recovery on three OSes) and adds the license text. |
| M2 Contracts v0 | Public API, events, provider and Worker protocol schemas with fixtures and a generated TypeScript SDK. |
| M3 Platform core | prototype Runtime core adapted into Amaru with Policy, Approval, PostgreSQL storage and the NATS bus. |
| M4 Local MVP | The end-to-end supervised delivery flow in Amaru Web on three OSes. |
| M5 Remote execution | Separate Workers, leases, remote environments, retries and parallel branches. |
| M6 Plugin platform | Manifest, permissions, TypeScript SDK, UI and logic extension hosts. |
| M7 Self-hosted | Identity, Organizations, external PostgreSQL/NATS/S3 configuration, packaging, upgrade and backup. |
| M8 Desktop | Tauri Desktop wrapping the shared UI, instance management, local supervision. |
| M9 Cloud | Row-level tenancy per Organization, managed and customer-run Workers, NATS cluster and infrastructure at scale. |
| M10 Ecosystem | Plugin registry, signing and distribution. |

## 12. Success criteria and validation

| Criterion | Validation method |
|---|---|
| A new Provider or Capability is added without changing core code, contract shape or UI architecture. | A Provider added in a separate change; diff review shows no core/contract/UI edits. *(prototype passed this with Tripo.)* |
| No consequential action happens without a recorded Approval or Grant when Policy requires one. | Automated tests on Policy enforcement + a recorded end-to-end run attempting the action without approval. |
| A Platform restart during a Workflow never loses or duplicates work. | Fault-injection tests (kill during each Step state) + a recorded manual run. |
| Any Artifact can be traced to its Execution, inputs, context, Provider, checks, cost and approver. | Audit walkthrough on a recorded run. |
| The same contracts serve every client. | Contract fixtures validated in Go and TypeScript tests; no client-specific endpoints. |
| Local deployment works on Windows, macOS and Linux. | Automated tests on all three + recorded manual validation per OS. |
| Accepted work is efficient. | At most one implementation and one review run per accepted change in the recorded MVP validation runs, with cost recorded *(prototype Cycle 7 acceptance target)*. |

## 13. Assumptions, risks, open questions

**Assumptions**
- The prototype Runtime's domain model, Capability→Provider boundary and recovery semantics are a sound starting point for the Platform core; this is checked component by component during migration, not assumed for every detail.
- Single-user local use is valuable on its own before Cloud or team features.
- Users accept running a local Platform process (with its bundled PostgreSQL) and opening its UI in a browser for the MVP.
- Validation checks for these and other assumptions: [validation register](../architecture/OPEN_QUESTIONS.md#validation-register).
- The earlier prototype's React/shadcn dashboard is a usable base for Amaru Web once rewired to the generated SDK.

**Risks**: maintained in the [roadmap risk register](../architecture/ROADMAP.md#risk-register).

**Open questions**: none; Q1–Q54 are closed ([OPEN_QUESTIONS](../architecture/OPEN_QUESTIONS.md) records how each was closed). Product-level resolutions:
- External trackers beyond GitHub (FR-3): connected through Integration Plugins; inbound items create Work Items linked to the external issue (Work Items stay Amaru-owned); once Signals ship (FEAT-WRK-006, P2) inbound items land as Signals for triage instead; outbound links point to the tracker. GitHub is first-party; other trackers follow as Plugins.
- Knowledge scope: committed project documentation indexed lexically ([ADR-0030](../architecture/adr/0030-knowledge-is-lexically-indexed-project-documentation.md)).
- Obsidian plugin: retired ([ADR-0031](../architecture/adr/0031-obsidian-plugin-is-retired.md)).
