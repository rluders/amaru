---
title: Command idempotency rejects conflicting reuse and has no automatic expiry in M3
status: accepted
date: 2026-11-12
deciders: Ricardo Lüders
---

# 0058. Command idempotency rejects conflicting reuse and has no automatic expiry in M3

## Context

[EXE-005 #157](https://github.com/rluders/amaru/issues/157) requires an explicit policy for reuse of a key with a different payload and for deduplication retention before exposing the affected public commands. This decision supplies those rules; it does not claim that every mutating command is implemented or verified.

Verified by source inspection at integration commit `acb23c7`:

- The coordinator's Submit, Cancel and DecideApproval commands accept keys, fingerprint requests and record original outcomes. [CommandRecord](../../../services/platform/internal/domain/command.go) distinguishes duplicate delivery from conflicting reuse; [replay](../../../services/platform/internal/coordinator/command.go) returns `ErrKeyReused` on a fingerprint mismatch.
- The [commands migration](../../../services/platform/internal/store/migrations/sql/20261027150000_commands.sql) makes `(name, key)` unique and stores the fingerprint and result. It defines no expiry. [ADR-0052](0052-adapt-the-prototype-execution-manager-into-the-execution-coordinator.md) requires durable command keys rather than a process map.
- [OpenAPI](../../../packages/contracts/openapi/openapi.yaml) describes execution submission keys, but cancellation and Approval decision do not yet expose equivalent key inputs. Its cancellation description also differs from the coordinator's terminal-state behavior. These are implementation follow-ups, not changes implemented by this ADR.
- Grant and trust mutations in the coordinator currently lack command keys. A check of Submit and Cancel alone cannot establish the issue's "every mutating command" criterion.

An arbitrary expiry would permit a sufficiently late retry to repeat a consequential mutation. Retaining complete outcomes forever also has storage and data-retention costs. M3 needs a clear temporary rule without inventing a cleanup service or changing the accepted data-retention requirements.

## Decision

**Accepted by the product owner on 2026-11-12; implementation remains pending in #157.**

1. A key identifies one command request within its authoritative Platform store and command name. In M3, preserve the existing `(command name, key)` namespace; Project, target and actor are request identity, not separate key namespaces. Clients must generate fresh keys for new requests. Multi-Organization public exposure must first enforce Organization isolation under [ADR-0022](0022-cloud-tenancy-row-level-isolation-by-organization.md); this decision does not authorize a shared cross-Organization replay namespace.
2. The same key and the same command-defined normalized request return the original durably recorded outcome, even after state changes or restart. A changed target, actor or consequential payload is conflicting reuse: return a conflict, perform no mutation and reveal no original result to an unauthorized caller. Authentication and authorization apply to replay as well as first delivery. Request identity includes all fields that affect the command; it excludes transport metadata. Each exposed command must document its normalization rather than promise equivalence for arbitrary JSON encodings.
3. Persist the outcome and the domain mutation atomically. Concurrent duplicates have one committed outcome. A failure before commit leaves no successful record; a lost acknowledgement after commit replays the original outcome. Worker attempt IDs and Step side-effect keys remain separate from command keys. This does not promise exactly-once external side effects: durable recovery and Provider guards still apply.
4. For M3, command deduplication records have **no automatic expiry**. Restart, terminal Execution state, and Event replay-window expiry do not release keys. No command-record cleanup is introduced by #157. Existing data-class retention and security requirements remain in force; if a required purge reaches command results, define a compatible deletion/tombstone policy before enabling that purge. This temporary rule must be revisited before a production retention/deletion feature or multi-Organization API is exposed.
5. Document keys in the affected versioned public contract before exposing commands. Choose their wire representation in the contract change, with fixtures and SDK parity; this ADR introduces no header, endpoint or CLI flag. Inventory every caller-facing mutating coordinator command, including Grant and trust changes, and either provide durable idempotency or explicitly resolve its scope with the owner. Internal reconciliation and Worker protocol messages retain their existing state/version/attempt semantics.

## Consequences

The decision preserves the existing coordinator conflict behavior and avoids silent re-execution after a TTL. Storage grows with command volume; recorded results may contain retained domain data. M3 does not establish a production garbage-collection or erasure design. An approved future policy must preserve the documented retry guarantee or explicitly version and bound it.

#157 must inventory public mutations, reconcile contract/coordinator differences, add missing keys and transactional records, and verify sequential/concurrent duplicates, restart, conflicting payloads and failure between commit and acknowledgement. This ADR is design evidence only; its source inspection is not a test result.

Boundary review: the Platform continues to own state, Policy, Approvals and Events; Clients supply keys through versioned contracts and Workers do not own command history. No dependency, infrastructure, migration from Weave, or new permission is introduced. Local and server deployments use the same store semantics. Verdict: **approved by the product owner**; implementation verification remains required.

## Alternatives considered

- **Return the original result for any reused key:** hides changed user intent and can disclose an unrelated outcome; reject conflicting requests instead.
- **Overwrite the old request or re-execute after a fixed TTL:** can repeat consequential mutations after a delayed retry. A bounded guarantee may be appropriate later, but needs explicit API and deletion rules first.
- **Expire command records with Event replay windows or Execution completion:** those lifetimes describe different guarantees and would invalidate delayed command retries.
- **Keep deduplication only in memory:** loses its guarantee on restart and contradicts ADR-0052.

## Approval

Proposed by Codex and accepted by Ricardo Lüders on 2026-11-12 in the repository task conversation for #157 / [PR #318](https://github.com/rluders/amaru/pull/318). Explicit owner approval: "Proposal approved. You can update and get it merged." This approves the policy; it does not establish implementation or validation of #157.
