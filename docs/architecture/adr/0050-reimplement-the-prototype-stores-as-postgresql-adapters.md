---
title: Reimplement the prototype stores as PostgreSQL adapters, with the prototype tests as specification
status: proposed
date: 2026-10-27
deciders:
---

# 0050. Reimplement the prototype stores as PostgreSQL adapters, with the prototype tests as specification

## Context
- [ADR-0007](0007-reuse-of-earlier-internal-code-is-a-separate-activity.md) bars moving code from the earlier internal implementation until a human approves a migration ADR for that component. This is the draft migration ADR for its `runtime/internal/store/*` directories (`MADR-store`, issue #76, sub-issue of WP-M3-02 / #14). Every issue that moves this component is gated on it ([delivery §8](../../engineering/delivery.md#8-human-approval-gates)).
- **Source (verified 2026-10-27):** the Runtime repository of the earlier internal implementation (named in the private migration notes), baseline commit **`c8a2f6e`** pinned by [ADR-0032](0032-migration-baseline-is-a-pinned-commit.md). From a `git archive` of that commit:

  | Directory | Non-test Go files | Lines | Tests (files, lines, functions) | Third-party imports |
  |---|---|---|---|---|
  | `store/fsstore` | 10 | 1,459 | 5, 542, 17 | `gopkg.in/yaml.v3` |
  | `store/fsscan` | 3 | 429 | 4, 266, 8 | `gopkg.in/yaml.v3` |
  | `store/projectfs` | 1 | 106 | 1, 68, 3 | none |
  | `store/userstore` | 1 | 233 | 1, 129, 5 | none |

  `go test -count=1 ./internal/store/...` passes on the extracted tree (33 tests, 4 packages, 2026-10-27, Linux).
- **The source is not PostgreSQL.** At `c8a2f6e` every store is file-based: `fsstore` writes `execution.json` (atomically replaced) and an append-only `events.jsonl` per Execution, plus Work, Agent, Team and Workflow files under the project; `userstore` keeps per-user secrets, settings and consents; `projectfs` writes project files and inspects the tree; `fsscan` scans the file tree for Skills and Playbooks. A search of the extracted `runtime` tree for `database/sql` and `pgx` finds nothing (2026-10-27). [ADR-0015](0015-postgresql-is-the-only-database-bundled-in-compact-installs.md) says the earlier prototype "already uses PostgreSQL only, with 30 migrations"; **that claim is not verified at the baseline** and no PostgreSQL code is a migration source here. If it lives in another component or commit, it needs its own migration ADR.
- **Target:** PostgreSQL is the only engine in every deployment ([ADR-0015](0015-postgresql-is-the-only-database-bundled-in-compact-installs.md)); there is no file-store adapter. Migrations are versioned and forward-only ([ADR-0023](0023-self-hosted-oci-image-atlas-migrations-operator-owned-backup.md)); row-level isolation by Organization is accepted ([ADR-0022](0022-cloud-tenancy-row-level-isolation-by-organization.md)); durable execution state follows [ADR-0042](0042-durable-execution-with-explicit-restart-semantics.md). Project definitions live in Git, Teams and operational state in the Platform ([ADR-0043](0043-project-definitions-in-git-teams-and-operational-state-in-platform.md)).
- **Planned disposition** (migration plan, `MADR-store`): **Reimplement, with the prototype tests as specification.** There is nothing to port: the storage engine, schema, transactions and tenancy all differ.
- **Mixed content:** the same directories hold M4 delivery-loop state (`autoloop.go`), shapes whose Amaru home is undecided (`hooks.go`, `settings.go`, `trust.go`, `userstore`, Playbooks in `fsscan`) and project-file writing that belongs to the `.amaru/` layout work. [ADR-0049](0049-adapt-the-prototype-domain-and-ports-into-services-platform.md) already leaves `hook`, `trust`, `grant`, `settings` and `playbook` unmoved.
- **Unverified:** that the 33 prototype tests cover the failure modes that matter on PostgreSQL (concurrent writers, partial failure, restart). The prototype never measured it; the new adapter tests are extended, not trusted.

## Decision
1. **Component and source.** The prototype `internal/store/*` at commit `c8a2f6e` (ADR-0032) is the **behavioral specification** for the Platform storage adapters. No later commit is used.
2. **Disposition: Reimplement.** No prototype store file is copied into this repository. The adapters are written in Amaru against PostgreSQL and the ports the Platform core defines when the first consumer needs them (the `ports` package is deferred by WP-M3-01, see `services/platform/PROVENANCE.md`). The prototype directories are not kept in sync.
3. **History option: none to import.** Because nothing is copied, no history is imported. Provenance is a section in `services/platform/PROVENANCE.md` listing, for each prototype test used as specification, its source path and blob id and the Amaru test that carries the behavior (or why it is dropped).
4. **File disposition.**

   | Source | Disposition | Target / note |
   |---|---|---|
   | `fsstore/fsstore.go` (Execution snapshot and event log) | **Reimplement** | Execution, Step and Event storage on PostgreSQL (FEAT-EXE-002 persistence part, #129). Atomic replace becomes a transaction; append-only `events.jsonl` becomes an ordered Event table; "corrupt data is skipped" is not carried (a database row is valid or absent) |
   | `fsstore/work.go`, `fsstore/team.go` | **Reimplement** | Work Packages, Work Items and Teams as Platform state ([ADR-0043](0043-project-definitions-in-git-teams-and-operational-state-in-platform.md)); the file readers are also the reference for the legacy importer (`amaru import legacy`, [LEGACY_IMPORTER](../LEGACY_IMPORTER.md)) |
   | `fsstore/agent.go`, `fsstore/workflow.go`, `fsstore/workflow_template.go` | **Reference only** | Agents and Workflows are project definitions in Git (`.amaru/`, [ADR-0043](0043-project-definitions-in-git-teams-and-operational-state-in-platform.md)), not database rows; the readers inform the legacy importer only |
   | `fsstore/{autoloop,hooks,settings,trust}.go` | **Deferred / requires human decision** | M4 delivery loop and undecided shapes, as in ADR-0049 item 4. Not moved |
   | `projectfs/projectfs.go` | **Not moved by this ADR** | project-file writing belongs to the `.amaru/` layout work (WP-M1-05); its symlink-escape and write tests are cited as specification for that writer |
   | `userstore/userstore.go` | **Not moved by this ADR** | per-user secrets, settings and consents map to configuration scopes and Policy (WP-M3-04, WP-M3-07); needs its own decision |
   | `fsscan/*` | **Not moved by this ADR** | Skill and Playbook discovery (FEAT-SKL-001; Playbook awaits the owner's decision) |
   | Source ADRs, dogfood records | Reference only | cited, not copied |

5. **Renames** follow the [glossary term map](../../product/GLOSSARY.md#2-term-mapping-from-predecessor-repositories): Workspace (tenant) → Organization; Workspace (agent worktree) → Worktree; Autoloop → Delivery loop; Playbook (workflow sense) → Workflow template; `weave.*` identifiers and paths → Amaru names. No `weave`/`orkestra` string remains in `services/platform` except in `PROVENANCE.md`.
6. **Behavior carried as specification.** From `fsstore`: round trip of an Execution and its ordered Steps; events are appended in sequence order and never rewritten; output is capped while the lifecycle is kept (`TestOutputCapKeepsLifecycle`); concurrent Executions do not interfere (`TestConcurrentExecutions`); path-like ids are rejected (`TestRejectsPathLikeIDs`, now an id-format check); Work and Workflow links survive a restart (`TestWorkflowAndWorkItemLinksSurviveRestart`); the event log is not held open (`TestEventLogIsNotHeldOpen`, becomes connection hygiene). Tests that exist only for file corruption or the `.gitignore` write are dropped, with the reason listed in `PROVENANCE.md`. Behavior that intentionally changes (tenancy, transactions, ADR-0042 restart rule) is listed there too; no test is edited merely to pass. Tests use fakes and the bundled PostgreSQL started by the test harness; no real agent or paid API runs in `make check`.
7. **Gate.** No store behavior is implemented against this specification until this ADR is `accepted` with a recorded approval. Only then do #177 (schema versioning and migrations) and #193 (project metadata store) start.

## Readiness criteria (migration plan §5)
| # | Criterion | State | Evidence |
|---|---|---|---|
| 1 | Source side stable at `c8a2f6e` | **Met** | ADR-0032; commit verified; no later commit used |
| 2 | Inventory refreshed with tests run | **Met** | WP-M1-07 (#8) closed; store tests re-run at `c8a2f6e` on 2026-10-27 (33 passed, see Context) |
| 3 | Target ADRs accepted | **Met** | ADR-0015, 0022, 0023, 0042, 0043 accepted. Undecided shapes are excluded so open decisions are not pre-empted |
| 4 | Contracts stable, with fixtures, in Amaru | **Met** | M2 exit validation (#12) closed; the domain core is merged (WP-M3-01, #13) |
| 5 | Behavior covered by tests that run in Amaru | **Met by plan** | Item 6; becomes evidence in the first implementation PRs |
| 6 | Verified execution | **Gap, listed** | The prototype stores were exercised through their own tests and the Runtime only. Accepting this ADR accepts that gap |
| 7 | License resolved | **Met** | The source is MIT, © Ricardo Lüders, and the holder consents to relicensing ([ADR-0026](0026-fair-code-community-license-with-commercial-licensing.md)). Nothing is copied, and no license file is added |
| 8 | Explicit product-owner approval | **Not met** | Awaiting acceptance |

## Consequences
- **Easier:** no file-format debt or dual storage; one schema, one dialect, tenancy from the first migration; the prototype tests give a behavioral checklist.
- **Harder:** nothing is reused, so the adapter, schema and migration tooling are new work; the specification is only as good as 33 tests, and the PostgreSQL failure modes (concurrent writers, partial failure) need new tests.
- **Security and compatibility:** no secrets or personal data are transferred (`userstore` is excluded). Amaru promises no storage compatibility with the prototype files; the legacy importer is the bridge ([LEGACY_IMPORTER](../LEGACY_IMPORTER.md)).
- **Maintenance:** the storage adapters have a single owner (the Platform owner); the prototype copy is frozen and not a second source of truth.
- **Follow-up:** the ADR-0015 claim about prototype PostgreSQL code needs correcting or sourcing (record in OPEN_QUESTIONS if it stays unverified). `projectfs`, `userstore`, `fsscan`, `autoloop`, `hooks`, `settings` and `trust` each need a decision or migration ADR before they move. The importer has no sub-issue under #14 yet.

## Alternatives considered
- **Port the file stores and add PostgreSQL later:** keeps working code but violates ADR-0015 (no file-store adapter) and creates a second storage path to retire.
- **Port the stores behind the ports with a Postgres backend of the same shape:** the prototype layout (one directory per Execution, one file per item) does not map to transactions, tenancy or SKIP LOCKED timers; the shape would be rewritten anyway.
- **Reimplement from documents only:** loses the 33 behavior tests as a checklist.

## Approval
Proposed by: Claude (migration ADR draft for #76, 2026-10-27). Approved by: _pending_.
