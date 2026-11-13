# Linux PRJ-002 exact-scope secret metadata resolution

Date: 2026-11-12. Executor: Codex. Platform: Linux 7.2.11-100.fc43.x86_64 x86_64 GNU/Linux. Tools: host Go 1.26.0, Node 25.7.0, pnpm 11.5.0. Base: #18 integration `1f023d39cd4435e7092091a93b1f6709531938ee` (merged PR #345). Scope: [#191](https://github.com/rluders/amaru/issues/191); both #191 and #18 remain open.

## Change and architecture review

Added pure `domain.SecretReference` / `SecretIdentity`, metadata-only `ports.SecretIndex` and `coordinator.SecretResolver`. The Instance Organization and index are trusted composition dependencies. Existing ProjectAccess authorizes attachment/current owner access before and after metadata IO. No existing caller is migrated: this is a new internal operation, with no production composition or wire contract.

Verdict: **sound within accepted ADR-0062**. Domain remains pure; Platform owns resolution; no Worker/Client state or value access is introduced. No dependency, infrastructure, migration, durable storage contract, predecessor code, trust grant or licensing change. A simpler name-only lookup cannot enforce the accepted no-fallback rule. One exact read has no write, transaction, crash-recovery or replay effect; errors return no identity. Revocation during IO withholds even successful metadata. Returned identity is not authority and must not authorize later value access without fresh Policy/Approval checks.

## Regression coverage

`TestExactScopeSecretResolution` uses two fake attached Projects with the same display and secret names, plus Organization and foreign-Organization records. Five subtests check exact identities; one index lookup per request; no Project/Organization/user fallback; case/space preservation; absent and invalid scope; rejected inputs/credentials and cancelled context before IO; revocation during IO; backend error redaction with no partial success; and required trusted dependencies. All fixtures are public test strings. The index has no secret value API.

## Commands and outcomes

Results are local evidence, not hosted CI.

- Merged PR #345 baseline: `make check` — PASS, exit 0 (`/tmp/amaru-345-merged-check.log`).
- Candidate: `make check` — PASS, exit 0 (`/tmp/amaru-191-secret-check.log`), including 42 SDK tests, 348 TypeScript contract tests, 346 matching Go/TypeScript fixture outcomes and all Platform checks.
- `cd services/platform && go test -race -count=1 -run TestExactScopeSecretResolution -v ./internal/coordinator` — PASS, parent plus five subtests, no skips.
- Initial test setup used a wrong working directory/import and failed before running tests; both were corrected. A sandboxed attempt could not write the Go cache; the required-access focused run above passed.
- `make docs-check`, `git diff --cached --check` and `trail check --base origin/feat/18-wp-m3-06-public-api-events` — PASS; Trail reports no conflicts, with one lexical advisory about unrelated historical PostgreSQL concurrency evidence. No PostgreSQL validation is claimed here.

Behavior evidence: fake-backed application integration tested with the race detector; no production or real-tool secret validation.

## Limits

Fake-backed application integration only. No plaintext access/delivery, first-use Policy/Approval integration, encrypted production store/index, redaction pipeline, public HTTP/SDK schema, product startup, real Provider or process restart is validated. No database schema/store semantics changed; PostgreSQL was not run for this slice. Native macOS/Windows were not run; Linux remains the required M3 platform and other OSes best effort. No claim of multi-Organization membership or RLS enforcement.
