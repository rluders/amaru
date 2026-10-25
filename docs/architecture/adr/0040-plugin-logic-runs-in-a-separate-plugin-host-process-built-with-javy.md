---
title: Plugin logic runs as Javy-built QuickJS Wasm on wazero in a separate plugin-host process
status: accepted
date: 2026-10-25
deciders: Ricardo Lüders
supersedes: 0020
---

# 0040. Plugin logic runs as Javy-built QuickJS Wasm on wazero in a separate plugin-host process

## Context
- [ADR-0020](0020-plugin-logic-runs-in-quickjs-on-wazero.md) chose QuickJS compiled to WebAssembly on wazero for plugin `logic` (product-owner answer to [Q14](../OPEN_QUESTIONS.md#q14-sandbox-for-typescript-plugin-logic): "Pick the most secure option"). It left two things open or inconsistent:
  - It ran modules "inside the Platform's extension host", and [ADR-0019](0019-isolation-tiers-for-plugins-and-providers.md) said "inside the Platform", while [ADR-0009](0009-modular-monolith-platform-with-separate-worker-role.md) says "Plugin hosts and external Providers are always separate processes". The security model, NFR-18, the PRD and the feature catalog follow ADR-0009 ([Q23](../OPEN_QUESTIONS.md#q23-where-plugin-logic-executes)).
  - The build toolchain (Javy or the Extism JS PDK compiler) was left "chosen in the M6 spike" ([Q24](../OPEN_QUESTIONS.md#q24-plugin-logic-build-toolchain)).
- Verified 2026-10-25:
  - Javy (`bytecodealliance/javy`): Apache-2.0, v9.1.0 (2026-07-30), active (last push 2026-10-23). Javy *plugins* define the JavaScript globals and the host imports of the generated module; a plugin written in the WASI preview 2 style is initialized to a `wasm32-unknown-unknown` module with **no WASI imports**, using only host calls the embedder defines (https://github.com/bytecodealliance/javy/blob/main/docs/docs-using-extending.md).
  - Extism JS PDK (`extism/js-pdk`): BSD-3-Clause, v1.7.0 (2026-08-17). Its modules import the Extism kernel ABI, which assumes the Extism runtime that ADR-0020 rejected (unmaintained Go SDK, permissive `allowed_hosts: null` default).
- Delegated in the 2026-10-25 planning prompt ("make the most defensible decision").

## Decision
- Plugin `logic` is TypeScript, bundled to JavaScript and compiled with **Javy** together with an **Amaru Javy plugin** (Rust, preview-2 style). The result is a `wasm32-unknown-unknown` module with no WASI imports.
- The Amaru Javy plugin exposes only Amaru host calls: read and emit Events, call the public API with the plugin's granted permissions, structured logging, and key-value plugin state. Its import namespace and ABI are versioned with the plugin SDK ([ADR-0025](0025-api-and-protocol-versioning-policy.md)).
- Modules run on **wazero** inside a **separate `amaru plugin-host` process**. That process is the same binary as the Platform, started and supervised by the Platform, with one process per Organization. It talks to the Platform only through the public API and the event stream, using a scoped token per plugin installation.
- Per-instance limits are kept from ADR-0020:
  - a memory cap;
  - a CPU deadline through context cancellation;
  - a call quota;
  - one instance per plugin per Organization, with no shared memory between plugins.
- Native code is never loaded into the Platform process. A plugin-host crash affects only that Organization's plugin `logic`; the Platform restarts the host with back-off.
- The same module runs unchanged in compact, server and Cloud.

## Consequences
- Two boundaries protect the Platform from plugin code: the Wasm sandbox and the process boundary. ADR-0009, the security model and NFR-18 hold without exception.
- Amaru maintains a small Rust Javy plugin and the matching Go host bindings. The plugin SDK documents the supported JavaScript subset (no Node APIs).
- One extra process per Organization with plugins. In compact installs this is one process.
- Validation (M6, validation register A10): an event-reacting example plugin runs against a fuzzed host-call surface. Targets: cold start < 50 ms, memory cap enforced, plugin-host crash isolated.
- Revisit if: Javy's plugin API stops supporting WASI-free modules, or QuickJS performance blocks real plugins. In that case heavy logic moves to `worker-adapter` components.

## Alternatives considered
- **Extism JS PDK**: maintained, but it needs the Extism kernel ABI, which means either the rejected Extism runtime or reimplementing its kernel.
- **Run wazero in the Platform process (ADR-0020 as written)**: one less process, but it breaks ADR-0009 and puts third-party code in the process that holds every Organization's state.
- **Deno or Node in a child process**: ambient authority by default (see ADR-0020).

## Approval
Proposed by: Claude (second-pass open-question resolution, 2026-10-25) · Approved by: Ricardo Lüders on 2026-10-25, planning prompt that delegated every remaining decision ("make the most defensible decision"; status choice "accepted, delegated" confirmed in the same session).
