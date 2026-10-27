---
title: Adapt the prototype execution manager into the Execution coordinator, on durable PostgreSQL state
status: proposed
date: 2026-10-27
deciders:
---

# 0052. Adapt the prototype execution manager into the Execution coordinator, on durable PostgreSQL state

## Context
- [ADR-0007](0007-reuse-of-earlier-internal-code-is-a-separate-activity.md) bars moving code from the earlier internal implementation until a human approves a migration ADR for that component. This is the draft migration ADR for the `app` package of its Runtime (`runtime/internal/app`; `MADR-app`, issue #77, sub-issue of WP-M3-03 / #15). Every issue that moves this component is gated on it ([delivery §8](../../engineering/delivery.md#8-human-approval-gates)).
- **Source (verified 2026-10-27):** the Runtime repository of the earlier internal implementation (named in the private migration notes, not part of this repository), baseline commit **`c8a2f6e`** ([ADR-0032](0032-migration-baseline-is-a-pinned-commit.md)). From a `git archive` of that commit, `internal/app` is 26 non-test Go files plus test files; it imports only the standard library, `domain` and `ports` (verified with a scan of the import blocks). It has **252 test functions**, of which **138** are in the delivery-loop, intake and local-check files. `go test -count=1 ./internal/app/...` passes on the extracted tree (2026-10-27, Linux, 12 s).
- **Planned disposition** (migration plan, `MADR-app`): **Adapt** into the Execution coordinator. [ADR-0042](0042-durable-execution-with-explicit-restart-semantics.md) already says the Platform's state machine is "adapted from the prototype coordinator".
- **What the prototype coordinator is.** `Manager` keeps one in-memory `record` per Execution behind a single mutex, with a goroutine per run, an in-memory idempotency map, an in-memory event bus and an optional `ExecutionStore` that is written after the fact ("nil → history is memory-only"). Restart marks queued or running Executions `failed` with `runtime_restarted`, except a workflow whose running Step has a provider job id, which waits for `ResumeInterrupted`. Agent slots are a process-wide counter. Approval waits are a channel on the record.
- **Why it is not a straight port.** ADR-0042 and [ADR-0044](0044-restart-semantics-clarifications-from-the-m1-spike.md) replace the restart rule and put all execution state in PostgreSQL with a monotonic version, a transactional outbox, timers and persisted Approval gates; a waiting Step holds no goroutine. The M1 spike (WP-M1-02, [report](../investigations/2026-10-26-wp-m1-02-storage-and-execution.md)) validated that design against a crash-injection matrix; the prototype never had one. The domain it drives was already rewritten in WP-M3-01 ([ADR-0049](0049-adapt-the-prototype-domain-and-ports-into-services-platform.md)), and its storage in WP-M3-02 ([ADR-0050](0050-reimplement-the-prototype-stores-as-postgresql-adapters.md)).
- **Mixed content in the source `app`.** It holds the coordinator, but also the delivery loop (`autoloop*`, `intake`, `localcheck`, 138 tests), inbound hooks (`hooks.go`, 814 lines), the provider registry and context assembly, and one use-case service per catalog (Agents, Skills, Teams, Work, Workflows, Playbooks, Projects, Settings). Most of these belong to other Work Packages.
- **Out of this ADR.** Process-tree cancellation and the Worker OS layer (`runtime/internal/platform`, `MADR-platform`, #78) and the remote-job providers (`runtime/internal/provider/remotejob`, `MADR-remotejob`, #79) have their own migration ADRs. The coordinator depends on the interface they implement, not on their code.
- **Unverified:** that the 112 non-loop tests cover the lifecycle, cancel, detach and approval-race paths the coordinator needs. The prototype never measured it, so the moved tests are extended, not trusted.

## Decision
1. **Component and source.** Migrate the coordinator files of `internal/app` at commit `c8a2f6e` (ADR-0032), as listed in item 4. No later commit is used.
2. **Disposition: Adapt, with the state model replaced.** The prototype is a behavioral reference and migration source. Its in-memory `record`, single mutex, goroutine-per-run, in-memory idempotency map, after-the-fact persistence and `runtime_restarted` rule are **not** carried over. The coordinator is written against the ADR-0042 model: state in PostgreSQL with optimistic versions, transitions committed with their outbox messages, timers and gates as rows, and recovery by reconciling non-terminal Steps at start. `services/platform` is canonical from the first commit; the prototype is not kept in sync.
3. **History option: fresh import with provenance** (migration plan default), as in ADR-0049: the source commit is recorded here, in `services/platform/README.md` and in `PROVENANCE.md` with each file's source path and blob id. No `git subtree` or `filter-repo`.
4. **File disposition.**

   | Source (`internal/app/`) | Disposition | Target / note |
   |---|---|---|
   | `manager.go`, `steps.go` | Adapt | Execution coordinator: create, run, finish, cancel, detach, resume. `PlanSteps`, `finalizeSteps`, capability checks, step input and output collection move as logic; `record`, `Restore` and `ResumeInterrupted` are rewritten for ADR-0042/0044 (FEAT-EXE-002 #129, EXE-003 #152, WFL-004 #212) |
   | `manager.go` idempotency (`idem`, `lookupIdem`) | Adapt | Idempotency keys persisted with the Execution request (FEAT-EXE-005 #157), not a process map |
   | `agentslots.go` | Adapt | Concurrency limits (FEAT-EXE-006 #158); slots are accounted in PostgreSQL so a restart does not leak them |
   | `eventbus.go` | Adapt | Per-Execution ordered Event log and relay (outbox). The in-memory fan-out becomes a subscriber of the relay. Event shape follows the contract, not the prototype |
   | `teamsteps.go` | Adapt | Review Step verdict, separation of duties and the Approval wait. The wait becomes a persisted gate; the Approval record and evidence binding are WP-M3-04 ([ADR-0036](0036-approvals-bind-to-evidence-and-gate-work.md), [ADR-0048](0048-approval-reuse-uncovered-capabilities-and-policy-defaults.md)), so only the coordinator side moves here |
   | `registry.go`, `context.go` | **Deferred** | Provider registry and context assembly belong to WP-M3-05 (Providers and context assembly, #17). The coordinator consumes them through a port; it does not move them |
   | `workflow.go`, `workflow_templates.go` | Adapt | Workflow definition service for FEAT-WFL-001 #138, versioning and run pinning #210, variables #211. Definitions live in Git ([ADR-0043](0043-project-definitions-in-git-teams-and-operational-state-in-platform.md)); templates stay as the Workflow template feature |
   | `agents.go`, `skills.go`, `teams.go`, `playbooks.go`, `catalogwatch.go` | **Deferred** | Catalog services for Agents, Skills, Teams and the Playbook (knowledge-note) index. Each moves with its own feature issue, not with the coordinator |
   | `work.go`, `artifacts.go`, `projects.go`, `settings.go` | **Deferred** | Work, Artifact, Project and settings use-cases; they sit over the stores from ADR-0050 and move with the Work, Project and configuration-scope features |
   | `hooks.go` | **Requires human decision** | Inbound hooks. The home in Amaru (Inbound webhook trigger WFL-009 #217, Policy and signing) is undecided; this ADR does not pre-empt it. Not moved until the owner chooses |
   | `autoloop*.go`, `intake*.go`, `localcheck.go` and their tests | **Deferred** | M4 Delivery loop, covered by `MADR-autoloop` (#87). Not moved by this ADR |
   | Source ADRs, dogfood records | Reference only | cited, not copied |

5. **The Worker interface.** The coordinator never executes a tool. It drives a Worker through a port owned by the Platform: start, cancel, detach, wait, resume, with an Event sink. The **embedded Worker** (FEAT-ENV-001 #145) is the in-process implementation and the only one built in M3; a remote Worker implements the same port later ([ADR-0013](0013-outbound-pull-based-workers-with-leases.md)). The port is defined with its first implementation, not ahead of it.
6. **Renames** follow the [glossary term map](../../product/GLOSSARY.md#2-term-mapping-from-predecessor-repositories): Runtime → Platform; `Manager` → Execution coordinator; per-agent Execution/Task → Step Execution; Autoloop → Delivery loop; Playbook (workflow sense) → Workflow template; `weave.*` identifiers and import paths → Amaru names. No `weave` or `orkestra` string remains in `services/platform` except in `PROVENANCE.md`. Go package layout is decided in the first implementation PR under the [repository layout](../../engineering/repository-layout.md) rules.
7. **Behavior-preserving tests that move.** The prototype tests of every file marked Adapt are the specification: `manager_test`, `steps_test`, `resume_test`, `persist_test`, `eventbus_test`, `agentslots_test`, `teams_approval_race_test` (15 + 11 + 2 + 3 + 6 + 6 + 5 = 48 functions) and the Workflow tests (`workflow_test`, `workflow_templates_test`, `graph_test`; 10 functions). They are reimplemented against the PostgreSQL-backed coordinator and a fake Worker, behind the `pg` tag where they need a database, in `make check` on Linux with macOS and Windows best-effort ([validation](../../engineering/validation.md#cross-platform-validation-m3-and-m4)). Tests whose behavior intentionally changes are listed in `PROVENANCE.md` with the reason; none is edited merely to pass. They change at least for: restart (`runtime_restarted` → ADR-0042 rule, `interrupted`, no automatic re-run), idempotency (persisted key), slots (persisted) and event ordering (log `seq`, not bus order). The crash-injection matrix of the M1 spike is ported as the WP-M3-03 acceptance suite.
8. **Gate.** No file from the source is added to this repository until this ADR is `accepted` with a recorded approval. Only then do #129, #152, #157, #158, #138, #210, #211, #212 and #145 start moving code. Merging this draft is not acceptance; issue #77 stays open until the decision.

## Readiness criteria (migration plan §5)
| # | Criterion | State | Evidence |
|---|---|---|---|
| 1 | Source side stable at `c8a2f6e` | **Met** | ADR-0032; commit verified; no later commit used |
| 2 | Inventory refreshed with tests run | **Met** | WP-M1-07 (#8) closed; `internal/app` tests re-run at `c8a2f6e` on 2026-10-27 (see Context) |
| 3 | Target ADRs accepted | **Met** | ADR-0013, 0036, 0042, 0043, 0044, 0048, 0049, 0050 accepted. The hooks and the deferred catalog files are excluded so that open decisions are not pre-empted |
| 4 | Contracts stable, with fixtures, in Amaru | **Met** | M2 exit validation (#12) closed; `packages/contracts` holds the Execution, Step and Event types |
| 5 | Behavior covered by tests that run in Amaru | **Met by plan** | Item 7; becomes evidence in the first implementation PRs |
| 6 | Verified execution | **Gap, listed** | The coordinator was exercised by its own tests and by the prototype Runtime, never by a recorded click-through. The durable design was verified by the M1 spike on its own code, not on this migration. Accepting this ADR accepts that gap |
| 7 | License resolved | **Met** | The source is MIT, © Ricardo Lüders, and the holder consents to relicensing ([ADR-0026](0026-fair-code-community-license-with-commercial-licensing.md)). No license file is added by this work |
| 8 | Explicit product-owner approval | **Not met** | Awaiting the product owner |

## Consequences
- **Easier:** the coordinator starts from a tested lifecycle (cancel races, detach, approval wake-ups, step ordering) instead of re-deriving it, and its durability comes from a design already validated by a crash-injection spike.
- **Harder:** the state model is rewritten, so review effort sits in the transaction boundaries, the reconcile at start and the test conversion, not in file moves. Concurrency moves from a mutex to optimistic versions and row claims, which changes how each prototype race test must be expressed.
- **Security and compatibility:** no secrets, credentials or personal data are transferred (scanned in each PR). Amaru does not promise wire or storage compatibility with the prototype (the owner decided on 2026-10-27 that there is no importer).
- **Maintenance:** the coordinator has a single owner (the Platform owner); the prototype copy is frozen and is not a second source of truth.
- **Follow-up:** after acceptance, update the migration inventory in the private notes and mark these files migrated (separate approval in the source repository). `hooks.go`, the deferred catalog files and the registry each need a decision or a migration ADR before they move. Revisit if the Worker port (item 5) turns out to need a shape the embedded Worker cannot satisfy.

## Alternatives considered
- **Port the manager unchanged, then add durability:** keeps the tests green sooner, but builds the in-memory model that ADR-0042 replaces and leaves the restart rule wrong for as long as it lasts.
- **Reimplement from the documents only (as for the stores):** loses the cancel, detach and approval-race behavior the prototype tests pin down. The coordinator is logic-heavy, unlike the stores, so the source has more to give.
- **Move the whole `app` package as one unit:** pulls M4 delivery-loop logic, undecided hooks and every catalog service into M3, contradicting the WP scope and pre-empting the Policy and Provider decisions.
- **Adopt an external workflow engine:** rejected earlier by ADR-0014/ADR-0042; not reopened here.

## Approval
Proposed by: Claude (migration ADR draft for #77, 2026-10-27). Not yet approved. Acceptance needs an explicit, recorded decision by the product owner; the **Deferred** and **Requires human decision** rows of item 4 and the criterion 6 gap are the points to review.
