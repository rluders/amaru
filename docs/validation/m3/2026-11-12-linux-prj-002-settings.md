# Linux PRJ-002 discovery settings validation

Date: 2026-11-12. Runner: Codex on the owner's Linux development host (not hosted CI). Base: `aca50915c0f9b6418251845eeb9894a117f2241d` (#18 integration). Implementation commit: `e146d53787c55ee02a358ddb7ba1b01a07dd7bac`. Scope: fresh implementation under accepted ADR-0064; no Weave code import.

Environment: Linux 7.2.11-100.fc43.x86_64, x86_64; Go host 1.26.0 (Platform module toolchain 1.27), Node v25.7.0, pnpm 11.5.0, disposable PostgreSQL 17.5 on loopback port 55435. Native macOS/Windows were not run; M3 requires Linux and treats other OSes as best effort.

## Commands and outcomes

- `make check`: PASS, exit 0. Final local log `/tmp/amaru-191-settings-check-final.log`. Includes Go tests/vet, architecture checks, documentation checks, 348 TypeScript contract tests, 346 shared fixtures with identical Go/TypeScript outcomes, and generated SDK/typecheck plus 40 SDK tests. Logs are temporary local artifacts, not published CI evidence.
- From `services/platform`: `go test -race -count=1 -run 'TestSettingsStore|TestProjectSettingsAccess|TestParseUpdateSettings' ./internal/ports/fakes ./internal/coordinator ./internal/domain`: PASS, all three packages. Final coverage includes concurrent first writes; the fake retains in-memory state across adapter reuse.
- With `AMARU_TEST_PG` set to the disposable PostgreSQL DSN, `make platform-test-pg`: first run FAIL (exit 2 from make), unrelated Worker session cleanup cancellation in `TestLimitsWithPostgres/a_failed_result_and_a_refused_attempt_free_their_slots`; one full rerun PASS, exit 0. Logs `/tmp/amaru-191-settings-pg.log` and `/tmp/amaru-191-settings-pg-rerun.log`.
- From `services/platform`, with the same disposable DSN: `go test -race -tags pg -count=1 -run 'TestSettingsStore|TestSettingsRollback|TestProjectSettingsUpgrade' ./internal/store/postgres ./internal/store/migrations`: PASS, both packages. Log `/tmp/amaru-191-settings-focused-pg.log`.
- Unchanged integration base reproduction: `go test -tags pg -count=20 -run 'TestLimitsWithPostgres/a_failed_result_and_a_refused_attempt_free_their_slots' ./internal/store/postgres` with the disposable DSN: FAIL, four of twenty repetitions reported a wrapped context cancellation during policy/store reads. This establishes the independent baseline defect [#339](https://github.com/rluders/amaru/issues/339); no shutdown behavior or test suppression is included in the settings change.

## Evidence level and limits

**Unit-tested:** strict settings JSON parsing and nonempty typed patches; application authorization before store access, foreign/unattached selectors, current owner role and revoked-session refusal. Rejected writes preserve both Projects.

**Integration-tested with fake and PostgreSQL adapters:** default false for attached Projects, two Projects with identical display names and independent settings, repeated explicit boolean updates, invalid/missing/cancelled operations, concurrent first writes, final committed explicit value and reads after adapter recreation. PostgreSQL-specific tests force an AFTER UPDATE failure and verify rollback, then upgrade a pre-settings database twice and verify preserved Project data with no invented preference records. This is actual database behavior, not a simulated SQL implementation. Adapter recreation uses the same running database; it is not a server/process restart test.

**Contract/SDK tested:** unchanged boolean payloads, explicit false, rejected null/string/extra fields, generated-description staleness and selected-Project GET/PATCH paths against fake fetch. These do not prove a running HTTP resource.

**Architecture review: sound for this slice.** Platform retains the preference; domain types/patch parsing remain pure; existing store adapters own persistence. No new runtime dependency, infrastructure, trust grant, config grammar, Event contract or implicit file mutation. The upsert provides the approved last-committed-value semantics for the only existing switch. A future field must preserve omitted fields atomically; version/replay semantics require reviewed contract work.

**Not validated:** product HTTP serving, actual Skill discovery at attached roots, composed two-client API isolation, multi-Organization authorization, real Providers, PostgreSQL process-kill/server restart and native non-Linux behavior. No new real-tool validation is claimed. Settings do not complete #191, #18 or M3. The independent shutdown defect remains open as #339.
