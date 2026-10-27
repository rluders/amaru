# Provenance

Fresh import with provenance, as decided in [ADR-0047](../../docs/architecture/adr/0047-migrate-the-prototype-protocol-into-packages-contracts.md) (accepted 2026-10-26). `packages/contracts` is canonical from its first commit; the source is not kept in sync afterwards.

**Source:** the `protocol/` directory of the earlier internal implementation's Runtime repository, at commit **`c8a2f6e`** ([ADR-0032](../../docs/architecture/adr/0032-migration-baseline-is-a-pinned-commit.md)). The source repository is not named in this repository's public documents (owner decision, 2026-10-26). No later commit was used.

## Files transferred

| Source path (at `c8a2f6e`) | Blob id | Disposition | Target |
|---|---|---|---|
| `protocol/schema.json` (146 `$defs`) | `c2071368d071d8b6187334984f873af5b75ee401` | Adapted: 98 definitions kept, split one file per type, renamed; 48 dropped | `schemas/*.json` |
| `protocol/events.schema.json` (22 `$defs`) | `485adbc03d53fd01c7c249ac640017932efee60e` | Adapted: 15 kept, 3 dropped, base replaced | `schemas/events/*.json` |
| `protocol/provider.schema.json` (6 `$defs`) | `60ac9313143e2f17fdb8d841b8a5e389e222da33` | Adapted | `schemas/provider/*.json` |
| `protocol/openapi.yaml` (56 paths) | `1eafb129ffaf2bb20a5ac6cf5ee3bea3fba27830` | Adapted: 25 paths kept, 4 added, rest dropped | `openapi/openapi.yaml` |
| `protocol/fixtures/**` (271 files) | tree of `protocol/fixtures` at the same commit | Re-validated against the adapted schemas in API-002 ([#117](https://github.com/rluders/amaru/issues/117)): 208 imported with the recorded transforms below, 63 dropped; see "Fixtures" | `fixtures/` |

The conversions (schemas, then fixtures) were one-time scripted transformations followed by hand edits; the scripts are not part of this repository. Everything under `schemas/`, `openapi/` and `proto/` is hand-maintained from here on.

## Renames applied (glossary term map, ADR-0047 §5)

- `weave.*` namespaces → `amaru.*` (`amaru.provider/v1`, `amaru.worker.v1`); `$id` base is `https://amaru.invalid/contracts/v1/schemas/` (reserved `.invalid` TLD, no real host).
- Autoloop → Delivery loop (`AutoloopLedger` → `DeliveryLoopLedger`, field `autoloop` → `deliveryLoop`); Workspace → `Checkout` (a Platform-managed Git worktree); `workspaceWrite`/`workspaceCommands` → `worktreeWrite`/`worktreeCommands`; `weaveConfig` → `config`; `runtimeVersion` → `platformVersion`; header `X-Weave-Protocol` → `X-Amaru-Protocol`; provider secret env prefix → `AMARU_SECRET_`.
- References to the source's own ADR numbers and dogfood findings were removed from descriptions, because they would collide with Amaru's ADR numbering.

## Intentional behavior changes (and why)

| Change | Reason |
|---|---|
| `GET /work-packages/{id}/autoloop` (registered, never in `openapi.yaml`) → `GET /v1/projects/{projectId}/work-packages/{id}/delivery-loop`, declared | Fixes the recorded drift. Name from the glossary (Autoloop → Delivery loop); kebab-case path segment as the other multiword segments (`work-packages`, `work-items`). |
| `DeliveryLoopPolicy.merge` removed; `merging` phase removed from the ledger phases | Baseline rule 2: the Platform never merges in the MVP. The human merges (`awaiting_merge`). |
| `ExecutionState` gains `waiting`; `StepState` becomes the DOMAIN_MODEL §5.2 set (`unsupported` → `refused`) | Accepted domain model, ADR-0042, ADR-0044. |
| `Usage` (`credits`, `quantity`, `unit`, `durationMs`) → `UsageRecord`/`UsageReport` | ADR-0033: tokens, cost with currency, per-number source; unknown is never zero. |
| Events: every event gains `id`, `resource`, optional `correlationId`; `seq` is the per-Project cursor; `ResyncRequired` frame replaces closing the socket with code 4410; `EventList` is an object with `nextCursor` | WRK-003. |
| `GET /approvals` (pending steps) and `POST …/steps/{stepId}/approval` → `Approval` resource with Evidence and a decision route | ADR-0036. `ApprovalState` on `StepExecution` is kept as the prototype view and is not yet tied to an `Approval` id. |
| `ErrorCode` gains `forbidden` | A refused role, Team or separation-of-duties decision needs a typed error (POLICY_AND_APPROVAL_SHAPES §3.4). |
| Wire major is `/v1` and the first Amaru major; no `/v0` | ADR-0025, ADR-0047. |

## Not carried over (dropped, with reason)

| Dropped | Reason |
|---|---|
| MCP servers, connector validation, agent/MCP grants, per-project MCP trust (`/mcp/**`, `/connectors/**`, `/agent-grants/**`, `/agents/{id}/mcp-grant`) | Prototype configuration model; Amaru routes tool access through Capabilities and Policy (ADR-0037). Not in M2. |
| Outbound hooks, hook egress hosts, hook consent (`/hook-egress/**`, `/projects/{id}/hooks/**`) | Webhook Triggers are FEAT-WFL-009 (M7) and go through Policy; the prototype shapes do not match. |
| Playbooks, Skills and their trust, Workflow templates (`/playbooks/**`, `/skills/**`, `/templates/**`) | Skills and templates are later features; their references were pruned from Agent. Add them additively when scheduled. |
| Intake (`IntakePolicy`, `IntakeReport`, `Eligibility`, `…/intake/sync`) | Tracker integration (INT-001) is not in M2. `IssueRef` stays as the Work Item's external link. |
| Provider enable and secret routes (`PATCH /providers/{id}`, `PUT …/secrets/{name}`) | Secret handling is a separate security design; read-only `GET /providers` stays. |
| Placeholder-image generator (`GenerateSpec`), `ResolvedAgent` snapshot MCP fields, plugin/Obsidian-specific discovery | Prototype-only. |

## Fixtures (API-002)

All 271 prototype fixtures were run against the adapted schemas. A fixture was imported only if its outcome (valid or invalid) held after the transforms; none was edited to force an outcome. Imported files carry their prototype path in `source` in `fixtures/manifest.json`.

| Result | Count | Why |
|---|---|---|
| Imported | 208 | The outcome held after the transforms below. |
| Dropped: the type was removed | 49 | Fixtures of `McpServer*`, `McpImport*`, `McpToolList`, `SetMcpAllowedTools`, `ConnectorValidation`/`ValidateConnector`, `Playbook*`, `TrustPlaybook`, `Skill*`, `WorkflowTemplate`/`InstantiateWorkflowTemplate`/`InstantiateReport`, `SetProviderSecret`, `UpdateProvider`, `PendingApproval*`, `ApprovalDecision` (see "Not carried over"; the last three were replaced by `Approval`). |
| Dropped: the shape was removed | 14 | Agent `skills` and `playbooks` pins, Artifact `generate`, Work Item `eligibility`, Work Package `intake`, asset, hook and usage-unit cases, a Step with agent skills. Fixtures with an optional removed field (`skills`, `playbooks`) were imported with that field deleted. Invalid fixtures that would now fail only because of a removed field were dropped, so every imported invalid fixture fails for the reason its name gives. |

Transforms applied before validating (the same rename rules as the schemas): `Autoloop` → `DeliveryLoop` and `autoloop` → `deliveryLoop` (including `autoloop.decision` → `delivery_loop.decision`); `weaveConfig` → `config`; `runtimeVersion` → `platformVersion`; `workspace` → `checkout`; step state `unsupported` → `refused`; event type `runtime.state` → `platform.state`; every event gained `id` (a ULID derived from `seq`) and `resource`; a bare event array became `{events: [...]}`; the prototype `usage` objects (`credits`, `durationMs`) were deleted because the shape is gone (new usage fixtures replace them); three values naming the prototype (a branch, a repository name, a settings sentence) were replaced by neutral ones. 107 fixtures are new (Approval, Policy, Principal, Grant, usage, WRK-003 events, Step timeout and interruption, Worker envelope).

## Added by Amaru (not migrated)

Approval, Evidence, Policy request/decision, Grant and Principal shapes ([POLICY_AND_APPROVAL_SHAPES](../../docs/architecture/POLICY_AND_APPROVAL_SHAPES.md)); the Worker protocol `proto/amaru/worker/v1/worker.proto` ([WORKER_PROTOCOL](../../docs/architecture/WORKER_PROTOCOL.md), written in M1, not migrated); usage records; approval, policy, grant, usage and stale-result events; `routeguard`; `components.schemas.StreamFrame` in the OpenAPI document (the WebSocket frames, so generated clients take the frame types from the contract; additive, no route changes).
