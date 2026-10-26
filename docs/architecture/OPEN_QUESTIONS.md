# Open Architectural Questions

Questions that block or shape decisions. Each is closed by an ADR ([DECISIONS](DECISIONS.md)) and kept here under "Closed" with the product owner's answer, the decision and a link; questions are never deleted. Headings keep their original text so links stay stable.

## Questions

**None open.** Two passes on 2026-10-25 closed every question:
- **First pass, Q1–Q22 and W1–W4.** A product-owner planning session answered or delegated each question and accepted ADRs 0014–0039. Owner answers are kept as given, with spelling normalized. Where the owner delegated an answer ("you can decide"), Claude decided under that delegation and the decision was accepted in the same session.
- **Second pass, Q23–Q54.** A full sweep of the planning documents found questions that had been left implicit: choices deferred to spikes, contradictions between ADRs, unset numeric targets, and design questions recorded only as feature "risks". The owner delegated all of them in the planning prompt ("make the most defensible decision"). Significant ones are accepted ADRs 0040–0043, which supersede 0020, 0019, 0014 and 0027. Reversible ones are decided in the document named in each entry. No first-pass owner answer was overturned. Two were refined:
  - The owner's "WebAssembly" answer (Q6/Q14) now also requires a separate process (Q23).
  - Q13's metadata split now keeps Teams out of Git (Q27).

A new question is added here with **why it matters**, **evidence required** and **what it blocks**, and is closed by an ADR.

## Validation register

Decisions that rest on assumptions name how they are checked. Each check is a roadmap task; a failed check reopens the decision through a superseding ADR.

