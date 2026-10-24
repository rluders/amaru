# Amaru — Product Requirements

Status: **draft for review** (2026-10-24). Nothing described here is implemented in this repository. Where behavior already exists in the predecessor project, it is marked *(Weave)*; see the [Weave inventory](../migration/weave-inventory.md). Terms are defined in the [glossary](GLOSSARY.md).

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

## 3. Target users

| User | Needs | Phase |
|---|---|---|
| **Individual engineer** working locally with AI agents and tools | Delegate work with clear boundaries, keep evidence, recover from failures, switch agent vendors without rewriting workflows. | MVP |
| **Technical lead of a small team** | Shared workflows and Skills, review and approval of agent work, visibility of what ran and why. | Later |
| **Self-host operator** (platform/infra engineer) | Install, upgrade, back up and secure an Amaru deployment inside their own network. | Later |
| **Plugin / integration author** | Stable contracts and an SDK to add Capabilities, Providers, integrations and UI. | Later |

### Representative use cases

1. **Delegated change with approval.** An engineer defines a Work Item, runs a Workflow where an agent investigates and proposes a change, local checks validate it, and the engineer approves before anything is pushed. *(Weave: workflow + agent + local checks exist; Approval step does not.)*
2. **Recoverable multi-step run.** A long Workflow survives a Runtime or machine restart and resumes or fails explicitly, never silently. *(Weave: restart semantics and resumable remote jobs exist.)*
3. **Route work to a specialized machine.** A Step needing a Windows/GPU tool runs on a registered workstation while the engineer works on a laptop. *(New.)*
4. **Vendor-neutral agent work.** The same Workflow runs with different agent providers chosen per Agent. *(Weave: Claude, Codex, OpenCode behind `agent.run`.)*
5. **Audit a past result.** Months later, someone sees exactly which context, Skills, provider, inputs and outputs produced an Artifact, and who approved it. *(Weave: context/Skill snapshots and hashes exist; approver identity does not.)*

## 4. Product principles

1. **Human authority.** Consequential actions require an explicit, recorded human decision ([ADR-0006](../architecture/adr/0006-human-authorization-for-consequential-actions.md)). No auto-approved agent permissions.
2. **Evidence over claims.** Results carry the evidence that produced them. The product never presents planned or simulated behavior as real.
3. **Orchestrate, don't replace.** The user's repositories and tools stay authoritative. Amaru never moves or takes ownership of user files.
4. **Vendor and tool neutrality.** Workflows reference Capabilities, not vendors (Capability → Provider → Adapter, proven in Weave).
5. **One platform, many deployments.** Cloud and self-hosted share one domain model and one set of contracts ([ADR-0002](../architecture/adr/0002-one-platform-multiple-deployment-models.md), proposed).
6. **Clients are clients.** Desktop, Web and editor plugins present and command; they do not own execution state ([ADR-0003](../architecture/adr/0003-desktop-is-a-client.md), proposed).
7. **Incremental by evidence.** Infrastructure (queues, databases, durable engines, distributed workers) is introduced only when a measured need and a recorded decision justify it.
8. **Secure by default.** Least privilege, explicit trust for project-supplied content, no secrets in logs or events.

## 5. Core domain concepts

