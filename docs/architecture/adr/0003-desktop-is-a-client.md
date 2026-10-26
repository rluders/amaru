---
title: Desktop is a client, not the owner of execution state
status: accepted
date: 2026-10-25
deciders: Ricardo Lüders
---

# 0003. Desktop is a client, not the owner of execution state

## Context
The prototype enforces that its Obsidian plugin "never spawns processes or holds domain logic"; the Runtime owns execution state, and the plugin survives Runtime restarts through a reconnect state machine (validated in the prototype). Amaru adds a native Desktop and a Web client, and Workers on other machines. If the Desktop owned state, closing a laptop would stop work and Web/Cloud clients would need a different model.

## Decision
Amaru Desktop is a client of the Platform's public API, like Web and editor plugins. It may install, start, stop and supervise a **local** Platform process, but it reads and changes Platform state only through the public API. Execution state, Events, Approvals and Policy live in the Platform.

## Consequences
- Every Desktop feature is available to other clients through the same API.
- The native host (Tauri v2, [ADR-0018](0018-desktop-shell-is-tauri-v2.md)) stays thin: OS integration and process supervision only.
- Desktop-only shortcuts into Platform storage are not allowed.

## Alternatives considered
- *Desktop embeds the Platform as a library*: simpler local install, but couples execution lifetime to the UI and breaks the Web/Cloud parity.

## Approval
Proposed by: Claude (bootstrap, 2026-10-24) · Approved by: Ricardo Lüders on 2026-10-25, product-owner planning session that resolved all open questions (explicit acceptance of every pending ADR).
