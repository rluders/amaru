# services/platform

Go module `github.com/rluders/amaru/services/platform`: the Platform core. Introduced by WP-M3-01 ([#13](https://github.com/rluders/amaru/issues/13)) under [ADR-0049](../../docs/architecture/adr/0049-adapt-the-prototype-domain-and-ports-into-services-platform.md), which moves selected code from the earlier internal implementation at baseline commit `c8a2f6e` ([ADR-0032](../../docs/architecture/adr/0032-migration-baseline-is-a-pinned-commit.md)). Provenance per file: [PROVENANCE.md](PROVENANCE.md).

## What exists (unit-tested, Linux only so far)
| Path | Content |
|---|---|
| `internal/domain` | Execution and Step Execution state machines (FEAT-EXE-001, [#118](https://github.com/rluders/amaru/issues/118)), per [DOMAIN_MODEL §5.1–5.2](../../docs/architecture/DOMAIN_MODEL.md#5-lifecycles), with the restart rule of [ADR-0042](../../docs/architecture/adr/0042-durable-execution-with-explicit-restart-semantics.md)/[ADR-0044](../../docs/architecture/adr/0044-restart-semantics-clarifications-from-the-m1-spike.md). Standard library only. |
| `internal/domain` (Capabilities) | Typed Capability catalog (FEAT-CAP-001, [#111](https://github.com/rluders/amaru/issues/111)): `CapabilitySpec`/`PortSpec` matching `packages/contracts`, validation, a `Catalog` that accepts further specs, and the built-in specs as data (`catalog.json`, six engineering entries). No Provider exists yet. |
| `internal/domain` (Agents) | Agent definitions (FEAT-AGT-001, [#110](https://github.com/rluders/amaru/issues/110)): `Agent` and `AgentPatch` matching `packages/contracts`, validation, and `AgentHistory`, an append-only record of content-hashed revisions with a stale-revision (lost-update) guard; deleting appends a tombstone revision and keeps every earlier one; a deleted id can be created again as a later revision of the same history. Pure domain: no storage, so nothing is persisted yet. |
| `internal/domain` (snapshots) | Agent and Skill snapshots per Step (FEAT-AGT-004, [#114](https://github.com/rluders/amaru/issues/114)): `ResolvedAgent` taken from an `AgentVersion` (so it carries that version's `revision` and `hash`) with the `SkillUse` records (name, version, content hash) injected into the Step; `StepExecution.BindAgent` sets it once while the Step is pending. The `ResolvedAgent` contract gained required `revision`/`hash` and `skills`. Skill notes, trust and pin enforcement (FEAT-SKL-001) and Provider resolution (FEAT-AGT-002) are not implemented; nothing is persisted yet. |
| `internal/arch` | Architecture test: the core imports no OS, process, network, vendor or adapter package (negative-tested by adding a forbidden import). |

**Wire shape:** the Agent contract carries `revision` and `hash` (the head's `AgentVersion`), and `UpdateAgent`/delete carry `expectedRevision`, which `AgentHistory` checks. Nothing serves the API yet.

Nothing else of the Platform exists yet: no storage, API, coordinator or Worker. Approval states, Work Items and Agent snapshots arrive with the other sub-issues of #13.

## Check
`make platform-test` (part of `make check`). Tests use no real agents, network or paid APIs.
