---
title: TypeScript plugin logic runs in QuickJS compiled to WebAssembly on wazero
status: superseded
date: 2026-10-25
deciders: Ricardo Lüders
superseded-by: 0040
---

# 0020. TypeScript plugin logic runs in QuickJS compiled to WebAssembly on wazero

## Context
- Question: [Q14](../OPEN_QUESTIONS.md#q14-sandbox-for-typescript-plugin-logic). Product-owner answer: "Pick the most secure option."
- Verified 2026-10-25:
  - wazero is a pure-Go WebAssembly runtime, Apache-2.0, v1.12.0 (2026-05-29), repository moved to `github.com/wazero/wazero`.
  - Extism's Go SDK runs on wazero but its last release is v1.7.1 (2025-03-19) with no pushes since 2025-05; its manifest treats `allowed_hosts: null` as *all hosts* (https://extism.org/docs/concepts/manifest). Its JS PDK compiles with QuickJS-ng.
  - Deno's permission model is bypassed entirely by `--allow-run` or `--allow-ffi` (https://docs.deno.com/runtime/fundamentals/security/), and adds a separate runtime to ship.
- Wasm has no ambient authority: a module can only call host functions the embedder provides.

## Decision
- Plugin `logic` is written in TypeScript, bundled to JavaScript and compiled with a QuickJS(-ng) engine into a **WebAssembly module**, executed by **wazero** inside the Platform's extension host.
- The host instantiates modules with:
  - **No WASI filesystem, network, clock-precision or environment access.**
  - Only Amaru host functions: read/emit Events, call the public API with the plugin's granted permissions, structured logging, key-value plugin state.
  - A per-instance memory cap, a CPU deadline via context cancellation (`WithCloseOnContextDone`), and a call quota.
  - One instance per plugin per Organization; no shared memory between plugins.
- The same module runs unchanged in compact, server and Cloud.
- The build toolchain (Javy or the Extism JS PDK compiler, both QuickJS-based) is chosen in the M6 spike; the Extism Go SDK is **not** used at runtime.

## Consequences
- Strongest portable isolation: memory-safe sandbox, deny-by-default authority, same behavior on all OSes without admin rights.
- Plugin logic cannot use Node APIs or npm packages that need them; the SDK documents a supported subset.
- QuickJS is slower than V8; logic is for event reactions and orchestration, not heavy compute (heavy work belongs in `worker-adapter`).
- Validation (M6): an event-reacting example plugin runs with a fuzzed host-call surface; startup < 50 ms and memory cap enforced (targets to measure).
- Revisit if: QuickJS performance or the toolchain blocks real plugins.

## Alternatives considered
- **Deno with permissions**: good ergonomics, but permission flags can disable the sandbox and it is another runtime per OS.
- **Node with OS sandboxing**: ambient authority by default; OS sandboxes are not portable.
- **Extism Go SDK**: convenient, but unmaintained and permissive `null` defaults.

## Approval
Proposed by: Claude (open-question resolution, 2026-10-25) · Approved by: Ricardo Lüders on 2026-10-25, product-owner planning session (answer recorded in OPEN_QUESTIONS Q14: "pick the most secure option").
