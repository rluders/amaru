# AGENTS.md — shared policy for coding agents

This file is the **authoritative** policy for every agent and automated tool working in this repository. Tool-specific files (e.g. [CLAUDE.md](CLAUDE.md)) add operational detail and must not contradict it. Loading model: [docs/engineering/agent-instructions.md](docs/engineering/agent-instructions.md).

## What this repository is
Amaru is an engineering execution platform that coordinates humans, AI agents, tools, workflows and execution environments; the engineer stays in control. **This repository currently holds the product foundation only**: documentation, decisions, governance, Skills and a documentation checker (`tools/doccheck`). Earlier internal prototypes exist outside this repository and are **not** part of it.

Start here: [README](README.md) · [PRD](docs/product/PRD.md) · [Architecture](docs/architecture/ARCHITECTURE.md) · [Decisions](docs/architecture/DECISIONS.md) · [Open questions](docs/architecture/OPEN_QUESTIONS.md) · [Glossary](docs/product/GLOSSARY.md).

## Hard rules
1. **Do not claim what does not exist.** Label behavior as implemented-and-verified, proposed, prototype-only, open, or requires-decision ([status vocabulary](docs/engineering/CONVENTIONS.md#status-vocabulary)). Never present planned, simulated or prototype behavior as available in Amaru.
2. **Never invent APIs**, CLI flags, library behavior or platform facts. Verify in official docs or source; if you cannot, say "unverified" and record it.
3. **Decisions belong to humans.** Draft ADRs as `proposed`; never mark an ADR `accepted` yourself ([DECISIONS](docs/architecture/DECISIONS.md#convention)). Significant architecture choices need explicit approval — stop and ask.
4. **Do not migrate prototype code** into this repository, and do not change the prototype for Amaru's sake, without an approved migration ADR ([ADR-0007](docs/architecture/adr/0007-reuse-of-earlier-internal-code-is-a-separate-activity.md)).
5. **No speculative scaffolding.** Create a directory or package only with real content, a stated purpose and a validation target ([repository layout](docs/engineering/repository-layout.md)). No fake implementations, empty packages or placeholder services.
6. **No infrastructure without an ADR**: workflow engines, brokers, databases, object storage, containers.
7. **Never commit secrets.** Never log or print them. Never weaken the security baseline ([security model](docs/security/SECURITY_MODEL.md)); never auto-approve agent permissions.
8. **Preserve existing work.** Look before deleting or overwriting; never discard uncommitted changes; no destructive git operations on shared branches.
9. **License**: decided in [ADR-0026](docs/architecture/adr/0026-fair-code-community-license-with-commercial-licensing.md) (fair-code community license + commercial licensing; Apache-2.0 for contracts and SDKs), and the root [LICENSE](LICENSE) (Amaru Community License 1.0) was added by the product owner's instruction on 2026-10-26 ([#66](https://github.com/rluders/amaru/issues/66)); [`packages/contracts/LICENSE`](packages/contracts/LICENSE) (Apache-2.0) was added at the owner's instruction on 2026-10-26; [`packages/sdk/LICENSE`](packages/sdk/LICENSE) (Apache-2.0) was added at the owner's instruction on 2026-10-26; other SDK license files do not exist yet. Do not change license files or license text without an explicit owner instruction; describe licensing by linking ADR-0026 and the LICENSE; do not call Amaru "open source".

## Architecture boundaries (accepted, not implemented; see ARCHITECTURE)
- The **Platform** owns domain state, Events, Policy and Approvals. **Clients** (Web, Desktop, CLI) only use the public API. **Workers** run Steps through **Providers**; they hold no authoritative state.
- Workflows reference **Capabilities**, never vendors. A new Provider must not change core code, contract shapes or UI architecture.
- Contracts are versioned and UI-agnostic; fixtures are shared by all consumers.
- Domain code is pure: no OS, process, network or vendor imports.

## Quality and testing
- Before claiming done, run `make check` and report the exact result. Validation guide: [docs/engineering/validation.md](docs/engineering/validation.md).
- Tests must not require real agents, paid APIs or specialized tools; use fakes. Real-tool validation is recorded separately with date and versions.
- Distinguish unit-tested, integration-tested, real-validated and not validated. A test you did not run is not evidence; local results are not CI.
- Keep changes minimal and in scope; one logical change per commit.
- Work is tracked as GitHub issues with planning IDs; branches, PR bases and closing rules follow the [delivery workflow](docs/engineering/delivery.md).

## Documentation
- English for code, comments, commits and docs.
- Canonical docs live under `docs/`; root `PRD.md` / `ARCHITECTURE.md` are indexes only. Terms come from the [glossary](docs/product/GLOSSARY.md).
- When code or decisions change, update the affected docs in the same change. `make docs-check` must pass.
- Conventions: [docs/engineering/CONVENTIONS.md](docs/engineering/CONVENTIONS.md).

## Dependencies
Standard library first. Every new dependency needs a stated reason, verified docs, a pinned version and an acceptable license. Current third-party dependencies: none in `tools/doccheck`; `packages/contracts` pins `gopkg.in/yaml.v3` v3.0.1 (OpenAPI YAML), `santhosh-tekuri/jsonschema/v6` v6.0.3 and `google.golang.org/protobuf` v1.36.12 (Go fixture validation), and, for the TypeScript fixture suite from a committed pnpm lockfile, `ajv` 8.17.1, `ajv-formats` 3.0.1, `@bufbuild/protobuf` 2.16.0, `typescript` 5.9.3 and `@types/node` 22.20.5 (reasons, licenses and versions in its [README](packages/contracts/README.md#dependencies); test tooling only, none ships to users); `packages/sdk` pins `openapi-fetch` 0.17.0 (runtime, the typed client) and `openapi-typescript` 7.13.0 (generator, dev only), both MIT, plus `typescript` 5.9.3 and `@types/node` 22.20.5, from its own committed pnpm lockfile ([README](packages/sdk/README.md#dependencies)); the throwaway [`spikes/wp-m1-02`](spikes/wp-m1-02/README.md) module pins `embedded-postgres` v2, `nats-server`, `nats.go` and `pgx` for WP-M1-02 only (reasons and versions in its README); the spike module itself is not a Platform dependency. `services/platform` pins `ariga.io/atlas` v1.3.0 (Apache-2.0; reads the embedded migration directory and executes migration files as a library, [ADR-0023](docs/architecture/adr/0023-self-hosted-oci-image-atlas-migrations-operator-owned-backup.md)) and `github.com/jackc/pgx/v5` v5.11.0 (MIT; PostgreSQL driver, used by the PostgreSQL-tagged tests until an adapter opens a connection), with the transitive modules in its `go.mod` (details: [services/platform README](services/platform/README.md#dependencies)). One of them, `hashicorp/hcl/v2` (MPL-2.0), is linked through Atlas; its acceptance is a pending owner decision.

## Skills
Reusable procedures live in `.agents/skills/<name>/SKILL.md` (canonical; `.claude/skills/` holds symlinks):
- `architecture-review` — boundaries, dependencies, trade-offs, failure modes of a change or proposal.
- `adr-authoring` — write or evaluate an ADR.
- `testing-and-validation` — choose and run the right checks, report evidence honestly.
- `repository-bootstrap` — add or restructure top-level areas and tooling safely.
- `documentation-maintenance` — keep docs consistent with implementation and decisions.

## Reporting
Reports state: what changed (files), what was run (commands + outcomes), what was not validated and why, and what needs a human decision. Separate facts from recommendations.
