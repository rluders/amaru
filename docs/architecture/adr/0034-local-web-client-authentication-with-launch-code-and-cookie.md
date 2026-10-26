---
title: The local Web client authenticates with a one-time launch code and a loopback session cookie
status: accepted
date: 2026-10-25
deciders: Ricardo Lüders
---

# 0034. The local Web client authenticates with a one-time launch code and a loopback session cookie

## Context
- Question: [Q21](../OPEN_QUESTIONS.md#q21-local-web-client-authentication). Product-owner answer: "You can decide."
- The prototype binds `127.0.0.1:0`, writes a per-start bearer token to a 0600 `runtime.json`, allows only `Origin: app://obsidian.md`, accepts `?token=` on the WebSocket, and has **no Host-header check** (DNS rebinding relies on the token). A browser tab cannot read a 0600 file.
- Threats: other local web pages (CSRF, DNS rebinding), other OS users on the same machine, token leakage through URLs or logs.

## Decision
- The Platform binds **loopback only** (`127.0.0.1` and `::1`), one Instance per OS user, state file with the endpoint and an admin secret readable only by that user (0600; Windows ACL restricted to the user SID).
- **Launch flow**: `amaru open` (or Platform start) reads the state file, asks the Platform for a **one-time launch code** (≥256 bits, single use, 60-second TTL) and opens `http://127.0.0.1:<port>/auth/launch?code=…`. The Platform exchanges the code for a **server-side session** and redirects to a URL without the code.
- Session cookie: `HttpOnly`, `SameSite=Strict`, `Path=/`, host-only; `Secure` where the browser allows it on loopback.
- **Host allowlist**: requests whose `Host` is not `127.0.0.1:<port>`, `[::1]:<port>` or `localhost:<port>` get 421/403 (DNS rebinding).
- **Origin checks**: state-changing requests and WebSocket upgrades must carry an allowed `Origin`; plus a CSRF token on non-GET requests.
- No tokens in query strings after launch; the WebSocket uses the session cookie.
- CLI and Desktop authenticate with the state-file secret, never with the browser session.

## Consequences
- Other local pages cannot drive the API; other OS users cannot obtain a session without reading the user's files.
- Opening a bookmarked URL without a session shows "run `amaru open`".
- Validation (M4): security review and tests for rebinding, cross-origin requests, code replay; Windows ACL verified (prototype GAP-05).

## Alternatives considered
- **Token embedded in the served page**: any page that can load it gets the token.
- **Bearer token pasted by the user**: poor UX, ends up in storage.
- **OS-level mechanisms (Unix-socket auth)**: browsers cannot use them.

## Approval
Proposed by: Claude (open-question resolution, 2026-10-25) · Approved by: Ricardo Lüders on 2026-10-25, product-owner planning session (decision delegated to Claude in OPEN_QUESTIONS Q21 and accepted in the same session).
