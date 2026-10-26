# Engineering Conventions

Applies to everyone and every agent working in this repository. Shared policy for agents is in [AGENTS.md](../../AGENTS.md); this file holds the detail.

## Language
- **English** for source code, identifiers, code comments, commit messages, technical docs and product docs.
- Human-facing product docs may add other languages later only as translations of the English source.

## Status vocabulary
Every statement about behavior uses one of these, explicitly when it could be misread:

| Label | Meaning |
|---|---|
| **Implemented and verified** | Code exists in this repo and a named check ran and passed. |
| **Implemented, not validated** | Code exists; no test or recorded run proves the behavior. |
| **Proposed** | Designed, recommended or decided (doc, proposed or accepted ADR) but not built. An accepted ADR is a decision, not an implementation. |
| **Exists in the prototype** / **[Prototype]** | Built in an earlier internal prototype, not in Amaru. |
| **Open** | Unresolved; tracked in [OPEN_QUESTIONS](../architecture/OPEN_QUESTIONS.md). |
| **Requires decision** | Waiting on a human. |

Never describe planned or prototype behavior as available in Amaru.

## Markdown
- One `#` title per file; numbered `##` sections in long reference docs so anchors are stable.
- Relative links only for files in this repo; they must resolve (`make docs-check`). Absolute URLs for other repositories.
- Mermaid for diagrams, in ```` ```mermaid ```` fences. Label proposed components in the diagram or its caption.
- Frontmatter only where a tool reads it: ADRs (`title`, `status`, `date`, `deciders`) and Skills (`name`, `description`). Single-line `key: value` pairs; the checker does not parse multi-line YAML.
- Canonical documents live under `docs/`. Root `PRD.md` and `ARCHITECTURE.md` are short indexes; do not duplicate content into them.

## Configuration and secrets
- No secrets, tokens, keys or personal credentials in the repository, ever — including tests, fixtures and docs examples. Use obvious placeholders (`<api-key>`).
- Local configuration uses ignored files (`.env*`, `*.local.*`); commit an `.env.example` when a component needs one.
- Secrets are never logged, printed in errors, or written to events or artifacts.

## Generated files
- Build output (`bin/`, `dist/`, `build/`, `target/`, `node_modules/`) is ignored.
- If a generated file must be committed (e.g. a generated SDK), its header says so, its generator and command are documented, and `make check` fails when it is stale.
- Never hand-edit generated files or lockfiles; regenerate with the tool that owns them.

## Dependencies
- Standard library first. A new dependency needs a stated reason in the change description: what it does, why the standard library or existing code is not enough, maintenance and license status, and size.
- Verify the dependency's API against its official documentation or source before use; never invent APIs.
- Pin exact versions via the ecosystem's lockfile (`go.sum`, `pnpm-lock.yaml`, `Cargo.lock`). Upgrades are separate changes.
- Infrastructure dependencies (workflow engines, brokers, databases, object storage) need an accepted ADR ([ADR-0008](../architecture/adr/0008-incremental-monorepo-layout.md)).
- Current third-party dependencies: **none**.

## Code
- Small, explicit interfaces around real responsibilities, defined by the consumer. No speculative abstractions, no globals, no hidden side effects.
- Domain code is pure: no OS, process, network or vendor imports. OS code lives in a dedicated platform layer.
- `context.Context` on every call that may block (Go). No `sh -c`; argv arrays with explicit cwd and env.
- Go: `gofmt`, `go vet`. TypeScript (when introduced): `strict`, no `any`.

## Versioning
- Contracts are versioned ([ADR-0004](../architecture/adr/0004-shared-versioned-contracts.md)). Within a major version, additive changes only.
- No release versions or tags exist yet; release versioning is decided before the first release.

## Commits and changes
- One logical change per commit; message says what and why.
- Pre-existing uncommitted work is never discarded or overwritten. Look before deleting.
