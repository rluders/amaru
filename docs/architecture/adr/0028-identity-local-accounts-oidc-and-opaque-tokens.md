---
title: Identity uses local accounts and OIDC, server-side sessions and revocable opaque tokens
status: accepted
date: 2026-10-25
deciders: Ricardo Lüders
---

# 0028. Identity uses local accounts and OIDC, server-side sessions and revocable opaque tokens

## Context
- Question: [Q15](../OPEN_QUESTIONS.md#q15-identity-and-authentication-model). Product-owner answer: "Pick the best approach."
- earlier prototype `auth/auth.go`: bcrypt passwords seeded equal to the username, HS256 JWT (12 h) with no refresh or revocation — a counter-example.
- Verified 2026-10-25: `github.com/coreos/go-oidc/v3` is Apache-2.0, v3.21.0, maintained.
- Approvals and audit need attributable identities (SEC-6); self-hosters often have an IdP; Cloud needs tenant-scoped identity ([ADR-0022](0022-cloud-tenancy-row-level-isolation-by-organization.md)).

## Decision
- **Human identities**:
  - Compact: one owner account created at first run; local login via the launch flow ([ADR-0034](0034-local-web-client-authentication-with-launch-code-and-cookie.md)).
  - Self-hosted: local accounts (argon2id password hashes, optional TOTP) **and** OIDC login (go-oidc + `golang.org/x/oauth2`); admins may disable local accounts.
  - Cloud: OIDC only (vendor IdP plus customer IdP federation per Organization).
- **Sessions**:
  - Browser: server-side session stored in PostgreSQL, HttpOnly Secure SameSite=Lax cookie (Strict on loopback), idle and absolute timeouts, CSRF tokens on state-changing requests.
  - Desktop and CLI: OAuth 2.0 authorization code with PKCE on a loopback redirect, or device authorization grant on headless machines; each Instance's refresh token is stored in the OS keychain. Desktop holds one credential per Instance.
- **Machine identities**: Workers use enrolled Ed25519 keys ([ADR-0024](0024-remote-worker-registration-identity-and-delivery.md)); service accounts (CI, service plugins, integrations) use **opaque, scoped, expiring API tokens** stored hashed and revocable.
- **No long-lived bearer JWTs**; any short-lived signed token the Platform issues is backed by a revocable server-side record.
- Every principal has an id recorded in Events, Approvals and Grants.

## Consequences
- Immediate revocation and auditable sessions; standard IdP integration.
- More server state than stateless JWTs; acceptable at Amaru's scale.
- Validation (M7): OIDC with two IdPs, session revocation, keychain storage on three OSes.

## Alternatives considered
- **OIDC only everywhere**: blocks small self-hosted installs without an IdP.
- **Stateless JWT sessions**: no revocation (earlier prototype).
- **Embedding an IdP (Dex/Keycloak)**: another service to operate.

## Approval
Proposed by: Claude (open-question resolution, 2026-10-25) · Approved by: Ricardo Lüders on 2026-10-25, product-owner planning session (decision delegated to Claude in OPEN_QUESTIONS Q15 and accepted in the same session).
