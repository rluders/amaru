# Migration Plan: Weave → Amaru

Status: **plan only.** No code, history, branch or data has been migrated. None will be without explicit approval ([ADR-0007](../architecture/adr/0007-weave-migration-is-a-separate-activity.md), accepted). The strategy is to adapt the Weave Runtime as the Platform core ([ADR-0011](../architecture/adr/0011-platform-adapted-from-weave-runtime.md), accepted 2026-10-25), starting from `feat/orkestra-dogfood` @ `c8a2f6e` ([ADR-0032](../architecture/adr/0032-migration-baseline-is-orkestra-dogfood-c8a2f6e.md)). Evidence and per-subsystem recommendations: [REPOSITORY_ASSESSMENT](REPOSITORY_ASSESSMENT.md).

## 1. Separate things

| | What | Where | Now |
|---|---|---|---|
| **A. Weave Runtime** | The current implementation and its historical identity (Weave/Orkestra): Runtime `weaved`, Obsidian plugin, protocol, ADR-001…029, dogfood records. | `rluders/weave-obsidian` | Active; Cycle 7 complete on `feat/orkestra-dogfood`. |
| **A′. Weave prototype** | Earlier Go + Postgres backend and React/shadcn dashboard. | `rluders/weave` | Inactive since 2026-09-28. |
| **B. Amaru foundation** | Product scope, architecture, decisions, governance, tooling, master roadmap. | `rluders/amaru` (this repo) | M0. |
| **C. Weave completion** | Optional. Amaru does not wait for Weave-side work: findings F-39/F-40/F-41/F-27 are fixed in Amaru as acceptance tests ([ADR-0032](../architecture/adr/0032-migration-baseline-is-orkestra-dogfood-c8a2f6e.md)). | `rluders/weave-obsidian` | Not required. |
| **D. Migration** | Moving selected code and behavior from A and A′ into B. | All | Not started; gated by §5. |

A, A′ and C never depend on B. B may cite A and A′ as evidence but never claims their behavior as its own.

## 2. Dispositions

Each component gets one disposition, recorded in a migration ADR before anything moves:

| Disposition | When | Typical evidence |
|---|---|---|
| **Reuse** (move as is, rename) | Fits the accepted Amaru architecture and contracts; well tested; no Obsidian or Weave-only assumptions. | Tests pass unchanged after the move; architecture test still holds. |
| **Adapt** (move, then change) | Sound core, but boundaries differ (Platform/Worker split, Policy hooks, storage ports, generated SDK). | Behavior-preserving tests written first, kept green through the change. |
| **Reimplement** (new code, old tests or records as specification) | The accepted architecture contradicts the design (e.g. file persistence replaced by PostgreSQL, ADR-0015; prototype auth). | Weave tests or click-through records translated into Amaru acceptance tests. Previously called *Rewrite*. |
| **Reference only** | Useful for learning (docs, decisions, investigations, discarded designs); not migrated. | Cited from Amaru documents. |
| **Retire** | Obsidian-specific UI, unbacked prototype pages, superseded features, providers without users. | Note why; history stays in the source repository. |

## 3. Components and likely dispositions

These dispositions are decided as the plan of record (2026-10-25). Under [ADR-0007](../architecture/adr/0007-weave-migration-is-a-separate-activity.md) each component still needs its migration ADR before code moves; that ADR confirms the disposition with tests run at the baseline commit, and changes it only on new evidence. The milestone is where the migration ADR is needed.

