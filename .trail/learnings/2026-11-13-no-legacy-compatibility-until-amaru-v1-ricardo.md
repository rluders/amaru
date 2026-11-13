---
title: No legacy compatibility until Amaru v1.0
author: Ricardo
date: 2026-11-13
branch: feat/191-clone-binding
id: l-260c250c
scope: repo
status: active
replaces: l-b397132b
src: agent
touches:
  - AGENTS.md
  - docs/architecture/adr/0066-platform-owned-active-project-clone.md
  - docs/engineering/CONVENTIONS.md
  - docs/engineering/project-access.md
  - docs/validation/m3/2026-11-12-linux-prj-002-clone-binding.md
  - services/platform/internal/ports/porttest/project_clone.go
  - services/platform/internal/ports/project_clone.go
  - services/platform/internal/store/migrations/project_clone_pg_test.go
  - services/platform/internal/store/migrations/sql/20261027120000_projects.sql
  - services/platform/internal/store/migrations/sql/20261112233000_project_clones.sql
---

## Decided
The owner made the no-legacy rule repository-wide until Amaru v1.0 is released, not merely until a first pre-release. AGENTS.md hard rule 10 and docs/engineering/CONVENTIONS.md#versioning are authoritative. Update current code, schemas, contracts, fixtures and consumers directly; test fresh installations. Keep shared contract definitions/version identifiers, but do not add old-version compatibility layers, deprecated aliases, old-format readers, backfills or upgrade/downgrade paths for unreleased versions.

## Tried and ruled out
PR #349 originally added a separate clone migration, inverse SQL and old-schema upgrade test despite the unreleased product. The revised PR puts project_clones in the initial Project schema and removes that compatibility path. Limiting this to only clone bindings or ending the rule at a pre-v1.0 release misses the owner's explicit v1.0 cutoff.

## Watch out
Do not reset or delete existing development data to enforce fresh-install acceptance. Atomic registration, normal reattachment/idempotence, runtime recovery, authorization and immutable Execution provenance still matter. Compatibility obligations in the accepted versioning ADRs apply from the Amaru v1.0 release. This is about the product release, not the existing /v1 API prefix or schema version identifiers.
