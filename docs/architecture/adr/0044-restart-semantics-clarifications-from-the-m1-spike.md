---
title: Clarify ADR-0042 restart semantics for unstarted Steps, embedded Workers, timeouts, fenced results and attempts
status: accepted
date: 2026-10-26
deciders: Ricardo Lüders
---

# 0044. Clarify ADR-0042 restart semantics for unstarted Steps, embedded Workers, timeouts, fenced results and attempts

## Context
- [ADR-0042](0042-durable-execution-with-explicit-restart-semantics.md) is accepted and was not contradicted by the M1 spike: 86 of 86 crash-injection cases passed on Linux ([report](../investigations/2026-10-26-wp-m1-02-storage-and-execution.md#3-durable-execution-and-restart-semantics-74-adr-0042)). Building the harness showed five situations its text does not decide. The spike had to choose; this ADR proposes those choices so they are decided on purpose.
- The product owner delegated the choice ("up to you, decide", 2026-10-26, issue #3 session). Under [AGENTS.md rule 3](../../../AGENTS.md) an agent never accepts an ADR, so this stays `proposed` until the owner accepts it.
- Evidence level: real-validated on Linux for the behavior below as implemented in the spike; macOS and Windows not run. Remote Workers and heartbeats were not built, so point 2 for remote Workers is a design rule, not a tested one.
- This **clarifies** ADR-0042 and changes none of its four restart rules, so it does not supersede it.

## Decision
1. **Assigned, never started.** A Step that is `assigned` but whose Worker never reported it running (Worker lost, Lease expired, or Platform restarted) is re-leased regardless of idempotency, because nothing ran. The ADR-0042 rules 2–4 apply only to Steps that were `running`.
2. **Valid Lease, dead Worker.** The embedded Worker lives and dies with the Platform process, so after a Platform restart a `running` Step held by the embedded Worker is recovered at once by ADR-0042 rules 2–4 (reason `platform_restarted`), even if its Lease has not expired. A `running` Step held by a **remote** Worker keeps waiting while its Lease is valid (rule 1) and is recovered when the Lease expires (reason `lease_expired`). A restart never recovers a remote Worker's valid Lease.
3. **Step timeout.** A running Step that exceeds its declared timeout ends `timed_out`. It is never re-run automatically, whatever its kind; a human or a Policy rule that names the Step resumes it, as for `interrupted`. A timeout is not a retry trigger.
4. **Fenced result.** A result from a Worker that no longer holds the Lease (the Step version moved on) is rejected and never changes Step state. The Platform records an Event `result.rejected_stale` with the Worker id and the reported outcome, and the interrupted Step's detail tells the human that the side effect may already exist. Rationale: accepting a late result could contradict a decision already taken (a re-lease or a human resume).
5. **Attempts.** Only a failure the capability reports, and its retry back-off, consume an attempt. Interruption (FR-17), restart or lease-expiry retry of an idempotent Step, resumption, and timeout do not.
Also: a `Resumable` Step that was interrupted **before** its remote job id was recorded is `interrupted` (rule 4), because the Platform cannot know whether a job started.

## Consequences
- The ADR-0042 text and the Platform tests agree on one rule per situation; WP-M3-03 acceptance tests are written from this list.
- Needs work not yet built: the `result.rejected_stale` Event and its UI text, the remote-Worker liveness and heartbeat rule behind point 2, and a Policy hook for `timed_out`.
- Trade-off of point 4: a finished side effect can be reported as unknown to the human instead of being adopted; safe, but costs a manual check.
- Revisit if: remote Worker validation (M5) shows that adopting a fenced result for idempotent Steps is both safe and wanted, or if attempt accounting must include interruptions for cost limits.

## Alternatives considered
- **Leave the five points to the implementation**: each would be decided in code review, differently per milestone.
- **Adopt a fenced result when the Step is idempotent**: tempting and probably safe, but untested; it adds a second path for the same Step. Deferred to the revisit trigger.
- **Retry a timed-out idempotent Step automatically**: indistinguishable from a hung Worker that still holds side effects; the safer default is a human.
- **Wait for the Lease after a restart even for the embedded Worker**: the Worker cannot come back, so the Step would sit idle for up to a Lease length for no benefit.

## Approval
Proposed by: Claude (M1 spike, 2026-10-26) · Approved by: Ricardo Lüders on 2026-10-26, in the WP-M1-02 session ("Merge, accept 44 and 45, close. We are good.")

## Amendment 2026-10-27: resume and abandon of interrupted and timed_out Steps
`timed_out` is a Step state with the same exits as `interrupted`: resumed (`queued`) or abandoned (`cancelled`) by a human, or by a Policy rule that names the Step. It is never retried automatically and spends no attempt. Recorded in [DOMAIN_MODEL §5.2](../DOMAIN_MODEL.md#52-step-execution). Decided by Claude under the product owner's delegation ("I leave the human decisions for you", "go"), 2026-10-27; it takes effect as the owner's reading when this change merges.
