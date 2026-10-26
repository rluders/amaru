---
name: documentation-maintenance
description: Keep Amaru product, architecture, decision and engineering docs consistent with each other and with what is actually implemented. Use after changing behavior, decisions, terminology or structure, and when auditing docs for drift or overclaiming.
---

# Documentation maintenance

## Purpose
Docs that are true: no overclaiming, no contradictions, no stale status.

## When to use
- After a code, contract, structure or decision change.
- When an ADR status changes or an open question closes.
- Periodic audit, or before a review or release.

## Workflow
1. **Find affected docs**: `rg -n "<term>" README.md AGENTS.md CLAUDE.md docs .agents`. Start from the [glossary](../../../docs/product/GLOSSARY.md) for terminology changes.
2. **Status honesty.** Each behavior statement uses the [status vocabulary](../../../docs/engineering/CONVENTIONS.md#status-vocabulary). Audit for overclaiming: `rg -n -i "supports|provides|available|implemented|works" README.md docs` and confirm each hit against code or a recorded run.
3. **Single source.** Content lives in one canonical doc under `docs/`; others link to it. Root `PRD.md` / `ARCHITECTURE.md` stay indexes.
4. **Cross-document consistency.** Check together: PRD scope ↔ ROADMAP stages ↔ ARCHITECTURE labels ↔ DECISIONS statuses ↔ OPEN_QUESTIONS ↔ README status table. An ADR status change updates every doc that cites it.
5. **Prototype references.** Claims about the prototype cite a recorded evidence snapshot with its commit; if newer evidence is used, refresh the snapshot and its commit.
6. **Commands.** Every command in README/AGENTS/CLAUDE/validation exists and was run.
7. **Contradictions you cannot resolve**: do not pick silently. Record them (OPEN_QUESTIONS or the relevant doc) and ask.

## Output
The doc edits and a list: files changed, contradictions found and how each was resolved or escalated.

## Verification
- `make docs-check` passes.
- Each overclaiming grep hit was checked.
- No ADR was marked accepted without a recorded approval.
