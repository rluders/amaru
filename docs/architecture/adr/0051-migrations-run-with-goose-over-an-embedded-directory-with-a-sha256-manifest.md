---
title: Migrations are goose-format files run with goose over an embedded directory with a sha256 manifest
status: accepted
date: 2026-10-27
deciders: Ricardo Lüders (product owner)
supersedes: 0023
---

# 0051. Migrations are goose-format files run with goose over an embedded directory with a sha256 manifest

## Context
- [ADR-0023](0023-self-hosted-oci-image-atlas-migrations-operator-owned-backup.md) chose Atlas Community Edition to author migrations into a goose-format directory with `atlas.sum`, applied by the Platform at start. It did not say whether the Platform links Atlas.
- **Verified 2026-10-27 while implementing FEAT-OPS-002 (#177):**
  - `ariga.io/atlas` v1.3.0 (Apache-2.0) works as a Go library (`sql/migrate` executor, `sqltool.GooseDir`, `atlas.sum` hashing), but its PostgreSQL driver imports `schemahcl`, which links `github.com/hashicorp/hcl/v2` v2.13.0 (**MPL-2.0**) into the Platform binary. The revisions table of Atlas lives in its CLI, so the Platform had to write its own.
  - `github.com/pressly/goose/v3` v3.29.0 (MIT) runs goose-format SQL files as a library, in one transaction per file with its version row. Its transitive modules are `mfridman/interpolate` (MIT), `sethvargo/go-retry` (Apache-2.0), `go.uber.org/multierr` (MIT) and `golang.org/x/sync` (BSD-3-Clause), read from each module's LICENSE file.
  - goose stores no file hash, so it cannot detect against a live database that an applied file was edited.
- The product owner decided on 2026-10-27 to use goose: "We are now using goose" (answering whether ADR-0023 should be amended or superseded). Whether MPL-2.0 would be acceptable under [ADR-0026](0026-fair-code-community-license-with-commercial-licensing.md) was not decided and no longer matters here.
- A superseding ADR is used because the migration tooling is part of ADR-0023's substance; an accepted ADR is not edited in substance ([DECISIONS](../DECISIONS.md#convention)).

## Decision
- **Unchanged from ADR-0023:** one multi-arch OCI image with the Platform binary (Web assets included) whose `worker` subcommand runs a Worker; a reference `compose.yaml` runs Platform, PostgreSQL, NATS and RustFS; a Helm chart follows at M9. Upgrades are by image tag, from any release to the next minor, and across majors only from the latest minor; the Platform refuses to start against a schema newer than it knows. Backup and restore are operator responsibility; Amaru documents the data to back up and provides `amaru admin check` for version and integrity.
- **Migrations (replaces ADR-0023's Atlas rules):**
  - Migrations are goose-format SQL files `<YYYYMMDDHHMMSS>_<name>.sql` with an `-- +goose Up` section; RLS policies and functions are written by hand in them. Atlas is not a Platform dependency. How files are authored (by hand or with any tool, including the Atlas CLI) is not constrained; whether the Atlas CLI can author them without `atlas.sum` is unverified.
  - The directory is embedded in the binary with `migrations.sum`, a sha256 manifest of its files. The Platform refuses to run, and `go test` fails, if the directory does not match the manifest.
  - goose runs the files as a library, applied by the Platform at startup (or `amaru migrate`), each file in one transaction with its version row (table `amaru_schema_migrations`), under a PostgreSQL advisory lock, as the owner role.
  - Forward-only: a `Down` section is never run. Breaking schema changes use expand/contract across two releases so rolling upgrades keep working.
  - The Platform refuses a database that records a version it does not know (never a downgrade) and a pending file older than an applied one.
  - CI runs the unit tests and, where a server exists, the PostgreSQL-tagged tests against an empty and a seeded database.

## Consequences
- **Easier:** no MPL-2.0 module and no hand-written revisions table in the binary; one small, MIT runner; the same files, embed and start-up flow as before.
- **Harder:** an applied file edited after release is caught only at build time (`migrations.sum`), not against a live database; the Atlas lint and diff features are not part of the pipeline.
- **Follow-up:** implemented in `services/platform/internal/store/migrations` (#177). The M7 upgrade and restore drill of ADR-0023 still applies. Revisit if a run-time hash check becomes necessary (a small side table would add it).

## Alternatives considered
- **Atlas as a library (ADR-0023 read literally):** works, but links an MPL-2.0 module the owner had not accepted.
- **Atlas CLI at runtime:** an extra binary in compact installs (rejected in ADR-0023).
- **In-house runner reading the goose files:** removes every runner dependency and could keep a hash check, but is more code to own for no gain over goose.

## Approval
Proposed by: Claude (2026-10-27, during #177). Approved by: Ricardo Lüders (product owner) on 2026-10-27 in the WP-M3-02 session: "Yes. We are now using goose." The text above was written by an agent from that decision; the owner may edit it.
