# Linux ART-002 multi-report Usage acceptance — 2026-11-14

## Scope and refreshed queue

Base: #19 integration head `fca245e4a5ff42cc008f1e8149c5e22d5204e21b` after merged #385. Branch: `feat/124-multi-report-usage-evidence`; isolated worktree `/tmp/amaru-124-next`; PR base `feat/19-wp-m3-07-worktrees-configuration-scopes`. Refs #124 and #19. Neither issue is complete; no parent closure or merge is authorized by this record.

GitHub refresh on 2026-11-14 found **25 open M3 issues and no open PRs**. Native blocked-by relationships, including closed prerequisites, were fetched for every open M3 issue. Children inherit parent blockers. The table is a selection-time snapshot, not a claim that every ready dependency has a finished implementation.

| Open M3 issue | Parent | Native blockers |
|---|---|---|
| #219 | #18 | #135 (closed) |
| #197 | #16 | #124 (open), #195 (closed) |
| #191 | #18 | #120 (closed) |
| #190 | #17 | #127 (open), #113 (open) |
| #189 | #17 | #113 (open) |
| #188 | #16 | #135 (closed) |
| #170 | #18 | #84 (closed), #83 (closed) |
| #163 | #17 | #80 (open), #81 (open), #82 (open) |
| #154 | #19 | #85 (closed), #120 (closed) |
| #131 | #17 | #80 (open), #81 (open), #82 (open) |
| #128 | #17 | #80 (open), #81 (open), #82 (open) |
| #127 | #17 | #80 (open), #81 (open), #82 (open) |
| #125 | #19 | #197 (open) |
| #124 | #19 | #119 (closed), #123 (closed) |
| #113 | #17 | #80 (open), #81 (open), #82 (open) |
| #112 | #17 | #80 (open), #81 (open), #82 (open) |
| #92 | #18 | #84 (closed), #83 (closed) |
| #82 | #17 | #8 (closed) |
| #81 | #17 | #8 (closed) |
| #80 | #17 | #8 (closed) |
| #20 | — | #16 (open), #17 (open), #18 (open), #19 (open) |
| #19 | — | #15 (closed) |
| #18 | — | #15 (closed) |
| #17 | — | #15 (closed) |
| #16 | — | #15 (closed) |

All four open Work Package parents have closed #15 as their native blocker. Thus #124 is dependency-ready (#119/#123 closed and code present on #19), along with #154, several #18/#16 children and provider migration decision work. #197 still waits for #124, and #125 waits for #197; those are not ready implementation alternatives. Provider children blocked by #80/#81/#82 cannot start merely because their parent #17 is ready. Historical issue-body notes claiming #15 is open or no Usage is recorded are stale; native state and merged code take precedence. No dependency edge needs changing after #385: that PR advances partial #124 work, not completion of its acceptance. No issue or Project state was mutated.

Selected **one bounded #124 validation slice**: multi-report Worker results through existing capture, persistence, replay and the Evidence Reader. The existing 21-case suite used only one report; checking one pending row per result could not demonstrate report ordering or exact ordinal references. Nine added cases exercise two reports across success/failure/cancellation and normal/before-commit/after-commit delivery. The PostgreSQL acknowledgement guard now checks every stored publication id and contiguous ordinal rather than only the row count. Exact and conflicting/reordered replay must preserve the receipt, lifecycle history and bundle digest. Sources, unknown-vs-zero and decimal currency values remain separate; no aggregation is introduced.

This is acceptance coverage of existing behavior, not a production behavior change. It advances ADR-0069's first Usage slice without inventing the missing public Event relay, contract or remaining recorder shapes. #154 is also ready but requires a separate production launch/reporting slice; switching there would leave this specific Usage acceptance gap untested.

## Authority, product intent and review

