# Delivery Workflow

How planned work becomes GitHub issues, branches and pull requests. The plan itself lives in the [master roadmap](../architecture/ROADMAP.md), the [Work Packages](../architecture/WORK_PACKAGES.md) and the [feature catalog](../product/FEATURE_CATALOG.md); GitHub tracks its execution. The mapping from planning IDs to issues is in the [backlog map](backlog-map.md).

## 1. Sources and planning IDs
Every issue carries exactly one planning ID, in a hidden first-line marker `<!-- amaru-backlog-id: <ID> -->` and in its **Planning ID** line. The marker is how the backlog is resumed or audited without duplicates: search for it before creating an issue.

| ID form | Source | Kind |
|---|---|---|
| `WP-M<m>-<nn>` | WORK_PACKAGES | Work Package (milestones M0–M4) |
| `E<m>.<n>` | ROADMAP epic list | Epic (milestones M5–M9) |
| `<AREA>-<nnn>` | FEATURE_CATALOG (`FEAT-` prefix dropped) | Feature |
| `MADR-<component>` | MIGRATION_PLAN §3 | Migration ADR to draft before code moves ([ADR-0007](../architecture/adr/0007-weave-migration-is-a-separate-activity.md)) |
| `SPK-<topic>`, `SEC-<topic>` | ROADMAP critical path, validation register | Spike or review with a hypothesis and follow-up decision |
| `OWN-<topic>` | ROADMAP M1 license tasks | Product-owner task |
| `EXIT-M<m>` | ROADMAP exit criteria | Milestone exit record (M4 uses WP-M4-07) |

Each scheduled feature has one issue, or is folded into a Work Package or epic issue that covers only that feature. P2 features in the backlog and all P3 features have no issue until a milestone schedules them.

## 2. Issue structure
Required sections, in order: Problem and outcome · Scope · Out of scope · Acceptance criteria · Sub-issues (parents only) · Dependencies · Verification · Definition of done · Delivery. Acceptance criteria are copied from the source document (WP *Done when*, catalog *Acceptance*, roadmap exit criteria) as a checklist; when the source changes, update the issue in the same change.

Each issue gets one GitHub milestone (`M0` … `M10`, no due dates) and labels:

| Label | Use |
|---|---|
| `work-package`, `epic`, `feature` | Planning kind |
| `spike` | Validation experiment or review; its result is a report under `docs/architecture/investigations/` |
| `migration-adr` | Drafting a migration ADR (status `proposed`; only the product owner accepts) |
| `owner-task` | Human-only work (license, legal, trademark, ADR acceptance); agents do not do it |
| `milestone-exit` | Milestone exit evidence |
| `P0`, `P1`, `P2` | Catalog priority, on feature issues |

## 3. Parents and sub-issues
- A Work Package or epic with two or more independently verifiable parts is a **parent** with native GitHub sub-issues; otherwise it is a direct issue.
- A parent defines the whole deliverable and its integration acceptance. Closing every sub-issue is necessary but not sufficient: the integrated result must also meet the parent's criteria.
- A sub-issue inherits every blocker of its parent; those are not repeated as native links.
- The feature-to-epic split for M5–M9 is **proposed** (the roadmap lists epics and milestone features, not the mapping). Confirm it when the milestone's Work Packages are written.

## 4. Dependencies
- Native GitHub **blocked by** relationships are authoritative. Milestone order is not a dependency.
- Links are transitively reduced: an edge implied by another blocker is listed in the body under "Also required", not linked.
- A milestone that depends on another is blocked by that milestone's exit issue.
- Where a catalog dependency conflicts with the accepted Work Package order, the issue records it as a **soft dependency**: build against the contract or a fake and integrate when the other issue lands.
- Do not start an issue whose blockers are open. Issues with no open blockers in the same milestone may run in parallel.

## 5. Branches and pull requests
Branch names: `<prefix>/<issue-number>-<slug>`, with prefix `feat` (implementation), `fix` (defects), `docs` (migration ADRs, exit records, documentation) or `spike` (experiments and reviews). Example: `feat/123-plugin-host`.

**Parent with sub-issues**
1. Create the parent's integration branch from `main` when its first sub-issue starts, for example `feat/120-plugin-host`.
2. Each sub-issue gets its own branch from the integration branch, for example `feat/123-plugin-host-tests`, and its PR targets the integration branch, never `main`.
3. Review, validate and merge each sub-issue PR into the integration branch. Close the sub-issue by hand with a link to the merged PR: `Closes #n` acts only on merges into `main`.
4. Keep the integration branch current by merging `main` into it (never rebase a shared branch).
5. When all sub-issues are merged, validate the combined result against the parent's acceptance criteria and open one integration PR from the integration branch to `main`. It is a separate review boundary: approval of child PRs does not replace it.
6. Merge only when the parent's acceptance criteria and required checks pass. Close the parent after the merge, then delete the integration branch.

**Direct issue**
1. Branch from `main` (or from an active integration branch, if the issue says it depends on one).
2. Open a PR to that base that says `Closes #n`; merge after review and checks.

## 6. Evidence and definition of done
- Every PR states what changed, the exact commands run and their outcomes, and what was not validated ([validation](validation.md)). No hosted CI exists yet; until it does, the required check is `make check` run locally and reported, and the PR must say so.
- Evidence levels: unit-tested, integration-tested, real-validated, not validated. A fake-backed test is not real validation; recorded runs name the date, OS and versions.
- Done means: acceptance criteria checked with linked evidence; `make check` passes; affected docs updated in the same change; no secrets; merged into the designated base.
- Spikes close with a report and the follow-up decision taken (proceed, or a superseding ADR drafted as `proposed`).
- An issue never claims behavior that the merged code does not have ([status vocabulary](CONVENTIONS.md#status-vocabulary)).

## 7. Traceability and change
- New planned work starts in the planning documents, then gets an issue with a planning ID; update the backlog map in the same change.
- When a Work Package, feature or exit criterion changes, update its issue body and the backlog map. Do not renumber planning IDs.
- When M4 nears, M5–M10 epics are decomposed into Work Packages in WORK_PACKAGES; existing epic and feature issues are reused, not duplicated.
