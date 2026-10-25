---
title: Usage and cost are recorded per Step Execution with explicit provenance
status: accepted
date: 2026-10-25
deciders: Ricardo Lüders
---

# 0033. Usage and cost are recorded per Step Execution with explicit provenance

## Context
- Question: [Q20](../OPEN_QUESTIONS.md#q20-cost-and-usage-telemetry-model). Product-owner answer: "You can decide."
- Weave `domain.Usage` is `{Credits, Quantity, Unit, DurationMS}` with no tokens or currency; the Claude provider drops `usage` and `total_cost_usd` (finding F-41). Providers report different things: tokens, credits, nothing.
- PRD principle 9 and FR-33 require visible cost, attributable to accepted work.

## Decision
- Each Step Execution has zero or more **Usage records**: provider, model, `input_tokens`, `output_tokens`, `cache_read_tokens`, `cache_write_tokens`, `cost_amount` (decimal) + `cost_currency`, `credits`, `wall_duration_ms`, Environment id and cost class, `captured_at`.
- Every numeric field is nullable and paired with a **source**: `reported` (from the provider), `estimated` (computed by Amaru), or `unknown`. Unknown is never stored as zero.
- Estimates use a **versioned price table** (per provider/model, effective dates); the table version is recorded with each estimate. Environment time is priced from the Environment template's cost class.
- **Roll-up**: Step Execution → Execution → Work Item → Work Package, shown with the share of reported vs estimated vs unknown. Cost of accepted work includes all attempts (failed, repaired, rejected) that led to it.
- Providers return usage in the `result` message of `amaru.provider/v1`; adapters for agent CLIs parse their reported usage (fixes F-41).

## Consequences
- Honest totals: dashboards never under-report by treating unknown as free.
- Price tables need maintenance; stale tables are visible by version.
- Validation (M4): usage captured from each supported agent CLI in recorded runs.

## Alternatives considered
- **Tokens only**: misses credit-based and tool-only providers.
- **Unknown as zero**: hides cost.

## Approval
Proposed by: Claude (open-question resolution, 2026-10-25) · Approved by: Ricardo Lüders on 2026-10-25, product-owner planning session (decision delegated to Claude in OPEN_QUESTIONS Q20 and accepted in the same session).
