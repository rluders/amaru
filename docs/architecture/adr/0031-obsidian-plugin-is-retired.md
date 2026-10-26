---
title: The prototype Obsidian plugin is retired, with no Amaru Obsidian client
status: accepted
date: 2026-10-25
deciders: Ricardo Lüders
---

# 0031. The prototype Obsidian plugin is retired, with no Amaru Obsidian client

## Context
- Question: [Q18](../OPEN_QUESTIONS.md#q18-future-of-the-obsidian-plugin). Product-owner answer: "Obsidian plugin is retired."
- [ADR-0010](0010-local-mvp-client-is-a-web-ui.md): Amaru Web is the first-party client. ADR-0007 forbids changing the prototype for Amaru's sake.

## Decision
- Amaru has **no Obsidian client** and no Obsidian integration plugin. FEAT-CLI-013 is dropped.
- The prototype Obsidian plugin is not migrated. Its UI behaviors that Amaru needs are specified as Amaru Web features.
- The prototype is left unchanged by Amaru work; archiving it is the owner's action once the Local MVP (M4) replaces daily use.

## Consequences
- One first-party client to maintain until Desktop (M8).
- Users who keep notes in Obsidian use Amaru Web; Knowledge still indexes their Markdown ([ADR-0030](0030-knowledge-is-lexically-indexed-project-documentation.md)).

## Alternatives considered
- **Thin Obsidian integration over the SDK**: maintenance of a second client for a niche path.
- **Keep prototype as a parallel product**: splits effort.

## Approval
Proposed by: Claude (open-question resolution, 2026-10-25) · Approved by: Ricardo Lüders on 2026-10-25, product-owner planning session (answer recorded in OPEN_QUESTIONS Q18).
