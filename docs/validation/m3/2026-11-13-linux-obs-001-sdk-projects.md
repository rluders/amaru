# OBS-001: SDK Project operational log capture (Linux)

Date: 2026-11-13. Run by Codex for Ricardo Lüders. Candidate: the commit containing this record, based on #18 integration `bf3c9f8ff177026f0f2da4ce221dedaa4967fac0` (merged PR #367, head `b7d2010`). Issue: #170; related acceptance: #191/#18.

## Environment

Linux x86_64, kernel `7.2.11-100.fc43.x86_64`; local host, physical/VM classification unverified. Host Go 1.26.0; Platform selects Go 1.27.0. Node v25.7.0, pnpm 11.5.0, Git 2.56.0. No database, browser or agent process used.

## Scope and evidence

**Integration-tested:** the existing SDK fixture composes real authentication/discovery, generated SDK clients, Project/settings HTTP adapters, Git inspection and operational logging with fake storage over loopback. The test now captures both child output channels, awaits process close to drain them, and requires clean shutdown. Native bearer and Node cookie/CSRF clients exercise successful attach/list/detail/settings, clone reselection/recovery, invalid input, missing Projects, auth/Origin/CSRF refusal, conflicts and logout.

Each actual SDK HTTP response must correspond to one JSON operational record. Status multisets agree despite concurrent requests. Every record carries `http.request`, `sdk-fixture`, a timestamp, a fresh nonempty correlation and the appropriate succeeded/rejected outcome. The exercised statuses include 200, 201, 204, 303, 400, 401, 403, 404 and 409. SDK preflight refusals that send no HTTP request require no server record.

Every network request supplies an untrusted correlation marker. Both output channels must omit it, the actual administrator secret, issued launch code, session cookie and value, CSRF token, temporary roots, Project keys, hostile Origin and selected body values. Stderr must be empty; stdout must contain only the expected structured records. Assertions do not print captured output or secret values.

## Commands and outcomes

- `make sdk-ts`: PASS, exit 0; 47 tests, zero failures/skips. Includes generated-schema staleness and TypeScript checks. Initial sandboxed attempt failed before tests because pnpm could not open its existing cache database; rerun with cache access passed.
- Negative control: temporarily inserted a write of the fixture's administrator secret to its captured stdout before serving, then ran `node --test --experimental-strip-types test/auth.test.ts` from `packages/sdk`. Expected FAIL, exit 1, with `Captured diagnostics contain sensitive input`. The mutation was restored before final checks; no credential is printed in the test report or committed.
- Initial `make check`: FAIL, exit 2 at the unchanged `TestFailureAndPanicAreReported` (five-second wait for both Steps to fail). Immediate focused `go test -count=1 -run '^TestFailureAndPanicAreReported$' -v ./internal/worker/embedded` passed; `go test -race -count=100 -run '^TestFailureAndPanicAreReported$' ./internal/worker/embedded` passed. No test, timeout or runtime code was changed. Inspection of the fixture queue, Worker Offer/Accept and coordinator scheduling/settlement found no confirmed cause; the failure remains unresolved. Passing reruns do not prove its absence.
- Final `make check`: PASS, exit 0; includes 346 matching Go/TypeScript fixture outcomes, 47 SDK tests, Platform tests, host/macOS/Windows vet, PostgreSQL-tagged compile/vet, documentation and formatting checks. Local checks are not hosted CI.
- Final `make docs-check` and `git diff --check`: PASS, exit 0.

Local logs: `/tmp/amaru-170-sdk-capture.log`, `/tmp/amaru-170-sdk-negative.log`, `/tmp/amaru-170-sdk-check.log`, `/tmp/amaru-170-sdk-check-rerun.log`, `/tmp/amaru-170-worker-repro.log`, `/tmp/amaru-170-worker-stress.log`. These are local files, not hosted artifacts.

## Orchestration refresh and limits

GitHub open M3 issues, native blockers, issue delivery bases and latest merged PRs were refreshed before publication. #170 blockers #83/#84 and inherited #15 are closed; their changes are on the selected #18 base. #92/#170/#191/#219 target that same integration branch. Provider children remain behind open #80–#82 owner migration gates; #197 waits on #124, #125 on #197, and exit #20 on #16–#19. Other ready work belongs to its designated #16/#19 base. No open competing PR was present at selection. This bounded capture slice advances #170 verification without a new architecture decision or production scaffold.

**Not validated:** product startup or CLI, PostgreSQL SDK/server/process restart, native browser cookie behavior, real Providers, Execution/Event public transport, full product stdout/stderr, native macOS/Windows and Node 22. The capture covers this fixture's exercised channels and inputs, not every possible encoded or unknown secret. Execution/Step correlation is covered separately by existing coordinator/Worker tests, not asserted for these resource requests. No runtime, contract, dependency, schema or architecture change. #170/#191/#18/M3 remain open; nothing is merged or closed by this change.

Trail review: `trail check --base bf3c9f8ff177026f0f2da4ce221dedaa4967fac0` exits 1 with four lexical conflicts in the new Project access documentation. Reviewed individually: (1) test-fixture wording changes no guard-before-cancellation ordering; (2) concurrent capture describes log response ordering, with no second active-clone lookup; (3) clean shutdown/fake storage wording adds no filesystem/path imports to storage; (4) the explicit lack of PostgreSQL/product validation adds no migration, upgrade or compatibility layer. Advisory matches do not establish a fixed Worker timeout; that failure remains explicitly unresolved above. Trail is not reported as passing.
