---
title: PostgreSQL is the only database; compact installs bundle and supervise it
status: accepted
date: 2026-10-25
deciders: Ricardo Lüders
---

# 0015. PostgreSQL is the only database; compact installs bundle and supervise it

## Context
- Question: [Q3](../OPEN_QUESTIONS.md#q3-database-and-persistence-strategy). Product-owner answer: "Postgres." In this session the product owner chose a bundled PostgreSQL for compact installs over a second SQLite adapter.
- Earlier docs required compact installs to be one process with embedded storage and no Docker (NFR-1, NFR-34). A PostgreSQL server is not embeddable in-process; the intent of NFR-34 is that the user installs and operates nothing extra.
- Evidence:
  - The earlier prototype already uses PostgreSQL only, with 30 migrations, and refuses SQLite outside tests.
  - `fergusstrange/embedded-postgres` v2.0.1 (MIT, 2026-10-19) runs real PostgreSQL 15–18 binaries as a supervised child, with native Windows amd64 CI; by default it downloads binaries on first start (verified https://github.com/fergusstrange/embedded-postgres, 2026-10-25).
  - Row-level security ([ADR-0022](0022-cloud-tenancy-row-level-isolation-by-organization.md)), full-text search ([ADR-0030](0030-knowledge-is-lexically-indexed-project-documentation.md)) and `SKIP LOCKED` timers ([ADR-0014](0014-durable-execution-on-own-state-machine-over-postgres.md)) all need PostgreSQL features.

## Decision
- **PostgreSQL is the only database engine** in every deployment. There is no SQLite or file-store adapter. Minimum version 16; development and tests target 18.
- **Compact (local) installs**: the installer ships pinned, checksummed PostgreSQL binaries for the target OS. The Platform starts PostgreSQL as a supervised child process on first run (`initdb` into the per-user state directory), listens on a Unix socket or a random loopback port with a generated password, stops it on shutdown, and kills it if the Platform dies. Nothing is downloaded at runtime.
- **Server and Cloud**: the operator provides PostgreSQL (or a managed service) via a connection string.
- Schema changes use versioned, forward-only migrations ([ADR-0023](0023-self-hosted-oci-image-atlas-migrations-operator-owned-backup.md)).
- The supervising library (embedded-postgres or a small in-house supervisor) is chosen in the M1 spike; the decision above does not depend on it.
- NFR-34 is restated: compact installs need **no service the user installs or operates**; bundled, supervised child processes are allowed.

## Consequences
- One schema, one dialect, one migration set; queries, RLS and FTS behave identically locally and in Cloud.
- Compact installers grow by roughly 30–60 MB per OS (to be measured) and run a second process.
- Backup of a compact install is "stop Amaru, copy the data directory" or `pg_dump` through `amaru backup`.
- Validation (M1): bundled PostgreSQL starts, survives Platform crash/restart and upgrades its minor version on Windows (incl. non-admin user and antivirus), macOS (signed/notarized child binary) and Linux.
- Revisit if: the Windows or macOS spike cannot run a bundled server without admin rights or notarization blockers.

## Alternatives considered
- **PostgreSQL for server/Cloud plus SQLite for compact**: true single process, but two adapters, two migration sets and dialect drift forever; RLS and FTS differ.
- **Require users to install PostgreSQL or Docker locally**: worst onboarding; contradicts NFR-1.
- **Files (the prototype)**: no transactions, no multi-user, no migrations (GAP-07).

## Approval
Proposed by: Claude (open-question resolution, 2026-10-25) · Approved by: Ricardo Lüders on 2026-10-25, product-owner planning session (answer recorded in OPEN_QUESTIONS Q3; bundled compact option chosen in the same session).

## Notes
Cross-references added 2026-10-25. They leave the decision above unchanged:
- The supervisor library is `embedded-postgres` v2 ([Q28](../OPEN_QUESTIONS.md#q28-postgresql-supervisor-library)).
- The listener is a Unix socket on Linux/macOS and a loopback port on Windows ([Q29](../OPEN_QUESTIONS.md#q29-compact-postgresql-listener)).
- The fallback is an installer-managed service ([Q30](../OPEN_QUESTIONS.md#q30-fallback-if-bundled-postgresql-fails)).
- The installer budget is ≤ 120 MB (validation A15).
