# Weave inventory (snapshot)

**Snapshot:** `rluders/weave-obsidian`, branch `main` = `claude/tender-mayer-a9x20h`, commit `d31c977` ("ElevenLabs text to speech adapter (audio.speech) and remotejob.DoRaw"), 36 commits, clean working tree, read on **2026-10-24**.

This is a point-in-time reading of another repository, made for migration planning. It is not maintained live; refresh it before any migration decision ([MIGRATION_PLAN §5](MIGRATION_PLAN.md#5-readiness-criteria)). Claims below come from Weave's own `docs/ROADMAP.md` evidence labels (*implemented*, *tested*, *real-validated*) plus a direct look at the tree; **no Weave test was run for this snapshot**.

## Identity and license
- Product name in code and docs: **Weave** (Runtime binary `weaved`, Go module `github.com/rluders/weave/runtime`, provider protocol `weave.provider/v1`). Earlier design files use **Orkestra**.
- License: **MIT**, "Copyright (c) 2026 Ricardo Lüders".

## Structure and stack
| Area | Content | Size (files) |
|---|---|---|
| `runtime/` | Go 1.27 daemon. `cmd/weaved`; `internal/{domain,app,ports,provider,platform,transport,store,auth,discovery,arch}` | 90 Go files, 66 `_test.go` files |
| `plugin/` | Obsidian plugin, TypeScript strict, esbuild, vitest; React only in the workflow editor; three.js viewer | 23 test files (Weave ROADMAP records 387 plugin tests on 2026-10-04) |
| `protocol/` | `openapi.yaml` (OpenAPI 3.1, `/v1`), `schema.json`, `events.schema.json`, `provider.schema.json` (JSON Schema 2020-12), fixtures in `events/`, `provider/`, `types/` | 193 fixtures |
| `docs/` | architecture, protocol, provider protocol, security, adoption, MCP, playbooks, UI mapping, click-through validation records, investigations, spikes, ROADMAP | — |
| `.claude/skills/` | `graft`, `obsidian-plugin`, `runtime-integration`, `testing`, `ui-reference` | — |
| `.github/workflows/ci.yml` | Go vet/test on ubuntu/macos/windows, race on Linux, cross-vet; plugin typecheck/lint/test. ROADMAP GAP-05 says it has never run on GitHub. | — |

Go dependencies: `coder/websocket`, `rluders/lane`, `yaml.v3`, `santhosh-tekuri/jsonschema` (tests). Validation: `make check` = `go vet` + `go test -race ./...` + plugin typecheck/lint/test.

## Implemented behavior (by Weave's evidence level)
**Real-validated** (recorded manual runs, Linux): per-user Runtime discovery, singleton guard, graceful shutdown; execution lifecycle and persistence with restart semantics; plugin reconnect state machine; project adoption writing only in `.weave/`; workflow definition/run and step executions; workflow ordering edges, data bindings and layout in the React Flow editor; provider connections UI; Claude agent provider; Agents and provider resolution; Skills with trust and context assembly; an external provider run outside tests.

**Implemented and tested, not real-validated**: multi-project Runtime (two-vault run not recorded); Work Packages/Items; capability catalog and external provider protocol; remote jobs (`Resumable`/`Detachable`); Blender `scene.inspect`; `asset.generate` placeholder; Meshy, Tripo, Replicate, fal, ElevenLabs adapters (fakes only, no real API calls); Skill hash pinning; Playbooks first slice; webhooks; artifact index.

**Run for real in tests**: Codex and OpenCode agent providers, Pandoc and ffmpeg converters, MCP client phase 1 against the reference filesystem server.

## Behavior a migration must preserve
These are covered by Weave tests and are the strongest candidates for Amaru acceptance tests:
- Capability → Provider boundary; domain/app/ports import no OS, provider or MCP packages (`runtime/internal/arch/arch_test.go`).
- Execution state machine and restart semantics (`runtime/internal/app/*_test.go`, `persist_test.go`, `resume_test.go`).
- Protocol shapes validated by the same fixtures in Go and TypeScript.
- Security boundary: loopback bind, bearer token, Origin allowlist, env allowlist, stdin-only prompts, writes confined to `.weave/`, no auto-approved agent permissions (`docs/security.md`).
- Deterministic context assembly with Agent/Skill snapshots and `context.md` evidence (ADR-011).
- Provider outputs as role + project-relative path inside the run's output directory; the Runtime computes hashes (ADR-012).

## Architecture decisions
Weave ADRs (`docs/decisions/`), status at snapshot:

| ADR | Topic | Status |
|---|---|---|
| 001 | Architecture | proposed (partly superseded by 009) |
| 002 | Execution persistence | accepted |
| 003 | `minAppVersion` | no status line (1.7.2 derived, never executed) |
| 004 | Work persistence | accepted |
| 005 | Artifact index | accepted |
| 006 | `asset.generate` | accepted |
| 007 | Workflow | accepted, partly superseded by 008 |
| 008 | Workflow step execution | accepted |
| 009 | Capability provider (Blender) | accepted |
| 009 | Multi-project Runtime | accepted (duplicate number) |
| 010 | Agents | accepted |
| 011 | Skills | accepted |
| 012 | Capability providers extension | accepted |
| 013 | Workflow graph | accepted |
| 014 | Webhooks | accepted |
| 015 | `.blend` preview | declined |
| 016 | MCP client provider | accepted for phase 1 |
| 017 | Skills evolution | proposed (parts built) |
| 018 | Playbooks | proposed (first slice built) |
| 019 | Teams | proposed |
| 020 | Agents evolution | proposed |

## Known gaps (Weave ROADMAP §8)
GAP-05 Windows/macOS unvalidated · GAP-06 no change events for Work/Workflows/Agents · GAP-07 no history retention/redaction/schema migration, two Runtimes per project unsafe · GAP-08 several validations not recorded · GAP-09 no capability permission model · GAP-10 setup wizard partial · GAP-11 UI not compared with references · GAP-12 `asset.generate` SVG only · GAP-13 `minAppVersion` never executed.

## Outstanding cycles (not Amaru work)
Meshy against the real API (blocked on a paid key); decisions on proposed ADR-017–020; Runtime autostart; MCP phase 2; Reviews, Signals, Pipelines (planned, no ADR).

## Discrepancies noticed while reading
Recorded here only; **not fixed in Weave** (out of scope for the bootstrap).
- Weave ROADMAP GAP-01 says "Git has a single commit … almost everything exists only in the uncommitted working tree"; the repository now has 36 commits and a clean tree.
- Weave `CLAUDE.md` lists Work Package, Work Item, Workflow, Artifact, Skill in the MVP domain; Weave `PRD.md` lists them as out of scope / future. Weave ROADMAP §10 already notes this.
- Two ADRs share number 009 (also noted in Weave ROADMAP §10).
