# Initial Work Packages

Status: **accepted** (2026-10-25). This is the decomposition of milestones M0–M4 of the [master roadmap](ROADMAP.md) into Work Packages (WP), the units to plan in subsequent engineering cycles. Later milestones stay at epic level in the roadmap until M4 is closer. Features are catalog IDs without the `FEAT-` prefix ([catalog](../product/FEATURE_CATALOG.md)).

**Conventions**
- IDs follow `WP-M<milestone>-<nn>` and are stable.
- Each WP links its GitHub issue ([backlog map](../engineering/backlog-map.md), [delivery workflow](../engineering/delivery.md)) and lists:
  - **Goal**
  - **Scope**
  - **Features / decisions**
  - **Depends on**
  - **Done when** (acceptance)
  - **Validation**
- *prototype* means the earlier internal Runtime; *earlier prototype* means a second, earlier internal prototype.
- WPs that move code require a migration ADR first (migration readiness criteria, [ADR-0007](adr/0007-reuse-of-earlier-internal-code-is-a-separate-activity.md)).

## M0 Foundation reconciled

### WP-M0-01 Planning package review — done 2026-10-25
- **Issue:** [#1](https://github.com/rluders/amaru/issues/1)
- **Goal:** product-owner review of this planning package.
- **Scope:** PRD, catalog, architecture views, roadmap and migration documents; every open question answered or delegated; every pending ADR decided.
- **Depends on:** —
- **Done when:** ADR statuses are updated with recorded approvals (done: ADRs 0002–0005 and 0008–0039 accepted); [OPEN_QUESTIONS](OPEN_QUESTIONS.md) has no open question (done).
- **Validation:** `make check`.

## M1 Decisions and spikes

Decisions are made; these Work Packages validate them and put them into effect.

### WP-M1-01 License texts in effect
- **Issue:** [#2](https://github.com/rluders/amaru/issues/2)
- **Goal:** apply [ADR-0026](adr/0026-fair-code-community-license-with-commercial-licensing.md).
- **Scope:**
  - Record the license state: decided in ADR-0026 and stated in the [README](../../README.md#license); no separate legal review of the draft (product-owner decision, 2026-10-26; [#65](https://github.com/rluders/amaru/issues/65) closed).
  - CLA text and signing tooling.
  - Trademark search for "Amaru".
  - Vulnerability disclosure policy (`SECURITY.md`).
- **Depends on:** WP-M0-01.
- **Done when:** the license state is recorded as above and the remaining sub-issues are done. The root `LICENSE` ([#66](https://github.com/rluders/amaru/issues/66), product owner; agents never add license text) is not part of this Work Package and does not gate migration: ADR-0026 already resolves the relicensing of the prototype code (sole copyright holder), and the file was added by the product owner's instruction on 2026-10-26 (README). The Apache-2.0 `LICENSE` of each contract or SDK package is added by the product owner in the Work Package or epic that creates that package (WP-M2-01, WP-M2-02, E6.3, and E6.6 for the Go Worker SDK), because those directories do not exist in M1 ([repository layout](../engineering/repository-layout.md)).
- **Validation:** owner decision recorded (this entry and the closing comment of #65).
- **Status (2026-10-26):** sub-issues done. CLA ([CLA.md](../../CLA.md), manual signing record), `SECURITY.md` (reporting channel enabled when the repository goes public) and the name decision ([name search](investigations/2026-10-26-amaru-name-search.md); official registers not searched) were approved by the product owner. The root `LICENSE` ([#66](https://github.com/rluders/amaru/issues/66)) was added on 2026-10-26 at the product owner's instruction; package licenses remain owner actions.

### WP-M1-02 Storage and execution validation spike
- **Issue:** [#3](https://github.com/rluders/amaru/issues/3)
- **Goal:** validate [ADR-0042](adr/0042-durable-execution-with-explicit-restart-semantics.md), [ADR-0015](adr/0015-postgresql-is-the-only-database-bundled-in-compact-installs.md) and [ADR-0016](adr/0016-nats-jetstream-message-bus-embedded-in-compact-installs.md) (checks A2, A9).
- **Scope:**
  - Bundle and supervise PostgreSQL on Windows (non-admin), macOS (signed child binary) and Linux; supervise with `embedded-postgres` v2 (local binary provider over the bundled binaries, persistent data directory, owner-pipe supervision); Unix socket on Linux/macOS, loopback port + SCRAM password on Windows. Record whether A12 holds; if not, the in-house supervisor fallback needs a note in TECHNOLOGY_BENCHMARK.
  - Embed NATS JetStream in the same process; outbox relay with `Nats-Msg-Id`.
  - Run the representative Workflow with crash injection at every Step state and between commit/publish/ack, a multi-day Approval wait under clock control, timeouts and leases.
- **Depends on:** WP-M0-01.
- **Done when:** a report is in `investigations/` with footprint per OS and crash results; any failed check has a superseding ADR.
- **Validation:** spike test logs.

### WP-M1-03 Contract versioning rules in fixtures
- **Issue:** [#4](https://github.com/rluders/amaru/issues/4)
- **Goal:** apply [ADR-0025](adr/0025-api-and-protocol-versioning-policy.md) before any new endpoint.
- **Scope:** fixture conventions for additive changes; `buf breaking` configuration for Protobuf packages; deprecation headers; contract package `0.x` until the Local MVP release (1.0.0 at M4), URL major `/v1` from M2; TypeScript generation with `openapi-typescript` + `openapi-fetch` (pinned at M2).
- **Depends on:** WP-M0-01.
- **Done when:** the conventions are in `packages/contracts` docs when that package is created (M2) and the generator pipeline is configured.
- **Validation:** review against the prototype protocol history.
- **Deliverables:** [CONTRACT_VERSIONING](CONTRACT_VERSIONING.md) (proposed design; no implementation).

### WP-M1-04 Approval, Policy, roles and configuration shapes
- **Issue:** [#5](https://github.com/rluders/amaru/issues/5)
- **Goal:** contract shapes for [ADR-0036](adr/0036-approvals-bind-to-evidence-and-gate-work.md), [ADR-0037](adr/0037-one-policy-evaluates-capabilities-grants-and-approvals.md), [ADR-0038](adr/0038-roles-teams-and-separation-of-duties.md) and [ADR-0039](adr/0039-configuration-scopes-and-resolution-order.md).
- **Scope:**
  - Approval record and Evidence binding; invalidation.
  - Policy evaluation inputs/outputs; the default consequential-action list ([SECURITY_MODEL rule 12](../security/SECURITY_MODEL.md#baseline-rules)) mapped onto Capabilities; the `idempotent` flag per Capability ([ADR-0042](adr/0042-durable-execution-with-explicit-restart-semantics.md)).
  - Trigger paths: webhook, schedule, delivery loop.
  - Configuration keys per scope and lock rules.
- **Depends on:** WP-M0-01.
- **Done when:** shapes are drafted for M2.
- **Validation:** walkthrough of UC-1 and UC-5.
- **Deliverables:** [POLICY_AND_APPROVAL_SHAPES](POLICY_AND_APPROVAL_SHAPES.md) (Proposed draft; evidence level: not validated).

### WP-M1-05 `.amaru/` layout and importer design
- **Issue:** [#6](https://github.com/rluders/amaru/issues/6)
- **Goal:** apply [ADR-0043](adr/0043-project-definitions-in-git-teams-and-operational-state-in-platform.md).
- **Scope:**
  - File layout and schemas under `.amaru/`.
  - Two-clone and Cloud behavior as decided in [ADR-0043](adr/0043-project-definitions-in-git-teams-and-operational-state-in-platform.md): a Project is keyed by repository; each Execution records which clone, the commit SHA, the content hash and a definition snapshot; Cloud fetches at a commit.
  - Team references by slug and their validation.
  - Design of the one-shot `amaru import legacy` (definitions and Work Items).
- **Depends on:** WP-M1-02.
- **Done when:** layout and importer design are documented.
- **Validation:** data-class review against [DATA_ARCHITECTURE](DATA_ARCHITECTURE.md).
- **Deliverables (Proposed, not validated):** [AMARU_DIRECTORY](AMARU_DIRECTORY.md) (layout, schemas, Project/clone/Execution provenance, data-class review) and [LEGACY_IMPORTER](LEGACY_IMPORTER.md) (mapping, conflicts, idempotency, recovery). Open decision: [Q55](OPEN_QUESTIONS.md#q55-project-key-and-linked-repository-placement).

### WP-M1-06 Worker protocol schemas
- **Issue:** [#7](https://github.com/rluders/amaru/issues/7)
- **Goal:** `amaru.worker.v1` Protobuf schemas for M2 contracts ([ADR-0029](adr/0029-worker-protocol-over-websocket-and-protobuf.md), [ADR-0024](adr/0024-remote-worker-registration-identity-and-delivery.md)). Implementation is in M5.
- **Scope:**
  - Messages from [EXECUTION_ENVIRONMENTS §3](EXECUTION_ENVIRONMENTS.md#3-worker-lifecycle-and-protocol-draft): handshake, enrollment challenge, offer, assignment, lease, events, result, cancel, secret delivery.
  - In-process transport for the embedded Worker.
  - Idempotency and lease fields.
- **Depends on:** WP-M1-02.
- **Done when:** schemas and fixtures are reviewed.
- **Validation:** review; `buf lint`.
- **Deliverables (Proposed, not validated):** [WORKER_PROTOCOL](WORKER_PROTOCOL.md): schema draft (`buf lint` clean on a scratch copy), fixtures, in-process transport, idempotency and lease fields.

### WP-M1-07 Inventory refresh at the baseline
- **Issue:** [#8](https://github.com/rluders/amaru/issues/8)
- **Goal:** a refreshed Amaru inventory at the pinned baseline ([ADR-0032](adr/0032-migration-baseline-is-a-pinned-commit.md)).
- **Scope:**
  - Refresh the prototype evidence inventory to the baseline commit.
  - Write prototype findings F-27, F-39, F-40, F-41 as Amaru acceptance tests (specifications for M3/M4).
- **Depends on:** —
- **Done when:** the inventory names the baseline commit and test counts were obtained by running `make check` there.
- **Validation:** prototype test run output at the baseline.

## M2 Contracts v0

### WP-M2-01 Contracts package from the prototype protocol
- **Issue:** [#9](https://github.com/rluders/amaru/issues/9)
- **Goal:** `packages/contracts` adapted from the prototype `protocol/`.
- **Scope:**
  - Migration ADR.
  - Rename namespaces per the glossary.
  - Add Approval/Policy, change events (WRK-003), the Worker protocol draft and usage fields (OBS-002).
  - Fix the drift (`/work-packages/{id}/autoloop`).
- **Features:** API-001, API-002.
- **Depends on:** WP-M1-01, WP-M1-03, WP-M1-04, WP-M1-06.
- **Done when:**
  - Valid and invalid fixtures pass in Go and TypeScript in `make check`.
  - The product owner has added the Apache-2.0 `LICENSE` to `packages/contracts` ([ADR-0026](adr/0026-fair-code-community-license-with-commercial-licensing.md); agents never add license text).
- **Validation:** fixture tests.
- **Status (2026-10-26):** in progress. [ADR-0047](adr/0047-migrate-the-prototype-protocol-into-packages-contracts.md) accepted (#67 closed); the contract files, Worker Protobuf with a `buf breaking` baseline and the route-drift guard are in `packages/contracts` (#68). Q56–Q58 were decided by [ADR-0048](adr/0048-approval-reuse-uncovered-capabilities-and-policy-defaults.md) (accepted 2026-10-26) and the Approval/Policy shapes follow it. Shared fixtures (315: 208 re-validated from the prototype, 107 new for Approval, Policy, Principal, Grant, usage, events, Step timeout and interruption, and the Worker envelope) are validated in Go and TypeScript with identical outcomes (#117). The product owner's `LICENSE` is not done.

### WP-M2-02 Generated TypeScript SDK
- **Issue:** [#10](https://github.com/rluders/amaru/issues/10)
- **Goal:** `packages/sdk` generated from the contracts, including an event-stream client with cursor resume.
- **Features:** API-003.
- **Depends on:** WP-M2-01.
- **Done when:**
  - Generation is reproducible.
  - The staleness check is in `make check`.
  - The reconnect logic is reimplemented with tests, using the prototype's plugin's `connection.ts` as a behavioral reference only ([ADR-0031](adr/0031-obsidian-plugin-is-retired.md)).
  - The product owner has added the Apache-2.0 `LICENSE` to `packages/sdk` ([ADR-0026](adr/0026-fair-code-community-license-with-commercial-licensing.md)).
- **Validation:** unit tests.
- **Status (2026-10-26):** in progress. `packages/sdk` has the generated types (`openapi-typescript`), the typed client (`openapi-fetch`), the event-stream client with cursor resume and reconnect, and a staleness check in `make check` (`make sdk-ts`); unit-tested with fakes only. `packages/sdk/LICENSE` (Apache-2.0) was added on 2026-10-26 at the product owner's instruction.

### WP-M2-03 Contract drift guard
- **Issue:** [#11](https://github.com/rluders/amaru/issues/11)
- **Goal:** the Platform cannot register a route that is not in the contract.
- **Scope:** a test that enumerates registered routes and compares them with OpenAPI. It lands with the first Platform code in M3, so the guard ships with the API.
- **Features:** API-001.
- **Depends on:** WP-M2-01.
- **Done when:** the test fails on a deliberately unregistered route.
- **Validation:** the test itself.
- **Status (2026-10-26):** done for M2. The guard is the `packages/contracts/routeguard` test utility, delivered with the OpenAPI contract in [#68](https://github.com/rluders/amaru/issues/68); `TestRegisteredButUndeclaredRouteFails` registers the prototype's undeclared `…/autoloop` route and asserts the guard reports it, and `TestDeclaredButUnregisteredRouteFails` covers the other direction (unit-tested, fake route list). It has not run against a real router, because no Platform code exists: wiring it into the Platform's tests is part of WP-M3-06 ("Drift guard").

## M3 Platform core

### WP-M3-01 Domain and ports migration
- **Issue:** [#13](https://github.com/rluders/amaru/issues/13)
- **Goal:** prototype `runtime/internal/domain` and `ports` adapted into `services/platform`, with the architecture test.
- **Scope:**
  - Rename identifiers.
  - New states from [DOMAIN_MODEL §5](DOMAIN_MODEL.md#5-lifecycles): Execution `waiting`; Step `refused`, `leased`, `awaiting_approval`, `interrupted`; Approval `expired`, `voided`; restart rule of [ADR-0042](adr/0042-durable-execution-with-explicit-restart-semantics.md).
  - Organization as an implicit scope.
- **Features:** EXE-001, WRK-001, WRK-002, AGT-001, AGT-004, CAP-001.
- **Depends on:** M2; a migration ADR.
- **Done when:** ported domain tests and the architecture test are green on three OSes.
- **Validation:** `make check` on Linux, macOS and Windows per the [cross-platform validation](../engineering/validation.md#cross-platform-validation-m3-and-m4) rules (no hosted CI).

### WP-M3-02 Storage adapter
- **Issue:** [#14](https://github.com/rluders/amaru/issues/14)
- **Goal:** storage ports implemented on PostgreSQL ([ADR-0015](adr/0015-postgresql-is-the-only-database-bundled-in-compact-installs.md)).
- **Scope:**
  - Schema versioning and migrations.
  - Start-up lock.
  - Importer for the prototype definitions, Work Packages, Work Items and Teams (`amaru import legacy`).
- **Features:** PRJ-005, OPS-002, EXE-002 (persistence part).
- **Depends on:** WP-M1-02, WP-M1-05, WP-M3-01.
- **Done when:** adapter conformance tests and migration tests pass, and a prototype project-definition fixture imports.
- **Validation:** tests.

### WP-M3-03 Execution coordinator with embedded Worker
- **Issue:** [#15](https://github.com/rluders/amaru/issues/15)
- **Goal:** prototype `app` manager, steps and recovery adapted behind the Worker interface.
- **Scope:**
  - Per-Step timeouts.
  - Idempotency.
  - Concurrency slots.
  - Restart recovery.
  - The crash-injection suite.
- **Features:** EXE-002, EXE-003, EXE-005, EXE-006, WFL-001, WFL-002, WFL-003, WFL-004, ENV-001, CAP-004.
- **Depends on:** WP-M3-01, WP-M3-02.
- **Done when:** the crash-injection suite passes and a cancel on Windows kills the process tree.
- **Validation:** tests on Linux, macOS and Windows per the [cross-platform validation](../engineering/validation.md#cross-platform-validation-m3-and-m4) rules; the Windows process-tree cancel is a native Windows run.

### WP-M3-04 Policy, Grants and Approval
- **Issue:** [#16](https://github.com/rluders/amaru/issues/16)
- **Goal:** the new authorization layer.
- **Scope:**
  - Grants and Approval Steps with evidence binding.
  - Consequential-action checks on every trigger.
  - Audit events.
  - Trust and egress consent carried over.
- **Features:** POL-001, POL-002, POL-003, POL-004, REV-001, REV-003, PRJ-003.
- **Depends on:** WP-M1-04, WP-M3-03.
- **Done when:** ungranted use is refused before start, and approval waits survive restarts.
- **Validation:** tests and a recorded attempt without a grant.

### WP-M3-05 Providers and context assembly
- **Issue:** [#17](https://github.com/rluders/amaru/issues/17)
- **Goal:** agent providers, external provider protocol, Skills and context.
- **Scope:**
  - Claude, Codex and OpenCode providers, where Codex and OpenCode refuse unenforceable `WorkspaceWrite`.
  - Workspace command sandbox.
  - The provider protocol renamed.
  - Health probes.
  - Usage parsing hooks.
- **Features:** CAP-002, CAP-003, CAP-005, AGT-002, AGT-003, SKL-001, SKL-002, POL-005, POL-006, POL-007.
- **Depends on:** WP-M3-03; migration ADRs per provider.
- **Done when:** captured-output tests pass and a recorded real run is made per agent provider.
- **Validation:** tests and recorded runs.
- **Contract follow-up (from the M2 exit, [record](investigations/2026-10-27-m2-contracts-v0-exit.md)):** the prototype's Skill listing and trust routes and its provider secret routes are not in the `/v1` contract. This Work Package (Skills) and the Work Package for POL-007 (secrets) add the OpenAPI operations and fixtures they need, in the same change as the implementation.

### WP-M3-06 Public API, events and local authentication
- **Issue:** [#18](https://github.com/rluders/amaru/issues/18)
- **Goal:** the HTTP API and event stream serving the M2 contracts.
- **Scope:**
  - Work, Workflow and Agent change events.
  - Compatibility endpoint.
  - Loopback auth per [ADR-0034](adr/0034-local-web-client-authentication-with-launch-code-and-cookie.md).
  - Drift guard (WP-M2-03).
  - Structured logs.
- **Features:** EXE-004, WRK-003, API-005, IAM-001, OBS-001, PRJ-001, PRJ-002.
- **Depends on:** WP-M3-03.
- **Done when:** contract tests pass against the running server, the drift guard (`routeguard`) passes against the server's real route registrations, and security tests for Origin, Host and CSRF pass.
- **Validation:** tests.

### WP-M3-07 Worktrees, configuration scopes, evidence
- **Issue:** [#19](https://github.com/rluders/amaru/issues/19)
- **Goal:** the remaining core pieces.
- **Scope:**
  - Worktree isolation.
  - Configuration resolution ([ADR-0039](adr/0039-configuration-scopes-and-resolution-order.md)).
  - Artifacts with Platform hashes.
  - Evidence query.
  - Retention per data class and redaction of stored provider output (moved here from WP-M3-02; see [cross-package feature dependencies](#cross-package-feature-dependencies)).
- **Features:** ENV-010, OPS-003, ART-001, ART-002, ART-003.
- **Depends on:** WP-M3-03; for ART-003 also REV-003 from WP-M3-04 (feature-level link).
- **Done when:** the audit query returns the full Evidence for a test Execution.
- **Validation:** tests.

## M4 Local MVP

### WP-M4-01 Web shell and design system
- **Issue:** [#21](https://github.com/rluders/amaru/issues/21)
- **Goal:** `apps/web` built from the prototype's design system, served by the Platform.
- **Scope:**
  - Migration ADR for the prototype dashboard assets.
  - Tokens, components and layouts.
  - Navigation per `IA_REVIEW.md`.
  - Accessibility baseline.
  - Remove pages without a backend.
- **Features:** CLI-001, CLI-002, CLI-004, CLI-007.
- **Depends on:** M3; WP-M2-02; LICENSE files in effect (WP-M1-01).
- **Done when:** the app builds, is served from the Platform and passes accessibility checks on its shell.
- **Validation:** component tests and automated accessibility checks.

### WP-M4-02 Core screens
- **Issue:** [#22](https://github.com/rluders/amaru/issues/22)
- **Goal:** Projects, Work, Workflows (text editor), Executions (timeline, logs), Instance health, settings (providers, Policy, Local Checks).
- **Features:** CLI-003, CLI-005, CLI-006, OBS-003, ADM-001, PRJ-004.
- **Depends on:** WP-M4-01.
- **Done when:** every screen uses only the SDK, and the browser end-to-end tests pass.
- **Validation:** end-to-end tests.

### WP-M4-03 Delivery loop migration
- **Issue:** [#23](https://github.com/rluders/amaru/issues/23)
- **Goal:** the prototype Autoloop adapted as the supervised delivery loop on the Amaru coordinator.
- **Scope:**
  - Budgets moved into Policy.
  - Base refresh (merge, never rebase).
  - Restart recoveries.
  - F-39 and F-40 behavior.
- **Features:** WRK-004, WRK-005.
- **Depends on:** M3; a migration ADR.
- **Done when:** ported Autoloop tests are green, and the F-39/F-40 regression tests pass. The loop is built against the M3 ports (forge port, Local Check runner, review Step) with fakes; its dependencies on VAL-001, REV-004, REV-005 and INT-001 are soft ([cross-package feature dependencies](#cross-package-feature-dependencies)), and its real-validation run belongs to WP-M4-07.
- **Validation:** tests.

### WP-M4-04 Checks, CI and review
- **Issue:** [#24](https://github.com/rluders/amaru/issues/24)
- **Goal:** the validation and review chain.
- **Scope:**
  - Local Checks bound to commits.
  - Quality-gate policy.
  - CI observation and classification.
  - Agent review with `{{validation}}` evidence.
- **Features:** VAL-001, VAL-002, VAL-003, REV-004.
- **Depends on:** WP-M4-03.
- **Done when:** a review without validation Evidence is impossible, and CI unavailability is shown with its cause.
- **Validation:** tests.

### WP-M4-05 Forge delivery
- **Issue:** [#25](https://github.com/rluders/amaru/issues/25)
- **Goal:** GitHub integration through a forge port.
- **Scope:**
  - Draft PRs, marked ready after approval.
  - Reconcile.
  - Credentials only to forge processes.
- **Features:** INT-001, INT-002, REV-005.
- **Depends on:** WP-M4-03.
- **Done when:** fake-forge tests pass and a recorded PR is made against a real repository.
- **Validation:** tests and a recorded run.

### WP-M4-06 Approval inbox, diff and cost
- **Issue:** [#26](https://github.com/rluders/amaru/issues/26)
- **Goal:** the human side of the flow.
- **Scope:**
  - Inbox across Projects.
  - Diff and Evidence view.
  - Per-Step token and cost display.
- **Features:** REV-002, REV-006, OBS-002.
- **Depends on:** WP-M4-02, WP-M4-04.
- **Done when:** an approver sees diff, checks, review verdict and cost before deciding, and stale notices clear.
- **Validation:** end-to-end tests.

### WP-M4-07 Packaging and MVP validation
- **Issue:** [#27](https://github.com/rluders/amaru/issues/27)
- **Goal:** installable local builds and the MVP exit evidence.
- **Scope:**
  - Signed per-OS archives (no Docker, no admin rights); native installers come with Desktop (M8).
  - Install documentation.
  - Recorded supervised runs on Linux, macOS and Windows.
  - Audit walkthrough.
  - Efficiency check (≤1 implementation and ≤1 review per accepted change).
- **Features:** OPS-001.
- **Depends on:** WP-M4-01 … 06.
- **Done when:** the [M4 exit criteria](ROADMAP.md#m4-local-mvp) are met and recorded in `docs/` validation records, including the WRK-004 recorded supervised run per OS (the first point where the loop, checks, review and forge delivery all exist).
- **Validation:** the recorded runs, per the [cross-platform validation](../engineering/validation.md#cross-platform-validation-m3-and-m4) rules.

## Cross-package feature dependencies

Status: **accepted** as planning guidance (2026-10-25, product-owner review of PR #223). The Work Package order above stays the accepted baseline; this section records how catalog dependencies that cross it are resolved, and the changes it makes to that order (ART-003 only). Rationale: every soft dependency below is built against a port that an earlier Work Package already delivers, the only hard dependency against the order (ART-003) changes the order instead of being softened, and the resulting issue graph has no cycle (including blockers inherited through parents) and no issue blocked by a later milestone.

Where a catalog dependency crosses Work Packages against their order, it is resolved here (backlog refinement, 2026-10-25). *Hard* means the dependent cannot be built or accepted without the other feature; *soft* means it is built against an existing port or contract with fakes, and the other feature is needed only for the real-validation run ([catalog §1](../product/FEATURE_CATALOG.md#1-how-to-read-this-catalog)).

| Dependency | Work Packages | Resolution | Reason |
|---|---|---|---|
| CAP-002 ↔ POL-006 (cycle) | WP-M3-05 | **Corrected**: POL-006 depends on CAP-002 and AGT-003; CAP-002 no longer depends on POL-006. | Refusal is implemented in the adapted providers (PRD FR-24), so it needs the providers and the requested restrictions. CAP-002 now declares and enforces confinement flags; refusing unenforceable ones is POL-006. No requirement was dropped. |
| ART-003 → ART-002 | WP-M3-02 → WP-M3-07 | **Hard; order changed**: ART-003 moves from WP-M3-02 to WP-M3-07, and also depends on REV-003. | Retention keeps Artifacts while an Approval or accepted Work Item references them, and redaction applies to provider output that is first stored by the coordinator and providers ([DATA_ARCHITECTURE §5](DATA_ARCHITECTURE.md#5-retention-and-redaction)). None of that exists when WP-M3-02 runs. WP-M3-02's *Done when* never covered ART-003, so it is unchanged. |
| WRK-001 → PRJ-001 | WP-M3-01 → WP-M3-06 | **Soft; order kept.** | Work Items need the Project entity and key, which WP-M3-01 migrates with the domain; PRJ-001 is the attach flow (metadata location, stable ID across path spellings, API). WRK-001's acceptance is met by domain tests. |
| WRK-004 → VAL-001 | WP-M4-03 → WP-M4-04 | **Soft; order kept.** | The loop calls Local Checks through the runner port ported in M3. Real check binding (head commit, config hash) is exercised in the WP-M4-07 recorded runs. |
| WRK-004 → REV-004 | WP-M4-03 → WP-M4-04 | **Soft; order kept.** | The loop starts review Steps (a Step type from M3); the structured verdict and `{{validation}}` Evidence are completed in WP-M4-04, whose CI-observation and validation-Evidence code comes out of the prototype loop files that WP-M4-03 migrates first. |
| WRK-004 → REV-005 | WP-M4-03 → WP-M4-05 | **Soft; order kept.** | Draft-only delivery is a call on the forge port (`OpenPR(..., draft)`), ported in WP-M3-01; tests use a fake forge. |
| WRK-004 → INT-001 | WP-M4-03 → WP-M4-05 | **Soft; order kept.** | Same forge port; the GitHub adapter arrives in WP-M4-05. |

Reversing WP-M4-03 and WP-M4-04/05 was considered and rejected: in the migration baseline ([ADR-0032](adr/0032-migration-baseline-is-a-pinned-commit.md)) the delivery loop's files (`runtime/internal/app/autoloop*`) hold CI observation and the validation-Evidence glue, so checks and review would need the loop's migration ADR anyway, and the loop's acceptance does not need real adapters. Reversing would also create a blocker cycle through sub-issues. The WRK-004 real run is part of WP-M4-07, which depends on all M4 Work Packages, so no issue waits on a later one.

Rules for further resolutions (see also [delivery §4](../engineering/delivery.md#4-dependencies)):
- Classify first, then sequence. A hard dependency against the order changes the order (as for ART-003); a soft one keeps it and names the Work Package that holds the real-validation run.
- A dependency is soft only if the port or contract it is built against already exists in an earlier Work Package; otherwise it is hard.
- A soft dependency lets implementation start earlier; it does not waive integration testing against the real feature or the parent Work Package's *Done when*.
- A sub-issue inherits its parent's blockers: never link a Work Package to a sub-issue of a Work Package it blocks.

## Coverage check

The union of the features listed in WP-M2-* … WP-M4-* equals the M2–M4 feature sets in the [roadmap](ROADMAP.md#feature-traceability): 73 P0 features.
