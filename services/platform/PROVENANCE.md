# Provenance

Fresh import with provenance ([ADR-0049](../../docs/architecture/adr/0049-adapt-the-prototype-domain-and-ports-into-services-platform.md) item 3). Source: the Runtime repository of the earlier internal implementation (named in the private migration notes), commit `c8a2f6e983beba59edc446f36df2f442edaa9855`. Blob ids are git object ids at that commit.

| Source path | Blob id | Target | Disposition |
|---|---|---|---|
| `runtime/internal/domain/execution.go` | `ecfba2094004460f908a34c611cd5a330202cf04` | `internal/domain/execution.go` | Adapted: `waiting` state; transition table from DOMAIN_MODEL §5.1; per-source-state methods; prototype fields (capability, provider, workspace, outputs, steps) not ported |
| `runtime/internal/domain/step.go` | `0d614c92a97e7587fca3d7e04768388491675e0a` | `internal/domain/step.go` | Adapted: states `pending`, `refused`, `awaiting_approval`, `leased`, `interrupted`; `unsupported` becomes `refused`; ADR-0042/0044 restart rule (`Recover`) |
| `runtime/internal/domain/errors.go` | `b77ce403de3a3dda8cda1fe909e920cf47b32c9c` | `internal/domain/errors.go` | Only the `ExecError` idea; the protocol error codes are not ported |
| `runtime/internal/domain/execution_test.go` | `b2ce9443dd06536da5b718f040b88c2e4cea0241` | `internal/domain/execution_test.go` | Rewritten: every state pair checked against an independent restatement of §5.1 |
| `runtime/internal/domain/step_test.go` | `4b47c2367b47891e255854b0f436bb1586724552` | `internal/domain/step_test.go` | Rewritten likewise for §5.2 |
| `runtime/internal/arch/arch_test.go` | `c7431bfd25d1593e8747a80de3bfc0e5dbe22a84` | `internal/arch/arch_test.go` | Adapted: Amaru module path and layout; also forbids `os`, `net*` and any third-party import in the core |
| `runtime/internal/domain/capabilities.go` | `ee1225963b3d6c3b78c38eed1157f24e95f28649` | `internal/domain/capability.go`, `catalog.go`, `catalog.json` | Adapted: port/spec types and validation aligned with `packages/contracts` (`CapabilitySpec`, `PortSpec`); catalog moved from a Go slice to embedded data; a `Catalog` value that accepts further specs. Built-in entries limited to the engineering subset (see below) |
| `runtime/internal/domain/graph_test.go` (`TestCatalogIsValid`) | `eef1f8afb0f34b12450450249e7145390dedcba0` | `internal/domain/capability_test.go` | Rewritten and extended; other tests of this file belong to Workflows |

## Intentional behavior changes
- Execution `queued` can now only be cancelled, not finished `succeeded`/`failed` directly (prototype `Finish` allowed any terminal state from `queued`); `waiting` can only fail or be cancelled. Source: DOMAIN_MODEL §5.1.
- A Step can no longer jump `queued → succeeded`/`skipped`; skipping is `pending → skipped` and needs a reason.
- A pending approval no longer keeps a Step `running`; it is `awaiting_approval`, and the parent Execution is `waiting`.

- Built-in Capabilities: of the prototype's 30 entries only `code.test`, `mcp.call`, `asset.upload`, `doc.convert`, `image.convert`, `media.transcode` are carried (product owner choice, 2026-10-26). The 3D, motion, audio-generation, scene and engine entries (`mesh.*`, `character.rig`, `motion.*`, `image.generate`, `audio.*`, `scene.*`, `engine.*`) are not migrated; domain-specific Capabilities come from plugins (M6). Prototype ADR numbers in descriptions were removed.
- `Capability` validity follows `packages/contracts` (dotted identifier with at least two segments); the prototype pattern differed.

## Not ported here (reason)
- `SettleSteps`, `Progress`, `Snapshot`, `PendingApproval`: coordinator and read-model concerns; revisit in WP-M3-03 (#15). The prototype `ApprovalState` is replaced by the Approval work.
- `CapabilityOffer`, `CapabilityInfo` (provider offers, availability): Provider work (FEAT-CAP-002/005).
- `MediaTypeFor` and the media-type constants: output hashing and file handling, not catalog.
- `StepExecution` content fields (capability, files, usage, agent snapshot, verdict, ...): arrive with FEAT-AGT-004 (#114), FEAT-CAP-001 (#111) and the Approval work.
