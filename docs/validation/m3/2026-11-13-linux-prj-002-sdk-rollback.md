# PRJ-002 SDK PostgreSQL rollback acceptance — Linux

Date: 2026-11-13. Executor: Codex, owner-authorized continuation after PR #370.
Base: `ad5f1bdb4f5aa6f2772527a25e4b958a1b70595a`, designated #18 integration
`feat/18-wp-m3-06-public-api-events`. Candidate: accompanying PR commit.
Environment: local Linux 7.2.11-100.fc43.x86_64 amd64; Node v25.7.0;
PostgreSQL 17.5; Platform Go 1.27.0 (root Go 1.26.0). Local results are not CI.

## Change and boundary review

The existing opt-in SDK/PostgreSQL test now recreates its owners for three phases:
successful attachment/settings writes, rejected attachments, and final readback.
A test-only PostgreSQL AFTER INSERT OR UPDATE trigger raises a private diagnostic
after clone insertion. The real TypeScript SDK attempts both reattachment of A to
a replacement clone and attachment of a new Project. Both must return status 500,
no success data, and exactly the fixed `internal` error envelope. After each
failure the SDK checks unchanged list/roots and A=true/B=false settings. Final
list/detail/settings readback uses a fresh pool, stores, coordinator, Auth, HTTP
server and SDK client over the retained database. No orphan Project or changed
clone selection may survive. Temporary Git roots are real; credentials and child
assertion diagnostics are not printed.

Verdict: sound under existing Project registration and ADR-0067 contracts. This
extends the accepted test-only failure injection already used by the Go HTTP
PostgreSQL test. The simpler happy-path SDK test in #370 cannot establish rollback
through the SDK. No runtime, dependency, schema, contract, infrastructure or
architecture decision changes. The SDK uses only the public HTTP boundary and
owns no authoritative state. The private disposable database is created/dropped
by the existing helper; the temporary server uses a private Unix socket and no
TCP listener. Existing development data and shared worktrees are untouched.

## Validation

- `make check`: PASS, exit 0; 348 contract tests, 346 matching Go/TypeScript
  fixture outcomes, 48 default SDK tests, generated-code/type checks, Platform
  tests and docs-check. Log: `/tmp/amaru-m3-next-check.log`.
- From `services/platform`, with `AMARU_TEST_PG` pointing to the private disposable
  server: `go test -race -count=1 -tags pg -run TestProjectsPostgresSDKRecreation -v ./internal/adapter/httpapi`:
  PASS, one top-level test, write/reject/read subtests, no skips.
  Log: `/tmp/amaru-m3-next-focused.log`.
- Final `make docs-check`, `git diff --check` and staged whitespace check: PASS.
- `trail check --base origin/feat/18-wp-m3-06-public-api-events`: PASS, exit 0,
  no conflicts, 17 advisory lexical matches reviewed. No storage filesystem
  imports, clone-inclusive command fingerprint, legacy migrations, auth ordering,
  SQL panic recovery or lifecycle changes were introduced. No passing rerun is
  used to dismiss an earlier failure; no new failure occurred in these checks.
  Other matches concern unchanged Git inspection, Workflow authorization,
  contract fixtures and Project-board configuration.

The disposable PostgreSQL server was stopped after validation.

Evidence level: **integration-tested**, actual SDK/HTTP/Git/PostgreSQL components.
The default check typechecks and vets this opt-in test; only the tagged command
executes its PostgreSQL cases. Owner/pool recreation is not database-process
restart, crash recovery, production startup/readiness, browser or Provider
validation. Native non-Linux behavior and full #191/#18/M3 acceptance are **not
validated** by this change.

## Refreshed M3 acceptance and queues

Snapshot after #370, verified from GitHub issue bodies, native dependencies,
sub-issue states, recent all-state PRs and fetched remote refs. #363–#370 are
merged into #18; no PR was open at selection. No issue is closed by this slice.
Historical issue notes claiming #15/#83/#84 or #135 remain open are superseded
by their native closed states, not by an inference from partial implementation.

| Issue | Current evidence and remaining acceptance | Designated PR base / prerequisite |
|---|---|---|
| #92 IAM-001 | Auth adapters, wire/SDK and Linux browser evidence merged; full startup/discovery/`amaru open` and composed transport acceptance remain open. | #18 integration; native #83/#84 and inherited #15 closed. |
| #170 OBS-001 | Logging adapter, Worker diagnostics (#364), SDK Project capture (#368) and rejected-auth capture (#369) merged. Complete running-product channel capture/instrumentation remains open. | #18 integration; #83/#84/#15 closed. |
| #191 PRJ-002 | Project access/HTTP, SDK isolation and PostgreSQL readback (#365–#370) merged. This slice adds SDK failure rollback. Full Work/definition/Provider/secret and product composition acceptance remains open. | #18 integration; #120/#15 closed, prerequisite implementation present on base. |
| #219 WRK-003 | #135 ordered replay prerequisite is closed and on #18. Full typed mutation, external-file detection and two-client conflict acceptance remain open; prerequisite closure alone proves none of these. | #18 integration; #135/#15 closed. |
| #16 WP-M3-04 | #188 audit remains open, native #135 closed; #197 remains open, blocked by #124 (#195 closed). Other children closed; parent integrated acceptance still open. | `feat/16-wp-m3-04-policy-grants-and` at `d2c393a`; #135 commit `fa8bb47` is not its ancestor. Bring reviewed prerequisite code to this base before #188 integration work. |
| #17 WP-M3-05 | #80/#81/#82 migration owner gates open; #112/#113/#128/#131/#163 blocked by them. #127 remains open; #189 waits for #113; #190 waits for #127/#113. #122/#123 closed. | `feat/17-wp-m3-05-providers-and-context` (not a current remote ref). Resolve migration gates before vendor reuse, then establish designated base per delivery workflow; real Provider runs remain owed. |
| #19 WP-M3-07 | #124 Evidence and #154 worktree children open with closed native prerequisites; #125 retention waits for #197. #85/#119/#178 closed. Full Evidence-chain integration remains owed. | `feat/19-wp-m3-07-worktrees-configuration-scopes` (not a current remote ref). Establish designated base from main and verify prerequisite availability before implementation; do not borrow #18 as its PR base. |

Selection: #191 has its prerequisites on the actual designated base and a concrete
SDK failure-path gap directly after #370. The bounded test uses existing rollback
semantics and failure injection; it requires no new human architecture decision.
No M3 exit, main integration, board-status mutation, merge or issue closure is
claimed. Linux remains required; other OSes are best effort under the recorded
M3 scope decision.
