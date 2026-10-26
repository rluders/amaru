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
- *Weave* means `weave-obsidian`. *Prototype* means `weave`.
- WPs that move code require a migration ADR first ([MIGRATION_PLAN §5](../migration/MIGRATION_PLAN.md#5-readiness-criteria), [ADR-0007](adr/0007-weave-migration-is-a-separate-activity.md)).

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
  - Legal review of the Amaru Community License 1.0 draft.
  - CLA text and signing tooling.
  - Trademark search for "Amaru".
  - Vulnerability disclosure policy (`SECURITY.md`).
- **Depends on:** WP-M0-01.
- **Done when:** the product owner adds the root `LICENSE` and Apache-2.0 license files for contract and SDK packages (agents never add license text).
- **Validation:** legal review recorded.

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
- **Validation:** review against the Weave protocol history.

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

### WP-M1-05 `.amaru/` layout and importer design
- **Issue:** [#6](https://github.com/rluders/amaru/issues/6)
- **Goal:** apply [ADR-0043](adr/0043-project-definitions-in-git-teams-and-operational-state-in-platform.md).
- **Scope:**
  - File layout and schemas under `.amaru/`.
  - Two-clone and Cloud behavior as decided in [ADR-0043](adr/0043-project-definitions-in-git-teams-and-operational-state-in-platform.md): a Project is keyed by repository; each Execution records which clone, the commit SHA, the content hash and a definition snapshot; Cloud fetches at a commit.
  - Team references by slug and their validation.
  - Design of the one-shot `amaru import weave` (definitions and Work Items).
- **Depends on:** WP-M1-02.
- **Done when:** layout and importer design are documented.
- **Validation:** data-class review against [DATA_ARCHITECTURE](DATA_ARCHITECTURE.md).

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

### WP-M1-07 Inventory refresh at the baseline
- **Issue:** [#8](https://github.com/rluders/amaru/issues/8)
- **Goal:** a refreshed Amaru inventory at `feat/orkestra-dogfood` @ `c8a2f6e` ([ADR-0032](adr/0032-migration-baseline-is-orkestra-dogfood-c8a2f6e.md)).
- **Scope:**
  - Refresh [weave-inventory](../migration/weave-inventory.md) to the baseline commit.
  - Write Weave findings F-27, F-39, F-40, F-41 as Amaru acceptance tests (specifications for M3/M4).
- **Depends on:** —
- **Done when:** the inventory names the baseline commit and test counts were obtained by running `make check` there.
- **Validation:** Weave test run output at the baseline.

## M2 Contracts v0

### WP-M2-01 Contracts package from the Weave protocol
- **Issue:** [#9](https://github.com/rluders/amaru/issues/9)
- **Goal:** `packages/contracts` adapted from Weave `protocol/`.
- **Scope:**
  - Migration ADR.
  - Rename namespaces per the glossary.
  - Add Approval/Policy, change events (WRK-003), the Worker protocol draft and usage fields (OBS-002).
  - Fix the drift (`/work-packages/{id}/autoloop`).
- **Features:** API-001, API-002.
- **Depends on:** WP-M1-01, WP-M1-03, WP-M1-04, WP-M1-06.
- **Done when:** valid and invalid fixtures pass in Go and TypeScript in `make check`.
- **Validation:** fixture tests.

### WP-M2-02 Generated TypeScript SDK
- **Issue:** [#10](https://github.com/rluders/amaru/issues/10)
- **Goal:** `packages/sdk` generated from the contracts, including an event-stream client with cursor resume.
- **Features:** API-003.
- **Depends on:** WP-M2-01.
- **Done when:**
  - Generation is reproducible.
  - The staleness check is in `make check`.
  - The reconnect logic is reimplemented with tests, using the Weave plugin's `connection.ts` as a behavioral reference only ([ADR-0031](adr/0031-obsidian-plugin-is-retired.md)).
- **Validation:** unit tests.

### WP-M2-03 Contract drift guard
- **Issue:** [#11](https://github.com/rluders/amaru/issues/11)
- **Goal:** the Platform cannot register a route that is not in the contract.
- **Scope:** a test that enumerates registered routes and compares them with OpenAPI. It lands with the first Platform code in M3, so the guard ships with the API.
- **Features:** API-001.
- **Depends on:** WP-M2-01.
- **Done when:** the test fails on a deliberately unregistered route.
- **Validation:** the test itself.

## M3 Platform core

### WP-M3-01 Domain and ports migration
- **Issue:** [#13](https://github.com/rluders/amaru/issues/13)
- **Goal:** Weave `runtime/internal/domain` and `ports` adapted into `services/platform`, with the architecture test.
- **Scope:**
  - Rename identifiers.
  - New states from [DOMAIN_MODEL §5](DOMAIN_MODEL.md#5-lifecycles): Execution `waiting`; Step `refused`, `leased`, `awaiting_approval`, `interrupted`; Approval `expired`, `voided`; restart rule of [ADR-0042](adr/0042-durable-execution-with-explicit-restart-semantics.md).
  - Organization as an implicit scope.
- **Features:** EXE-001, WRK-001, WRK-002, AGT-001, AGT-004, CAP-001.
- **Depends on:** M2; a migration ADR.
- **Done when:** ported domain tests and the architecture test are green on three OSes.
- **Validation:** `make check` (OS matrix run locally or in CI once enabled).

### WP-M3-02 Storage adapter
- **Issue:** [#14](https://github.com/rluders/amaru/issues/14)
- **Goal:** storage ports implemented on PostgreSQL ([ADR-0015](adr/0015-postgresql-is-the-only-database-bundled-in-compact-installs.md)).
- **Scope:**
  - Schema versioning and migrations.
  - Start-up lock.
  - Importer for Weave definitions, Work Packages, Work Items and Teams (`amaru import weave`).
- **Features:** PRJ-005, OPS-002, EXE-002 (persistence part), ART-003.
- **Depends on:** WP-M1-02, WP-M1-05, WP-M3-01.
- **Done when:** adapter conformance tests and migration tests pass, and a Weave `.weave/` fixture imports.
- **Validation:** tests.

### WP-M3-03 Execution coordinator with embedded Worker
- **Issue:** [#15](https://github.com/rluders/amaru/issues/15)
- **Goal:** Weave `app` manager, steps and recovery adapted behind the Worker interface.
- **Scope:**
  - Per-Step timeouts.
  - Idempotency.
  - Concurrency slots.
  - Restart recovery.
  - The crash-injection suite.
- **Features:** EXE-002, EXE-003, EXE-005, EXE-006, WFL-001, WFL-002, WFL-003, WFL-004, ENV-001, CAP-004.
- **Depends on:** WP-M3-01, WP-M3-02.
- **Done when:** the crash-injection suite passes and a cancel on Windows kills the process tree.
- **Validation:** tests on three OSes.

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
- **Done when:** contract tests pass against the running server, and security tests for Origin, Host and CSRF pass.
- **Validation:** tests.

### WP-M3-07 Worktrees, configuration scopes, evidence
- **Issue:** [#19](https://github.com/rluders/amaru/issues/19)
- **Goal:** the remaining core pieces.
- **Scope:**
  - Worktree isolation.
  - Configuration resolution ([ADR-0039](adr/0039-configuration-scopes-and-resolution-order.md)).
  - Artifacts with Platform hashes.
  - Evidence query.
- **Features:** ENV-010, OPS-003, ART-001, ART-002.
- **Depends on:** WP-M3-03.
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
- **Goal:** the Weave Autoloop adapted as the supervised delivery loop on the Amaru coordinator.
- **Scope:**
  - Budgets moved into Policy.
  - Base refresh (merge, never rebase).
  - Restart recoveries.
  - F-39 and F-40 behavior.
- **Features:** WRK-004, WRK-005.
- **Depends on:** M3; a migration ADR.
- **Done when:** ported Autoloop tests are green, and the F-39/F-40 regression tests pass.
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
- **Done when:** the [M4 exit criteria](ROADMAP.md#m4-local-mvp) are met and recorded in `docs/` validation records.
- **Validation:** the recorded runs.

## Coverage check

The union of the features listed in WP-M2-* … WP-M4-* equals the M2–M4 feature sets in the [roadmap](ROADMAP.md#feature-traceability): 73 P0 features.
