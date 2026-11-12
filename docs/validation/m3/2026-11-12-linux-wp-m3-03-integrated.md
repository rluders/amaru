# WP-M3-03 integrated Linux validation

Date: 2026-11-12. Scope: [WP-M3-03 #15](https://github.com/rluders/amaru/issues/15), after its final child [PR #319](https://github.com/rluders/amaru/pull/319) merged. Run by Codex in the product owner's session.

Tested source commit: `00d427bab2b821dc32ede9696e3ed37e07029f98` on `feat/15-wp-m3-03-execution-coordinator-with`. It contains all thirteen child deliveries. `main` at `54263cd24aa5bd57f8e978131bcd9695c0c09026` is already an ancestor. This record is a documentation-only follow-up; no covered code changes after the run.

## Environment

- Fedora Linux 43 Workstation, x86_64, kernel `7.2.11-100.fc43.x86_64`.
- Local Codex execution environment; `systemd-detect-virt` reports `container-other`. Physical-machine/full-VM classification is not verified. These are Linux process tests, with no native macOS/Windows claim.
- Root Go 1.26.0; Platform module Go 1.27.0; Node v25.7.0; pnpm 11.5.0; Buf 1.69.0; Podman 5.8.8.
- PostgreSQL 17.11, disposable `docker.io/library/postgres:17.11-alpine` container `amaru-15-integrated-pg`, bound only to `127.0.0.1:55457`. Tests create/drop separate databases. Local trust authentication is confined to this test instance; no production data, real credentials, paid APIs or real agents are used.

## Commands and outcomes

Local checks, not hosted CI. Commands run at the repository root unless stated otherwise.

| Command | Result |
|---|---|
| `make check` | **PASS, exit 0.** Formatting, vet, doccheck, architecture, domain and fake-backed integration suites; 336 contract fixtures have identical Go/TypeScript outcomes; TypeScript corpus suite 338 tests and SDK 17 tests pass. Windows/macOS vet is a compilation signal only. |
| `AMARU_TEST_PG='postgres://postgres@127.0.0.1:55457/postgres?sslmode=disable' GOFLAGS=-json make platform-test-pg` | **PASS, exit 0.** 1,196 passing tests/subtests across 16 tested packages; zero failed/skipped tests. Four packages have no test files. PostgreSQL and Linux process integration-tested. |
| `make docs-check` | **PASS, exit 0**, repeated after adding this record. |
| `git diff --check` and `trail check` | **PASS, exit 0.** No learning conflicts. Lexical advisories refer to historical OS/contract-test wording; the owner-approved Linux-first M3 scope and current passing contract checks are preserved. |

## Integrated acceptance

**The Linux integration criterion is integration-tested:** the crash-injection suite and process-tree cancellation pass together on the source commit above.

- `TestCrashInjectionWithPostgres`: 32 shared crash-suite subtests pass against retained PostgreSQL state. Reconciliation is repeated to check convergence.
- `TestCoordinatorCrashProcess`: 29 scenarios pass, killing actual coordinator test-binary children at state/protocol/assignment/acknowledgement boundaries, then starting fresh coordinator processes over retained PostgreSQL data. Includes persisted Approval waits across two restarts.
- `TestCancelEndsTheProcessTreeOfARunningStep`: both `tree` and `stubborn-tree` pass. A human cancellation travels through the coordinator and embedded Worker to the OS Runner; real child/grandchild processes terminate, including forced termination after the grace period, and Execution/Step cancellation settles. Persistence in this process-tree case uses the fake store; PostgreSQL cancellation cases run separately in the same full suite.
- The full suites also include the embedded Worker interface parity cases, per-Step timeouts, pinned definitions/variables, resumable fake remote jobs, concurrency limits and all seven command-idempotency groups. See the focused [EXE-002](2026-11-12-linux-exe-002.md), [ENV-001](2026-11-12-linux-env-001.md), [EXE-006](2026-11-12-linux-exe-006.md) and [EXE-005](2026-11-12-linux-exe-005.md) records for the detailed assertions and scope. Those historical runs are supplemented by this combined run, not substituted for it.

## Limits and delivery boundary

The crash suite kills a coordinator test binary; PostgreSQL remains alive. PostgreSQL crashes/power loss, a complete serving Platform executable, HTTP authentication, real Providers, and exactly-once external effects are not validated. Process groups do not terminate descendants that deliberately leave the group; that accepted ADR-0053 limit remains covered by `TestADescendantThatLeavesTheGroupIsNotReached` and is not a confinement guarantee.

Native macOS and Windows are **not validated**, best effort for M3 under the [owner-approved Linux-first scope](../../engineering/validation.md#scope-decision-product-owner-2026-11-12). M4 and security requirements are unchanged.

This record supports #15's separate integration review to `main`. It does not authorize that merge, close #15, complete #16–#19, or establish [M3 exit #20](https://github.com/rluders/amaru/issues/20). After #15 merges and is verified, #16 can resume; #168 is its next child with satisfied direct feature prerequisites. #188 and #197 still require their own remaining prerequisites.
