---
title: The migration baseline is a pinned commit of the earlier internal implementation
status: accepted
date: 2026-10-25
deciders: Ricardo Lüders
---

# 0032. The migration baseline is a pinned commit of the earlier internal implementation

## Context
- Question: [Q19](../OPEN_QUESTIONS.md#q19-migration-source-branch). The product owner chose the development branch of the earlier internal implementation that holds the latest work.
- Verified 2026-10-25: that branch is the only one carrying the delivery loop, local checks and restart recoveries, and it is well ahead of the repository's default branch with nothing behind it. A permissive license is present.
- Four open findings of the earlier implementation matter for Amaru: a fixed 30-minute provider pause (F-27), a re-implementation without changes that spends an attempt (F-39), unblock that re-implements instead of re-reviewing (F-40), and token and cost usage being dropped (F-41).

## Decision
- The migration baseline is **one pinned commit** on that branch, recorded in the private migration notes. Each migration ADR names this commit (or a later commit on the same branch, stated explicitly).
- Amaru does **not** wait for the earlier implementation to merge the branch or to fix F-27, F-39, F-40 and F-41. These findings become Amaru acceptance tests and are fixed in Amaru after the affected component migrates (F-41 is covered by [ADR-0033](0033-usage-and-cost-telemetry-model.md)).

## Consequences
- The critical path no longer depends on work outside this repository.
- Fixes made in the earlier implementation, if the owner makes them, are optional input.

## Alternatives considered
- **The default branch**: lacks the delivery loop and recoveries.
- **Wait for a merge and fixes**: schedule risk outside Amaru's control.

## Approval
Proposed by: Claude (open-question resolution, 2026-10-25) · Approved by: Ricardo Lüders on 2026-10-25, product-owner planning session (answer recorded in OPEN_QUESTIONS Q19; branch name corrected).
