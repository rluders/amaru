# Migration Plan: Weave → Amaru

Status: **plan only.** No code, history, branch or data has been migrated, and none will be without explicit approval ([ADR-0007](../architecture/adr/0007-weave-migration-is-a-separate-activity.md), accepted).

## 1. Four separate things

| | What | Where | Now |
|---|---|---|---|
| **A. Weave** | The current implementation and its historical identity (Weave/Orkestra): Runtime `weaved`, Obsidian plugin, protocol, ADR-001…020, validation records. | `rluders/weave-obsidian` | Active, continues its own cycles. |
| **B. Amaru foundation** | Product scope, architecture, decisions, governance, tooling. | `rluders/amaru` (this repo) | Stage 0. |
| **C. Weave completion** | Finishing Weave's outstanding cycles (validation, proposed ADRs, Windows/macOS, history hygiene). | `rluders/weave-obsidian` | [ROADMAP stage 1](../architecture/ROADMAP.md#stage-1--complete-the-existing-runtime-cycles). |
| **D. Migration** | Moving selected code and behavior from A into B. | Both | Not started; gated by §5. |

A and C never depend on B. B may cite A as evidence but never claims A's behavior as its own.

## 2. Decision criteria per component

Each component gets one disposition, recorded in a migration ADR before anything moves:

| Disposition | When | Typical evidence |
|---|---|---|
| **Reuse** (move as is, rename) | Fits the accepted Amaru architecture and contracts; well tested; no Obsidian or Weave-only assumptions. | Tests pass unchanged after the move; architecture test still holds. |
| **Adapt** (move, then change) | Sound core, but boundaries differ (e.g. Platform/Worker split, Policy hooks, storage ports). | Behavior-preserving tests written first, kept green through the change. |
| **Rewrite** (new code, old tests as spec) | The accepted architecture contradicts the design (e.g. persistence replaced by Q1/Q3 outcome). | Weave tests or click-through records translated into Amaru acceptance tests. |
| **Retire** (do not migrate) | Obsidian-specific UI, superseded features, providers without users. | Note why; history stays in Weave. |

## 3. Likely boundaries (hypotheses, to be confirmed)

| Weave component | Likely Amaru home | Likely disposition | Depends on |
|---|---|---|---|
| `protocol/` (OpenAPI, schemas, fixtures) | `packages/contracts` | Adapt (rename `weave.*`, add Policy/Approval, Worker protocol) | Q11, ADR-0004 |
| `runtime/internal/domain`, `ports` | Platform core | Reuse or adapt | Domain model review |
| `runtime/internal/app` (manager, steps, context assembly) | Platform execution coordinator | Adapt (Platform/Worker split, Policy, Approval) | Q1, ADR-0006 |
| `runtime/internal/store/*` (files) | Platform storage adapter (local) | Adapt or rewrite | Q3 |
| `runtime/internal/provider/*` | Worker providers / provider plugins | Reuse per provider | Plugin model (ADR-0005) |
| `runtime/internal/platform` (process, paths) | Worker OS layer | Reuse | Windows validation (GAP-05) |
| `runtime/internal/transport/httpapi`, `webhook` | Platform API | Adapt | Contracts stage |
| `runtime/internal/{auth,discovery}` | Local deployment adapter | Reuse for local | Identity design |
| `plugin/` (Obsidian) | Stays in Weave, or becomes an Amaru editor integration | Decide later | FR-41 client decision |
| `plugin/src/ui/flow` (React Flow editor), viewer | `packages/ui` candidates | Adapt | First client |
| Weave ADRs, click-through records, investigations | Cited from Amaru; copied only if history import is chosen | Keep in Weave | §4 |
| `.claude/skills/*` | Amaru `.agents/skills/` equivalents | Rewrite for Amaru layout | — |

## 4. Preserving history and evidence

Options, to be chosen by the product owner at migration time (not now):
1. **History-preserving import** (`git subtree add` or `git filter-repo` into a path): keeps blame and commit history for moved code; larger repo; Weave commit messages and names carry over.
2. **Fresh import with provenance**: copy at a tagged Weave commit, record the source commit in the migration ADR and file headers where useful; history stays readable in Weave.

Either way: tag the Weave commit used (`weave-migration-<component>`), keep Weave's ADRs and validation records reachable, and port the tests with the code.

Licensing must be settled first: Weave is MIT (© 2026 Ricardo Lüders); Amaru's license is undecided ([Q12](../architecture/OPEN_QUESTIONS.md#q12-licensing-and-commercial-boundaries)). Moving MIT code into a differently licensed repository requires keeping the MIT notice for that code or relicensing by the copyright holder, and checking for any third-party contributions.

## 5. Readiness criteria

Migration of a component may start only when **all** hold:

1. **Weave side done**: Weave stage 1 exit criteria met for that component (no IN PROGRESS item it depends on; relevant proposed ADRs decided).
2. **Inventory refreshed**: [weave-inventory](weave-inventory.md) updated to the commit being migrated.
3. **Target decided**: the Amaru ADRs the component depends on are accepted (e.g. contracts, persistence, Platform/Worker boundary).
4. **Contracts stable**: the component's contracts exist in Amaru with fixtures (Stage 3).
5. **Test coverage**: the behavior to preserve (inventory § "Behavior a migration must preserve") is covered by tests that will run in Amaru.
6. **Verified execution**: real-validated behavior (click-through or recorded run) exists for what is migrated, or the gap is listed and accepted.
7. **License resolved** (§4).
8. **Explicit approval**: the product owner approves a migration ADR naming the component, the Weave commit, the disposition and the history option.

## 6. Procedure per component (once approved)

1. Branch in Amaru; write or port the acceptance tests first.
2. Move/adapt code; rename identifiers per the [glossary](../product/GLOSSARY.md#term-mapping-from-weave).
3. `make check` green; record what was not validated.
4. Update the inventory and mark the component migrated in Weave's docs (in a Weave change, approved there).
5. Decide whether Weave keeps maintaining the component or freezes it.

## 7. Current assessment

**Not ready.** Blocking: Weave cycles outstanding (stage 1), no accepted Amaru contracts or persistence decision, license undecided, Windows/macOS unvalidated in Weave. The Weave Runtime is nevertheless the strongest evidence base for the Platform core: its Capability→Provider boundary, contract discipline and restart semantics are tested and partly real-validated.
