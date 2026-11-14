# ART-002 reader identity and failure boundaries

Date: 2026-11-13 (America/Sao_Paulo). Run by Codex in Ricardo's repository session.
Linux x86_64, kernel 7.2.11-100.fc43.x86_64; physical/VM classification not established.
Tools: Platform Go 1.27.0, host Go 1.26.0, Node 25.7.0, pnpm 11.5.0.
Code: the accompanying PR head on `feat/124-art-002-evidence-bundle-per-execution`;
base `fe979304525614b82b7e2f2886e1ceea625a2ad2` (`main` when selected).

## Selection and delivery snapshot

After #374 merged into #18 at `a28a6e2e8d38d07629bba25ef1a1719bed2a3e22`,
refreshed GitHub native children/blockers, recent merged PRs and remote heads.
PRs #371–#374 all target #18 and are merged. No open PR competed with this slice.
The shared checkout contains unrelated design and agent-tool changes; all work here uses
an isolated worktree at `/tmp/amaru-m3-375`.

| Lane | Native state and base check | Selection |
|---|---|---|
| #16 | Parent #15 closed. #188 open, native #135 closed; implementation `fa8bb47` absent from designated #16 head `d2c393a8a8d4817d18620630cc212644839ae8d0`. #197 open, #124 open/#195 closed. Other native children closed. | Do not start #188 against a base missing its prerequisite. #18 is not an alternative base. |
| #19 | Parent #15 closed. #124 open, #119/#123 closed; #154 open, #85/#120 closed. #125 open, blocked by open #197. #85/#119/#178 closed. Designated branch absent at selection. | Select bounded #124 reader correctness using existing main code/ports. #154's Worker entry/enforced-root integration remains open. |
| #18 | Parent #15 closed. #92/#170/#191/#219 open; other native children closed. Current designated head is the #374 merge above. | No fallback needed; no #18 code imported into #19. |

Created `feat/19-wp-m3-07-worktrees-configuration-scopes` from `main` at the base above
only when concrete #124 work started, per [delivery policy](../../engineering/delivery.md#5-branches-and-pull-requests).
The child PR targets that exact branch. Native issue states are observations, not a claim
that all child acceptance is implemented. No issue closed and no PR merged in this session.

## Change and evidence

`Reader.Bundle` now rejects a returned Execution whose ID differs from the requested ID,
and a successful context lookup whose Execution/Step identity differs from its lookup.
Both return `domain.ErrInvalid` and a zero bundle. This enforces the existing
`ExecutionStore.Get`/`ContextStore.Get` contracts. Errors omit the unexpected IDs.
No new public contract, Event kind, Artifact semantics, dependency or infrastructure.

The new identity regression on unchanged production code failed in two subtests:
`execution` and `context_step` returned nil error. `context_execution` was already
rejected by the domain builder. With the guards, all three pass.

| Command | Result |
|---|---|
| `cd services/platform && go test -count=1 ./internal/evidence -run TestBundleRejectsMismatchedStoreIdentity` before fix | FAIL, exit 1: 2 identity subtests reproduced the defect. Initial sandbox attempt could not write the Go cache; the actual reproduction ran with cache access. |
| `cd services/platform && go test -race -count=1 -v ./internal/evidence` after fix | PASS, exit 0: 3 top-level tests, 3 identity cases, 12 store failure/cancellation cases, and pagination success/second-page failure; no skips. |
| `make check` | PASS, exit 0, local Linux. Includes existing store-backed Evidence tests over fakes, contract/SDK checks, docs-check and cross-OS vet. |
| `git diff --check` | PASS, exit 0. |

**Unit-tested:** mismatched store responses fail closed; wrapped errors/cancellation from
Execution, Event, Approval, audit, context and Artifact reads return no partial bundle.
Audit pagination resumes from the last sequence, filters unrelated Approvals and a different
resource type with the same ID, preserves order, and refuses a bundle if its second page fails.
The existing missing-context path still succeeds through the fake ContextStore in the pagination test.

## Remaining acceptance and limits

#124 and #19 remain open. This is internal reader correctness, not service/API authorization
or Organization/Project isolation. The reader still fetches parts sequentially, not as one
snapshot. `Final` reflects the Execution state read by `Get`; it does not certify a consistent
snapshot or freeze Artifact versions. Stable input produces a stable digest.

Checks, diffs, Step logs, usage and workspace snapshots remain explicit `Gaps`. No missing
recorder, public Evidence endpoint/fixture or CLI behavior is invented. Full attributable
Evidence-chain integration and the parent's audit walkthrough remain outstanding; see the
[original ART-002 record](2026-10-28-linux-art-002.md).

PostgreSQL runtime, real Providers, a human audit walkthrough, native macOS/Windows and
full product startup/Evidence integration were not validated in this slice. PostgreSQL and
other OSes are compile/vet checked by `make check`; this is not runtime evidence or hosted CI.
No new human architecture decision is required for these existing-port identity checks.
