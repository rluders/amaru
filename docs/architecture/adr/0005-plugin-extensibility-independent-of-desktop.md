---
title: Plugin extensibility independent of the desktop framework
status: accepted
date: 2026-10-25
deciders: Ricardo Lüders
---

# 0005. Plugin extensibility independent of the desktop framework

## Context
Plugins are a core product direction (UI extensions, integrations, Capabilities/Providers, environment adapters). The desktop shell (Tauri v2, [ADR-0018](0018-desktop-shell-is-tauri-v2.md)) must not leak into the plugin model, and Web must run the same UI. Weave's external provider protocol already lets third parties add Providers as executables without touching the Runtime.

## Decision
Plugins are written against Amaru contracts and the plugin SDK only — never against Tauri, Electron or any native host API. Executable plugin code (Providers, environment adapters) runs out of process by default. UI extensions run in an isolated client-side host and reach the Platform only through the public API. A plugin declares its contributions and permissions in a manifest that the Platform enforces.

## Consequences
- Changing the desktop framework does not break plugins.
- The same UI extension works on Desktop and Web.
- Isolation, logic sandbox, manifest and signing: [ADR-0019](0019-isolation-tiers-for-plugins-and-providers.md), [ADR-0020](0020-plugin-logic-runs-in-quickjs-on-wazero.md), [ADR-0021](0021-plugin-packaging-signing-permissions-and-compatibility.md).

## Alternatives considered
- *Framework-native plugins* (e.g. Tauri plugins in Rust): deeper OS access, but ties the ecosystem to one host and excludes Web.

## Approval
Proposed by: Claude (bootstrap, 2026-10-24) · Approved by: Ricardo Lüders on 2026-10-25, product-owner planning session that resolved all open questions (explicit acceptance of every pending ADR).
