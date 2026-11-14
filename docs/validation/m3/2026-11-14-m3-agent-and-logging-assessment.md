# M3 queue after #388: Agent composition deferred; logging assessment

Date: 2026-11-14. Local Linux assessment by Codex; not hosted CI.
Base: `feat/18-wp-m3-06-public-api-events`,
`5ba8886ed87c6ed9250cf716119592bbda0be766` (merged PR #388).
Assessment branch: `docs/219-agent-composition-assessment`.
Refs [#219](https://github.com/rluders/amaru/issues/219),
[#170](https://github.com/rluders/amaru/issues/170), parent
[#18](https://github.com/rluders/amaru/issues/18).

## Agent assessment: needs human decision, deferred

The owner explicitly deferred Agent authority in this task conversation:
“Defer the Agent authority decision; do not choose either model.” No model is
selected, no ADR is proposed or changed, and no Agent mutation implementation is
authorized by this assessment. This is a semantic gate for this slice, not a new
native GitHub dependency or a claim that every #219 task is blocked.

Existing contracts settle several parts already:

- [AgentHistory](../../../services/platform/internal/domain/agent_history.go)
  preserves immutable revisions, hashes, deletion tombstones and recreation;
  stale patches/replacements conflict, and unchanged content adds no revision.
  #388 adds domain regression evidence, not durable transaction evidence.
- [UpdateAgent](../../../packages/contracts/schemas/UpdateAgent.json) requires
  `expectedRevision`. Do not invent a replacement revision scheme.
- [ADR-0062](../../architecture/adr/0062-project-bound-public-operations-and-secret-resolution.md)
  defines authorized Project selection and local administrator access, with
  foreign/missing resources indistinguishable. Project selection is not authority.
- [ADR-0059](../../architecture/adr/0059-adapt-the-prototype-http-handlers-to-the-platform-public-api.md)
  permits selected HTTP adaptation only through Platform operations, with scope,
  stale-write protection and publication of committed changes.

The unresolved authority boundary prevents a bounded persistence/application slice:
[ADR-0043](../../architecture/adr/0043-project-definitions-in-git-teams-and-operational-state-in-platform.md)
places Project Agents in Git and says runtime writes do not land in the Project
tree (decision lines 18–31). [OpenAPI](../../../packages/contracts/openapi/openapi.yaml)
`createAgent` and `deleteAgent` instead describe writing/removing
`.Amaru/agents/<id>.json` (lines 687 and 761), while
[Agent](../../../packages/contracts/schemas/Agent.json) describes lowercase
`.amaru/agents/<id>.json`. A casing correction alone cannot resolve authority.
AgentHistory has no Project binding or durable adapter; current ports have no
Agent history persistence contract. Adding a PostgreSQL authoritative catalog
would silently settle more than an adapter implementation detail.

For a future owner review, the alternatives are Git-authoritative definitions
with an explicitly approved API/file reconciliation boundary, or Platform-authoritative
mutable Agents with a superseding definition-storage decision. Neither is chosen.
The review must settle how Project/Agent identity and revision histories relate
across clones, and what happens if definition publication and durable
history/Event commit fail on opposite sides of a crash. Preserve the existing
revision and authorization rules wherever applicable. Existing audit kinds cover
Grants, Approvals and trust; do not invent Agent audit kinds or infer that Agent
mutations are exempt from attribution. The existing `agent.changed` wire shape
in [CatalogChanged](../../../packages/contracts/schemas/events/CatalogChanged.json)
describes external file edits; the durable producer/commit composition for API
and file changes still needs resolution. No new event names are proposed.

The [roadmap M3](../../architecture/ROADMAP.md#m3-platform-core), issue acceptance,
both owner article PDFs in the shared checkout and the architecture image were
reviewed as context. The image's Platform Data Layer/Control Plane and articles'
separation of intelligence from durable authority support the accepted boundaries;
they do not approve storage or Event semantics. Image SHA-256:
`cc5abaf9ff3167c7edd5ffc9be5a6129d177d77ae75e6823b4dfa23eadd35a0f`.
The PDFs are untracked product context, not normative contracts or changed files.

## Independent #170 assessment and selected next slice

Verdict: **ready for bounded verification under existing behavior**. #170's native
#83/#84 and inherited #15 are closed and available on this base. No Agent
decision, Provider migration or new infrastructure is needed for the following
test-only slice:

Exercise the actual
[Platform Lifecycle](../../../services/platform/internal/platform/lifecycle.go)
with a non-nil diagnostic queue, failing/panicking fake stage runners, and the
existing operational logger. Assert safe JSON with trusted Instance/correlation
identity, no raw stage names/errors/panic payloads, preserved failure and admission
behavior, and bounded completion with nil/full diagnostic queues. Drain only after
producers join, using the existing lifecycle channel ownership rules.

Why this slice: existing
[lifecycle adapter tests](../../../services/platform/internal/adapter/operational/lifecycle_test.go)
exercise raw Lane and a logger together; existing
[Platform lifecycle tests](../../../services/platform/internal/platform/lifecycle_test.go)
pass nil diagnostics. The selected test closes the composition evidence gap
between those boundaries without changing logging schemas, authority or runtime
behavior. It is selected, **not implemented** in this assessment.

Existing [Runner instrumentation](../../../services/platform/internal/adapter/operational/runner.go)
uses assignment attempt/Execution/Step identity and bounded handoff, omitting
payloads and errors. Existing
[SDK capture](../../../packages/sdk/test/auth.test.ts) checks real child-process
stdout/stderr for auth and Project requests. Its
[server](../../../services/platform/internal/adapter/localauth/testserver/main.go)
is explicitly test-only, not product startup. The
[command logging test](../../../services/platform/internal/adapter/operational/log_test.go)
manually writes records after commands; that does not prove complete production
instrumentation. Full running-product command/Worker/channel coverage remains
open. Do not promote these partial checks to complete #170 acceptance.

## Queue refreshed read-only

All 25 open M3 issues had native blockers queried after #388; no open PRs were
observed. Historical issue-body blockers were not treated as current authority.

| Queue | Native blockers observed | Current interpretation |
| --- | --- | --- |
| #219 | #135 closed; parent #15 closed | Agent authority slice deferred by owner; other acceptance open |
| #170, #92 | #83/#84 closed; parent #15 closed | Prerequisites ready; #170 verification slice selected above |
| #191 | #120 closed; parent #15 closed | Remaining integration acceptance open |
| #124 | #119/#123 closed; parent #15 closed | Scoped public contract/access composition remains next on #19 |
| #154 | #85/#120 closed | Remaining acceptance open |
| #188 | #135 closed | Remaining acceptance open |
| #80/#81/#82 | #8 closed | Migration decisions still require owner approval |
| #112/#113/#127/#128/#131/#163 | #80/#81/#82 open | Blocked |
| #189 | #113 open | Blocked |
| #190 | #127/#113 open | Blocked |
| #197 | #124 open, #195 closed | Blocked |
| #125 | #197 open | Blocked |
| #16/#17/#18/#19 | #15 closed | Integration acceptance open |
| #20 | #16/#17/#18/#19 open | M3 exit blocked |

Observed remote main: `d01934ea51a2839d3f957d23c9814a598a056cba`.
#19: `2a0136ba75fd554e5b94f9437f06eb8fee6998ed`; #387 remains on #19,
not on #18 or main. No M3 completion or M4 transition is claimed.

## Validation and limits

Go 1.26.0 linux/amd64, Node 25.7.0, pnpm 11.5.0. This change is an
assessment only. No new fake/PostgreSQL Agent tests are appropriate because the
Agent implementation is stopped; no PostgreSQL runtime, real Provider, complete
two-client Agent API/Event flow or native macOS/Windows validation is claimed.
Shared checkout and other worktrees are preserved. No merge, issue closure,
issue comment, acceptance edit or Project mutation was performed.

Commands (local Linux, existing suites; no new tests added):

- From `services/platform`,
  `GOCACHE=/tmp/amaru-219-assessment-cache go test -race -count=1 -v ./internal/adapter/operational ./internal/platform`:
  initial run failed because sandbox loopback sockets were denied; rerun with
  loopback access PASS, exit 0, 39 top-level tests, no skips. These are unit and
  integration evidence for existing components, not proof of the selected new
  lifecycle-wrapper diagnostic scenario. Log:
  `/tmp/amaru-170-assessment-tests-rerun.log`.
- Initial `GOCACHE=/tmp/amaru-219-assessment-cache make check`: FAIL, exit 2,
  assessment ADR-0062 link used the wrong filename; corrected before rerun.
  Second sandbox attempt: FAIL, exit 2, pnpm SQLite dependency-cache access
  denied. These failures are retained rather than counted as passing checks.
- `GOCACHE=/tmp/amaru-219-assessment-cache make check` with dependency-cache
  and loopback access: PASS, exit 0. Contract parity: 346 identical Go/TypeScript
  fixture outcomes; TypeScript contract tests 348/348 and SDK tests 48/48.
  Platform tests and host/Windows/macOS/PostgreSQL-tag vet passed. Cross-OS vet
  and PostgreSQL-tag vet are not native OS or database runtime validation.
  Log: `/tmp/amaru-219-assessment-check-rerun.log`.
- `GOCACHE=/tmp/amaru-219-assessment-cache make docs-check`: PASS, exit 0,
  `doccheck: ok` after correcting the link.
- `git diff --check`: PASS, exit 0.
