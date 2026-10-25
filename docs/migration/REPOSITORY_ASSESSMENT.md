# Repository Assessment

Status: **assessment** (read-only investigation, 2026-10-25). Compares the three repositories subsystem by subsystem and recommends a disposition for each. **No code has been moved.** Dispositions are recommendations. Each one becomes binding only through a migration ADR approved under [MIGRATION_PLAN §5](MIGRATION_PLAN.md#5-readiness-criteria).

## 1. Scope and method

| Repository | State inspected | How |
|---|---|---|
| `rluders/amaru` | `main` tracking `origin/claude/tender-mayer-a9x20h` at `71d4dfe` (1 commit, 46 files) | Read all docs, ADRs, Skills, `tools/doccheck`; `make check` green |
| `rluders/weave-obsidian` | Branch `feat/orkestra-dogfood` at `c8a2f6e` (120 commits, clean tree; 84 commits ahead of `origin/main` `d31c977`) | Read code, tests, `protocol/`, `docs/` (29 ADRs, findings, cycle reports); counted files and tests. **Tests were not run.** |
| `rluders/weave` | `main` at `824af76` (53 commits, 2026-09-15 … 09-28, clean tree) | Read backend, dashboard, docs, investigations. **Tests were not run.** |

Counts come from file and `func Test*` enumeration, not from test execution. The evidence levels follow [validation](../engineering/validation.md).

## 2. Repository summaries

**`amaru`.** This is the product foundation.
- It holds documents, governance, Skills and a stdlib-only documentation checker.
- It has no product code and no LICENSE file.
- Its documents had predated this assessment. They described Weave at `d31c977` and treated the client choice as open.

**`weave-obsidian` (Weave Runtime).** This is the most mature execution implementation.
- **Code and tests:** Go daemon `weaved` (about 27.4k non-test lines, 19.2k test lines, 546 test functions); TypeScript Obsidian plugin (9.6k lines, about 281 test cases); protocol schemas with 271 fixtures; CI matrix on three OSes, which Weave records as never run on GitHub.
- **Operational lessons:** seven dogfood cycles against real GitHub repositories, recorded as findings F-0…F-45.
- **Verdict at Cycle 7:** "Not ready for unattended engineering", but ready for a supervised single-package flow.

**`weave` (prototype).**
- **Backend:** Go service on PostgreSQL, with users, workspaces, Signals and a state-machine workflow engine. Executions run synchronously inside HTTP requests in disposable Podman containers, and the agent's diff is captured. Verification runs and evidence-linked approvals are included. It has 171 test functions.
- **Dashboard:** React 19 / TanStack / shadcn, with about 70 routes, many without backend support, and no tests.
- **Docs:** strong product and decision documents, including a self-hosting log where two real PRs were merged through it.
- **License:** none.

## 3. Subsystem comparison

| Subsystem | `weave-obsidian` (Runtime) | `weave` (prototype) | `amaru` | Recommendation | Evidence | Prerequisites / risks |
|---|---|---|---|---|---|---|
| **Domain model** | Rich, pure, tested. Work, Workflow DAG, Execution/Step, Agent, Skill, Team, Capability, Autoloop ledger, Local Checks, Grants. | Workspace, WorkflowRun, state-machine workflows, Signal, VerificationRun, evidence-linked Approval. | Glossary + ER sketch | **Adapt** Runtime domain. **Reference** prototype for Organization, Signal and evidence links. | `runtime/internal/domain/*`; `backend/internal/domain/*` | ADR-0027 metadata split; new states ([DOMAIN_MODEL](../architecture/DOMAIN_MODEL.md)) |
| **Architecture boundaries** | Hexagonal; `arch_test.go` enforces core purity | Modular monolith; 2.9k-line `service.go` | ADR-0009 accepted | **Reuse** the architecture test; **Retire** the prototype `service.go` structure | `internal/arch/arch_test.go`; `application/service.go` | — |
| **Persistence** | Files, atomic writes, no migrations, read at startup | PostgreSQL, 30 migrations, custom runner | PostgreSQL decided (ADR-0015) | **Reimplement** on PostgreSQL behind ports with the outbox. **Reference** prototype migrations. | `store/fsstore`; `store/migrations/*.sql` | M1 validation spike; one-shot importer for `.weave/` (ADR-0027) |
| **Execution engine** | Sequential Steps, restart recovery, Resumable/Detachable, idempotency keys, slots; no timeouts or leases | Synchronous in-request, 5-minute timeout, no recovery | Requirements only | **Adapt** Runtime coordinator; add timeouts, leases, Worker interface | `app/manager.go`, `app/steps.go`; `lifecycle.go`, `podman.go` | ADR-0014; crash-injection suite |
| **Delivery loop** | Autoloop: worktrees, local checks, CI wait/repair, agent review, draft PR, reconcile, restart recoveries | Workflow run → PR via capability gateway | — | **Adapt**; F-39/F-40/F-41/F-27 fixed in Amaru as acceptance tests (ADR-0032) | `app/autoloop*.go`, ADR-021/026/027/028/029 | Baseline `c8a2f6e` (ADR-0032); supervised scope |
| **Contracts** | OpenAPI + JSON Schema + 271 fixtures (Go and TS); hand-typed TS; drift on one route | None (hand-rolled router) | ADR-0004 accepted | **Adapt** into `packages/contracts`; **Reimplement** TS types by generation | `protocol/*`; `plugin/src/runtime/types.ts` | ADR-0025; namespace rename |
| **Events** | WebSocket, per-Project bus, 1024 replay ring, close 4410 | None | Required | **Adapt**; persist in PostgreSQL, fan out over NATS, WebSocket with resumable cursor | `app/eventbus.go`, `transport/httpapi` | ADR-0016, ADR-0025 |
| **Agent providers** | Claude (hermetic flags, sandbox), Codex (read-only), OpenCode (relies on user config); Codex and OpenCode ignore `WorkspaceWrite` | CLI drivers for OpenCode (`--auto`), Codex and Claude Code in containers | — | **Adapt** Runtime providers with the confinement-refusal fix. **Reference** prototype container drivers for the sandbox Environment. | `provider/{claude,codex,opencode}`; `runtime/cli_driver.go` | Vendor CLI drift |
| **Other providers** | Blender, Meshy, Tripo, fal, Replicate, ElevenLabs, Pandoc, ffmpeg, MCP client, GitHub | GitHub PR adapter | — | **Adapt** GitHub and MCP; others become **plugins** later (P2) | `provider/*` | Plugin platform (M6) |
| **Skills and context** | Trusted, hash-pinned Skills; deterministic `context.md` | Skills attached to Agents | — | **Reuse** | ADR-011/017 | — |
| **Validation evidence** | Local Checks bound to head + config hash; CI evidence selection | VerificationRun in sandbox; diff Artifact | — | **Reuse** Local Checks; **Adapt** VerificationRun and diff capture | `domain/localcheck.go`, `app/autoloop_evidence.go`; `domain/verification_run.go`, `podman.go:computeDiff` | — |
| **Approvals and policy** | Approval steps, inbox, Teams with SoD, grants (`needs_grant`), trust | Approval linked to Execution + VerificationRun; required reviewer team | ADR-0006 accepted | **Adapt** both into one model (ADR-0036, ADR-0037) | `domain/step.go` approval, `grant.go`, `team.go`; `domain/approval.go` | WP-M1-04 |
| **Security controls** | Loopback, token, Origin allowlist, argv/env allowlist, stdin prompts, workspace-command denylist, egress consent, HMAC webhooks, secrets port | `--cap-drop ALL`, resource limits, env-file credentials; nested Podman blocked | Security model | **Reuse** Runtime controls; **Adapt** container isolation; **Reimplement** local auth for browsers (ADR-0034) | `docs/security.md`; `podman.go`, `INVESTIGATION.md` | ADR-0034 |
| **Identity and tenancy** | Single user | bcrypt users, HS256 JWT, admin/user roles, workspaces; tenancy not enforced; no register/reset | — | **Reimplement** (M7); **Reference** prototype schema | `backend/internal/auth/auth.go`, migrations 026/027 | ADR-0028 |
| **UI: design system** | Obsidian CSS variables | shadcn radix-nova components, oklch tokens, Geist, conventions | — | **Reuse** (after license) | `dashboard/src/components/ui`, `index.css`, `CONVENTIONS.md` | LICENSE files in effect (ADR-0026) |
| **UI: product screens** | ~30 DOM pages tied to Obsidian; React Flow editor; three.js viewer | ~70 TanStack routes; many without a backend; no tests | — | **Adapt** prototype layout shells and real screens; **Reimplement** the Runtime React Flow editor in Web with it as reference (M7, ADR-0031); **Retire** unbacked pages; **Reference** Obsidian pages for behavior | `dashboard/src/features/*`, `docs/IA_REVIEW.md`; `plugin/src/ui/*` | NFR-38 |
| **UI: client runtime** | Discovery, connection state machine, reconnect with `since`, stores | Raw `fetch` hooks, `useAuthorizedQuery` | — | **Reimplement** the Runtime connection logic in the SDK with it as reference (ADR-0031); **Retire** prototype hooks | `plugin/src/runtime/connection.ts`, `plugin/src/state/*` | WP-M2-02 |
| **Obsidian integration** | Plugin views, adoption, markdown post-processor | — | — | **Reference only**; plugin retired | `plugin/src/main.ts` | ADR-0031 |
| **Deployment** | Per-user daemon; discovery file; Linux validated | docker-compose dev; nested Podman self-host blocked | DEPLOYMENT accepted | **Reuse** discovery for compact; **Retire** prototype compose `backend-stable` | `internal/discovery`; `docker-compose.dev.yml` | — |
| **Docs and decisions** | 29 ADRs (duplicate 009), findings, cycle reports, security doc | 26 decisions, VISION, DOMAIN, WORKFLOWS, IA review, investigations | Canonical | **Reference only**; cite, do not copy | `docs/decisions`, `docs/dogfood`; `docs/*` | — |
| **Agent instructions / Skills** | graft, obsidian-plugin, runtime-integration, testing, ui-reference | `.claude/` | 5 Amaru Skills | **Reference**; Amaru Skills stay canonical | `.claude/skills/*` | — |
| **Tests** | 546 Go test functions; 32 TS test files; fixtures | 171 Go test functions incl. Postgres and live GitHub e2e; 0 dashboard tests | doccheck tests | Port Runtime tests with migrated code; port prototype tests only for adapted pieces | — | Run before each migration ADR |

## 4. Strengths and weaknesses

**`weave-obsidian`**
- *Strengths:*
  - enforced boundaries;
  - recovery semantics;
  - contract discipline with fixtures;
  - security posture;
  - operational learning from real dogfood.
- *Weaknesses:*
  - file persistence without migrations;
  - single-process execution;
  - no timeouts or leases;
  - Obsidian-bound UI;
  - unvalidated Windows/macOS;
  - documentation drift (e.g. `security.md` still lists ADR-029 as proposed);
  - two ADRs numbered 009.

**`weave`**
- *Strengths:*
  - modern web stack and design system;
  - multi-user concepts;
  - the evidence chain (diff → verification → approval);
  - container sandbox flags;
  - product thinking (VISION, IA review).
- *Weaknesses:*
  - synchronous execution;
  - god-object service;
  - unenforced tenancy;
  - UI without backend;
  - no UI tests;
  - no license.

**`amaru`**
- *Strengths:* governance, ADR discipline, honest status labels, doc checking.
- *Weaknesses before this assessment:* the inventory was stale; the prototype was not considered; there was no feature catalog or traceability.

## 5. Naming differences

The same words mean different things in the predecessors. Canonical Amaru terms are in the [glossary mapping](../product/GLOSSARY.md#2-term-mapping-from-predecessor-repositories).

The notable conflicts:
- **Workspace:** a tenant in the prototype, a git worktree in the Runtime.
- **Execution:** a workflow run in the Runtime, a per-agent run in the prototype.
- **Playbook:** a template before Weave ADR-018, a knowledge note after it.
- **Autoloop vs delivery loop.**

## 6. Follow-ups found during the assessment

These are recorded here only. They are not fixed in the other repositories.
- Weave `openapi.yaml` lacks `GET /work-packages/{id}/autoloop`, which `server.go` registers.
- Weave `plugin/src/runtime/types.ts` cites a non-existent `test/runtime.contract.test.ts`.
- Weave `Makefile` `.PHONY` omits `plugin-check`.
- Weave docs drift: ROADMAP sections stale, `security.md` ADR-029 status.
- Prototype `Makefile` hardcodes a user path for `air`.
- The Amaru inventory header named the Weave branch `claude/tender-mayer-a9x20h` (the Amaru branch). Corrected in the refreshed inventory.
