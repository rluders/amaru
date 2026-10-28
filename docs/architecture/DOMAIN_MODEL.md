# Conceptual Domain Model

Status: **accepted** (2026-10-25). Conceptual, not a database schema. Labels follow [ARCHITECTURE](ARCHITECTURE.md): **[Prototype]** exists in an earlier internal prototype, **[Proposed]** is new. Terms: [glossary](../product/GLOSSARY.md).

## 1. Principles

1. **Definitions vs runs.** Definitions (Workflow, Agent, Skill, Policy) are versioned; runs (Execution, Step Execution) reference the exact version and keep snapshots. A later edit never rewrites history **[Prototype]**.
2. **Humans own intent; the Platform owns execution state.** Work Item status is human-set; Execution state is Platform-set **[Prototype ADR-004]**.
3. **Knowledge stays in the Project.** Domain objects reference project files by path and hash; they do not copy or own them.
4. **Capabilities, not vendors.** Steps invoke Capabilities; Providers are resolved at run time and recorded.
5. **Not every concept is a persistent entity.** Evidence is a view over Events, Artifacts and snapshots; a Lease is operational state of the coordinator.

## 2. Conceptual diagram

```mermaid
erDiagram
    ORGANIZATION ||--o{ PROJECT : owns
    ORGANIZATION ||--o{ USER_MEMBERSHIP : has
    ORGANIZATION ||--o{ POLICY : defines
    ORGANIZATION ||--o{ EXECUTION_ENVIRONMENT : registers
    ORGANIZATION ||--o{ PLUGIN_INSTALLATION : enables
    PROJECT ||--o{ LINKED_REPOSITORY : declares
    PROJECT ||--o{ WORK_PACKAGE : groups
    WORK_PACKAGE ||--o{ WORK_ITEM : contains
    WORK_ITEM }o--o{ WORK_ITEM : "depends on"
    PROJECT ||--o{ WORKFLOW : defines
    WORKFLOW ||--|{ WORKFLOW_VERSION : "is versioned as"
    PROJECT ||--o{ AGENT : defines
    AGENT }o--o{ SKILL : references
    WORK_ITEM ||--o{ EXECUTION : "worked on by"
    WORKFLOW_VERSION ||--o{ EXECUTION : "run as"
    EXECUTION ||--|{ STEP_EXECUTION : contains
    EXECUTION ||--|{ EVENT : emits
    STEP_EXECUTION }o--|| CAPABILITY : invokes
    CAPABILITY ||--o{ PROVIDER : "implemented by"
    STEP_EXECUTION }o--o| AGENT_SNAPSHOT : uses
    STEP_EXECUTION }o--o| WORKER : "leased to"
    WORKER }o--|| EXECUTION_ENVIRONMENT : "runs in"
    STEP_EXECUTION ||--o{ ARTIFACT : produces
    STEP_EXECUTION ||--o{ CHECK_RESULT : records
    STEP_EXECUTION ||--o| APPROVAL : "gated by"
    POLICY ||--o{ GRANT : contains
    GRANT }o--|| CAPABILITY : authorizes
    APPROVAL }o--|| USER_MEMBERSHIP : "decided by"
    PLUGIN_INSTALLATION ||--|{ PLUGIN_COMPONENT : contains
    PLUGIN_COMPONENT }o--o{ CAPABILITY : contributes
    PLUGIN_COMPONENT }o--o{ PROVIDER : contributes
```

## 3. Entities and responsibilities

