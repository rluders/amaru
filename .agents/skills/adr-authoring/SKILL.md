---
name: adr-authoring
description: Create, revise or evaluate an Architecture Decision Record in docs/architecture/adr following the Amaru ADR convention (numbering, frontmatter, statuses, index, approval rule). Use whenever a significant or hard-to-reverse decision is proposed, changed or superseded.
---

# ADR authoring

## Purpose
Record decisions so a future reader knows what was decided, why, on what evidence, and who approved it — and never mistakes a proposal for a decision.

## When to use
- A choice about boundaries, contracts, infrastructure, security model, licensing, or migration.
- An open question in OPEN_QUESTIONS has enough evidence to decide.
- An accepted ADR needs to change (write a superseding ADR).

Not for: reversible implementation details; unresolved questions without evidence (add them to OPEN_QUESTIONS instead).

## Workflow
1. **Check the log.** Read [DECISIONS.md](../../../docs/architecture/DECISIONS.md) and related ADRs and open questions. Avoid duplicates; find what this supersedes.
2. **Number.** Next free `NNNN` (look at `docs/architecture/adr/`). Never reuse a number.
3. **Copy** `docs/architecture/adr/0000-template.md` to `NNNN-lowercase-title.md`.
4. **Frontmatter**: `title`, `status: proposed` (or `under-investigation`), `date: YYYY-MM-DD`. Leave `deciders` empty until approval. Single-line values only.
5. **Context**: forces and evidence with links. Separate verified facts from assumptions. Cite prototype ADRs or tests when they are the evidence.
6. **Decision**: active voice, checkable ("The Platform owns…", "Clients must not…").
7. **Consequences**: what gets easier/harder, follow-up work, revisit triggers.
8. **Alternatives**: real options with the reason each lost.
9. **Index**: add one row to the DECISIONS table with the same status and date.
10. **Approval**: ask the human. Only after an explicit approval, set `status: accepted`, `deciders: <name>`, update `date`, fill the Approval section (who, when, where), and update the index row. An agent never accepts its own proposal; a request to investigate is not approval.
11. **Superseding**: new ADR with `supersedes: NNNN`; set the old one to `superseded` with `superseded-by`, and update both index rows. Do not rewrite an accepted ADR's substance.

## Evaluating an existing ADR
Check: is the decision checkable? Is evidence linked and current? Are alternatives real? Is the status honest (built-on but only `proposed` is a finding)? Do other docs contradict it?

## Output
The ADR file and the index row, plus a one-paragraph summary for the human with the decision requested.

## Verification
- `make docs-check` passes (frontmatter, numbering, index row and status).
- `rg -n "ADR-NNNN|NNNN-" docs` shows references use the right number.
