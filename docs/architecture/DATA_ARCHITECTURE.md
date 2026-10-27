# Data Architecture

Status: **accepted** (2026-10-25), not implemented. Storage decisions: [ADR-0015](adr/0015-postgresql-is-the-only-database-bundled-in-compact-installs.md), [ADR-0016](adr/0016-nats-jetstream-message-bus-embedded-in-compact-installs.md), [ADR-0017](adr/0017-content-addressed-artifact-store-filesystem-or-s3.md), [ADR-0043](adr/0043-project-definitions-in-git-teams-and-operational-state-in-platform.md). This document sets out the data classes, the requirements of each class and where each class lives.

## 1. Data classes

| Class | Examples | Owner | Source of truth | Requirements |
|---|---|---|---|---|
| **Project knowledge** | Code, docs, ADRs, Skills, Playbooks, Artifact notes | User | The user's Git repositories | Never moved or rewritten by Amaru. Read by path + hash. |
| **Definitions** | Workflows, Agents, Team references (by slug), templates, check configs, Project configuration | User (edited via Amaru or Git) | Committed files under `.amaru/` ([ADR-0043](adr/0043-project-definitions-in-git-teams-and-operational-state-in-platform.md)) | Versioned by Git history; each Execution stores the commit SHA, content hash and an immutable snapshot of the definitions it used; diffable; concurrent edits detected (prototype F-7). |
| **Operational state** | Teams and memberships, Work Packages, Work Items, Executions, Step states, leases, delivery-loop ledgers, approvals pending, outbox | Platform | PostgreSQL | Transactional updates, crash-safe, queryable. Not committed to user repositories. |
| **History and audit** | Events, decisions, Grants, Approval decisions, snapshots of Agents/Skills/context | Platform | Event log + run history | Append-only, ordered by a per-Execution `seq` assigned by the Platform (never by Worker clocks), retained per §5, exportable. |
| **Logs and evidence** | Provider stdout/stderr, transcripts, CI log excerpts, check output | Platform | Run history / artifact store | Size-bounded, redacted, retention-limited (GAP-07). |
| **Artifacts** | Diffs, generated files, meshes, audio, reports | Platform (generated) / user (project files) | Artifact store; project tree for project files | Content-hashed, immutable once recorded, possibly large. |
| **Secrets** | API keys, tokens, webhook secrets, plugin secrets | User / Organization | Secrets backend | Write-only from clients; never in other classes. |
| **Configuration** | Instance, Organization, user and Project settings; provider and plugin config | Operator / user | Config files + Platform store | Scoped resolution order ([ADR-0039](adr/0039-configuration-scopes-and-resolution-order.md)); validated by schema. |
| **Deployment state** | Schema version, installed plugins, registered Workers | Platform | Platform store | Migrated on upgrade; included in backups. |

## 2. Current predecessors (verified)

- **The prototype Runtime** keeps everything in files; there is no database.
  - **Write mechanics:** files are written atomically with a temp file and rename.
  - **Locations:**
    - the per-project `work`, `workflows`, `agents` and `teams` folders: committed definitions.
    - the per-project runtime folder, with `executions/<id>/{execution.json,events.jsonl}` and the delivery-loop ledgers: gitignored operational state.
    - user state dir: credentials and grants.
  - **Gaps:** no schema migrations; work files are read only at startup.
- **PostgreSQL in a predecessor: unverified.** An earlier claim that a prototype used PostgreSQL with 30 migrations, custom runner and `ArtifactRoot` could not be confirmed. The Runtime at `c8a2f6e` is file-based, with no `database/sql` or `pgx` ([ADR-0050](adr/0050-reimplement-the-prototype-stores-as-postgresql-adapters.md)). Treat it as no evidence for or against the PostgreSQL decision.

## 3. Store roles (decided)

