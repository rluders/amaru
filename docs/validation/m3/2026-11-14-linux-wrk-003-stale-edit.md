# WRK-003 stale Agent edit domain verification — Linux

Date: 2026-11-14. Runner: Codex, local Linux x86_64 environment
(kernel 7.2.11-100.fc43.x86_64); physical/VM host classification unavailable.
Go 1.26.0, Node 25.7.0, pnpm 11.5.0. Local evidence, not hosted CI.
Candidate: `feat/219-stale-edit-conflict`; this record and the test are in the
same commit. Base: `feat/18-wp-m3-06-public-api-events` at
`32df8a167681811ccb5b9420c3fa97efa00add60`.
Refs [#219](https://github.com/rluders/amaru/issues/219), parent
[#18](https://github.com/rluders/amaru/issues/18). Both remain open.

## Selection and architecture review

One bounded #219 verification slice was selected over public Evidence exposure.
[Roadmap M3](../../architecture/ROADMAP.md#m3-platform-core) requires headless
API integration; WP-M3-06 owns committed changes and WP-M3-07 owns Evidence.
[ADR-0059](../../architecture/adr/0059-adapt-the-prototype-http-handlers-to-the-platform-public-api.md)
explicitly requires stale full-field Agent edits to preserve newer unrelated
fields. `UpdateAgent.expectedRevision` and the OpenAPI `updateAgent` operation
already specify conflict rejection. `AgentHistory.Update` and `Replace` already
implement it. The earlier `TestLostUpdateIsRejected` covered competing Role
changes, not the Worktree permission regression and reconciliation sequence.

Verdict: **sound for this pure domain verification slice**. No contracts,
authentication, persistence, dependencies, infrastructure, or production behavior
change. No new significant architecture decision is made. A test-only HTTP server
or in-memory Agent store would not prove the absent production mutation boundary;
this change does not create either or claim two-client integration acceptance.

The competing #124 route is not ready for immediate public implementation.
Accepted ADR-0069, inspected on #19 at `2a0136b`, requires versioned contract
review before exposure, and authorization within the metadata read boundary.
OpenAPI has no Evidence bundle operation/schema. #18's ProjectAccess Evidence
ownership check and #19's internal repeatable-read bundle do not together provide
that composed boundary: the latter has no principal or authorization scope. A
new endpoint cannot safely serialize the domain type and call that integration
complete. Next there is contract/access composition review with #18, including
error indistinguishability, bounded size and scoped child/audit reads. No new
ADR is asserted necessary merely because that implementation/review is missing.

The owner's shared, untracked architecture image and both article PDFs were
inspected in the original checkout, without staging or copying them into this
branch. The diagram's Control Plane/Data Layer and the architecture article's
sections 3, 6 and 7 place authority and durable history in the Platform; the
vision article describes replaceable agents and accountable execution. They
support the accepted Platform/client boundary, not a new Evidence protocol,
Agent store, Brain service or optional infrastructure. Canonical accepted ADRs
remain authoritative. Inspected SHA-256 identities (no immutable repo links):

- `docs/architecture/architecture.png`: `cc5abaf9ff3167c7edd5ffc9be5a6129d177d77ae75e6823b4dfa23eadd35a0f`
- Vision article PDF: `5186454ec5772acc1b15e6f4e9330a25e249a16d7cdd731804a469fb4a6b0d84`
- Architecture article PDF: `66b9d4687196157e9277075954e33af85a1c8c5de18e82b76576178a0f5091dc`

## Behavior and limits

`TestAgentStaleEditPreservesWorktreeDecision` covers enabling and revoking
Worktree writes/command lists, each through a stale patch and whole-definition
replacement. Rejection returns no successful result, leaves every recorded
revision unchanged and leaves the open editor's input intact. Refetch plus a
Role-only patch preserves both edits and prior revisions. Current duplicate
content is a no-op; obsolete revisions conflict even for matching content.
`TestAgentInvalidReplacementPreservesOpenEdit` verifies invalid command/write
combinations leave history and revision intact so the open edit can still save.
These are **unit-tested domain scenarios**, with sequentially interleaved editors.

Negative control: temporarily bypassing the revision comparison made all four
stale-edit subcases fail by accepting revision 3 with obsolete permission fields.
The original production source was restored before the final checks. This proves
the regression detects loss of the guard, not a reproduced predecessor incident.

**Not validated:** authenticated two-client Agent requests, durable/concurrent
Agent transactions, Project isolation of Agent edits, typed Event publication,
Event replay/duplicates, actual file watching/invalid-file ingestion, full #219,
#18 or M3 acceptance, real Providers, native macOS/Windows. `Replace` tests do not
establish file detection. No Agent store/application/HTTP composition exists on
this base; the accepted auth boundary is not bypassed to simulate it. No database
code changed, so no PostgreSQL runtime check is claimed or needed for this slice.

## Commands and outcomes

From `services/platform`:

- `GOCACHE=/tmp/amaru-219-conflict-cache go test -race -count=1 -v ./internal/domain -run 'TestAgent(StaleEditPreservesWorktreeDecision|InvalidReplacementPreservesOpenEdit)'`:
  PASS, exit 0; two top-level tests, four stale-edit subcases, no skips.
- Negative control `go test -count=1 ./internal/domain -run TestAgentStaleEditPreservesWorktreeDecision`:
  expected FAIL, exit 1; all four stale-edit subcases detected accepted obsolete values.

An initial shell path mistake created no file and the first focused invocation
reported no tests to run; it is not validation evidence. The successful focused
run above was after the file was added.

From repository root:

- Initial `GOCACHE=/tmp/amaru-219-conflict-cache make check`: FAIL, exit 2;
  pnpm could not open its existing store database under the filesystem sandbox.
  Earlier Go/contracts/docs/protocol checks passed; this was not a test failure.
- `GOCACHE=/tmp/amaru-219-conflict-cache make check` rerun with access to the
  existing dependency cache: PASS, exit 0. Go/TS fixture parity and SDK generation,
  type checks and tests pass (SDK 48/48); Platform tests and host/Windows/macOS/
  PostgreSQL-tag vet pass. Log: `/tmp/amaru-219-check-rerun.log`.
- `GOCACHE=/tmp/amaru-219-conflict-cache make docs-check`: PASS, exit 0,
  `doccheck: ok`, after adding this record.
- `git diff --check`: PASS, exit 0. No production guard changes remain.

## Queue refreshed after #387

Read-only GitHub REST native dependency queries were run for **all 25 open M3
issues**, plus selected closed prerequisites. Issue-body historical blockers were
not used as authority. No issue bodies/comments, acceptance checkboxes, Project
boards, issue closures or merges were changed. No open PRs were present at the
start of this work.

| Queue | Direct native blockers observed | Interpretation |
| --- | --- | --- |
| #124 | #119, #123 closed | Parent #19's #15 closed; prerequisites ready, API review above pending |
| #219 | #135 closed | Parent #18's #15 closed; #83/#84 closed, code on base |
| #191 | #120 closed | Parent #18 ready; remaining integration acceptance stays open |
| #92, #170 | #83, #84 closed | Parent #18 ready; remaining acceptance stays open |
| #154 | #85, #120 closed | Parent #19 ready; remaining acceptance stays open |
| #188 | #135 closed | Remaining audit acceptance stays open |
| #80, #81, #82 | #8 closed | Migration decision issues; native readiness is not owner approval |
| #112, #113, #127, #128, #131, #163 | #80, #81, #82 open | Blocked by migration decisions |
| #189 | #113 open | Blocked |
| #190 | #127, #113 open | Blocked |
| #197 | #124 open; #195 closed | Blocked; internal Evidence progress does not satisfy #124 |
| #125 | #197 open | Blocked |
| #16, #17, #18, #19 | #15 closed | Integration acceptance still open |
| #20 | #16, #17, #18, #19 open | M3 exit blocked |

Fetched branch heads:

- `origin/main`: `d01934ea51a2839d3f957d23c9814a598a056cba`;
  PR #379 merge at 2026-11-14 01:17:36 UTC. The M4 self-development marker is
  present in canonical PRD, ROADMAP and WORK_PACKAGES on main. It is an M4 exit
  target, not an available M3 capability.
- #18 integration: `32df8a167681811ccb5b9420c3fa97efa00add60`.
- #19 integration: `2a0136ba75fd554e5b94f9437f06eb8fee6998ed`;
  PR #387 merge at 2026-11-14 05:07:53 UTC. Its snapshot reader is on #19,
  not on main or #18. No integration branch was merged or rewritten here.

Next prerequisite-ready #219 task: review and implement a Project-bound durable
Agent mutation boundary using the existing expectedRevision contract, under
ADR-0043/0059/0062, before authenticated two-client and typed change tests. Resolve
ownership/transaction/publication composition from accepted decisions and ports
first; any significant missing semantics require human approval. This domain
slice does not make that full implementation automatically ready. Independent
#170 operational coverage remains prerequisite-ready; #124's reviewed scoped
contract/application composition remains the competing path. Shared dirty work
was preserved in the original checkout; only this isolated worktree is committed.
