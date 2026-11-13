# PRJ-002 SDK PostgreSQL acceptance — Linux

Date: 2026-11-13. Executor: Codex, on the owner-authorized #191 task.
Base: `a0e379befcdb9a7e633f3071772e99010c3899b5` (#369 merged into #18).
Environment: Linux 7.2.11-100.fc43.x86_64 amd64; Node v25.7.0; PostgreSQL 17.5; Go 1.27.0 for Platform (root Go 1.26.0).

## Scope and architecture review

Verdict: sound. The actual SDK calls existing authenticated Project/settings HTTP adapters backed by existing PostgreSQL stores. The test reuses `projectDatabase`, existing migrations and pinned pgx; no runtime dependency, infrastructure, schema or contract changes. Two temporary real Git repositories remain at their original roots. The SDK has no database access.

The test attaches A and B, updates A's setting, verifies B remains false and both Project resources remain unchanged. It then closes the server/Auth/pool and recreates the pool, stores, coordinator, Auth and HTTP adapters against the retained disposable database. A fresh SDK client verifies list/detail metadata, clone roots and both settings. The compatibility health response is fixture metadata, not production readiness. Credentials travel through the child environment; child diagnostics are discarded to avoid exposing requests or credentials.

The simpler existing Go HTTP PostgreSQL test does not exercise the SDK. The default fake-backed SDK test does not establish PostgreSQL durability. This bounded composition combines those existing paths without changing product startup. The database helper creates and drops only its own uniquely named database; the caller supplies a disposable server and database-creation permission.

## Validation

- `pnpm install --frozen-lockfile` in `packages/sdk`: PASS; existing pinned dependencies only. Initial focused attempt failed because dependencies were absent; installed from the existing lockfile before the passing run. Initial sandbox attempts could not write the shared Go/pnpm cache or bind the test server; required-access runs succeeded.
- From `services/platform`, with `AMARU_TEST_PG` pointing to a newly initialized private Unix-socket PostgreSQL 17.5 server using local peer authentication and no TCP listener: `go test -race -count=1 -tags pg -run TestProjectsPostgresSDKRecreation -v ./internal/adapter/httpapi`: PASS, one top-level test and both write/read subtests, no skips.
- Same server/environment: `go test -race -count=1 -tags pg -v ./internal/adapter/httpapi`: PASS, 22 top-level tests, no skips. Log: `/tmp/amaru-191-sdk-pg-focused.log`.
- `make check`: PASS, exit 0; includes 348 contract tests, 346 matching Go/TypeScript fixture outcomes, 48 default SDK tests, generated-code/type checks and Platform checks. Log: `/tmp/amaru-191-sdk-pg-check.log`. PostgreSQL SDK fixture is typechecked but not executed by the default SDK test glob.
- `make docs-check` and `git diff --check`: PASS, exit 0.
- `trail check --base origin/feat/18-wp-m3-06-public-api-events`: exit 1, three lexical conflicts at `docs/engineering/validation.md:109` in the final staged diff, reviewed individually. The new validation instructions do not change Lane panic unwinding, shutdown readiness ordering or coordinator receive-pump ownership. Those production paths are untouched.

All results are local, not hosted CI. The disposable test database is dropped by the existing helper; the temporary server is stopped after validation.

## Evidence limits

This is SDK/HTTP/Git/PostgreSQL integration evidence. Recreation retains the same database server and Git roots; it does not establish PostgreSQL process restart, crash recovery, product startup/readiness, real browser behavior, Provider execution or full #191/#18/M3 acceptance. Only Linux ran. No merges or issue closures are authorized by this change.
