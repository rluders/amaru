---
name: repository-bootstrap
description: Add or restructure top-level areas, packages, toolchains, workspace files or Makefile targets in the Amaru monorepo safely and incrementally. Use when creating apps/, services/, packages/ or tools/ entries, introducing a language toolchain, or changing repository structure or ignore rules.
---

# Repository bootstrap

## Purpose
Grow the monorepo one real component at a time, without empty scaffolds, fake code or lost work.

## When to use
- Creating a new directory under `apps/`, `services/`, `packages/` or `tools/`.
- Introducing Go/pnpm/Cargo workspaces, formatters, linters or CI.
- Moving or renaming top-level files.

## Preconditions (stop and ask if any fails)
- The component has real content now and a documented purpose ([repository-layout](../../../docs/engineering/repository-layout.md) "introduced when" column is satisfied).
- Any infrastructure dependency has an accepted ADR (ADR-0008).
- Migrating Weave code requires an approved migration ADR (ADR-0007). Otherwise do not copy it.

## Workflow
1. **Inspect state**: `git status`, `git log -1`, existing files at the target path, ignored files. Never overwrite or delete unseen content; never discard uncommitted work.
2. **Verify tooling** you will use (`go version`, `pnpm --version`, `cargo --version`, official docs for generators). Do not invent CLI flags.
3. **Create the minimum**: the code, a `README.md` (purpose, owner, how to build/test), and tests.
4. **Wire validation**: add Makefile targets and include them in `make check`. A component without a validation target is not done.
5. **Dependencies**: justify each, pin via lockfile, verify license ([CONVENTIONS](../../../docs/engineering/CONVENTIONS.md#dependencies)).
6. **Ignore rules**: add build outputs to `.gitignore`; never ignore source or lockfiles.
7. **Workspaces** only with the second module of a language.
8. **Docs**: update the layout table in repository-layout, README layout section, and AGENTS/CLAUDE commands if new commands exist.

## Output
The new component, its Makefile wiring, updated docs, and a short note: what was created, why now, what it depends on, how it is validated.

## Verification
- `make check` passes with the new target included.
- `git status` shows only intended files; no build output is staged.
- README and layout docs list only commands that work.
