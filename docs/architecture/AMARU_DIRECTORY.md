# The `.amaru/` Directory, Project Identity and Execution Provenance

Status: **Proposed** (2026-10-26), not implemented, not validated. Design deliverable of [WP-M1-05](WORK_PACKAGES.md#wp-m1-05-amaru-layout-and-importer-design) ([#6](https://github.com/rluders/amaru/issues/6)). It applies the accepted [ADR-0043](adr/0043-project-definitions-in-git-teams-and-operational-state-in-platform.md) and [DATA_ARCHITECTURE](DATA_ARCHITECTURE.md). It does not create `.amaru/`, a schema file, a parser or a migration. The importer that fills this layout is designed in [LEGACY_IMPORTER](LEGACY_IMPORTER.md).

Two points are **Requires decision** (proposed in [ADR-0046](adr/0046-project-key-is-the-normalized-remote-plus-root-commit.md), awaiting the product owner) (§5.1 repository key, §3.4 Linked repository). Everything else follows from accepted ADRs.

## 1. Principles

- `.amaru/` holds **definitions only**: text a human reviews in a pull request ([DATA_ARCHITECTURE §1](DATA_ARCHITECTURE.md#1-data-classes)).
- Nothing Amaru writes at runtime lands in the Project tree ([ADR-0043](adr/0043-project-definitions-in-git-teams-and-operational-state-in-platform.md)). There is no `runtime/`, `state/`, `cache/` or `logs/` directory under `.amaru/`. Convenience of serialization is never a reason to commit state.
- Authorization data never lives in Git: Team membership, Approval authority, Grants and trust decisions are Platform-owned.
- Secrets are referenced by **name** only ([ADR-0039](adr/0039-configuration-scopes-and-resolution-order.md)).
- Every file carries `schemaVersion`; loaders refuse unknown future versions ([DATA_ARCHITECTURE §4](DATA_ARCHITECTURE.md#4-consistency-rules)).
- Files are edited by humans, by Amaru clients (atomic temp file + rename, as in [TECHNOLOGY_BENCHMARK](TECHNOLOGY_BENCHMARK.md)) or by the importer. The Platform reads them; it writes only on an explicit user action through a client or `amaru import legacy`.

## 2. Layout

```text
.amaru/
  config.yaml            # Project configuration, Local Check definitions
  workflows/<id>.yaml    # one Workflow (graph of Steps) per file
  agents/<id>.yaml       # Project Agents (override Organization Agents of the same id)
  templates/<id>.yaml    # Workflow templates
  repositories.yaml      # Linked repositories (Proposed; see §3.4)
```

Skills and Playbooks are Project knowledge, not `.amaru/` content: they stay where the user keeps them and are **referenced** by path (config `knowledge` globs; Agent `skills`/`playbooks` lists) and read by path + hash ([DATA_ARCHITECTURE §1](DATA_ARCHITECTURE.md#1-data-classes)).

Ids are lowercase `[a-z0-9][a-z0-9-]{0,62}`, equal to the file stem. A file whose stem and `id` differ is invalid.

## 3. Per-path specification

Common rules for every file below:
- **Versioning.** `schemaVersion: <integer>` (major). Minor additions are optional fields. Versioning rule in §4.
- **Git vs Platform.** The file is the source of truth. The Platform stores only the per-Execution snapshot (§5.3) and change Events.
- **Secrets.** Forbidden values. A key whose name suggests a secret (`secret`, `token`, `password`, `apiKey`, `credential`) must hold a secret **name**, validated against the pattern `[A-Z][A-Z0-9_]*`; any other value is a validation error.

### 3.1 `config.yaml`
| Aspect | Specification |
|---|---|
| Purpose | Project scope of [ADR-0039](adr/0039-configuration-scopes-and-resolution-order.md): Workflow defaults, Local Check definitions, Knowledge globs, Project Agent defaults. |
| Owner | Project maintainers (via pull request). |
| Fields (proposed) | `schemaVersion`; `project.name`; `defaults.workflow`; `checks[]` = `{name, command[], dir, timeoutSec}` with an optional `maxRuns`; `knowledge.globs[]`; `webhooks.outbound[]` = `{id, url, events[], secret}` (`secret` = name). |
| Stays in Platform | Policy, egress consent for each URL, Provider connections, retention, Organization defaults, locked keys. Project config may only **tighten** security-relevant keys. |
| Validation | Schema; `command` is an argv array, never a shell string; `dir` is project-relative with no `..`; `timeoutSec` bounded by the Organization maximum. |
| Import | Prototype `config.yaml` holds only a schema version; checks, settings and outbound hooks are mapped by the importer ([LEGACY_IMPORTER §3](LEGACY_IMPORTER.md#3-entity-mapping)). |

### 3.2 `workflows/<id>.yaml`
| Aspect | Specification |
|---|---|
| Purpose | One Workflow: a DAG of Steps ([DOMAIN_MODEL](DOMAIN_MODEL.md)). |
| Owner | Project maintainers. |
| Fields (proposed) | `schemaVersion`, `id`, `name`, `description`, `variables[]` = `{name, default}`, `steps[]` = `{id, name, type, capability, prompt, input{}, agent, team, role, reviews}`, `edges[]` = `{from, fromPort, to, toPort}`, `triggers[]` (webhook Trigger = `{id, inputs[], secret}`). Editor layout (node positions) lives in an optional `layout:` block so diffs of logic stay clean. |
| References | `capability` names a Capability, never a vendor or tool ([AGENTS.md](../../AGENTS.md)). `agent` is an Agent id (Project or Organization). `team` is a Team **slug**. |
| Stays in Platform | Execution state, Step state, Approvals, Lease assignments. |
| Validation | Schema; unique step ids; edges reference existing steps and form a DAG; every `agent` resolves; every `team` slug exists (§6); `reviews` points to an earlier step. |
| Import | From the prototype Workflow file; see importer mapping. |

### 3.3 `agents/<id>.yaml` and `templates/<id>.yaml`
| Aspect | Specification |
|---|---|
| Purpose | Agent: instructions, model/provider preference, Skill and Playbook references, requested Capabilities, tool scope. Template: parametrized Workflow plus the Agents it needs. |
| Fields (proposed, Agent) | `schemaVersion`, `id`, `name`, `role`, `instructions`, `skills[]`, `playbooks[]`, `requestedCapabilities[]`, `providerPreference[]`, `model`, `models{}`, `workspaceWrite`, `workspaceCommands[]`, `mcp[] = {server, tools[]}`. |
| Stays in Platform | Organization Agents; Agent **snapshots** per Execution (run history); trust in an Agent's Skills and MCP servers (a Platform decision bound to a content hash, see §5.4). |
| Validation | Schema; Skill/Playbook references resolve by path; `workspaceWrite` and `mcp` are *requests* that Policy still gates. |
| Import | Direct field mapping; prototype `status` and timestamps are not carried (Git history replaces them). |

### 3.4 `repositories.yaml` (Linked repositories) — Requires decision
[DOMAIN_MODEL](DOMAIN_MODEL.md) classes a Linked repository (delivery target, base branch, delivery policy) as a Git-reviewable definition, but [ADR-0043](adr/0043-project-definitions-in-git-teams-and-operational-state-in-platform.md) does not list it. Recommendation: commit `{id, remote, baseBranch, deliveryPolicy}` and keep **absolute local paths in the Platform** (they differ per clone). Until decided, the importer reports prototype repository entries as `needs-decision` and writes nothing for them.

### 3.5 Explicitly not in `.amaru/`
| Data | Where | Why |
|---|---|---|
| Teams, members, team roles | Platform (PostgreSQL) | Membership gates Approvals; repository write must not grant authority (ADR-0043). |
| Work Packages, Work Items, ledgers | Platform | Operational state. |
| Executions, Step state, checkpoints, Events | Platform | Crash-safe transactional state; spike evidence in [the WP-M1-02 report](investigations/2026-10-26-wp-m1-02-storage-and-execution.md). |
| Logs, transcripts, check output, Artifacts, `context` evidence | Run history / Artifact store | Size-bounded, redacted, retention-limited. |
| Secrets, hook secret values, consent records | Secrets backend / Platform | Never in another class. |
| Trust decisions, Grants | Platform | Content-bound trust is keyed by hash, not stored beside the content. |
| Generated or cached indexes (Knowledge index) | PostgreSQL | Derived data. |
| Absolute paths, machine names, user names | Platform clone record (§5.2) | Differ per clone and leak host information. |

## 4. Validation, unknown fields and versions

| Case | Behavior |
|---|---|
| `schemaVersion` missing | Invalid. (Legacy files without it are handled only by the importer, never by the loader.) |
| `schemaVersion` greater than the loader supports | Refuse to load; the Project is `definitions-unsupported`; no Execution starts. Message names the file and the version. |
| `schemaVersion` older, supported | Load through an explicit in-memory migration; never rewrite the file implicitly. |
| Unknown field, same major | **Warn** and ignore at load; **fail** in `amaru`'s strict check mode used by CI and before an Execution starts. A typo must not silently disable a gate. |
| Unknown top-level file or directory under `.amaru/` | Warn; ignored. Never deleted. |
| Duplicate id across files or between Project and Organization | Project overrides Organization for that Project ([DOMAIN_MODEL](DOMAIN_MODEL.md)); two Project files with one id: invalid. |
| Parse error in one file | That definition is invalid; others stay usable; Executions referencing it do not start. |
| Out-of-band edit | Change Event emitted (FEAT-WRK-003); next Execution records the new revision. |

Evidence level: **not validated**. No parser or fixtures exist; fixtures arrive with the M2 contracts and WP-M3-02.

## 5. Project identity, clones and Cloud

### 5.1 Project key — Requires decision
[ADR-0043](adr/0043-project-definitions-in-git-teams-and-operational-state-in-platform.md) and WP-M1-05 state that a Project is keyed by repository. Neither says how the key is derived. Options:

| Option | Key | For | Against |
|---|---|---|---|
| A (recommended) | Normalized remote URL (scheme/host lowercased, `.git` and trailing slash dropped, credentials stripped) **plus** the root commit SHA as a disambiguator | Same key from any clone; stable across path spelling; detects an unrelated repo that reuses a URL | Forks share a root commit (a fork is a different Project unless the user links it); repositories without a remote need option C |
| B | Remote URL only | Simple | Rename/transfer breaks identity; mirrors collide |
| C | `project.id` ULID generated by Amaru and stored in `.amaru/config.yaml` | Survives remote changes and works with no remote | Copy-paste of a repo duplicates the id; forces a committed id |

Recommended: A, with C as a fallback for repositories with no remote (the id is generated on `attach` and committed after review). Alias handling: if a second remote normalizes differently, the user links it to the Project explicitly; Amaru never merges Projects automatically. This question is recorded in [OPEN_QUESTIONS](OPEN_QUESTIONS.md#q55-project-key-and-linked-repository-placement) and needs a human decision before WP-M3-01 builds `attach`.

### 5.2 Clone record (Platform)
A **clone** is one working copy that registered with a Platform. Record: `cloneId` (ULID), `projectKey`, `hostLabel`, `rootPath` (absolute, Platform-only), `registeredAt`, last observed `HEAD` and branch, last seen dirty flag. Many clones per Project. Clone records are operational state; no clone information is committed. Operational state (Work Items, Executions, Approvals) is keyed by Project, so two clones never conflict on it.

### 5.3 Execution provenance
Every Execution records, at start, in one PostgreSQL transaction with its first Event:

| Field | Meaning |
|---|---|
| `projectKey` | Repository identity (§5.1). |
| `cloneId` | Which clone supplied the definitions (`null` for Cloud fetches, which record `source: fetch` and the remote). |
| `commitSha` | `HEAD` of the clone, or the explicit commit fetched. |
| `dirty` | `true` if `.amaru/` or any referenced Project knowledge differs from `commitSha`. |
| `contentHash` | `sha256` over the canonical listing of **definitions used**: for each file, sorted by path, `path NUL sha256(bytes) LF`. Bytes are hashed as stored (no re-serialization) with CRLF normalized to LF. Algorithm id is stored (`sha256-v1`) so it can change. |
| `definitionSnapshot` | Immutable copy of the resolved definitions used (Workflow, Agents, config, resolved Team slugs, Skill/Playbook hashes) in run history. |
| `teamSlugs` | Slugs referenced and the Platform Team ids they resolved to at start. |

The snapshot makes resume independent of Git: a resumed Execution uses its own snapshot, never the current branch head ([DOMAIN_MODEL](DOMAIN_MODEL.md): an Execution runs exactly one Workflow version). With `(commitSha, contentHash, definitionSnapshot)` an auditor can reconstruct source and configuration; with `dirty = true` the snapshot is the *only* faithful record, so Evidence flags it. Dirty trees may run locally by policy; the Organization may forbid them.

Untested design: the WP-M1-02 spike exercised state-machine crashes, leases and the outbox, **not** snapshots, hashing or `.amaru/` loading. Snapshot size is bounded by recording only used definitions and referencing Skills/Playbooks by hash, not body (size risk flagged for FEAT-WFL-002).

### 5.4 Trust and the content hash
Trust in Skills, Playbooks and MCP servers is a Platform decision bound to a content hash. If trusted content changes, trust no longer matches and a human must re-trust; `.amaru/` never carries trust.

### 5.5 Behavior matrix
| Situation | Behavior |
|---|---|
| Two clones, same repository, same commit | One Project, two clone records; Executions record the clone used; operational state shared. |
| Two clones, divergent commits | Each Execution uses the definitions of the clone that requested it and records its `commitSha`. No merge. Concurrent definition divergence is shown in the UI as "clone X is N commits ahead/behind"; Platform never picks a winner silently. |
| Uncommitted working-tree changes | Local Platform may read the working tree; `dirty = true`, flagged in Evidence; snapshot holds the exact bytes. Remote/Cloud never see them. |
| Cloud/remote execution | Requires an explicit `commitSha`; fetch that commit, verify the object exists and the content hash after checkout; never follow a branch head. |
| Commit missing (not pushed or garbage-collected) | Execution refuses to start with `definitions-unavailable`, naming the SHA. A running Execution is unaffected (it has its snapshot). |
| Repository inaccessible (auth, network, deleted) | Start refused with an error that does not include credentials; existing Executions continue; Project shows `repository-unreachable`. |
| Stale or unknown Team slug | Definition fails validation and does not start ([ADR-0043](adr/0043-project-definitions-in-git-teams-and-operational-state-in-platform.md)). An Execution already running keeps the Team ids resolved at start. A slug re-pointed to another Team is a Platform Event and applies to **new** Executions only. |
| Conflicting definitions (same id in two files; Project vs Organization) | Same-id Project files: invalid. Project vs Organization: Project wins, shown in Evidence. Git merge-conflict markers in a file: parse error, definition invalid. |
| Fork or renamed remote | New key unless the user links it (§5.1). |
| Shallow clone | Allowed if the needed commit is present; otherwise `definitions-unavailable`. |

## 6. Team references by slug

- Syntax: Team slugs match `[a-z0-9][a-z0-9-]{0,62}`; references appear in Workflow `team` fields and gate definitions only as slugs.
- Validation points: (1) *Check time* (`amaru` validation, CI): slugs are validated against the Platform's Teams **if a Platform is reachable**; otherwise reported as `unverified`, not valid. (2) *Start time*: mandatory; an unknown slug blocks the Execution. (3) *Approval time*: authority is evaluated from Platform membership at decision time, never from a Git file.
- Team creation, membership and role changes are Platform Events audited by actor ([ADR-0038](adr/0038-roles-teams-and-separation-of-duties.md)).
- Slug deletion/rename: blocked while a committed definition at the Project's default branch references it, or allowed with a warning listing referencing Projects (Platform decision; unspecified detail → [OPEN_QUESTIONS Q55](OPEN_QUESTIONS.md#q55-project-key-and-linked-repository-placement) notes it).

## 7. Data-class review against DATA_ARCHITECTURE

| Class ([§1](DATA_ARCHITECTURE.md#1-data-classes)) | Appears in `.amaru/`? | Review result |
|---|---|---|
| Project knowledge | Referenced by path + hash only | Never moved or rewritten. |
| Definitions | **Yes**, all of it | Versioned by Git; snapshot + `commitSha` + `contentHash` per Execution; diffable text; out-of-band edits emit Events. |
| Operational state | No | Teams, Work Items, Executions, leases, ledgers, outbox stay in PostgreSQL (§3.5). |
| History and audit | No | Snapshots and Events in run history. |
| Logs and evidence | No | Never committed. |
| Artifacts | No | Artifact store. |
| Secrets | **Names only** | Pattern-validated; values rejected. |
| Configuration | Project scope only | Tighten-only for security keys ([ADR-0039](adr/0039-configuration-scopes-and-resolution-order.md)). |
| Deployment state | No | Platform store. |

Review is a document review. No automated check enforces it yet; a lint for secret-like values and unknown directories is a candidate for the M2 contract work (not scheduled here).

## 8. Related

[ADR-0043](adr/0043-project-definitions-in-git-teams-and-operational-state-in-platform.md) · [ADR-0039](adr/0039-configuration-scopes-and-resolution-order.md) · [ADR-0038](adr/0038-roles-teams-and-separation-of-duties.md) · [DATA_ARCHITECTURE](DATA_ARCHITECTURE.md) · [DOMAIN_MODEL](DOMAIN_MODEL.md) · [LEGACY_IMPORTER](LEGACY_IMPORTER.md) · [WP-M1-05](WORK_PACKAGES.md#wp-m1-05-amaru-layout-and-importer-design) (#6) · [WP-M1-02 report](investigations/2026-10-26-wp-m1-02-storage-and-execution.md) (#3) · WP-M3-01/WP-M3-02 consume this design.
