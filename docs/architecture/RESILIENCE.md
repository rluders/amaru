# Failure and Recovery Model

Status: **accepted** (2026-10-25), not implemented. Extends [ARCHITECTURE §11](ARCHITECTURE.md#11-workflow-durability-and-recovery) and [§17](ARCHITECTURE.md#17-failure-modes-and-recovery-expectations). The engine that implements this model is Amaru's own state machine on PostgreSQL with a transactional outbox to NATS ([ADR-0042](adr/0042-durable-execution-with-explicit-restart-semantics.md), [ADR-0016](adr/0016-nats-jetstream-message-bus-embedded-in-compact-installs.md)). This document states the requirements it must meet.

## 1. Evidence from Weave (verified at `weave-obsidian` `c8a2f6e`)

- **Restart handling.**
  - `Manager.Restore` fails unfinished Executions with `runtime_restarted`.
  - `ResumeInterrupted` reattaches remote jobs (`Resumable`) and pending approvals, but only when the Workflow's steps are unchanged.
  - Event sequence numbers resume from the persisted maximum.
- **Delivery-loop ledger.**
  - The Autoloop records each side effect before taking the next.
  - Restart recoveries are capped at 3 per attempt and do not consume attempts (ADR-029; F-25).
- **Idempotency.** Commands carry `IdempotencyKey`. The Autoloop and hooks use deterministic keys.
- **Cancellation.** Process-group termination with a grace period. A Windows variant exists but is unvalidated (GAP-05).
- **Retries.** None per Step.
  - Autoloop: attempt, repair and review budgets.
  - Outbound webhooks: up to 5 retries.
  - Remote APIs: `Retry-After` is honored.
- **Provider quota.** A quota error pauses the provider for a fixed 30 minutes without spending attempts. Weave finding F-27 (stated reset time ignored) is an Amaru acceptance test: the pause honors the provider's stated reset time ([ADR-0032](adr/0032-migration-baseline-is-orkestra-dogfood-c8a2f6e.md)).
- **Gaps.**
  - No per-Step wall-clock timeout.
  - No leases.
  - Single-instance check is a liveness probe, not a lock.
  - Non-terminal events are not fsynced (GAP-07).

## 2. Requirements

| Concern | Requirement | Feature |
|---|---|---|
| Persistent state | Every transition is durable before it is acknowledged or acted on. | EXE-002 |
| Recovery after Platform failure | Per [ADR-0042](adr/0042-durable-execution-with-explicit-restart-semantics.md): waiting Steps keep waiting; Steps with a `Resumable` remote job resume; Steps whose Capability is declared idempotent retry as a new Lease; every other running Step ends `interrupted` (`platform_restarted`) and is never re-run automatically. | EXE-002 |
| Recovery after machine failure | Same rule when a Lease expires (reason `worker_lost`). | ENV-004 |
| Idempotency | Every command and every Step assignment carries an ID. Consequential Capabilities are idempotent or guarded by a check-before-act on external state (e.g. "does the PR exist?"). | EXE-005 |
| Retries and backoff | Only for Steps whose Capability is declared idempotent. Exponential backoff with jitter. Attempts recorded as Events. | WFL-005 |
| Cancellation | User cancel and Policy cancel propagate to Workers and terminate process trees. Cancel is idempotent. | EXE-003 |
| Timeouts | Per-Step wall-clock timeout. Defaults: agent Steps 60 min, tool and check Steps 30 min, Approval waits none; a Workflow or Capability may override within an Organization maximum (default 8 h). On expiry the Step is cancelled and failed with `timeout`. | WFL-004 |
| Scheduling and concurrency | Slot limits at three levels: global (sum of connected Worker capacity), per Project (default 2 concurrent agent Steps) and per Environment (capacity declared by its Workers; editor Environments 1). The Platform checks slots before publishing an assignment, so fairness does not depend on the JetStream consumer. Queued work shows why it waits. | EXE-006, EXE-007 |
| Human approval and intervention | Waits are durable for days. Approvals can be cancelled, voided by subject change, or expired by Policy. | REV-001 |
| Worker registration, health, leases | Heartbeat every 10 s; Lease duration 30 s, renewed by heartbeat (initial values, validated in the M5 spike, NFR-10). Expiry follows [ADR-0042](adr/0042-durable-execution-with-explicit-restart-semantics.md): retry as a new Lease if idempotent, else `interrupted`. | ENV-004 |
| Event delivery | Events are ordered per Project and per Execution with a cursor. Clients resume from the cursor. The replay window is 7 days in compact installs and 30 days on servers ([DATA_ARCHITECTURE §5](DATA_ARCHITECTURE.md)); beyond it the client gets an explicit resync. | EXE-004 |
| Lost or duplicated messages | Completion is accepted at most once per attempt ID. Late reports from expired leases are recorded and ignored. | ENV-004 |
| Long-running agents | Bounded by timeouts. Remote jobs are detachable. Persistent agent sessions are P3 (EXE-009). | CAP-004 |
| Provider quota and outages | Pause the provider until its stated reset time. Do not spend attempts. Surface the cause. | EXE-008 |
| Execution history and evidence | Written before Step success. Survives restarts. Retained per [DATA_ARCHITECTURE §5](DATA_ARCHITECTURE.md). | ART-002 |

## 3. Failure catalogue

| Failure | Detection | Expected behavior | Test |
|---|---|---|---|
| Platform crash mid-Step | Startup scan of non-terminal Steps | Keep waiting, resume, retry if idempotent, or `interrupted` with reason | Kill in every Step state (crash-injection suite) |
| Worker process crash | Lease expiry | Retry as a new Lease if idempotent, else `interrupted` (`worker_lost`) | Kill Worker during run |
| Network partition between Worker and Platform | Missed heartbeats | Worker continues until lease end; reconnects and reports; duplicate completion ignored | Partition simulation |
| Provider hangs | Step timeout | Process tree killed; Step failed `timeout` | Hanging fake provider on each OS |
| Agent produces no change | Diff empty after implementation | Not counted as an attempt; reported (F-39) | Fake provider returning no change |
| Review without evidence | Review prompt validation | Review refused until validation Evidence exists (F-38) | Unit test |
| Approval never given | Approval age | Remains `waiting`, visible in the inbox; the engine expires it when a deadline is set (configurable expiry Policy: FEAT-REV-008) | Clock-controlled test |
| Approval subject changed | New commit on the branch | Approval voided; re-request | Integration test |
| Forge webhook missed | Periodic reconcile | State corrected within the reconcile interval, 30 s by default (Weave ADR-027) | Fake forge |
| CI unavailable (billing, outage) | CI classification | Item blocked with the cause shown (F-36); no repair attempts spent | Fake CI |
| Storage write failure | Write error | Transition not acknowledged; operation retried or failed visibly | Fault-injecting storage adapter |
| Corrupt or future-version record | Loader | Refuse that Project with a clear error; never guess | Fixture tests |
| Client disconnect | Socket close | No effect on Executions; resume from cursor | Reconnect tests |
| Two Platforms on one data directory | Start-up lock | Second instance refuses to start (file lock, not only a liveness probe) | Concurrency test |

## 4. Verification approach

- A **crash-injection suite** runs the representative Workflow ([ARCHITECTURE §15](ARCHITECTURE.md#15-representative-workflow-execution)) and kills the Platform at every Step state transition. It asserts there is no lost or duplicated consequential action. It runs in `make check` with fakes.
- A **partition suite** for Workers runs from M5.
- **Recorded manual runs** on each OS for cancellation and restart. Weave never validated Windows (GAP-05).