| Store | Role | Decision |
|---|---|---|
| **Files in the Project (`.amaru/`)** | Definitions reviewed in Git: Workflows, Agents, Team references, templates, Project configuration, Local Check definitions | [ADR-0043](adr/0043-project-definitions-in-git-teams-and-operational-state-in-platform.md); read from the working tree locally, fetched at a commit remotely |
| **PostgreSQL** | Operational state, Teams, Work Packages and Work Items, history and audit, Knowledge index (when FEAT-PRJ-007 ships), configuration above Project scope, deployment state | Only engine in every deployment; bundled and supervised in compact installs ([ADR-0015](adr/0015-postgresql-is-the-only-database-bundled-in-compact-installs.md)) |
| **NATS JetStream** | Assignment work queues and Event fan-out; never the source of truth | Embedded in compact, cluster for multi-instance; fed by the outbox ([ADR-0016](adr/0016-nats-jetstream-message-bus-embedded-in-compact-installs.md)) |
| **Artifact store** | Artifact blobs, large logs, remote Worker uploads; versions and history in PostgreSQL | Content-addressed; filesystem (compact) or S3-compatible with RustFS as reference ([ADR-0017](adr/0017-content-addressed-artifact-store-filesystem-or-s3.md)) |
| **Secrets backend** | Secrets | Compact: OS keychain, fallback encrypted file (0600); server/Cloud: envelope encryption in PostgreSQL with a KMS-held key per Organization ([ADR-0022](adr/0022-cloud-tenancy-row-level-isolation-by-organization.md)) |

Storage ports stay in domain code so tests use fakes; production has one adapter per port (PostgreSQL) plus the two Artifact adapters.

## 4. Consistency rules

- **Compact PostgreSQL listener**: a Unix domain socket in the per-user state directory on Linux and macOS; on Windows a random loopback TCP port with a generated SCRAM password stored in the user-only state file. The supervisor is `embedded-postgres` v2 with a local binary provider over the installer-bundled binaries ([TECHNOLOGY_BENCHMARK §3](TECHNOLOGY_BENCHMARK.md)).

- A state transition, its Event and the messages it causes are written in one PostgreSQL transaction; messages leave through the outbox relay to NATS, so clients never see an Event for a state that did not commit and consumers deduplicate by id.
- Every persisted record carries a schema version. Loaders refuse unknown future versions and migrate older ones explicitly (FEAT-OPS-002).
  - *Implemented, unit-tested and PostgreSQL-tested on Linux (#177):* the migration runner in `services/platform/internal/store/migrations` runs goose-format files from an embedded directory (sha256 manifest `migrations.sum`), records versions in `amaru_schema_migrations`, refuses a database newer than the build and a file older than an applied one, and applies each file with its version row in one transaction under an advisory lock. No table exists yet; per-record schema versions for each persisted format come with each adapter.
- Artifacts are written before the Step that produced them is marked succeeded. Their hashes are computed by the Platform.
- Definitions edited outside Amaru (files under `.amaru/` changed on disk or in Git) are detected and produce change Events (FEAT-WRK-003); every Execution records the definition revision (commit SHA + content hash).

## 5. Retention and redaction

Retention is an Organization Policy setting per data class ([ADR-0017](adr/0017-content-addressed-artifact-store-filesystem-or-s3.md) for Artifacts). Deployment-wide defaults [Proposed, validated with the first self-hosted operators at M7]:

| Class | Default |
|---|---|
| Events, Approvals, Grants and other audit records | Kept indefinitely (Cloud: at least 1 year, then per contract) |
| Logs and evidence (stdout/stderr, transcripts) | 30 days |
| Intermediate Artifacts (not referenced by an Approval or accepted Work Item) | 90 days |
| Artifacts referenced by an Approval or an accepted Work Item | Kept while referenced |
| Event replay window for clients (cursor resume) | 7 days compact, 30 days server and Cloud |

- Approvals and audit Events are retained at least as long as the Artifacts they justify; a shorter setting is rejected.
- Redaction:
  - known secret values delivered to a Lease are removed **on the Worker** before any Event, log or upload leaves it; configurable patterns are applied again on the Platform before storage
  - redaction failures are counted and surfaced
- Export and deletion per Organization (PRIV-2).

## 6. Migration of existing data

Moving prototype project data into Amaru is a migration concern. It requires:
- no runtime compatibility with the prototype's project folder ([ADR-0043](adr/0043-project-definitions-in-git-teams-and-operational-state-in-platform.md));
- a one-shot importer (`amaru import legacy`) for definitions and Work Items, scheduled in the roadmap; layout and design (Proposed): [AMARU_DIRECTORY](AMARU_DIRECTORY.md), [LEGACY_IMPORTER](LEGACY_IMPORTER.md);
- Prototype operational history stays read-only in the prototype; it is not imported;
- no live dual-write.
