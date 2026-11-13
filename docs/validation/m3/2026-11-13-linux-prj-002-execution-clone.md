# PRJ-002 immutable Execution clone provenance — Linux

Date: 2026-11-13. Scope: the bounded #191 slice following merged PR #350 (`d8a946631d1321633c5f30b21848cd248fa23f64`), against `feat/18-wp-m3-06-public-api-events`. Candidate: the commit containing this record. Local results are not hosted CI.

## Change and architecture review

The trusted Workflow source returns the definition and its captured Project clone together. RunWorkflow records the root beside the immutable revision/hash/snapshot in the existing atomic Execution/Step/command transaction. Project authorization is checked before and after source IO and before Submit. Returned clone identity must match the selected Project. Requests cannot choose a replacement root.

**Architecture review: sound within accepted ADR-0066 rule 5.** Platform keeps authoritative operational provenance; pure domain values gain no OS imports. Existing Git checks and native path handling stay in the attach adapter. Existing fake/PostgreSQL stores are reused, with no dependency or infrastructure change. The clone root is internal provenance, never sent as a remote Worker instruction or added to the public schema. A second active-clone lookup would be simpler to wire but could record a different root after concurrent reattach; returning it with the definition avoids that race.

Clone selection is server state, excluded from the command intent fingerprint. Identical authorized retries with available content return the original Execution/root; new keys capture the current clone. Different requested content, revision, attribution or variables keep existing conflict semantics. Execution creation and command result are atomic, so a lost response retains the same committed outcome. Normal commits preserve provenance; PostgreSQL also refuses direct root updates. Recovery uses durable definitions/Step inputs without any active-clone lookup. Missing original source does not cause fallback. No new human architecture decision is needed.

The initial definition schema and checksum are updated directly under the owner-approved no-legacy rule until Amaru v1.0. No upgrade, downgrade, backfill or old-format reader is added, and no existing development database is reset.

## Validation

- Merged PR #350 integration: `make check` PASS, exit 0 (`/tmp/amaru-350-merged-check.log`).
- Candidate: `make check` PASS, exit 0 (`/tmp/amaru-351-check.log`).
- `go test -race -count=1 ./internal/attach ./internal/adapter/localauth ./internal/coordinator ./internal/store/migrations` PASS, exit 0 (`/tmp/amaru-351-race.log`).
- Full Platform `AMARU_TEST_PG=… go test -race -count=1 -tags pg ./...` PASS, exit 0 (`/tmp/amaru-351-pg.log`); each test uses a fresh disposable database.
- Final `go test -race -count=10 ./internal/attach ./internal/adapter/localauth` PASS, exit 0 (`/tmp/amaru-351-attach-race.log`).
- With `AMARU_TEST_PG` pointing to disposable PostgreSQL on loopback, final `go test -race -count=10 -tags pg ./internal/store/postgres -run 'TestDefinitionWithPostgres|TestProjectCloneGitAndReopen'` PASS, exit 0 (`/tmp/amaru-351-pg-final.log`).
- `git diff --check` PASS. Documentation links are included in the required repository check and checked again after finalizing this record.
- `trail check --base origin/feat/18-wp-m3-06-public-api-events` exits 1 with eight reviewed lexical conflicts: the changed source/docs still reauthorize after IO, storage introduces no native path imports, and the initial schema has no separate legacy migration. These matches do not reverse those learnings; the commit/PR records the rationale. The new learning records why clone selection is excluded from caller intent.

Initial focused tests exposed a test-adapter field/method name collision and a reused deterministic test Execution ID after store reconstruction; both fixtures were corrected and the final suites above passed. No production failure was hidden by disabling a check.

## Evidence and limits

- Shared fake/PostgreSQL definition suite: root persistence across store reconstruction, immutable Commit behavior, original-root replay after selection changes, new-key selection, invalid provenance refusal.
- Real temporary Git + fake stores: one captured binding supplies both definition and Execution provenance even when reattach occurs during lookup; replay after reattach preserves the earlier root and a new key captures the latest root.
- Real temporary Git + PostgreSQL: attach two clones, read a clone-only revision after pool reopen, submit, reattach the clone lacking that revision, delete the source clone, close/reopen the pool and reconstruct the coordinator. The persisted original root, revision/hash, Step records and durable definition remain available. A new read through the active clone fails closed.
- Existing real compact Auth lifecycle tests continue to withhold content after logout, expiry, cancellation and shutdown; they use callback sources. This is separate from the Git tests' controlled authentication callback.

Pool/store/coordinator reconstruction is integration evidence, not a process-kill or PostgreSQL server restart. No full product HTTP/SDK run or Provider filesystem recovery is claimed. Current Workers use already durable Step inputs; inputs requiring an unavailable original filesystem are outside this slice and must never use active-clone fallback. Public replay still loads requested content before replay and fails closed if unavailable. Filesystem replacement race immunity and native macOS/Windows runtime remain unvalidated. Linux is required for M3; other OSes are best effort. #191, #18 and M3 remain open.
