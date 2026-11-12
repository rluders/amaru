# Execution completion and failure drain

The coordinator implements [ADR-0063](../architecture/adr/0063-terminal-execution-aggregation-and-failure-drain.md) for [#335](https://github.com/rluders/amaru/issues/335). The Platform store remains authoritative. Worker results commit the Step first, then settle its parent before acknowledgement. Submission refusals, Approval rejection, human abandonment and startup reconciliation use the same settlement path. A crash between those writes is repaired from stored Steps, without re-running a completed Step.

## Outcomes and human control

An explicit Execution cancellation request wins until the parent is terminal. Otherwise a failed/refused Step makes the eventual outcome failed; absent failure, a cancelled Step makes it cancelled; otherwise every Step must have succeeded or been skipped. Empty runs do not auto-complete. Pre-start completion preserves a missing start timestamp. Parent state, end timestamp and its single terminal Event commit atomically.

Failure prevents new leases and cancels pending, queued and awaiting-Approval Steps. Requested Approvals for those Steps are voided with `execution_failed`, without inventing a human decision. Worker-held Steps receive cancellation through the existing session port; a result that commits first is retained. The parent stays open until every Step is terminal.

An interrupted or timed-out Step remains a human wait during ordinary failure drain, and its parent remains waiting. `Coordinator.ResumeStep` refuses a failed or explicitly cancelling run without changing the Step; `AbandonStep` lets a human or named Policy rule end the wait. Healthy resume continues to queue the Step for normal Policy and lease admission. Explicit Execution cancellation can abandon those waits. Both commands are idempotent, and their `ProjectAccess` wrappers authenticate and verify Project ownership before mutation or replay. They are internal application operations; product HTTP routes remain outstanding in #18.

The parent failure is chosen by the lexicographically smallest failed/refused Step ID. Its code and message are redacted again; refusal, missing errors and Approval rejection use fixed safe fallback messages. A parent terminal Event contains the outcome kind without failure text. Existing contract names `execution.completed`, `execution.failed` and `execution.cancelled` are reused as stored kinds; this does not implement projection of the internal Event row into the public contract envelope.

## Concurrency and recovery

Lifecycle writes validate the parent version and the Step snapshot used for their decision. `ExecutionChange.CheckSteps` checks read-only versions atomically without increasing those Step versions. The fake store validates under its lock; PostgreSQL locks the parent row for every commit, including Step-only writes without Events, before checking and writing rows. This prevents a stale snapshot from admitting work after a sibling failure or completing a parent after a competing human transition. Optimistic conflicts cause a fresh read and bounded retry.

Reconciliation retains ADR-0042/0044 recovery rules for held work, then resumes drain and aggregation. Repeated recovery and duplicate results preserve a settled parent's state and terminal Event. Failure drain uses the existing best-effort cancellation channel: a missing session, full cancellation channel or external remote job is not proof of process termination. No retry engine, remote-job cancellation guarantee or distributed capacity scheduler is added.

## Evidence and remaining scope

The shared `TerminalSuite` covers in-process and JSON-serialized fake Worker links, and is also run against isolated PostgreSQL. The existing durable-write and child-process kill matrices include the parent aggregation commit. [Linux validation](../validation/m3/2026-11-12-linux-execution-aggregation.md) records commands and results.

This fixes ordinary parent completion; it does not complete #191, #18 or M3. Public HTTP composition, real Providers, remote Worker transport and native macOS/Windows behavior require their own evidence. Linux is the required M3 acceptance platform; other OSes remain best effort.
