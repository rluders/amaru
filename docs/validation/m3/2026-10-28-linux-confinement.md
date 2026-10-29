# Agent write confinement (#154, ADR-0057) validation record: Linux

| Field | Value |
|---|---|
| Date | 2026-10-28 |
| Commit | the head of the `feat/154-agent-write-confinement` branch at the PR (the code under test) |
| OS | Fedora Linux 43 Workstation, kernel 7.2.11-100.fc43.x86_64, x86_64 |
| Landlock ABI | 10 (read from the kernel by the tests; `/sys/kernel/security/lsm` lists `landlock`) |
| Tools | go1.27.0 in `services/platform`; no third-party dependency added |
| Run by | AI agent in the product owner's session |

Scope: Linux, one host. Nothing here says anything about macOS, Windows, other kernels or CI. The real-kernel tests skip with a message when Landlock ABI 3 is absent, so on such a host the confinement is **not validated**.

## Commands and results
| Command | Result |
|---|---|
| `make check` | exit 0 (local, Linux) |
| `make docs-check` | ok |
| `GOOS=windows` and `GOOS=darwin` `go build ./...` / `go vet` of the touched packages | build ok (compiles; the confinement is a refusal there) |
| Mutation: the ruleset is never applied (`landlock_restrict_self` skipped) | 2 tests fail |
| Mutation: truncate not handled | 1 fails |
| Mutation: reparenting (`REFER`) not handled | 1 fails (rename across directories inside the root) |
| Mutation: device nodes not allowed | 1 fails |
| Mutation: create-file / remove-file rights not handled | 1 fails each |
| Mutation: ABI check removed from the refusal | 1 fails |
| Mutation: home-directory rule removed / `~/.ssh` rule removed | 1 fails each |

## Evidence by claim
| Claim | Evidence level | What shows it |
|---|---|---|
| A confined process and its descendants cannot write outside the roots | **integration-tested** (real kernel, one host) | A re-executed test binary through the launch helper: create, mkdir, append, truncate, remove outside the root are denied and the outside file is unchanged; the same inside the root succeed; a descendant in a new session (`setsid`) is denied too |
| Links do not escape | **integration-tested** | Writing through a symlink that points out, moving an outside file in, hard-linking one in, and moving an inside file out are all denied; renames across directories inside the root work |
| A Step is refused when confinement cannot be enforced | **unit-tested** | Fake probe: no mechanism, ABI 2, no helper configured; no process is started |
| Unsafe writable roots are refused | **unit-tested** | Relative, `..`, missing, a file, `/`, the home directory, a symlink to it, above it, inside `~/.ssh`, the Project checkout and above it |
| The helper fails closed | **unit-tested** | Bad usage exits 125 without running anything; the helper marker never reaches the agent or comes from a spec |
| The worktree is the agent's only root, the checkout is not writable | **integration-tested** | `worktree` test: a confined `sh` writes in its worktree and is denied in the checkout; the checkout is refused as a root |
| Works with a real agent CLI | **not validated** | No vendor CLI was run under confinement |

## Limits, stated
- Reads, the network and signals are not restricted. A confined agent can read what the user can read and use the network (egress consent is #168).
- Landlock does not restrict file metadata changes (`chmod`, `chown`, `utime`, extended attributes; kernel user guide). A confined agent can still change the mode or timestamps of files it can reach outside the root. This is a gap in what ADR-0057 promises ("writes") and is recorded here.
- `/dev/null` and `/dev/tty` are writable so programs can discard output; nothing else under `/dev` is.
- An agent cannot `git commit` inside its worktree: git writes to the shared repository under the Project checkout, which is not a writable root. The Worker commits on its behalf.
- A confined agent has no writable temporary directory unless its Provider declares one.

## Not covered by this record
- No Worker `main` exists yet, so nothing calls `ConfineExecMain` in production code and no Provider starts a confined agent. The enforced roots are not recorded in a Step result or Event (the Worker `Result` has no outputs field).
- Per-Provider runs (WP-M3-05); macOS and Windows (always refuse); CI (never run for this repository).
