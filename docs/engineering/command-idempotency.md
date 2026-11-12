# Command idempotency in the M3 coordinator

**Implemented in the coordinator; public HTTP handlers are not implemented.** [ADR-0058](../architecture/adr/0058-command-idempotency-conflicts-and-retention.md) defines conflicting reuse and retention. [EXE-005 #157](https://github.com/rluders/amaru/issues/157) applies it to the following complete inventory of caller-facing mutating coordinator commands.

| Command | Key input | Normalized request identity |
|---|---|---|
| `Submit` (`execution.submit`) | `SubmitRequest.Key` | Project and Environment IDs; principal including sponsor and role; trigger (empty becomes `manual`); planned Step specs in order, including inputs, timeout and Skills; pinned Workflow snapshot and revision; variable overrides. Map keys use Go JSON's sorted encoding; Step inputs remain exact bytes. |
| `Cancel` (`execution.cancel`) | `CancelRequest.Key` | Execution ID, exact reason, Actor kind and Policy authority, authenticated human `ActorID`. |
| `DecideApproval` (`approval.decide`) | `DecideRequest.Key` | Approval ID, principal including role, outcome, exact rationale and Evidence digest. |
| `CreateGrant` (`grant.create`) | required `key` argument | Principal and Grant content after `NewGrant` normalization: ID, level, Project, subject, ordered Capabilities, resource, definition hash and expiry. Server-owned version, creation time, status and revocation fields are reset; creation time is excluded. |
| `RevokeGrant` (`grant.revoke`) | required `key` argument | Principal, Grant ID and exact reason. |
| `TrustContent` (`trust.content`) | required `key` argument | Principal, Project key, kind, ref and exact content hash. |
| `RevokeTrust` (`trust.revoke`) | required `key` argument | Principal, Project key, kind, ref and exact reason. |

Keys contain 1–128 Unicode characters. Their namespace is `(command name, key)` within the authoritative store, including across Projects; actor and target are identity fields, not namespaces. Equivalent arbitrary JSON encodings, reordered lists, whitespace changes and role changes are not promised equivalent. Clients use a fresh key for a new request.

The original successful outcome is recorded atomically with the mutation and its Events/audit events. Concurrent duplicates have one committed outcome. A matching replay returns that outcome, including its original timestamps and versions, after later state changes or restart. Conflicting payload, target or actor returns `domain.ErrKeyReused` without the stored result or another mutation. Invalid attribution and insufficient human authority are refused before replay. Current caller identity/role must come from the trusted caller boundary; these internal methods do not authenticate credentials or implement Organization isolation. Multi-Organization HTTP exposure remains gated by ADR-0022.

A failure before commit leaves no successful command; a failure after commit but before acknowledgement is retried with the same key. Command records have **no automatic expiry in M3**, including after terminal Execution state or Event expiry. This does not implement production erasure or retention cleanup; ADR-0058 requires a compatible policy first.

Earlier internal human cancellation records lack actor identity. Their keys remain reserved and conflict with identified requests; they are not silently replayed to a different person or re-executed. Policy cancellations with no `ActorID` preserve their existing fingerprint. No existing record is deleted.

A repeated cancellation key replays its original outcome and sends no new Worker message. A new cancellation request for a terminal Execution returns `domain.ErrInvalidTransition` (public `conflict`). A committed cancellation whose notification is lost is settled through the existing durable cancellation/reconciliation path; this is not an exactly-once external side-effect guarantee.

Store helpers, Policy's automatic expiry/lapse bookkeeping, reconciliation and Worker protocol messages are internal operations, not additional caller commands. Worker attempt IDs, fencing tokens and stable Step side-effect keys remain separate. No new Provider, HTTP endpoint, permission, dependency or infrastructure is introduced here.

## Contract and validation boundary

The proposed v1 HTTP bodies `CreateExecution`, `RunWorkflow`, `CancelExecution` and `DecideApproval` require `idempotencyKey`. Cancellation's optional reason is in the body; actor identity is derived from authentication, never a body field. The generated SDK includes these inputs. Requiring formerly optional keys and a cancellation body tightens the unreleased v1 draft; this is permitted before its first fixture freeze by [contract versioning](../architecture/CONTRACT_VERSIONING.md#1-fixture-conventions-for-additive-changes), not a compatibility rule for a released major. Grant/trust mutations have no public endpoints yet; their future contract must carry keys before exposure. The HTTP adapter and its response mapping remain API-005 work; coordinator outcomes are internal domain snapshots, not HTTP response implementations.

`coordtest.IdempotencySuite` runs all seven commands through sequential and 16-way concurrent delivery, adapter/coordinator recreation, a one-year clock advance, conflicting payload/actor reuse, denied replay, missing keys, precommit failure and lost acknowledgement after a durable commit. It checks original results, command records, single audit effects and cancellation Event counts. Later Grant/trust changes cannot replace an old outcome. `CommandAtomicitySuite` checks command-record rollback for duplicate Grant IDs, stale Grant versions and stale trust revocation.

- `make check`: fake-backed integration and unit tests, shared Go/TypeScript contract fixtures, generated SDK checks and fake-fetch SDK tests.
- `make platform-test-pg`: the same command suites against PostgreSQL, alongside the existing recovery and process-kill suites. PostgreSQL is separately required; it is not run by `make check`.

The [Linux EXE-005 record](../validation/m3/2026-11-12-linux-exe-005.md) identifies the implementation commit, commands and observed results.

These checks do not establish a serving HTTP API, multi-Organization authorization, real Provider side effects, PostgreSQL/power-loss recovery, or native macOS/Windows behavior.
