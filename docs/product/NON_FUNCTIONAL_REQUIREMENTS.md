# Non-functional Requirements

Status: **accepted planning baseline** (2026-10-25, owner-reviewed with the planning package). The canonical quality baseline for Amaru. [PRD §8](PRD.md#8-non-functional-requirements) summarizes NFR-1 to NFR-8; their meaning is unchanged here and new IDs are added from NFR-9. Nothing is implemented in this repository.

**Targets.** Every requirement has a concrete initial target. Where evidence does not yet exist, the number is an **initial target** chosen from the prototype precedent and common practice, marked *(initial)*, and the named milestone validates it (validation register A14 in [OPEN_QUESTIONS](../architecture/OPEN_QUESTIONS.md#validation-register)). A failed validation changes the number here with its evidence; it does not reopen the requirement.

**Columns.** *Compact* = local or single-host self-hosted install; *Server* = small-organization or distributed self-hosted; *Cloud* = managed service. "Same" means the requirement does not differ.

## 1. Reliability and durability

| ID | Requirement | Compact | Server | Cloud | Target / how set |
|---|---|---|---|---|---|
| NFR-3 | No acknowledged state change is lost on restart; crash behavior is defined and tested. | Same | Same | Same | Crash-injection suite (every Step state) passes in M3. |
| NFR-9 | Approval waits and remote jobs survive Platform restarts and last for days. | Same | Same | Same | Test with a simulated multi-day wait (clock control) in M3. |
| NFR-10 | Lost Workers are detected and their Steps resolved without operator action. | n/a (embedded Worker) | Lease expiry | Lease expiry | Heartbeat 10 s, Lease 30 s; a lost Worker's Steps are resolved within 60 s *(initial)*. Validated by the M5 partition suite. |
| NFR-11 | Duplicate or replayed messages never cause a duplicate consequential action. | Same | Same | Same | Idempotency tests for every consequential Capability in M3 and M5. |
| NFR-12 | Platform availability. | Best effort (user machine) | Operator-defined; single instance acceptable | 99.9 % monthly API availability *(initial; contractual SLA set with commercial terms)* | Validated in the M9 pilot. |

## 2. Performance and scalability

| ID | Requirement | Compact | Server | Cloud | Target / how set |
|---|---|---|---|---|---|
| NFR-13 | Platform overhead is small relative to agent run time. | Same | Same | Same | Platform overhead per Step transition (commit + publish) p95 < 50 ms on the compact install *(initial)*. Measured in M3, enforced from M4. |
| NFR-14 | UI responsiveness for core flows (Work list, Execution timeline, Approval). | Same | Same | Same | Local API reads p95 < 200 ms; Web client first load < 2 s and view changes < 500 ms on the compact install *(initial)*. Measured in M4 on the recorded MVP runs. |
| NFR-15 | Scale of a single Instance (Projects, concurrent Steps, history size). | One user, a few concurrent agents (WO default `--max-agents` 2) | 50 users, 20 Workers, 20 concurrent Steps per single Platform instance *(initial)* | Horizontal scaling | Load test in M7 validates the Server figure; M9 for Cloud. |
| NFR-16 | Event stream keeps up with clients and replays a bounded window. | Same | Same | Same | Replay window 7 days compact, 30 days Server/Cloud; Events persist beyond it in PostgreSQL *(initial; WO keeps a 1024-event ring)*. Measured in M3. |

## 3. Security and isolation

| ID | Requirement | Compact | Server | Cloud | Target / how set |
|---|---|---|---|---|---|
| NFR-17 | Every request is authenticated, except health. | Loopback + one-time launch code → session cookie (ADR-0034) | User sessions, tokens | Same as Server + tenant | Security tests in M3 / M7. |
| NFR-18 | Plugins and Providers run with only declared and granted permissions, isolated from the Platform process (Wasm `logic` in the separate plugin-host process, ADR-0040/0041). | Same | Same | Same + per-tenant isolation | Escape tests in M6. |
| NFR-19 | Secrets never appear in Events, logs, Artifacts, errors or UI. | Same | Same | Same | Secret-fixture tests in every release (WO precedent). |
| NFR-20 | No consequential action without a recorded Grant or Approval. | Same | Same | Same | Policy tests in M3; ADR-0006. |
| NFR-21 | Supply-chain integrity of releases and plugins. | Checksums | Signed releases | Signed releases and plugins | cosign-signed plugin packages from M6 ([ADR-0021](../architecture/adr/0021-plugin-packaging-signing-permissions-and-compatibility.md)); signed Desktop updates (ADR-0018). |

Details: [security model](../security/SECURITY_MODEL.md).

## 4. Privacy and data ownership

| ID | Requirement | Compact | Server | Cloud | Target / how set |
|---|---|---|---|---|---|
| NFR-22 | Project knowledge stays in the user's repositories; Amaru stores operational state separately. | Same | Same | Same | Data architecture review in M3. |
| NFR-23 | No data leaves the machine unless a remote Provider or Integration is enabled (PRIV-1). | Required | Required for the Instance's network | Explicit data-processing terms | Egress tests in M3. |
| NFR-24 | Users can export and delete their data (PRIV-2). | Bundled PostgreSQL data + Artifacts; `amaru backup` and export | Export tool | Export + deletion with stated retention | M7 (Server), M9 (Cloud). |

## 5. Portability

| ID | Requirement | Compact | Server | Cloud | Target / how set |
|---|---|---|---|---|---|
| NFR-1 | Local deployment runs on Windows, macOS and Linux without Docker/Podman. | Required | — | — | Recorded install and run per OS in M4. |
| NFR-25 | Workers run on Windows, macOS and Linux; environment-specific tools are optional capabilities. | Embedded | Required | Managed Linux + customer Workers on any OS | M5 pilot on Windows. |
| NFR-26 | Web client works in current Chromium, Firefox and WebKit-based browsers. | Same | Same | Same | End-to-end tests in M4. |

## 6. Extensibility

| ID | Requirement | Target / how set |
|---|---|---|
| NFR-27 | Adding a Provider or Capability changes no core code, contract shape or UI architecture. | PRD §12 test, from M3. |
| NFR-28 | A plugin written against the SDK runs unchanged in Web and Desktop, and on any deployment where its components' targets are available. | Example plugins tested in both clients in M6 and M8. |

## 7. Compatibility and versioning

| ID | Requirement | Target / how set |
|---|---|---|
| NFR-2 | Public contracts versioned; additive within a major version; fixtures tested in every consuming language. | M2. |
| NFR-29 | Clients, Workers and plugins detect incompatible Instances and fail explicitly. | M3 (clients), M5 (Workers), M6 (plugins). |
| NFR-30 | Supported version skew between Platform and Workers/clients is documented. | [ADR-0025](../architecture/adr/0025-api-and-protocol-versioning-policy.md): current and previous major supported, at least 12 months of overlap; Workers negotiate the protocol major at handshake. |

## 8. Observability

| ID | Requirement | Compact | Server | Cloud | Target / how set |
|---|---|---|---|---|---|
| NFR-4 | Enough Events and metadata to reconstruct who did what, when, with which inputs and at what cost. | Same | Same | Same | Audit walkthrough in M4. |
| NFR-31 | Metrics and traces exportable to standard tooling. | Optional | Required | Required | M7. |
| NFR-32 | A support diagnostics bundle without secrets. | Required | Required | Internal tooling | M7. |

## 9. Recoverability and operations

| ID | Requirement | Compact | Server | Cloud | Target / how set |
|---|---|---|---|---|---|
| NFR-6 | Retention, redaction, schema migration of persisted data. | Same | Same | Same | Defaults in [DATA_ARCHITECTURE §5](../architecture/DATA_ARCHITECTURE.md) (audit indefinite, logs 30 days, intermediate Artifacts 90 days); M3. |
| NFR-7 | Install, upgrade, backup and restore documented and tested. | Install + upgrade | All four | Vendor runbooks | M4 (compact install), M7 (server). |
| NFR-33 | Recovery point and recovery time objectives. | User backups | Operator-defined | RPO ≤ 15 min (PostgreSQL point-in-time recovery), RTO ≤ 4 h | *(initial)* Validated by a restore drill in M9. |
| NFR-34 | Operational footprint: the compact install needs no service the user installs or operates; the Platform supervises its bundled PostgreSQL and embeds NATS. | Required | Database server allowed | Managed services allowed | [ADR-0015](../architecture/adr/0015-postgresql-is-the-only-database-bundled-in-compact-installs.md), [ADR-0016](../architecture/adr/0016-nats-jetstream-message-bus-embedded-in-compact-installs.md); compact installer increment ≤ 120 MB per OS and idle memory ≤ 300 MB for Platform + PostgreSQL + NATS *(initial)*; validated by the M1 spike. |

## 10. Maintainability

| ID | Requirement | Target / how set |
|---|---|---|
| NFR-5 | Core tests use fakes; real-tool validation recorded separately. | From M3. |
| NFR-35 | Architecture boundaries enforced by tests (domain imports no OS, vendor or transport code). | WO `internal/arch/arch_test.go` ported in M3. |
| NFR-36 | `make check` covers every area and is the release gate; hosted CI mirrors it when introduced. | From M2. |

## 11. Accessibility and usability

| ID | Requirement | Target / how set |
|---|---|---|
| NFR-37 | Core flows meet WCAG 2.2 AA. | Automated checks + manual keyboard pass in M4. |
| NFR-38 | The UI shows only real state; unimplemented areas are explicit empty states. | Review in M4 (WO UI rule; WP pages without backend are removed or labelled). |

## 12. Deployment and operational complexity

| ID | Requirement | Target / how set |
|---|---|---|
| NFR-8 | Cloud tenant isolation, availability targets and horizontal scaling. | M9. |
| NFR-39 | Every infrastructure dependency added to the Platform has an accepted ADR and a compact-mode answer (embedded or optional). | Enforced by ADR-0008 and AGENTS.md rule 6. Current answers: PostgreSQL bundled (ADR-0015), NATS embedded (ADR-0016), object storage optional via the filesystem adapter (ADR-0017). |

## 13. Requirements that differ by deployment

| Concern | Compact | Server | Cloud |
|---|---|---|---|
| Identity | Single local owner, launch code + session cookie (ADR-0034) | Users, Organizations, local accounts and OIDC (ADR-0028) | Same + tenant boundary |
| Storage | Bundled PostgreSQL + filesystem artifacts | PostgreSQL + filesystem or S3-compatible (RustFS reference) | Managed PostgreSQL + S3-compatible object storage |
| Messaging | Embedded NATS JetStream | Embedded (single node) or NATS cluster | NATS cluster |
| Workers | Embedded | Separate processes, remote machines | Managed + customer-run |
| Availability | Best effort | Operator-defined | 99.9 % monthly target for the API (initial; contractual SLA set with commercial terms) |
| Upgrades | User-driven | Operator-driven with migrations | Vendor-driven, zero-downtime via expand/contract migrations and rolling instances (ADR-0023) |
| Plugins | Local components only | Plus service plugins | Plus vendor-reviewed service plugins |
