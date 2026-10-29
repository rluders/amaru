# GitHub management system validation

Date: 2026-10-29. Repository baseline: `0f3eaf6169440b14b946c27dd04628db4ba3ac4f` on `main`. Configuration performed through authenticated GitHub CLI, GraphQL and REST APIs. The local documentation worktree starts from that baseline; local checks are not CI evidence.

## Inventory and changes

The live initial inventory contained 224 issues (166 open, 58 closed), 11 milestones, 157 parent relationships, 288 native blocker links and 89 merged PRs, with no open PRs. Every issue had a milestone. No effective dependency cycle or duplicate planning ID was found. The only owner Project was unrelated; no Project was linked to Amaru.

Created [Amaru — Roadmap and Delivery](https://github.com/users/rluders/projects/2), private, owned by `rluders`, linked to `rluders/amaru`. The Project contains the existing backlog, not a copy of its issues. Status is the only custom field, with Backlog, Ready, In Progress, Blocked, In Review and Done. Native milestone/parent/progress/reviewer/linked-PR metadata is retained.

The seven saved views are Executive Overview, Delivery Board, Roadmap, Milestone Planning, Backlog and Triage, Blocked and At Risk, and PR and Integration. Filters are repository-scoped. Delivery uses Status columns; tables use Status or milestone grouping where useful. The roadmap retains manual M0–M10 sequence without invented dates. The PR view is intentionally empty until a PR is open.

Corrections preserve issue numbers and discussion:

- #15 reopened: four required children (#129, #145, #157, #158) remain open, and merged PR #289 explicitly records incomplete behavior. The owner-approved Work Package OS deferral is preserved; M3 exit evidence remains outstanding.
- #60/#12 exit criteria now show the recorded owner-approved M1/M2 resolutions and carried work, rather than implying unqualified validation. M1/M2 milestones closed; all 11 milestone descriptions link their goals, dependencies and exit records without due dates.
- Current M3 impediments and owner gates identify a responsible owner and next action. #157 remains Blocked for its explicit public idempotency-contract clarification. Future milestone prerequisites stay in Backlog rather than making every future issue an active risk.
- #250 gains the source-backed `OWN-adr-0048` marker for its completed owner-decision record; canceled importer #270 remains not planned without an invented catalog ID.
- Existing labels are retained. `needs-attention` supports actionable risk filtering; `task` supports standalone engineering work. Catalog P0/P1/P2 meanings are preserved rather than reinterpreted as urgency.

## Automation and capability limits

Removed automatic issue closure, issue-closure-to-Done and PR-link-driven status changes. Removed redundant auto-add-sub-issues after GitHub returned server errors during imports; explicit idempotent imports completed through the API. Default Item added to project and Pull request merged rules are retained. Issue completion, reopening, owner acceptance and milestone closure require evidence review and manual reconciliation.

Native issue types are not configured: the REST endpoint lists default types, but assignment and readback return null on this personal repository. Labels are the documented fallback. No custom type field was added. GitHub's REST view creation API supports grouping; GraphQL updates alone do not expose those controls. System timestamp fields accepted by GraphQL view configuration were rejected by the REST view endpoint, so native timestamp columns use GraphQL and no unsupported timestamp sort is claimed.

Built-in repository auto-add and visual hierarchy settings require an authenticated UI session. At the time of this record, the connected browser is signed out. These settings are not claimed as configured or visually verified. Until enabled, maintainers must add new issues and PRs explicitly; issue forms request `rluders/2`, which requires the opener to have Project write access. The forms and operating-guide changes take effect only after their documentation PR merges. No token, secret or custom automation service was introduced.

## Gauntlet and verification

Final live readback: **224 issues reviewed and added; 39 issue records updated; 185 unchanged outside their Project membership/status.** No issue was created, deleted or recreated. There are 167 open and 57 closed issues. The 157 parent links and 288 blocker links match the initial inventory exactly. The canceled decisions #65/#270 remain closed as not planned, with no Done status. All 223 scheduled/historical planning records map to the backlog map with matching milestones and parents; #270 is the documented canceled exception.

Project status: 132 Backlog, 4 Ready (#15, #129, #145, #158), 31 Blocked and 55 Done; 2 canceled records have no status. These are workflow counts, not a delivered-value percentage. There are 33 attention items and 36 explicitly assigned current/risk items. No cycle, duplicate planning ID, duplicate Project item, missing milestone or closed parent with open required children remains.

| Saved view | Verified issue count before the documentation PR |
|---|---|
| Executive Overview | 64 |
| Delivery Board | 35 |
| Roadmap | 64, in M0–M10 order |
| Milestone Planning | 66, M3 excluding canceled importer #270 |
| Backlog and Triage | 132 |
| Blocked and At Risk | 33, including every Blocked item |
| PR and Integration | 0; the documentation PR is added separately after creation |

The saved filters were executed through the REST view-items endpoint, not just compared as strings. The board has native Status columns and milestone swimlanes. Executive and Milestone Planning group by Status; backlog and risk group by Milestone. The roadmap uses ungrouped manual ordering to preserve M0–M10 without lexical M1/M10/M2 ordering. No unsupported timestamp sort or visual hierarchy setting is claimed.

A temporary Project-only draft verified **Item added → Backlog** and persisted transitions through Ready → In Progress → Blocked → In Review → Done → Backlog. It was deleted after the check; no repository issue or permanent test artifact was created. This tests Project field behavior, not issue/PR event workflows or the truth of acceptance criteria. The retained Pull request merged rule is enabled, but its merge event was not exercised; no PR was merged for this audit.

| Gauntlet pass | Result |
|---|---|
| A — structural integrity | API verification passes: one Amaru Project, repository link, 224 distinct backlog items, one custom field and seven saved views. |
| B — data integrity | Passes: assignments and native edges preserved, effective cycles checked, actionable blockers, parent closure corrected and canceled work excluded from Done. |
| C — usability | Saved filters and M0–M10 ordering pass through REST. Current milestone, next Ready work, owner gates, exit evidence and lack of committed dates are explicit. Visual browser walkthrough remains pending. |
| D — operation | Project-only lifecycle and safe default verified. Repository-wide auto-add and real issue/PR event testing remain pending; manual reconciliation is documented. |
| E — correction | Reopened #15, reconciled qualified exits, completed risk/owner coverage, corrected #250's header to its existing M2 milestone, removed temporary duplicate view configurations and fixed roadmap order. |
| F — final verification | Fresh GraphQL/REST readback passes for the applied configuration and backlog. **The full Gauntlet is not declared complete while the UI/automation limitations remain.** |

Local validation (Linux): `make check` **exit 0**, covering documentation, Go vet/tests, contract checks, Protobuf checks, Go/TypeScript fixture parity, SDK generation/type/tests and Platform checks. Four issue forms parsed with PyYAML and passed assertions for unique identifiers, required execution fields, valid labels and Project destination. `git diff --check` passed. Final documentation-only validation and Trail results are appended with the review record. PostgreSQL-backed tests and macOS/Windows native runs were not required or run for this management/documentation change.

Review checks: `make docs-check` **exit 0** after the completed audit record; `trail check --base origin/main` **exit 0**, no conflicts. Trail's grouping advisories were reviewed: the configuration uses REST for grouping and GraphQL for supported view updates, consistent with the learning. No behavior was changed to suppress a check.

## Remaining UI steps and next cycle

1. Sign into GitHub in the connected browser and open the Project's Workflows menu. Enable **Auto-add to project**, select `rluders/amaru`, and use `is:open` to cover new open issues and PRs. Keep the verified Item added default of Backlog. Verify a real new item is imported; auto-add does not backfill existing items.
2. Visually verify saved grouping/filter behavior and enable Show hierarchy in the table views where useful. Do not add dates to manufacture a timeline.
3. Merge the documentation PR after review to activate issue forms and the PR template. Until auto-add is enabled, explicitly add new issues and PRs; form-based addition works only for openers with Project write access.
4. For the next cycle, use Milestone Planning for M3, start with #129/#145/#158 under reopened #15, and resolve #157's contract clarification and #80–#84 owner gates. Finish #15 before its dependent Work Packages are considered Ready. Review #20 for remaining exit evidence and #27 for carried installation/packaging conditions. All real implementation, integration and owner approvals remain separate work.

