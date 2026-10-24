# Open Architectural Questions

Unresolved questions that block or shape decisions. Each lists **why it matters**, **evidence required** to decide, and **what it blocks**. A question is closed by an ADR ([DECISIONS](DECISIONS.md)); move it to the "Closed" section with a link, do not delete it.

Nothing here is decided. Candidate technologies are named only to scope the investigation.

## Questions

### Q1. Durable workflow technology
- **Question:** Keep a hand-written durable state machine (Weave's model: persisted Step states, explicit restart semantics, Resumable remote jobs), or adopt a durable execution engine (Temporal, or alternatives such as Restate, DBOS, Hatchet, or a database-backed queue)?
- **Why it matters:** Determines how long-running Workflows, Approval waits, retries and timers survive failures, and how heavy self-hosted and local installs become.
- **Evidence required:** (1) Weave's restart/resume behavior and its known gaps (fsync of non-terminal events, schema migration — Weave GAP-07); (2) for each candidate: local single-binary story on Windows/macOS/Linux, operational footprint for self-hosting, licensing, Go SDK maturity, support for multi-day human waits, testability without the server; (3) a spike running one Weave-shaped Workflow (agent Step + Approval wait + remote job) on the top candidate.
- **Blocks:** Platform persistence design (Q3), Worker protocol (Q10), Cloud architecture (Q8).

### Q2. Queue and messaging strategy
- **Question:** Does the Platform need a message broker (NATS, RabbitMQ, a cloud queue) or a database-backed work queue, and from which deployment onward?
- **Why it matters:** Remote Workers and multi-instance Platforms need reliable Step assignment; a broker is an extra service for every self-hosted install.
- **Evidence required:** Expected Step throughput and fan-out; whether Q1's choice already provides task queues; delivery semantics needed (at-least-once + idempotent Steps vs exactly-once); operational cost per deployment model.
- **Blocks:** Remote Workers (Q10), multi-instance Platform, Cloud.

### Q3. Database and persistence strategy
- **Question:** Which storage per deployment: files (Weave), embedded SQL (SQLite), server SQL (PostgreSQL)? One engine everywhere or adapters per deployment?
- **Why it matters:** Query needs (audit, search across Executions), concurrency (multi-user), backup/restore, and migration effort for existing Weave data.
- **Evidence required:** Data volume and query patterns from Weave usage; schema migration story; behavior on Windows file locking; whether Q1's engine owns part of the state; cost of maintaining two adapters.
- **Blocks:** Contracts stage (persisted shapes), self-hosted (Q9), migration of Weave data.

### Q4. Artifact and object-storage strategy
- **Question:** Where do generated and remote Artifacts live: in the Project tree (Weave), a Platform-managed directory, or object storage (S3-compatible)? How are large binary outputs from remote Workers transferred?
- **Why it matters:** Users' repositories must not fill with generated binaries by surprise; remote Workers need somewhere to put outputs; audit needs immutable hashes.
- **Evidence required:** Artifact sizes from Weave providers (meshes, audio, images); retention needs; whether Git LFS or the user's own storage should be an option; local-only operation without any object store.
- **Blocks:** Remote Workers (Q10), Cloud (Q8).

### Q5. Desktop framework
- **Question:** Is Tauri v2 with a Rust host the right Desktop shell, versus Electron, Wails (Go), or Web-only for the first release?
- **Why it matters:** Bundle size, security model, OS integration (tray, notifications, auto-update, process supervision of the local Platform), the language of native code, and release pipeline cost.
- **Evidence required (official docs + a spike):** Tauri v2 capability/permission model and IPC; sidecar support for supervising the Go Platform binary; webview differences (WebView2 / WKWebView / WebKitGTK) for the React UI; code signing and updater on all three OSes; whether any Rust code beyond the shell is needed. Rust/cargo 1.97 is available in the bootstrap environment; the Tauri CLI is not installed and nothing has been tried.
- **Blocks:** Desktop stage, `apps/desktop` scaffold, client-first decision (PRD FR-41).

### Q6. Plugin sandboxing and execution isolation
- **Question:** How are plugin code and Provider processes isolated: out-of-process only (Weave's external providers), WebAssembly, OS sandboxes, containers in server deployments?
- **Why it matters:** Plugins run on users' machines next to their code and secrets; Cloud must isolate tenants' plugins.
- **Evidence required:** Threat model per extension kind ([ARCHITECTURE §6](ARCHITECTURE.md#6-plugin-and-extension-model)); what OS sandboxing is practical on Windows/macOS/Linux without admin rights; how UI extensions are isolated inside the client (iframe/worker + API-only access).
- **Blocks:** Plugin platform stage.

### Q7. Plugin distribution, signing, permissions and compatibility
- **Question:** How are plugins packaged, signed, discovered, granted permissions and checked for compatibility with Platform/contract versions?
- **Why it matters:** Supply-chain risk; users must know what a plugin can do before enabling it; upgrades must not break Workflows silently.
- **Evidence required:** Manifest design draft; signing options (Sigstore or vendor keys); compatibility ranges against contract versions (Q11); lessons from Weave's per-user provider install directory.
- **Blocks:** Plugin platform stage, any marketplace (out of scope until then).

### Q8. Cloud tenant isolation
- **Question:** What is the tenancy model (shared Platform with row-level isolation, per-tenant instances, per-tenant Workers) and how are secrets and Artifacts isolated?
- **Why it matters:** Security and compliance of the managed offering; cost model.
- **Evidence required:** Q1/Q3/Q4 decisions; threat model for cross-tenant access; whether customer-run Workers are mandatory for code access.
- **Blocks:** Cloud stage only. Does not block the local MVP.

### Q9. Self-hosted installation, upgrades, backup and recovery
- **Question:** What does an operator install (single binary, container image, Helm chart), and how are upgrades, schema migrations, backup and restore done?
- **Why it matters:** Self-hosted is a product surface, not an afterthought; every infrastructure dependency added in Q1–Q4 multiplies operator burden.
- **Evidence required:** Chosen dependencies; migration tooling; a restore drill on a test deployment.
- **Blocks:** Self-hosted stage.

### Q10. Remote execution environments
- **Question:** How do Workers on other machines (including Windows/GPU workstations) register, authenticate, obtain the right project revision, receive secrets, run interactive tools, and report back?
- **Why it matters:** A core product promise (PRD use case 3), with security and reliability risk.
- **Evidence required:** Worker protocol draft derived from Weave's Provider contract (`Resumable`, `Detachable`, `weave.provider/v1`); project sync options (Git checkout at a commit vs file sync); Windows process-tree termination (unvalidated in Weave, GAP-05); a spike with one remote Worker on a second machine.
- **Blocks:** Remote Workers, Cloud with customer-run Workers.

### Q11. API compatibility and versioning
- **Question:** Keep Weave's path-versioned `/v1` with additive-only changes? How are the client SDK, provider protocol and Worker protocol versioned relative to each other? What event transport for remote clients (WebSocket, SSE — rejected in Weave for local, HTTP long-poll)?
- **Why it matters:** Clients, plugins and Workers upgrade independently; Cloud and self-hosted run different versions.
- **Evidence required:** Weave protocol history (it stayed `/v1`, changed in place, before any release); compatibility policy draft; SDK generation tooling choice.
- **Blocks:** Contracts stage.

### Q12. Licensing and commercial boundaries
- **Question:** Which license for the community edition, and what is commercially licensed? The stated direction is a **source-available** community license with separate commercial licensing.
- **Why it matters:** Blocks accepting external contributions, publishing releases, and migrating Weave code (MIT, © 2026 Ricardo Lüders — the copyright holder must consent to relicensing; third-party contributions, if any, need checking).
- **Evidence required:** Legal review of candidates (for example FSL-1.1, BUSL-1.1, Elastic License 2.0 — none of which is OSI-approved open source); contributor agreement need (CLA/DCO); which components (SDK, contracts, plugin SDK) may need a more permissive license to grow an ecosystem; trademark status of the name "Amaru" (not verified).
- **Blocks:** First public release, external contributions, migration of Weave code.
- **Current state:** no LICENSE file in this repository; all rights reserved until decided.

### Carried over from Weave
These are open in [`weave-obsidian` `docs/ROADMAP.md`](https://github.com/rluders/weave-obsidian/blob/main/docs/ROADMAP.md) and are product-shaping for Amaru:
- **W1. Approval model** (Weave Q2): who approves, how Approval relates to Work Item status, separation of duties. Required for PRD FR-13 / ADR-0006.
- **W2. Capability permission model** (Weave GAP-09): how `requestedCapabilities` becomes enforced Policy. Required for PRD FR-24.
- **W3. Teams** (Weave ADR-019, proposed): roles and Runtime-enforced separation of duties.
- **W4. Configuration scopes** (Weave Q8): which settings are project, user or deployment level, and the resolution order.

## Investigation plan for the architecture study

Focused, evidence-first; not a full benchmark ([ROADMAP stage 2](ROADMAP.md#stage-2--architecture-and-technology-study)).

1. **Inventory first**: confirm the Weave behaviors Amaru must keep (from [weave-inventory](../migration/weave-inventory.md)) and turn them into acceptance tests the new design must pass.
2. **Q1 + Q3 together**: one spike per shortlisted option (at most two) running the representative Workflow from [ARCHITECTURE §15](ARCHITECTURE.md#15-representative-workflow-execution) with a forced crash during each Step state. Record install footprint per OS.
3. **Q5**: a Tauri v2 spike that supervises a dummy Go binary as a sidecar and renders a React page calling it over loopback HTTP; verify against official Tauri v2 docs; record signing/updater requirements.
4. **Q10**: draft the Worker protocol from the Weave Provider contract; spike one remote Worker.
5. **Q11**: write the compatibility policy before any new endpoint.
6. **Q12**: legal decision; not an engineering spike.

Each spike ends with a short report in `docs/architecture/investigations/` and, if conclusive, a proposed ADR.

## Closed

None yet.
