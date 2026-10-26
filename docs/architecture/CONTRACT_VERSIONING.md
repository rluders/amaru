# Contract versioning rules

**Status: Proposed.** Design for WP-M1-03 ([#4](https://github.com/rluders/amaru/issues/4)). Nothing here is implemented: no `packages/contracts` directory, generator, `buf` configuration or fixture exists yet ([repository layout](../engineering/repository-layout.md)). This document applies [ADR-0025](adr/0025-api-and-protocol-versioning-policy.md) (accepted) and does not change it. Every code block is an illustrative draft, not a file in the repository.

## Scope
In: how ADR-0025 is made checkable through shared fixtures, `buf breaking`, deprecation headers, version numbers and TypeScript generation. Out: implementing any of it (M2, [WP-M2-01](WORK_PACKAGES.md#wp-m2-01-contracts-package-from-the-prototype-protocol) [#9](https://github.com/rluders/amaru/issues/9), [WP-M2-02](WORK_PACKAGES.md#wp-m2-02-generated-typescript-sdk) [#10](https://github.com/rluders/amaru/issues/10), [WP-M2-03](WORK_PACKAGES.md#wp-m2-03-contract-drift-guard) [#11](https://github.com/rluders/amaru/issues/11)); changing the ADR-0025 rules; the release process.

## Rule recap (from ADR-0025)
- Within a major, changes are additive only; a breaking change creates a new major. Public HTTP uses path `/v1`; other contracts use `amaru.<kind>/v<major>`; Protobuf packages use `amaru.<kind>.v<major>`.
- The previous major stays supported at least 12 months after the next ships.
- Deprecations use `Deprecation` and `Sunset` headers plus the changelog.

## 1. Fixture conventions for additive changes
Fixtures are shared by all consumers (Go and TypeScript) ([ADR-0004](adr/0004-shared-versioned-contracts.md)). Proposed layout and rules, to be created with the package at M2:

| Rule | Why |
|---|---|
| One fixture directory per contract major: `fixtures/<kind>/v<major>/`. | A major is a compatibility line. |
| A fixture is never edited once released, except to fix a documented error. A new field adds a new fixture file; old files stay. | Old fixtures prove old payloads still parse. |
| `valid/` fixtures must parse in every consumer. `invalid/` fixtures must be rejected, each with the expected error code. | One corpus, same verdict everywhere. |
| Every released fixture is replayed against the current schema of the same major. | This is the additive-only check. |
| `forward/` fixtures carry unknown optional fields and unknown enum values; consumers must accept them. | ADR-0025 requires clients to tolerate new enum values and optional fields. |
| Removing or renaming a field, tightening a type, making an optional field required or removing an enum value fails the replay. It requires a new major and a new fixture directory. | Breaking changes are visible in review. |
| Fixtures contain no secrets or real tokens. | AGENTS rule 7. |

Additive change checklist for a PR: new fixture added; old fixtures untouched; replay passes in Go and TypeScript; changelog entry written.

## 2. `buf breaking` for Protobuf packages
Applies only to contracts that are Protobuf ([ADR-0025](adr/0025-api-and-protocol-versioning-policy.md): packages `amaru.<kind>.v<major>`). Per the buf documentation, `buf breaking` compares the current schema with a past version and reports breaking changes; its rule categories, strictest to most lenient, are `FILE`, `PACKAGE`, `WIRE_JSON` and `WIRE`, with `FILE` the default ([buf docs](https://buf.build/docs/breaking/), [rules](https://buf.build/docs/breaking/rules/), read 2026-10-26).

Proposal: use `FILE`, because generated Go and TypeScript code are consumers; relax only by ADR. Draft, not created:

```yaml
# DRAFT, not in the repository. Key names unverified; check the buf configuration reference at M2.
version: v2
breaking:
  use:
    - FILE
```

```text
# DRAFT CI step (unverified syntax of the --against reference; confirm at M2)
buf breaking --against '<last released contracts>'
```

The comparison baseline is the last released contracts, as ADR-0025 states. How that baseline is stored (git tag, archive or registry) is unverified and is decided in WP-M2-01. Before the first release there is no baseline, so the gate starts at the first tagged contracts release.

## 3. Deprecation headers
Proposal, aligned with ADR-0025:
- A deprecated endpoint or field keeps working within its major. Responses from a deprecated endpoint carry `Deprecation` and, once a removal date is set, `Sunset`, and the changelog and OpenAPI description name the replacement.
- The `Sunset` date is at least 12 months after the next major ships.
- Header value formats (RFC 9745 for `Deprecation`, RFC 8594 for `Sunset`) are cited from memory and are **unverified**; confirm against the RFCs when WP-M2-01 specifies the exact values.
- SDKs announce deprecations one minor ahead (ADR-0025).
- A fixture for a deprecated response must include the headers.

## 4. Version numbers
| Item | Rule | When |
|---|---|---|
| Contract package | Version `0.x` until the Local MVP release; `1.0.0` at M4. Within `0.x`, breaking changes are allowed but still need a changelog entry and a fixture change. The wire contract major is already `v1`, so `0.x` is a package maturity signal, not a wire version. | Package created at M2 ([ROADMAP](ROADMAP.md)). |
| URL major | `/v1` from M2. | M2 |
| SDK | semver; one SDK major targets one API major (ADR-0025). | M2 |

Open point for review: whether the `0.x` window permits wire-breaking changes while the URL already says `/v1`. This proposal says yes for the package only before any external consumer exists; ADR-0025 does not address it. Requires human confirmation.

## 5. TypeScript generation
Proposed toolchain, pinned at M2 (WP-M2-02): `openapi-typescript` generates types from the OpenAPI schema and `openapi-fetch` is the typed client.

Verified from the official docs (read 2026-10-26, [openapi-typescript](https://openapi-ts.dev/introduction), [openapi-fetch](https://openapi-ts.dev/openapi-fetch/)):
- Package names are `openapi-typescript` (dev dependency, run with `npx openapi-typescript <schema>`) and `openapi-fetch` (runtime, uses `createClient`).
- The documented install is `npm i openapi-fetch` and `npm i -D openapi-typescript typescript`.
- npm registry latest versions on 2026-10-26: `openapi-typescript` 7.13.0, `openapi-fetch` 0.17.0. Both are pre-1.0 for `openapi-fetch`, so pin exact versions.

Unverified: CLI flags needed for reproducible output, compatibility between the two packages' versions, and OpenAPI 3.1 feature coverage for Amaru's schemas. WP-M2-02 verifies these and records the pinned versions and the staleness check ([WP-M2-02](WORK_PACKAGES.md#wp-m2-02-generated-typescript-sdk)).

## 6. Where these conventions move at M2
When `packages/contracts` is created (WP-M2-01, with its Apache-2.0 `LICENSE` added by the product owner per [ADR-0026](adr/0026-fair-code-community-license-with-commercial-licensing.md)):
- Sections 1 to 4 move to `packages/contracts/README.md` (or a `docs/` file there) and become the package's own rules; this file becomes a short pointer and the rationale.
- `buf.yaml` is created beside the Protobuf sources only if Protobuf contracts exist then; otherwise section 2 stays proposed.
- Section 5 moves to the SDK package docs and the generator target in the Makefile ([repository bootstrap](../engineering/repository-layout.md)).
- The drift guard ([WP-M2-03](WORK_PACKAGES.md#wp-m2-03-contract-drift-guard), #11) enforces that routes match the OpenAPI contract.

## Evidence per acceptance criterion
Evidence levels per [validation](../engineering/validation.md#evidence-levels).

| Acceptance criterion (#4) | Level | Evidence |
|---|---|---|
| Conventions are in `packages/contracts` docs when that package is created (M2) | **Not validated** | Package does not exist; this document is a proposal only. |
| Generator pipeline is configured | **Not validated** | No generator, `buf` config or Makefile target exists. |
| Review against the prototype protocol history (verification) | **Not validated** | Not performed in this change; the prototype history is outside this repository. Pending human or follow-up review. |
| Docs build and links | Local check only | `make docs-check` and `make check` results are recorded in the PR; they do not validate the design. |

## Open decisions
- Whether `0.x` permits wire-breaking changes under `/v1` (section 4).
- Baseline storage for `buf breaking` (section 2).
- Whether any contract is Protobuf at M2; if not, section 2 stays dormant.
