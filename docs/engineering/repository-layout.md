# Repository Layout

What exists now, and the target layout areas are introduced into ([ADR-0008](../architecture/adr/0008-incremental-monorepo-layout.md)).

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
│   ├── product/            PRD, feature catalog, NFRs, glossary
│   ├── architecture/       architecture hub + views, roadmap, work packages, decisions + adr/, open questions
│   ├── engineering/        conventions, this file, agent instructions, validation
│   ├── security/           security model
├── spikes/wp-m1-02/        Go module: WP-M1-02 validation experiments (throwaway, not the Platform)
├── packages/contracts/     versioned API, event, Provider and Worker contracts, shared fixtures, route-drift guard (Go module, Protobuf, TypeScript test tooling)
└── tools/doccheck/         Go (stdlib only): offline documentation checker
```

There is **no LICENSE file** yet: the license is decided in [ADR-0026](../architecture/adr/0026-fair-code-community-license-with-commercial-licensing.md) and stated in the [README](../../README.md#license); the product owner adds the root file before the first public release or external contribution, and each package's Apache-2.0 file when the package is created (WP-M1-01).

## Target areas (not created)

| Path | Purpose | Introduced when (evidence required) |
|---|---|---|
| `packages/sdk` | Client SDK generated from contracts (TS first) | M2: contracts exist; the Web client needs it. |
| `services/platform` | Go Platform (API, domain, coordinator, embedded Worker) | [M3](../architecture/ROADMAP.md#m3-platform-core): M1 validation spike passed and a migration ADR moves prototype Runtime code here ([ADR-0011](../architecture/adr/0011-platform-adapted-from-earlier-internal-runtime.md)). |
| `apps/web` | Web client served by the Platform | [M4](../architecture/ROADMAP.md#m4-local-mvp): Web is the MVP client ([ADR-0010](../architecture/adr/0010-local-mvp-client-is-a-web-ui.md)). |
| `packages/plugin-sdk` | Plugin author SDK and manifest schema | M6: implements [ADR-0041](../architecture/adr/0041-isolation-tiers-for-plugins-providers-and-steps.md)–[ADR-0021](../architecture/adr/0021-plugin-packaging-signing-permissions-and-compatibility.md). |
| `packages/ui`, `packages/app` | Shared design system and product screens | When Desktop (M8) needs to share them; until then inside `apps/web`. |
| `apps/desktop` | Desktop native host | M8: Tauri v2 per [ADR-0018](../architecture/adr/0018-desktop-shell-is-tauri-v2.md), after its validation spike. |
| `.github/workflows/` | Hosted CI | Requested by the product owner; mirrors `make check`. |

Rules for introducing an area:
1. It has real content, a README stating purpose and owner, and a validation target in the root Makefile that `make check` runs.
2. Language workspaces (`go.work`, `pnpm-workspace.yaml`, Cargo workspace) arrive with the second module of that language.
3. It follows the [conventions](CONVENTIONS.md) and the boundaries in [ARCHITECTURE](../architecture/ARCHITECTURE.md).
4. Use the `repository-bootstrap` Skill (`.agents/skills/repository-bootstrap/SKILL.md`).

## Why `spikes/wp-m1-02` exists now
WP-M1-02 ([#3](https://github.com/rluders/amaru/issues/3)) must validate bundled PostgreSQL, embedded JetStream and restart semantics by running them, and the evidence must be reproducible. The code is a separate Go module with its own pinned dependencies, [README](../../spikes/wp-m1-02/README.md) and `make spike-vet` / `make spike-m1-02` targets. It holds no Platform code and no code from earlier prototypes; it is removed or replaced when `services/platform` is introduced (M3). Its third-party dependencies are scoped to the spike module and do not make the Platform depend on them.

## Why `packages/contracts` exists now
WP-M2-01 ([#9](https://github.com/rluders/amaru/issues/9)): the contract must exist, versioned and checkable, before the Platform, SDK or Workers that depend on it. It was introduced with the accepted migration ADR ([ADR-0047](../architecture/adr/0047-migrate-the-prototype-protocol-into-packages-contracts.md)) and holds OpenAPI, JSON Schemas, the Worker Protobuf and a route-drift guard. It is a separate Go module (the guard reads YAML, which the standard library cannot, so it pins `gopkg.in/yaml.v3`); `make contracts-test`, `make contracts-proto` and `make contracts-ts` are in `make check`. The shared fixtures (API-002, [#117](https://github.com/rluders/amaru/issues/117)) are validated in Go and TypeScript, which adds a Node toolchain inside the package: a `package.json` and committed pnpm lockfile for test tooling only, not a workspace (a workspace comes with a second Node package, such as the SDK in WP-M2-02). Its [README](../../packages/contracts/README.md) states what is and is not there; the package `LICENSE` is added by the product owner, not by agents.

## Why `tools/doccheck` exists now
The bootstrap's deliverable is documentation whose value depends on staying consistent: links, ADR statuses and Skill adapters. Off-the-shelf link checkers (markdownlint, lychee) are not installed here and would not check the ADR index or the Skill symlink scheme. A stdlib-only Go tool adds no dependencies, matches the Platform's planned language, and has unit tests.
