# PRJ-002 settings SDK integration — Linux, 2026-11-12

## Scope and source

Refs #191 and #18. Base: merged PR #342, `fbb9c1299746b0e595a104f21369e62a8b9a4f3d`, on `feat/18-wp-m3-06-public-api-events`. Implementation and this evidence ship together on the reviewed PR head.

The existing `authfixture` Go process now composes the real compact authentication, ProjectAccess and settings HTTP adapters over the existing fake stores. Two same-name registered Projects use one remote identity containing `/` and `#` and one local `id:` identity. The generated TypeScript SDK uses native bearer and cookie/CSRF clients against that process on an ephemeral loopback listener. Node supplies Cookie/Origin; no browser is running.

The existing auth negotiation, launch, session and one-use replay assertions remain. Two new subtests cover default false, explicit false, repeated patches, cross-client reads, concurrent updates of different Projects, encoded identity routing, missing Project reads/updates, anonymous reads, SDK rejection of mutations without CSRF, server rejection of wrong CSRF, and unchanged settings after denied requests. Post-logout settings GET/PATCH are rejected while the native client still reads the unchanged value. Assertions check status, response data and no-store headers on reads.

## Checks and outcomes

All results below are local Linux results, not hosted CI.

- Merged base `make check`: PASS, exit 0 (`/tmp/amaru-342-merged-check.log`).
- Initial `cd packages/sdk && node --test test/auth.test.ts`: FAIL as expected on the old fixture, settings GET returned 404. An initial anonymous PATCH assertion also encountered the existing SDK's missing-CSRF refusal; the test now checks that refusal explicitly and uses anonymous GET for server-side 401.
- `cd packages/sdk && pnpm run typecheck && node --test test/auth.test.ts`: PASS, exit 0; three tests (parent and two subtests), zero skips.
- `cd packages/sdk && for i in {1..10}; do node --test test/auth.test.ts || exit; done`: PASS, exit 0; ten independent process launches, three tests each, zero failures/skips (`/tmp/amaru-191-settings-sdk-repeat.log`).
- Candidate `make check`: PASS, exit 0 (`/tmp/amaru-191-settings-sdk-check.log`). Includes SDK generation/typecheck and 42 passing tests, 348 TypeScript contract tests, 346 shared Go/TypeScript fixture outcomes and Platform checks.
- Final `make docs-check` and `git diff --check`: PASS. Trail review passed with no conflicts; lexical advisories refer to unrelated PostgreSQL/OS/shutdown learnings.

Environment: Linux `7.2.11-100.fc43.x86_64`, x86_64; host Go 1.26.0 with the Platform module's Go 1.27 toolchain; Node v25.7.0; pnpm 11.5.0. Physical machine/full VM classification was not independently verified.

## Evidence level and limits

**Integration-tested:** generated SDK, child Go process, real loopback HTTP, real authentication/application/settings handlers; state is fake. This closes the isolated SDK-to-settings transport evidence gap, not the full #191 acceptance gate.

**Not validated:** product startup/CLI, actual browser SDK execution, real Git attachment, PostgreSQL HTTP/restart composition, Skill discovery, other resource families, real Providers, native macOS/Windows and Node 22. These require their own delivery work or unavailable execution environments. No store, migration, wire shape, dependency, configuration syntax, trust grant or predecessor import changes. #191/#18/M3 remain open.

The composition reuses accepted ADR-0062/0064 semantics and preserves the health/auth guards and operational logging. Platform owns settings; SDK clients only use the public resource. No new architecture decision is introduced.