| Entity | Responsibility | Scope / owner | Persistence | Status |
|---|---|---|---|---|
| Organization | Owns users, Projects, Policies, Environments, plugins. One implicit Organization in a local Instance. | Instance | Platform store | [Prototype: *Workspace*]; [Proposed] |
| User membership | A user's role in an Organization (Owner, Admin, Maintainer, Contributor, Viewer), optionally narrowed or raised per Project ([ADR-0038](adr/0038-roles-teams-and-separation-of-duties.md)). | Organization | Platform store | [Prototype: `workspace_members`] |
| Project | Identity, root/repository location, trust, settings. | Organization | Platform store + `.amaru/` definitions ([ADR-0043](adr/0043-project-definitions-in-git-teams-and-operational-state-in-platform.md)) | [Prototype] |
| Linked repository | Delivery target with base branch and delivery policy. | Project | Definition (Git-reviewable) | [Prototype ADR-026] |
| Work Package | Goal, status, delivery-loop and quality-gate policy. | Project | Platform store ([ADR-0043](adr/0043-project-definitions-in-git-teams-and-operational-state-in-platform.md)) | [Prototype] |
| Work Item | Intent, human status, owner, dependencies, external issue link. Internal id is a ULID; the display key is `<PROJECTKEY>-<n>`, unique per Instance; references across Instances use the Instance URL + ULID. | Work Package | Platform store ([ADR-0043](adr/0043-project-definitions-in-git-teams-and-operational-state-in-platform.md)) | [Prototype] |
| Workflow / Workflow version | Graph of Steps, edges, variables; versions are the Git history of `.amaru/`; each Execution stores the commit SHA, content hash and an immutable snapshot of the version it ran. | Project | Definition (YAML) + snapshot in run history | [Prototype]; versioning [Proposed] |
| Agent / Agent snapshot | Instructions, model and provider preference, Skills, tool scope; the snapshot is what a Step used. | Organization or Project ([ADR-0039](adr/0039-configuration-scopes-and-resolution-order.md); the Organization library UI is FEAT-AGT-006, P2) | Definition (Project) or Platform store (Organization); snapshot in run history | [Prototype] |
| Skill | Trusted procedural knowledge with hash. | Organization or Project | Read from the Project (Project Skills) or the Platform store (Organization Skills) | [Prototype] |
| Team | Members (humans, Agents) with team roles; referenced by slug from definitions and gates. | Organization | Platform store ([ADR-0043](adr/0043-project-definitions-in-git-teams-and-operational-state-in-platform.md), [ADR-0038](adr/0038-roles-teams-and-separation-of-duties.md)) | [Prototype ADR-019] |
| Capability | Typed spec (inputs, outputs). | Catalog (built-in or contributed) | Catalog | [Prototype] |
| Provider | Implementation of Capabilities, with availability per Environment. | Environment / plugin | Registry | [Prototype] |
| Execution | A run of a Workflow version (or a single Step) for an optional Work Item; lifecycle; trigger; requesting identity. | Project | Run history | [Prototype] |
| Step Execution | State, resolved Agent, inputs, outputs, verdict, approval state, usage, external job reference, assigned Worker. | Execution | Run history | [Prototype] |
| Event | Immutable fact with a Platform-assigned per-Execution `seq` (Evidence is ordered by `seq`, never by Worker clocks) and a stream cursor. | Project / Execution | Event log | [Prototype] |
| Artifact | Path or object key, role, hash, size, producing Step. | Step Execution | Artifact store | [Prototype] |
| Check result | Local Check, CI or Verification Run outcome bound to a commit or Artifact. | Step Execution / Work Item | Run history | [Prototype ADR-029]; [Prototype: VerificationRun] |
| Approval | Request and decision, approver identity, Evidence references, expiry. | Step Execution | Run history | [Prototype: approval steps]; evidence binding [Prototype] |
| Policy / Grant | Rules and standing authorizations for Capabilities, egress, plugins. | Organization / Project | Platform store | [Prototype: grants ADR-024]; general Policy [Proposed] |
| Execution Environment | A machine or sandbox class with OS, tools, labels, trust level. | Organization | Platform store | [Proposed] |
| Worker | A connected process in an Environment with identity, manifest, health. | Execution Environment | Operational state | [Proposed] |
| Plugin installation / component | Installed package version, granted permissions, enabled components per target. | Organization (the implicit Organization in a local Instance; [ADR-0021](adr/0021-plugin-packaging-signing-permissions-and-compatibility.md)) | Platform store | [Proposed] |
| Delivery-loop ledger | Per Work Item: phase, attempts, runs, PR, CI, local checks, recoveries. | Work Package | Operational state | [Prototype `domain/autoloop.go`] |

