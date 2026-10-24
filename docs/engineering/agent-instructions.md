# Agent Instructions and Skills: Loading Model

How instruction files and Skills reach each agent tool, and why the repository is laid out this way. Verified sources and dates are listed; re-verify when a tool changes.

## Files and authority

| File | Role | Authority |
|---|---|---|
| [`AGENTS.md`](../../AGENTS.md) | Shared policy for every agent: boundaries, quality, testing, docs, security, reporting. | **Authoritative** for shared policy. |
| [`CLAUDE.md`](../../CLAUDE.md) | Claude Code entry point. Imports `AGENTS.md`, then adds Claude-specific operation (Skills, tools). | Claude-specific only. Must not restate or contradict `AGENTS.md`. |
| `.agents/skills/<name>/SKILL.md` | Canonical Skill content. | The only place Skills are edited. |
| `.claude/skills/<name>` | Symlink to `../../.agents/skills/<name>`. | Adapter; never a copy. |

When `CLAUDE.md` and `AGENTS.md` seem to disagree, `AGENTS.md` wins and `CLAUDE.md` is fixed.

## Instruction loading by tool

| Tool | Reads | Source / confidence |
|---|---|---|
| **Claude Code** | `CLAUDE.md` in the working directory and parents. When a `CLAUDE.md` exists it reads **only** the `CLAUDE.md` files, not `AGENTS.md`, unless `CLAUDE.md` imports it with `@AGENTS.md` — which ours does. Imports resolve relative to the importing file. | code.claude.com/docs/en/memory, read 2026-10-24. Verified in docs. |
| **Codex and other `AGENTS.md`-aware tools** | `AGENTS.md`. | Widely documented convention; not re-verified for each tool here. |
| **Tools that read neither** | Nothing automatically. Point them at `AGENTS.md` in the task prompt. | — |

`CLAUDE.md` therefore stays short: the `@AGENTS.md` import makes shared policy load for Claude without duplication.

## Skill discovery by tool

| Tool | Project Skill location | Symlinks | Source / confidence |
|---|---|---|---|
| **Claude Code** | `.claude/skills/<name>/SKILL.md` (working directory up to repo root). It does **not** read `.agents/`. | A `<name>` entry may be a symlink to a directory; it reads `SKILL.md` from the target. | code.claude.com/docs/en/skills, read 2026-10-24. Verified in docs. |
| **Codex** | `.agents/skills/<name>/SKILL.md` | Official docs state symlinked skill folders are followed. | Path from secondary sources only (official page was unreachable from this environment on 2026-10-24). **Re-verify.** |

Frontmatter is limited to `name` and `description`: these are the fields both tools document; Claude-only fields (`allowed-tools`, `paths`, …) would be ignored or rejected elsewhere.

## Why symlinks, and the Windows caveat

One canonical copy avoids Skills that silently diverge. `make docs-check` fails if a `.claude/skills/*` entry is a copy, points at the wrong Skill, or a canonical Skill has no adapter.

On Windows, Git checks out symlinks as plain text files unless `core.symlinks=true` and the user may create symlinks (Developer Mode or admin). Windows contributors should enable both before cloning (`git clone -c core.symlinks=true …`). If that is not possible, the Claude adapter will not work on that machine; read Skills directly from `.agents/skills/`. Do not commit copies to work around it.

## Adding a Skill
1. Create `.agents/skills/<name>/SKILL.md` with `name: <name>` and a `description` that says when to use it.
2. `ln -s ../../.agents/skills/<name> .claude/skills/<name>`
3. `make docs-check`.

Create a Skill only for a repeatable procedure with clear inputs, steps, output and verification. Skills in this repo: `architecture-review`, `adr-authoring`, `testing-and-validation`, `repository-bootstrap`, `documentation-maintenance`.
