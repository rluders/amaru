# Technology Selection

Status: **decided** (2026-10-25). Every technology choice below is recorded in an accepted ADR; this document keeps the comparison and the verified facts behind each choice. Nothing is implemented. The M1–M9 spikes **validate** these choices ([validation register](OPEN_QUESTIONS.md#validation-register)); a failed validation leads to a superseding ADR.

**Evidence labels**
- **[Verified]** Confirmed in source code, an official repository or official documentation. Source and date cited.
- **[Reported]** Not verified from a primary source; not relied on by any ADR.

External facts were verified on **2026-10-25** from the official repositories, release pages and documentation linked in each ADR.

## 1. Amaru requirements that drive the choices

From the [PRD](../product/PRD.md), [NFRs](../product/NON_FUNCTIONAL_REQUIREMENTS.md) and [resilience model](RESILIENCE.md):
- **R1** Compact install needs no service the user installs or operates, on Windows, macOS and Linux, with no Docker (NFR-1, NFR-34 as restated by [ADR-0015](adr/0015-postgresql-is-the-only-database-bundled-in-compact-installs.md)).
- **R2** Durable multi-day human waits, restart recovery, idempotency, timeouts and leases (NFR-3, NFR-9 – NFR-11).
- **R3** Same domain and contracts across compact, server and Cloud (ADR-0002).
- **R4** Go Platform. The existing code is Go (ADR-0011).
- **R5** License compatible with a fair-code commercial model and self-hosting ([ADR-0026](adr/0026-fair-code-community-license-with-commercial-licensing.md)).
- **R6** Testable with fakes and without servers in `make check` (NFR-5).

## 2. Durable workflow execution

**Selected: own state machine on PostgreSQL** ([ADR-0042](adr/0042-durable-execution-with-explicit-restart-semantics.md)).

| Candidate | Why not selected |
|---|---|
| Temporal | Separate server cluster in every install; deterministic-code workflows clash with declarative YAML graphs. The product owner judged it too heavy. |
| Restate | A second server process; license not verified from a primary source [Reported]. |
| Hatchet | Postgres-based, but imposes its own engine, schema and gRPC worker model on top of the domain prototype already models. |
| DBOS Transact | Library model fits R4/R6, but its checkpoint schema and workflow-as-code model duplicate the prototype state machine. |
| Conductor | JVM server, database and queue; too heavy for R1. |

**Validation (M1):** representative Workflow with an agent Step, Local Check, multi-day Approval wait under clock control, remote job and forced crashes in every state, on bundled PostgreSQL on three OSes.

## 3. Persistence and object storage

**Selected:** PostgreSQL everywhere, bundled in compact installs ([ADR-0015](adr/0015-postgresql-is-the-only-database-bundled-in-compact-installs.md)); content-addressed Artifact store with filesystem and S3-compatible adapters, RustFS as reference ([ADR-0017](adr/0017-content-addressed-artifact-store-filesystem-or-s3.md)).

| Candidate | Facts | Outcome |
|---|---|---|
| Files (the prototype) | [Verified] Atomic temp+rename, no migrations | Kept only for committed definitions under `.amaru/` ([ADR-0043](adr/0043-project-definitions-in-git-teams-and-operational-state-in-platform.md)). |
| SQLite (`modernc.org/sqlite`) | [Verified] BSD-3-Clause, pure Go, Windows support (pkg.go.dev, v1.61.0) | Rejected: a second adapter and dialect next to PostgreSQL. |
| PostgreSQL | [Verified] RLS, FTS, `SKIP LOCKED`. (A prior claim of a prototype store with 30 migrations is unverified: the Runtime at `c8a2f6e` is file-based, [ADR-0050](adr/0050-reimplement-the-prototype-stores-as-postgresql-adapters.md).) | **Selected.** |
| `fergusstrange/embedded-postgres` | [Verified 2026-10-25, README] MIT, v2 stable (v2.0.1, 2026-10-19), zero third-party dependencies in production packages, PG 15–18, native CI on Linux/macOS amd64+arm64 and Windows amd64/arm64; downloads binaries by default, but a `LocalProvider` / custom `BinaryProvider` uses preinstalled binaries; persistent `DataDir`; an independent supervisor closes PostgreSQL when the owner process dies (owner pipe). The README positions it mainly for tests. | **Selected** as the compact supervisor, with `LocalProvider` over the installer-bundled pinned binaries (nothing downloaded at runtime). Its test focus is the main risk; the M1 spike validates production use (A12). Fallback: an in-house supervisor of the same bundled binaries. |
| In-house PostgreSQL supervisor (fallback for A12) | [Compared 2026-10-26, WP-M1-02; `embedded-postgres` ran on Linux, macOS and Windows hosted VMs] Would own: start `postgres` from the bundled binaries, readiness check, stop, kill on owner death (owner pipe or Job Object), stale-directory cleanup, first-start rebuild, wait for the previous postmaster. | **Not needed on Linux:** `embedded-postgres` v2.0.1 with `LocalProvider` and an explicit supervisor path did all of these except the last three, which are small Platform-side rules (see the [investigation](investigations/2026-10-26-wp-m1-02-storage-and-execution.md)). Costs of the fallback: the owner-death supervision on Windows (Job Objects, restricted token) and macOS is the part the library already implements and that the spike could not test there. Re-evaluate after the macOS and Windows runs. |
| RustFS | [Verified] Apache-2.0, GA 1.0.0 (2026-09-16), 1.0.4 latest; versioning and object lock available; Windows binary published, Windows support not stated | **Reference S3 backend.** |
| MinIO | [Verified] Repository archived, "no longer maintained" | Rejected. |
| SeaweedFS / Garage | [Verified] Apache-2.0 / AGPL-3.0 | Usable through the S3 adapter; not the reference. |

## 4. Messaging and queues

**Selected: NATS JetStream**, embedded in compact installs, cluster for multi-instance ([ADR-0016](adr/0016-nats-jetstream-message-bus-embedded-in-compact-installs.md)).

- [Verified] `nats-server` Apache-2.0, v2.15.1 (2026-10-09); embeddable in Go (`server.NewServer`, `DontListen`, `InProcessConn`, `nats.InProcessServer`).
- [Verified] JetStream dedup by `Nats-Msg-Id` within a duplicate window (default 2 minutes); work-queue retention; explicit acks, `MaxDeliver`, `BackOff`.
- Rejected: PostgreSQL-only queue until Cloud (owner chose one bus model from day one); RabbitMQ (not embeddable). `riverqueue/river` (MPL-2.0, v0.49.0) was considered as a PostgreSQL queue library and is not needed.

## 5. Desktop framework

**Selected: Tauri v2** ([ADR-0018](adr/0018-desktop-shell-is-tauri-v2.md)).

| Candidate | Facts | Outcome |
|---|---|---|
| Tauri v2 | [Verified] Stable since 2024-10-02, 2.12.5 latest; `externalBin` sidecars; per-window capabilities; mandatory updater signing | **Selected.** |
| Wails v3 | [Verified] Still beta (v3.0.0-beta.34, 2026-10-20) | Rejected: immature. |
| Electron | Large footprint and Node attack surface | Rejected. |

**Validation (M8):** sidecar supervision, keychain, signed updater on three OSes.

## 6. Plugin execution and sandboxing

**Selected:** isolation tiers ([ADR-0041](adr/0041-isolation-tiers-for-plugins-providers-and-steps.md)); `logic` as Javy-built QuickJS Wasm on wazero in a separate plugin-host process ([ADR-0040](adr/0040-plugin-logic-runs-in-a-separate-plugin-host-process-built-with-javy.md)); signed OCI packages ([ADR-0021](adr/0021-plugin-packaging-signing-permissions-and-compatibility.md)).

| Candidate | Facts | Outcome |
|---|---|---|
| Sandboxed iframes + postMessage | — | **Selected** for `ui`. |
| wazero + QuickJS(-ng) Wasm | [Verified] wazero Apache-2.0, v1.12.0, moved to `github.com/wazero/wazero` | **Selected** for `logic`. |
| Extism Go SDK | [Verified] Last release v1.7.1 (2025-03-19); `allowed_hosts: null` allows all hosts | Not used at runtime. |
| Javy (`bytecodealliance/javy`) | [Verified 2026-10-25] Apache-2.0, v9.1.0 (2026-07-30), active; Javy plugins can produce `wasm32-unknown-unknown` modules with no WASI imports and embedder-defined host calls | **Selected** build toolchain for `logic`, with an Amaru Javy plugin ([ADR-0040](adr/0040-plugin-logic-runs-in-a-separate-plugin-host-process-built-with-javy.md)). |
| Extism JS PDK | [Verified 2026-10-25] BSD-3-Clause, v1.7.0 (2026-08-17); modules target the Extism kernel ABI | Rejected: needs the Extism runtime or a reimplementation of its kernel. |
| Deno permissions | [Verified] `--allow-run`/`--allow-ffi` bypass the sandbox | Rejected. |
| Child processes + OS sandbox | No portable OS sandbox without admin rights | Process isolation + Policy; Landlock on Linux when available. |
| Rootless containers | [Verified] Nested Podman fails in the prototype | **Selected** for `container`-class Environments and `service`: one container per Lease, single level only ([ADR-0041](adr/0041-isolation-tiers-for-plugins-providers-and-steps.md)). |
| cosign + OCI (ORAS) | [Verified] cosign v3.1.4 keyless and offline bundle verification; ORAS v1.3.6 Apache-2.0 | **Selected** for signing and distribution. |

## 7. Remote execution and worker management

**Selected:** own Worker protocol, Protobuf over WebSocket with `github.com/coder/websocket` ([ADR-0029](adr/0029-worker-protocol-over-websocket-and-protobuf.md)); enrollment and key identity ([ADR-0024](adr/0024-remote-worker-registration-identity-and-delivery.md)); agents beside Environment tools ([ADR-0035](adr/0035-agents-run-beside-environment-hosted-tools.md)).

- [Verified] `coder/websocket` ISC, v1.8.15 (2026-06-15), maintained, already used by the prototype. `gorilla/websocket` low-activity (v1.5.3, 2024-06); excluded.
- Rejected: HTTP long-poll (latency for cancel/lease traffic); gRPC bidi (HTTP/2 through proxies); engine-native worker protocols (no engine is used).
- [Verified] Epic's Unreal MCP plugin: UE 5.8, experimental, loopback-only `127.0.0.1:8000/mcp`, no authentication, sequential on the game thread. Community servers (e.g. `chongdashu/unreal-mcp`, `flopperam/unreal-engine-mcp`, `runreal/unreal-mcp`) are early-stage. The M5 pilot uses Epic's plugin.
- Reference patterns studied, not adopted: GitHub Actions runners, Buildkite agents, Temporal task queues, Hatchet workers.
- **Provisioning (M7):** provisioner plugins behind `amaru.provisioner/v1`; Proxmox VE is the first first-party provisioner ([EXECUTION_ENVIRONMENTS §5](EXECUTION_ENVIRONMENTS.md#5-provisioning-and-lifecycle-feat-env-014-m7)).

## 8. Self-hosted packaging and identity

**Selected:** OCI image + reference compose, Atlas-authored migrations applied by the Platform ([ADR-0023](adr/0023-self-hosted-oci-image-atlas-migrations-operator-owned-backup.md)); local accounts + OIDC ([ADR-0028](adr/0028-identity-local-accounts-oidc-and-opaque-tokens.md)).

- [Verified] Atlas Community Edition Apache-2.0; versioned `migrate diff/apply/status` free; lint and RLS schema support are not in Community.
- [Verified] goose MIT v3.29.0; golang-migrate MIT v4.20.1.
- [Verified] `coreos/go-oidc/v3` Apache-2.0, v3.21.0.
- [Verified] PostgreSQL RLS: `FORCE ROW LEVEL SECURITY`, transaction-scoped `set_config` ([ADR-0022](adr/0022-cloud-tenancy-row-level-isolation-by-organization.md)).
- [Verified] pgvector: PostgreSQL License, v0.8.7 — reserved for opt-in semantic Knowledge ([ADR-0030](adr/0030-knowledge-is-lexically-indexed-project-documentation.md)).

### Contracts: TypeScript SDK generation

**Selected:** `openapi-typescript` (types) + `openapi-fetch` (typed client) from the OpenAPI 3.1 contract (FEAT-API-003, WP-M1-03). Exact versions are pinned when `packages/sdk` is created (M2).

| Candidate | Facts | Outcome |
|---|---|---|
| `openapi-typescript` + `openapi-fetch` | [Verified 2026-10-25, openapi-ts.dev and npm] MIT; OpenAPI 3.0 and 3.1 including discriminators; runtime-free types; Node.js only (no Java); `openapi-typescript` 7.13.0, `openapi-fetch` 0.17.0 | **Selected**: smallest runtime, types follow the JSON Schema shapes directly, fits the staleness check. |
| `@hey-api/openapi-ts` | [Verified 2026-10-25, npm] MIT, 0.99.0 (pre-1.0) | Not selected: larger generated surface, still pre-1.0. |
| OpenAPI Generator | Java toolchain | Rejected: adds a JVM to `make check`. |

The event-stream client (WebSocket, cursor resume) is hand-written on top of the generated types, because OpenAPI does not describe the stream.

## 9. Platforms worth studying for patterns

Read these for architecture lessons only. None is planned as a dependency.

| Project | What to learn |
|---|---|
| Backstage | Plugin packaging with frontend/backend parts; permission framework |
| Grafana | Frontend plugin sandboxing and signing levels |
| VS Code | Extension host process model; contribution points; activation events |
| Buildkite / GitHub Actions runners | Outbound worker registration and job leasing |
| Argo Workflows | YAML DAG definitions with persisted state |

## 10. Output

- Each validation spike ends with a report in `docs/architecture/investigations/`, created with the first report. A failed validation produces a superseding ADR.
- Dependencies are added to code only when the component that needs them is built, with a pinned version and the reason ([conventions](../engineering/CONVENTIONS.md#dependencies)).