| Source | Component | Amaru home | Disposition | Milestone | Depends on |
|---|---|---|---|---|---|
| Weave | `protocol/` (OpenAPI, schemas, fixtures) | `packages/contracts` | Adapt (rename `weave.*`, add Policy/Approval, Worker protocol, usage; fix drift) | M2 | ADR-0025, ADR-0036/0037, package Apache-2.0 `LICENSE` (ADR-0026, WP-M2-01) |
| Weave | `plugin/src/runtime/connection.ts`, `events.ts` | `packages/sdk` (event client) | Reimplement (behavioral reference; plugin code is not migrated, [ADR-0031](../architecture/adr/0031-obsidian-plugin-is-retired.md)) | M2 | WP-M2-02 |
| Weave | `runtime/internal/domain`, `ports`, `arch` | `services/platform` core | Adapt | M3 | Domain model review |
| Weave | `runtime/internal/app` (manager, steps, context assembly) | Execution coordinator | Adapt (Worker interface, timeouts, Policy, Approval) | M3 | ADR-0042 |
| Weave | `runtime/internal/store/*` | Storage adapters | Reimplement on PostgreSQL behind ports; `fsscan` logic reused for the lexical Knowledge indexer (ADR-0030) | M3 | ADR-0015, ADR-0043 |
| Weave | `runtime/internal/provider/{claude,codex,opencode,gitrepo,github,remotejob,mcp}` | Worker providers | Adapt (confinement refusal, usage parsing) | M3–M4 | per provider |
| Weave | Other `provider/*` (Blender, Meshy, Tripo, fal, Replicate, ElevenLabs, Pandoc, ffmpeg) | Plugins | Pandoc and ffmpeg: Adapt as M6 example `worker-adapter` plugins (they validate A6). Blender, Meshy, Tripo, fal, Replicate, ElevenLabs: Retire (no users); Reference only, re-built as plugins only on a user request | M6 | Plugin platform |
| Weave | `runtime/internal/platform` (process, paths) | Worker OS layer | Reuse | M3 | Windows validation |
| Weave | `runtime/internal/transport/{httpapi,webhook}` | Platform API | Adapt | M3 (API, plus the egress check of `webhook/sender.go` used by FEAT-POL-004: private-host block and per-URL consent, under the `httpapi` migration ADR), M7 (webhooks) | Contracts |
| Weave | `runtime/internal/{auth,discovery}` | Compact deployment adapter | Adapt (browser auth per ADR-0034) | M3 | ADR-0034 |
| Weave | `runtime/internal/app/autoloop*` | Delivery loop | Adapt | M4 | F-39/F-40/F-41/F-27 acceptance tests, baseline ADR-0032 |
| Weave | `plugin/src/ui/flow` (React Flow editor) | Workflow visual editor in `apps/web` | Reimplement (behavioral reference, [ADR-0031](../architecture/adr/0031-obsidian-plugin-is-retired.md)) | M7 | Web client |
| Weave | `plugin/src/ui/viewer` (three.js, mermaid) | Artifact viewer plugins | Reimplement as `ui` plugins (behavioral reference) | P2 | Plugin platform |
| Weave | Obsidian plugin views, adoption, post-processor | — | Reference only; retired | — | ADR-0031 |
| Prototype | `dashboard/src/components/ui`, tokens, conventions | `apps/web` → `packages/ui` | Reuse | M4 | ADR-0026 |
| Prototype | Layout shells, backed feature screens | `apps/web` | Adapt | M4 | ADR-0026 |
| Prototype | Unbacked pages | — | Retire for M4 (Reference for M7) | M4 | — |
| Prototype | `runtime/{driver,cli_driver,podman}.go` | Sandbox Environment Worker | Adapt | M5 | ADR-0026 |
| Prototype | Evidence chain (VerificationRun, diff, approval links) | Domain + Worker | Adapt (concepts) | M3 (model), M5 (sandbox runs) | ADR-0036 |
| Prototype | Auth, Workspaces | Identity (M7) | Reimplement | M7 | ADR-0028 |
| Prototype | `application/service.go`, `http/router.go`, migrations, compose self-host | — | Reference only / Retire | — | — |
| Both | ADRs, decisions, investigations, dogfood records | Cited from Amaru | Reference only | — | §4 |
| Both | `.claude/skills/*` | Amaru `.agents/skills/` | Reference only (Amaru Skills are canonical) | — | — |

## 4. Preserving history and evidence

Default: **option 2, fresh import with provenance**, for every component. The Weave history stays readable in the source repositories, the Amaru repository stays small, and the sole copyright holder needs no blame-based provenance. A migration ADR may choose option 1 for a component whose history matters for review (e.g. the execution coordinator), stating why:
1. **History-preserving import** (`git subtree add` or `git filter-repo` into a path). It keeps blame and commit history for moved code, at the cost of a larger repository. Weave commit messages and names carry over.
2. **Fresh import with provenance.** Copy the code at a tagged source commit, and record that commit in the migration ADR and in file headers where useful. The history stays readable in the source repository.

Either way:
- tag the source commit (`weave-migration-<component>`);
- keep ADRs and validation records reachable;
- port the tests with the code.

Licensing is settled by [ADR-0026](../architecture/adr/0026-fair-code-community-license-with-commercial-licensing.md); migration is not gated by the root LICENSE file (product-owner decision 2026-10-26, WP-M1-01), because the copyright holder of the migrated code consents:
- **Weave Runtime is MIT** (© 2026 Ricardo Lüders). The copyright holder relicenses it into Amaru; the MIT notice is preserved in the source history.
- **The Weave prototype has no license file.** The holder consents to its reuse (recorded in ADR-0026).
- Third-party contributions were checked on 2026-10-25: only the owner and the owner's automation identities authored commits.

## 5. Readiness criteria

Migration of a component may start only when **all** of these hold:

