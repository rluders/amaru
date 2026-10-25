---
title: Self-hosted ships as an OCI image with Atlas-authored migrations and operator-owned backup
status: accepted
date: 2026-10-25
deciders: Ricardo Lüders
---

# 0023. Self-hosted ships as an OCI image with Atlas-authored migrations and operator-owned backup

## Context
- Question: [Q9](../OPEN_QUESTIONS.md#q9-self-hosted-installation-upgrades-backup-and-recovery). Product-owner answer: "Container image. We use a database migration tool (Atlas), and the rest is responsibility of the person that handles the self-host."
- Verified 2026-10-25 (https://atlasgo.io/community-edition): Atlas Community Edition is Apache-2.0 and supports versioned `migrate diff/apply/status` for PostgreSQL; `migrate lint`, `down`, `test` and schema support for RLS, functions, triggers and extensions are not in the Community Edition. goose (MIT, v3.29.0) applies versioned SQL migrations as a Go library.
- Compact installs are one binary with a bundled PostgreSQL ([ADR-0015](0015-postgresql-is-the-only-database-bundled-in-compact-installs.md)); they cannot depend on an external CLI to migrate.

## Decision
- **Packaging**: one multi-arch OCI image containing the Platform binary (with Web assets) whose `worker` subcommand runs a Worker. A reference `compose.yaml` runs Platform, PostgreSQL, NATS and RustFS. A Helm chart follows at M9.
- **Migrations**:
  - Authored with **Atlas Community Edition** (`atlas migrate diff`) into a versioned SQL directory in goose format with `atlas.sum` integrity; RLS policies and functions are written by hand in those SQL files.
  - The directory is embedded in the binary and applied by the Platform at startup (or `amaru migrate`), under an advisory lock, as the owner role.
  - Forward-only. Breaking schema changes use expand/contract across two releases so rolling upgrades keep working.
  - CI checks the directory hash and runs every migration against an empty and a seeded database.
- **Upgrades**: operators upgrade by image tag; supported path is any release to the next minor, and across majors only from the latest minor. The Platform refuses to start against a schema newer than it knows.
- **Backup and restore are operator responsibility**: Amaru documents the data to back up (PostgreSQL, artifact store, secrets KMS key, configuration) and provides `amaru admin check` for version and integrity; it does not ship a backup system.

## Consequences
- One artifact for self-hosted and Cloud; compact installs use the same migrations.
- Atlas Pro features (lint) are not required; a lint step can be added later without changing the format.
- Validation (M7): upgrade across two minor releases and a documented restore drill on the reference compose deployment.

## Alternatives considered
- **Single static binary only**: fine for compact, but operators expect an image.
- **Helm-first**: premature before Cloud.
- **Atlas CLI at runtime**: an extra binary in compact installs.

## Approval
Proposed by: Claude (open-question resolution, 2026-10-25) · Approved by: Ricardo Lüders on 2026-10-25, product-owner planning session (answer recorded in OPEN_QUESTIONS Q9).