**Not first-class entities:** *Evidence* (a query over Events, snapshots, Artifacts, check results, Approvals), *Lease* (coordinator state attached to a Step Execution), *Task* (not used; see glossary mapping), *Signal* (P2; would be its own entity when scheduled).

## 4. Key relationships and rules

- A Work Item belongs to exactly one Work Package; dependencies are within a Project (prototype limits them to one Package; Amaru allows any Work Package of the same Project, never across Projects; see [§7](#7-resolved-domain-questions)).
- An Execution runs exactly one Workflow version; resume after restart requires the same version **[Prototype `ResumeInterrupted`]**.
- A Step Execution has at most one active Lease and at most one Approval at a time.
- An Approval references the Evidence presented: commit SHA, diff Artifact, check results. If the subject changes (new commit) while the Approval is `requested`, it is voided and re-requested; a decided Approval is never edited and does not authorize the action when the evidence digest has changed ([ADR-0048](adr/0048-approval-reuse-uncovered-capabilities-and-policy-defaults.md)) **[Proposed]**.
- Grants authorize Capabilities for subjects (Agent, Workflow, Plugin component) in a scope (Organization, Project); Approvals authorize one occurrence (ADR-0006).
- Providers are resolved per Step from the Agent's preference, Policy and the Environments that advertise the Capability.

## 5. Lifecycles

### 5.1 Execution
```mermaid
stateDiagram-v2
    [*] --> queued
    queued --> running : first step starts
    running --> waiting : step awaits approval, worker, or a person to resume it
    waiting --> running : approval given / worker leased / step resumed
    running --> succeeded
    running --> failed
    waiting --> failed : approval rejected / expired
    queued --> cancelled
    running --> cancelled
    waiting --> cancelled
    succeeded --> [*]
    failed --> [*]
    cancelled --> [*]
```
The prototype has `queued, running, succeeded, failed, cancelled` with a pending approval keeping the step `running` (`domain/execution.go`). Amaru adds an explicit `waiting` state so clients and schedulers can distinguish idle waits **[Proposed]**.

### 5.2 Step Execution
```mermaid
stateDiagram-v2
    [*] --> pending
    pending --> skipped : trigger / condition
    pending --> refused : policy or confinement refusal
    pending --> awaiting_approval : policy requires approval
    pending --> queued : planned
    awaiting_approval --> queued : approved
    awaiting_approval --> failed : rejected / expired
    queued --> leased : worker accepts
    leased --> running
    running --> succeeded
    running --> failed
    running --> queued : restart or lease lost, idempotent
    running --> interrupted : restart or lease lost, not idempotent
    running --> timed_out : declared timeout exceeded
    interrupted --> queued : resumed by a human or a named Policy rule
    timed_out --> queued : resumed by a human or a named Policy rule
    interrupted --> cancelled : abandoned by a human or a named Policy rule
    timed_out --> cancelled : abandoned by a human or a named Policy rule
    leased --> queued : lease expired before start
    running --> cancelled
    queued --> cancelled
```
Prototype states: `queued, running, succeeded, failed, cancelled, skipped, unsupported` plus `runtime_restarted` as an error code (`domain/step.go`). `refused`, `leased`, `awaiting_approval` and `interrupted` are **[Proposed]**; `unsupported` becomes `refused` with a reason. A Worker lost mid-Step follows the restart rule of [ADR-0042](adr/0042-durable-execution-with-explicit-restart-semantics.md) (`interrupted` with reason `worker_lost` unless idempotent), so there is no separate `lost` state. `awaiting_approval` is the one waiting state for both an explicit Approval Step (Step type `approval`) and a Policy-required approval before any other Step starts; nothing runs while a Step is in it.

**Implementation status (2026-10-27):** the Execution and Step state machines of §5.1–5.2 are implemented and unit-tested in `services/platform/internal/domain` ([#118](https://github.com/rluders/amaru/issues/118)); only the transitions drawn above are possible. **Decided 2026-10-27 (product owner delegated the choice):** (a) `interrupted` and `timed_out` are not terminal. Only a human, or a Policy rule that names the Step, resumes (`→ queued`, reason `resumed`) or abandons (`→ cancelled`) them; nothing else can, and nothing re-runs them automatically. Neither spends an attempt (ADR-0044). The caller checks the Execution still runs the same Workflow version and records an Event with the actor. (b) An Execution with an `interrupted` or `timed_out` Step is `waiting` (§5.1), not `failed`, so it stays recoverable; abandoning the Step lets a person cancel or fail the Execution. `timed_out` is a state, as ADR-0044 §3 says; it is not a reason on `failed`. **Not built:** the Policy hook that names a Step for resume (ADR-0044 follow-up) and the timeout clock itself.

**Durable state and restart reconciliation (2026-10-27, [#129](https://github.com/rluders/amaru/issues/129)):** Execution and Step rows carry a `version` for optimistic concurrency; a Step also records `idempotent` (copied from its Capability when planned), an optional `remoteJob` (`id`, `resumable`) and the `lease` it holds (`owner`, `expiresAt`). `StepExecution.Reconcile` applies the [ADR-0042](adr/0042-durable-execution-with-explicit-restart-semantics.md) restart rule to one Step: `pending`, `awaiting_approval`, `queued`, `interrupted` and `timed_out` keep waiting; `leased` keeps waiting while its Lease is valid and goes back to `queued` (reason `lease_expired`) once it ended, whatever the Step's kind, because nothing ran; `running` with a resumable remote job goes to `queued` keeping the job (rule 2), an idempotent one goes to `queued` for a new Lease (rule 3), any other becomes `interrupted` (rule 4). A re-queued `running` Step carries the reason `platform_restarted`. When a Step is interrupted, a `running` Execution moves to `waiting`; it does not fail. The coordinator records one event per changed Step and one for the Execution, in the same transaction as the change. Not modelled yet: the Worker re-attaching to a resumable job, timers and Lease expiry sweeps, the outbox.

**Attempts and fencing (2026-10-27, [#145](https://github.com/rluders/amaru/issues/145)):** each Lease is one attempt. A Step records the latest `attemptId` and a `fenceToken` that grows by one at every Lease and survives the Lease, so a late result is told from a duplicate (same attempt, Step already finished) and from a stale one (older token, or the Step is held by another attempt), per [ADR-0044](adr/0044-restart-semantics-clarifications-from-the-m1-spike.md) point 4. `Lease(owner, attemptId, expiresAt)` issues the token; `RenewLease` and `HoldsLease` check attempt and token; `ReleaseLease(reason)` moves `leased → queued` (`lease_expired`, or `attempt_rejected` when the Worker refuses). A Step also records its `capability` and resolved `inputs`. Only the latest attempt of a Step is remembered, so a result for an earlier attempt is refused as stale without an Event.

**Idempotent commands (2026-10-27, [#157](https://github.com/rluders/amaru/issues/157)):** a mutating command carries an idempotency key. The Platform stores the command's name, key, a fingerprint of the request and the outcome in the same write as the change (`commands`, unique on name and key). The same key with the same request returns the stored outcome (not the state reached since) and changes nothing; the same key with a different request is refused. Implemented for `execution.submit`; Worker messages are not commands, because the fencing token already makes a repeated Result a duplicate. Records are kept; their retention is not decided.

**Concurrency limits (2026-10-28, [#158](https://github.com/rluders/amaru/issues/158)):** an Execution carries a `projectId` and an optional `environmentId`, fixed at creation. The Platform leases a queued Step only while the Steps that are leased or running stay under the global, per-Project (default 2) and per-Environment limits. A Step held back stays `queued` with a reason (`limit:global`, `limit:project`, `limit:environment`) and a `step.deferred` event; leasing it clears the reason. Slots are derived from Step states, not stored, so they cannot leak.

**Workflow definitions (2026-10-28, [#138](https://github.com/rluders/amaru/issues/138)):** a Workflow is a YAML file (`schemaVersion: 1`) of steps, edges and variables. Steps are `agent`, `tool`, `system`, `review` or `approval`; a tool or system step names a Capability, never a provider. Edges order steps and can carry a value between named ports; without edges the steps run in list order, and a cycle is refused. Limits: 50 steps, 200 edges, 50 variables, 8000-character prompt. The file is read strictly (unknown fields, duplicate keys, anchors and aliases, a second document and any other `schemaVersion` are errors); an earlier format is never loaded by accident, only through an explicit migration, which does not exist yet. Execution does not read a Workflow yet: pinning (WFL-002), substitution (WFL-003) and timeouts (WFL-004) are separate.

**Definition pinning (2026-10-28, [#210](https://github.com/rluders/amaru/issues/210), FEAT-WFL-002):** an Execution submitted from a Workflow stores a `DefinitionPin`: the workflow id, the Git commit id it was read at (empty if uncommitted), the SHA-256 of the snapshot, and the snapshot (canonical JSON of the Workflow). Only an `active` Workflow can be pinned. The pin is written with the Execution and never changes; the Platform reads the running definition from it, so resume does not depend on Git. Implemented and unit/integration-tested against the fake and PostgreSQL; the commit id is supplied by the caller, and nothing reads `.amaru/` yet.

### 5.3 Work Item
`todo → in_progress → done`, with `blocked` reachable from `todo` and `in_progress` and back. Status is human-set; the delivery loop may move an item only as its Policy allows and records the transition as an Event **[Prototype]**.

**Implementation status (2026-10-27):** the Work Item status machine and the rules below are implemented and unit-tested in `services/platform/internal/domain` ([#166](https://github.com/rluders/amaru/issues/166)): only a human, or the delivery loop with a Policy authority, may set a status; `done` is final; package progress is derived from the items. **Decided 2026-10-27 (product owner delegated the choice):** (a) `done` is final for automation. A human may reopen it (`done → in_progress`) and must give a reason; the delivery loop, Policy rules and Agents cannot. The domain keeps no history, so the caller records the reason in the Event it emits. (b) `<PROJECTKEY>` is 2–10 characters, `A–Z`/`0–9`, starting with a letter. (c) Work Package status: `planned → active → completed`, `cancelled` from `planned` or `active`, human reopen `completed → active`, `cancelled` final; only a human sets it, and progress stays derived from the items. (d) The Work Item owner is a human; the assignee is a typed reference (`human`, `agent` or `team`) added with the feature that needs it (§7).

**Dependencies (2026-10-27, [#218](https://github.com/rluders/amaru/issues/218)):** implemented and unit-tested. An item does not start (move to `in_progress`) until every dependency is `done`; a dependency missing from the items given counts as not done. Edits that would add an unknown, duplicate, self or cyclic dependency are rejected. A block records which dependencies it waits on when the person gives no reason. **Decided 2026-10-27:** the domain cannot check that a dependency stays within one Project (it sees no Project); the service layer that sees Projects rejects cross-Project edges (§7). The rule is checked when an item starts and stays start-only: nothing moves an item already `in_progress`, and a dependency that regresses later should raise a visible Event for people, not change status (not built). **Not built:** (f) An item waiting on dependencies stays `todo` unless a person or the delivery loop marks it `blocked`; no status is set automatically. (g) External issue dependencies wait for INT-003.

### 5.4 Delivery-loop phases (per Work Item)
`waiting → implementing → validating → reviewing → awaiting_ci → awaiting_merge → done`, with `blocked`, `needs_grant` and `paused` reachable from any active phase **[Prototype `AutoloopPhase`]**. Amaru keeps the phases and moves budget rules (attempts, repairs, reviews, restart recoveries) into Policy.

### 5.5 Approval
`requested → approved | rejected | expired | voided` (voided when its subject changes). **[Prototype: pending/approved/rejected]**; `expired` and `voided` **[Proposed]**.

### 5.6 Worker
`registering → idle ⇄ busy → draining → offline`, with `unhealthy` on missed heartbeats **[Proposed]**.

### 5.7 Plugin installation
`installed → enabled ⇄ disabled → removed`, with `incompatible` when the Instance's contract version leaves the plugin's range **[Proposed]**.

## 6. Differences between the predecessor models

| Topic | prototype Runtime | earlier prototype | Amaru choice |
|---|---|---|---|
| Workflow run | Parent Execution with Step Executions | `WorkflowRun` with per-agent `Execution`s and a state-machine workflow (`workflows/state_machine.go`) | Execution + Step Execution; Workflows are graphs, not state machines |
| Workflow shape | DAG of Steps | States and transitions | DAG; explicit approvals are Steps of type `approval`, and Policy can add an approval gate before any Step (both wait in `awaiting_approval`) |
| Tenant | None (single user) | Workspace | Organization |
| Evidence | Context, events, CI log selection | Diff, VerificationRun, Approval links | Union, as a query |
| Agent definition | Rich (tools, MCP, workspace commands) | Agent + Skills + Team | prototype shape plus Team from both |
| Persistence | Files | PostgreSQL | Storage ports; PostgreSQL everywhere ([ADR-0015](adr/0015-postgresql-is-the-only-database-bundled-in-compact-installs.md)) |
| Intake | GitHub issue intake | Signals with triage | Intake now (P1); Signals later (P2) |

## 7. Resolved domain questions

- **Work Item dependencies** may cross Work Packages within one Project; cross-Project dependencies are not supported.
- **Workflow definitions** live in Git under `.amaru/`; operational state, including Work Items, lives in the Platform ([ADR-0043](adr/0043-project-definitions-in-git-teams-and-operational-state-in-platform.md)).
- **Teams** are Organization-scoped Platform state; definitions reference them by slug ([ADR-0043](adr/0043-project-definitions-in-git-teams-and-operational-state-in-platform.md)).
- **Agents** are Organization principals without login; they act only through Executions, never hold credentials or an Organization role, and are attributed to a human sponsor ([ADR-0038](adr/0038-roles-teams-and-separation-of-duties.md)).
- **Agents and Skills** exist at Organization and Project level; a Project definition with the same id overrides the Organization one ([ADR-0039](adr/0039-configuration-scopes-and-resolution-order.md)).
- **Work Item owner** is always a human; Agents and Teams are assignees. Separation of duties is enforced per gate against the initiator and the Agent's human sponsor ([ADR-0036](adr/0036-approvals-bind-to-evidence-and-gate-work.md), [ADR-0038](adr/0038-roles-teams-and-separation-of-duties.md)).
- **Execution Environments** belong to exactly one Organization. Vendor-run Cloud pools are vendor-owned and leased to one Organization at a time; customer-run Workers are never shared across Organizations ([ADR-0022](adr/0022-cloud-tenancy-row-level-isolation-by-organization.md)).
- **Knowledge references** from Work Items and Steps are path + section + content hash at a commit, resolved by path at the recorded commit; once the Knowledge index ships (FEAT-PRJ-007, P2) search and reference suggestions go through it ([ADR-0030](adr/0030-knowledge-is-lexically-indexed-project-documentation.md)).
