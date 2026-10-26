---
title: Compact installs come in a bundled and a slim flavor on one PostgreSQL engine; heavy work runs on remote Workers
status: proposed
date: 2026-10-26
deciders:
---

# 0045. Compact installs come in a bundled and a slim flavor on one PostgreSQL engine; heavy work runs on remote Workers

## Context
- [ADR-0015](0015-postgresql-is-the-only-database-bundled-in-compact-installs.md) is accepted: PostgreSQL is the only engine, and compact installs bundle and supervise it. It already says Server and Cloud use an operator-provided PostgreSQL through a connection string.
- The M1 spike ([report](../investigations/2026-10-26-wp-m1-02-storage-and-execution.md)) validated the bundled PostgreSQL on Linux, macOS and Windows (hosted VMs): 9 of 9 scenarios and 86 of 86 crash cases on each, with a conditions list. It also measured the cost: installed bundle 128 MB on Linux, 136 MB on Windows and **270 MB on macOS** (94.6 MB compressed there). The NFR-34 budget (read as the compressed installer download, [report](../investigations/2026-10-26-wp-m1-02-storage-and-execution.md#environment)) leaves about 25 MB for the Platform binary on macOS; the spike binary alone is 30 MB. The bundle was not trimmed; no trimmed size is measured.
- The product owner's direction (2026-10-26, issue #3 session, delegated: "just pick up the best option"): PostgreSQL is required for the SaaS offering; nothing else is required to ship PostgreSQL if a lighter way matches the features; heavy processing may stay with the hosted service, or users may run parts of the platform as containers on another machine and use the web or desktop client against it.
- Already accepted and relevant: one Platform in several deployment models ([ADR-0002](0002-one-platform-multiple-deployment-models.md)), Desktop is a client ([ADR-0003](0003-desktop-is-a-client.md)), remote Workers register and connect outbound ([ADR-0024](0024-remote-worker-registration-identity-and-delivery.md)), and Server and Cloud run the same image in both roles ([DEPLOYMENT §4](../DEPLOYMENT.md#4-resolved-deployment-points)).
- Verified in the spike: the durable-execution design relies on PostgreSQL features (row locking with `SKIP LOCKED`, compare-and-set in transactions, a transactional outbox); [ADR-0022](0022-cloud-tenancy-row-level-isolation-by-organization.md) and [ADR-0030](0030-knowledge-is-lexically-indexed-project-documentation.md) rely on row-level security and full-text search. Not evaluated: PostgreSQL-in-WASM or any other embedded engine.

## Decision
- **The engine does not change.** PostgreSQL 16 or newer is the only database in every flavor and deployment ([ADR-0015](0015-postgresql-is-the-only-database-bundled-in-compact-installs.md)). No second engine or adapter is added.
- **Compact installs have two flavors built from the same Platform binary.** They differ only in whether the installer ships the PostgreSQL bundle and its supervisor:
  - **Bundled** (default for a single-user local install): ships the pinned PostgreSQL bundle and supervises it as ADR-0015 describes.
  - **Slim**: ships no PostgreSQL. The Platform requires a PostgreSQL connection string and refuses to start without one, with an error that names the missing setting. The database can be a container, a remote host or a managed service the user already operates.
- **An explicitly configured connection string always wins.** The Platform starts the bundled PostgreSQL only when the bundle is present and no connection string is configured.
- **Heavy or specialized processing runs on Workers, not in the compact Platform.** A user may run the Platform (and, separately, Workers) as containers on another machine and use Web or Desktop against it through the public API ([ADR-0003](0003-desktop-is-a-client.md), [ADR-0024](0024-remote-worker-registration-identity-and-delivery.md)). No Platform feature may depend on the bundled flavor.
- **Requirement scope** (proposal; the requirements file changes only after the owner accepts this ADR): NFR-34 ("no service the user installs or operates") and NFR-1 (no Docker) apply to the **bundled** flavor. The slim flavor is outside NFR-34 by design because the user provides PostgreSQL.
- **Not decided here:** which features, if any, are available only in the hosted service. That is a product and licensing question for the product owner ([ADR-0026](0026-fair-code-community-license-with-commercial-licensing.md)); a technical reason (a capability that needs compute the local machine lacks, delivered through a Worker) is not a feature gate.

## Consequences
- Easier: the macOS size problem no longer blocks the client and server story; users who already run PostgreSQL, or want containers, skip the bundle. No second dialect to test or migrate.
- Harder: two installer artifacts per OS to build, sign and document; the slim flavor needs clear connection and version checks and a documented minimum PostgreSQL version (16). Bundle precedence rules must be tested (bundle plus connection string, neither, connection string only).
- Required follow-up (not scheduled): measure a trimmed PostgreSQL bundle per OS (strip, drop unused components) and record installed and compressed sizes against NFR-34; re-run the crash matrix on the trimmed bundle.
- Revisit if a trimmed macOS bundle still exceeds the budget with the Platform binary, if bundled-PostgreSQL upgrades fail in a later validation (the spike did not test a minor-version upgrade), or if a second engine becomes necessary for a feature the owner wants to ship locally.

## Alternatives considered
- **Add a SQLite or file-store adapter for local installs:** rejected. It was rejected in ADR-0015 and the spike strengthens that: the durable-execution state machine, RLS and full-text search need PostgreSQL behavior, so local and Cloud would diverge and every test suite would double.
- **Ship only the bundled flavor:** rejected as the sole option because of the measured macOS size and because it forces a database on users and servers that already have one.
- **Ship only the slim flavor:** rejected as the sole option because the single-user local install then needs the user to run a database, which NFR-34 exists to prevent.
- **Installer-managed PostgreSQL service as a fallback:** kept as the existing fallback ([Q30](../OPEN_QUESTIONS.md#q30-fallback-if-bundled-postgresql-fails)); it needs admin consent and a superseding ADR, and the spike found no failure that triggers it.
- **PostgreSQL-in-WASM or another embedded engine:** not evaluated; would need its own spike and an ADR.

## Approval
Proposed by: an AI agent, delegated by the product owner in the issue #3 session on 2026-10-26 · Approved by: _(pending; an agent never accepts an ADR)_
