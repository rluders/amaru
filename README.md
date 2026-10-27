<p align="center">
  <img src=".github/logo.png" alt="Amaru" width="160">
</p>

<h1 align="center">Amaru</h1>

<p align="center"><strong>Carry the work across every world. The engineer sets the course.</strong></p>

<p align="center">Status: product foundation · Source-available (fair-code) · No release yet</p>

---

*Amaru is an engineering execution platform. In Andean tradition the amaru is the serpent that crosses the worlds and carries water between them; here it carries work between humans, AI agents, tools and environments, and the engineer decides where it flows.*

Amaru coordinates humans, AI agents, tools, knowledge, workflows and execution environments across the software development lifecycle. Humans set intent, scope, quality criteria and approval boundaries; agents carry out delegated work inside explicit policies; workflows coordinate multi-step activities; and every result, decision and piece of evidence stays observable and auditable.

Amaru does not replace Git, GitHub, issue trackers, IDEs, CI or specialized engineering tools. It integrates with them and orchestrates work across them.

> **Status: product foundation.** This repository contains product scope, a feature catalog, an accepted (not yet implemented) architecture, a decision log, a master roadmap, engineering governance and a documentation checker. **No part of the Amaru Platform, Desktop, Web client, Cloud or Self-hosted offering is implemented here yet.** Nothing here is ready to install or use.

## The problem