Project, Work Package, Work Item, Workflow, Step, Execution, Event, Artifact, Evidence, Agent, Skill, Capability, Provider, Execution Environment, Worker, Approval, Policy, Plugin. Definitions and implementation status: [glossary](GLOSSARY.md). Relationships: [architecture § domain model](../architecture/ARCHITECTURE.md#7-domain-model).

## 6. Product surfaces and deployment models

| Surface | Role | Status |
|---|---|---|
| **Amaru Platform** | Owns domain state, contracts, policies and execution coordination. | Proposed. Weave's Go Runtime is the closest existing implementation. |
| **Workers / Execution Environments** | Run Steps through Providers on a given machine. | Proposed. Weave runs everything on the local machine. |
| **Amaru Self-hosted** | The Platform operated by the user or their organization. | Direction only. |
| **Amaru Cloud** | The Platform operated as a managed service. | Direction only. |
| **Amaru Desktop** | Native client; Tauri v2 + Rust host under evaluation ([open question](../architecture/OPEN_QUESTIONS.md#q5-desktop-framework)). | Direction only. |
| **Amaru Web** | Browser client sharing contracts, SDK and UI components with Desktop. | Direction only. |
| **Plugins** | UI extensions, integrations, Capabilities/Providers, environment adapters. | Direction only. Weave has an external provider protocol. |
| **Editor integrations** (e.g. the Obsidian plugin) | Clients over the same contracts. | Exists in Weave for Obsidian. |

A local, single-user deployment (Platform + local Worker on the engineer's machine) is the first target; it is also how Weave works today.

## 7. Functional requirements

IDs are stable. **MVP** = required for the first Amaru release. **Future** = direction, not committed.

### 7.1 Projects and work
| ID | Requirement | Phase |
|---|---|---|
| FR-1 | Attach an existing Project without moving, rewriting or taking ownership of its files; Amaru metadata is stored separately. | MVP *(Weave)* |
| FR-2 | Create and track Work Packages and Work Items with human-set status and links to Executions. | MVP *(Weave)* |
| FR-3 | Link Work Items to external trackers (GitHub issues, etc.) without duplicating them as the source of truth. | Future |

### 7.2 Workflows and execution
| ID | Requirement | Phase |
|---|---|---|
| FR-10 | Define Workflows as versioned graphs of Steps with ordering and data edges, stored in a reviewable text format. | MVP *(Weave: YAML DAG)* |
| FR-11 | Run a Workflow or a single Step as an Execution with a defined lifecycle, cancellation and an ordered, replayable Event stream. | MVP *(Weave)* |
| FR-12 | Persist Execution state so a Platform restart never silently loses or duplicates work; interrupted Steps end in an explicit state or resume when the Provider supports it. | MVP *(Weave: restart semantics, Resumable/Detachable remote jobs)* |
| FR-13 | **Approval Step**: pause an Execution until an authorized human approves or rejects, recording the decision and the evidence shown. | MVP *(New)* |
| FR-14 | Retries, branching, parallel branches, compensation. | Future |
| FR-15 | Reusable Workflow templates (Playbooks) instantiated into Projects. | Future *(Weave: first slice, proposed)* |

### 7.3 Agents, Skills, Capabilities, Providers
| ID | Requirement | Phase |
|---|---|---|
| FR-20 | Define Agents with instructions, model/provider preference, Skill references and requested Capabilities. | MVP *(Weave)* |
| FR-21 | Skills are user-owned, explicitly trusted, optionally hash-pinned; context assembly is deterministic and recorded per Execution. | MVP *(Weave)* |
| FR-22 | Capabilities have typed specs; Workflows reference Capabilities, never vendors. Adding a Provider changes no core code, contract shape or UI architecture. | MVP *(Weave)* |
| FR-23 | Third parties add Providers without rebuilding the Platform (external provider protocol). | MVP *(Weave: `weave.provider/v1`)* |
| FR-24 | **Capability policy**: an Agent or Workflow can use only Capabilities its Policy grants; ungranted use is refused before start. | MVP *(New; Weave GAP-09)* |

### 7.4 Artifacts and evidence
| ID | Requirement | Phase |
|---|---|---|
| FR-30 | Record outputs as Artifacts with project-relative path, role and content hash computed by the Platform. | MVP *(Weave)* |
| FR-31 | Each Execution keeps its Evidence: assembled context, Agent/Skill snapshots, inputs, outputs, logs and validation results. | MVP *(Weave, partial)* |
| FR-32 | Large Artifacts in object storage with retention policies. | Future |

### 7.5 Clients, plugins, environments
| ID | Requirement | Phase |
|---|---|---|
| FR-40 | All clients use the same versioned public contracts; no client-only backdoors. | MVP |
| FR-41 | At least one first-party client for the MVP. Which one (existing Obsidian plugin, Desktop, Web) is an open decision ([ROADMAP stage 5](../architecture/ROADMAP.md#stage-5--desktop-client)). | MVP |
| FR-42 | Plugins declare their contributions and permissions; the Platform enforces them. | Future |
| FR-43 | Register remote Execution Environments (e.g. a Windows/GPU workstation) and route Steps to them by required Capability. | Future |
| FR-44 | Multi-user projects, roles, separation of duties (an actor cannot approve its own work). | Future *(Weave: ADR-019 proposed)* |

## 8. Non-functional requirements

| ID | Requirement | Phase |
|---|---|---|
| NFR-1 | Runs on Windows, macOS and Linux for the local deployment without requiring Docker/Podman. | MVP *(Weave: Linux validated; Windows/macOS unvalidated)* |
| NFR-2 | Public contracts are versioned; changes within a major version are additive; clients ignore unknown fields. Contract fixtures are tested in every language that consumes them. | MVP *(Weave)* |
| NFR-3 | Durability: no acknowledged state change is lost on a clean restart; behavior after a crash is defined and tested. | MVP |
| NFR-4 | Observability: structured logs, Events per Execution, and enough metadata to reconstruct who did what, when, with which inputs. | MVP |
| NFR-5 | Tests of core behavior must not need real agents or tools installed (fakes), and real-tool validation is recorded separately. | MVP *(Weave)* |
| NFR-6 | History retention, redaction of sensitive provider output, schema migrations of persisted data. | MVP *(Weave GAP-07 open)* |
| NFR-7 | Self-hosted installation, upgrade, backup and restore are documented and tested. | Future |
| NFR-8 | Cloud tenant isolation, availability targets, horizontal scaling. | Future |

## 9. Security, privacy and human control

- **SEC-1** The local Platform binds to loopback only and authenticates every request except health. *(Weave: bearer token, 0600 discovery file.)*
- **SEC-2** Secrets reach Providers only through a secrets port; they never appear in events, logs, artifacts or UI notices. *(Weave)*
- **SEC-3** Project files, Skills and tool output are untrusted input (prompt injection). Nothing project-supplied executes implicitly. *(Weave)*
- **SEC-4** No auto-approved agent permissions. Write access, network egress and other consequential actions require Policy grants and, where the Policy says so, an Approval. *(Partly Weave; Policy and Approval are new.)*
- **SEC-5** Child processes get an explicit argv, cwd and environment allowlist; no shell invocation. *(Weave)*
- **SEC-6** Every Approval and Policy change is attributable to an identity and recorded as an Event.
- **SEC-7** Remote Execution Environments and plugins are authenticated and limited to what they declared and were granted. *(Future.)*
- **PRIV-1** Local deployments send no data anywhere unless the user enables a remote Provider or Integration; enabled egress is visible and revocable.

Details: [security model](../security/SECURITY_MODEL.md).

## 10. Scope

**In scope for the MVP**: single-user local deployment; Projects, Work Packages, Work Items; graph Workflows with sequential execution; Executions with durable state and Events; Agents, Skills, Capabilities, Providers (built in and external); Artifacts and Evidence; the Approval Step; capability Policy; one first-party client; versioned contracts with shared fixtures.

**Out of scope for the MVP** (may come later): Cloud offering; multi-user and tenancy; remote Execution Environments; plugin marketplace, signing and distribution; parallel branches and retries; durable workflow engines, message brokers or distributed databases; billing; mobile; autonomous agents that create Agents, Skills or Workflows without a human.

**Not the product**: replacing Git, GitHub, IDEs, CI, issue trackers or engines; reorganizing user files; hiding agent actions from the user.

## 11. Phases

Phases follow the [architecture roadmap](../architecture/ROADMAP.md); there are no dates.

1. **Foundation** (this repository today): scope, architecture, governance, decision log.
2. **Weave completion**: finish outstanding Weave cycles; that work continues in `weave-obsidian`.
3. **Architecture study**: answer the blocking [open questions](../architecture/OPEN_QUESTIONS.md) with evidence.
4. **Contracts**: stable Platform API, event and provider contracts with fixtures.
5. **MVP**: local Platform + Worker + one client, meeting the MVP requirements above; the migration of selected Weave code happens here only after its [readiness criteria](../migration/MIGRATION_PLAN.md#5-readiness-criteria) are met and approved.
6. **Extensibility, Desktop, Self-hosted, Cloud**: in that order unless evidence says otherwise.

## 12. Success criteria and validation

| Criterion | Validation method |
|---|---|
| A new Provider or Capability is added without changing core code, contract shape or UI architecture. | A Provider added in a separate change; diff review shows no core/contract/UI edits. *(Weave passed this with Tripo.)* |
| No consequential action happens without a recorded Approval when Policy requires one. | Automated tests on Policy enforcement + a recorded end-to-end run attempting the action without approval. |
| A Platform restart during a Workflow never loses or duplicates work. | Fault-injection tests (kill during each Step state) + a recorded manual run. |
| Any Artifact can be traced to its Execution, inputs, context, Provider and approver. | Audit walkthrough on a recorded run. |
| The same contracts serve every client. | Contract fixtures validated in Go and TypeScript tests; no client-specific endpoints. |
| Local deployment works on Windows, macOS and Linux. | CI on all three + recorded manual validation per OS. |

## 13. Assumptions, risks, open questions

**Assumptions**
- The Weave Runtime's domain model and Capability→Provider boundary are a sound starting point; this is checked during the migration assessment, not assumed for every detail.
- Single-user local use is valuable on its own before Cloud or team features.
- Users accept running a local daemon (Weave's model).

**Risks**
- *Scope creep toward the full vision* before the local MVP is solid. Mitigation: phases with exit criteria; out-of-scope list above.
- *Premature infrastructure* (durable engines, brokers) adding operational weight. Mitigation: [ADR-0008](../architecture/adr/0008-incremental-monorepo-layout.md) (proposed) and evidence gates.
- *Two codebases diverging* while Weave keeps evolving. Mitigation: [migration plan](../migration/MIGRATION_PLAN.md) with an inventory snapshot and explicit readiness review.
- *Licensing not decided*, blocking external contributions and distribution. See [Q12](../architecture/OPEN_QUESTIONS.md#q12-licensing-and-commercial-boundaries).
- *Agent-vendor CLIs change* behavior under the Providers. Mitigation: Provider tests with captured outputs; real-tool validation recorded.

**Open questions**: [OPEN_QUESTIONS.md](../architecture/OPEN_QUESTIONS.md). Product-specific ones: which client ships first (FR-41); how Work Items relate to external trackers (FR-3); what "Knowledge" means beyond Skills and project files (carried over from Weave ROADMAP Q9).
