# Linux Project HTTP PostgreSQL evidence (#191)

Date: 2026-11-13. Runner: Codex, local Linux x86_64 workspace, kernel 7.2.11-100.fc43.x86_64. Base: #18 integration `97e0ad70a2e4b0a8d354a74ce34138e2966b382e` (merged PR #364). Branch: `feat/191-project-http-postgres`. Platform Go 1.27.0 (root launcher 1.26.0), PostgreSQL 17.5, Git 2.56.0, Node v25.7.0, pnpm 11.5.0.

## Selection and scope

GitHub readback confirmed [#191](https://github.com/rluders/amaru/issues/191) open with native blocker #120 and inherited prerequisites #15/#83/#84 closed. Their implementations and accepted ADR-0062/0067 are on the verified #18 integration head. This is the existing child's two-Project integration verification, within the approved public Project mapping; no product startup or new parent scope is implemented. #20 remains blocked by #16–#19 and was not started. Existing branches/worktrees and dirty shared checkout were preserved. The abandoned #170 product-log-coverage worktree/branch received no source edits and was left untouched after the user's stop instruction.

The existing two-client Project HTTP/Git test now accepts the existing storage interfaces and runs unchanged cases against both fake storage and PostgreSQL. The PostgreSQL case creates its own disposable database, applies embedded migrations, and composes the actual Auth, ProjectAccess, Project/settings handlers, Git inspector and PostgreSQL stores. It uses two real loopback HTTP clients: native bearer and browser cookie/Origin/CSRF. This is an automated browser-transport client, not a native browser or generated SDK.

Cases verify distinct identities despite matching names, attach/list/detail and deterministic ordering, settings isolation, retry/second-clone selection with preserved metadata/settings, dirty-config exclusion, foreign root refusal, all-or-error lists, unavailable selected clone without fallback, explicit recovery and logout rejection. The shared suite additionally verifies a rejected settings write after logout leaves the persisted preference unchanged.

After those cases, the PostgreSQL test closes the whole connection pool, opens a new pool and reconstructs application/HTTP adapters. Authenticated in-process HTTP requests verify detail and settings against the saved metadata, selected roots and preferences. A database trigger then fails after clone insert/update. First attachment leaves no orphan Project; replacement-clone attachment leaves the old selection, metadata and settings unchanged. Both HTTP errors contain exactly the fixed internal-error envelope, without the injected private database diagnostic.

Only tests and documentation change. No runtime behavior, storage schema, contracts, dependency, permission or architecture decision changes. The existing `pg` tag keeps the external database requirement out of default checks. Test pools close before their disposable database is dropped; the separate loopback server used for this session is stopped after validation.

## Checks and outcomes

Commands run locally from the repository root unless otherwise specified. PostgreSQL commands use `AMARU_TEST_PG` pointing to the separate disposable loopback server; no credentials are recorded here.

- Initial sandboxed `go test -race -count=1 -tags pg ./internal/adapter/httpapi` from `services/platform`: FAIL before tests because the Go build cache was read-only. Rerun with cache/loopback access passed; this environment failure is not test evidence.
- `go test -race -count=1 -tags pg -v ./internal/adapter/httpapi` from `services/platform`: PASS, exit 0, initial HTTP suite against PostgreSQL. Log: `/tmp/amaru-191-http-pg-focused.log`.
- After strengthening replacement/first-attach rollback and revoked settings assertions, `go test -race -count=10 -tags pg -run '^TestProjects' -v ./internal/adapter/httpapi` from `services/platform`: PASS, exit 0, 100 top-level passes (10 cases repeated ten times), zero failures/skips. Log: `/tmp/amaru-191-http-pg-repeat.log`.
- Final focused `go -C services/platform test -race -count=1 -tags pg -v ./internal/adapter/httpapi`: PASS, exit 0, 21 top-level tests, zero failures/skips. Log: `/tmp/amaru-191-http-pg-final.log`.
- `make check`: PASS, exit 0; includes docs, format/vet/tests, contract/proto checks, 346 matching Go/TypeScript fixture outcomes, 44 SDK tests with zero failures/skips, default Platform tests, architecture checks and PostgreSQL-tag compile/vet. Log: `/tmp/amaru-191-http-pg-check.log`. PostgreSQL runtime evidence comes from the focused tagged commands above, not `make check`.

## Evidence level and limits

**Integration-tested:** real Git, loopback native/browser transport, compact authorization, Project/settings adapters and PostgreSQL together; pool/adaptor recreation; injected transaction failure and fixed error mapping. The fake-backed variant also remains in default checks. These are local results, not hosted CI or real product validation.

**Not validated:** production entrypoint, process/server restart or power loss, generated SDK against the composed product, Execution/Event HTTP transport, Work dependencies, remaining definition types, production secret delivery, Provider availability/real agents, native macOS/Windows or filesystem replacement immunity. No full PostgreSQL suite or crash matrix was rerun: the change adds adapter tests and changes no storage/coordinator behavior. Pool reopen is not a database/server or process restart. #191, #18 and M3 remain open; no merge or issue closure is performed.

Final verification: `make check` after the test/documentation edits PASS, exit 0 (`/tmp/amaru-191-http-pg-check-final.log`), with 346 matching fixture outcomes and 44 SDK passes, zero SDK failures/skips. `make docs-check` and staged whitespace checks passed. The disposable PostgreSQL server stopped successfully.

`trail check --base origin/feat/18-wp-m3-06-public-api-events` examined all five staged files and exited 1 with eight lexical documentation conflicts, not a passing check. Reviewed individually: the five Project-access matches concern filesystem checks in stores, a separate clone migration, post-I/O Workflow authorization, clone-root fingerprinting and auth-before-cancellation ordering; this slice changes none of those implementations and adds no migration. The three validation-guide matches concern Lane/SQL panic recovery, startup cancellation order and Coordinator receive-pump joining; none is changed or claimed verified here. The new rollback test injects a PostgreSQL statement error, not a panic or lifecycle failure. Passing repetitions do not establish absence of unrelated scheduling races. No learning or check was disabled.
