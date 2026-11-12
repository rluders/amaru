---
title: Adapt selected prototype auth and discovery helpers for compact installs
status: proposed
date: 2026-11-12
deciders:
---

# 0060. Adapt selected prototype auth and discovery helpers for compact installs

## Context

This proposal addresses `MADR-auth-discovery` ([#84](https://github.com/rluders/amaru/issues/84), parent [#18](https://github.com/rluders/amaru/issues/18)). [ADR-0007](0007-reuse-of-earlier-internal-code-is-a-separate-activity.md) requires explicit component approval before predecessor code moves. Accepted [ADR-0034](0034-local-web-client-authentication-with-launch-code-and-cookie.md) already defines the local browser security model; this proposal selects reusable helpers, not a replacement authentication design.

**Verified source:** `rluders/weave-obsidian`, commit `c8a2f6e983beba59edc446f36df2f442edaa9855` ([ADR-0032](0032-migration-baseline-is-a-pinned-commit.md)), components `runtime/internal/auth/` and `runtime/internal/discovery/`. Four Go files contain five tests. All five passed on Linux on 2026-11-12 in an isolated extraction of that commit; the [evidence record](../../validation/m3/2026-11-12-linux-madr-auth-discovery.md) records the command, source blobs and limits. These are predecessor unit/loopback tests, not Amaru browser or OS security validation.

The source generates a 32-byte random shared token, compares tokens with `crypto/subtle`, and publishes endpoint/PID/token/version/start-time JSON using a temporary file and rename. It requests a 0700 directory and creates a 0600 temporary file on Unix. It does not verify an existing directory's permissions or implement Windows ACLs. Its owner cleanup reads then removes the file without synchronization; another writer can replace it between those operations. Its liveness probe accepts any health response with HTTP 200, follows the default HTTP client's redirect policy, and does not establish process identity. Source token tests can print token values on failure. None of these limitations is an Amaru security guarantee.

Accepted [ADR-0059](0059-adapt-the-prototype-http-handlers-to-the-platform-public-api.md) excludes predecessor auth middleware from the HTTP migration boundary. That middleware permits missing Origin and Event-stream query tokens, and has no Host check. Amaru must implement ADR-0034 rather than retain these desktop-client assumptions.

## Decision

Propose **Adapt selected helpers**, using **fresh import with provenance**, subject to product-owner approval of the exact component, commit, boundary, history option and evidence gaps below.

1. **Import boundary.** Adapt `auth/token.go`'s `NewToken` and `Equal`, plus selected `discovery/discovery.go` publication, read, cleanup and liveness behavior. Do not import `MustRandHex` without a real caller; it is not an authentication primitive. Preserve error handling, replace unsafe assumptions, and retain only tests relevant to the Amaru target. Each implementation PR records source paths and blob IDs in `services/platform/PROVENANCE.md` under this ADR. Amaru becomes canonical; no predecessor change or synchronization is authorized. A later source revision requires renewed provenance review and owner approval.
2. **Placement and boundaries.** Compact discovery and HTTP/session adapters live outside the pure domain under `services/platform`, with no new package until real content and its validation target land. The Platform owns authorization and authoritative domain state. CLI/Desktop read the per-user state file and use its administrator secret; the browser never reads that file or receives that secret. Adapt Runtime → Platform and Plugin → client per the glossary. Rename predecessor discovery/version terminology for Amaru without adding legacy aliases; the private file format is not a public API schema. Self-hosted/Cloud identity remains governed by ADR-0028, not by the compact administrator secret.
3. **Discovery lifecycle.** Bind loopback only, one Instance per OS user, publish only after the listener is ready, and keep secrets readable only by that user: 0700 state directory/0600 file on Unix and a user-SID ACL on Windows per ADR-0034. Validate existing paths and permissions; refuse unsafe publication rather than silently weakening protection. Clean up failed temporary writes, preserve a valid previous record on failure, and serialize publication/ownership checks/removal so a stopping Instance cannot erase a replacement record. PID alone is not identity. Treat stale, malformed, non-loopback or inconsistent endpoints as unusable; do not follow discovery-probe redirects off loopback or send credentials to an unverified destination. Validate health protocol/build fields against current contracts before reuse; a 200 response alone proves neither identity nor compatibility. Restart rotates the administrator secret and invalidates prior launch/session credentials.
4. **Browser authentication is new Amaru work under #92.** Preserve ADR-0034: authenticated CLI/Desktop requests obtain a single-use code of at least 256 random bits with a 60-second TTL; exchange it for a server-side session and immediately redirect to a code-free URL. Consume codes atomically so concurrent exchanges cannot both succeed. The host-only cookie is HttpOnly, SameSite=Strict, Path=/, with Secure where the browser permits it on loopback. Validate the bound loopback Host/port; require allowed Origin for browser mutations and WebSocket upgrades and CSRF tokens for non-GET browser requests. Cookie authentication must not fall through to a blanket missing-Origin bypass. Credentialed CLI/Desktop requests remain a distinct path, without browser cookies. No bearer tokens in browser storage, served pages, later URLs or WebSocket query parameters. Unauthenticated health and the launch exchange have narrow explicit handling, not a general authentication exemption.
5. **Contracts and logs.** Existing [OpenAPI](../../../packages/contracts/openapi/openapi.yaml), [Health](../../../packages/contracts/schemas/Health.json), [Error](../../../packages/contracts/schemas/Error.json), [fixtures](../../../packages/contracts/fixtures/manifest.json), routeguard and SDK remain authoritative. Health's integer `protocolVersion` differs from `platformVersion`. The OpenAPI Origin extension's token-only wording does not fully express ADR-0034's cookie path; #92 must reconcile that metadata and add any required auth operations/shapes with contract review, fixtures, routeguard and SDK regeneration together. This proposal invents no launch-code issuance endpoint or CSRF wire format. Errors, diagnostics and test failures must not print secrets, cookies, launch codes or raw launch URLs; #170's capture must cover this adapter. Rejections dispatch no command and produce no domain mutation.
6. **Scope and gates.** This documentation change moves no code and adds no dependency, infrastructure or license text. #92 supplies the implementation, #106 compatibility and #170 secret-safe logging; #18 owns running-server composition. #83's acceptance does not accept this proposal. #84 stays open until explicit owner approval is recorded and this ADR is merged as accepted. Linux is the required M3 target; native macOS/Windows are best effort, with unmet ACL/security evidence recorded and no claim that an insecure fallback is supported.

## Behavior verification plan

All target rows are **not validated in Amaru**; implementation uses Amaru-owned fixtures, fake clocks/randomness/failure seams where needed, temp directories and loopback clients, with no predecessor checkout, real agent or paid API prerequisite.

| Behavior | Required verification |
|---|---|
| Randomness and comparison | Token size/encoding, distinct tokens, equal/unequal/empty/malformed inputs, entropy failure with no publication/session; no credential values in failure messages. Source: `TestTokenUniqueAndComparable` |
| Private atomic publication | New and existing directories, unsafe permissions/path substitution, partial write/close/rename failure, old valid record preserved, no temporary litter, unreadable/malformed files. Native Linux file-mode checks; Windows user-SID ACL checked separately. Source: `TestWriteIsAtomicPrivateAndRemovable`, `TestWriteRequiresEndpoint` |
| Ownership and restart | Missing/stale record, PID reuse, two simultaneous starters, old cleanup interleaved with replacement publication, repeated shutdown and crash recovery; replacement record survives. Prior secret/code/session cannot authenticate after restart. Source: `TestRemoveIfOwnerLeavesAnotherRuntimesRecord` does not test the race |
| Safe discovery and compatibility | Dead/unresponsive endpoint, timeout/cancellation, malformed/non-loopback endpoint, foreign redirect, misleading 200, protocol mismatch and valid current health; no secret sent to an unverified endpoint. Source: `TestReadAndLive`; #106 supplies compatibility |
| Launch/session | Authorized issuance, unauthorized issuance rejected, successful launch and code-free redirect, 60-second boundary, replay, concurrent exchange, invalid code, missing/expired session, cookie attributes and restart invalidation. Fake clock for deterministic expiry; native browser run separately |
| Browser trust boundary | Allowed/foreign/malformed Host including wrong port, IPv4/IPv6 loopback, foreign/missing Origin, CORS preflight, absent/wrong CSRF, cookie-authenticated mutations and WebSocket upgrade; rejection causes no mutation. CLI/Desktop secret path remains distinct; Event-stream query token is rejected |
| Leak prevention and composition | Capture HTTP headers/bodies/redirects, structured logs and test diagnostics for success/failure/replay; no admin secret, session credential or launch code outside the narrowly required launch exchange. Contract fixtures and SDK against running server; two Projects remain isolated; run Linux launch in a real browser before claiming browser validation |

## Readiness criteria

| # | Criterion | State and evidence |
|---|---|---|
| 1 | Exact component and inspected commit | **Met:** full SHA and two component paths above; blob IDs in the evidence record |
| 2 | Refreshed inventory and actual tests | **Met for predecessor evidence:** four files, five passing tests; source limitations explicitly recorded |
| 3 | Accepted target and boundaries | **Met:** ADR-0003/0004, ADR-0025/0028/0034, ADR-0007/0032 and ADR-0059; no new browser auth model is selected here |
| 4 | Existing contracts and fixtures | **Met for identifying the target:** linked schemas, fixtures, OpenAPI/SDK/routeguard. Token-only Origin metadata and absent auth wire details require review in #92; this is not running-server evidence |
| 5 | Self-contained Amaru behavior tests | **Unmet in implementation; plan supplied:** table above. Must pass before implementation acceptance |
| 6 | Real validation | **Unmet:** no Amaru discovery/browser/security run. Linux required for M3; native macOS/Windows and Windows ACL not validated. Owner must accept this evidence gap for the migration decision |
| 7 | Licensing and provenance | **Met for decision basis:** [ADR-0026](0026-fair-code-community-license-with-commercial-licensing.md); implementation records file/blob provenance. No license text changes |
| 8 | Owner approval | **Unmet:** component, exact commit, selected Adapt disposition, fresh-import history and evidence gaps await explicit approval |

## Consequences

The small random-token and file-publication helpers can inform compact deployment without importing obsolete browser authentication. Most browser security and discovery failure verification remains new Amaru work. Five passing source tests do not complete #92, #18 or M3. No dependency is introduced; any future dependency needs its own verified justification and pinned version.

Architecture review verdict: **needs human decision**, with implementation requirements stated above. Domain purity, client/API separation and deployment boundaries remain intact. Same-OS-user processes remain inside the accepted compact trust boundary; this does not provide isolation from a hostile process running as that user. Revisit if the source revision, import scope, discovery ownership model or accepted browser contract changes.

## Alternatives considered

- **Import both packages unchanged:** keeps unverified path permissions, Windows ACL assumptions, cleanup race, permissive discovery probes and credential-printing tests.
- **Write every helper afresh:** viable where safe behavior diverges, but discards small useful standard-library primitives and file lifecycle tests. Adapt selectively; do not preserve code for its own sake.
- **Reuse bearer/query-token middleware:** violates ADR-0034 and exposes credentials to browser URLs; excluded by ADR-0059.

## Approval

Proposed by: Codex on 2026-11-12 for #84. Approved by: pending. No approval is inferred from the request to continue or from acceptance of ADR-0059. Readiness checks 5, 6 and 8 remain unmet.
