# Linux PRJ-002 application access evidence — 2026-11-12

Scope: #191's first implementation slice, under #18. Accepted [ADR-0062](../../architecture/adr/0062-project-bound-public-operations-and-secret-resolution.md) was approved by Ricardo Lüders and merged through [PR #330](https://github.com/rluders/amaru/pull/330), commit `e1f5e7935776f4fd0536cd41e5909c715836b30e`. The README diagram merged separately through [PR #331](https://github.com/rluders/amaru/pull/331), commit `fe979304525614b82b7e2f2886e1ceea625a2ad2`. The implementation starts from the #18 integration branch after syncing main, `a5016d5`.

## Commands and outcomes

Local Linux results, not hosted CI:

- `make check` on accepted #330: passed, exit 0; `/tmp/amaru-330-accepted-check.log`.
- `make check` on merged #331/main: passed, exit 0; `/tmp/amaru-331-merged-check.log`.
- `make check` on the merged/synced #18 base: passed, exit 0; `/tmp/amaru-330-synced-check.log`.
- In `services/platform`, `go test -count=1 -run 'TestProjectAccess|TestPrincipalRequires|TestAuthenticatedProjectAccess' ./internal/coordinator ./internal/adapter/localauth`: passed, exit 0, both packages; `/tmp/amaru-191-access-boundary.log` (initial focused run).
- In `services/platform`, `go test -race -count=1 ./internal/coordinator ./internal/adapter/localauth ./internal/arch`: passed, exit 0, both complete affected package suites and the architecture suite, including the final focused cases; `/tmp/amaru-191-access-race.log`.
- Candidate `make check`: passed, exit 0; `/tmp/amaru-191-access-check.log`. Includes 344 contract tests (342 shared fixtures with Go/TypeScript parity), 39 SDK tests, formatting, architecture checks, native Linux tests and cross-platform/tagged vet compilation. The initial run caught a coordinator-to-Evidence implementation import; the final code uses `ports.EvidenceReader` and preserves the existing dependency rule.
- Candidate `git diff --cached --check` and `trail check --base origin/feat/18-wp-m3-06-public-api-events`: passed, exit 0. Trail reviewed all 11 staged files with no conflicts. Six lexical advisories refer to unrelated native GitHub metadata, unchanged Linux-first M3 scope or a previous contract-runner workaround; no new requirement or workaround was added.

## Demonstrated behavior

**Unit-tested with fake stores and real domain/coordinator code:** `TestProjectAccess` registers two stable Project keys with identical display/Step names. Each receives a distinct Execution and Approval. Public request fields cannot replace the authorized Project, local administrator or manual trigger. Owned queries/Evidence/pinned definitions succeed; foreign and absent Execution/Approval IDs return the identical not-found value and empty output. Foreign Evidence is refused even with a nil Evidence reader; foreign Event filters are refused before stream access or resync cursor return.

The test records a successful decision and cancellation in Project B, then attempts to replay their keys under Project A. Both are refused before returning outcomes or mutating rows, Events or audit. Same-key Submit under another Project conflicts without allocating an Execution/Approval. Same-Project Submit replay preserves the original outcome after state changes. Twelve concurrent cancellation replays are read-only. Current authentication denial applies to reads and all three mutations before replay. Empty, invalid and unattached Project selectors and non-owner/non-human attribution are refused. Execution, Step, Approval, Event and audit snapshots demonstrate rejected/duplicate command invariance.

**Unit-tested with real auth middleware:** `TestPrincipalRequiresLiveAuthenticatedContext` covers credential-free health, unauthenticated spoofed identity headers, native and cookie identities, another Auth instance, cancelled request context, logout, expiry and shutdown. `TestAuthenticatedProjectAccessRechecksLogoutBeforeReplay` composes the actual auth guard, `Auth.Principal` and `ProjectAccess` around fake stores in a test handler. A valid browser session submits and cancels; after logout its captured context cannot retrieve the recorded cancellation outcome or mutate the Execution. Existing Origin, CSRF and socket-lifetime suites remain enabled.

**Source review:** the [operation inventory](../../engineering/project-access.md) maps the complete OpenAPI operation families and seven internal mutating coordinator commands to this slice or named remaining gates. Existing stores already preserve Execution Project identity; no schema, migration, persisted store contract or persisted command normalization changed. Internal Instance-wide scheduler/recovery methods remain available. No predecessor code, dependency, infrastructure or license change.

## Not validated and remaining gates

This slice does not complete #191. Projects here are registered fake records, not two attached temporary Git trees. Project-selected settings/definitions and cache identity, Work/dependency isolation, matching-name secret resolution and authorized encrypted production secret delivery, independent dispatch under failure/wait, direct Artifact/list access and composed two-client API/SDK acceptance remain pending. Submit's Workflow/Steps and Evidence/Event dependencies are trusted internal inputs; future product handlers must select/load them for the authorized Project, never accept them as client authority. Do not expose raw coordinator methods as public handlers.

No durable-adapter changes were made and no PostgreSQL server suite was run for this slice; `make check` compiles tagged PostgreSQL tests but does not execute them. The auth composition uses an in-process test handler, not a new real HTTP server or new Firefox run. No new runtime, real Provider/agent or paid API validation. No macOS/Windows execution; Linux remains required and the other OSes best effort under the recorded M3 scope. #191, #18 and M3 remain open.
