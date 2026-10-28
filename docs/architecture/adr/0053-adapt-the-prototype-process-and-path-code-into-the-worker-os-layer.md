---
title: Adapt the prototype process, executable-lookup and environment code into the Worker OS layer
status: proposed
date: 2026-10-28
deciders:
---

# 0053. Adapt the prototype process, executable-lookup and environment code into the Worker OS layer

## Context
- [ADR-0007](0007-reuse-of-earlier-internal-code-is-a-separate-activity.md) bars moving code from the earlier internal implementation until a human approves a migration ADR for that component. This is the draft migration ADR for the `platform` package of its Runtime (`runtime/internal/platform`; `MADR-platform`, issue #78, sub-issue of WP-M3-03 / #15). Every issue that moves this component is gated on it ([delivery §8](../../engineering/delivery.md#8-human-approval-gates)); in M3 that is FEAT-EXE-003 (#152, cancellation with process-tree termination).
- **Source (verified 2026-10-28):** the Runtime repository of the earlier internal implementation (named in the private migration notes, not part of this repository), baseline commit **`c8a2f6e`** ([ADR-0032](0032-migration-baseline-is-a-pinned-commit.md)). From a `git archive` of that commit:

  | File | Lines | Role |
  |---|---|---|
  | `process.go` | 155 | `Runner.Start`: argv only (never a shell), explicit working directory, allow-listed environment, stdin or an interactive stdin pipe, `Terminate(grace)`, termination on context cancel |
  | `process_unix.go` | 27 | child in its own process group (`Setpgid`); `SIGTERM` then `SIGKILL` to the group |
  | `process_windows.go` | 33 | child in a new process group; `taskkill /T` (and `/F` after the grace period) |
  | `resolver.go` | 80 | executable lookup: explicit path, `PATH` (`exec.LookPath`, `PATHEXT` on Windows), then per-user install directories |
  | `paths.go` | 46 | per-user state directory, `WEAVE_STATE_DIR` override |
  | tests | 143 | 8 test functions: environment allow-list, stdin, terminate, context cancel, resolver, state directory |

  It imports only the standard library and `ports` (verified with a scan of the import blocks). `go test -count=1 ./internal/platform/...` passes on the extracted tree (2026-10-28, Linux).
- **Planned disposition** (migration plan, `MADR-platform`): **Reuse** into the Worker OS layer. This draft proposes **Adapt** instead, for the reasons below; the choice is item 2 of the Decision and is the owner's.
- **What the source does not give us.**
  - **The tree is not tested.** `TestTerminateStopsProcess` and `TestContextCancelTerminates` start one helper process that sleeps and check that it ends. No test starts a grandchild, so "terminates the whole process tree" ([FEAT-EXE-003](../../product/FEATURE_CATALOG.md#32-workflows-and-execution)) is a design intention of the process-group code, not tested behavior.
  - **Windows is unvalidated.** The catalog records "Windows variant unvalidated" and names GAP-05. `taskkill /T` walks the parent-process chain at call time: a descendant whose parent has already exited is not found, and the call depends on `taskkill.exe` being present and on `PATH`.
  - **Unix process groups can be escaped.** A descendant that calls `setsid` or `setpgid` leaves the group and survives `SIGKILL` to it. The source does not say so.
  - **Vendor names in the OS layer.** The environment allow-list passes the prefixes `ANTHROPIC_` and `CLAUDE_`, and `resolver.go` searches `~/.claude/local` and a `claude` install directory. Amaru's rule is that the core and the shared layers name no vendor ([AGENTS.md](../../../AGENTS.md#architecture-boundaries-accepted-not-implemented-see-architecture)); what a Provider needs in its environment is the Provider's declaration.
  - **A prototype-only concern.** `paths.go` locates the discovery files of a per-user daemon and is named for the prototype (`WEAVE_STATE_DIR`). [ADR-0049](0049-adapt-the-prototype-domain-and-ports-into-services-platform.md) already drops the ports for the per-user daemon; Amaru's Platform configuration is decided elsewhere ([ADR-0039](0039-configuration-scopes-and-resolution-order.md)).
- **What Amaru already has.** The embedded Worker ([#145](https://github.com/rluders/amaru/issues/145)) hands a `Runner` a context that ends at the Lease revocation, Worker stop or, since [#212](https://github.com/rluders/amaru/issues/212), the Step's Deadline, and states that the Runner must stop what it started. Nothing in `services/platform` starts a process yet, and the ports for it (the prototype's `ProcessRunner`, `ProcSpec`, `ExecutableResolver`) do not exist in `internal/ports`; per ADR-0052 item 5 they are defined with their first implementation.
- **Placement constraint.** `internal/arch` forbids the core from importing `internal/platform`, `internal/adapter`, `internal/provider` and `internal/store`, and forbids `os`, `os/exec`, `syscall` and the network in the core. The OS layer therefore lives outside the core, wired in the composition root. A Go package named `platform` inside the module called `platform` would also be confusing.
- **Dependencies.** The Platform module has no `golang.org/x/sys` requirement today (verified in `go.mod`, 2026-10-28). Windows Job Objects are not in the standard library's `syscall` package.
- **Unverified:** that the allow-list below is complete for the Providers of WP-M3-05; that a Job Object contains every descendant a Windows tool starts (some can be created outside the job with `CREATE_BREAKAWAY_FROM_JOB` if the job allows it); behavior on macOS.

## Decision
1. **Component and source.** Migrate `process.go`, `process_unix.go`, `process_windows.go` and `resolver.go` of `internal/platform`, with their tests, at commit `c8a2f6e` (ADR-0032). No later commit is used.
2. **Disposition: Adapt, not Reuse.** The prototype is a behavioral reference and migration source. The changes in items 4 and 5 are required by the acceptance criteria of FEAT-EXE-003 and by the vendor-neutral rule, so the code does not move unchanged. `services/platform` is canonical from the first commit; the prototype is not kept in sync.
3. **History option: fresh import with provenance** (migration plan default), as in ADR-0049: the source commit is recorded here, in `services/platform/README.md` and in `PROVENANCE.md` with each file's source path and blob id. No `git subtree` or `filter-repo`.
4. **File disposition.**

   | Source (`internal/platform/`) | Disposition | Target / note |
   |---|---|---|
   | `process.go` | Adapt | Process runner behind a port defined with its first user (#152). Keeps argv-only start, explicit directory, stdin and interactive stdin, grace-then-kill, kill on context end. The environment allow-list keeps the OS and proxy names and drops `ANTHROPIC_` and `CLAUDE_`: a Provider passes what it needs through the Step's environment declaration |
   | `process_unix.go` | Adapt | Process group as in the source, plus a grandchild test. The `setsid` escape is documented as a limit, not hidden |
   | `process_windows.go` | Adapt, **mechanism requires human decision** | See item 5 |
   | `resolver.go` | Adapt | `PATH` and explicit-path lookup move. The per-user install directories name a vendor's CLI (`claude`); they move to the Provider that needs them, which passes extra search directories |
   | `paths.go` | **Not moved** | Per-user daemon state directory. The Platform's own data and configuration locations follow ADR-0039 and are decided with the feature that needs them |
   | Source ADRs, dogfood records | Reference only | cited, not copied |

5. **Windows termination mechanism (decision requested).** Options: **(a)** keep `taskkill /T /F` as in the source: no new dependency, but the weaknesses above; **(b)** a Windows Job Object created with `golang.org/x/sys/windows` and `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE`, the child assigned to the job, closing or terminating the job ends every process in it, including a descendant whose parent exited. This draft recommends **(b)**: it is the mechanism that can meet "terminates the whole process tree", and it needs one new dependency (BSD-3-Clause; pinned version and license recorded in the implementing PR, per the dependency rule in AGENTS.md). If the owner chooses (a), #152 states that Windows termination is best-effort and does not claim the tree. Either way Windows stays **not validated** until a run on Windows is recorded; validation is Linux-first ([validation](../../engineering/validation.md#cross-platform-validation-m3-and-m4)).
6. **Placement.** The OS layer is a package outside the core, for example under `internal/worker/` (not named `platform`), importing only the standard library, `ports` and, for Windows, `golang.org/x/sys/windows`. `internal/arch` is extended so that `ports`, `domain`, `coordinator` and the embedded Worker still import none of it; the composition root wires it in. The package layout is fixed in the implementing PR under the [repository layout](../../engineering/repository-layout.md) rules.
7. **Renames** follow the [glossary term map](../../product/GLOSSARY.md#2-term-mapping-from-predecessor-repositories); `weave`, `orkestra` and `WEAVE_*` identifiers are not carried. No such string remains in `services/platform` except in `PROVENANCE.md`.
8. **Behavior-preserving tests that move.** The six prototype tests of the moved files: environment allow-list drops secrets, stdin delivered, terminate stops the process, context cancel terminates, resolver. They run in `make check` on Linux with fakes and a re-executed test binary as the helper process (no real agent or paid API), with macOS and Windows best-effort. Changed or added, listed in `PROVENANCE.md`: the allow-list test drops the vendor prefixes; **new** tests start a child that starts a grandchild and require both to end after `Terminate` and after context cancel, and require the grace period to be honored; a test shows that a descendant that leaves the group is **not** killed on Unix (the documented limit). None is edited merely to pass. `TestStateDirEnvOverride` and `TestStateDirDefaultIsAbsoluteAndNamed` do not move with `paths.go`.
9. **Gate.** No file from the source is added to this repository until this ADR is `accepted` with a recorded approval. Only then does #152 start moving code. Merging this draft is not acceptance; issue #78 stays open until the decision.

## Readiness criteria (migration plan §5)
| # | Criterion | State | Evidence |
|---|---|---|---|
| 1 | Source side stable at `c8a2f6e` | **Met** | ADR-0032; commit verified; no later commit used |
| 2 | Inventory refreshed with tests run | **Met** | WP-M1-07 (#8) closed; `internal/platform` tests re-run at `c8a2f6e` on 2026-10-28 (see Context) |
| 3 | Target ADRs accepted | **Met** | ADR-0013, 0042, 0044, 0049, 0052 accepted. The Windows mechanism (item 5) is left to this ADR's approval |
| 4 | Contracts stable, with fixtures, in Amaru | **Met** | M2 exit validation (#12) closed. The OS layer touches no contract; the Deadline and the `timeout` code already exist (#212) |
| 5 | Behavior covered by tests that run in Amaru | **Met by plan** | Item 8; becomes evidence in #152 |
| 6 | Verified execution | **Gap, listed** | Tree termination was never exercised by the source (one process, no grandchild) and Windows was never run. Accepting this ADR accepts that the first evidence is Amaru's own tests, Linux only |
| 7 | License resolved | **Met** | The source is MIT, © Ricardo Lüders, and the holder consents to relicensing ([ADR-0026](0026-fair-code-community-license-with-commercial-licensing.md)). No license file is added by this work |
| 8 | Explicit product-owner approval | **Not met** | Awaiting the owner. This draft is `proposed` |

## Consequences
- **Easier:** the 300 lines that matter (start, group, grace, kill, lookup) come with a tested shape, and the FEAT-EXE-003 acceptance is tested against a real grandchild instead of assumed.
- **Harder:** the Windows mechanism is a dependency decision, and the Windows part cannot be validated on the Linux-first validation plan; it ships labelled not validated.
- **Security and compatibility:** the environment allow-list is a security control (secrets of other tools are dropped); narrowing it by removing vendor prefixes means a Provider must declare what it needs, which is the intent. No secrets or personal data are transferred (scanned in each PR). Amaru promises no compatibility with the prototype (no importer, owner decision of 2026-10-27).
- **Maintenance:** the OS layer has a single owner (the Platform owner); the prototype copy is frozen.
- **Follow-up:** after acceptance, update the migration inventory in the private notes and mark these files migrated (separate approval in the source repository). The port for processes is defined in #152. Revisit if a Provider needs a confinement the process group cannot give (cgroups, sandboxes: a new ADR, since no confinement ADR exists yet; see POL-006 in the feature catalog).

## Alternatives considered
- **Reuse unchanged (the planned disposition):** fastest, but moves vendor names into the shared layer, ships a tree-termination claim with no tree test, and keeps the weakest Windows mechanism without saying so.
- **Reimplement from the documents only:** loses the small amount of validated behavior (argv-only, allow-list, grace-then-kill, lookup order) for no gain; the source is small enough to review whole.
- **A process-supervision library:** a new dependency for a feature of about 200 lines, and none was evaluated against the three-OS requirement. Not rejected on merit, only not needed to start.
- **Containers or cgroups for confinement and kill:** stronger on Linux, unavailable on the other two OSes in the compact install, and a separate infrastructure decision (AGENTS.md rule 6).

## Approval
Proposed by: Claude (migration ADR draft for #78, 2026-10-28). Not yet approved by the product owner; `deciders` stays empty until then.
