# PRJ-002 committed Workflow selection — Linux, 2026-11-12

## Scope and baseline

PR #332 is merged into `feat/18-wp-m3-06-public-api-events` at `727f58bd943d50ec45bf5029ac663c3bf72a28f6`. The integration worktree was fast-forwarded and its `make check` passed locally (exit 0). Main was already current at `fe97930`; the shared user checkout and its unrelated changes were preserved. Accepted [ADR-0062](../../architecture/adr/0062-project-bound-public-operations-and-secret-resolution.md) authorizes this application slice of [#191](https://github.com/rluders/amaru/issues/191), under [#18](https://github.com/rluders/amaru/issues/18).

`attach.WorkflowReader` reads only the selected attached Project's committed Workflow at an explicit full commit ID. `ProjectAccess.Workflow` and `RunWorkflow` authorize before loading and reuse the existing coordinator for Policy, Approval, immutable snapshots and command replay. The [application guide](../../engineering/project-access.md#committed-workflow-selection) describes composition and limits. No dependency, infrastructure, persisted store/migration, Worker/Provider, wire contract or license change.

## Checks

- `make check`: passed locally on Linux, exit 0. Includes Platform/architecture suites, 344 contract tests, Go/TypeScript parity across 342 shared fixtures and 39 SDK tests. Native macOS/Windows and PostgreSQL-tagged vet are compilation signals only.
- From `services/platform`, `go test -race -count=1 ./internal/attach ./internal/coordinator ./internal/domain ./internal/arch`: passed locally, exit 0.
- `git diff --cached --check` and `trail check --base origin/feat/18-wp-m3-06-public-api-events`: passed, no conflicts. Trail advisories were reviewed; no requirements were changed.

These are local results, not hosted CI.

## Evidence levels

- **Integration-tested:** two real temporary Git repositories attached with the existing Attacher; same Workflow ID/Step name, different definitions and Project keys. Selected loads and actual coordinator submissions using fake authoritative stores retain separate Project, actor, revision and snapshot data. Loading/submission leave the Project working trees unchanged.
- **Integration-tested:** missing Workflow/foreign commit, a foreign commit deliberately fetched into the selected object database, non-commit object and wrong clone mapping are rejected. Full lowercase revisions and slug-only IDs prevent option/path selectors. Invalid/future/unknown-field/oversized content, ID mismatch and committed symlinks are refused with safe errors.
- **Integration-tested:** working-tree edits and later committed content do not alter explicit old-revision replay; a changed revision or another Project cannot reuse the same command key. Eight concurrent old-revision replays preserve the original outcome. Snapshots of existing Execution/Step/Event state remain unchanged after rejected operations and replays. A bad Project definition does not stop valid loading/replay in the other Project; this is not dispatch/failure-progress acceptance.
- **Unit-tested:** unattached Project and revoked identity are refused before source access; identity revoked during source loading is rechecked before Submit. A source returning another ID, invalid content or a draft cannot create an Execution. Existing pin validation retains empty-revision support for trusted internal submissions.

## Review and remaining gates

Boundary review: **sound** within accepted ADR-0062. The coordinator imports only domain/ports; the Git adapter handles filesystem/process details through existing process ports. A handler-only filter would leave non-HTTP callers exposed; the common application boundary prevents that. No Client/Worker owns authoritative state and no new vendor-specific interface is introduced. Failed loading produces no command success, Execution or domain Event. Snapshot/outcome persistence remains the existing atomic store operation. Source/clone lookup are trusted composition dependencies.

Not validated: production HTTP/SDK Workflow routes, persisted clone lookup, Project settings and other definition types, Work dependency mutation, secrets resolution/encryption/assignment delivery, independent dispatch under failure/Approval wait, durable-adapter restart, PostgreSQL execution, real Providers/browser behavior or native non-Linux runtime. No store semantics changed, so no new PostgreSQL run was required. Linux is required; other OSes remain best effort. #191, #18 and M3 stay open; this slice does not close their acceptance.
