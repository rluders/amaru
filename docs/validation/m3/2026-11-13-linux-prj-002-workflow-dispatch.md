# PRJ-002 committed Workflow dispatch isolation — Linux

## Scope and baseline

Bounded acceptance evidence for [#191](https://github.com/rluders/amaru/issues/191), based on #18 integration commit `55516862288a1dedb194876a6b978dbb9b1e5360` (merged PR #373). This connects previously separate committed-Workflow selection and independent Worker progress tests. It changes no runtime, contract, storage, dependency or architecture decision. The governing accepted decisions are [ADR-0062](../../architecture/adr/0062-project-bound-public-operations-and-secret-resolution.md) and [ADR-0066](../../architecture/adr/0066-platform-owned-active-project-clone.md); existing Execution completion semantics remain unchanged.

Run date: 2026-11-13 America/Sao_Paulo (2026-11-14 UTC). Environment: Linux x86_64, Git 2.56.0; shell Go 1.26.0 with the Platform module's selected toolchain. Local results are not hosted CI.

## Behavior exercised

`TestCommittedWorkflowsProgressIndependently` in [workflow_progress_test.go](../../../services/platform/internal/attach/workflow_progress_test.go) uses two real temporary Git repositories and persisted bindings in fake Platform storage. Both contain `same-flow` and Step `test`, with distinct definition contents. Requests enter through ProjectAccess, load committed definitions through WorkflowReader and dispatch through the actual coordinator and single-slot embedded Worker over the in-process link. The Runner is fake.

- Explicit test-owner Project and hash-pinned Workflow trust are preconditions; default Policy remains active. A committed `git.push` Workflow in the affected Project waits for Approval and never reaches the Runner.
- A later invalid committed definition is rejected without allocating an Execution ID. The waiting Execution, Steps, Events and Approval stay unchanged.
- A subsequent valid definition in that Project reports fake Provider unavailability; its Execution fails. The healthy Project's independently loaded Workflow succeeds.
- After coordinator/Worker recreation over retained fake state, another healthy Workflow succeeds. Earlier Execution/Step/Event/Approval snapshots are byte-identical. Pinned definitions retain their original Project contents despite later commits; cross-Project Execution queries return bare not-found with no Execution.
- Exact Runner counts are two healthy runs and one failure; any other Capability reaching the Runner fails the test.

The first test run failed because the test supplied Project trust without the Workflow's content hash; Policy correctly returned `policy.trust_changed`. Adding explicit hash-pinned Workflow trust corrected the test precondition. No production behavior was changed to make the test pass.

## Validation

Commands from `services/platform`:

- `go test -race -count=1 ./internal/attach -run TestCommittedWorkflowsProgressIndependently -v` — PASS, one top-level test, no skips.
- `go test -race -count=1 ./internal/attach ./internal/coordinator ./internal/worker/embedded ./internal/arch` — PASS, all four packages, no failed packages.

Repository check: initial `make check` FAIL, exit 2; one scoped rerun with the unrelated Worker test restored PASS, exit 0. The scoped run includes 348 passing TypeScript contract checks, 346 identical Go/TypeScript fixture outcomes, 48 passing SDK tests and passing Platform packages. PostgreSQL-tagged tests were compiled/vetted, not executed. Both full-check outcomes are retained below.

**Evidence level:** integration-tested with real Git, fake Platform persistence and fake Provider outcomes. Not validated: PostgreSQL execution, process/server restart, real Provider discovery/execution, secret delivery, public Workflow/Execution HTTP or SDK transport, product startup, native macOS/Windows. Recreation means replacing coordinator/Worker objects, not killing a process. No Workflow reconciliation, new typed change Event, fairness or retry behavior is established.

## Refreshed delivery gates

GitHub issues, native blockers, open PRs, recent merges and remote heads were refreshed on 2026-11-14 UTC. No open PR existed at selection. Main was `fe97930`; #18 integration was `5551686`. Recent merged work includes #370 PostgreSQL SDK Project acceptance, #371 rejected SDK attachment rollback, #372 session-bound CSRF evidence and #373 log sink panic containment. This slice does not repeat those SDK/auth/log cases.

| Queue | Native prerequisites and designated base | Remaining gate |
|---|---|---|
| #92 | #83/#84 and inherited #15 closed; `feat/18-wp-m3-06-public-api-events` | Product startup/state-directory/discovery publication, CLI `amaru open`, composed API/Event transport acceptance |
| #170 | #83/#84 and inherited #15 closed; same #18 base | Complete running-product instrumentation and output-channel capture; existing adapter/Worker/SDK diagnostics are partial evidence |
| #191 | #120 and inherited #15 closed; same #18 base | Authoritative Project-bound Work/dependency operations; other definition types and execution configuration; encrypted production secrets and authorized Policy/Approval assignment delivery; production Provider handling and full product/API composition |
| #219 | #135 and inherited #15 closed, ordered replay code on #18; same #18 base | Typed committed mutations, disk/API definition reconciliation, two-client conflict/no-silent-overwrite behavior; ordered replay alone does not define these semantics |
| #16 | #15 closed; `feat/16-wp-m3-04-policy-grants-and` at `d2c393a` | #188 has closed #135, but its implementation `fa8bb47` is not an ancestor of this base. Bring reviewed prerequisite code to that base before dependent work. #197 waits for open #124; #195 is closed |
| #17 | #15 closed; designated `feat/17-wp-m3-05-providers-and-context` absent remotely | Migration approval gates #80/#81/#82 open; #112/#113/#127/#128/#131/#163 natively depend on them. #189/#190 retain their dependent Provider/confinement gates. Establish the designated base and satisfy approvals before dependent vendor work |
| #19 | #15 closed; designated `feat/19-wp-m3-07-worktrees-configuration-scopes` absent remotely | #124 has closed #119/#123; #154 has closed #85/#120. These are safe alternative candidates after establishing their base from main and checking prerequisite code. #125 waits for open #197. Full Evidence/worktree integration remains owed |

The selected #191 slice is permitted by accepted contracts and closes only a composition-evidence gap. Metadata-only exact-scope secret resolution already exists; it is not encrypted delivery or #163 completion. Work records currently lack an authoritative Project-bound mutation path; pure dependency helpers cannot prove that boundary. This change does not choose reconciliation/version/Event semantics for #219. These issues, #18 and M3 remain open; no merge, closure or Project-board mutation is part of this work.

## Full-check failure investigation

The first `make check` exited 2: existing `TestFailureAndPanicAreReported` timed out waiting for both sibling Steps to fail. The affected four-package race suite independently passed. A focused unchanged-test run (`go test -race -count=30 ./internal/worker/embedded -run '^TestFailureAndPanicAreReported$' -v`) passed all 30 repetitions; this does not invalidate the observed full-check failure.

Exact initial failure from `/tmp/amaru-191-workflow-check.log`:

```text
--- FAIL: TestFailureAndPanicAreReported (5.00s)
    embedded_test.go:128: timed out waiting for both Steps to fail
FAIL
FAIL    github.com/rluders/amaru/services/platform/internal/worker/embedded    6.136s
make: *** [Makefile:62: platform-test] Error 1
```

Source-based diagnosis, not a deterministic reproduction of the original schedule: the existing test puts both outcomes in one Execution. `coordinator/settle.go` cancels queued siblings and signals running siblings after failure, so the requirement that both report their individual failure codes can depend on scheduling. Thirty passing focused reruns do not prove the full-check failure resolved.

An exploratory local test edit separated the two Executions; with that edit, a further focused 30-run race test and a full `make check` passed. Those results are **not validation of this PR**: the unrelated edit was removed from both the worktree and index at the user's request. The Worker test is identical to the integration base. No runtime/test repair for that independent failure is included.

The new #191 acceptance test separately passed `go test -race -count=10 ./internal/attach -run '^TestCommittedWorkflowsProgressIndependently$'` (10 repetitions, no skips).

The one requested scoped `make check` rerun passed (exit 0), with `embedded_test.go` identical to `5551686`; log `/tmp/amaru-191-scoped-check.log`. This passing rerun does not prove the independent failure resolved.

`trail check --base origin/feat/18-wp-m3-06-public-api-events` exited 1 with five lexical conflicts in the added Project access documentation. Reviewed individually: Workflow post-I/O authorization unchanged; command fingerprints/CloneRoot handling unchanged; no native path checks/imports added to storage; no migration or legacy compatibility added; transport-guard/cancellation precedence unchanged. No runtime source changed. This is not reported as a passing Trail check.
