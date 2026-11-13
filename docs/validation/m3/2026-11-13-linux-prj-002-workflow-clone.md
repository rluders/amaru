# Linux PRJ-002 persisted Workflow clone lookup

Date: 2026-11-13. Base: `509e5f01c7efc0129526b42d68d56fd8c850f279` (merged PR #349), parent integration `feat/18-wp-m3-06-public-api-events`. Candidate: implementation commit containing this record. Agent-run local Linux x86_64 environment; Go module toolchain 1.27.0, Git 2.56.0, disposable PostgreSQL 17.5 on loopback. Local evidence, not hosted CI.

## Change and review

`attach.WorkflowReader` uses the persisted active clone from the Attacher's Instance store instead of an arbitrary callback. `ProjectAccess.Workflow` withholds definitions after current authorization is lost during source IO. Tests and the engineering inventory are updated together. No schema, migration, dependency, infrastructure, public route or contract change. No compatibility reader or alias is retained before Amaru v1.0.

Architecture review: **sound** within accepted ADR-0062/0066. Platform owns binding selection; the adapter owns native Git/path checks; domain and Worker boundaries are unchanged. The simpler alternative of leaving Location injected would keep production selection unspecified. Capturing one root avoids switching midway through the read; missing/moved/foreign roots fail closed. Lookup failure emits a fixed error; cancellation and revoked authorization withhold content. Reads make no mutation, so failure/retry has no partial write. The compact native root is never a remote Worker instruction. No new human decision is required by this approved implementation slice.

## Checks

- Merged base: `make check` — PASS, exit 0; `/tmp/amaru-349-merged-check.log`.
- Candidate: `make check` — PASS, exit 0; `/tmp/amaru-350-check-final.log` and `/tmp/amaru-350-check-final.exit`.
- `go test -race -count=10 ./internal/attach ./internal/adapter/localauth ./internal/coordinator` — PASS, all three packages (11.485s, 1.254s, 239.835s); `/tmp/amaru-350-race.log`. Final `go test -race -count=1 ./internal/attach ./internal/adapter/localauth ./internal/coordinator` — PASS, exit 0; `/tmp/amaru-350-race-final.log` and `.exit`.
- With `AMARU_TEST_PG` pointing to disposable loopback PostgreSQL: `go test -race -count=10 -tags pg ./internal/store/postgres -run TestProjectCloneGitAndReopen` — PASS, exit 0, 2.808s; `/tmp/amaru-350-pg.log`. The server was stopped after validation.
- `make docs-check` and `git diff --check` — PASS. Initial sandbox-only attempts could not write the Go cache or use test networking; the above commands ran with those permissions available.
- `trail check --base origin/feat/18-wp-m3-06-public-api-events` — exit 1, two lexical conflicts reviewed. Both concern project-access.md mentioning existing Project storage and clone paths. Neither restores legacy support: existing Project-only callers remain valid; the approved ProjectCloneStore now supplies Workflow roots; no schema or compatibility path changes. This rationale is also recorded in the PR and commit. Native path validation stays in attach, not storage.

## Covered behavior

- **Integration-tested:** real temporary Git Projects retain separate definitions with matching IDs, explicit revision pinning and bounded regular-blob validation. All callers now use the same clone store as attach.
- **Integration-tested:** reattachment after binding capture leaves the in-flight read at its captured root; the next read selects the new clone. A commit exclusive to the prior clone fails instead of falling back. Moving the active root or changing its repository identity fails even when the other clone remains available. Missing bindings fail without inventing a path.
- **Unit-tested:** controlled authentication withholds a successfully loaded definition after revocation; already revoked/unattached requests never look up a clone. Real compact Auth callbacks additionally exercise logout, exact expiry, cancellation and shutdown during source IO, withholding content and rejecting later source access.
- **Integration-tested:** after PostgreSQL connection-pool close/reopen, WorkflowReader loads a definition commit present only at the persisted selected clone. This is not a PostgreSQL server or process restart.

## Limits and remaining work

Not validated: native macOS/Windows, process/server restart, concurrent filesystem replacement immunity, real Providers, product HTTP/SDK/startup and complete two-Project product acceptance. Git tests use controlled authentication; Auth lifecycle tests use callback sources. No claim of a single product end-to-end composition is made.

Execution creation still pins the definition/revision but does not persist clone provenance. Implement immutable provenance and recovery next under ADR-0066; reattachment must never substitute an existing Execution's source. Authenticated public attach and public Project config/Git/error mapping remain pending, as do independent secret Policy/Approval/encrypted delivery, Work/dependency and other #191/#18 gates. #191/#18/M3 remain open. Existing data is preserved; no legacy upgrade or downgrade work is added.
