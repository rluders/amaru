# Weave findings as Amaru acceptance-test specifications

**Status: Proposed. These are specifications, not tests.** No Amaru Platform, Worker or test code exists yet; nothing here is implemented or verified. Each spec becomes a test when its owning Work Package builds the component, and the Work Package is not done until that test passes ([validation](../engineering/validation.md)).

Source: Weave `docs/dogfood/2026-10-orkestra/findings.md` and `cycle7.md` at the baseline `c8a2f6e` ([ADR-0032](../architecture/adr/0032-migration-baseline-is-orkestra-dogfood-c8a2f6e.md), [inventory](weave-inventory.md)). The findings are open in Weave and stay unfixed there; Amaru fixes them after the affected component migrates.

Conventions for every spec: tests use **fakes** (fake Provider, fake clock, fake forge); no real agent, paid API or network. Terms follow the [glossary](../product/GLOSSARY.md). Evidence level when implemented: unit-tested (fakes). A recorded real run is a separate item.

| Spec | Finding | Amaru feature | Owning Work Package | Evidence level today |
|---|---|---|---|---|
| [AT-F27](#at-f27-provider-pause-honors-the-stated-reset-time) | F-27 | FEAT-EXE-008 | M5 (catalog); the loop's pause handling is exercised in WP-M4-03 | Specified, not implemented |
| [AT-F39](#at-f39-a-run-that-changes-nothing-does-not-spend-an-attempt) | F-39 | FEAT-WRK-005 | WP-M4-03 | Specified, not implemented |
| [AT-F40](#at-f40-unblock-after-a-review-failure-re-reviews-an-unchanged-head) | F-40 | FEAT-WRK-005 | WP-M4-03 | Specified, not implemented |
| [AT-F41](#at-f41-usage-and-cost-are-recorded-with-provenance) | F-41 | FEAT-OBS-002 ([ADR-0033](../architecture/adr/0033-usage-and-cost-telemetry-model.md)) | M4 | Specified, not implemented |

## AT-F27 Provider pause honors the stated reset time
**Weave behavior (baseline).** `domain.ProviderPause` is a constant 30 minutes (`runtime/internal/domain/autoloop.go:511`). The provider said "resets 2:10pm"; items retried every 30 minutes, 7 refused starts over about 4 hours; no attempt was spent (cycle 7).

**Specification.**
1. *Given* a fake Provider that refuses a Step with a quota error stating a reset instant T, *when* the refusal is handled, *then* the Provider is paused until T (not for a fixed duration) and no attempt is consumed.
2. *Given* the clock advances to just before T, *then* no new start is attempted on that Provider; *when* it reaches T, *then* exactly one start is attempted.
3. *Given* a quota error with **no parsable reset time**, *then* a documented fallback pause applies, the event records that the reset time was unknown, and no attempt is consumed. (The fallback value is a design choice, not fixed here.)
4. *Given* a stated reset time in the past or implausibly far in the future, *then* it is bounded by a documented limit and the event records the clamp.
5. A refused start while paused is not counted as a failure and produces one event per pause, not one per retry.

**Open point.** Parsing locale/timezone of provider messages ("resets 11:10am") is unverified; the spec requires the Provider adapter, not the core, to turn it into an absolute instant.

## AT-F39 A run that changes nothing does not spend an attempt
**Weave behavior (baseline).** The no-change rule (`no_changes`, `runtime/internal/app/autoloop.go:1029`) only sees an empty diff against the base. Attempts 2–4 produced no new commit (head `8600f44` each time) yet were logged as `commit` / `update_pr pushed attempt n` and counted: 4 attempts for one real implementation.

**Specification.**
1. *Given* an implementation Step on an existing change whose head is H, and a fake Provider that makes no edits, *when* the Step ends, *then* the head is still H, the outcome is `no_changes`, the attempt counter is unchanged, and no push or pull-request update is performed.
2. *Then* the run is reported (event and Work Item view) with the reason, and the run is not hidden.
3. *Given* a Provider that makes an edit which results in the same tree as H (no net change), *then* the same outcome applies.
4. *Given* a Provider that makes a real change, *then* the attempt is counted exactly once.
5. After N consecutive `no_changes` outcomes (N from Policy), the item is blocked for a human with the cause, instead of looping.

## AT-F40 Unblock after a review failure re-reviews an unchanged head
**Weave behavior (baseline).** After the attempt budget is exhausted with the last failure in review and the pull-request head unchanged, an unblock re-implemented (attempt 4, a no-op) instead of re-reviewing. Unblock while *reviewing* already re-reviewed.

**Specification.**
1. *Given* an item blocked after the budget was spent, the last failed phase is review, and the change head equals the head that was reviewed, *when* a human unblocks it, *then* the next phase is review, not implementation, and no implementation Step is started.
2. *Given* the last failed phase is implementation, *then* unblock resumes implementation.
3. *Given* the change head differs from the reviewed head, *then* review starts on the new head.
4. The unblock grants a bounded number of extra attempts per Policy and records which phase it resumed and why.
5. Restart recovery follows the same phase rule and does not consume attempts ([RESILIENCE](../architecture/RESILIENCE.md)).

## AT-F41 Usage and cost are recorded with provenance
**Weave behavior (baseline).** The Claude provider runs with `--no-session-persistence`; `parse.go` drops `usage` and `total_cost_usd` from the `result` line. Tokens and cost were not measured for any cycle.

**Specification** (contract per [ADR-0033](../architecture/adr/0033-usage-and-cost-telemetry-model.md)).
1. *Given* a fake Provider whose `result` message carries tokens and cost, *then* the Step Execution gets a Usage record with those values marked `reported`, and the same values appear after a restart.
2. *Given* a Provider that reports nothing, *then* the numeric fields are `unknown`, **never zero**, and aggregates show the unknown share.
3. *Given* a price table version, an `estimated` value records the table version.
4. *Given* three attempts (one failed, one rejected, one accepted) on a Work Item, *then* the Work Item total includes all three and shows reported/estimated/unknown shares.
5. A real-agent run for each supported agent CLI is recorded separately with date and versions (M4 validation); fakes alone do not satisfy it.

## What this document does not claim
- No test in this file exists, ran or passed. They are specifications derived from Weave's findings and Amaru's accepted decisions.
- Weave's own behavior was read from its findings and code references, and the baseline test suite was run ([inventory](weave-inventory.md#test-run-at-the-baseline)). Whether Weave has tests that reproduce these four findings was not checked.
- Parameters left open above (fallback pause, clamp, N) are design choices for the owning Work Package.
