---
title: Pre-release clone binding needs no legacy migration path
author: Ricardo
date: 2026-11-13
branch: feat/191-clone-binding
id: l-b397132b
scope: repo
status: active
src: agent
touches:
  - docs/architecture/adr/0066-platform-owned-active-project-clone.md
  - docs/engineering/project-access.md
  - docs/validation/m3/2026-11-12-linux-prj-002-clone-binding.md
  - services/platform/internal/ports/porttest/project_clone.go
  - services/platform/internal/ports/project_clone.go
  - services/platform/internal/store/migrations/project_clone_pg_test.go
  - services/platform/internal/store/migrations/sql/20261027120000_projects.sql
  - services/platform/internal/store/migrations/sql/20261112233000_project_clones.sql
  - services/platform/internal/store/migrations/sql/migrations.sum
---

## Decided
The owner clarified on PR #349 that Amaru has not been released and legacy compatibility is unnecessary. For this clone-binding change, put project_clones in the initial Project schema (services/platform/internal/store/migrations/sql/20261027120000_projects.sql), regenerate migrations.sum, and test fresh databases. ADR-0066 records the explicit clarification waiving its upgrade/reversible-migration and old-unbound-row requirements.

## Tried and ruled out
A separate clone migration with inverse SQL and an old-schema upgrade fixture added compatibility work for a release that does not exist. Remove that path and its tests; do not carry it as an acceptance gate. Current Project-only callers remain because current internal composition uses them, not as a legacy guarantee.

## Watch out
This applies before release and does not authorize resetting anyone's existing development database. Atomic rollback, normal reattachment preserving metadata/settings, missing-binding errors, current authentication and immutable Execution provenance remain required. Reassess migration compatibility when an actual release or supported deployed data exists.
