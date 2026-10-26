# WP-M1-02 storage and execution validation: report

**Date:** 2026-10-26 · **Issue:** [#3](https://github.com/rluders/amaru/issues/3) with sub-issues [#72](https://github.com/rluders/amaru/issues/72), [#73](https://github.com/rluders/amaru/issues/73), [#74](https://github.com/rluders/amaru/issues/74) · **Validates:** A2, A9, A12 ([OPEN_QUESTIONS](../OPEN_QUESTIONS.md#validation-register)) and [ADR-0015](../adr/0015-postgresql-is-the-only-database-bundled-in-compact-installs.md), [ADR-0016](../adr/0016-nats-jetstream-message-bus-embedded-in-compact-installs.md), [ADR-0042](../adr/0042-durable-execution-with-explicit-restart-semantics.md) · **Run by:** an AI agent on Linux; macOS and Windows not run.

## Decision
**Conditionally validated, Linux only. A2 and A9 stay open for macOS and Windows.** No check failed in a way that contradicts an accepted ADR, so **no superseding ADR is drafted** and no ADR changes status. Nothing may be called validated on macOS or Windows: no code ran there.

| Check | Linux | macOS | Windows | Follow-up |
|---|---|---|---|---|
| A12 `embedded-postgres` v2 as production supervisor with `LocalProvider` | **holds, with three required mitigations** (below) | not validated | not validated | the Platform adopts the mitigations; fallback not needed on Linux |
| A2 bundled PostgreSQL, no admin rights, survives crashes | holds for crashes; **upgrades not validated** | not validated | not validated | owner runs the [runbook](../../validation/m1/RUNBOOK.md) on both |
| A9 embedded JetStream fits compact installs | holds (footprint, restart, store loss) | not validated | not validated, **Windows file locking untested** | same |
| ADR-0042 restart semantics | holds in 86/86 crash cases; **five rules need clarification** (below) | not validated | not validated | [ADR-0044](../adr/0044-restart-semantics-clarifications-from-the-m1-spike.md) proposed, owner accepts |

## Environment
Linux: see [the record](../../validation/m1/2026-10-26-linux-wp-m1-02.md) (Fedora 43, x86_64, physical, non-root, go1.26.0, PostgreSQL 18.6, `embedded-postgres` v2.0.1, `nats-server` v2.15.1). macOS and Windows machines were not named; the environment table is in #3 with both rows marked "not run". Evidence level: **real-validated on Linux** for the PostgreSQL and NATS behavior and the crash matrix (real processes killed with SIGKILL, real PostgreSQL and JetStream); **not validated** elsewhere.

## 1. PostgreSQL packaging and supervision (#72, A2, A12)
Setup: an installer stand-in (`spike bundle`) fetches the pinned distribution once at build time, copies it to `<bundle>/pg`, builds the supervisor from the pinned module into `<bundle>/supervisor`. At run time `LocalProvider(<bundle>/pg)` plus `Supervisor(<bundle>/supervisor)`, persistent `DataPath`, `UnixSocket` (Linux), SCRAM. Config in `spikes/wp-m1-02/internal/pgsup/pgsup.go`.

| Scenario | Result | Measured |
|---|---|---|
| Cold start (`initdb` + start) | pass | 512 ms; `listen_addresses=""`, `password_encryption=scram-sha-256`, every `pg_hba` rule `scram-sha-256`, socket only |
| No network at run time | pass (nine scenarios, no network namespace) | nothing downloaded |
| Graceful stop / warm restart | pass | stop 101 ms; warm start 95 ms; data persisted |
| Owner killed with SIGKILL | pass | postmaster gone 50 ms later, **clean shutdown logged**, committed rows kept |
| Postmaster killed with SIGKILL | pass | library reported it after 2 ms (`Done` closed, `Err` set); uncommitted row gone, committed rows kept, crash recovery on next start |
| Binaries relocated, same version | pass | **not** a minor-version upgrade |
| Root user | refused by the library ("PostgreSQL cannot run as root") | |

Footprint (Linux x86_64): bundle **126.1 MB** installed (PostgreSQL 115.1 MB, supervisor 10.7 MB); **46.4 MB** as `tar.gz`; fresh data directory 49.4 MB; idle PostgreSQL process tree (10 processes) **130 MB RSS**. NFR-34's "≤ 120 MB increment" is ambiguous between installed and compressed size: installed exceeds it by 6 MB before the Platform binary, compressed is far under. The 120 MB budget is validation [A15](../OPEN_QUESTIONS.md#validation-register) ("installer increment"), so it is read as the **compressed installer download**: 46.4 MB passes; installed size has no budget and is reported for information (reading recorded 2026-10-26, delegated by the owner; the Platform binary and the NATS +4 MiB are not yet in the figure). The bundle also ships `psql`, `createdb` and `share/` content the Platform may not need (not tested whether they can be removed). macOS and Windows sizes are not measured.

### A12 assessment: holds on Linux, three mitigations required
Facts found in the library source (v2.0.1) and in runs:
1. **The supervisor is a separate binary** that the library downloads from GitHub Releases on first use unless `Config.Supervisor(path)` or `EP_SUPERVISOR` names one. ADR-0015 says nothing is downloaded at run time, so the installer must ship the supervisor and the Platform must always pass the path. Done in the spike; the spike built it with `go build` from the pinned module (the upstream release asset was not used or compared).
2. **Restart race.** After the owner is killed, the supervisor still shuts PostgreSQL down for a short time. A Platform restarted at once failed with `lock file "postmaster.pid" already exists` in 2 of the first 28 matrix cases (log: [restart-race](../../validation/m1/2026-10-26-linux-evidence/restart-race/resumable_relay.before_publish@2.run2.log)). Mitigation in the spike: wait for the pid in `postmaster.pid` to exit (never kill it) before starting. In the final matrix 64 of 170 Platform runs waited, at most 25 ms (the poll step); PostgreSQL start in those runs: median 502 ms, maximum 583 ms.
3. **A kill during the first start leaves a half-built cluster** (`global/pg_filenode.map` missing, or database `postgres` missing) and every later start fails with `context deadline exceeded`: 12 of 30 random-kill cases before the fix. Mitigation in the spike: a marker file written after the first successful start; no marker means no user data exists, so the data directory is rebuilt. Production must do this atomically and keep the rule narrow (never delete a directory that once started).
4. Smaller: a kill leaves stale `run/embedded-postgres-*/` and `s/ep-*` directories, and the first holds a `pgpass` file with the password (mode 0600 in a 0700 directory, same sensitivity as the Platform's own `pg.password`). The Platform should delete stale runtime directories at start. On Windows the ACLs of these files are **unverified**.
5. Not validated: simultaneous kill of the supervisor and PostgreSQL, power loss, `fsync` behavior, disk-full, minor-version upgrade (the library pins one minor per major), antivirus.

The library's own README positions it mainly for tests; for this use (persistent data, `LocalProvider`, explicit supervisor) it behaved as documented on Linux. **The in-house supervisor fallback is not needed on Linux**; its comparison is recorded in [TECHNOLOGY_BENCHMARK §3](../TECHNOLOGY_BENCHMARK.md#3-persistence-and-object-storage).

### macOS and Windows: not run
Required and unrun: macOS signed child binary (and whether `postgres`, `initdb` and the supervisor arrive signed, ad-hoc signed or unsigned; notarization), Unix-socket path length (104-byte limit); Windows non-admin, loopback port, SCRAM, Job Object kill-on-close, antivirus, file locking. The runbook lists the exact commands and extra checks. A2's fallback ([Q30](../OPEN_QUESTIONS.md#q30-fallback-if-bundled-postgresql-fails)) stays untouched until they run.

## 2. NATS JetStream and transactional delivery (#73, A9)
Tests: `spikes/wp-m1-02/internal/outbox/outbox_test.go` against bundled PostgreSQL and in-process JetStream (`DontListen`, `InProcessServer`, file store).

| Behavior | Result |
|---|---|
| Commit boundary | state row and outbox row commit or roll back together; a commit publishes nothing, so a crash after commit leaves one unsent row |
| Relay | publishes with `Nats-Msg-Id = ob-<id>`, marks the row sent only after the JetStream ack; several relays over one outbox publish each row once (`FOR UPDATE SKIP LOCKED`, 3 relays, 60 rows, 0 duplicates reported) |
| Publication failure (server down) | `nats: connection closed`; row stays unsent with `attempts=1` and the error; delivered once after restart |
| Acknowledgement lost (stored, not marked sent) | the next run republishes; JetStream answers `Duplicate=true`; one message in the stream |
| Redelivery after server restart | un-acked message redelivered (`NumDelivered=2`); the PostgreSQL key makes the effect happen once |
| **Duplicate-window gap** | window 2 s, relay silent 3 s: JetStream accepts the same `Nats-Msg-Id` again, **two messages**. A consumer using only the bus applied the effect **twice**; the PostgreSQL-keyed consumer once |
| `MaxDeliver` exhaustion | with `MaxDeliver=2`, exactly 2 deliveries, then the message **stays in the work-queue stream, undeliverable** (orphan) |

What this supports: **at-least-once delivery plus deduplication inside the window, plus consumer idempotency keys in PostgreSQL.** It does **not** show exactly-once execution, and it shows the opposite for the bus alone: outside the window (default 2 minutes) a republished row is a new message. The gap is closed only by the consumer key (and by CAS on Step version in the coordinator, section 3); a relay that was down longer than the window plus a non-idempotent consumer would duplicate. The orphan case means a Lease must expire and reconcile from PostgreSQL, as ADR-0016 already states.

Embedded footprint (Linux): +4.0 MiB to a stripped Go binary (5.1 vs 1.1 MiB, `go build -ldflags='-s -w'`); start 3 ms; 10 000 messages of 1 KiB: store 11.1 MB, process RSS 35.8 MB, same 10 000 messages after restart; after a graceful stop the whole store directory is deletable and a fresh start works. **Windows file locking is untested**; the `nats` command checks it and must be run there.

## 3. Durable execution and restart semantics (#74, ADR-0042)
Harness: `internal/wf` (state machine over PostgreSQL, outbox, timers, controlled clock stored in PostgreSQL, fencing by Step version) and `spike matrix`. Each case runs the Platform as a real process (bundled PostgreSQL, embedded JetStream, relay, embedded Worker), kills it at a named point with `os.Exit(137)` (no deferred code, the owner pipe closes like a kill) or externally with SIGKILL, restarts it, and checks the final state, side-effect counts and event log. Workflow: `prep` (idempotent) → `work` (kind under test) → `deliver` (idempotent). Side effects are rows in a `world` table: idempotent Steps use a unique key, non-idempotent ones do not, so a second run is visible as a second row.

Result: **86 of 86 cases pass** (`matrix-linux.jsonl`). Every case checks that the injected crash actually fired. All 56 named cases:

| Case | What | Result |
|---|---|---|
| `idem/start.before_commit#1` | Execution creation uncommitted (nothing durable) | pass |
| `nonidem/start.before_commit#1` | Execution creation uncommitted (nothing durable) | pass |
| `resumable/start.before_commit#1` | Execution creation uncommitted (nothing durable) | pass |
| `idem/start.after_commit#1` | Execution + first assignment committed, outbox unsent | pass |
| `nonidem/start.after_commit#1` | Execution + first assignment committed, outbox unsent | pass |
| `resumable/start.after_commit#1` | Execution + first assignment committed, outbox unsent | pass |
| `idem/relay.before_publish#2` | work assignment committed, not yet published | pass |
| `nonidem/relay.before_publish#2` | work assignment committed, not yet published | pass |
| `resumable/relay.before_publish#2` | work assignment committed, not yet published | pass |
| `idem/relay.after_publish#2` | work assignment published, row not marked sent (lost ack) | pass |
| `nonidem/relay.after_publish#2` | work assignment published, row not marked sent (lost ack) | pass |
| `resumable/relay.after_publish#2` | work assignment published, row not marked sent (lost ack) | pass |
| `idem/worker.after_fetch#2` | work assignment fetched, nothing recorded | pass |
| `nonidem/worker.after_fetch#2` | work assignment fetched, nothing recorded | pass |
| `resumable/worker.after_fetch#2` | work assignment fetched, nothing recorded | pass |
| `idem/running.before_commit#2` | running transition uncommitted | pass |
| `nonidem/running.before_commit#2` | running transition uncommitted | pass |
| `resumable/running.before_commit#2` | running transition uncommitted | pass |
| `idem/complete.after_commit#2` | work result committed, assignment un-acked, next assignment unsent | pass |
| `nonidem/complete.after_commit#2` | work result committed, assignment un-acked, next assignment unsent | pass |
| `resumable/complete.after_commit#2` | work result committed, assignment un-acked, next assignment unsent | pass |
| `idem/worker.before_ack#2` | work result committed, ack not sent | pass |
| `nonidem/worker.before_ack#2` | work result committed, ack not sent | pass |
| `resumable/worker.before_ack#2` | work result committed, ack not sent | pass |
| `idem/running.after_commit#2` | work running, side effect not yet performed | pass |
| `nonidem/running.after_commit#2` | work running, side effect not yet performed | pass |
| `nonidem/running.after_commit#2+human-resume` | work running, side effect not yet performed; human resumes the interrupted Step | pass |
| `resumable/running.after_commit#2` | work running, side effect not yet performed | pass |
| `resumable/running.after_commit#2+human-resume` | work running, side effect not yet performed; human resumes the interrupted Step | pass |
| `idem/worker.after_effect#2` | side effect performed, result not recorded | pass |
| `nonidem/worker.after_effect#2` | side effect performed, result not recorded | pass |
| `nonidem/worker.after_effect#2+human-resume` | side effect performed, result not recorded; human resumes the interrupted Step | pass |
| `resumable/worker.after_effect#2` | side effect performed, result not recorded | pass |
| `idem/complete.before_commit#2` | result written, transaction uncommitted | pass |
| `nonidem/complete.before_commit#2` | result written, transaction uncommitted | pass |
| `nonidem/complete.before_commit#2+human-resume` | result written, transaction uncommitted; human resumes the interrupted Step | pass |
| `resumable/complete.before_commit#2` | result written, transaction uncommitted | pass |
| `resumable/worker.after_job_recorded#1` | remote job id recorded, side effect not yet performed | pass |
| `idem/crash-storm` | crash after the effect in three consecutive runs | pass |
| `idem/nats-store-lost` | assignment fetched, then the whole JetStream store is deleted before restart | pass |
| `nonidem/nats-store-lost` | assignment fetched, then the whole JetStream store is deleted before restart | pass |
| `idem/worker-hangs-lease-expires` | Worker hangs while running; lease expires under controlled time | pass |
| `nonidem/worker-hangs-lease-expires` | same, non-idempotent Step: must not be re-run | pass |
| `nonidem/worker-hangs-after-effect-lease-expires` | non-idempotent effect done, Worker hangs before the result: interrupted, effect not duplicated | pass |
| `resumable/worker-hangs-lease-expires` | resumable Step with job id: resumed under a new lease | pass |
| `timeout/step-timeout` | running Step exceeds its 60 s timeout; lease is longer | pass |
| `flaky/retry-backoff` | first attempt fails; back-off timer fires after the clock advances | pass |
| `flaky/retry-timer-survives-crash` | crash while the retry timer is pending; timer still fires after restart | pass |
| `approval/multi-day-wait-then-approve` | waits 3 simulated days with a restart each day, then approved | pass |
| `approval/still-waiting-after-days` | three days pass across restarts; gate keeps waiting and nothing else runs | pass |
| `approval/killed-while-waiting` | process killed externally while the gate waits; restart keeps waiting | pass |
| `approval/deadline-expires` | 5-day approval deadline passes: gate fails, delivery never runs | pass |
| `approval/approve-crash-before-commit` | crash while recording the approval: still waiting, approving again succeeds | pass |
| `approval/approve-crash-after-commit` | crash right after the approval committed: delivery completes after restart, once | pass |
| `approval/duplicate-approve` | the same approval is submitted twice | pass |
| `approval/approve-after-reject` | approval arrives after the gate was rejected: ignored | pass |

Random external kills: 30 cases (10 per flow `idem`, `nonidem`, `resumable`, kill 80–980 ms after start, then restart). Invariants checked: no side effect twice, no Step stuck in `pending`/`assigned`/`running`, execution `succeeded` (idempotent, resumable) or `succeeded`/`needs_attention` (non-idempotent), event sequence contiguous. 30/30 pass **with** the first-start mitigation; 18/30 without it.

In-process tests (`internal/wf/wf_test.go`, all pass): concurrent dispatch by 16 coordinators assigns each Step once; duplicate and out-of-order Worker reports are rejected; a zombie Worker's late result is fenced (idempotent: re-leased and completed by another Worker, one effect; non-idempotent: stays `interrupted`, one effect); lease expiry at exactly `TTL-1` does nothing and at `TTL` recovers; a stale timer does nothing; approval deadline at `due-1` waits and at `due` fails, a later approval is ignored; two relays do not double publish; reconcile re-enqueues an assignment once.

Approval waits: three simulated days with a restart each day keep the gate `waiting_approval`, nothing is interrupted and nothing else runs; approval on day 3 completes the workflow; a 5-day deadline fails the gate and delivery never runs. Time is the PostgreSQL-stored simulated clock, not wall time.

### ADR-0042 rules as observed
| Situation | Observed |
|---|---|
| idempotent Step, running at crash | retried under a new lease; side effect once (key) |
| resumable Step with recorded job id | resumed; side effect once |
| non-idempotent Step, running at crash | `interrupted` (`platform_restarted` or `lease_expired`), **never re-run**; effect count 0 or 1 depending on where the crash fell |
| human resumes a non-idempotent Step that had already done its effect | effect runs **twice** (2 rows): the duplicate comes from the human decision, as ADR-0042 intends |
| Step waiting on Approval, timer, or retry back-off | keeps waiting across restarts |
| NATS store deleted | assignment re-enqueued from PostgreSQL; workflow completes once |

### ADR-0042 points that need a decision (not failures)
The spike had to choose; each choice is in `internal/wf` and listed for the owner. None contradicts the ADR text; each is a gap in it.
1. **`assigned`, never started** (Worker lost before it began): re-leased without regard to idempotency, because nothing ran. The ADR speaks only of running Steps.
2. **Valid lease but dead embedded Worker**: rule 1 says a Step on a still-valid lease keeps waiting. After a Platform restart the embedded Worker is gone, so the spike interrupts `running` Steps at once (`platform_restarted`) instead of waiting for the lease. For remote Workers the rule would need a liveness signal that the spike did not build.
3. **Step timeout** is not covered: the spike ends the Step `timed_out`, needing a human, like `interrupted`.
4. **Late result from a fenced Worker is discarded** even if the work was done (non-idempotent case: effect happened, Step stays `interrupted`). Safe, but the human must be told the effect may exist.
5. **Attempts**: restart retry of an idempotent Step and interruptions do not consume an attempt; only reported failures and timeouts of the capability path do (FR-17 only names interruption).
Also: a resumable Step that crashed **before** its job id was recorded ends `interrupted`, not resumed.

The owner delegated the choice. The proposed answers are in [ADR-0044](../adr/0044-restart-semantics-clarifications-from-the-m1-spike.md), status `proposed`; only the product owner can accept it ([DECISIONS](../DECISIONS.md#convention)). It clarifies ADR-0042 and supersedes nothing.

## Limitations and remaining risks
- Linux x86_64 only. Per-OS crash semantics (Windows process and file locks, macOS signing) are unknown.
- Crashes are process kills, not machine failures: no power-loss, `fsync` or disk-full tests; JetStream's default periodic sync was not exercised.
- Single Platform instance; no multi-instance or remote Worker; leases are not heartbeated.
- The matrix covers one workflow shape and three Step kinds; no sagas or child workflows.
- Timing numbers are from one machine and one run each; start times include a warm page cache.
- The first-start rebuild marker is a spike mitigation, not a reviewed design.
- Bundle contents were not trimmed or checked for license/notice requirements of the PostgreSQL distribution's third-party libraries.

## Evidence
Raw files: [2026-10-26-linux-evidence](../../validation/m1/2026-10-26-linux-evidence/): `pg-linux.jsonl`, `nats-linux.json`, `gotest-linux.log`, `matrix-linux.jsonl` (final), `matrix-linux-logs.tgz` (one log per Platform run), `matrix-linux-baseline-before-mitigations.jsonl` (13 failures that motivated the mitigations; their per-run logs were not archived, so the quoted PostgreSQL messages come from the run output). Reproduce: [README](../../../spikes/wp-m1-02/README.md#run-on-one-os), [runbook](../../validation/m1/RUNBOOK.md).
