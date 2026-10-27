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
| `runtime/internal/domain/agent.go` | `41f42674f218db18ed2c1a70afb8633704b9e7d3` | `internal/domain/agent.go`, `agent_history.go` | Adapted: fields and limits aligned with the `Agent`/`UpdateAgent` contracts; `AgentPatch`; new `AgentHistory` (content hash, revisions, lost-update guard, delete appends a tombstone, recreate continues the history). Snapshot types (`ResolvedAgent`, `Resolution`, `ToolIsolation`, `MCPUse`) left for #114 and the tool-scope feature |
| `runtime/internal/domain/agent_test.go` | `333d9675745a3c276630012e6d4e8acbaec92439` | `internal/domain/agent_test.go` | Validation and slug tests carried; MCP and workflow-step tests not ported; versioning tests are new |
| `runtime/internal/domain/skill.go` (`SkillUse`, `HashBody`) and `agent.go` (`ResolvedAgent`, `Resolved`) | `41f42674f218db18ed2c1a70afb8633704b9e7d3` (agent.go), `96d04d6d030fa9cc8a698acfc5d7e7b3a2f23eef` (skill.go) | `internal/domain/agent_snapshot.go` | Adapted: only the snapshot types. `Resolved` becomes `AgentVersion.Resolve`, which takes revision and hash from the recorded version and validates Provider, resolution, isolation and Skills. `Skill`, `SkillIssue`, `ValidateSkillFields`, `ParseSkillRef` wait for FEAT-SKL-001 |
| `runtime/internal/domain/skill_test.go` | `78e18ba4f31e9017ad6ae57884eb649138969702` | `internal/domain/agent_snapshot_test.go` | New tests for resolution, snapshot immutability, bind-once and fixture agreement; `TestParseSkillRef` not ported with `ParseSkillRef` |

## Intentional behavior changes
- Execution `queued` can now only be cancelled, not finished `succeeded`/`failed` directly (prototype `Finish` allowed any terminal state from `queued`); `waiting` can only fail or be cancelled. Source: DOMAIN_MODEL §5.1.
- A Step can no longer jump `queued → succeeded`/`skipped`; skipping is `pending → skipped` and needs a reason.
- A pending approval no longer keeps a Step `running`; it is `awaiting_approval`, and the parent Execution is `waiting`.

- Built-in Capabilities: of the prototype's 30 entries only `code.test`, `mcp.call`, `asset.upload`, `doc.convert`, `image.convert`, `media.transcode` are carried (product owner choice, 2026-10-26). The 3D, motion, audio-generation, scene and engine entries (`mesh.*`, `character.rig`, `motion.*`, `image.generate`, `audio.*`, `scene.*`, `engine.*`) are not migrated; domain-specific Capabilities come from plugins (M6). Prototype ADR numbers in descriptions were removed.
- `Capability` validity follows `packages/contracts` (dotted identifier with at least two segments); the prototype pattern differed.
- Agent: `workspaceWrite`/`workspaceCommands` are `worktreeWrite`/`worktreeCommands` (contract names; glossary Worktree). Limits follow the contract (`models` ≤16, `worktreeCommands` ≤32). The prototype's file-per-Agent store and last-write-wins save are replaced by revisions: an edit made against a stale revision is `ErrConflict` (FEAT-AGT-001 "F-7 lost-update fix").
- `ResolvedAgent`: `workspaceWrite`/`workspaceCommands` become `worktreeWrite`/`worktreeCommands`; `revision` and `hash` are new and required, `playbooks` and `mcp` are not carried (no contract field; tool scope is AGT-003). `skills` is carried as `SkillUse`.
- Agent `skills`, `playbooks` and `mcp` fields are not carried: the Agent contract has none of them. Skills arrive with FEAT-SKL-001, tool scope with FEAT-AGT-003; Playbook references wait for the owner's decision on `playbook.go` (ADR-0049 item 4).

## Not ported here (reason)
- `SettleSteps`, `Progress`, `Snapshot`, `PendingApproval`: coordinator and read-model concerns; revisit in WP-M3-03 (#15). The prototype `ApprovalState` is replaced by the Approval work.
- `CapabilityOffer`, `CapabilityInfo` (provider offers, availability): Provider work (FEAT-CAP-002/005).
- `MediaTypeFor` and the media-type constants: output hashing and file handling, not catalog.
- `StepExecution` content fields (capability, files, usage, agent snapshot, verdict, ...): arrive with FEAT-AGT-004 (#114), FEAT-CAP-001 (#111) and the Approval work.
