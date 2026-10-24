---
title: Plugin extensibility independent of the desktop framework
status: proposed
date: 2026-10-24
---

# 0005. Plugin extensibility independent of the desktop framework

## Context
Plugins are a core product direction (UI extensions, integrations, Capabilities/Providers, environment adapters). The desktop framework is undecided ([Q5](../OPEN_QUESTIONS.md#q5-desktop-framework)) and Web must run the same UI. Weave's external provider protocol already lets third parties add Providers as executables without touching the Runtime.

## Decision
Plugins are written against Amaru contracts and the plugin SDK only — never against Tauri, Electron or any native host API. Executable plugin code (Providers, environment adapters) runs out of process by default. UI extensions run in an isolated client-side host and reach the Platform only through the public API. A plugin declares its contributions and permissions in a manifest that the Platform enforces.

## Consequences
- Changing the desktop framework does not break plugins.
- The same UI extension works on Desktop and Web.
- Manifest, sandboxing and signing are still open ([Q6](../OPEN_QUESTIONS.md#q6-plugin-sandboxing-and-execution-isolation), [Q7](../OPEN_QUESTIONS.md#q7-plugin-distribution-signing-permissions-and-compatibility)).

## Alternatives considered
- *Framework-native plugins* (e.g. Tauri plugins in Rust): deeper OS access, but ties the ecosystem to one host and excludes Web.

## Approval
Proposed by Claude in the bootstrap. Awaiting product-owner decision.
