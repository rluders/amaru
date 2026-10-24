# CLAUDE.md

Shared policy (authoritative, loaded via import):

@AGENTS.md

## Claude Code specifics

This file only adds Claude-specific operation. If anything here conflicts with `AGENTS.md`, `AGENTS.md` wins; fix this file.

- **Skills**: project Skills load from `.claude/skills/`, which are symlinks to the canonical `.agents/skills/`. Edit Skills only under `.agents/skills/`. Use them:
  - designing or reviewing a boundary, dependency or proposal → `architecture-review`
  - any ADR → `adr-authoring`
  - before claiming a change is done → `testing-and-validation`
  - adding a top-level area, package, toolchain or Makefile target → `repository-bootstrap`
  - after changing behavior, decisions or terminology → `documentation-maintenance`
- **Plan first** for anything beyond a small edit: inspect the repo state, then propose. Stop and ask at architecture decisions; use the question tool for real choices, not for confirmation.
- **Verify, then claim.** Check Claude Code, library and platform behavior in official docs (WebFetch) or source before relying on it; cite the source and date in docs that depend on it.
- **Weave reference**: when `weave-obsidian` is available alongside this repo, read it as evidence only. Never write to it from an Amaru task.
- **Final reports** use the split: implemented and locally verified · documented and proposed · not implemented · not validated · requires human decision.
- **Commands**: `make check` (all), `make docs-check`, `make test`. Nothing else exists yet; do not suggest commands for components that are not in the repo.
