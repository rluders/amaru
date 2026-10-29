# Delivery Workflow

How planned work becomes GitHub issues, branches and pull requests. The plan itself lives in the [master roadmap](../architecture/ROADMAP.md), the [Work Packages](../architecture/WORK_PACKAGES.md) and the [feature catalog](../product/FEATURE_CATALOG.md); GitHub tracks its execution. The mapping from planning IDs to issues is in the [backlog map](backlog-map.md).

## 1. Sources and planning IDs
Every scheduled planning issue carries exactly one planning ID, in a hidden first-line marker `<!-- amaru-backlog-id: <ID> -->` and in its **Planning ID** line. The marker is how the backlog is resumed or audited without duplicates: search for it before creating an issue. Incoming reports may be unassigned during triage; use the issue number as their identity until a source-backed planning ID exists. Explicitly canceled work need not acquire a new planning ID just to normalize history.

| ID form | Source | Kind |
|---|---|---|
| `WP-M<m>-<nn>` | WORK_PACKAGES | Work Package (milestones M0–M4) |
| `E<m>.<n>` | ROADMAP epic list | Epic (milestones M5–M9) |
| `<AREA>-<nnn>` | FEATURE_CATALOG (`FEAT-` prefix dropped) | Feature |
| `MADR-<component>` | migration plan | Migration ADR to draft before code moves ([ADR-0007](../architecture/adr/0007-reuse-of-earlier-internal-code-is-a-separate-activity.md)) |
| `SPK-<topic>`, `SEC-<topic>` | ROADMAP critical path, validation register | Spike or review with a hypothesis and follow-up decision |
| `OWN-<topic>` | ROADMAP M1 license tasks | Product-owner task |
| `EXIT-M<m>` | ROADMAP exit criteria | Milestone exit record (M4 uses WP-M4-07) |

Each scheduled feature has one issue, or is folded into a Work Package or epic issue that covers only that feature. P2 features in the backlog and all P3 features have no issue until a milestone schedules them.

## 2. Issue structure
Required information, in order: Problem and outcome · Scope · Out of scope · Acceptance criteria · Sub-issues (parents only) · Dependencies · Verification · Definition of done · Delivery. The recurring issue forms group related information into fewer fields; all of this information must still be present before Ready. Acceptance criteria are copied from the source document (WP *Done when*, catalog *Acceptance*, roadmap exit criteria) as a checklist; when the source changes, update the issue in the same change.

Each issue gets one GitHub milestone (`M0` … `M10`, no due dates) and labels:

