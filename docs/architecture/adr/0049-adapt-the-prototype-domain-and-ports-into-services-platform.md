---
title: Adapt the prototype domain, ports and architecture test into services/platform by fresh import with provenance
status: accepted
date: 2026-10-26
deciders: Ricardo Lüders (product owner)
---

# 0049. Adapt the prototype domain, ports and architecture test into services/platform by fresh import with provenance

## Context
- [ADR-0007](0007-reuse-of-earlier-internal-code-is-a-separate-activity.md) bars moving code from the earlier internal implementation until a human approves a migration ADR for that component. This is the migration ADR for its `runtime/internal/domain`, `runtime/internal/ports` and `runtime/internal/arch` directories (`MADR-domain`, issue #75, sub-issue of WP-M3-01 / #13). Every issue that moves this component is gated on it ([delivery §8](../../engineering/delivery.md#8-human-approval-gates)).
- **Source (verified 2026-10-26):** the Runtime repository of the earlier internal implementation (named in the private migration notes, not part of this repository), at the baseline commit **`c8a2f6e`** pinned by [ADR-0032](0032-migration-baseline-is-a-pinned-commit.md). From a `git archive` of that commit:

  | Directory | Non-test Go files | Lines | Notes |
  |---|---|---|---|
  | `internal/domain` | 25 (+ 11 test files) | 3,713 | imports only the standard library (`crypto/sha256`); 29 test functions (test files total 680 lines) |
  | `internal/ports` | 1 (`ports.go`) | 431 | imports only `domain` |
  | `internal/ports/fakes` | 1 (`fakes.go`) | 97 | in-memory fakes; imports only `ports` |
  | `internal/arch` | 0 (one `arch_test.go`, 3 tests) | — | parses imports with `go/parser`; forbids `os/exec`, `syscall`, providers, platform and store packages from the core |

  `go test -count=1 ./internal/domain/... ./internal/ports/... ./internal/arch/...` passes on the extracted tree (2026-10-26, Linux). `ports` and `fakes` have no tests of their own.
- **Planned disposition** (migration plan, `MADR-domain`): **Adapt** into `services/platform` in M3, as the pure domain core ([AGENTS.md architecture boundaries](../../../AGENTS.md#architecture-boundaries-accepted-not-implemented-see-architecture): domain code has no OS, process, network or vendor imports).
- **Not a straight port.** The prototype Execution and Step lifecycle predates [ADR-0042](0042-durable-execution-with-explicit-restart-semantics.md)/[ADR-0044](0044-restart-semantics-clarifications-from-the-m1-spike.md), Approvals ([ADR-0036](0036-approvals-bind-to-evidence-and-gate-work.md), [ADR-0048](0048-approval-reuse-uncovered-capabilities-and-policy-defaults.md)) and the Organization scope. WP-M3-01 adds the states listed in [DOMAIN_MODEL §5](../DOMAIN_MODEL.md#5-lifecycles); those are written in Amaru, not migrated.
- **Mixed content in the source `domain`:** it also holds delivery-loop logic (`autoloop.go`, `intake.go`, `localcheck.go`, `commit.go`) that belongs to M4, and shapes whose Amaru home is undecided (`hook.go`, `trust.go`, `grant.go`, `settings.go`). WP-M3-01 puts "delivery-loop changes (M4)" out of scope.
- **Unverified:** that the 29 prototype domain tests cover the transition tables. The prototype never measured it; the ported transition tests are extended, not trusted.

## Decision
1. **Component and source.** Migrate `internal/domain`, `internal/ports` (with `fakes`) and `internal/arch` at commit `c8a2f6e` (ADR-0032). No later commit is used.
2. **Disposition: Adapt.** The prototype is a behavioral reference and migration source. `services/platform` is canonical from the first commit; the prototype directories are not kept in sync (fixes go to Amaru).
3. **History option: fresh import with provenance** (migration plan default). No `git subtree`/`filter-repo`: a single sole-copyright-holder source whose blame adds nothing to review. Provenance: the source commit is recorded in this ADR, in `services/platform/README.md`, and in a `PROVENANCE.md` listing each transferred file with source path and blob id. Tagging the source commit happens only with the owner's separate approval in the source repository (ADR-0007 rule 1).
4. **File disposition.**

   | Source | Disposition | Target / note |
   |---|---|---|
   | `domain/{execution,step}.go` | Adapt | Execution/Step state machines (FEAT-EXE-001, #118) with the new states and the ADR-0042/0044 restart rule; transition tables are rewritten as data-driven tests |
   | `domain/{work}.go` | Adapt | Work Packages and Work Items (FEAT-WRK-001, #166); dependency graph and ordering for FEAT-WRK-002 (#218) |
   | `domain/{agent,skill}.go` | Adapt | Agent definitions (FEAT-AGT-001, #110); Agent and Skill snapshots per Step (FEAT-AGT-004, #114) |
   | `domain/{capabilities,provider}.go` | Adapt | typed Capability catalog (FEAT-CAP-001, #111); Workflows reference Capabilities, never vendors |
   | `domain/{workflow,workflow_template,team,project,artifact,output,errors,doc}.go` | Adapt | renamed per item 5; Project keyed per [ADR-0046](0046-project-key-is-the-normalized-remote-plus-root-commit.md); Organization as an implicit scope |
   | `domain/event.go` | Adapt | aligned with the change-event contract in `packages/contracts` (WRK-003), not with the prototype event shape |
   | `domain/{autoloop,intake,localcheck,commit}.go` and their tests | **Deferred** | M4 Delivery loop; covered by `MADR-autoloop` (#87) or its own migration ADR. Not moved by this ADR |
   | `domain/{hook,trust,grant,settings,playbook}.go` | **Requires human decision** | home in Amaru is undecided (Policy/Approval model, plugin hooks, knowledge notes). Not moved until the owner chooses; this ADR does not pre-empt ADR-0037 or ADR-0038 |
   | `ports/ports.go` | Adapt | ports used by the Platform; each port is reviewed against the boundary rules (Platform owns state, Workers hold none). Ports for removed concerns (Obsidian vault, per-user daemon) are dropped |
   | `ports/fakes/fakes.go` | Adapt | in-memory fakes for domain and coordinator tests (no real agents or paid APIs) |
   | `arch/arch_test.go` | Adapt | rewritten for the Amaru layout: pure domain; no `os/exec`, `syscall`, network or vendor imports; providers depend only on ports and domain. Extended to run on three OSes |
   | Source ADRs, dogfood records | Reference only | cited, not copied |

5. **Renames** follow the [glossary term map](../../product/GLOSSARY.md#2-term-mapping-from-predecessor-repositories): Workspace (agent worktree) → Worktree; Workspace (tenant) → Organization; parent Execution as is, per-agent Execution/Task → Step Execution; Autoloop → Delivery loop; Playbook (workflow sense) → Workflow template; `weave.*` identifiers and import paths → Amaru names. No `weave`/`orkestra` string remains in `services/platform` except in `PROVENANCE.md`. Go module path and package layout are decided in the first implementation PR under the [repository layout](../../engineering/repository-layout.md) rules, not here.
6. **Behavior-preserving tests that move.** The prototype tests of every file marked Adapt (agent, artifact path cleaning, execution transitions, step lifecycle and settle, work item/package validation, workflow and team validation, graph ordering and catalog validity, skill reference parsing) and the three architecture tests (`TestCoreIndependentOfProvidersAndOS`, `TestProvidersDependOnlyOnPortsAndDomain`, `TestCoreDoesNotImportMCP`). Tests whose behavior intentionally changes (new states, ADR-0042 restart rule) are listed in `PROVENANCE.md` with the reason; none is edited merely to pass. They run in `make check` on Linux, macOS and Windows with fakes only.
7. **Gate.** No file from the source is added to this repository until this ADR is `accepted` with a recorded approval. Only then do #118, #166, #218, #110, #114 and #111 start. Merging this draft is not acceptance; issue #75 stays open until the decision.

## Readiness criteria (migration plan §5)
| # | Criterion | State | Evidence |
|---|---|---|---|
| 1 | Source side stable at `c8a2f6e` | **Met** | ADR-0032; commit verified; no later commit used |
| 2 | Inventory refreshed with tests run | **Met** | WP-M1-07 (#8) closed; domain, ports and arch tests re-run at `c8a2f6e` on 2026-10-26 (see Context) |
| 3 | Target ADRs accepted | **Met** | ADR-0009, 0036, 0037, 0038, 0042, 0044, 0046, 0048 accepted. The deferred and undecided files in item 4 are excluded precisely so that open decisions are not pre-empted |
| 4 | Contracts stable, with fixtures, in Amaru | **Met** | M2 exit validation (#12) closed; `packages/contracts` holds the shared types and fixtures the domain must agree with |
| 5 | Behavior covered by tests that run in Amaru | **Met by plan** | Item 6; becomes evidence in the first implementation PRs |
| 6 | Verified execution | **Gap, listed** | The domain is a library; the prototype exercised it only through its own tests and the Runtime, never a recorded click-through. Accepting this ADR accepts that gap |
| 7 | License resolved | **Met** | The source is MIT, © Ricardo Lüders, and the holder consents to relicensing ([ADR-0026](0026-fair-code-community-license-with-commercial-licensing.md)). No license file is added by this work |
| 8 | Explicit product-owner approval | **Met** | Accepted by the product owner on 2026-10-26 (see Approval) |

## Consequences
- **Easier:** a reviewed, tested pure core exists before storage (WP-M3-02) and the coordinator (WP-M3-03) depend on it; the architecture test locks the boundary from day one.
- **Harder:** the new states and the restart rule make this more than a copy; review effort sits in the state machines, not the file moves.
- **Security and compatibility:** no secrets, credentials or personal data are transferred (scanned in each PR). Amaru does not promise wire or storage compatibility with the prototype; the shared types come from `packages/contracts`.
- **Maintenance:** `services/platform` core has a single owner (the Platform owner); the prototype copy is frozen and not a second source of truth. Drift between domain types and contracts is guarded by the M2 fixture checks.
- **Follow-up:** after acceptance, update the migration inventory in the private notes and mark these directories migrated (separate approval in the source repository). The deferred and undecided files each need a decision or migration ADR before they move. Revisit if a later source commit changes these directories before the migration lands.

## Alternatives considered
- **History-preserving import:** keeps blame, but one holder authored the code; it enlarges the repository for no review benefit.
- **Reimplement from the documents only:** loses the validated behavior and its 29 domain tests, and the architecture test.
- **Move the whole `domain` package as one unit:** pulls M4 delivery-loop logic and undecided shapes into M3, contradicting the WP scope and risking pre-empting Policy/Approval decisions.

## Approval
Proposed by: Claude (migration ADR draft for #75, 2026-10-26). Approved by: Ricardo Lüders (product owner) on 2026-10-26, in the WP-M3-01 session after reviewing the draft ("Approved"). The Deferred and Requires-human-decision rows of item 4 were reviewed as drafted and stay unmoved; the criterion 6 gap is accepted with the ADR.
