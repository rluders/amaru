---
title: The Local MVP client is a Web UI served by the Platform
status: accepted
date: 2026-10-25
deciders: Ricardo Lüders
---

# 0010. The Local MVP client is a Web UI served by the Platform

## Context
- PRD FR-41 required one first-party client for the MVP and left the choice open between the Obsidian plugin, a Desktop app and a Web client.
- Verified: the earlier prototype has a React 19 + TanStack Router/Query + Tailwind 4 + shadcn/ui dashboard with layout shells, theme tokens and about 70 route files, many without backend support and no tests.
- Verified: the prototype's Obsidian plugin couples about 16 files to Obsidian APIs; its client, state and type layers are host-independent.
- Desktop (Tauri v2, [ADR-0018](0018-desktop-shell-is-tauri-v2.md)) ships at M8; putting it on the MVP critical path adds packaging, signing and webview risk.
- The product owner chose this direction on 2026-10-25 during master-roadmap planning; this ADR records it for formal acceptance.

## Decision
- The Local MVP's first-party client is **Amaru Web**: a React + TypeScript single-page application **served by the local Platform** and opened in the user's browser.
- It uses only the generated SDK over the public API.
- The product UI lives in shared packages so the later Desktop (M8) wraps the same UI instead of rebuilding it.
- The earlier prototype dashboard is the starting asset (design system, shells), adapted under a migration ADR; pages without a backend are not carried over as if functional.
- Local authentication of the browser client follows [ADR-0034](0034-local-web-client-authentication-with-launch-code-and-cookie.md) and is implemented before M4.

## Consequences
- Desktop is no longer on the MVP critical path; it follows at M8.
- The Web client serves self-hosted and Cloud later without a second UI.
- Browser-to-loopback security (CSRF, DNS rebinding, token handling) becomes an MVP concern.
- No OS-native integration (tray, notifications) in the MVP.
- The Obsidian plugin is not the Amaru MVP client; it is retired ([ADR-0031](0031-obsidian-plugin-is-retired.md)).
- Revisit if: browser-based local use proves unacceptable to users in MVP validation.

## Alternatives considered
- **Tauri Desktop first**: closer to the long-term workspace but adds native packaging and signing to the critical path.
- **Keep the Obsidian plugin**: least new UI, but ties the product to Obsidian, contrary to the vision.

## Approval
Proposed by: Claude (master roadmap planning, 2026-10-25), recording product-owner direction given the same day · Approved by: Ricardo Lüders on 2026-10-25, product-owner planning session that resolved all open questions (explicit acceptance of every pending ADR).
