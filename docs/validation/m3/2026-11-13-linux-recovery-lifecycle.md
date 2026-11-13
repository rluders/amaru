# Linux recovery completion gate (#18)

Date: 2026-11-13. Runner: Codex in the local Linux x86_64 workspace (machine kind unverified). Base: #18 integration `cf2a97d` (merged PR #360); branch `feat/18-recovery-lifecycle`. Platform Go 1.27.0, PostgreSQL 17.5. Local results are not hosted CI.

## Scope and boundary review

Verdict: sound under accepted [ADR-0068](../../architecture/adr/0068-platform-process-readiness-and-bounded-shutdown.md). The concrete `internal/platform.Recovery` owner acquires the already-owned pool through Storage readiness, constructs one Coordinator with the existing PostgreSQL Execution store and configured Coordinator options, and invokes existing Reconcile once. Construction acquires nothing. Its completion latch supports WaitReady before Start and withholds the Coordinator on failure, panic or cancellation. All initialization shares Storage's absolute deadline. Downstream execution owners must obtain this same Coordinator after readiness and join before Storage closes the pool. Register recovery after schema/messaging and before background services, Workers and HTTP; these remaining owners are not composed here.

Stop joins reconciliation within its cleanup context, including after partial initialization; it owns no pool and neither retries reconciliation nor undoes commits. Critical failure still terminates the process without in-process restoration/restart. Durable reconciliation on a later fresh startup preserves ADR-0042/0044 semantics. No domain/port change, new dependency, schema change, signal handler, supervisor, infrastructure or public contract. A fire-and-forget reconciliation function would admit traffic too early; accepting a preconstructed PostgreSQL Coordinator would require its pool to exist before Storage.Start acquires it. This owner constructs and publishes the actual instance at the acquisition boundary, without a general factory framework.

## Checks and evidence

- `go test -race -count=10 ./internal/platform ./internal/arch` from `services/platform`: PASS, exit 0; log `/tmp/amaru-recovery-lifecycle-race.log`.
- Against the isolated disposable PostgreSQL, `go test -race -count=3 -tags pg -run TestPostgresStorageSchemaLifecycle -v ./internal/platform`: PASS, exit 0, three runs and no skips; log `/tmp/amaru-recovery-lifecycle-pg.log`. Each run creates/drops a fresh database and recreates pool/schema/recovery/Lane owners three times. Recovery precedes dependent admission; the ordinary running Step becomes interrupted, the idempotent running Step becomes queued and both lose their old Leases. A later fresh owner leaves the entire reconciled Execution, Steps and Events unchanged. The middle run fails in a later stage and still cleans up its pool; the external server remains available.

- Final `go test -race -count=1 ./internal/platform`: PASS, exit 0.
- Candidate `make check`: PASS, exit 0, including docs-check, matching contract fixtures, 44 SDK tests and all default Platform suites; log `/tmp/amaru-recovery-lifecycle-check.log`. Windows/macOS cross-vet and PostgreSQL-tagged vet passed; these are not native/runtime validation.
- Final `make docs-check`, staged whitespace check and `trail build`: PASS, exit 0. The disposable validation server was stopped with its own pg_ctl after testing.
- `trail check --base origin/feat/18-wp-m3-06-public-api-events`: exit 1, three documentation matches against the SQL driver panic learning. They describe the real reconciliation stage and pinned test dependency, not SQL panic repair. Existing driver-panic subprocess tests remain unchanged; no resource cleanup after a broken driver is promised. The independent watchdog and fatal exit remain required. Other informational matches do not change authorization, storage path, schema, compatibility or Linux-first rules. These lexical conflicts are also explained in the commit/PR.

Evidence: **unit-tested** constructor validation, readiness waiting, accessor cancellation, startup errors/deadline/panic, and bounded join with a driver ignoring cancellation; **integration-tested** actual Lane/fake SQL and real PostgreSQL reconciliation/stage ordering. The initial cancellation assertion incorrectly expected Lane to fail intentional root cancellation: Lane returns success after clean cancellation cleanup. The corrected test requires recovery readiness/Coordinator to remain unavailable and all acquired resources to close; genuine error, deadline and panic cases still require a Lane failure. No production code was changed to hide that result.

The first validation-only PostgreSQL start omitted its explicit port and failed because port 5432 was occupied. Retrying with this disposable server's recorded loopback port succeeded; the existing server was untouched. No credential values are recorded.

## Limits

No production entrypoint, independent aggregate startup watchdog, bundled PostgreSQL owner, NATS/outbox, background/execution/HTTP owners, admission/discovery or diagnostic consumer is composed. Default fake SQL uses an empty Execution list; nontrivial durable state is tested separately against PostgreSQL. SQL driver panics can still strand database/sql ownership: the existing subprocess cases and [storage investigation](2026-11-13-linux-storage-lifecycle.md) remain applicable. Cancellation-ignoring IO cannot be forced to join; timeout requires process failure, not graceful-cleanup claims.

Owner/pool recreation against a running database is not process/server restart. Native macOS/Windows, real Provider, public SDK/product composition and production crash/failure-matrix acceptance remain **not validated**. #18/#92/#170/#191 and M3 remain open. No new human architectural decision is requested.
