# Repository Layout

What exists now, and the target layout areas are introduced into ([ADR-0008](../architecture/adr/0008-incremental-monorepo-layout.md), proposed).

## Current

```
amaru/
├── README.md               public entry point
├── AGENTS.md               shared agent policy (authoritative)
├── CLAUDE.md               Claude Code entry; imports AGENTS.md
├── PRD.md, ARCHITECTURE.md short indexes into docs/
├── Makefile                local validation (make check)
├── .agents/skills/         canonical Skills (SKILL.md per directory)
├── .claude/skills/         symlinks to .agents/skills/* for Claude Code
├── docs/
│   ├── product/            PRD, glossary
│   ├── architecture/       architecture, roadmap, decisions + adr/, open questions
│   ├── engineering/        conventions, this file, agent instructions, validation
│   ├── security/           security model
│   └── migration/          Weave migration plan and inventory snapshot
└── tools/doccheck/         Go (stdlib only): offline documentation checker
```

There is **no LICENSE file**: the license is undecided ([Q12](../architecture/OPEN_QUESTIONS.md#q12-licensing-and-commercial-boundaries)).

## Target areas (not created)

| Path | Purpose | Introduced when (evidence required) |
|---|---|---|
| `packages/contracts` | API, event, provider and Worker schemas + golden fixtures | Stage 3: API versioning ([Q11](../architecture/OPEN_QUESTIONS.md#q11-api-compatibility-and-versioning)) decided; first schema drafted. |
| `services/platform` | Go Platform (API, domain, coordinator, local Worker) | Q1/Q3 decided, contracts exist; or a migration ADR moves Weave Runtime code here. |
| `packages/sdk` | Client SDK generated from contracts (TS first) | First client needs it. |
| `packages/plugin-sdk` | Plugin author SDK and manifest schema | Stage 4: Q6/Q7 decided. |
| `packages/ui` | Shared React components | Second client (Desktop + Web) or a migrated component needs sharing. |
| `apps/desktop` | Desktop native host + UI | Stage 5: Q5 decided by a spike. |
| `apps/web` | Web client | Self-hosted/Cloud stage, or Web chosen as first client. |
| `.github/workflows/` | Hosted CI | Requested by the product owner; mirrors `make check`. |

Rules for introducing an area:
1. It has real content, a README stating purpose and owner, and a validation target in the root Makefile that `make check` runs.
2. Language workspaces (`go.work`, `pnpm-workspace.yaml`, Cargo workspace) arrive with the second module of that language.
3. It follows the [conventions](CONVENTIONS.md) and the boundaries in [ARCHITECTURE](../architecture/ARCHITECTURE.md).
4. Use the `repository-bootstrap` Skill (`.agents/skills/repository-bootstrap/SKILL.md`).

## Why `tools/doccheck` exists now
The bootstrap's deliverable is documentation whose value depends on staying consistent: links, ADR statuses and Skill adapters. Off-the-shelf link checkers (markdownlint, lychee) are not installed here and would not check the ADR index or the Skill symlink scheme. A stdlib-only Go tool adds no dependencies, matches the Platform's planned language, and has unit tests.
