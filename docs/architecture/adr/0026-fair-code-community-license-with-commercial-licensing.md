---
title: Amaru uses a fair-code community license with separate commercial licensing
status: accepted
date: 2026-10-25
deciders: Ricardo Lüders
---

# 0026. Amaru uses a fair-code community license with separate commercial licensing

## Context
- Question: [Q12](../OPEN_QUESTIONS.md#q12-licensing-and-commercial-boundaries). Product-owner answer: "Fair-code usage from the draft Amaru Community License 1.0."
- The draft "Amaru Community License 1.0" (fair-code, source-available; permits internal commercial use and self-hosting; restricts resale and hosted offerings) is held by the product owner and is not in this repository.
- Verified 2026-10-25:
  - `weave-obsidian` is MIT, "Copyright (c) 2026 Ricardo Lüders" on `main` and `feat/orkestra-dogfood`.
  - `weave` (prototype) has no LICENSE file.
  - `git shortlog -sne --all`: `weave-obsidian` commits are by Ricardo Lüders (130) and the owner's own automation identity "Weave Autoloop" (32); `weave` by Ricardo Lüders (52) and "Weave Agent" (2). No third-party contributors.
- Ecosystem growth needs permissive terms for code that others embed (client SDK, plugin SDK, contracts).

## Decision
- The **Amaru Platform, Worker, clients and Desktop** are licensed under the **Amaru Community License 1.0** (fair-code, source-available), with **separate commercial licenses** for uses it restricts (resale, hosted/managed offerings, OEM).
- **Contracts (`packages/contracts`), the client SDK, the plugin SDK and the Worker SDK** are licensed under **Apache-2.0** so plugins and integrations can use any license.
- **Contributions** require a **Contributor License Agreement** granting the project a broad license (no copyright assignment); a DCO alone does not allow dual licensing.
- **Weave code** may be migrated: the copyright holder of both predecessor repositories is the product owner, who consents to relicensing; automation-authored commits were made under the owner's control. MIT notices are preserved in migrated files' history.
- Amaru is never described as "open source".
- Follow-ups (roadmap, owned by the product owner): legal review of the draft text, adding `LICENSE` files (community license at root, Apache-2.0 in SDK/contract packages), CLA tooling, trademark search for "Amaru". Agents do not add or alter license text (AGENTS.md rule 9).

## Consequences
- Migration under ADR-0007/ADR-0011 is no longer blocked by licensing (assumption A7 resolved).
- Hosted competitors need a commercial license; self-hosting stays free under the community license.
- A CLA adds friction for contributors.
- Revisit if: legal review finds the draft unenforceable or incompatible with Apache-2.0 SDK packages.

## Alternatives considered
- **FSL-1.1 / BUSL-1.1 / Elastic License 2.0**: established texts with similar intent; the product owner prefers the Amaru draft (one of these can replace it if legal review requires).
- **OSI open source (Apache-2.0/AGPL) for everything**: weakens the commercial boundary.
- **DCO instead of CLA**: insufficient for dual licensing.

## Approval
Proposed by: Claude (open-question resolution, 2026-10-25) · Approved by: Ricardo Lüders on 2026-10-25, product-owner planning session (answer recorded in OPEN_QUESTIONS Q12; ADR-only resolution without a LICENSE file chosen in the same session).
