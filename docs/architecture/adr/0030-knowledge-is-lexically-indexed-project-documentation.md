---
title: Knowledge is project documentation indexed lexically in PostgreSQL
status: accepted
date: 2026-10-25
deciders: Ricardo Lüders
---

# 0030. Knowledge is project documentation indexed lexically in PostgreSQL

## Context
- Question: [Q17](../OPEN_QUESTIONS.md#q17-knowledge-scope-and-retrieval). Product-owner answer: "Your choice."
- Weave ADR-022 (accepted, not built) defines Knowledge as a read-only indexed view returning cards (path, title, aliases, tags, headings) and single sections, explicitly "not semantic indexing". Weave `store/fsscan` indexes Markdown frontmatter up to 1 MiB.
- Embeddings require sending content to a model provider (privacy, cost) and a vector store; pgvector is available under the PostgreSQL license.
- Agents already read source code directly through their tools; Knowledge is about context selection ("least context necessary").

## Decision
- **Scope**: committed Markdown in the Project — documentation, ADRs, Skills, Playbooks, Artifact notes — plus Amaru definitions under `.amaru/`. **Not** source code, binaries, ignored or secret files. Projects can add include/exclude globs in `.amaru/config.yaml`.
- **Retrieval**: lexical, using PostgreSQL full-text search over title, headings, frontmatter and section text, returning cards and sections (Weave ADR-022 shape).
- **Index location**: Platform PostgreSQL, per Project and Organization-scoped; rebuilt from Git at a commit; never written into the Project.
- **Semantic retrieval** (pgvector) is not part of the product until an evaluation on recorded runs shows lexical retrieval missing relevant context; it would then be opt-in per Organization with a declared embedding provider.

## Consequences
- No content leaves the Platform for indexing; works offline and in compact installs.
- Recall on paraphrased queries is weaker; Agents can still read files directly.
- Validation (after M4): measure context hit rate and size on recorded runs (FEAT-OBS-002).

## Alternatives considered
- **Semantic by default**: privacy and cost for every Project.
- **Index source code**: duplicates agent tooling, large and sensitive.

## Approval
Proposed by: Claude (open-question resolution, 2026-10-25) · Approved by: Ricardo Lüders on 2026-10-25, product-owner planning session (decision delegated to Claude in OPEN_QUESTIONS Q17 and accepted in the same session).
