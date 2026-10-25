---
title: Amaru Desktop is a Tauri v2 shell over the Web client
status: accepted
date: 2026-10-25
deciders: Ricardo Lüders
---

# 0018. Amaru Desktop is a Tauri v2 shell over the Web client

## Context
- Question: [Q5](../OPEN_QUESTIONS.md#q5-desktop-framework). Product-owner answer: "Tauri v2 seems pretty mature, Electron is terrible and Wails is too immature."
- Verified 2026-10-25:
  - Tauri 2.0 stable since 2024-10-02; latest 2.12.5 (2026-10-21) (https://v2.tauri.app/blog/tauri-20/).
  - Sidecars via `bundle.externalBin` with a per-target-triple binary, launched through the shell plugin with `sidecar: true` and argument validators (https://v2.tauri.app/develop/sidecar/).
  - Capabilities grant permissions per window and platform; app commands are reachable from all windows unless restricted in `build.rs` (https://v2.tauri.app/security/capabilities/).
  - The updater requires signed updates (https://v2.tauri.app/plugin/updater/).
  - Wails v3 is still beta (v3.0.0-beta.34, 2026-10-20).
- [ADR-0003](0003-desktop-is-a-client.md) and [ADR-0010](0010-local-mvp-client-is-a-web-ui.md): Desktop is a client that wraps the same Web UI, at M8.

## Decision
- Amaru Desktop uses **Tauri v2** with a thin Rust host.
- The host:
  - Ships the Platform binary as an `externalBin` sidecar and supervises it for local Instances.
  - Loads the same React Web client; all data goes through the public API, never Tauri commands.
  - Exposes only OS integration commands (tray, notifications, deep links, keychain access for Instance credentials, open-in-browser), restricted per window in `build.rs` and capabilities.
  - Uses the signed Tauri updater on all three OSes.
- No product logic in Rust.

## Consequences
- Small bundles and a narrow native surface; Rust stays limited to the shell.
- WebView differences (WebView2, WKWebView, WebKitGTK) must be covered by UI tests on each OS.
- Code-signing certificates for Windows and macOS notarization are a release prerequisite (M8).
- Validation (M8 spike): sidecar supervision, keychain, updater and signing on Windows, macOS and Linux.
- Revisit if: WebKitGTK gaps block the UI on Linux, or sidecar supervision cannot meet NFRs.

## Alternatives considered
- **Electron**: large bundles and a broad Node attack surface; rejected by the product owner.
- **Wails v3**: Go-native, but still beta.
- **Web-only forever**: loses tray, notifications and local Instance supervision.

## Approval
Proposed by: Claude (open-question resolution, 2026-10-25) · Approved by: Ricardo Lüders on 2026-10-25, product-owner planning session (answer recorded in OPEN_QUESTIONS Q5).
