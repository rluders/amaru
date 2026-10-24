---
title: Record architecture decisions as ADRs
status: accepted
date: 2026-10-24
deciders: Ricardo Lüders (product owner)
---

# 0001. Record architecture decisions as ADRs

## Context
Amaru starts as documentation before code, and its predecessor (Weave) showed both the value of ADRs and their failure modes: two files numbered ADR-009, ADRs left `proposed` while their content was built, and docs contradicting accepted ADRs (Weave `docs/ROADMAP.md` §10). Agents write much of the documentation, so the line between "recommended" and "approved" must be explicit.

## Decision
Significant decisions are recorded as numbered ADRs under `docs/architecture/adr/`, following the convention in [DECISIONS.md](../DECISIONS.md): fixed statuses, frontmatter, one index row per ADR, `accepted` only with a named human decider, and supersession instead of rewriting accepted ADRs. `make docs-check` enforces numbering, frontmatter and index consistency.

## Consequences
- Duplicate numbers and index/status drift fail the local check.
- Agents can draft ADRs but cannot accept them.
- Small, reversible choices do not need ADRs; open questions stay in OPEN_QUESTIONS until evidence exists.

## Alternatives considered
- *A single decisions document*: harder to cite and supersede individually.
- *Weave's format unchanged*: no machine-checkable status; the observed drift would repeat.

## Approval
Proposed by Claude in the bootstrap plan. Approved by Ricardo Lüders on 2026-10-24 (plan approval, Amaru bootstrap session).
