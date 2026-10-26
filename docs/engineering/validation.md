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

## Cross-platform validation (M3 and M4)

Status: **adopted for planning** (backlog refinement, 2026-10-25); no Platform code exists yet, so none of this has been run. M3 and M4 exit criteria require results on Linux, macOS and Windows ([roadmap M3](../architecture/ROADMAP.md#m3-platform-core), [M4](../architecture/ROADMAP.md#m4-local-mvp), PRD §12, NFR-1). No hosted CI is planned, so the matrix is run on real machines and recorded. The criteria are not relaxed because CI is absent.

### Platforms
A run counts for an OS only when it executes natively on that OS: Linux (x86_64), macOS (Apple silicon) and Windows (x86_64, 10 or 11), on physical machines or full VMs. WSL, Docker on macOS or Windows, Wine and cross-compilation do not count for the OS they emulate; `GOOS=<os> go vet`/`go build` on another machine is a useful early signal, not evidence.

### What runs where
| Check | Linux | macOS | Windows | Notes |
|---|---|---|---|---|
| `make check` (fmt, vet, unit and architecture tests, `docs-check`, and the targets M2–M4 add: contract fixtures, SDK staleness, Web unit tests) | native | native | native | WP-M3-01 and the M3 exit require ported tests green on all three. On Windows, run it from Git Bash with GNU Make, or run the commands the Makefile target lists. |
| Storage, migration and start-up-lock tests on bundled PostgreSQL | native | native | native | Socket on Linux/macOS, loopback port + SCRAM on Windows (DEPLOYMENT). |
| Crash-injection suite, timeouts, restart recovery | native | native | native | Process and file-lock semantics differ per OS (WP-M3-03). |
| Process-tree cancel | native | native | **native, required** | WP-M3-03 *Done when* names Windows explicitly (GAP-05). |
| Worktree tests (ENV-010) | native | native | native | Path and file-locking behavior. |
| Loopback authentication tests (Origin, Host, CSRF; ADR-0034) | native | native | native | Listener and browser-launch behavior. |
| Browser end-to-end tests (M4) | native | native | native | Chromium on each OS at least. |
| Automated accessibility checks (M4) | one OS | — | — | DOM-level; one OS is enough. |
| Recorded headless run with a real agent provider (M3 exit) | recorded | recorded | recorded | At least one provider per OS; WP-M3-05 also needs one recorded run per provider, on any OS. |
| Install from the signed archive and recorded supervised run on a real Go repository (M4 exit, WRK-004, NFR-1) | recorded | recorded | recorded | No Docker and no admin rights. |

Checks that only read text (`docs-check`, gofmt, pure contract fixtures) give the same result everywhere; they still run in every OS's `make check`, which is cheap.

### Evidence
- **Same commit**: a milestone's OS evidence is for one commit on `main` (the exit candidate), or for the head of the integration PR when a Work Package's *Done when* names three OSes. A later change to code the run covered makes the record stale; re-run before closing.
- **Record**: one Markdown file per OS run under `docs/validation/<milestone>/`, named `<YYYY-MM-DD>-<os>-<scope>.md` (the directory is created with the first record). It states: date; commit SHA; OS name, version and architecture; machine kind (physical or VM); tool versions (`go version`, Node, PostgreSQL bundle, browser, agent CLI); the exact commands; pass/fail and counts per command; skipped or failing tests with the reason; who ran it. Logs too large for the record are attached to the GitHub comment and linked from the record; they must not contain secrets.
- **GitHub**: the person who ran it comments on the Work Package issue (or the exit issue) with the OS, the commit SHA and a link to the record, and ticks that OS in the issue's OS matrix checklist. The record itself is merged through a `docs/` PR.

### Blocking
- A Work Package whose *Done when* names three OSes, the M3 exit issue and the M4 exit Work Package (WP-M4-07) stay open until each OS has a current record. A missing or failing OS is a blocker, recorded as a comment naming the OS and the cause.
- Only the product owner may change an exit criterion, by changing the roadmap in a reviewed PR; a missing environment is never a reason to tick a box.

### Coordination
- Before the first M3 integration PR, the product owner names in the M3 exit issue who provides each OS (person and machine or VM); likewise for M4 in WP-M4-07. Default owner for all three OSes is the product owner until they delegate an OS in a comment on the exit issue (decided 2026-10-25). The machine or VM used for each OS is listed in the issue before its first run.
- The contributor who opens an integration PR runs `make check` on their own OS and requests the other two runs by commenting on the issue; the PR is not merged before all three records exist when the Work Package requires them.
- If hosted CI is introduced later (by an ADR), its runs may replace the native `make check` rows, not the recorded runs.
- **Hosted runners (product owner decision, 2026-10-26, in the WP-M1-02 session):** GitHub-hosted runners (Linux, macOS Apple silicon, Windows) are accepted as the native environment for building and validating the Platform and the desktop app on macOS and Windows. They are full VMs, so they satisfy the native rule above; the record still names the runner image and states what a runner cannot show (for example Windows runners run as an administrator, so non-admin behavior is not shown). The first use is the scoped workflow `.github/workflows/spike-m1-02.yml` for WP-M1-02. Integrations that need a Windows or macOS machine with third-party software installed (for example an Unreal Engine MCP inside a VM) are a separate concern, deferred, and not a blocker for these Work Packages. Introducing a general CI pipeline still needs its own decision; this entry covers only what is named here.
