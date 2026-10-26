# spikes/wp-m1-02 — storage and execution validation spike

**Purpose.** Executable experiments for [WP-M1-02](../../docs/architecture/WORK_PACKAGES.md#wp-m1-02-storage-and-execution-validation-spike) ([#3](https://github.com/rluders/amaru/issues/3)): validate [ADR-0015](../../docs/architecture/adr/0015-postgresql-is-the-only-database-bundled-in-compact-installs.md), [ADR-0016](../../docs/architecture/adr/0016-nats-jetstream-message-bus-embedded-in-compact-installs.md) and [ADR-0042](../../docs/architecture/adr/0042-durable-execution-with-explicit-restart-semantics.md) (checks A2, A9, A12). **This is not the Platform.** Nothing here is production code, nothing is migrated from an earlier prototype, and the module is deleted or replaced when `services/platform` is introduced (M3, by ADR).

**Owner:** product owner. **Report:** [investigations](../../docs/architecture/investigations/). **Validation target:** `make spike-vet` (part of `make check`) and `make spike-m1-02` (needs a bundle, runs the real experiments).

## Dependencies (spike module only; pinned in `go.mod`)
| Module | Version | Why | License |
|---|---|---|---|
| `github.com/fergusstrange/embedded-postgres/v2` | v2.0.1 | PostgreSQL supervisor under test (A12, Q28) | MIT |
| `github.com/nats-io/nats-server/v2`, `github.com/nats-io/nats.go` | v2.15.1, v1.55.0 | embedded JetStream under test (A9, ADR-0016) | Apache-2.0 |
| `github.com/jackc/pgx/v5` | v5.11.0 | PostgreSQL driver for the experiments | MIT |

## Layout
- `cmd/spike` — `bundle` (installer stand-in), `pg` (A2/A12 scenarios), `nats` (embedded footprint and file handling), `platform` (crash-injection harness).
- `internal/pgsup` — starts the bundled PostgreSQL the way ADR-0015/Q28/Q29 describe it.
- `internal/bus`, `internal/outbox` — embedded JetStream, transactional outbox, relay, idempotent consumer.
- `internal/wf` — durable execution state machine and the crash-injection matrix.

## Run on one OS
Needs Go 1.26, network only for `bundle`. Use a **non-admin** user. Paths below are examples.

```sh
cd spikes/wp-m1-02
go build -o spike ./cmd/spike
./spike bundle -out ./bundle                       # build-time fetch: PostgreSQL 18.6.1 distribution + supervisor
./spike pg -bundle ./bundle -state ./state-pg -out pg.jsonl      # A2/A12 scenarios, prints one JSON line each
./spike nats                                                      # embedded JetStream footprint + file handling
./spike matrix -bundle ./bundle -work ./mx -out matrix.jsonl -fuzz 10   # crash-injection matrix, Approval waits, leases, timers
SPIKE_BUNDLE=$PWD/bundle go test -count=1 -v ./internal/...        # outbox/relay/dedup and coordinator tests
```

The per-OS runbook is [docs/validation/m1/RUNBOOK.md](../../docs/validation/m1/RUNBOOK.md). A run counts for an OS only when it executes natively there ([validation](../../docs/engineering/validation.md#cross-platform-validation-m3-and-m4)).
