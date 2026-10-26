# Glossary

The single source of Amaru terminology. Other documents use these terms with these meanings. When a term changes, change it here first.

**Status column**
- *Exists in the prototype*: implemented in an earlier internal Runtime that is not part of this repository. Nothing in this repository implements it yet.
- *Exists in earlier prototype*: implemented in a second, earlier internal prototype that is not part of this repository.
- *New*: an Amaru concept with no implementation anywhere.

## 1. Terms

| Term | Meaning | Status |
|---|---|---|
| **Organization** | The top-level owner of Projects, users, Policies and billing in multi-user deployments. A local single-user instance has one implicit Organization. | New (earlier prototype calls it *Workspace*) |
| **Instance** | One running Amaru Platform deployment (a local install, a self-hosted server, or a Cloud tenant endpoint) that a client connects to. | New |
| **Project** | An existing engineering codebase or workspace that Amaru coordinates. Its files stay authoritative and are never reorganized; Amaru metadata lives apart from them. | Exists in the prototype (an Obsidian vault with a per-project definition folder) |
| **Linked repository** | A Git repository a Project declares as a delivery target, possibly outside the Project folder. | Exists in the prototype (ADR-026 `repositories[]`) |
| **Work Package** | A group of related Work Items with a goal (an epic, a milestone, a feature). It may carry a delivery Policy (e.g. automatic delivery loop, local checks). | Exists in the prototype |
| **Work Item** | A unit of intended work with a status set by a human. It may depend on other Work Items and links to the Executions that worked on it. | Exists in the prototype |
| **Signal** | An observation (issue, alert, idea) not yet accepted as work; triaged into a Work Item, rejected or deferred. | Exists in earlier prototype; Amaru: P2 ([feature catalog](FEATURE_CATALOG.md)) |
| **Workflow** | A versioned definition of multi-step work: a graph of Steps with ordering and data edges, plus variables. | Exists in the prototype (YAML DAG, sequential execution) |
| **Workflow template** | A reusable Workflow starting point instantiated into a Project. Replaces the prototype's earlier use of "Playbook" for this. | Exists in the prototype (ADR-018) |
| **Step** | One node of a Workflow: trigger, agent, tool, system, review or Approval step. An Approval Step and a Policy-required approval both wait in the Step state `awaiting_approval`. | Exists in the prototype |
| **Trigger** | What starts an Execution: a human action, an inbound webhook, a schedule, or the delivery loop. Every Trigger goes through Policy. | Partly prototype (manual, webhook) |
| **Execution** | One run of a Workflow or a single Step, with a lifecycle (`queued → running ⇄ waiting → succeeded/failed/cancelled`; `waiting` while a Step awaits an Approval or a Worker), an ordered Event stream and recorded Evidence. The earlier prototype calls a workflow-level run *WorkflowRun*; Amaru uses **Execution**. | Exists in the prototype |
| **Step Execution** | The record of one Step inside an Execution: state, resolved Agent snapshot, inputs, outputs, verdict, Approval state, usage. The unit Workers run. Amaru does not use *Task* for this. | Exists in the prototype (`domain/step.go`) |
| **Event** | An immutable, ordered record of something that happened in an Execution or to platform state. The basis of observability and audit. | Exists in the prototype (per-execution `events.jsonl`) |
| **Artifact** | An output or input of work that the platform tracks: a file in the Project, a generated asset, a diff, a report. Identified by path and content hash. | Exists in the prototype (index of typed notes + generated files) |
| **Evidence** | What an Execution records to justify its result: assembled context, Skill/Agent snapshots, outputs and hashes, diffs, logs, Local Check and Verification results, usage and cost. | Exists in the prototype (partial) |
| **Local Check** | A human-configured command (no shell) run against a specific commit in an isolated worktree; its result is bound to that commit and the check configuration. Complements, never replaces, hosted CI. | Exists in the prototype (ADR-029) |
| **Verification Run** | A Local Check or test command executed by the Platform against an Artifact in a sandboxed environment, producing pass/fail Evidence. | Exists in earlier prototype (`domain/verification_run.go`) |
| **Delivery loop** | An opt-in, per-Work-Package reconciler that drives Work Items through implement → check → review → pull request, bounded by attempt and repair budgets, with humans approving and merging. | Exists in the prototype (*Autoloop*, ADR-021) |
| **Agent** | A reusable definition of an AI worker: instructions, model preference, provider preference, referenced Skills, requested Capabilities, tool scope. Not a running process. | Exists in the prototype |
| **Team** | A named set of Agents and humans with roles, used for review and separation of duties. Organization-scoped Platform state; definitions reference it by slug ([ADR-0043](../architecture/adr/0043-project-definitions-in-git-teams-and-operational-state-in-platform.md)). | Exists in the prototype (ADR-019) |
| **Skill** | User-owned procedural knowledge (Markdown) that an Agent may reference, with explicit trust and a content hash. | Exists in the prototype |
| **Playbook** | A read-only knowledge note describing a procedure for humans and Agents. Not executable. | Exists in the prototype (ADR-018) |
| **Knowledge** | Human-readable project content Amaru may index and use as context (documentation, decisions, Skills, Playbooks, Artifacts notes) that stays in the Project and under Git. Amaru reads it; it never owns it. Indexing and retrieval: committed Markdown, lexical search ([ADR-0030](../architecture/adr/0030-knowledge-is-lexically-indexed-project-documentation.md)). | Partly prototype (ADR-022) |
| **Capability** | A named, typed unit of work the platform can perform (`agent.run`, `code.test`, `doc.convert` …), independent of which tool performs it. | Exists in the prototype (catalog of typed specs) |
| **Provider** | An implementation of one or more Capabilities for a concrete tool or service (Claude CLI, Blender, a remote API). Built in, or an external executable speaking a provider protocol. | Exists in the prototype |
| **Adapter** | The code inside a Provider that translates between the neutral Capability contract and a concrete tool's I/O. | Exists in the prototype |
| **Execution Environment** | A machine or sandbox where Steps actually run, with its own tools, resources, operating system and trust level (the user's laptop, a Windows/GPU workstation, a container sandbox, a Cloud worker). | New |
| **Environment template** | An operator-defined recipe for starting an Execution Environment on demand: base image or VM, installed tools (e.g. Unreal Engine 5 + MCP plugin), Capabilities, labels, cost class, idle timeout. | New |
| **Worker** | A process inside an Execution Environment that accepts Step assignments from the Platform and runs them through Providers. Holds no authoritative domain state. | New (the prototype's Runtime is a single local worker + coordinator in one process) |
| **Enrollment token** | A one-time, short-lived token an Organization admin issues to register a Worker; the Worker exchanges it for a key-based identity ([ADR-0024](../architecture/adr/0024-remote-worker-registration-identity-and-delivery.md)). | New |
| **Outbox** | The PostgreSQL table into which every state change writes the messages it causes, in the same transaction; a relay publishes them to the message bus ([ADR-0042](../architecture/adr/0042-durable-execution-with-explicit-restart-semantics.md), [ADR-0016](../architecture/adr/0016-nats-jetstream-message-bus-embedded-in-compact-installs.md)). | New |
| **Usage record** | Tokens, cost, credits and duration of one Step Execution, each value marked reported, estimated or unknown ([ADR-0033](../architecture/adr/0033-usage-and-cost-telemetry-model.md)). | New (prototype `Usage` has credits and duration only) |
| **Lease** | A time-limited claim by a Worker on a Step assignment, renewed by heartbeat (10 s heartbeat, 30 s Lease by default); one Lease is one Step attempt. When it expires the Step is retried as a new Lease if its Capability is idempotent, otherwise it ends `interrupted` ([ADR-0042](../architecture/adr/0042-durable-execution-with-explicit-restart-semantics.md)). | New |
| **Worktree** | An isolated Git working copy the Platform creates for one Work Item or Step. Not to be confused with the earlier prototype's tenant *Workspace*. | Exists in the prototype (`provider/gitrepo`) |
| **Approval** | An explicit human decision required before a consequential action proceeds, recorded with who, what, when and on what Evidence. | Exists in the prototype as approval steps (ADR-019); evidence binding exists in earlier prototype |
| **Grant** | A standing, recorded authorization (by a human) for an Agent, Workflow or Plugin to use a Capability or resource. | Exists in the prototype (agent grants, ADR-024) |
| **Policy** | Rules that decide which Capabilities an Agent, Workflow or Plugin may use, where, and which actions need an Approval. Grants are its standing decisions. | New (prototype has grants and trust but no general Policy, GAP-09) |
| **Plugin** | A packaged extension of Amaru with a manifest, declared permissions and one or more Plugin Components. | New |
| **Plugin Component** | One deployable part of a Plugin with a declared target: UI extension, sandboxed TypeScript logic, service, Worker adapter or local adapter ([extensibility](../architecture/EXTENSIBILITY.md)). | New |
| **Extension host** | The isolated runtime that loads Plugin Components of one target and mediates their access to the public API: sandboxed frames for `ui`, the `amaru plugin-host` process for `logic` ([ADR-0040](../architecture/adr/0040-plugin-logic-runs-in-a-separate-plugin-host-process-built-with-javy.md)). | New |
| **Integration** | A connection to an external system Amaru coordinates but does not replace (Git hosting, issue trackers, CI, IDEs). Often delivered as a Plugin. | Partly prototype (GitHub via `gh`) |
| **Amaru Platform** | The deployable system that owns domain state, contracts, policies and execution coordination. | New (proposed) |
| **Amaru Cloud** | A managed deployment of the Platform operated by the Amaru vendor. | New (proposed, M9; [ADR-0022](../architecture/adr/0022-cloud-tenancy-row-level-isolation-by-organization.md)) |
| **Amaru Self-hosted** | A user-operated deployment of the same Platform. | New (proposed, M7; [ADR-0023](../architecture/adr/0023-self-hosted-oci-image-atlas-migrations-operator-owned-backup.md)) |
| **Amaru Desktop** | A native desktop client of the Platform. Not the owner of execution state. | New (proposed, M8; [ADR-0018](../architecture/adr/0018-desktop-shell-is-tauri-v2.md)) |
| **Amaru Web** | The browser client of the Platform; the first-party client of the Local MVP ([ADR-0010](../architecture/adr/0010-local-mvp-client-is-a-web-ui.md)). | New (proposed) |
| **Local MVP** | The first Amaru product milestone: one user, a local Platform with an embedded Worker, the Web client, on Windows, macOS and Linux ([ROADMAP M4](../architecture/ROADMAP.md#m4-local-mvp)). | New |

## 2. Term mapping from predecessor repositories

Two earlier internal codebases use overlapping names. **"Prototype"** in Amaru documents means the first, Runtime-based one unless *earlier prototype* is written.

| Prototype Runtime | Earlier prototype | Amaru term | Note |
|---|---|---|---|
| Runtime (per-user daemon) | Backend service | Platform + local Worker | The prototype combines coordination and execution in one per-user daemon. Amaru separates the roles ([ADR-0009](../architecture/adr/0009-modular-monolith-platform-with-separate-worker-role.md)). |
| Obsidian plugin | Dashboard (React) | Client | The Obsidian plugin is retired with no Amaru successor ([ADR-0031](../architecture/adr/0031-obsidian-plugin-is-retired.md)); the earlier prototype's dashboard is the starting point for Amaru Web. |
| Vault and per-project definition folder | Project (Postgres row) | Project / project metadata location | Definitions are committed under `.amaru/`; operational state lives in the Platform ([ADR-0043](../architecture/adr/0043-project-definitions-in-git-teams-and-operational-state-in-platform.md)). |
| — | Workspace | Organization | "Workspace" is ambiguous (a tenant in the prototype, a Git working copy elsewhere); Amaru does not use it as a domain term. |
| Workspace (agent worktree) | Workspace directory (container) | Worktree | |
| Execution (parent) | WorkflowRun | Execution | |
| Step Execution | Execution / Task | Step Execution | The prototype's per-agent *Execution* corresponds to an Amaru Step Execution. *Task* is not an Amaru term. |
| Autoloop | — | Delivery loop | |
| Local checks | VerificationRun | Local Check / Verification Run | Same intent; Amaru keeps both terms for commit-bound vs artifact-bound checks. |
| Playbook (before its rename) | — | Workflow template | After the rename the prototype uses Playbook for a knowledge note, as Amaru does. |
| External provider protocol | Driver | Provider protocol / Provider | Amaru's external Provider protocol is `amaru.provider/v1` ([ADR-0025](../architecture/adr/0025-api-and-protocol-versioning-policy.md) naming), adapted from the prototype's protocol in M2. There is no compatibility shim: prototype Providers are ported, not run unchanged. |
