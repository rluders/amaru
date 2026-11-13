# Validation

What can be checked locally today, and how to report it.

## Commands

| Command | What it does | Needs |
|---|---|---|
| `make check` | Runs all of the below. | Go ≥ 1.27, or Go ≥ 1.21 with `GOTOOLCHAIN=auto` (downloads 1.27) |
| `make fmt` | Fails if any Go file under `tools/doccheck`, the spike, `packages/contracts` or `services/platform` is not `gofmt`-formatted. | Go |
| `make vet` | `go vet` on `tools/doccheck`. | Go |
| `make test` | Unit tests of `tools/doccheck` (`-count=1`). | Go |
| `make contracts-test` | `go vet` and tests in `packages/contracts`: every schema parses and has a unique `$id`, every `$ref` resolves, `operationId`s are unique, no prototype names remain, and the route-drift guard (`routeguard`). Also validates the shared fixture corpus (`fixtures/manifest.json`, schemas and Worker Protobuf) in Go: declared outcome and error id per fixture, manifest and directory in sync, coverage of each area, no secret-like values or prototype names in fixtures (API-002). | Go, `buf` |
| `make contracts-proto` | In `packages/contracts`: `buf lint`, `buf format -d --exit-code`, and `buf breaking` against `baseline/worker-v1.binpb`. | `buf` (verified with 1.69.0) |
| `make contracts-ts` | In `packages/contracts`: `pnpm install --frozen-lockfile`, `tsc --noEmit`, the same fixture corpus validated with Ajv (Draft 2020-12) and `@bufbuild/protobuf`, and a parity check: Go and TypeScript each write one `fixture<TAB>outcome` line per fixture and the two files must be identical. | Node >= 22.18, pnpm (verified with Node 25.7, pnpm 11.5) |
| `make sdk-ts` | In `packages/sdk`: `pnpm install --frozen-lockfile`, the staleness check (regenerates `src/generated/schema.ts` from `packages/contracts/openapi` with `openapi-typescript` and fails if the committed file differs), `tsc --noEmit` (includes compile-time checks that unknown routes and missing parameters are type errors), and unit tests of the typed client (fake `fetch`) and the event stream (fake sockets: cursor resume, gap, resync, backoff, give-up). | Node ≥ 22.18, `pnpm` |
| `make platform-test` | In `services/platform`: `go vet` for the host and, as an early signal only, for Windows and macOS; `go vet -tags pg` (compile check of the PostgreSQL tests) and `go test` of the domain state machines (every state pair against the documented lifecycle) and the architecture test (core imports no OS, process, network, vendor or adapter package). | Go |
| `make platform-test-pg` | In `services/platform`: `go test -tags pg`, the PostgreSQL-backed adapter tests (the coordinator crash-injection matrix and Linux child-process kill suite, the Worker-interface assignment suite, the idempotent-command and concurrency-limit suites and the `ExecutionStore` behavior suite against PostgreSQL; migration runner: fresh and repeated apply, forward upgrade, downgrade refusal, rollback and retry of a failing file, out-of-order refusal, concurrent runners, lock timeout). Needs `AMARU_TEST_PG`, a DSN whose role may create databases; **not part of `make check`** (`make check` only compiles them with `go vet -tags pg`). Linux only so far. | Go, a PostgreSQL server |
| `make docs-check` | Runs `doccheck` on the repository: relative links and anchors, unclosed fences and empty Mermaid blocks, ADR names/frontmatter/numbering and the DECISIONS index, Skill frontmatter and `.claude/skills` symlinks. | Go |

`doccheck` is offline: it does **not** check external URLs, Mermaid syntax (only that blocks are non-empty and closed), spelling, or whether prose is true. Those need review (`documentation-maintenance` Skill).

No hosted CI is configured; local results are not CI results.

## Evidence levels
Use these words in reports and docs:
- **Unit-tested**: automated tests with fakes cover the behavior.
- **Integration-tested**: automated tests exercise real components together (real process, real files, real network on loopback).
- **Real-validated**: a recorded manual or end-to-end run against the real tool or service, with date, versions and result written down in the repo.
- **Not validated**: none of the above. Say so.

A test that was not run is not evidence. A green local run is not CI. A fake-backed test is not real validation.

## Reporting
State for each check: exact command, outcome (pass/fail, counts), and anything skipped and why (missing tool, no network, platform not available).

