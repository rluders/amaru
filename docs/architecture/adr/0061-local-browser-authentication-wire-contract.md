---
title: Define the compact browser authentication wire contract
status: proposed
date: 2026-11-12
deciders:
---

# 0061. Define the compact browser authentication wire contract

## Context

[ADR-0034](0034-local-web-client-authentication-with-launch-code-and-cookie.md) accepts single-use launch codes, server-side cookie sessions, Host/Origin checks and CSRF protection. [ADR-0060](0060-adapt-the-prototype-auth-and-discovery-for-compact-installs.md) accepts selective helper adaptation, but explicitly leaves issuance operations and CSRF wire details to contract review in [IAM-001 / #92](https://github.com/rluders/amaru/issues/92).

**Verified baseline:** #18 integration `9e368b468cb4933320ea51a16d5050c2c9742fb0`, including merged PR #325. The [OpenAPI contract](../../../packages/contracts/openapi/openapi.yaml) declares bearer authentication only, permits Event-stream query tokens and describes missing Origin as a token path. There are no auth operations or session schemas. The [SDK](../../../packages/sdk/README.md) checks compatibility before product requests but does not implement browser authentication. The [routeguard](../../../packages/contracts/routeguard/routeguard.go) applies the first top-level server base to every path; the accepted unversioned `/auth/launch` cannot be represented correctly by silently declaring it under `/v1`.

This proposal completes the compact authentication contract. It does not supersede ADR-0034 or authorize additional predecessor imports. **Nothing below is implemented or browser-validated.** Public authentication operations, credential lifetimes and the SDK browser transport are consequential security/contract choices requiring owner approval before implementation.

## Decision

Proposed for approval:

### Operations and credentials

| Operation | Authentication and result |
|---|---|
| `POST /v1/auth/launch-codes` | CLI/Desktop administrator bearer only, no Cookie or Origin. Empty body; `201` JSON `{code, expiresAt}`. Code has at least 256 random bits, expires exactly 60 seconds after issuance and is consumed atomically once. The CLI constructs the bound loopback `/auth/launch?code=…` URL; the administrator secret is never part of that URL. |
| `GET /auth/launch?code=…` | Narrow launch exception: exact bound Host, exactly one well-formed code, no administrator bearer required. Successful exchange creates a server-side session, sets the cookie and returns `303 Location: /`. Invalid, expired or consumed codes return the same generic `401` error without echoing the code. No open redirect parameter. |
| `GET /v1/auth/session` | Existing browser cookie only. `200` JSON `{csrfToken, expiresAt}` for that session; missing/expired session returns generic `401`. This operation never creates or extends a session. |
| `DELETE /v1/auth/session` | Browser cookie plus allowed Origin and `X-Amaru-CSRF`; revoke the session and expire the cookie, return `204`. |

Use `amaru_session` as a host-only, HttpOnly, SameSite=Strict, Path=/ cookie with an opaque random credential of at least 256 bits. Follow ADR-0034's Secure requirement; verify cookie delivery in the actual Linux browser before claiming browser support, rather than assuming that an HTTP loopback Secure cookie is accepted. Do not add a Domain attribute. Store sessions and launch codes only in Platform memory, independent of domain state, and rotate the administrator secret at restart. Restart invalidates all previous credentials.

A session has an **eight-hour absolute lifetime**, without sliding renewal. Cookie Max-Age and server expiry agree; expiry is enforced on every request. A new launch creates an independent session; logout revokes only that session. Generate a separate random CSRF token of at least 256 bits per session. Return it only through the authenticated session operation; clients retain it in memory and send it as `X-Amaru-CSRF` for non-GET browser operations. Never return the session credential in JSON. An authenticated WebSocket checks session expiry during its lifetime and closes when expired or revoked; an established socket cannot outlive its authorization.

Auth responses, including errors and redirects, use `Cache-Control: no-store` and `Referrer-Policy: no-referrer`. Launch responses contain no third-party content and immediately redirect to the fixed code-free root. Any unauthenticated bookmark page explains how to run `amaru open`; it contains no secret. Use the existing Error envelope/codes: `unauthorized` (401), `forbidden` or `forbidden_origin` (403), `invalid_request` (400), `rate_limited` (429), and `internal` (500). Messages never echo credentials or launch URLs.

### Authentication paths and request guards

- Bind loopback before publishing discovery. Validate the exact loopback Host and bound port for **all** requests, including health and auth. Reject malformed, foreign or wrong-port Host before dispatch; do not trust forwarded headers. The allowed names remain those of ADR-0034.
- Cookie authentication requires no Authorization header. Browser non-GET requests require an exact allowed loopback Origin and the matching session CSRF token; WebSocket upgrades require allowed Origin and a valid cookie. A missing, null, malformed or foreign Origin is rejected for these browser operations. Session reads expose no credential cross-origin: no wildcard CORS or foreign-origin responses. `GET /auth/launch` is the sole code-exchange exception, not a general missing-Origin bypass.
- CLI/Desktop use administrator bearer authentication with **no browser cookie or Origin**. Reject mixed credentials and an Origin-bearing bearer request instead of falling through to a privileged browser bypass. The issuance endpoint uses only this path. Non-browser WebSockets may use an Authorization header without Origin; cookie WebSockets follow the browser guards.
- Reject `token` query parameters on Event streams; remove the SDK's browser query-token transport. Browser Event streams use the ambient same-origin session cookie. Keep the native/header transport distinct. Browser HTTP uses same-origin cookie credentials and the session CSRF header; bearer client behavior remains available for CLI/Desktop.
- Health remains unauthenticated, Host-checked and free of credentials. Compatibility probes remain credential-free. Authentication rejection dispatches no domain command. Launch/session operations do not change authoritative product state.

### Contract and implementation delivery

After approval, update OpenAPI, schemas, shared positive/negative fixtures, Go/TypeScript validation, routeguard, generated SDK types and SDK consumers together in #92. Define operation IDs `createLaunchCode`, `exchangeLaunchCode`, `getSession`, `deleteSession`, with schemas `LaunchCode` and `BrowserSession`. Add cookie/CSRF security metadata and replace token-only Origin/Event-stream wording. The unversioned launch operation uses a path-level server `/`; extend routeguard to honor path/operation server overrides and test the resulting absolute routes. Do not silently register an undocumented route or change the existing `/v1/health` path.

Place the compact HTTP/session and discovery adapters outside the pure domain. Keep ADR-0060's private discovery publication, permissions, ownership, restart, safe-probe and provenance requirements. No new dependency, broker or database is needed for session state. Validate before creating a record; entropy failure never publishes credentials. Bound in-memory code/session admission, expire unused entries and return 429 on exhaustion rather than evicting a live user's session. Log neither raw request targets nor Cookie, Authorization, CSRF headers or auth response bodies; #170 must capture these paths. #18 retains composed server acceptance.

## Consequences

Browser requests and native bearer requests have explicit, independently testable guards. The launch URL is transient; browser pages never receive the administrator secret. Restart and eight-hour expiry require `amaru open` again. Header-only native WebSocket callers need a transport capable of supplying Authorization; browser callers use cookies. Removing query tokens requires updating existing SDK callers/tests in the same implementation.

Implementation acceptance requires deterministic expiry/replay/concurrent-exchange tests, missing/foreign Origin and hostile Host tests, missing/wrong CSRF, mixed credentials, logout and restart invalidation, WebSocket expiry/revocation, entropy/admission failure, and zero command dispatch on rejection. Test the SDK against running loopback routes and compare registered routes with OpenAPI. Exercise exact 60-second/eight-hour boundaries with a fake clock. Capture redirects, response bodies and logs for leaks without printing test credentials. Native Linux discovery permissions and a real Linux browser launch remain required evidence for their respective claims; macOS/Windows remain best effort and Windows ACL behavior must not be inferred from Linux. No insecure permission fallback is allowed.

**Architecture review: sound as a proposal, pending owner decision.** Platform owns authentication and domain state; clients use public operations; OS/session adapters stay outside domain. There are no new dependencies or infrastructure. Partial code exchange consumes the code before exposing a session; an interrupted redirect requires a fresh launch. Duplicate exchange fails; restart discards auth state. Compatibility checks remain distinct from identity proof. Session expiry and rejected guards prevent stale authorization, and logs require explicit secret filtering. Revisit if remote identity, cross-origin UI hosting or persistent sessions are introduced; ADR-0028 governs other deployment identities.

## Alternatives considered

- Keep browser bearer/query tokens: violates ADR-0034 and exposes secrets through browser URLs.
- Put CSRF/session credentials in the launch redirect: creates another secret-bearing URL. A cookie-authenticated session read exposes only the CSRF value to the same-origin client.
- Use persistent or sliding sessions: adds storage/lifecycle policy and extends unattended authorization. The proposed fixed lifetime bounds exposure without new infrastructure.
- Implement unspecified routes first: creates a security contract without the owner review required by repository policy. Review this small wire decision before implementing #92.

## Approval

Proposed by: Codex on 2026-11-12 for #92. Awaiting product-owner approval of the operations, eight-hour absolute session lifetime, CSRF delivery and distinct browser/native transports. No approval recorded.
