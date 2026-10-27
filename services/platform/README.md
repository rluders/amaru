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
| `internal/store/migrations` | Versioned, forward-only PostgreSQL migrations (FEAT-OPS-002, [#177](https://github.com/rluders/amaru/issues/177)), per [ADR-0023](../../docs/architecture/adr/0023-self-hosted-oci-image-atlas-migrations-operator-owned-backup.md): an embedded goose-format directory with `atlas.sum`, read and executed with Atlas as a library (`ariga.io/atlas`), a revisions table (`amaru_schema_revisions`), an advisory lock, and one transaction per file together with its revision row. Refuses a database newer than the build (`ErrSchemaNewer`, no downgrade), an edited applied file (`ErrHistoryChanged`) and a directory that does not match `atlas.sum` (`ErrChecksum`). The embedded directory is empty: the first real migration arrives with the first adapter that needs a table (FEAT-PRJ-005, [#193](https://github.com/rluders/amaru/issues/193)). Unit tests run in `make check`; the PostgreSQL tests run with `make platform-test-pg` (below). |
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
Add a goose-format file `internal/store/migrations/sql/<YYYYMMDDHHMMSS>_<name>.sql` with an `-- +goose Up` section only (migrations are forward-only; a `Down` section is ignored), then refresh the integrity file with `go test ./internal/store/migrations -update` (or `atlas migrate hash --dir-format goose`). Never edit a released file: `ErrHistoryChanged` refuses it.

## Dependencies
| Module | Version | Licence | Why | Linked into |
|---|---|---|---|---|
| `ariga.io/atlas` | v1.3.0 | Apache-2.0 | migration directory, checksum and executor as a library (compact installs cannot require a CLI, ADR-0023) | `internal/store/migrations` |
| `github.com/jackc/pgx/v5` | v5.11.0 | MIT | PostgreSQL driver (database/sql adapter) | tests tagged `pg` for now |
| `github.com/hashicorp/hcl/v2` and others (see `go.mod`) | as pinned | MPL-2.0 (hcl); MIT, Apache-2.0 or BSD (the rest, from each module's LICENSE file, `go-openapi/inflect` not checked) | transitive, through Atlas' PostgreSQL driver (`schemahcl`) | `internal/store/migrations` |

The MPL-2.0 module is a transitive consequence of linking Atlas' driver; whether that is acceptable for the Platform's licensing (ADR-0026) is an open owner decision. `go-sqlite3`, listed in Atlas' `go.mod`, is not linked.
