# PRJ-002 Project slot isolation — Linux, 2026-11-14 UTC

## Scope and selection

Refreshed origin and native GitHub dependencies before selection. The designated #18 base is `feat/18-wp-m3-06-public-api-events` at `a28a6e2e8d38d07629bba25ef1a1719bed2a3e22`. Native #191 blocker #120 and parent #18 blocker #15 are closed. Work is isolated in `/tmp/amaru-191-next`; shared checkout user changes are preserved. This slice extends #374's committed Workflow regression under accepted [ADR-0062](../../architecture/adr/0062-project-bound-public-operations-and-secret-resolution.md), rule 6, using existing coordinator limit semantics. No startup proposal, new dependency, infrastructure or predecessor code is used.

Project-bound Work/dependency persistence was considered first. [DOMAIN_MODEL §5.3](../../architecture/DOMAIN_MODEL.md#53-work-item) and §7 define same-Project edges, start-only dependency checks and human reopening with a reason. They do not by themselves define the complete public mutation boundary:

- `CreateWorkItem` requires package/title but omits owner by default; `WorkItem` requires an owner. The omitted-owner mapping is not specified.
- The Platform assigns an Instance-unique `<PROJECTKEY>-<n>` display key, but Project attachment has no display-prefix allocation/collision rule or API for selecting that prefix. The Project identity from ADR-0046 is a different value.
- `UpdateWorkItem` has no reopen-reason field, while the domain requires a human reason and records it through a caller's Event. `blockedReason` is retained only for blocked items; reusing it as a reopen reason would choose new semantics.
- Work create/update bodies carry no command key or expected version. [ADR-0058 rule 5](../../architecture/adr/0058-command-idempotency-conflicts-and-retention.md#decision) requires documenting keys before public exposure or explicitly resolving mutation scope with the owner. Duplicate/concurrent Work mutations and atomic Work/Event outcomes need a defined contract before implementation.

**Human decision needed for that follow-up:** approve the Work creation/default/display-key rules and mutation replay/concurrency/reopen-reason contract, then align schemas, fixtures and SDK. This slice leaves those questions unresolved and adds no speculative Work port, store or service. Pure Work helpers alone cannot establish authoritative isolation. Other definition/configuration, encrypted authorized secret delivery, production Provider and product/Event composition gates remain open. ADR-0068 is accepted on this base, but its process owners and production failure matrix are not implemented by this test.

## Observed behavior

Two real temporary Git Projects attach at their original roots with matching Workflow/Step identifiers. Explicit test-owner hash-pinned trust is required and default Policy remains active. The affected Project holds one Runner through a channel barrier; its second Execution queues ahead of healthy work. Two Worker slots leave capacity for the healthy Project. Healthy completion precedes release, while the other Step remains queued for the Project limit with exactly one affected Runner call. Both affected Executions fail after release; the healthy Project completes again after coordinator/Worker object recreation. A pending Approval and all prior Execution/Step/Event/Approval snapshots remain unchanged. Exact Runner counts are two healthy and two fake failures.

A temporary negative probe changed only the test's `PerProject: 1` to zero. The focused race test failed (exit 1): the queued Step was already failed, with two affected Runner calls. The test limit was restored before all candidate checks. This demonstrates sensitivity to the Project limit; it is not a runtime defect or a shipped behavior change.

## Validation

- `go test -race -count=10 ./internal/attach -run '^TestCommittedWorkflowsProgressIndependently$' -v` — PASS, 10 runs, no skips (from `services/platform`).
- `go test -race -count=1 ./internal/attach ./internal/coordinator ./internal/worker/embedded ./internal/arch` — PASS, all four packages, exit 0 (from `services/platform`).
- `make check` — PASS, exit 0; log `/tmp/amaru-191-limit-check.log`. PostgreSQL-tagged code is compiled/vetted, not executed.
- `make docs-check` — PASS, exit 0; `doccheck: ok`.
- `git diff --check` — PASS, exit 0.
- `trail build` followed by `trail build --check` — graph current, exit 0. The initial pre-refresh graph was stale for the edited test; the wiring graph was rebuilt. Deep meaning nodes are not built.
- `trail check --base origin/feat/18-wp-m3-06-public-api-events` — exit 1, three lexical conflicts in the added Project access prose. Reviewed: no authorization helper or cancellation ordering changes; no migration/upgrade/compatibility implementation; no storage/native-path imports or behavior changes. Advisory matches do not establish a new scheduling, join, PostgreSQL or contract-runner defect. This is not a passing Trail semantic check.

Results are local Linux evidence, not hosted CI. An initial command used the wrong working directory and failed before editing; its test attempt also lacked Go cache write access. The corrected edit and all listed candidate Go checks ran with required access.

**Evidence level:** integration-tested on Linux with real Git, fake storage and fake Runner outcomes. Not validated: PostgreSQL runtime, process/server restart, real Provider, secret delivery, public Workflow HTTP/SDK, product startup or native non-Linux behavior. Object recreation is not a process crash. No fairness, unbounded queue scanning, per-Project override, Work persistence or full acceptance claim. #191/#18/M3 remain open.