| ID | Assumption | Decision | Checked at | If false |
|---|---|---|---|---|
| A1 | Weave Runtime components can be adapted faster than reimplemented. | [ADR-0011](adr/0011-platform-adapted-from-weave-runtime.md) | M3 (first migration ADR) | Reimplement the coordinator with Weave tests as specification. |
| A2 | A bundled, supervised PostgreSQL works without admin rights on Windows, macOS and Linux and survives crashes and upgrades. | [ADR-0015](adr/0015-postgresql-is-the-only-database-bundled-in-compact-installs.md) | M1 spike | Install PostgreSQL as an installer-managed service (one-time admin consent, the user operates nothing) on the affected OS, through a superseding ADR ([Q30](#q30-fallback-if-bundled-postgresql-fails)). |
| A3 | Users accept a browser UI for the local MVP. | [ADR-0010](adr/0010-local-mvp-client-is-a-web-ui.md) | M4 validation | Bring Desktop (M8) forward. |
| A4 | The Weave prototype design system can be reused with modest change. | [ADR-0010](adr/0010-local-mvp-client-is-a-web-ui.md) | M4 | Rebuild components on shadcn directly. |
| A5 | Outbound pull Workers, run inside an interactive session, satisfy Windows/UE5 use cases. | [ADR-0024](adr/0024-remote-worker-registration-identity-and-delivery.md) | M5 UE5 pilot | Add a session-broker design or keep a person at the machine. |
| A6 | A single manifest with targeted components is manageable for plugin authors. | [ADR-0012](adr/0012-plugin-package-with-targeted-components.md) | M6 example plugins | Allow multi-package plugins with a parent manifest. |
| A7 | Licensing permits moving MIT Weave code into Amaru. | [ADR-0026](adr/0026-fair-code-community-license-with-commercial-licensing.md) | **Resolved 2026-10-25**: sole copyright holder consents; no third-party authors in either repository. | — |
| A8 | Unreal MCP servers expose enough editor functionality for useful agent work. | [ADR-0035](adr/0035-agents-run-beside-environment-hosted-tools.md) | M5 UE5 pilot | Fall back to command-line engine Capabilities (build, test, commandlets). |
| A9 | Embedded NATS JetStream fits compact installs (footprint, Windows file locking). | [ADR-0016](adr/0016-nats-jetstream-message-bus-embedded-in-compact-installs.md) | M1 spike | Replace the embedded bus in compact mode with an in-process implementation of the `Bus` port. |
| A10 | Javy-built QuickJS Wasm in a separate plugin-host process performs well enough for plugin `logic` (cold start < 50 ms, memory cap enforced, crash isolated), and Javy keeps supporting WASI-free modules. | [ADR-0040](adr/0040-plugin-logic-runs-in-a-separate-plugin-host-process-built-with-javy.md) | M6 spike | Move heavy logic to `worker-adapter` components; revisit the toolchain or runtime. |
| A11 | The name "Amaru" is clear for use as a trademark. | [Q34](#q34-product-name-and-trademark) | Before the first public release (owner task, WP-M1-01) | Rename through an ADR before any public release. **Status 2026-10-26: not confirmed.** A preliminary public-source search found no register result (registers not searchable by the agent) and found other software named Amaru, one in developer tooling for AI agents: [name search](investigations/2026-10-26-amaru-name-search.md). |
| A12 | `embedded-postgres` v2, built mainly for tests, works as a production supervisor with `LocalProvider` over bundled binaries. | [Q28](#q28-postgresql-supervisor-library) | M1 spike | In-house supervisor of the same bundled binaries (start, health, stop, kill on owner death). |
| A13 | Container start-up per Lease is cheap relative to Step time. | [ADR-0041](adr/0041-isolation-tiers-for-plugins-providers-and-steps.md) | M5 | A per-Execution warm pool within one Organization, through a new ADR. |
| A14 | The initial numeric targets (Lease and heartbeat, timeouts, slots, replay window, latency, scale, retention) fit real use. | [Q43](#q43-retention-and-redaction-defaults), [Q44](#q44-initial-numeric-operating-targets) | The milestone named per target in [NFRs](../product/NON_FUNCTIONAL_REQUIREMENTS.md) | Change the number in the NFR document with its evidence; the requirement stays. |
| A15 | The compact installer increment stays ≤ 120 MB per OS and idle memory ≤ 300 MB. | [ADR-0015](adr/0015-postgresql-is-the-only-database-bundled-in-compact-installs.md), [ADR-0016](adr/0016-nats-jetstream-message-bus-embedded-in-compact-installs.md) | M1 spike | Trim the bundled PostgreSQL build (drop unused extensions and locales); if that is not enough, raise the budget in NFR-34 with evidence. |

## Validation spikes

Spikes no longer choose technologies; they validate accepted decisions. Each ends with a short report in `docs/architecture/investigations/` (created with the first report) and, if a check fails, a superseding ADR.

1. **M1 — storage and execution**: bundled PostgreSQL + embedded NATS + own state machine run the [representative Workflow](ARCHITECTURE.md#15-representative-workflow-execution) with a forced crash in each Step state on three OSes ([ADR-0042](adr/0042-durable-execution-with-explicit-restart-semantics.md), [ADR-0015](adr/0015-postgresql-is-the-only-database-bundled-in-compact-installs.md), [ADR-0016](adr/0016-nats-jetstream-message-bus-embedded-in-compact-installs.md)).
2. **M4: local Web authentication** security review ([ADR-0034](adr/0034-local-web-client-authentication-with-launch-code-and-cookie.md)) and usage capture from agent CLIs ([ADR-0033](adr/0033-usage-and-cost-telemetry-model.md)).
3. **M5 — remote Worker and UE5 pilot**: second machine, forced disconnects, Windows process trees, MCP gateway on a Windows VM ([ADR-0024](adr/0024-remote-worker-registration-identity-and-delivery.md), [ADR-0029](adr/0029-worker-protocol-over-websocket-and-protobuf.md), [ADR-0035](adr/0035-agents-run-beside-environment-hosted-tools.md)).
4. **M6 — plugin sandbox and signing** ([ADR-0040](adr/0040-plugin-logic-runs-in-a-separate-plugin-host-process-built-with-javy.md), [ADR-0041](adr/0041-isolation-tiers-for-plugins-providers-and-steps.md), [ADR-0021](adr/0021-plugin-packaging-signing-permissions-and-compatibility.md)).
5. **M7 — upgrade and restore drill**, OIDC ([ADR-0023](adr/0023-self-hosted-oci-image-atlas-migrations-operator-owned-backup.md), [ADR-0028](adr/0028-identity-local-accounts-oidc-and-opaque-tokens.md)).
6. **M8 — Tauri sidecar, signing and updater** ([ADR-0018](adr/0018-desktop-shell-is-tauri-v2.md)).
7. **M9 — cross-tenant isolation test** ([ADR-0022](adr/0022-cloud-tenancy-row-level-isolation-by-organization.md)).

## Closed

### Q1. Durable workflow technology
- **Question:** Keep a hand-written durable state machine (Weave's model), or adopt a durable execution engine (Temporal, Restate, DBOS, Hatchet, a database-backed queue)?
- **Owner answer:** "Temporal is way too overpower, let's use Postgres."
- **Decision:** Amaru's own durable state machine, adapted from Weave, persisted in PostgreSQL with timers, Approval waits, idempotency keys and a transactional outbox. → [ADR-0014](adr/0014-durable-execution-on-own-state-machine-over-postgres.md), superseded by [ADR-0042](adr/0042-durable-execution-with-explicit-restart-semantics.md) (restart rule only, [Q26](#q26-restart-semantics-for-non-resumable-steps))

### Q2. Queue and messaging strategy
- **Question:** Does the Platform need a message broker or a database-backed work queue, and from which deployment onward?
- **Owner answer:** "NATS, 'cause it can handle idempotency easily and have nice performance and scalability." In the closing session the owner chose NATS from day one, embedded in compact installs.
- **Decision:** NATS JetStream in every deployment, embedded in-process for compact/single-node, external cluster for multi-instance. PostgreSQL stays the source of truth (outbox → JetStream with `Nats-Msg-Id`; idempotent consumers; leases in PostgreSQL). → [ADR-0016](adr/0016-nats-jetstream-message-bus-embedded-in-compact-installs.md)
- **Note:** JetStream dedup is limited to a time window (default 2 minutes), so end-to-end idempotency still relies on PostgreSQL keys; this is part of the decision, not a reservation.

### Q3. Database and persistence strategy
- **Question:** Which storage per deployment: files, SQLite, PostgreSQL? One engine or adapters?
- **Owner answer:** "Postgres." In the closing session the owner chose a bundled PostgreSQL for compact installs.
- **Decision:** PostgreSQL is the only engine; compact installs ship pinned PostgreSQL binaries supervised by the Platform. NFR-34 restated as "no service the user installs or operates". → [ADR-0015](adr/0015-postgresql-is-the-only-database-bundled-in-compact-installs.md)

### Q4. Artifact and object-storage strategy
- **Question:** Where do generated and remote Artifacts live, and how are large outputs from remote Workers transferred?
- **Owner answer:** "RustFS is a good alternative, 'cause it can handle artifacts versions/history."
- **Decision:** Content-addressed `ArtifactStore` with versions/history in PostgreSQL; filesystem adapter for compact, S3-compatible adapter with **RustFS** as the reference backend (bucket versioning on) for self-hosted and Cloud; resumable uploads. → [ADR-0017](adr/0017-content-addressed-artifact-store-filesystem-or-s3.md)
- **Where the answer was refined:** RustFS is not mandatory in compact installs (NFR-34) and history is owned by the Platform so it does not depend on one product's versioning; RustFS GA (1.0, Apache-2.0) and MinIO's end of maintenance were verified on 2026-10-25.

### Q5. Desktop framework
- **Question:** Tauri v2, Electron, Wails, or Web-only?
- **Owner answer:** "Tauri v2 seems pretty mature, Electron is terrible and Wails is too immature."
- **Decision:** Tauri v2 thin shell with the Platform as sidecar, at M8. → [ADR-0018](adr/0018-desktop-shell-is-tauri-v2.md)

### Q6. Plugin sandboxing and execution isolation
- **Question:** How are plugin code and Provider processes isolated?
- **Owner answer:** "I would say, WebAssembly is a good stuff, and server deployments could be even better."
- **Decision:** Fixed tiers per target: `ui` in sandboxed iframes, `logic` in Wasm, adapters and Providers as separate processes, server/Cloud Steps in rootless per-Step containers never mixing Organizations. → [ADR-0019](adr/0019-isolation-tiers-for-plugins-and-providers.md), superseded by [ADR-0041](adr/0041-isolation-tiers-for-plugins-providers-and-steps.md) ([Q23](#q23-where-plugin-logic-executes), [Q25](#q25-container-granularity-and-environment-sandbox-class))

### Q7. Plugin distribution, signing, permissions and compatibility
- **Question:** How are plugins packaged, signed, discovered, granted permissions and checked for compatibility?
- **Owner answer:** "SDK + API versioning. Not sure, we need to investigate."
- **Decision:** `amaru.plugin/v1` manifest with `engines` ranges (API major + SDK major) and declared permissions granted per Organization; packages distributed as OCI artifacts or files; cosign signatures (keyless or private PKI, offline verification); unsigned only in developer mode. → [ADR-0021](adr/0021-plugin-packaging-signing-permissions-and-compatibility.md)

### Q8. Cloud tenant isolation
- **Question:** Tenancy model and isolation of secrets and Artifacts.
- **Owner answer:** "Row-level isolation. Every tenant has its own Workspace with multiple projects, this is how we scope secrets and artifacts."
- **Decision:** Shared Platform and PostgreSQL with RLS by `organization_id` plus app-level scoping; per-Organization envelope keys and artifact prefixes; single-tenant vendor Worker leases. The owner's "Workspace" is the glossary's **Organization** (term confirmed in the closing session). → [ADR-0022](adr/0022-cloud-tenancy-row-level-isolation-by-organization.md)

### Q9. Self-hosted installation, upgrades, backup and recovery
- **Question:** What does an operator install, and how are upgrades, migrations, backup and restore done?
- **Owner answer:** "Container image. We use database migration tool (Atlas), and the rest is responsibility of the person that handles the self-host."
- **Decision:** One OCI image (Platform + `worker` subcommand), reference compose; migrations authored with Atlas Community Edition, embedded and applied by the Platform; forward-only with expand/contract; backup/restore owned by the operator with documented scope. → [ADR-0023](adr/0023-self-hosted-oci-image-atlas-migrations-operator-owned-backup.md)

### Q10. Remote execution environments
- **Question:** How do remote Workers register, authenticate, obtain the project revision, receive secrets, run interactive tools and report back?
- **Owner answer:** "VMs and remote machines need to be registered by workspace or projects, configuration should be provided manually by user. You can spike and decide the best approach here."
- **Decision:** Manual registration per Organization (optionally per Project) with one-time enrollment tokens and Ed25519 Worker keys; Git checkout at commit SHA; per-Lease in-memory secrets; Worker inside the interactive session; Job Objects on Windows. → [ADR-0024](adr/0024-remote-worker-registration-identity-and-delivery.md)

### Q11. API compatibility and versioning
- **Question:** Versioning of the API, SDK, provider and Worker protocols; event transport for remote clients.
- **Owner answer:** "Additive only changes, breaking changes introduce new version. Propose other versioning for the rest."
- **Decision:** `/v1` additive-only, breaking → new major with 12-month overlap; `amaru.<kind>/vN` schemas; Protobuf `amaru.<kind>.vN` with `buf breaking`; handshake negotiation; semver SDKs; WebSocket event stream with resumable cursor. → [ADR-0025](adr/0025-api-and-protocol-versioning-policy.md)

### Q12. Licensing and commercial boundaries
- **Question:** Community license and commercial boundary.
- **Owner answer:** "Fair-code usage from draft Amaru Community License 1.0."
- **Decision:** Platform under the Amaru Community License 1.0 (fair-code, source-available) with separate commercial licensing; contracts and SDKs under Apache-2.0; CLA for contributions; Weave code reusable (sole copyright holder). The LICENSE text, legal review and trademark search are roadmap tasks owned by the product owner (legal review dropped 2026-10-26, [WP-M1-01](WORK_PACKAGES.md#wp-m1-01-license-texts-in-effect)). → [ADR-0026](adr/0026-fair-code-community-license-with-commercial-licensing.md)

### Q13. Project metadata location
- **Question:** Where does Amaru keep per-Project state?
- **Owner answer:** "You can choose the best approach, don't need to keep compatibility with weave."
- **Decision:** Definitions committed under `.amaru/`; operational state (including Work Items) in the Platform; Executions record the definition revision. → [ADR-0027](adr/0027-project-definitions-in-git-operational-state-in-platform.md), superseded by [ADR-0043](adr/0043-project-definitions-in-git-teams-and-operational-state-in-platform.md) (Teams only, [Q27](#q27-where-teams-live))

### Q14. Sandbox for TypeScript plugin logic
- **Question:** How does plugin `logic` run?
- **Owner answer:** "Pick up the most secure option."
- **Decision:** QuickJS compiled to WebAssembly, run by wazero with Amaru-only host functions, no WASI filesystem/network, memory and CPU limits. → [ADR-0020](adr/0020-plugin-logic-runs-in-quickjs-on-wazero.md), superseded by [ADR-0040](adr/0040-plugin-logic-runs-in-a-separate-plugin-host-process-built-with-javy.md) ([Q23](#q23-where-plugin-logic-executes), [Q24](#q24-plugin-logic-build-toolchain))

### Q15. Identity and authentication model
- **Question:** Local accounts vs OIDC, machine identities, sessions, Desktop with several Instances.
- **Owner answer:** "Pick up the best approach."
- **Decision:** Local accounts (argon2id) + OIDC for self-hosted, OIDC only in Cloud; server-side browser sessions; PKCE/device flow for Desktop and CLI with keychain storage; Worker keys and hashed, revocable service tokens; no long-lived JWTs. → [ADR-0028](adr/0028-identity-local-accounts-oidc-and-opaque-tokens.md)

### Q16. Worker protocol transport
- **Question:** Which transport carries the Worker protocol?
- **Owner answer:** "Strong preference to use WebSocket with Protobuf (don't use gorilla/websocket here, it is deprecated)."
- **Decision:** Protobuf frames over WebSocket via `github.com/coder/websocket`, application-level acks and resumption, large payloads over HTTP. → [ADR-0029](adr/0029-worker-protocol-over-websocket-and-protobuf.md)
- **Note:** `gorilla/websocket` is not formally deprecated; it is low-activity (last release v1.5.3, 2024-06, verified 2026-10-25). It stays excluded.

### Q17. Knowledge scope and retrieval
- **Question:** What Amaru indexes as Knowledge, lexical vs semantic, where the index lives.
- **Owner answer:** "Your choice. I don't know."
- **Decision:** Committed Markdown docs, ADRs, Skills, Playbooks, Artifact notes and `.amaru/` definitions (not source code); lexical PostgreSQL full-text search per Project; semantic retrieval only if evaluation proves lexical insufficient, opt-in per Organization. → [ADR-0030](adr/0030-knowledge-is-lexically-indexed-project-documentation.md)

### Q18. Future of the Obsidian plugin
- **Question:** Retire, keep as Weave-only, or rebuild as an Amaru integration?
- **Owner answer:** "Obsidian plugin is retired."
- **Decision:** Retired; no Amaru Obsidian client; FEAT-CLI-013 dropped. → [ADR-0031](adr/0031-obsidian-plugin-is-retired.md)

### Q19. Migration source branch
- **Question:** Which `weave-obsidian` commit is the migration baseline?
- **Owner answer:** "`feat/orkestra-dofgook`, I think this is the one that holds the latest commit." (The branch is `feat/orkestra-dogfood`; verified head `c8a2f6e`, 84 commits ahead of `origin/main`.)
- **Decision:** Baseline `feat/orkestra-dogfood` at `c8a2f6e`; Weave findings F-27, F-39, F-40, F-41 become Amaru acceptance tests instead of Weave prerequisites. → [ADR-0032](adr/0032-migration-baseline-is-orkestra-dogfood-c8a2f6e.md)

### Q20. Cost and usage telemetry model
- **Question:** How usage and cost are captured, how unknowns are represented and how cost is attributed.
- **Owner answer:** "You can decide."
- **Decision:** Usage records per Step Execution with nullable values and `reported`/`estimated`/`unknown` provenance, versioned price table, roll-up to Work Items including failed attempts. → [ADR-0033](adr/0033-usage-and-cost-telemetry-model.md)

### Q21. Local Web client authentication
- **Question:** How a browser tab authenticates to the loopback Platform safely.
- **Owner answer:** "You can decide."
- **Decision:** One-time launch code exchanged for an HttpOnly SameSite=Strict session cookie; Host allowlist against DNS rebinding; Origin and CSRF checks; per-OS-user instance. → [ADR-0034](adr/0034-local-web-client-authentication-with-launch-code-and-cookie.md)

### Q22. Agent placement relative to Environment-hosted tools
- **Question:** Does the agent run on the Worker next to the tools, or elsewhere with MCP proxied?
- **Owner answer:** "You can decide."
- **Decision:** On the Worker, beside the tools, behind a Worker-side MCP gateway that enforces Policy and audits calls; editor Environments leased exclusively. Epic's UE 5.8 MCP plugin is loopback-only and unauthenticated (verified 2026-10-25). → [ADR-0035](adr/0035-agents-run-beside-environment-hosted-tools.md)

### Carried over from Weave
These came from [`weave-obsidian` `docs/ROADMAP.md`](https://github.com/rluders/weave-obsidian/blob/main/docs/ROADMAP.md). No product-owner answer was recorded; they were resolved under the session's delegation.

### W1. Approval model
- **Question:** Who approves, how an Approval relates to Work Item status, what Evidence it binds to (Weave Q2).
- **Decision:** Human-only, role-authorized Approvals bound to specific Evidence, invalidated when Evidence changes; Work Item "awaiting approval" is derived. → [ADR-0036](adr/0036-approvals-bind-to-evidence-and-gate-work.md)

### W2. Capability permission model
- **Question:** How requested Capabilities, Grants and Approvals form one Policy (Weave GAP-09).
- **Decision:** One Policy function returning allow / require_approval / deny; requests are never authority; deny wins; missing authority parks the item. → [ADR-0037](adr/0037-one-policy-evaluates-capabilities-grants-and-approvals.md)

### W3. Teams
- **Question:** Roles and enforced separation of duties across humans and Agents (Weave ADR-019).
- **Decision:** Fixed Organization roles, Teams of humans and Agents, per-gate separation of duties, required reviewer Teams; Agents never approve. → [ADR-0038](adr/0038-roles-teams-and-separation-of-duties.md)

### W4. Configuration scopes
- **Question:** Which settings are deployment, Organization, user or Project level, and the resolution order (Weave Q8).
- **Decision:** Deployment < Organization < Project (`.amaru/config.yaml`) < user preferences, with locks from higher scopes and no secrets in Project configuration. → [ADR-0039](adr/0039-configuration-scopes-and-resolution-order.md)

### Second pass (2026-10-25)
Implicit questions found in the planning documents during the second sweep. The owner delegated each one in the planning prompt ("make the most defensible decision"), and Claude decided them. Each entry gives the question, the decision and where the decision now lives.

### Q23. Where plugin logic executes
- **Question:** ADR-0019 and ADR-0020 ran Wasm `logic` inside the Platform, but ADR-0009, the security model, NFR-18 and the PRD said plugin hosts are always separate processes. Which one holds?
- **Decision:** A separate `amaru plugin-host` process, one per Organization, supervised by the Platform. Native or third-party code never runs in the Platform. → [ADR-0040](adr/0040-plugin-logic-runs-in-a-separate-plugin-host-process-built-with-javy.md), [ADR-0041](adr/0041-isolation-tiers-for-plugins-providers-and-steps.md)

### Q24. Plugin logic build toolchain
- **Question:** Javy or the Extism JS PDK? ADR-0020 deferred this to the M6 spike.
- **Decision:** Javy with an Amaru Javy plugin. It produces a module with no WASI imports and only Amaru host calls. The Extism JS PDK needs the Extism kernel ABI, which ADR-0020 already rejected (verified 2026-10-25). → [ADR-0040](adr/0040-plugin-logic-runs-in-a-separate-plugin-host-process-built-with-javy.md); validation A10

### Q25. Container granularity and Environment sandbox class
- **Question:** ADR-0019 said "per Step (or per Lease)". Who decides whether an Environment is sandboxed?
- **Decision:** One container per Lease (per Step attempt), never reused. The Environment sandbox class is `container` or `none`, set by an Owner or Admin. Default: `container` on servers and Cloud, `none` on workstations. Policy can require `container`. → [ADR-0041](adr/0041-isolation-tiers-for-plugins-providers-and-steps.md); validation A13

### Q26. Restart semantics for non-resumable Steps
- **Question:** ADR-0014 said these Steps "follow their retry Policy", but the resilience view says they end `interrupted`. Which applies?
- **Decision:** Waiting Steps keep waiting. Steps with a `Resumable` remote job resume. Idempotent Steps retry as a new Lease. Every other Step ends `interrupted` and is never re-run automatically. A lost Worker follows the same rule, so there is no separate `lost` state. → [ADR-0042](adr/0042-durable-execution-with-explicit-restart-semantics.md)

### Q27. Where Teams live
- **Question:** ADR-0027 committed Teams to Git, but ADR-0038 Teams gate Approvals. With Teams in Git, repository write access would grant approval authority.
- **Decision:** Teams are Organization-scoped Platform state. Definitions reference them by slug. → [ADR-0043](adr/0043-project-definitions-in-git-teams-and-operational-state-in-platform.md)

### Q28. PostgreSQL supervisor library
- **Question:** ADR-0015 deferred the choice between `embedded-postgres` and an in-house supervisor to the M1 spike.
- **Decision:** `fergusstrange/embedded-postgres` v2 (MIT, no third-party dependencies). It uses `LocalProvider` over the installer-bundled binaries, a persistent data directory, and its owner-pipe supervisor to stop PostgreSQL if the Platform dies (README verified 2026-10-25). Fallback: an in-house supervisor. → [TECHNOLOGY_BENCHMARK §3](TECHNOLOGY_BENCHMARK.md#3-persistence-and-object-storage), WP-M1-02; validation A12

### Q29. Compact PostgreSQL listener
- **Question:** ADR-0015 said "Unix socket or a random loopback port". Which one, on which OS?
- **Decision:** A Unix socket in the per-user state directory on Linux and macOS. On Windows, a random loopback port with a generated SCRAM password kept in the user-only state file. → [DATA_ARCHITECTURE §4](DATA_ARCHITECTURE.md#4-consistency-rules), [DEPLOYMENT](DEPLOYMENT.md)

### Q30. Fallback if bundled PostgreSQL fails
- **Question:** The roadmap named an installer-managed service as the fallback, while the register named a "documented prerequisite".
- **Decision:** One fallback: an installer-managed PostgreSQL service on the affected OS. It needs one-time admin consent and the user operates nothing. Adopting it requires a superseding ADR. → validation A2, [ROADMAP M1](ROADMAP.md#m1-decisions-and-spikes)

### Q31. TypeScript SDK generator
- **Question:** WP-M1-03 left the generator open.
- **Decision:** `openapi-typescript` with `openapi-fetch` (MIT, OpenAPI 3.1, runtime-free types; verified 2026-10-25). The event-stream client is hand-written on the generated types. → [TECHNOLOGY_BENCHMARK §8](TECHNOLOGY_BENCHMARK.md#contracts-typescript-sdk-generation), FEAT-API-003

### Q32. Provider protocol name and Weave compatibility
- **Question:** The glossary called `weave.provider/v1` a "candidate", and FEAT-CAP-003 asked for a decision on a compatibility shim.
- **Decision:** `amaru.provider/v1`, adapted from `weave.provider/v1` in M2. No shim: Weave Providers are ported. → [GLOSSARY](../product/GLOSSARY.md), FEAT-CAP-003

### Q33. Contract versions before the first release
- **Question:** The roadmap kept the contract at `0.x` until M4, while ADR-0025 uses `/v1` with additive-only changes.
- **Decision:** The URL major is `/v1` from M2. The contract package stays `0.x` and may break until the Local MVP release. It becomes `1.0.0` at M4, and ADR-0025's additive-only rule applies from then on. → [ROADMAP M2](ROADMAP.md#m2-contracts-v0), WP-M1-03

### Q34. Product name and trademark
- **Question:** The name "Amaru" waits on a trademark search.
- **Decision:** The product is named Amaru. The search is an owner validation task before the first public release. A conflict leads to a rename through an ADR. Preliminary search 2026-10-26 (not a clearance; official registers not searched; overlaps found): [name search](investigations/2026-10-26-amaru-name-search.md). Owner decision pending. → validation A11, WP-M1-01

### Q35. Desktop sidecar bundling
- **Question:** ADR-0018 bundles the Platform as a sidecar, but the architecture and client views called local supervision "optional".
- **Decision:** Desktop always bundles the sidecar and starts it only when the user runs a local Instance. A Desktop connected only to remote Instances never starts it. → [ARCHITECTURE §5](ARCHITECTURE.md#5-desktop-and-web-clients), [CLIENTS §3](CLIENTS.md#3-desktop-client-m8)

### Q36. Approval as a Step type or a Step state
- **Question:** The domain model said human approvals are Steps, but it also had a Step state `awaiting_approval`.
- **Decision:** Both, with distinct roles. `approval` is a Step type. `awaiting_approval` is the single waiting state, used both by Approval Steps and by Policy-required approvals before any Step starts. → [DOMAIN_MODEL §5.2](DOMAIN_MODEL.md#52-step-execution)

### Q37. Work Item identifiers
- **Question:** How are Work Items identified across Projects and Instances (FEAT-WRK-001 risk)?
- **Decision:** A ULID internally and a display key `<PROJECTKEY>-<n>` unique per Instance. Links across Instances use the Instance URL plus the ULID. Imported Weave `WI-001` numbers are kept as aliases. → [DOMAIN_MODEL §3](DOMAIN_MODEL.md#3-entities-and-responsibilities), FEAT-WRK-001

### Q38. Workflow version storage, two clones and Cloud
- **Question:** Where are old Workflow versions kept once definitions live in Git? How do two clones and Cloud read them?
- **Decision:**
  - Versions are the Git history of `.amaru/`.
  - Each Execution stores the commit SHA, the content hash and an immutable snapshot of the definitions it ran, so resuming never depends on Git.
  - A Project is keyed by repository, and each Execution records which clone ran it.
  - Cloud fetches definitions at a commit.
  → FEAT-WFL-002, [DATA_ARCHITECTURE §1](DATA_ARCHITECTURE.md#1-data-classes), WP-M1-05

### Q39. Evidence ordering
- **Question:** How is Evidence ordered across parallel branches and remote Workers (FEAT-WFL-006 risk)?
- **Decision:** By the per-Execution Event `seq` the Platform assigns, never by Worker clocks. → FEAT-WFL-006, [DOMAIN_MODEL §3](DOMAIN_MODEL.md#3-entities-and-responsibilities)

### Q40. Repository credential scope
- **Question:** What credential does a linked repository use (FEAT-PRJ-004 risk)?
- **Decision:** One scoped credential per repository: a GitHub App installation token or a per-repository deploy key. It is delivered per Lease and never stored on the Worker. Hosts without per-repository tokens fall back to a user token, and the fallback is flagged in Evidence. → FEAT-PRJ-004

### Q41. Agent identity
- **Question:** What identity does an Agent have (FEAT-AGT-005 risk "identity model")?
- **Decision:**
  - Agents are Organization principals without a login.
  - They act only through Executions and are attributed to a human sponsor.
  - They never hold credentials or an Organization role, and never approve.
  → [DOMAIN_MODEL §7](DOMAIN_MODEL.md#7-resolved-domain-questions), FEAT-AGT-005

### Q42. Default consequential actions
- **Question:** Which actions need an Approval by default (WP-M1-04 "consequential-action list")?
- **Decision:** Without a standing Grant, these return `require_approval`:
  - `git.push` to a shared remote
  - merging a pull request
  - deploys
  - writes to external trackers
  - egress outside the Organization allowlist
  - deleting files outside the worktree
  - installing or upgrading a plugin
  - a Capability's first use of a secret

  Only an Owner or Admin can lower these, by Grant. → [SECURITY_MODEL rule 12](../security/SECURITY_MODEL.md#baseline-rules)

### Q43. Retention and redaction defaults
- **Question:** The architecture said retention and redaction "must be designed before the MVP", and ADR-0017 and the data view disagreed on scope.
- **Decision:** Retention is an Organization Policy setting per data class. Defaults:
  - audit records: kept indefinitely (Cloud: at least 1 year)
  - logs: 30 days
  - intermediate Artifacts: 90 days
  - Artifacts referenced by an Approval or an accepted Work Item: kept while referenced
  - event replay window: 7 days compact, 30 days server and Cloud

  Per-Lease secret values are redacted on the Worker, then patterns are scanned again on the Platform. → [DATA_ARCHITECTURE §5](DATA_ARCHITECTURE.md#5-retention-and-redaction); validation A14

### Q44. Initial numeric operating targets
- **Question:** Several NFRs and resilience rules said "set later": Lease and heartbeat periods, reconcile interval, timeouts, slots, latency, scale, availability and installer size.
- **Decision:** Initial targets, each with the milestone that validates it:
  - heartbeat 10 s, Lease 30 s (M5)
  - reconcile 30 s
  - Step timeouts: agent 60 min, tool or check 30 min, Organization maximum 8 h
  - slots: global, per Project (default 2) and per Environment, checked before publish so fairness does not depend on JetStream
  - Step transition overhead p95 < 50 ms; local API p95 < 200 ms; first UI load < 2 s (M4)
  - 50 users, 20 Workers and 20 concurrent Steps per server instance (M7)
  - 99.9 % Cloud API availability (M9)
  - installer increment ≤ 120 MB, idle memory ≤ 300 MB (M1)
  → [NFRs](../product/NON_FUNCTIONAL_REQUIREMENTS.md), [RESILIENCE §2](RESILIENCE.md#2-requirements); validations A14, A15

### Q45. Local distribution form
- **Question:** The local release was described as "archive or installer".
- **Decision:** Signed per-OS archives from M4, with no admin rights needed. Native installers ship with Desktop at M8. → [DEPLOYMENT §2](DEPLOYMENT.md#2-operations), FEAT-OPS-001

### Q46. Local command-line scope and `amaru backup`
- **Question:** The CLI was planned for M7, but launch codes, migrations, backup and import are needed earlier. ADR-0023 says Amaru ships no backup system.
- **Decision:**
  - The Platform binary ships local operator subcommands from M3/M4: `open`, `migrate`, `backup`, `import weave`, `admin check`, `worker` and `plugin-host`.
  - FEAT-CLI-012 at M7 is the remote API client.
  - `amaru backup` is supported for compact installs only; server backup stays with the operator.
  → [DEPLOYMENT §4](DEPLOYMENT.md#4-resolved-deployment-points), FEAT-CLI-012

### Q47. Self-approval rule
- **Question:** FR-44 and FEAT-REV-007 said "an actor cannot approve its own work", while ADR-0038 makes this per gate.
- **Decision:** ADR-0038 applies:
  - The rule is configured per gate: the approver differs from the initiator and from the Agent's human sponsor.
  - Agents never approve.
  - The rule is disabled in single-user compact installs, and the disabling is recorded.
  → PRD FR-44, FEAT-REV-007

### Q48. External tracker items before Signals, and Artifact paths
- **Question:** FR-3 sent inbound tracker items to Signals, which are unscheduled (P2). FR-30 still spoke of project-relative Artifact paths.
- **Decision:**
  - Inbound tracker items create linked Work Items until Signals ship; after that they land as Signals for triage.
  - Artifacts are content-addressed. Output reaches the Project tree only through an explicit Step.
  → [PRD §13](../product/PRD.md#13-assumptions-risks-open-questions), FR-30

### Q49. Engine features versus P2 features
- **Question:** The Knowledge index, Approval expiry and schedules are treated as core in some views but are P2 in the catalog. Compensation was "later".
- **Decision:**
  - The engine supports Approval `expired` and timer rows from M3. Policy-configured expiry (FEAT-REV-008) and schedule triggers (FEAT-WFL-010) stay P2.
  - Knowledge references resolve by path at a commit until the index (FEAT-PRJ-007, P2) ships.
  - There is no saga compensation: failure edges route to explicit cleanup Steps.
  → [DOMAIN_MODEL §7](DOMAIN_MODEL.md#7-resolved-domain-questions), [RESILIENCE](RESILIENCE.md), PRD FR-14

### Q50. Secrets scope versus encryption key
- **Question:** "Per-Project and per-user" secrets in the security model and catalog seemed to contradict ADR-0022's per-Organization key.
- **Decision:** A secret is visible at Organization or Project scope, and every secret is encrypted with the per-Organization data key. User credentials for clients live in the OS keychain (ADR-0028), not in the secrets port. → [SECURITY_MODEL](../security/SECURITY_MODEL.md#trust-boundaries), FEAT-POL-007

### Q51. Unreal MCP server choice
- **Question:** FEAT-ENV-009 used a community server "only if" Epic's plugin is unavailable, which left the rule unclear.
- **Decision:** Epic's Unreal MCP plugin for engine versions that ship it (UE 5.8+). A community server only for older engine versions, behind the same Worker-side gateway. → [EXECUTION_ENVIRONMENTS §4](EXECUTION_ENVIRONMENTS.md#4-environment-hosted-tools-and-mcp)

### Q52. Skill context budget
- **Question:** FEAT-SKL-004 had no default budget, and its benefit was "hard to measure".
- **Decision:** 8,000 tokens of Skills and Knowledge per Agent Step by default, configurable per Agent. The benefit is measured as cost per accepted change from usage records (ADR-0033). → FEAT-SKL-004

### Q53. Shared UI package timing
- **Question:** The client view said package names are "indicative" and packages "may start inside `apps/web`".
- **Decision:** UI code starts inside `apps/web` at M4. `packages/app` and `packages/ui` are extracted at M8, when Desktop is their second consumer. → [CLIENTS §1](CLIENTS.md)

### Q54. Weave migration details
- **Question:** The migration plan left these open:
  - whether migrated components are frozen
  - history-preserving import versus fresh import
  - what happens to media providers
  - how Obsidian plugin code (SDK reconnect logic, React Flow editor) is treated after ADR-0031
  - how `.weave/hooks.yaml` maps
  - whether its dispositions were "hypotheses"
- **Decision:**
  - Components are frozen in Weave when they migrate.
  - Default to a fresh import with provenance; a migration ADR may choose history-preserving import with a reason.
  - Pandoc and ffmpeg become M6 example plugins. The other media providers are retired.
  - Obsidian plugin code is reimplemented with the old code as a behavioral reference.
  - Inbound hooks become Workflow webhook Triggers, and outbound hooks become Project webhook configuration. Secrets and consent are re-entered by a human.
  - The dispositions are the plan of record. Each migration ADR (ADR-0007) confirms its own.
  → [MIGRATION_PLAN](../migration/MIGRATION_PLAN.md)