## Cross-platform validation (M3 and M4)

Status: **adopted for planning**, amended by the product owner on 2026-11-12. Linux is the required M3 acceptance platform; native macOS and Windows validation is best effort and missing evidence does not block M3 integration or exit. M4 retains its native three-OS exit requirements ([roadmap M3](../architecture/ROADMAP.md#m3-platform-core), [M4](../architecture/ROADMAP.md#m4-local-mvp), PRD §12, NFR-1). No hosted CI is planned, so the matrix is run on real machines and recorded. The M3 exception is an explicit owner scope decision, not evidence that other OSes work.

### Platforms
A run counts for an OS only when it executes natively on that OS: Linux (x86_64), macOS (Apple silicon) and Windows (x86_64, 10 or 11), on physical machines or full VMs. WSL, Docker on macOS or Windows, Wine and cross-compilation do not count for the OS they emulate; `GOOS=<os> go vet`/`go build` on another machine is a useful early signal, not evidence.

### What runs where
| Check | Linux | macOS | Windows | Notes |
|---|---|---|---|---|
| `make check` (fmt, vet, unit and architecture tests, `docs-check`, and the targets M2–M4 add: contract fixtures, SDK staleness, Web unit tests) | native | native | native | For M3, Linux is required and the other OSes are best effort; M4 still requires all three. On Windows, run it from Git Bash with GNU Make, or run the commands the Makefile target lists. |
| Storage, migration and start-up-lock tests on bundled PostgreSQL | native | native | native | Socket on Linux/macOS, loopback port + SCRAM on Windows (DEPLOYMENT). |
| Crash-injection suite, timeouts, restart recovery | native | native | native | Process and file-lock semantics differ per OS (WP-M3-03). |
| Process-tree cancel | native | native | native (best effort for M3) | Windows process-tree behavior (GAP-05) remains not validated until a native run is recorded. |
| Worktree tests (ENV-010) | native | native | native | Path and file-locking behavior. |
| Loopback authentication tests (Origin, Host, CSRF; ADR-0034) | native | native | native | Listener and browser-launch behavior. |
| Browser end-to-end tests (M4) | native | native | native | Chromium on each OS at least. |
| Automated accessibility checks (M4) | one OS | — | — | DOM-level; one OS is enough. |
| Recorded headless run with a real agent provider (M3 exit) | recorded | recorded | recorded | M3 requires a recorded Linux run; other OSes are best effort. WP-M3-05 still needs one recorded run per provider. |
| Install from the signed archive and recorded supervised run on a real Go repository (M4 exit, WRK-004, NFR-1) | recorded | recorded | recorded | No Docker and no admin rights. |

Checks that only read text (`docs-check`, gofmt, pure contract fixtures) give the same result everywhere; they still run in every OS's `make check`, which is cheap.

### Evidence
- **Same commit**: a milestone's OS evidence is for one commit on `main` (the exit candidate), or for the head of the integration PR when a Work Package's *Done when* names three OSes. A later change to code the run covered makes the record stale; re-run before closing.
- **Record**: one Markdown file per OS run under `docs/validation/<milestone>/`, named `<YYYY-MM-DD>-<os>-<scope>.md` (the directory is created with the first record). It states: date; commit SHA; OS name, version and architecture; machine kind (physical or VM); tool versions (`go version`, Node, PostgreSQL bundle, browser, agent CLI); the exact commands; pass/fail and counts per command; skipped or failing tests with the reason; who ran it. Logs too large for the record are attached to the GitHub comment and linked from the record; they must not contain secrets.
- **GitHub**: the person who ran it comments on the Work Package issue (or the exit issue) with the OS, the commit SHA and a link to the record, and ticks that OS in the issue's OS matrix checklist. The record itself is merged through a `docs/` PR.

### Blocking
- For remaining M3 Work Packages and the M3 exit, Linux evidence is required; missing macOS/Windows runs are recorded as **not validated**, not blockers. M4 Work Packages and the M4 exit (WP-M4-07) retain their native OS requirements; a missing or failing required OS blocks those criteria.
- Only the product owner may change an exit criterion, by changing the roadmap in a reviewed PR; a missing environment is never a reason to tick a box.

### Scope decision (product owner, 2026-10-28)
Historical decision, extended for M3 exit by the 2026-11-12 amendment below.
While the Platform is built on Linux, **Windows and macOS runs are deferred**: only best-effort compatibility is kept (code compiles and vets for both; no OS-specific claim is made). Work Packages whose *Done when* names three OSes may be integrated with Linux evidence only, and every Windows or macOS item is then recorded as **not validated** in the WP's validation record and its integration PR; nothing is described as working on those OSes. The deferred items stay owed: they are picked up before the M3 exit and are a blocker for the M3 exit issue unless the product owner changes the roadmap in a reviewed PR. First used by WP-M3-03 ([record](../validation/m3/2026-10-28-linux-wp-m3-03.md)): the Windows process-tree cancel is not validated.

### Coordination
- Before the first M3 integration PR, the product owner names in the M3 exit issue who provides each OS (person and machine or VM); likewise for M4 in WP-M4-07. Default owner for all three OSes is the product owner until they delegate an OS in a comment on the exit issue (decided 2026-10-25). The machine or VM used for each OS is listed in the issue before its first run.
- The contributor who opens an integration PR runs `make check` on Linux for remaining M3 work; macOS/Windows runs are best effort and their gaps stay recorded. Where M4 requires three OSes, request the other native runs and keep the PR open until the required records exist.
- If hosted CI is introduced later (by an ADR), its runs may replace the native `make check` rows, not the recorded runs.
- **Hosted runners (product owner decision, 2026-10-26, in the WP-M1-02 session):** GitHub-hosted runners (Linux, macOS Apple silicon, Windows) are accepted as the native environment for building and validating the Platform and the desktop app on macOS and Windows. They are full VMs, so they satisfy the native rule above; the record still names the runner image and states what a runner cannot show (for example Windows runners run as an administrator, so non-admin behavior is not shown). The first use is the scoped workflow `.github/workflows/spike-m1-02.yml` for WP-M1-02. Integrations that need a Windows or macOS machine with third-party software installed (for example an Unreal Engine MCP inside a VM) are a separate concern, deferred, and not a blocker for these Work Packages. Introducing a general CI pipeline still needs its own decision; this entry covers only what is named here.


### Scope decision (product owner, 2026-11-12)
The owner stated "Cross-platform is best effort only for now, our main target is Linux" and selected **All remaining M3** when asked whether this includes milestone exit. Linux checks remain required. Native macOS/Windows runs and recorded provider runs on those OSes are best effort and do not block remaining M3 work or M3 exit. Keep compatibility checks and record each unrun platform as **not validated**; do not mark it passed. This changes the earlier obligation to complete those runs before M3 exit, not the M4 criteria or the security baseline. See the amended [M3 roadmap](../architecture/ROADMAP.md#m3-platform-core).

## Embedded Worker boundary (ENV-001)

`make check` runs the coordinator assignment and cancellation suites over both the in-process link and a shared JSON serialization stand-in, plus `TestWorkerInterfaceParity` with the actual embedded Worker and a fake Runner. The latter checks success/failure, requested cancellation before result acknowledgement, duplicate-result deduplication, and restart recovery while the old Lease is still live (idempotent re-lease or non-idempotent interruption). Repeated reconciliation preserves settled state and Events. The Worker production package imports only the ports package within the module; the architecture test enforces that boundary.

These are fake-backed integration tests, not remote-worker or real-provider validation. The serialized link is not WebSocket/Protobuf and does not establish separate-process enrollment, reconnect or network behavior. Restart cases replace coordinator/Worker instances over a retained fake store; process-kill/PostgreSQL evidence belongs to the [EXE-002 record](../validation/m3/2026-11-12-linux-exe-002.md). See the [Linux ENV-001 record](../validation/m3/2026-11-12-linux-env-001.md) for commands and remaining limitations.

## Worker session shutdown

`make check` includes deterministic cancellation regressions for assignment, all Worker message handlers and sends. Session cancellation returns nil; unrelated store or transport failures remain observable. The Linux PostgreSQL reproduction, durability checks and limitations are recorded in the [Worker shutdown validation](../validation/m3/2026-11-12-linux-worker-shutdown.md).

The coordinator also captures its wake subscription before inspecting queued work. A deterministic fake-store regression submits after an empty queue snapshot and verifies assignment without another Offer or wake. Linux evidence and the distinction from the earlier independent-Project timeout are recorded in the [coordinator wake validation](../validation/m3/2026-11-13-linux-coordinator-wake.md).

## Concurrency limits (EXE-006)

The shared `LimitsSuite` runs over both in-process and JSON serialization links with the fake Execution store in `make check`, and against PostgreSQL in `make platform-test-pg`. It checks the default two slots per Project, independent Projects/Environments, global capacity, queued reasons and zero-as-uncapped configuration. For each scope, simultaneous Worker offers compete for one slot while the other two limits also apply. Success, failure and acknowledged cancellation free capacity once; a cancel request alone keeps the held slot. Duplicate results preserve Events and cannot admit an extra assignment. Startup reconciliation removes a lost running Lease; a repeated reconciliation does not release capacity again.

This is one coordinator serving multiple Worker sessions, not distributed scheduling across Platform processes. All leased/running Steps currently count, including non-agent Steps; agent-only accounting and per-Project overrides are not implemented. A zero limit means uncapped, not disabled; zero Worker credit is tested separately. Queued state/reasons are verified through the store, not a client UI. See the [Linux EXE-006 record](../validation/m3/2026-11-12-linux-exe-006.md) for evidence and limitations.

### Local authentication (IAM-001)

`make check` includes the compact auth/discovery Go tests, auth schema/fixture parity and SDK integration against a test-only Go loopback server. On Linux the discovery tests check native private modes and cross-process ownership. Real browser validation is separate: from `services/platform`, run `go test -tags browser -count=1 -run TestFirefoxLaunchSessionAndLogout -v ./internal/adapter/localauth`. It launches isolated native Firefox profiles, discards diagnostics that could include launch URLs and tests localhost/IPv4 loopback cookie, CSRF and WebSocket revocation. Evidence and unvalidated composition/platform items: [2026-11-12 IAM-001](../validation/m3/2026-11-12-linux-iam-001.md).

OBS-001 operational logging adapter evidence and remaining composition gates: [Linux record](../validation/m3/2026-11-12-linux-obs-001.md). `make check` includes secret-fixture capture and command/auth composition tests; it does not validate a running composed Platform server.

## Platform process lifecycle

The #357 [accepted ADR-0068](../architecture/adr/0068-platform-process-readiness-and-bounded-shutdown.md) has [isolated Lane compatibility evidence and a production failure matrix](../validation/m3/2026-11-13-platform-lifecycle-proposal.md). This is not production bootstrap, PostgreSQL process restart or NATS/outbox validation. Lane v2.1.0 is pinned and used in operational diagnostic tests; the [bounded safe handoff](../validation/m3/2026-11-13-linux-lifecycle-diagnostics.md) is implemented, with product consumer/composition still pending. Implementation must retain existing embedded Worker cancel-and-join/no-result and durable reconciliation tests.

`internal/platform` now supplies the external storage pool and schema owners: real Lane tests with fake SQL connections in `make check`, plus `TestPostgresStorageSchemaLifecycle` in `make platform-test-pg`. The latter creates a fresh database, checks all migrations before dependent readiness, retains data through failed later-stage startup and owner recreation, and verifies pool closure with the operator-owned server still running. [Linux evidence](../validation/m3/2026-11-13-linux-storage-lifecycle.md) records the driver-panic/cleanup limits; product bootstrap and remaining lifecycle stages are still pending.

Recovery readiness is covered by `go test -race -count=1 ./internal/platform` from `services/platform`. The PostgreSQL-tagged `TestPostgresStorageSchemaLifecycle` now also runs recovery before dependent admission: a persisted ordinary running Step becomes interrupted, an idempotent running Step returns to queued, and a later fresh owner does not repeat state or Events. See [Linux recovery evidence](../validation/m3/2026-11-13-linux-recovery-lifecycle.md); owner/pool recreation is not process/server restart.

`Lifecycle` watchdog tests run with `go test -race -count=1 ./internal/platform`: stuck Start/readiness/Stop, healthy lifetime past startup expiry, cancellation ordering, failure classification, single-use ownership and pool retention through rollback. Driver-panic subprocesses use the watchdog with an uncancelled parent. [Linux evidence](../validation/m3/2026-11-13-linux-startup-watchdog.md) records the initial cancellation-ordering regression and its correction. Process exit, child reaping and the remaining product stages are not validated by these helpers.