| Label | Use |
|---|---|
| `work-package`, `epic`, `feature` | Planning kind |
| `spike` | Validation experiment or review; its result is a report under `docs/architecture/investigations/` |
| `migration-adr` | Drafting a migration ADR (status `proposed`; only the product owner accepts) |
| `owner-task` | Human-only work (license, legal, trademark, ADR acceptance); agents do not do it |
| `awaiting-owner-approval` | The issue closes only on a recorded product-owner decision ([§8](#8-human-approval-gates)); on every migration ADR issue and on the license decisions |
| `milestone-exit` | Milestone exit evidence |
| `P0`, `P1`, `P2` | Catalog priority, on feature issues |

## 3. Parents and sub-issues
- A Work Package or epic with two or more independently verifiable parts is a **parent** with native GitHub sub-issues; otherwise it is a direct issue.
- A parent defines the whole deliverable and its integration acceptance. Closing every sub-issue is necessary but not sufficient: the integrated result must also meet the parent's criteria.
- A sub-issue inherits every blocker of its parent; those are not repeated as native links.
- The feature-to-epic grouping for M5–M9 is listed per epic in the [roadmap](../architecture/ROADMAP.md) (confirmed 2026-10-25). Each feature has exactly one owning epic; when the milestone's Work Packages are written, they reuse those issues.

## 4. Dependencies
- Native GitHub **blocked by** relationships are authoritative. Milestone order is not a dependency.
- Links are transitively reduced: an edge implied by another blocker is listed in the body under "Also required", not linked.
- A milestone that depends on another is blocked by that milestone's exit issue.
- Catalog dependencies are hard unless the catalog marks them **(soft)** ([catalog §1](../product/FEATURE_CATALOG.md#1-how-to-read-this-catalog)). A hard dependency is a native link, and if it runs against the Work Package order, the order changes; Work Package order never turns a hard dependency into a soft one.
- A soft dependency is not linked natively. The issue names it with its reason: the feature is built and accepted against an existing port or contract with fakes, and the other feature is needed only for the real-validation run, which is scheduled in an issue that depends on both. A soft dependency never waives integration testing or the parent Work Package's acceptance criteria. Current resolutions: [WORK_PACKAGES §Cross-package feature dependencies](../architecture/WORK_PACKAGES.md#cross-package-feature-dependencies).
- A blocker that is a sub-issue of another parent is satisfied only when its code is on the base the dependent issue branches from: normally after that parent's integration PR merges into `main`. Starting earlier requires branching from that integration branch, stated in the issue.
- No issue may wait on an issue that itself waits on it, directly or through parents (a sub-issue inherits its parent's blockers): check this before adding a link.
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

**Approval-gated issues** (`awaiting-owner-approval`): the PR says `Refs #n`, never `Closes #n`, because merging a draft is not the decision. Close the issue by hand only after the decision is recorded ([§8](#8-human-approval-gates)).

## 6. Evidence and definition of done
- Every PR states what changed, the exact commands run and their outcomes, and what was not validated ([validation](validation.md)). No general hosted CI exists yet; the scoped M1 spike workflow supplies only its named evidence. The required general check is `make check` run locally and reported, and the PR must say so. Where a Work Package or milestone exit needs Linux, macOS and Windows, follow the [cross-platform validation](validation.md#cross-platform-validation-m3-and-m4) rules and its recorded owner deferrals.
- Evidence levels: unit-tested, integration-tested, real-validated, not validated. A fake-backed test is not real validation; recorded runs name the date, OS and versions.
- Done means: acceptance criteria checked with linked evidence; `make check` passes; affected docs updated in the same change; no secrets; merged into the designated base.
- Spikes close with a report and the follow-up decision taken (proceed, or a superseding ADR drafted as `proposed`).
- An issue never claims behavior that the merged code does not have ([status vocabulary](CONVENTIONS.md#status-vocabulary)).

## 7. Traceability and change
- New planned work starts in the planning documents, then gets an issue with a planning ID; update the backlog map in the same change.
- When a Work Package, feature or exit criterion changes, update its issue body and the backlog map. Do not renumber planning IDs.
- When M4 nears, M5–M10 epics are decomposed into Work Packages in WORK_PACKAGES; existing epic and feature issues are reused, not duplicated.

## 8. Human approval gates
Some decisions belong only to the product owner (AGENTS.md hard rules 3 and 9). Their issues carry `awaiting-owner-approval`; agents may draft and prepare, never decide, accept or close.

| Gate | Issues | Closes when | Blocks |
|---|---|---|---|
| Migration ADR accepted ([ADR-0007](../architecture/adr/0007-reuse-of-earlier-internal-code-is-a-separate-activity.md), readiness criteria) | every `MADR-*` issue (see [backlog map](backlog-map.md)) | the ADR is merged with status `accepted` and a recorded approval. "Changes requested" keeps the issue open. | every issue that moves code of that component (native link, or through a parent) |
| License legal review | `OWN-legal-review` | closed 2026-10-26: no separate legal review (owner decision, [WP-M1-01](../architecture/WORK_PACKAGES.md#wp-m1-01-license-texts-in-effect)) | — |
| Root `LICENSE` in effect ([ADR-0026](../architecture/adr/0026-fair-code-community-license-with-commercial-licensing.md)) | `OWN-license-files` | the owner commits the root `LICENSE` | the first public release and the first external contribution; not migration (decided 2026-10-26) |
| Apache-2.0 `LICENSE` per contract or SDK package | the issue that creates the package (WP-M2-01, WP-M2-02, E6.3; E6.6 for the Go Worker SDK) | the owner commits the file in that package | that issue |
| CLA | `OWN-cla` | closed 2026-10-26: the owner approved the CLA text and the manual signing process ([CLA](../../CLA.md), [CONTRIBUTING](../../CONTRIBUTING.md#contributor-license-agreement)) | the first external contribution |

An issue of a gate is never closed because a PR merged, an issue was created, or a link is missing. Any new issue that moves prototype code gets a native link to its migration ADR issue (or sits under a parent that has one) when it is created.

## 9. Project and operating model

[Amaru — Roadmap and Delivery](https://github.com/users/rluders/projects/2) is the private operational Project for `rluders/amaru`, owned by `rluders` and linked to the repository. Its privacy is independent of repository visibility. The roadmap, Work Packages, feature catalog and ADRs remain authoritative; the Project does not create a second plan. Its README links every milestone's exit record.

Milestones bound outcomes; Work Packages and epics group delivery; features/tasks make acceptance actionable; native parent/sub-issue links provide decomposition; native blocked-by links provide prerequisites; PRs record implementation and integration. M0–M4 use the established Work Packages and later milestones use epics until refined. Do not add an artificial epic layer above every Work Package.

The one custom field is **Status**. Milestone, assignees, parent, sub-issue progress, linked PRs and reviewers use GitHub metadata. There are no estimates or invented start/target dates. The Roadmap view shows planned sequence, not a committed calendar schedule.

| Status | Entry and exit rules |
|---|---|
| Backlog | Future work or work needing refinement. Missing prerequisites are expected here; this is not an assertion that every future issue is at risk. |
| Ready | Scope, acceptance and verification are actionable; applicable owner decisions and direct/inherited blockers are satisfied; code is available on the required base. A parent may be Ready to coordinate executable children; its children still gate completion. |
| In Progress | Someone is actively implementing the issue. A branch alone is not evidence that work is active. |
| Blocked | Selected delivery work cannot advance. Record the blocker, responsible owner, next action and milestone in the issue, and add `needs-attention`. |
| In Review | Implementation awaits required review or integration. A child merge and the parent's integration review are separate boundaries. |
| Done | Acceptance and verification are evidenced, the designated-base integration is complete, and applicable owner approval is recorded. Closed issue state alone is insufficient. |

Closed `not planned` issues have no Done status and are excluded from delivery/roadmap views. Preserve their closure reason and discussion. A duplicate points to its canonical issue; deferred work returns to Backlog with its rationale and destination preserved. Reopening work requires a reason and immediate Project reconciliation; reopen an incomplete parent when required children remain open. Never discard a valid child merge or silently reverse an owner-approved exception.

### Classification and priority

Classification uses labels on this personal repository. The issue-types endpoint lists Feature, Bug and Task, but assignment returned `issueType: null` when re-queried on 2026-10-29; native types are therefore **not configured**. Preserve the established planning-role labels instead of claiming unsupported types work. A Work Package is not interchangeable with an engineering task. If native types become usable later, verify assignment/readback before changing this convention.

- Planning roles: `work-package`, `epic`, `feature`, `task`, `spike`, `migration-adr`, `owner-task`, `milestone-exit`. Use `task` for standalone engineering work, not as a replacement for all other roles.
- Governance: `awaiting-owner-approval` is a human gate; `needs-attention` is an actionable blocker, risk or evidence discrepancy with an issue-body explanation, responsible owner, next action and affected milestone.
- Catalog priority: `P0` means required for the Local MVP; `P1` means the first multi-environment/team-capable product; `P2` means Cloud, ecosystem or improvements. These labels are product scope, **not incident severity or a claim of urgency**. Keep the catalog assignment; do not label every Task P0.
- Repository-wide handling: `bug`, `documentation`, `accessibility`, `duplicate`, `invalid`, `wontfix`, `question`, `enhancement`, `good first issue`, `help wanted` retain their existing meanings where applicable. Do not bulk-replace meaningful labels or add speculative area labels.

Order Ready work manually by current milestone, prerequisite impact, then catalog priority. Re-evaluate after each integration or new blocker. No second priority field duplicates the catalog.

### Daily execution and views

| View | Use |
|---|---|
| Executive Overview | Work Packages, epics and exit records across milestones; inspect status, attention and child progress. |
| Delivery Board | Open selected work in Ready, In Progress, Blocked and In Review. Backlog is deliberately elsewhere. |
| Roadmap | Ordered M0–M10 outcomes and parent/progress metadata; no fabricated timeline dates. Visual hierarchy activation is a documented UI follow-up. |
| Milestone Planning | Initially M3; change the milestone filter to inspect all its work and its exit issue. |
| Backlog and Triage | Refine future work; distinguish definition gaps from satisfied readiness. |
| Blocked and At Risk | `needs-attention` items; read the issue for blocker, owner and next action. |
| PR and Integration | Open PRs, including drafts; inspect native review/check/base information in the linked PR. A merged child does not complete its parent. |

At daily triage, check the current exit record, Ready queue, attention view and open PRs. Review new reports through the issue forms; require a source-backed scope, acceptance criteria, verification, milestone/parent where appropriate and applicable native dependencies before Ready. Forms cannot interpolate a hidden first-line planning marker: the maintainer adds it during triage and updates the backlog map. Do not invent a planning ID for an unscheduled report.

Start work by checking inherited blockers and the integration base, assigning a responsible contributor and setting In Progress. Opening a non-draft PR moves the associated issue to In Review after the contributor confirms its scope is ready; drafts stay In Progress. Follow §5 for child and integration PRs. After merge, review the evidence before setting Done; human decision issues always retain their approval gate.

Plan a milestone by reviewing its objective, exit criteria, carry-over conditions, issue membership and prerequisites. Close it only after its exit record links evidence or an explicit recorded owner-approved resolution. Accepted deferrals remain visible in the destination issues and roadmap; they are not checked as validated. M1 and M2 have qualified exits; M3 is the current delivery milestone.

### Automation and measurement

The live configuration and its limitations are recorded in the [management validation record](../validation/management/2026-10-29-github-management.md). Safe rules add new work to Backlog and mark merged **PR records** Done. Automatic issue closure, issue-closure-to-Done and PR-link-driven issue status changes are removed: those events cannot prove acceptance. No automation accepts an ADR, approves legal work, completes a parent or closes a milestone. New items may briefly need manual status assignment; reconcile after every close, reopen, merge or dependency change.

Use native milestone open/closed scope, status group counts, sub-issue progress and linked exit checklists as navigation signals. A child progress percentage is GitHub's closed-child ratio, including closed dispositions; it is **not verified delivery percentage**. Exclude canceled work from completion reporting. Explain accepted deferrals and evidence gaps separately. For blocked age, use the dated attention entry, not the issue's last-updated time. No velocity, estimate baseline, date commitment or forecast is inferred from counts.

