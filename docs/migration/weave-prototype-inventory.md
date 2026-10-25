# Weave prototype inventory (snapshot)

**Snapshot:** `rluders/weave`, branch `main`, commit `824af76` ("feat: add artifact verification, evidence tracking, and diff capabilities", 2026-09-28), 53 commits from 2026-09-15, clean working tree, read on **2026-10-25**. **No tests were run.**

In Amaru documents this repository is called the **Weave prototype**, to distinguish it from the Weave Runtime in `weave-obsidian` ([inventory](weave-inventory.md)).

## Identity and license
- Go module `github.com/rluders/weave`. Product name "Weave".
- **No LICENSE file.** All rights are reserved by the copyright holder. The holder consents to reuse in Amaru ([ADR-0026](../architecture/adr/0026-fair-code-community-license-with-commercial-licensing.md)).

## Structure and stack
| Area | Content | Size |
|---|---|---|
| `backend/` | Go service: `cmd/server`, `cmd/seed`; `internal/{domain,application,http,store,auth,runtime,workflows,capability,app}`; `net/http`, `lib/pq`, `golang-jwt/jwt/v5`, bcrypt, `yaml.v3`, `rluders/lane`; `modernc.org/sqlite` in tests only | ≈10.1k non-test lines; ≈7.4k test lines; 171 `func Test*` |
| `backend/internal/store/migrations/` | Hand-written SQL `001_initial.sql` … `030_approval_execution_link.sql` | 30 migrations |
| `dashboard/` | React 19.2 + React Compiler, Vite 8, TanStack Router (file routes) and Query 5, Tailwind 4, shadcn/ui (`radix-nova`, neutral, lucide), TypeScript ~6, pnpm | ≈15.1k lines TS/TSX; ≈70 route files; **0 tests** |
| `docs/` | `VISION.md`, `ARCHITECTURE.md`, `DOMAIN.md`, `AGENTS.md`, `WORKFLOWS.md`, `DECISIONS.md` (26 decisions), `IA_REVIEW.md`, `self-hosting/README.md` | — |
| Root | `PRD.md` (≈50 KB), `INVESTIGATION.md`, `INVESTIGATION-EXECUTION-BOUNDARY.md`, `docker-compose.dev.yml`, `Makefile`, auth screenshots | — |

## Implemented behavior
**Recorded end to end** (self-hosting log, `docs/self-hosting/README.md`):
1. Work Item.
2. Workflow run.
3. Execution: OpenCode via OmniRoute in a disposable rootless Podman container.
4. Artifact, diff and Verification Run.
5. Approval gated by Team and linked to evidence.
6. `github.pull_request`.

Two PRs were merged this way.

**Implemented and tested:**
- Postgres store with migrations.
- bcrypt users and HS256 JWT with admin/user roles.
- Workspaces and membership. Project scoping is not enforced in `ListProjects`.
- State-machine workflows (`workflows/state_machine.go`).
- Capability gateway with an allowlist and an invocation log (`capability/gateway.go`, `github.go`).
- Signals with triage, promote, reject and defer.
- Teams with a required reviewer team.

**UI without a backend:**
- register, forgot and reset password (they show `AuthGapAlert`);
- most account pages;
- workspace billing, usage, audit log, roles, invitations, integrations.

**Not built, though decided in `docs/DECISIONS.md`:** NATS JetStream event transport, transactional outbox, Herdr execution infrastructure.

**Blocked:** the self-hosted `backend-stable` container cannot run nested rootless Podman (`newuidmap`). `INVESTIGATION.md` classifies this as "cannot safely run without changing a security boundary". `INVESTIGATION-EXECUTION-BOUNDARY.md` proposes a separate executor process. Status: not started.

## Assets and dispositions
Recommendations only. Binding dispositions require a migration ADR.

| Asset | Path | Disposition | Why |
|---|---|---|---|
| shadcn UI components | `dashboard/src/components/ui/*` (alert … yaml-editor) | **Reuse** | Standard, consistent, themable |
| Theme tokens and conventions | `dashboard/src/index.css`, `dashboard/CONVENTIONS.md` | **Reuse** | oklch tokens, light/dark, spacing/typography rules |
| Layout shells and system pages | `components/layout/{AppShell,AuthLayout,OnboardingLayout}.tsx`, `components/system/SystemStatePage.tsx` | **Adapt** | Rework navigation per `docs/IA_REVIEW.md` |
| Feature screens with backend | projects, work items/packages, executions, reviews, artifacts, workflows, agents/teams/skills | **Adapt** | Rewire to the generated SDK; Amaru domain names |
| Screens without backend | account, billing, usage, roles, invitations, audit log, integrations | **Retire** for M4; **Reference** for M7 | NFR-38: no unbacked UI |
| Data hooks | `features/*/hooks.ts`, `lib/api/useAuthorizedQuery.ts` | **Reimplement** | Replace with the generated SDK |
| Runtime driver and container runner | `backend/internal/runtime/{driver.go,cli_driver.go,podman.go}` | **Adapt** (M5 sandbox Environment) | Proven isolation flags; diff and persist-before-teardown order |
| Evidence chain | `domain/{artifact,verification_run,approval}.go`, migrations 028–030 | **Adapt** (concepts) | Approval bound to Execution and Verification Run |
| Capability gateway | `capability/gateway.go` | **Reference** | The Amaru Policy + catalog covers this |
| Workflow state machine | `workflows/state_machine.go` | **Reference only** | Amaru uses DAG Workflows |
| Application service and router | `application/service.go`, `http/router.go` | **Retire** (behavior reference only) | Monolithic, hand-routed, no contract |
| Auth | `internal/auth/auth.go` | **Reimplement** (M7) | Too minimal; see ADR-0028 |
| Migrations | `store/migrations/*.sql` | **Reference only** | Consolidated PostgreSQL schema with Atlas-authored migrations (ADR-0015, ADR-0023) |
| Compose self-host target | `docker-compose.dev.yml` (`backend-stable`) | **Retire** | Blocked by nested containers |
| Product docs | `PRD.md`, `docs/*`, investigations | **Reference only** | Cited by Amaru docs |