1. **Source side stable.** The component is read at the baseline `c8a2f6e` ([ADR-0032](../architecture/adr/0032-migration-baseline-is-orkestra-dogfood-c8a2f6e.md)) or a later commit named in its migration ADR. Weave findings are Amaru acceptance tests, not Weave-side prerequisites.
2. **Inventory refreshed.** [weave-inventory](weave-inventory.md) or [weave-prototype-inventory](weave-prototype-inventory.md) is updated to the commit being migrated, with tests **run** there.
3. **Target decided.** The Amaru ADRs the component depends on are accepted (e.g. contracts, persistence, Platform/Worker boundary, Policy model).
4. **Contracts stable.** The component's contracts exist in Amaru with fixtures (M2).
5. **Test coverage.** The behavior to preserve (inventory § "Behavior a migration must preserve") is covered by tests that will run in Amaru.
6. **Verified execution.** Real-validated behavior (a click-through or recorded run) exists for what is migrated, or the gap is listed and accepted.
7. **License resolved** (§4).
8. **Explicit approval.** The product owner approves a migration ADR naming the component, the source commit, the disposition and the history option.

## 6. Procedure per component (once approved)

1. Branch in Amaru; write or port the acceptance tests first.
2. Move or adapt the code; rename identifiers per the [glossary](../product/GLOSSARY.md#2-term-mapping-from-predecessor-repositories).
3. Run `make check` until green; record what was not validated.
4. Update the inventory, and mark the component migrated in the source repository's docs (in a change approved there).
5. Freeze the component in the source repository (decided policy, [§9](#9-retirement)): no further feature work there; fixes go to Amaru.

## 7. Sequence

Migration waves follow the [roadmap](../architecture/ROADMAP.md); there is no final "migration stage".

| Wave | Milestone | Components |
|---|---|---|
| 0 | M2 | `protocol/` → contracts; connection logic → SDK |
| 1 | M3 | domain, ports, arch test, app coordinator, platform OS layer, auth/discovery, agent and git/GitHub providers; storage reimplemented |
| 2 | M4 | delivery loop; prototype design system and screens |
| 3 | M5 | prototype container runner → sandbox Environment |
| 4 | M6+ | media providers as plugins; React Flow editor (M7) |
| — | M4 | Obsidian plugin retired; not migrated ([ADR-0031](../architecture/adr/0031-obsidian-plugin-is-retired.md)) |

## 8. Persistence and state migration

- **Definitions.** An importer reads Weave `.weave/` definitions (work, workflows, agents, teams, hooks, trust, settings) into `.amaru/` (Workflows, Agents, Project configuration; Weave `.weave/hooks.yaml` inbound hooks become Workflow webhook Triggers and outbound hooks become Project webhook configuration; hook secrets and egress consent are re-entered by a human, never copied) and the Platform store (Work Packages, Work Items and Teams) per [ADR-0043](../architecture/adr/0043-project-definitions-in-git-teams-and-operational-state-in-platform.md); `trust.json` and grants are not imported as-is, they are re-granted by a human. It runs per Project, after the user confirms it, and never modifies the source files.
- **Operational history.** Weave `runtime/executions` and Autoloop ledgers stay in Weave (read-only); they are not imported. In-flight Weave Executions are not migrated: users finish or cancel them in Weave first.
- **User-level state.** Credentials and grants are re-entered or imported through the secrets port with consent. Secrets are never copied into Amaru files.
- **No dual-write.** A Project is driven by either Weave or Amaru, never both.
- **Prototype data.** PostgreSQL data is not migrated. The prototype has no production users.

## 9. Retirement

- **Weave Runtime.** It is frozen per component as components migrate. It is retired when the Local MVP (M4) covers the Weave flows its users rely on, the Weave-only providers are either plugins or retired, and the product owner approves.
- **Obsidian plugin.** Retired with no Amaru successor ([ADR-0031](../architecture/adr/0031-obsidian-plugin-is-retired.md)).
- **Weave prototype.** Becomes reference-only once its design system and evidence concepts are adapted (M4/M5).

## 10. Current assessment

**Not ready.**

Blocking:
- M1 validation spike not yet run (bundled PostgreSQL, embedded NATS, crash suite).
- Contracts package not yet created (M2), although its rules are decided (ADR-0025, ADR-0036/0037).
- Windows/macOS unvalidated in Weave.

Resolved on 2026-10-25: license decision (ADR-0026; on 2026-10-26 the root LICENSE file was removed as a migration gate), baseline `c8a2f6e` (ADR-0032), persistence and execution decisions (ADR-0014–0016), Weave-side findings no longer prerequisites.

The Weave Runtime is nevertheless the strongest evidence base for the Platform core. Its Capability→Provider boundary, contract discipline, restart semantics and delivery loop are tested and partly real-validated.
