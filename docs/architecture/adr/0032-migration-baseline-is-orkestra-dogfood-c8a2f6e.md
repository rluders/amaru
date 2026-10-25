---
title: The Weave migration baseline is feat/orkestra-dogfood at c8a2f6e
status: accepted
date: 2026-10-25
deciders: Ricardo Lüders
---

# 0032. The Weave migration baseline is feat/orkestra-dogfood at c8a2f6e

## Context
- Question: [Q19](../OPEN_QUESTIONS.md#q19-migration-source-branch). Product-owner answer: "`feat/orkestra-dofgook`, I think this is the one that holds the latest commit." The branch is `feat/orkestra-dogfood` (typo in the answer).
- Verified 2026-10-25 in `weave-obsidian`: head `c8a2f6e983beba59edc446f36df2f442edaa9855` (2026-10-18, "Cycle 7: deployment, supervised httpsuite pilot, findings F-38..F-45"), 59 commits ahead of local `main`, 84 ahead of `origin/main`, 0 behind. It alone carries Weave ADR-021…029, the delivery loop, local checks and restart recoveries. MIT license present.
- Open Weave findings: F-27 (fixed 30-minute provider pause), F-39 (re-implementation without changes spends an attempt), F-40 (unblock re-implements instead of re-reviewing), F-41 (token/cost usage dropped).

## Decision
- The migration baseline is **`feat/orkestra-dogfood` at commit `c8a2f6e`**. Each migration ADR names this commit (or a later commit on the same branch, stated explicitly).
- Amaru does **not** wait for Weave to merge the branch to `main` or to fix F-27, F-39, F-40, F-41. These findings become Amaru acceptance tests and are fixed in Amaru after the affected component migrates (F-41 is covered by [ADR-0033](0033-usage-and-cost-telemetry-model.md)).

## Consequences
- The critical path no longer depends on work outside this repository.
- Weave-side fixes, if the owner makes them, are optional input.

## Alternatives considered
- **`origin/main` (`d31c977`)**: lacks the delivery loop and recoveries.
- **Wait for a Weave merge and fixes**: schedule risk outside Amaru's control.

## Approval
Proposed by: Claude (open-question resolution, 2026-10-25) · Approved by: Ricardo Lüders on 2026-10-25, product-owner planning session (answer recorded in OPEN_QUESTIONS Q19; branch name corrected).