Accepted [ADR-0069](../../architecture/adr/0069-durable-evidence-capture-and-authorized-bundle-reads.md#capture-and-publication-boundary-accepted-direction) requires producer order, conflict rejection, durable capture and publication intent before acknowledgement. [ADR-0033](../../architecture/adr/0033-usage-and-cost-telemetry-model.md) requires provenance, explicit unknowns and all-attempt attribution. No ADR status, public wire shape, dependency, infrastructure or approval rule changes.

Repository instructions, Trail, current integration-base [roadmap](../../architecture/ROADMAP.md#m3-platform-core), [WP-M3-07](../../architecture/WORK_PACKAGES.md#wp-m3-07-worktrees-configuration-scopes-evidence), architecture boundaries and accepted ADRs were read before implementation. The shared dirty checkout's roadmap/PRD additions were inspected as context and preserved. Its untracked architecture.png was visually inspected: Control Plane owns state, reviews and permissions; Platform Database and Artifact Storage are distinct; Experience clients use the Platform. Brain/vector/distributed illustrations do not authorize new M3 scope.

Both local owner PDFs were read as product intent, not authorization. *Why I'm Building an Engineering Execution Platform*, pp. 21–23, connects attributable history and cost to accepted outcomes and calls for a smaller validated core. *From Vision to Architecture*, pp. 10–11, requires reconstruction linking intent, process and artifacts, including failures. [Exact local filenames](../../architecture/adr/0069-durable-evidence-capture-and-authorized-bundle-reads.md#owner-article-context) remain untracked inputs and are not committed here.

Within the user's planning delegation, the executor recommends retaining **M4 Local MVP** as the first supervised Amaru-develops-Amaru target: M3 supplies the headless foundation; M4 supplies delivery/check/review and human approval. This session neither implements that flow nor changes milestone criteria. Significant architecture decisions still need explicit human approval; articles supply none. This test-only slice needs no new human decision.

Architecture-review verdict: **sound**. Platform-owned attribution, pure domain code, public-API-only clients and the Worker boundary are unchanged. Faults exercise the existing atomic commit and acknowledgement boundary; no new failure state is invented. Count-only checks were simpler but cannot detect wrong per-report references. Existing native macOS/Windows evidence is not inferred from cross-vet.

## Local evidence

Environment: Linux `7.2.11-100.fc43.x86_64`, x86_64; Go `go1.26.0 linux/amd64`; Node `v25.7.0`; pnpm `11.5.0`; disposable PostgreSQL `17.5`. Local results, not hosted CI.

- From `services/platform`: `GOCACHE=/tmp/amaru-124-query-cache go test -race -count=1 -v ./internal/coordinator -run '^TestUsageCapture$'` passed (exit 0), 30 shared cases including nine new multi-report cases.
- From `services/platform`: `AMARU_TEST_PG=postgres://usage_test@127.0.0.1:55481/postgres GOCACHE=/tmp/amaru-124-query-cache go test -race -count=1 -v -tags pg ./internal/store/postgres -run 'TestUsage(Capture|PublicationAtomicity)$'` passed (exit 0), two top-level tests, 30 shared cases, no skips. PostgreSQL supplies the Execution/capture store; the Reader uses empty fake context/Artifact stores. This is not an all-store audit walkthrough.
- From repository root: `GOCACHE=/tmp/amaru-124-query-cache make check` passed (exit 0) after a sandbox-only pnpm SQLite-store access failure (initial exit 2); rerun allowed store/loopback access. Includes Platform tests and host/Windows/macOS cross-vet, contracts Go/TypeScript parity for 336 fixtures, TypeScript 338/338 and SDK 17/17. Cross-vet is not native non-Linux validation.
- `GOCACHE=/tmp/amaru-124-query-cache make docs-check` passed (exit 0, `doccheck: ok`) after documentation updates. `trail build`, `trail build --check`, `trail check --base fca245e`, and `git diff --check` passed (exit 0); Trail reported no conflicts. Wiring graph current; optional deep layer not built.
- `/tmp/amaru-335-postgres/bin/pg_ctl -D /tmp/amaru-124-next-pgdata -m fast -w stop` passed (exit 0). Subsequent `pg_ctl ... status` returned expected exit 3, no server running.

## Limits and next task

**Unit-tested:** shared fake-backed multi-report capture/replay/Reader cases. **Integration-tested:** the targeted PostgreSQL Usage suite and publication atomicity. Faults are injected around Commit; they do not kill/restart the database or process. **Not validated:** full PostgreSQL suite, actual UsageRecorded publication, consistent multi-store snapshot, authorized public Evidence API and Organization boundary, checks/diffs/logs/workspace recorders, real Provider Usage, native macOS/Windows or complete ART-002 acceptance. No production behavior is newly implemented or real-validated by this slice.

Next dependency-ready task for orchestrator review: continue #124 with the internal PostgreSQL transaction-consistent metadata read scope under ADR-0069; first review the query/store shape and concurrency/cancellation cases against the existing Reader. It can proceed behind the internal boundary, but public exposure still requires versioned contract and access-isolation review. Any significant unresolved architecture choice must return to the human, not be inferred from this recommendation. #197/#125 remain blocked until #124 acceptance is actually met.

Shared dirty work, PDFs, diagram and other worktrees are preserved. Generated Trail cache is excluded from the PR. No merge, issue closure or parent acceptance is performed.
