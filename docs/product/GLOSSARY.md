# Glossary

The single source of Amaru terminology. Other documents use these terms with these meanings. When a term changes, change it here first.

**Status column**
- *Exists in Weave*: implemented in the predecessor project [`rluders/weave-obsidian`](https://github.com/rluders/weave-obsidian) (see [weave-inventory](../migration/weave-inventory.md)). Nothing in this repository implements it yet.
- *New*: an Amaru concept with no implementation anywhere.

| Term | Meaning | Status |
|---|---|---|
| **Project** | An existing engineering codebase or workspace that Amaru coordinates. Its files stay authoritative and are never reorganized; Amaru metadata lives apart from them. | Exists in Weave (an Obsidian vault + `.weave/`) |
| **Work Package** | A group of related Work Items with a goal (an epic, a milestone, a feature). | Exists in Weave |
| **Work Item** | A unit of intended work with a status set by a human. It may link to the Executions that worked on it. | Exists in Weave |
| **Workflow** | A versioned definition of multi-step work: a graph of Steps with ordering and data edges. | Exists in Weave (DAG, sequential execution) |
| **Step** | One node of a Workflow: an agent step, a tool step, or (future) a human Approval step. | Exists in Weave (agent and tool steps) |
| **Execution** | One run of a Workflow or a single Step, with a lifecycle (`queued → running → succeeded/failed/cancelled`), an ordered Event stream and recorded evidence. | Exists in Weave |
| **Event** | An immutable, ordered record of something that happened in an Execution or to platform state. The basis of observability and audit. | Exists in Weave (per-execution `events.jsonl`) |
| **Artifact** | An output or input of work that the platform tracks: a file in the Project, a generated asset, a report. Identified by path and content hash. | Exists in Weave (index of typed notes + generated files) |
| **Evidence** | What an Execution records to justify its result: assembled context, Skill/Agent snapshots, outputs and hashes, logs, validation results. | Exists in Weave (partial) |
| **Agent** | A reusable definition of an AI worker: instructions, model preference, provider preference, referenced Skills, requested Capabilities. Not a running process. | Exists in Weave |
| **Skill** | User-owned procedural knowledge (Markdown) that an Agent may reference, with explicit trust and a content hash. | Exists in Weave |
| **Capability** | A named, typed unit of work the platform can perform (`agent.run`, `scene.inspect`, `doc.convert` …), independent of which tool performs it. | Exists in Weave (catalog of typed specs) |
| **Provider** | An implementation of one or more Capabilities for a concrete tool or service (Claude CLI, Blender, a remote API). Built in, or an external executable speaking a provider protocol. | Exists in Weave |
| **Adapter** | The code inside a Provider that translates between the neutral Capability contract and a concrete tool's I/O. | Exists in Weave |
| **Execution Environment** | A machine or sandbox where Steps actually run, with its own tools, resources and trust level (the user's laptop, a Windows/GPU workstation, a Cloud worker). | New |
| **Worker** | A process inside an Execution Environment that accepts Steps from the Platform and runs them through Providers. | New (Weave's Runtime is a single local worker + coordinator in one process) |
| **Approval** | An explicit human decision required before a consequential action proceeds, recorded with who, what, when and on what evidence. | New (not built in Weave; ROADMAP Q2 there) |
| **Policy** | Rules that decide which Capabilities an Agent or Workflow may use, where, and which actions need Approval. | New (Weave has no capability permission model, GAP-09) |
| **Plugin** | A packaged extension of Amaru: UI contributions, integrations, Capabilities/Providers, or environment adapters, with declared permissions. | New |
| **Amaru Platform** | The deployable system that owns domain state, contracts, policies and execution coordination. | New (proposed) |
| **Amaru Cloud** | A managed deployment of the Platform operated by the Amaru vendor. | New (direction only) |
| **Amaru Self-hosted** | A user-operated deployment of the same Platform. | New (direction only) |
| **Amaru Desktop** | A native desktop client of the Platform. Not the owner of execution state. | New (direction only) |
| **Integration** | A connection to an external system Amaru coordinates but does not replace (Git hosting, issue trackers, CI, IDEs). | New |

## Term mapping from Weave

| Weave term | Amaru term | Note |
|---|---|---|
| Runtime (`weaved`) | Platform + local Worker | Weave combines coordination and execution in one per-user daemon. Amaru separates them conceptually; whether they stay one process locally is open ([OPEN_QUESTIONS](../architecture/OPEN_QUESTIONS.md)). |
| Obsidian plugin | One client among several | The plugin is a client and integration, not the Amaru Desktop. |
| Vault / `.weave/` | Project / project metadata location | Where Amaru stores project metadata is open. |
| `weave.provider/v1` | Provider protocol | A candidate for the Amaru provider contract, subject to the migration assessment. |
| Orkestra | (former name) | Appears only in historical design files. |
