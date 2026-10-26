# One-Shot Legacy Importer (`amaru import legacy`)

Status: **Proposed** (2026-10-26), not implemented, not validated. Design deliverable of [WP-M1-05](WORK_PACKAGES.md#wp-m1-05-amaru-layout-and-importer-design) ([#6](https://github.com/rluders/amaru/issues/6)). Implementation is [WP-M3-02](WORK_PACKAGES.md#wp-m3-02-storage-adapter) ([#14](https://github.com/rluders/amaru/issues/14)) and needs the migration ADR required by [ADR-0007](adr/0007-reuse-of-earlier-internal-code-is-a-separate-activity.md) and [ADR-0032](adr/0032-migration-baseline-is-a-pinned-commit.md). This document authorizes no code, no migration and no change to the prototype.

Target layout: [AMARU_DIRECTORY](AMARU_DIRECTORY.md). Applies [ADR-0043](adr/0043-project-definitions-in-git-teams-and-operational-state-in-platform.md) and [DATA_ARCHITECTURE §6](DATA_ARCHITECTURE.md#6-migration-of-existing-data).

## 1. Evidence base and limits

- **Source of mapping facts.** The prototype Runtime source at the baseline pinned by ADR-0032 (commit recorded in the private migration notes): its domain types, file-store code and path layout. Read on 2026-10-26. Test results for that baseline are in the refreshed inventory kept with the private notes.
- **Not available as evidence:** real user project folders. No counts, no sample data from real projects and no per-file schema versions exist: the prototype has no schema migration. The mappings below are therefore **derived from source, not validated against real data**. First real validation is the WP-M3-02 fixture and a dry run on a copy of a real project, recorded with date and versions.
- Entity coverage is limited to what the prototype source persists in its project folder. Nothing is added for completeness; unlisted data is excluded (§4).

## 2. Source discovery and prerequisites

| Step | Rule |
|---|---|
| Input | `amaru import legacy <project-root> [--platform <endpoint>] [--dry-run]`. `<project-root>` contains the prototype's hidden project folder. |
| Recognition | The folder must contain at least one of: `work/`, `workflows/`, `agents/`, `teams/`. Otherwise exit with "not a prototype project", writing nothing. |
| Read-only | The importer **never writes, renames or deletes** anything in the source folder, and takes no lock on it. It refuses to run if a prototype runtime is detected writing (a recent-modification check on `runtime/` plus a user confirmation) because in-flight Executions are not migrated ([DATA_ARCHITECTURE §6](DATA_ARCHITECTURE.md#6-migration-of-existing-data); users finish or cancel them first). |
| Target | A reachable Platform (local or remote), authenticated as an Owner/Admin of the Organization; the Project is attached (repository key per [AMARU_DIRECTORY §5.1](AMARU_DIRECTORY.md#51-project-key--requires-decision)). The Platform schema version must be current. |
| Git state | Target repository clean in `.amaru/` (or no `.amaru/`); otherwise the importer asks for `--allow-dirty` and records it. The importer writes files but **never commits**; the user reviews and commits (Git review is the point of ADR-0043). |
| Confirmation | Per Project, after the dry-run report is shown ([ADR-0043](adr/0043-project-definitions-in-git-teams-and-operational-state-in-platform.md): "after the user confirms"). |
| Size/format limits | Files over a bounded size (default 1 MiB) or that fail to parse are reported, not guessed. |

## 3. Entity mapping

Each row is justified by prototype source and by [ADR-0043](adr/0043-project-definitions-in-git-teams-and-operational-state-in-platform.md) / the recorded migration dispositions (hooks, trust: [OPEN_QUESTIONS Q54](OPEN_QUESTIONS.md#q54-prototype-migration-details)).

| Prototype data (file) | Evidence (persisted fields) | Target | Notes |
|---|---|---|---|
| Workflows (`workflows/<id>.yaml`; an older `.json` form is also read) | id, name, description, status, type, steps (id, name, type ∈ trigger/agent/tool/system/review/approval, tool, prompt, capability, input, agentId, teamId, role, reviews), variables, edges, layout, timestamps | `.amaru/workflows/<id>.yaml` | `tool` steps map to a Capability only when the tool name matches a documented Capability; otherwise the Step is flagged `unmapped-tool` and the Workflow is imported **disabled** with the reason. `teamId` → Team slug (see Teams). `status` and timestamps dropped (Git history). Layout kept in `layout:`. |
| Agents (`agents/<id>.json`) | id, name, role, instructions, skills, playbooks, requestedCapabilities, providerPreference, model, models, workspaceWrite, workspaceCommands, mcp, status, timestamps | `.amaru/agents/<id>.yaml` | `status`, timestamps dropped. Skill/Playbook names must resolve by path in the repository; unresolved ones are reported and kept as references. Vendor names in `providerPreference` are kept verbatim as preferences (Policy still gates). |
| Workflow templates (project-level, if present) | id, name, parameters, agents, skills, steps, edges | `.amaru/templates/<id>.yaml` | Same rules as Workflows. |
| Teams (`teams/<id>.json`) | id, name, description, members (agentId, role), separationOfDuties | **Platform Team** (Organization-scoped) holding the same Agents | Per ADR-0043 after user confirmation. Prototype Teams are agent compositions: members are Agents only; no human is invented. Slug = normalized `id`; collision with an existing Team slug is a conflict (§6). `separationOfDuties` is mapped only if the Platform Team model has the flag; otherwise reported `unsupported-field`. A Workflow's `teamId` is rewritten to the slug. |
| Work Packages (`work/packages/<id>.json`) | id, title, summary, status ∈ planned/active/completed/cancelled, autoloop policy, intake policy, timestamps | Platform Work Package | Policy sub-objects (autoloop, intake) map to delivery-loop/intake settings only where the Amaru model defines them; otherwise `unsupported-field` and preserved verbatim in import provenance metadata (not applied). |
| Work Items (`work/items/<id>.json`) | id, packageId, title, intent, status ∈ todo/in_progress/blocked/done, blockedReason, owner (kind human/agent, label), dependsOn, workflowId, external issue reference, eligibility, timestamps | Platform Work Item | New ULID + display key assigned by the Platform ([DOMAIN_MODEL](DOMAIN_MODEL.md)); prototype id kept as provenance. `dependsOn` and `packageId` are remapped through the identity map. `owner` kind=agent must resolve to an imported Agent. External issue ref (provider, repo, number, URL, state, labels) is kept as the Work Item external link. Eligibility is not applied (§4). `in_progress` items import as `in_progress` with an `imported` marker and **no Execution** is created. |
| Local Check definitions (inside a Work Package/Workflow policy) | name, command (argv), dir, timeoutSec, maxRuns | `.amaru/config.yaml` `checks[]` | Only definitions. Prior results (`LocalCheckState`) are history: excluded. |
| Hooks (`hooks.yaml`) | id, direction in/out, secret **name**, url, events, format, workflowId, inputs | Inbound → Workflow `triggers[]`; outbound → `config.yaml` `webhooks.outbound[]` | Secret **values** and egress consent are never copied; the importer lists each secret name and URL and requires a human to re-enter them and re-consent. |
| Settings (`settings.json`) | one flag: import host-tool skills | Dropped; reported | A prototype-specific toggle for a retired feature; no Amaru counterpart. |
| Project config (`config.yaml`) | schema version only | `.amaru/config.yaml` (created with `schemaVersion` and name) | Nothing else to map. |
| Linked repositories (project metadata) | id, absolute path, remote | `needs-decision` | See [AMARU_DIRECTORY §3.4](AMARU_DIRECTORY.md#34-repositoriesyaml-linked-repositories--requires-decision). |

## 4. Intentionally excluded

| Data | Why |
|---|---|
| Executions, event logs, `context` evidence (`runtime/executions/…`) | Operational history stays read-only in the prototype ([DATA_ARCHITECTURE §6](DATA_ARCHITECTURE.md#6-migration-of-existing-data)); cannot be replayed under ADR-0042 semantics. |
| Delivery-loop ledgers, hook queue, delivery records (`runtime/…`) | Operational state of a live runtime; no dual-write. |
| Trust records (`trust.json`) and Grants | Trust is bound to content hashes and is a human decision in the Platform; re-granted, never copied ([Q54](OPEN_QUESTIONS.md#q54-prototype-migration-details)). |
| Credentials and the user state dir | Re-entered or imported only through the secrets port with consent. |
| Eligibility flags on Work Items | Derived from a prototype loop policy that Amaru re-evaluates. |
| Retired providers/plugin settings, host-tool skills toggle | Disposition: retired ([Q54](OPEN_QUESTIONS.md#q54-prototype-migration-details)). |
| Prototype PostgreSQL data (earlier prototype) | Not migrated. |

## 5. Identity and provenance

- **Identity-map key:** `legacy:<kind>:<source-id>` scoped by `(projectKey, source fingerprint)`, with `kind` ∈ workflow, agent, team, package, item, hook. The key is stored in an `import_map` Platform table row per object: `{importRunId, key, targetKind, targetId, sourceDigest, importedAt}`.
- **Source digest:** `sha256` over the source file bytes (LF-normalized). Used for idempotency and conflict detection.
- **Provenance on every created object:** Platform objects carry `provenance: {origin: "legacy-import", importRunId, key, sourceDigest}`. File definitions carry a comment header `# imported-from: legacy:<kind>:<id> sha256:<digest> run:<importRunId>` (a comment, not data, so schema validation is unaffected).
- **Ids:** file definitions keep the prototype id when it satisfies the id rule (AMARU_DIRECTORY §2), else a normalized id with the original in the header. Platform Work Packages/Items get fresh ULIDs.
- The source fingerprint is `sha256` of the sorted list of `(relative path, digest)` of everything read. Two runs against unchanged sources have the same fingerprint.

## 6. Dry run, validation, conflicts

Phases (the first three never write):
1. **Scan** — enumerate and parse; collect per-file diagnostics.
2. **Plan** — build the identity map, resolve references, compute actions per object: `create`, `skip-identical`, `update-offer`, `conflict`, `unsupported`, `needs-decision`, `excluded`.
3. **Report** — human summary and machine-readable JSON (`importRunId`, fingerprint, per-object action, diagnostics, secrets-to-re-enter list). `--dry-run` stops here; the report is deterministic for the same input and target state.
4. **Apply** — only after explicit confirmation of the report's fingerprint (the same plan that was shown).

| Case | Handling |
|---|---|
| Target object exists with a provenance key and **same digest** | `skip-identical` (no-op). |
| Exists with provenance key, **source changed** since last import | `update-offer`: reported; applied only with `--update <key>`; never silently. |
| Exists **without** provenance (created by hand or another import) | `conflict`. Never overwritten. Choices: `--on-conflict skip` (default), `rename` (new slug/id suffix `-imported`), or `abort`. File definitions that already exist on disk are treated the same way. |
| Duplicates within the source (same id in `.json` and `.yaml`) | Prefer the `.yaml`, diagnose the other as `shadowed`. Two distinct objects normalizing to one id: `conflict`, nothing written for either. |
| Missing reference (Agent, Team, Work Item, package, Skill) | The referring object is imported **disabled / flagged** where the target type allows (`dangling-reference`); Work Items with a missing package are not imported (`orphan`). Never invent the target. |
| Unsupported field | Preserved verbatim in provenance metadata, reported, not applied. Unknown *structure* in an otherwise parseable file: the file is `unsupported`, not partially guessed. |
| Parse error | Object `unparseable`; others proceed unless `--strict`. |
| Partial entity (e.g., Workflow with one unmapped Step) | Imported disabled with all diagnostics; user fixes in Git and re-runs (§7). |
| Secret-looking value found in a source file | Value is **redacted from the report and not written**; the object is flagged `secret-detected`. |

## 7. Idempotency, transactions and recovery

- **Rerun semantics.** Rerunning on the same source and target is a no-op (`skip-identical` everywhere). Rerun after fixing sources or after a failure processes only objects not yet in the identity map or flagged `update-offer`. The import is one-shot in intent (no sync, no dual-write): a rerun never deletes anything, and never reflects deletions made in the source.
- **Platform writes.** One PostgreSQL transaction per Project import creates all Platform objects, the `import_map` rows, an `import_runs` row and the `import.completed` Event; resulting messages leave through the outbox, never published directly by the importer ([DATA_ARCHITECTURE §4](DATA_ARCHITECTURE.md#4-consistency-rules)). A crash before commit leaves no Platform change. Rationale: the WP-M1-02 spike showed delivery is at-least-once and consumers need idempotency keys in PostgreSQL ([A9](investigations/2026-10-26-wp-m1-02-storage-and-execution.md)); `importRunId` + `key` are those keys.
- **File writes.** Rendered to a staging directory next to the target (`.amaru.import-<runId>/`), validated with the AMARU_DIRECTORY validators, then moved into place file by file with atomic rename, never replacing an existing file. Order: files first, then the Platform transaction, so a Platform failure leaves only reviewable, uncommitted files. A crash between the two leaves files with no `import_runs` row; the next run sees them as `conflict` unless their header digest and key match, in which case they are adopted as `skip-identical`.
- **Recovery / rollback.**
  - *Before Platform commit:* delete the staging directory; remove uncommitted files listed in the run log (`git status` shows them; Git is the safety net since the importer never commits).
  - *After commit, wrong result:* `amaru import legacy --rollback <importRunId>` deletes Platform objects whose provenance matches that run and which have **not been modified since** (compared to `sourceDigest`/`updatedAt`) and whose Work Items have no Executions, Approvals or comments; anything touched is listed and left. It writes `import.rolled_back` Events. File definitions are reverted with `git`, by the user.
  - Rollback is a Platform feature of the importer, **Proposed** and **untested**.
- **Observability.** Events `import.started`, `import.planned`, `import.object_created`, `import.object_skipped`, `import.conflict`, `import.completed`, `import.failed`; every Event carries `importRunId`, the fingerprint and the actor. The run report is stored as an Artifact attached to the run.
- **Reproducibility.** The report records tool version, source fingerprint, target Platform schema version and flags; same inputs ⇒ same plan.
- **Spike carry-over (WP-M1-02, conditionally validated on the Linux, macOS and Windows hosted runners).** Passed on all three: PostgreSQL scenarios 9/9, crash matrix 86/86, with the documented mitigations. Conditions the report keeps open (A2, A9): Windows non-admin, macOS signing/notarization and footprint, upgrades and power loss. **Not tested by the spike and therefore untested design here:** importer transactions, definition snapshots, content hashing, `.amaru/` loading. No spike result contradicts this draft and no ADR change is required by it.

## 8. Fixture shape for WP-M3-02

A fixture is a synthetic prototype project folder checked in under the importer's test data (no real data, no secrets), containing: 2 Workflows (one fully mappable, one with an unmapped tool step), 2 Agents (one referencing a missing Skill), 1 Team with two members, 1 Package, 3 Items (with a dependency, an agent owner, an external issue link), 1 inbound and 1 outbound hook with secret names, one `.json` shadowed by a `.yaml`, one runtime folder (must be ignored). Expected outputs are a golden dry-run report and golden `.amaru/` tree. Tests use fakes for the Platform; no real agents or paid APIs.

## 9. Walkthrough against representative prototype shapes

Synthetic data shaped by the source structs; **no real project was read**.

1. **Work Item.** `{id: "wi-7", packageId: "wp-2", title: "Add retry", intent: "…", status: "in_progress", owner: {kind: "agent", label: "reviewer-a"}, dependsOn: ["wi-3"], workflowId: "feature"}`
   → plan: `wp-2` and `wi-3` must be in the map (else `orphan`/`dangling-reference`); new ULID and display key; owner `reviewer-a` must match imported Agent `reviewer-a`; status kept with an `imported` marker; no Execution created; `workflowId: feature` remapped to the imported Workflow id.
2. **Team.** `{id: "reviewers", members: [{agentId: "reviewer-a", role: "reviewer"}], separationOfDuties: true}`
   → Platform Team slug `reviewers` holding Agent `reviewer-a`; the slug is checked for collision with an existing Team; the user confirms; Workflow steps with `teamId: "reviewers"` become `team: reviewers` and validate at start against this Team ([AMARU_DIRECTORY §6](AMARU_DIRECTORY.md#6-team-references-by-slug)). No human members appear: humans are added in the Platform by an Admin.
3. **Inbound hook.** `{id: "nightly", direction: "in", secret: "NIGHTLY_KEY", workflowId: "feature", inputs: ["branch"]}`
   → Workflow `feature` gets `triggers: [{id: nightly, inputs: [branch], secret: NIGHTLY_KEY}]`; the report lists `NIGHTLY_KEY` under "re-enter secret"; no value is read or stored; if `feature` was imported disabled, the trigger is imported disabled too.

## 10. Data-class review (importer)

| Data | Class | Destination | Check |
|---|---|---|---|
| Workflows, Agents, templates, checks, outbound-hook config | Definitions | `.amaru/` files (reviewed in Git) | ✔ no runtime state, secrets by name only |
| Teams | Operational (authorization) | Platform | ✔ not committed |
| Work Packages, Work Items | Operational | Platform | ✔ not committed |
| Import run, identity map, events, report | History and audit | Platform | ✔ |
| Executions, logs, ledgers, trust, grants, credentials | Operational/History/Secrets | **Not imported** | ✔ stays in prototype |

## 11. Not decided here

- The Project key, proposed in [ADR-0046](adr/0046-project-key-is-the-normalized-remote-plus-root-commit.md) ([AMARU_DIRECTORY §5.1](AMARU_DIRECTORY.md#51-project-key--requires-decision)) and Linked-repository placement ([Q55](OPEN_QUESTIONS.md#q55-project-key-and-linked-repository-placement)).
- Whether Teams can carry `separationOfDuties` as an attribute ([ADR-0038](adr/0038-roles-teams-and-separation-of-duties.md) owns Team semantics).
- The migration ADR that authorizes implementing the importer (ADR-0007 gate).

Related: [ADR-0043](adr/0043-project-definitions-in-git-teams-and-operational-state-in-platform.md) · [AMARU_DIRECTORY](AMARU_DIRECTORY.md) · [DATA_ARCHITECTURE](DATA_ARCHITECTURE.md) · [WP-M1-05](WORK_PACKAGES.md#wp-m1-05-amaru-layout-and-importer-design) (#6) · [WP-M1-02](WORK_PACKAGES.md) (#3) · [WP-M3-02](WORK_PACKAGES.md#wp-m3-02-storage-adapter) (#14).
