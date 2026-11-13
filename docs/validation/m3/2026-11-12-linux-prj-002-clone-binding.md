# Linux PRJ-002 atomic clone binding validation

Date: 2026-11-12. Scope: first implementation slice of [accepted ADR-0066](../../architecture/adr/0066-platform-owned-active-project-clone.md), under [#191](https://github.com/rluders/amaru/issues/191) / [#18](https://github.com/rluders/amaru/issues/18). This record does not complete either issue or the full authenticated clone boundary.

## Environment and method

Linux `7.2.11-100.fc43.x86_64`, x86_64; Git `2.56.0`; PostgreSQL `17.5`. Host Go is `1.26.0`; the Platform module selects Go `1.27.0`. Tests use temporary Git repositories, the existing fake stores and isolated migrated databases on disposable PostgreSQL at `127.0.0.1:55435`. No real agents, paid APIs, external repository clone, new dependency or production database is used. The pre-existing service on port 5432 is untouched.

## Evidence

- **Integration-tested:** `Attacher.Attach` canonicalizes a symlinked root, selects a second real temporary Git clone sharing one Project identity, preserves original Project metadata, and leaves selection unchanged after failed validation or cancelled attachment. A changed remote at an occupied root cannot register a second Project. Existing attach regressions still pass, including read-only repository behavior and refusing executable Project content.
- **Unit-tested / integration-tested:** shared `porttest.ProjectCloneStore` runs against fake and PostgreSQL adapters. It covers first registration, same-root idempotence, explicit switching, preserving name/time/settings, foreign-root collision, rollback of new Project registration, existing Project binding preservation, invalid inputs, cancelled reads/writes, concurrent writers and legacy unbound Projects. Stores validate record shape (key match, nonempty/NUL-free root); trusted attach owns native path validation, canonicalization and filesystem/Git inspection.
- **Integration-tested:** PostgreSQL trigger failures after INSERT/UPDATE leave both Project registration and active binding unchanged. A held Project row lock and observed `pg_stat_activity` lock wait establish writer serialization until commit; a blocked writer cancelled before commit cannot change selection. This is deterministic lock-based evidence, not an inference from request timing or a passing concurrency rerun alone.
- **Integration-tested:** two real temporary Git clones attach through the PostgreSQL-backed adapter and share one Project/settings record. Closing the original database connection pool and constructing a fresh pool/store retrieves the selected second root and retained settings. The PostgreSQL server and test process are not restarted by this test.
- **Integration-tested:** upgrade from the previous embedded migration preserves existing Project/settings rows, creates no guessed clone rows, and repeats safely. Inverse SQL drops only the binding table inside a transaction that is then rolled back. The migration runner remains forward-only; no automatic downgrade API is introduced.

Commands from the repository root unless a working directory is stated:

| Command | Result |
| --- | --- |
| `make check` on approved ADR update and merged PR #348 | PASS, exit 0 on both; logs `/tmp/amaru-348-approved-check.log` and `/tmp/amaru-348-merged-check.log` |
| Platform: `go test ./internal/store/migrations -run TestEmbeddedDirectory -update`, followed by `go test ./internal/store/migrations -run TestEmbeddedDirectory` | Manifest regenerated; first invocation reports the old embedded checksum, fresh compilation passes |
| `make check` on this implementation | PASS, exit 0; `/tmp/amaru-191-clone-check.log` |
| Platform: `go test -race -count=10 ./internal/attach ./internal/ports/fakes` | PASS; `/tmp/amaru-191-clone-final-race.log` |
| Platform, disposable `AMARU_TEST_PG` set: `go test -race -count=10 -tags pg ./internal/store/postgres ./internal/store/migrations -run 'TestProjectClone'` | PASS; `/tmp/amaru-191-clone-pg.log` |

The full Platform PostgreSQL suite, `AMARU_TEST_PG` set and `go test -race -count=1 -tags pg ./...`, also passed (exit 0). `make docs-check`, `git diff --cached --check` and `trail check --base origin/feat/18-wp-m3-06-public-api-events` passed. All results are local Linux evidence, not CI.

An initial PostgreSQL conformance run exposed a local-time/UTC structural mismatch. The new registration method now returns UTC; legacy Get/List comparisons verify equal instants without changing those existing APIs. All targeted tests above passed after correction. An interim `make check` caught this record's not-yet-created documentation link. The full architecture test also rejected `path/filepath` in storage packages; native path checks now stay in attach, preserving the existing storage import boundary without exceptions. Final full results are recorded below and with the review PR.

## Remaining acceptance

**Not validated / not implemented here:** authenticated attach or trusted WorkflowReader lookup composition; identity and path revalidation at use; moved/replaced root behavior through that composition; revocation during lookup; immutable Execution clone provenance and recovery; full process/server restart; public Project config/Git inspection/error mapping; HTTP/SDK product startup; remote/Cloud placement; concurrent filesystem replacement immunity. Existing operational tables and execution definitions are unchanged, but that is not evidence of implemented clone provenance. Other OSes have no runtime evidence; Linux is the required M3 target and cross-platform remains best effort.

The binding is metadata, never authorization or a trust grant. A root can become unavailable after selection; callers must fail safely without guessing another root. The complete ADR-0066 boundary requires the remaining tests named in the ADR and production composition. Tool-native Skill discovery remains M7 under ADR-0065. #191/#18 and M3 stay open.
