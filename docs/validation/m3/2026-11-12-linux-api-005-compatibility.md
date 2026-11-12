# API-005 client compatibility — Linux, 2026-11-12

## Scope and environment

Issue [#106](https://github.com/rluders/amaru/issues/106), parent [#18](https://github.com/rluders/amaru/issues/18). Base: #18 integration at `f03f9e425cdc2dc46b9ee1a73ebc6c6a4e218a07`, including accepted ADR-0059 and ADR-0060. Runner: Codex for Ricardo Lüders. Linux `7.2.11-100.fc43.x86_64`, Node `v25.7.0`, pnpm `11.5.0`.

This fresh Amaru implementation adds HTTP compatibility checks to the existing TypeScript SDK. No predecessor code is imported. No contract, generated type, dependency or infrastructure changes. The pinned installed openapi-fetch 0.17.0 source/types were inspected: middleware receives the effective request base URL and fetch transport, awaits onRequest before sending, and propagates a thrown error without dispatch.

## Behavior and evidence

- **Unit-tested:** valid protocol major 1 permits product calls; additive fields and an unrelated platformVersion build string remain compatible. Every product operation probes again, including after a successful prior call, so a replacement with major 2 prevents the next mutation. A later compatible response permits recovery.
- **Unit-tested:** incompatible major, missing/string/fractional/zero version, null/missing health, invalid status/uptime/timestamp, malformed JSON, HTTP failure and thrown network errors prevent product dispatch. CompatibilityError exposes distinct incompatible, invalid-health and connection states with safe explanations and no response body, credentials or transport details.
- **Unit-tested:** probes omit the administrator bearer header, reject redirects, disable caching, and use the effective per-request base URL/transport. Direct health reads remain available without recursive negotiation. Existing typed product errors and command bodies remain covered.
- **Integration-tested:** a real Node HTTP fixture server on Linux loopback accepts compatible negotiation, rejects a subsequent mismatched response and refuses a health redirect. Exactly one product mutation reaches the fixture server; the redirect target receives none.
- **Unit-tested (existing coverage):** EventStream's independent Hello-major mismatch yields its incompatible state without event delivery or retry.

The mismatch fixtures are the health object variations in `packages/sdk/test/compatibility.test.ts`. Runtime checks are deliberately compatible with additive fields. The canonical Health protocol remains integer major 1; no health SemVer field, arbitrary supported-range option or Worker/Provider handshake is invented.

## Commands and results

- `make sdk-ts`: exit 0; frozen-lockfile install, generated type staleness check, TypeScript typecheck, **34 tests passed, 0 failed, 0 skipped**. Includes the loopback test.
- `make check`: exit 0 on the complete change; local Linux repository checks, not hosted CI.
- `git diff --check` and `trail check`: exit 0 before commit.

Architecture review verdict: **sound** within ADR-0025 and the accepted public API/client boundary. Clients read the public health contract; no domain state or authorization moves to the client. No new dependency or infrastructure. Invalid negotiation causes no product request. Simpler caller-only manual checks can be forgotten; the existing middleware enforces the order. An extra health round trip per product call avoids stale compatibility caches after restart. Compatibility does not establish identity or replace #92 authentication.

## Limits

No UI rendering exists in this SDK: explicit error kinds/messages are available for consuming clients. No composed Platform public API, native browser, real WebSocket server, Worker/Provider handshake or native macOS/Windows was validated. Node 22 was not run. Loopback fixture integration is not real Platform validation. #92 must still reconcile local session authentication and the existing WebSocket token transport; #18 owns combined server acceptance. Linux is the required M3 target and native macOS/Windows are best effort. This child change does not complete #18 or M3.
