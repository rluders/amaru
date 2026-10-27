---
title: Adapt the prototype protocol into packages/contracts by fresh import with provenance
status: accepted
date: 2026-10-26
deciders: Ricardo Lüders (product owner)
---

# 0047. Adapt the prototype protocol into packages/contracts by fresh import with provenance

## Context
- [ADR-0007](0007-reuse-of-earlier-internal-code-is-a-separate-activity.md) bars moving code from the earlier internal implementation until a human approves a migration ADR for that component. This is the migration ADR for its `protocol/` directory (`MADR-protocol`, issue #67, sub-issue of WP-M2-01 / #9). It is the first migration ADR; the delivery guide gates every issue that moves this component on it ([delivery §8](../../engineering/delivery.md#8-human-approval-gates)).
- **Source (verified 2026-10-26):** the `protocol/` directory of the earlier internal implementation's Runtime repository (named in the private migration notes, which are not part of this repository), at the baseline commit **`c8a2f6e`** pinned by [ADR-0032](0032-migration-baseline-is-a-pinned-commit.md). The commit exists on the baseline branch; the directory holds `openapi.yaml` (56 paths), `schema.json`, `events.schema.json`, `provider.schema.json` and 271 fixtures (`fixtures/types` 211, `fixtures/events` 46, `fixtures/provider` 14). Run from a `git archive` of that commit, its Go fixture test (`runtime/internal/protocol`) passes (`go test -count=1`, 2026-10-26). The inventory records the same shapes as validated by one fixture set in Go and TypeScript.
- **Planned disposition** ([migration plan](../../engineering/delivery.md#1-sources-and-planning-ids), `MADR-protocol`): **Adapt** into `packages/contracts` in M2, under [ADR-0025](0025-api-and-protocol-versioning-policy.md) and [ADR-0036](0036-approvals-bind-to-evidence-and-gate-work.md)/[ADR-0037](0037-one-policy-evaluates-capabilities-grants-and-approvals.md), with Apache-2.0 for the package ([ADR-0026](0026-fair-code-community-license-with-commercial-licensing.md)).
- **Known drift in the source (recorded, not fixed there):** `GET /work-packages/{id}/autoloop` is registered by the Runtime router but absent from `openapi.yaml`. "Autoloop" is the *Delivery loop* in the [glossary](../../product/GLOSSARY.md#2-term-mapping-from-predecessor-repositories); the canonical Amaru route is decided in API-001 (#68), not here.
- **Assumption, unverified:** that the 271 fixtures cover every schema branch; the prototype never measured it. API-002 adds the coverage check.

## Decision
1. **Component and source.** Migrate `protocol/` of the earlier internal implementation at commit `c8a2f6e` (ADR-0032). No later commit is used.
2. **Disposition: Adapt.** The prototype protocol is a behavioral reference and migration source, not a copy-through. `packages/contracts` is canonical from the first commit; the prototype directory is not kept in sync afterwards (fixes go to Amaru).
3. **History option: fresh import with provenance** (the default of the migration plan §4). No `git subtree`/`filter-repo`. The source is a single sole-copyright-holder directory of data files; blame adds nothing for review. Provenance: the source commit is recorded in this ADR, in `packages/contracts/README.md`, and in a `PROVENANCE.md` listing each transferred file with its source path and blob id. The source commit is tagged `migration-protocol` in the source repository only with the owner's separate approval there (this ADR does not change the source repository, ADR-0007 rule 1).
4. **File disposition.**

   | Source | Disposition | Target / note |
   |---|---|---|
   | `openapi.yaml` | Adapt | `packages/contracts/openapi/openapi.yaml`, major namespace `/v1`; glossary renames; add the delivery-loop route and the Amaru-only resources; drop prototype-only paths (Obsidian vault/adoption, per-vault discovery) |
   | `schema.json` | Adapt | split into per-type JSON Schemas under `packages/contracts/schemas/`; Draft 2020-12 |
   | `events.schema.json` | Adapt | change events (WRK-003): add event id, cursor, resource, correlation, resync signal |
   | `provider.schema.json` | Adapt | external Provider protocol `amaru.provider/v1` (ADR-0025 naming) |
   | `fixtures/{types,events,provider}/*` | Adapt | one shared corpus, re-validated against the adapted schemas; fixtures of removed shapes are dropped, not edited to pass |
   | `runtime/internal/protocol/` (Go fixture test and `protocoltest`) | Adapt (behavior) | rewritten as the Go consumer of the shared corpus; not copied |
   | Plugin TypeScript fixture consumer | Reimplement | new TypeScript consumer ([ADR-0031](0031-obsidian-plugin-is-retired.md): plugin code is not migrated) |
   | Source ADRs, dogfood records | Reference only | cited, not copied |

   Added by Amaru, not migrated: Approval/Policy shapes, the Worker protocol (`amaru.worker.v1` Protobuf), usage fields ([ADR-0033](0033-usage-and-cost-telemetry-model.md)). Identity endpoints (M7) and the plugin manifest (M6) stay out.
5. **Renames.** Identifiers follow the glossary term map: Autoloop → Delivery loop; Playbook (workflow sense) → Workflow template; `weave.*` namespaces → `amaru.*` (e.g. `amaru.provider/v1`, `amaru.worker.v1`); Workspace is not used as a domain term; Execution/Step Execution as mapped. No `weave`/`orkestra` string remains in `packages/contracts` except in `PROVENANCE.md`.
6. **Behavior-preserving tests that move.** The 271 fixtures' valid/invalid outcomes, as one corpus with an expected-outcome manifest, validated by both a Go and a TypeScript test in `make check` (API-002, #117). Fixtures whose behavior intentionally changes are listed in `PROVENANCE.md` with the reason. Tests use fakes; no real agents or paid APIs.
7. **Gate.** No file from the source is added to this repository until this ADR is `accepted` with a recorded approval. Only then do #68 and #117 start. Merging the draft PR is not acceptance; issue #67 stays open until the decision.

## Readiness criteria (migration plan §5)
| # | Criterion | State | Evidence |
|---|---|---|---|
| 1 | Source side stable at `c8a2f6e` | **Met** | ADR-0032; commit verified; no later commit used |
| 2 | Inventory refreshed with tests run | **Met** | WP-M1-07 (#8) closed; the protocol Go test re-run at `c8a2f6e` on 2026-10-26 (see Context) |
| 3 | Target ADRs accepted | **Met** | ADR-0025, 0026, 0029, 0033, 0036–0039, 0042, 0044 accepted. Open product-owner questions Q56–Q58 do **not** block this migration; they block parts of the Approval/Policy contract, which are added by Amaru, not migrated |
| 4 | Contracts stable, with fixtures, in Amaru | **Not met, by design** | This migration creates them (WP-M2-01); the versioning rules and drafts exist (WP-M1-03/04/06) |
| 5 | Behavior covered by tests that run in Amaru | **Met by plan** | Item 6 above; becomes evidence in API-002 |
| 6 | Verified execution | **Gap, listed** | Schemas are data; the prototype validated them by fixtures, not by a recorded click-through. The delivery-loop route has no recorded run. Accepting this ADR accepts that gap |
| 7 | License resolved | **Met** | The source is MIT, © Ricardo Lüders, and the holder consents to relicensing (ADR-0026, migration plan §4). The package `LICENSE` (Apache-2.0) is added only by the product owner and gates #9, not the migration |
| 8 | Explicit product-owner approval | **Met** | Accepted by the product owner on 2026-10-26 (see Approval) |

## Consequences
- **Easier:** one reviewed, versioned contract with a single fixture corpus before the Platform exists; later migrations (httpapi, SDK) depend on it.
- **Security and compatibility:** fixtures carry no secrets or personal data (scanned in the PR). Amaru does not promise wire compatibility with the prototype; clients of the prototype are not supported ([ADR-0031](0031-obsidian-plugin-is-retired.md)). `/v1` is the first Amaru major, so there is no `/v0` compatibility burden.
- **Generated files:** none are migrated. Anything generated (TypeScript types, Go bindings) is produced by a committed command with a staleness check, never hand-edited.
- **Maintenance:** `packages/contracts` has a single owner (the Platform contract owner); the prototype copy is frozen and is not a second source of truth. Drift between OpenAPI and Protobuf concepts is checked in API-001/API-002.
- **Follow-up:** after acceptance, update the migration inventory in the private notes and mark `protocol/` migrated (separate approval in the source repository). Revisit if a later source commit changes `protocol/` before the migration lands.

## Alternatives considered
- **History-preserving import:** keeps blame, but the directory is data authored by one holder; it enlarges the repository for no review benefit.
- **Reimplement from the documents only:** loses 271 fixtures of validated behavior.
- **Keep the prototype `protocol/` as the contract and generate Amaru types from it:** keeps prototype names and the known drift, and makes a frozen repository the authority.

## Approval
Proposed by: Claude (migration ADR draft for #67, 2026-10-26). Approved by: Ricardo Lüders (product owner) on 2026-10-26, in the WP-M2-01 session after reviewing PR #249 ("ADR-0047 accepted"; the source repository stays unnamed in public docs). Criterion 6 gap accepted with it.