Work delegated to AI agents and tools is scattered across terminals, chats, CI logs and scripts. There is rarely one record of what an agent was asked, with which context, what it changed and who accepted it; permissions and approvals differ per tool; multi-step work is stitched together by hand and lost on a crash; specialized tools are stuck on specific machines; and workflows get written against a single agent vendor. Details: [PRD §2](docs/product/PRD.md#2-problem).

## Principles

- **Human authority**: consequential actions require explicit, recorded human authorization ([ADR-0006](docs/architecture/adr/0006-human-authorization-for-consequential-actions.md)).
- **Evidence over claims**: results carry the context, inputs, outputs and approvals that produced them.
- **Orchestrate, don't replace**: your repositories and tools stay authoritative.
- **Vendor-neutral**: workflows reference capabilities, not vendors.
- **Incremental**: infrastructure is added when evidence demands it, not in advance.

Intended users: individual engineers working with AI agents first; team leads, self-host operators and plugin authors later ([PRD §3](docs/product/PRD.md#3-target-users)).

## Product surfaces

| Surface | What it is | Status |
|---|---|---|
| **Amaru Platform** | The deployable system: domain model, API contracts, policies, execution coordination. | Proposed (M3) |
| **Workers** | Run Steps in local or remote Execution Environments (Linux, Windows workstations, sandboxes). | Proposed (embedded M3, remote M5) |
| **Amaru Web** | Browser client served by the Platform; the Local MVP client. | Proposed (M4) |
| **Plugins** | TypeScript UI and logic, services, Worker and local adapters, with declared permissions. | Proposed (M6) |
| **Amaru Self-hosted** | The Platform, operated by you. | Proposed (M7) |
| **Amaru Desktop** | Native client wrapping the same UI (Tauri v2 thin Rust host, [ADR-0018](docs/architecture/adr/0018-desktop-shell-is-tauri-v2.md)). | Proposed (M8) |
| **Amaru Cloud** | The Platform, operated as a managed service. | Proposed (M9) |

Milestones (no dates) are in the [master roadmap](docs/architecture/ROADMAP.md); features in the [feature catalog](docs/product/FEATURE_CATALOG.md).

Cloud and Self-hosted are deployments of the same Platform, not separate products ([ADR-0002](docs/architecture/adr/0002-one-platform-multiple-deployment-models.md)). Clients never own execution state ([ADR-0003](docs/architecture/adr/0003-desktop-is-a-client.md)).

## Architecture at a glance (accepted, not implemented)

```mermaid
flowchart LR
    eng([Engineer]) --> clients[Clients<br/>Web · Desktop · CLI]
    clients -->|versioned public API| platform[Amaru Platform<br/>state · policy · approvals · events]
    platform --> workers[Workers<br/>local or remote environments]
    workers --> providers[Providers<br/>agent CLIs · tools · remote APIs]
    providers --> project[(Your repositories<br/>and tools)]
```

Full proposal and what is not built: [docs/architecture/ARCHITECTURE.md](docs/architecture/ARCHITECTURE.md).

## Repository layout

```
.agents/skills/     canonical agent Skills (.claude/skills/ holds symlinks to them)
docs/product/       PRD, feature catalog, non-functional requirements, glossary
docs/architecture/  architecture hub and views, roadmap, work packages, decisions (adr/), open questions
docs/engineering/   conventions, repository layout, validation, agent instructions
docs/security/      security model
tools/doccheck/     offline documentation checker (Go, standard library only)
packages/contracts/ versioned API, event, Provider and Worker contracts (OpenAPI, JSON Schema, Protobuf) shared fixtures and the route-drift guard
packages/sdk/       TypeScript SDK generated from the OpenAPI contract, with an event-stream client (cursor resume)
```

Application, service and package directories are added only when they have real content ([repository layout](docs/engineering/repository-layout.md)).

## Getting started

Requirements: Git and Go 1.27 or newer (Go 1.21+ with the default `GOTOOLCHAIN=auto` downloads 1.27 automatically). On Windows, clone with symlinks enabled: `git clone -c core.symlinks=true …` ([why](docs/engineering/agent-instructions.md#why-symlinks-and-the-windows-caveat)).

```sh
make check        # gofmt, go vet, unit tests and documentation checks
make docs-check   # documentation checks only
```

That is everything that runs today. Start reading at the [PRD](docs/product/PRD.md), then the [architecture](docs/architecture/ARCHITECTURE.md) and the [roadmap](docs/architecture/ROADMAP.md).

## Documentation

- Product: [PRD](docs/product/PRD.md) · [Feature catalog](docs/product/FEATURE_CATALOG.md) · [NFRs](docs/product/NON_FUNCTIONAL_REQUIREMENTS.md) · [Glossary](docs/product/GLOSSARY.md)
- Architecture: [Architecture](docs/architecture/ARCHITECTURE.md) · [Roadmap](docs/architecture/ROADMAP.md) · [Work Packages](docs/architecture/WORK_PACKAGES.md) · [Decisions](docs/architecture/DECISIONS.md) · [Open questions](docs/architecture/OPEN_QUESTIONS.md) · [all documents](docs/README.md)
- Engineering: [Conventions](docs/engineering/CONVENTIONS.md) · [Validation](docs/engineering/validation.md) · [Agent instructions](docs/engineering/agent-instructions.md)
- Security: [Security model](docs/security/SECURITY_MODEL.md)

## Contributing

The project is at the foundation stage and is not yet accepting external contributions. The CLA text ([CLA.md](CLA.md)) and signing process ([CONTRIBUTING](CONTRIBUTING.md#contributor-license-agreement)) are in place; the root [LICENSE](LICENSE) is in effect since 2026-10-26 ([ADR-0026](docs/architecture/adr/0026-fair-code-community-license-with-commercial-licensing.md)). For maintainers and agents: follow [AGENTS.md](AGENTS.md) and the [conventions](docs/engineering/CONVENTIONS.md), record significant decisions as ADRs, and run `make check` before proposing a change.

## License

The root [LICENSE](LICENSE) is the **Amaru Community License 1.0** (fair-code, source-available; Rights Holder LUDERS TECNOLOGIA LTDA), in effect since 2026-10-26 as decided in [ADR-0026](docs/architecture/adr/0026-fair-code-community-license-with-commercial-licensing.md). It permits personal use, Internal Use (including commercial organizations), self-hosting and non-commercial forks; resale, commercial sublicensing and Hosted Services require a separate written commercial license. External contributions require the [CLA](CLA.md). Contracts and SDKs will be Apache-2.0, with a `LICENSE` file added in each package when it is created; until then they carry no separate license. Read the license text for the exact terms. Amaru is not OSI-approved open source and should not be described as such.
