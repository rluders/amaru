# PRJ-002 independent Step progress — Linux, 2026-11-12

## Scope and baseline

PR #333 merged into `feat/18-wp-m3-06-public-api-events` at `f89fc1325a354931acc841c033e63472cd85c1fa`. The isolated integration worktree was fast-forwarded and merged `make check` passed locally (exit 0). Main remains current at `fe97930`; unrelated shared user changes were preserved. This verification slice follows accepted [ADR-0062](../../architecture/adr/0062-project-bound-public-operations-and-secret-resolution.md), under [#191](https://github.com/rluders/amaru/issues/191) and [#18](https://github.com/rluders/amaru/issues/18).

The only code addition is `services/platform/internal/worker/embedded/project_test.go`. It reuses the existing coordinator, embedded Worker, fake Execution store, test trust double, serialization link and duplicate-result/ack observers. There is no runtime, contract, dependency, infrastructure, migration or license change.

## Evidence

**Integration-tested with fakes:** six cases (three failure modes over two links) keep a pending Approval in one Project while that Project's other Step fails with a Provider error, reports a fake unavailable Provider, or panics. With one Worker slot, the other Project's matching-name Step succeeds, then new healthy work succeeds after coordinator/Worker recreation. Default Policy remains active and the waiting Approval never reaches the Runner. Every completed attempt receives accepted and duplicate acknowledgements; Runner counts and single terminal Step Events prove results do not rerun work. Old Step records, Approvals and Events are unchanged after restart and unrelated work.

The restart retains fake storage; it is not a process-kill or PostgreSQL durability test. `provider_unavailable` is a fake Runner result, not actual Provider selection/discovery or pre-assignment rejection. No real service is used.

## Checks

- `make check`: passed locally on Linux, exit 0; includes Platform/architecture checks, 344 contract tests, 342 shared-fixture parity cases and 39 SDK tests.
- From `services/platform`, `go test -race -count=1 ./internal/worker/embedded ./internal/coordinator ./internal/arch`: passed, exit 0.
- `git diff --cached --check` and `trail check --base origin/feat/18-wp-m3-06-public-api-events`: passed, no Trail conflicts.

These are local results, not hosted CI. Initial test drafts incorrectly selected uncovered `code.build`, compared submission-time Approval metadata with persisted metadata, and expected unimplemented parent Execution aggregation. The final cases use allowed `check.run`, persisted snapshots and explicit Step outcome assertions; default Policy and runtime behavior were not weakened.

## Review and remaining gates

Boundary review: **sound** within ADR-0062. Only test composition crosses coordinator/Worker boundaries through existing ports. The Worker receives Assignments and cannot read authoritative state; fake trust setup is not production authorization. Existing tests already covered individual outcomes; this addition checks those outcomes across Projects and restart without introducing runtime abstractions.

**Existing gap found:** successful/failed results settle Steps, but `coordinator.onResult` only settles requested cancellation at the Execution level. Their parent Executions remain `running`; ordinary terminal Execution aggregation is still an open #18 lifecycle gate. The test establishes independent Step progress, not complete Execution lifecycle acceptance.

Not validated: public two-client API/SDK acceptance, Project settings, authoritative Work dependencies, exact-scope secrets and encrypted authorized assignment delivery, persisted clone lookup, real Provider-unavailability behavior, PostgreSQL/process-crash recovery, or native non-Linux runtime. No store semantics changed, so PostgreSQL execution was not required for this test-only change. Linux remains required and other OSes best effort. #191, #18 and M3 remain open.
