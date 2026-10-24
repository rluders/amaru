# Validation

What can be checked locally today, and how to report it.

## Commands

| Command | What it does | Needs |
|---|---|---|
| `make check` | Runs all of the below. | Go ≥ 1.27, or Go ≥ 1.21 with `GOTOOLCHAIN=auto` (downloads 1.27) |
| `make fmt` | Fails if any Go file under `tools/doccheck` is not `gofmt`-formatted. | Go |
| `make vet` | `go vet` on `tools/doccheck`. | Go |
| `make test` | Unit tests of `tools/doccheck` (`-count=1`). | Go |
| `make docs-check` | Runs `doccheck` on the repository: relative links and anchors, unclosed fences and empty Mermaid blocks, ADR names/frontmatter/numbering and the DECISIONS index, Skill frontmatter and `.claude/skills` symlinks. | Go |

`doccheck` is offline: it does **not** check external URLs, Mermaid syntax (only that blocks are non-empty and closed), spelling, or whether prose is true. Those need review (`documentation-maintenance` Skill).

No hosted CI is configured; local results are not CI results.

## Evidence levels
Use these words in reports and docs:
- **Unit-tested**: automated tests with fakes cover the behavior.
- **Integration-tested**: automated tests exercise real components together (real process, real files, real network on loopback).
- **Real-validated**: a recorded manual or end-to-end run against the real tool or service, with date, versions and result written down in the repo.
- **Not validated**: none of the above. Say so.

A test that was not run is not evidence. A green local run is not CI. A fake-backed test is not real validation.

## Reporting
State for each check: exact command, outcome (pass/fail, counts), and anything skipped and why (missing tool, no network, platform not available).
