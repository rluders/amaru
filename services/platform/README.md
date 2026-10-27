# services/platform

Go module `github.com/rluders/amaru/services/platform`: the Platform core. Introduced by WP-M3-01 ([#13](https://github.com/rluders/amaru/issues/13)) under [ADR-0049](../../docs/architecture/adr/0049-adapt-the-prototype-domain-and-ports-into-services-platform.md), which moves selected code from the earlier internal implementation at baseline commit `c8a2f6e` ([ADR-0032](../../docs/architecture/adr/0032-migration-baseline-is-a-pinned-commit.md)). Provenance per file: [PROVENANCE.md](PROVENANCE.md).

## What exists (unit-tested, Linux only so far)
| Path | Content |
|---|---|
| `internal/domain` | Execution and Step Execution state machines (FEAT-EXE-001, [#118](https://github.com/rluders/amaru/issues/118)), per [DOMAIN_MODEL §5.1–5.2](../../docs/architecture/DOMAIN_MODEL.md#5-lifecycles), with the restart rule of [ADR-0042](../../docs/architecture/adr/0042-durable-execution-with-explicit-restart-semantics.md)/[ADR-0044](../../docs/architecture/adr/0044-restart-semantics-clarifications-from-the-m1-spike.md). Standard library only. |
| `internal/domain` (Capabilities) | Typed Capability catalog (FEAT-CAP-001, [#111](https://github.com/rluders/amaru/issues/111)): `CapabilitySpec`/`PortSpec` matching `packages/contracts`, validation, a `Catalog` that accepts further specs, and the built-in specs as data (`catalog.json`, six engineering entries). No Provider exists yet. |
| `internal/domain` (Agents) | Agent definitions (FEAT-AGT-001, [#110](https://github.com/rluders/amaru/issues/110)): `Agent` and `AgentPatch` matching `packages/contracts`, validation, and `AgentHistory`, an append-only record of content-hashed revisions with a stale-revision (lost-update) guard; deleting appends a tombstone revision and keeps every earlier one; a deleted id can be created again as a later revision of the same history. Pure domain: no storage, so nothing is persisted yet. |
| `internal/domain` (snapshots) | Agent and Skill snapshots per Step (FEAT-AGT-004, [#114](https://github.com/rluders/amaru/issues/114)): `ResolvedAgent` taken from an `AgentVersion` (so it carries that version's `revision` and `hash`) with the `SkillUse` records (name, version, content hash) injected into the Step; `StepExecution.BindAgent` sets it once while the Step is pending. The `ResolvedAgent` contract gained required `revision`/`hash` and `skills`. Skill notes, trust and pin enforcement (FEAT-SKL-001) and Provider resolution (FEAT-AGT-002) are not implemented; nothing is persisted yet. |
| `internal/domain` (Work) | Work Packages and Work Items (FEAT-WRK-001, [#166](https://github.com/rluders/amaru/issues/166)): ULID ids and a display key (`WorkKey`, `<PROJECTKEY>-<n>`); Work Item status machine of [DOMAIN_MODEL §5.3](../../docs/architecture/DOMAIN_MODEL.md#53-work-item) (`todo → in_progress → done`, `blocked` from `todo`/`in_progress` and back; `done` final); `SetStatus` accepts only a human, or the delivery loop carrying a Policy authority, otherwise `ErrNotPermitted`; package progress is derived (`ProgressOf`), never stored. Dependencies (FEAT-WRK-002, [#218](https://github.com/rluders/amaru/issues/218)): `DependsOn` lists Work Item ULIDs; `SetDependsOn` rejects unknown, duplicate, self and cyclic edges (`ErrDependencyCycle`, naming the items); `SetStatus` refuses to start an item while a dependency is not done (`ErrDependenciesNotDone`, naming them) and fills the blocked reason with the unmet dependencies when none is given. Workflow link, external issue link and delivery-loop state are not modelled (M4). |
| `internal/store/migrations` | Versioned, forward-only PostgreSQL migrations (FEAT-OPS-002, [#177](https://github.com/rluders/amaru/issues/177)), per [ADR-0051](../../docs/architecture/adr/0051-migrations-run-with-goose-over-an-embedded-directory-with-a-sha256-manifest.md): an embedded directory of goose-format files named `<YYYYMMDDHHMMSS>_<name>.sql` with a sha256 manifest (`migrations.sum`), run with goose as a library (each file in one transaction with its version row, table `amaru_schema_migrations`) under a PostgreSQL advisory lock. Refuses a database newer than the build (`ErrSchemaNewer`, no downgrade, with a message), a file older than an applied one (`ErrOutOfOrder`) and a directory that does not match `migrations.sum` (`ErrChecksum`). The embedded directory is empty: the first real migration arrives with the first adapter that needs a table (FEAT-PRJ-005, [#193](https://github.com/rluders/amaru/issues/193)). Unit tests run in `make check`; the PostgreSQL tests run with `make platform-test-pg` (below). |
| `internal/arch` | Architecture test: the core imports no OS, process, network, vendor or adapter package (negative-tested by adding a forbidden import). |

**Wire shape:** the Agent contract carries `revision` and `hash` (the head's `AgentVersion`), and `UpdateAgent`/delete carry `expectedRevision`, which `AgentHistory` checks. Nothing serves the API yet.

Nothing else of the Platform exists yet: no storage, API, coordinator or Worker. Approval states arrive with the other sub-issues of #13. Display-key allocation (the per-Instance counter) needs storage and arrives with WP-M3-02.

## Check
`make platform-test` (part of `make check`). Tests use no real agents, network or paid APIs.

## PostgreSQL tests
`make check` stays offline. Tests that need a server carry the build tag `pg` and read the DSN from `AMARU_TEST_PG` (a role that may create databases; each test migrates its own throwaway database):

```
AMARU_TEST_PG='postgres://user:pass@127.0.0.1:5432/postgres?sslmode=disable' make platform-test-pg
```

`make check` compiles them (`go vet -tags pg`) but does not run them. Linux only so far: they were run against PostgreSQL 17 on Linux, not on macOS or Windows.

## Migrations
Add a goose-format file `internal/store/migrations/sql/<YYYYMMDDHHMMSS>_<name>.sql` with an `-- +goose Up` section only (migrations are forward-only; a `Down` section is never run), then refresh the manifest with `go test ./internal/store/migrations -update`. Never edit a released file: goose keeps no file hash, so only `migrations.sum` (checked in `go test`) catches an edit made after release, not a live database. ADR-0051 replaces ADR-0023's `atlas.sum` with this manifest because the Platform does not link Atlas (see Dependencies).

## Dependencies
| Module | Version | Licence | Why | Linked into |
|---|---|---|---|---|
| `github.com/pressly/goose/v3` | v3.29.0 | MIT | runs the goose-format migration files, one transaction per file with its version row | `internal/store/migrations` |
| `github.com/jackc/pgx/v5` | v5.11.0 | MIT | PostgreSQL driver (database/sql adapter) | tests tagged `pg` for now |
| `github.com/mfridman/interpolate`, `github.com/sethvargo/go-retry`, `go.uber.org/multierr`, `golang.org/x/sync` and the pgx helpers (see `go.mod`) | as pinned | MIT, Apache-2.0, BSD-3-Clause (from each module's LICENSE file) | transitive | `internal/store/migrations` |

Atlas (`ariga.io/atlas`) was tried first and dropped: its PostgreSQL driver links `hashicorp/hcl/v2` (MPL-2.0), a licence the owner had not accepted for the Platform (ADR-0026). Whether the Atlas CLI can still author these files is unverified without `atlas.sum`.
