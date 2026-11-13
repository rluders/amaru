# Linux PRJ-002 authenticated secret metadata lifecycle

Date: 2026-11-12. Runner: Codex on the owner's Linux x86_64 workstation. Base: merged PR #346, `3ddf21696277577f5c06c6d44b705c6b0012b537`, on #18 integration. Candidate: the accompanying `feat/191-secret-auth` commit. This record concerns #191/#18; neither is complete.

## Change and review

Adds `services/platform/internal/adapter/localauth/secret_test.go` and the corresponding Project access documentation. No production code, contracts, schemas, dependencies, infrastructure, migrations or permission changes. Existing real auth helpers establish guarded request contexts; the production application boundary and metadata resolver run over fake Project/existence stores. No secret values exist in this fixture, and authentication credentials are neither printed nor persisted.

Architecture review: **sound within accepted ADR-0062**. Platform authorization remains authoritative, domain and Worker boundaries are unchanged, and exact Organization/Project/name identity is never delivery authority. The simpler mocked-auth-only test already exists but cannot establish real session logout/expiry/shutdown behavior across index IO. This read-only test introduces no transaction, crash/replay or deployment semantics and requires no new decision.

## Evidence

The parent regression has ten subtests: five lifecycle states (live, logout, exact absolute expiry, request cancellation, Instance shutdown), each with Project and explicit Organization references. Three independent authenticated contexts exercise matching names in two fake-attached Projects; exact one-call lookup assertions and zero returned identity on refusal prevent hidden fallback/partial success. Missing Project secrets fail despite matching foreign/Organization names. Explicit Organization access still refuses an unattached Project. Unauthenticated and another Instance's contexts do not read the index. In-flight rejection and repeated rejection are checked, together with independent client survival where applicable. Expiry uses an injected clock and lifecycle changes occur synchronously inside the index callback: no sleeps or timing race assumptions.

Commands on Linux (local evidence, not hosted CI):

- Integration base `make check`: PASS, exit 0 (`/tmp/amaru-346-merged-check.log`).
- `cd services/platform && go test -race -count=10 -run TestAuthenticatedSecretResolution -v ./internal/adapter/localauth`: PASS, exit 0 (ten repetitions, ten subtests each, no skips; `/tmp/amaru-191-secret-auth-race.log`).
- Candidate `make check`: PASS, exit 0 (`/tmp/amaru-191-secret-auth-final-check.log`; 42 SDK tests, 348 TypeScript contract tests, 346 matching Go/TypeScript fixture outcomes and all Platform checks).
- Final `git diff --cached --check`, `make docs-check` and staged `trail check --base origin/feat/18-wp-m3-06-public-api-events`: PASS, exit 0; no Trail conflicts. Two lexical advisories were reviewed: no PostgreSQL validation or three-OS requirement is claimed.

Regression sensitivity check: temporarily removing only the post-index authorization check makes `go test -count=1 -run TestAuthenticatedSecretResolution/logout/project ./internal/adapter/localauth` FAIL (exit 1, successful identity incorrectly returned). The source was restored byte-for-byte immediately; no production change is included. Log: `/tmp/amaru-191-secret-auth-mutation.log`.

Environment: Linux 7.2.11-100.fc43.x86_64, x86_64; host Go 1.26.0 with module toolchain selection, Node 25.7.0, pnpm 11.5.0.

## Limits and next gates

Evidence level: **fake-backed application/auth integration-tested**, not real validation. Requests use in-process HTTP recorders; retained contexts deliberately model authorization expiring during lookup. No product endpoint, real browser/SDK, actual Git attachment, PostgreSQL, concurrent session mutation, process restart, real Provider or native macOS/Windows execution was validated. Production state and storage are unchanged, so a PostgreSQL run is not required for this test-only slice. Linux is required for M3; other OSes remain best effort.

Metadata resolution does not authorize secret use. Selected-identity/Capability Policy and first-use Approval, encrypted production storage/index, redaction and authorized assignment delivery remain open. Other definition/configuration, authoritative Work/dependencies, clone lookup and composed two-client product acceptance gates remain unchanged; tool-native Skill discovery/import stays M7 under ADR-0065.
