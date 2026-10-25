# Weave inventory (snapshot)

**Snapshot:** `rluders/weave-obsidian`, branch `feat/orkestra-dogfood`, commit `c8a2f6e` ("Cycle 7: deployment, supervised httpsuite pilot, findings F-38..F-45", 2026-10-18), 120 commits, clean working tree, read on **2026-10-25**.

**Branches:**
- `origin/main` is `d31c977` (36 commits). It is an ancestor of the snapshot and is 84 commits behind it.
- The local `main` is stale at `08a46a0`.
- The previous snapshot (2026-10-24) read `d31c977`.
- The migration baseline is this snapshot commit, `c8a2f6e` on `feat/orkestra-dogfood` ([ADR-0032](../architecture/adr/0032-migration-baseline-is-orkestra-dogfood-c8a2f6e.md)).

This is a point-in-time reading of another repository, made for migration planning. It is not maintained live; refresh it before any migration decision ([MIGRATION_PLAN §5](MIGRATION_PLAN.md#5-readiness-criteria)). Claims come from reading code, tests and Weave's own docs. **No Weave test was run for this snapshot.**

## Identity and license
- **Product name in code and docs: Weave.**
  - Runtime binary `weaved`, Go module `github.com/rluders/weave/runtime`.
  - Provider protocol `weave.provider/v1`.
  - Plugin id `weave`.
- **Orkestra** names the dogfood system under test (`docs/dogfood/2026-10-orkestra/`).
- **License: MIT**, "Copyright (c) 2026 Ricardo Lüders".

## Structure and stack
| Area | Content | Size |
|---|---|---|
| `runtime/` | Go daemon. `cmd/weaved`; `internal/{domain,app,ports,provider/*,platform,transport/{httpapi,webhook},store/{fsstore,fsscan,projectfs,userstore},discovery,auth,project,connector,protocol,version,arch}` | 134 non-test Go files (≈27.4k lines); 103 test files (≈19.2k lines); 546 `func Test*` |
| `plugin/` | Obsidian plugin: TypeScript, esbuild, React 19 (workflow editor `ui/flow`, `@xyflow/react`), three.js viewer, vitest; `isDesktopOnly: true` | 71 source files (≈9.6k lines); 32 test files (≈281 cases) |
| `protocol/` | `openapi.yaml` (56 paths), `schema.json`, `events.schema.json`, `provider.schema.json`; fixtures in `fixtures/{types,events,provider}` | 271 fixtures |
| `docs/` | architecture, protocol, provider protocol, security, ROADMAP, 29 ADRs, click-through records, dogfood cycle reports and findings | — |
| `.claude/skills/` | `graft`, `obsidian-plugin`, `runtime-integration`, `testing`, `ui-reference` | — |
| `.github/workflows/ci.yml` | Go vet/test on ubuntu/macos/windows (race on Linux); plugin typecheck/lint/test | Weave records it as never run on GitHub (GAP-05) |
| `graft/` | Generated repository map (gitignored), tooling only | — |
| `design/` | Pencil design file and reference PNGs | — |

Go dependencies: `coder/websocket`, `rluders/lane`, `yaml.v3`, `santhosh-tekuri/jsonschema` (tests). No database. `make check` = vet + `go test -race -count=1 ./...` + plugin checks.

## Implemented behavior (by evidence level)

**Real-validated** (recorded manual or dogfood runs, Linux):
- **Runtime basics:** discovery, singleton guard (liveness probe), graceful shutdown; execution lifecycle and restart semantics.
- **Plugin:** reconnect; project adoption.
- **Workflows and agents:** workflow graphs and editor; Claude provider; Agents and Skills.
- **Delivery loop:** Autoloop end to end, including worktrees, sandboxed agents, `code.test`, CI wait, agent review, UI approval and draft PRs. Recorded in Cycles 1–7 on orkestra, httpsuite, canery, lane and mimic, plus the Orbital Courier vault.
- **Resilience:** restarts survived (Cycle 1: three restarts); base refresh and resume (Cycle 3).
- **Local checks:** a supervised pilot with deployment (Cycle 7).

**Implemented and tested, not real-validated:**
- multi-project runs beyond dogfood;
- remote-job providers on fakes (Meshy, Tripo, Replicate, fal, ElevenLabs);
- Blender `scene.inspect`;
- MCP client phase 1;
- webhooks;
- Teams;
- grants;
- intake reconcile.

**Run for real in tests:** Codex and OpenCode providers, Pandoc and ffmpeg, MCP against the reference filesystem server.

## Behavior a migration must preserve
- **Architecture:** the Capability → Provider boundary, with core purity enforced by `runtime/internal/arch/arch_test.go`.
- **Execution:** the state machine and restart semantics:
  - `Manager.Restore`, `ResumeInterrupted` (only when steps are unchanged);
  - idempotency keys;
  - Autoloop "record side effect before the next" ledger and restart recoveries (ADR-029).
- **Protocol:** shapes validated by the same fixtures in Go and TypeScript.
- **Security:**
  - loopback bind, bearer token, Origin allowlist;
  - env allowlist, stdin-only prompts;
  - writes confined to `.weave/`;
  - no auto-approved agent permissions;
  - workspace-command validation and denylist;
  - egress consent with private-host blocking;
  - HMAC webhooks (`docs/security.md`, `domain/agent.go`, `transport/webhook/sender.go`).
- **Context:** deterministic context assembly with Agent/Skill snapshots and `context.md` evidence (ADR-011).
- **Outputs:** role plus project-relative path, with hashes computed by the Runtime (ADR-012).
- **Validation and review:** Local Checks bound to head commit and config hash; review prompts containing validation evidence (F-38).

## Architecture decisions
Weave ADRs in `docs/decisions/`, status at snapshot:

| ADR | Topic | Status |
|---|---|---|
| 001 | Architecture (plugin/runtime/protocol) | proposed; discovery parts superseded by 009 |
| 002 | Execution persistence (`execution.json` + `events.jsonl`) | accepted |
| 003 | `minAppVersion` | no status line |
| 004 | Work Package / Work Item persistence | accepted |
| 005 | Artifact index | accepted |
| 006 | `asset.generate` | accepted |
| 007 | Workflow | accepted, partly superseded by 008 |
| 008 | Workflow step execution | accepted |
| 009 | Capability provider (Blender) | accepted |
| 009 | Multi-project Runtime | accepted (duplicate number) |
| 010 | Agents | accepted |
| 011 | Skills and context assembly | accepted |
| 012 | Capability specs, remote and external providers | accepted |
| 013 | Workflow graph, YAML | accepted |
| 014 | Webhooks | accepted |
| 015 | `.blend` preview | declined |
| 016 | MCP client provider | accepted (phase 1) |
| 017 | Skills evolution | accepted |
| 018 | Playbooks and Workflow templates | accepted |
| 019 | Teams (review + approval, opt-in separation of duties) | accepted |
| 020 | Agents evolution | partly accepted |
| 021 | Autoloop | accepted |
| 022 | Vault knowledge workflow | accepted |
| 023 | Hermetic agent runs and tool scope | phase 0 built; later phases accepted as direction |
| 024 | Agent grants and trusted projects | accepted as direction, partly built |
| 025 | Weave MCP gateway | accepted in direction, not built |
| 026 | Linked repositories, draft-only delivery, sandboxed workspace commands | accepted |
| 027 | Periodic GitHub reconcile, issue dependencies | accepted |
| 028 | Base refresh, resume after unblock | accepted |
| 029 | Local checks, explicit commit change set, restart recovery | accepted (2026-10-18) |

## Dogfood findings
Source: `docs/dogfood/2026-10-orkestra/findings.md`, plus the cycle reports.

**Open at snapshot**
- F-7: lost update when editing Agents.
- F-13: webhook relay operations.
- F-23: stale approval notices.
- F-27: fixed 30-minute quota pause.
- F-34: flaky test.
- F-35: log tail can miss failing lines.
- F-36: CI billing cause not shown.
- F-37: stale cancelled run listed.
- F-39: no-op implementation spends an attempt.
- F-40: unblock re-implements instead of re-reviewing.
- F-41: token and cost usage dropped.
- F-42: `dir: "."` rejected.
- F-43: no UI for `localChecks` or workflow variables.
- F-44: templates don't warn about a missing team.
- F-45: PR stays draft after approval.

**Cycle 7 verdict:** "Not ready for unattended engineering. Ready for: one package, one Go repository, local checks plus hosted CI, a person approving each review and doing each merge." One accepted change took 2 useful LLM runs and 6 wasted ones. 15 Autoloops are paused.

## Known gaps (Weave ROADMAP)
- GAP-05: Windows/macOS unvalidated.
- GAP-06: no change events for Work/Workflows.
- GAP-07: no retention, redaction or schema migration.
- Verified in code at snapshot:
  - no per-Step wall-clock timeout on agent runs;
  - steps run sequentially even when the DAG allows parallel branches;
  - no leases;
  - Codex and OpenCode providers do not act on `WorkspaceWrite`;
  - work files are not watched after startup.

## Discrepancies noticed while reading
Recorded here only; **not fixed in Weave**.
- `protocol/openapi.yaml` lacks `GET /work-packages/{id}/autoloop`, which `transport/httpapi/server.go` registers.
- `plugin/src/runtime/types.ts` cites `test/runtime.contract.test.ts`, which does not exist.
- `Makefile` `.PHONY` omits `plugin-check`.
- `docs/security.md` lists ADR-029 as proposed; the ADR is accepted.
- ROADMAP sections are stale (MCP and Teams are described as absent; "in progress" refers to ADR-012/013).
- Two ADRs share number 009.
