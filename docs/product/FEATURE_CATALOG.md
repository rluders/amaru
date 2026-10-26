# Feature Catalog

Status: **accepted planning baseline** (2026-10-25, owner-reviewed with the planning package). The catalog of Amaru product capabilities with stable IDs, priorities, planned milestones and acceptance criteria. **No feature is implemented in this repository.** The *Evidence* column points at predecessor code, which is evidence for the design, not Amaru behavior.

Related: [PRD](PRD.md) (requirements FR/NFR) · [non-functional requirements](NON_FUNCTIONAL_REQUIREMENTS.md) · [roadmap](../architecture/ROADMAP.md) (milestones M0–M10) · [work packages](../architecture/WORK_PACKAGES.md) · [repository assessment](../migration/REPOSITORY_ASSESSMENT.md).

## 1. How to read this catalog

**IDs**: `FEAT-<AREA>-NNN`, never renumbered or reused. A retired feature keeps its row with class *Out of scope*. The `FEAT-` prefix avoids collisions with Weave findings (`F-nn`) and PRD requirement IDs.

**Priority**
| Priority | Meaning |
|---|---|
| **P0** | Required for the Local MVP (milestones M1–M4). The MVP is not reached without it. |
| **P1** | Required for the first multi-environment, extensible, team-capable product: remote execution, plugins, self-hosted, Desktop (M5–M8). |
| **P2** | Cloud, ecosystem and valuable improvements (M9–M10 or backlog). Scheduled when a milestone needs it. |
| **P3** | Speculative. Not scheduled; kept to record the idea and its open questions. |

Priority is assigned by product need, not by whether a predecessor already has the feature.

**Class** (relation to existing work)
| Class | Meaning |
|---|---|
| **Reuse** | Existing behavior in a predecessor that should carry over with limited change. |
| **Improve** | Existing behavior that needs material improvement (a finding, a gap, a missing boundary). |
| **New** | A product capability that exists nowhere yet. |
| **Future** | A direction, not committed; acceptance criteria are written when it is scheduled. |
| **Out of scope** | Explicitly not built (see §5). |

**Availability**: `L` local/compact install · `SH` self-hosted server · `C` Cloud · `W` Web client · `D` Desktop client · `E` environment-specific (depends on what a Worker's machine offers).

**Evidence abbreviations**: `WO:` = `weave-obsidian` at `c8a2f6e` (paths under `runtime/internal/` unless stated) · `WP:` = `weave` (prototype) at `824af76` · `ADR-0nn` without a leading zero pair = Weave ADR; `ADR-00nn` = Amaru ADR.

**Depends on**: features (and ADRs) that must exist before this feature can be built and accepted (a *hard* dependency). A dependency marked **(soft)** is a sequencing and integration need only: the feature is built and accepted against a port or contract that already exists, with fakes in tests, and the depended-on feature must exist before the feature's real-validation run (recorded run), which is scheduled where both exist. Soft dependencies are listed with their rationale in [WORK_PACKAGES §Cross-package feature dependencies](../architecture/WORK_PACKAGES.md#cross-package-feature-dependencies). The dependency graph has no cycles.

**Detail**: every P0 and P1 feature has an entry in §3 with outcome, acceptance criteria, validation, extension involvement and risks. P2/P3 features carry only the summary row; their criteria are written when scheduled.

## 2. Catalog

### 2.1 Projects (PRJ)
| ID | Feature | Actor | P | Milestone | Class | Avail | Depends on | Evidence |
|---|---|---|---|---|---|---|---|---|
| FEAT-PRJ-001 | Attach a Project without moving its files | Engineer | P0 | M3 | Reuse | L SH C | — | WO: `project/project.go` (`ID`, `Load`), `POST /v1/projects` |
| FEAT-PRJ-002 | Many Projects per Instance | Engineer | P0 | M3 | Reuse | L SH C | PRJ-001 | WO: ADR-009 (multi-project), `app/projects.go` |
| FEAT-PRJ-003 | Project trust | Engineer | P0 | M3 | Reuse | L SH C | PRJ-001 | WO: `.weave/trust.json`, ADR-024 |
| FEAT-PRJ-004 | Linked repositories | Engineer | P0 | M4 | Reuse | L SH C | PRJ-003 | WO: ADR-026, `domain/project.go` `Repositories` |
| FEAT-PRJ-005 | Project metadata store | Platform | P0 | M3 | Improve | L SH C | ADR-0043 | WO: `.weave/` files; WP: Postgres `projects` |
| FEAT-PRJ-006 | Detach and archive a Project | Engineer | P1 | M7 | New | L SH C | PRJ-001 | WO: no detach (`app/projects.go`) |
| FEAT-PRJ-007 | Knowledge index (read-only) | Engineer, Agent | P2 | Backlog | Future | L SH | ADR-0030 | WO: `store/fsscan`, ADR-022 |

### 2.2 Work (WRK)
| ID | Feature | Actor | P | Milestone | Class | Avail | Depends on | Evidence |
|---|---|---|---|---|---|---|---|---|
| FEAT-WRK-001 | Work Packages and Work Items | Engineer | P0 | M3 | Reuse | L SH C | PRJ-001 (soft) | WO: `domain/work.go`, ADR-004 |
| FEAT-WRK-002 | Work Item dependencies | Engineer | P0 | M3 | Reuse | L SH C | WRK-001 | WO: `DependsOn`, ADR-021, F-17 |
| FEAT-WRK-003 | Change events for Work, Workflows, Agents | Client | P0 | M3 | Improve | L SH C | EXE-004 | WO: GAP-06; work files not watched (`cmd/weaved/main.go`) |
| FEAT-WRK-004 | Supervised delivery loop | Engineer | P0 | M4 | Improve | L SH C | WFL-001, ENV-010, VAL-001 (soft), REV-004 (soft), REV-005 (soft), INT-001 (soft) | WO: `app/autoloop*.go`, ADR-021/026/028/029, Cycle 7 |
| FEAT-WRK-005 | Attempt and budget accounting | Engineer | P0 | M4 | Improve | L SH C | WRK-004 | WO: F-39, F-40, F-32, F-25; `domain/autoloop.go` |
| FEAT-WRK-006 | Signals and triage inbox | Lead | P2 | Backlog | Future | W D | WRK-001 | WP: `domain/signal.go` |
| FEAT-WRK-007 | Work graph view | Engineer | P2 | Backlog | Future | W D | WRK-002 | WP: `features/work-graph` |

### 2.3 Workflows (WFL)
| ID | Feature | Actor | P | Milestone | Class | Avail | Depends on | Evidence |
|---|---|---|---|---|---|---|---|---|
| FEAT-WFL-001 | Workflow definitions as text graphs | Engineer | P0 | M3 | Reuse | L SH C | — | WO: `domain/workflow.go`, `.weave/workflows/*.yaml`, ADR-013 |
| FEAT-WFL-002 | Definition versioning and run pinning | Engineer, Auditor | P0 | M3 | Improve | L SH C | WFL-001 | WO: resume refuses changed steps (`ResumeInterrupted`) |
| FEAT-WFL-003 | Workflow variables | Engineer | P0 | M3 | Reuse | L SH C | WFL-001 | WO: `{{var}}`; F-15 truncation fixed |
| FEAT-WFL-004 | Per-Step timeouts | Platform | P0 | M3 | New | L SH C | EXE-001 | WO: no wall-clock timeout on agent runs (`app/manager.go`) |
| FEAT-WFL-005 | Step retries with backoff | Engineer | P1 | M5 | New | L SH C | EXE-002, ADR-0042 | WO: none per step; WP: none |
| FEAT-WFL-006 | Parallel branches | Engineer | P1 | M5 | New | L SH C | ENV-004 | WO: `sequence.Wait` is sequential |
| FEAT-WFL-007 | Workflow templates | Lead | P1 | M7 | Reuse | L SH C | WFL-001 | WO: ADR-018 |
| FEAT-WFL-008 | Visual workflow editor | Engineer | P1 | M7 | Improve | W D | CLI-001 | WO: `plugin/src/ui/flow` (React Flow); WP: `WorkflowEditorPage` |
| FEAT-WFL-009 | Inbound webhook trigger | Integrator | P1 | M7 | Reuse | SH C | POL-001 | WO: ADR-014, `app/hooks.go`, HMAC |
| FEAT-WFL-010 | Scheduled trigger | Engineer | P2 | Backlog | Future | SH C | POL-001 | — |
| FEAT-WFL-011 | Conditional branching and loops (no saga compensation; failure edges route to cleanup Steps) | Engineer | P2 | Backlog | Future | L SH C | WFL-006 | — |

### 2.4 Execution (EXE)
| ID | Feature | Actor | P | Milestone | Class | Avail | Depends on | Evidence |
|---|---|---|---|---|---|---|---|---|
| FEAT-EXE-001 | Execution and Step state machines | Platform | P0 | M3 | Reuse | L SH C | — | WO: `domain/execution.go`, `domain/step.go` |
| FEAT-EXE-002 | Durable state and restart recovery | Platform | P0 | M3 | Improve | L SH C | EXE-001, ADR-0042, ADR-0015 | WO: `Manager.Restore`, `ResumeInterrupted`, ADR-029 restart recoveries |
| FEAT-EXE-003 | Cancellation with process-tree termination | Engineer | P0 | M3 | Reuse | L SH C E | EXE-001 | WO: `platform/process_unix.go` (Setpgid), Windows variant unvalidated |
| FEAT-EXE-004 | Ordered event stream with replay | Client | P0 | M3 | Improve | L SH C | API-001 | WO: `app/eventbus.go` (1024 ring), WS close 4410 |
| FEAT-EXE-005 | Idempotent commands | Client | P0 | M3 | Reuse | L SH C | EXE-001 | WO: `CreateRequest.IdempotencyKey`, autoloop deterministic keys |
| FEAT-EXE-006 | Concurrency limits | Operator | P0 | M3 | Reuse | L SH C | EXE-001 | WO: `app/agentslots.go`, `--max-agents` |
| FEAT-EXE-007 | Scheduling queue with priorities and fairness | Lead | P1 | M5 | New | SH C | ENV-004 | — |
| FEAT-EXE-008 | Provider quota pause honoring reset time | Engineer | P1 | M5 | Improve | L SH C | EXE-002 | WO: `domain.ProviderPause` fixed 30 min, F-20, F-27 |
| FEAT-EXE-009 | Long-running agent sessions | Engineer | P3 | — | Future | SH C | ENV-004 | — |

### 2.5 API and contracts (API)
| ID | Feature | Actor | P | Milestone | Class | Avail | Depends on | Evidence |
|---|---|---|---|---|---|---|---|---|
| FEAT-API-001 | Versioned public API contract | Client, Plugin author | P0 | M2 | Improve | L SH C | ADR-0025 | WO: `protocol/openapi.yaml`, `schema.json`, 271 fixtures; drift on `/work-packages/{id}/autoloop` |
| FEAT-API-002 | Contract fixtures validated in Go and TypeScript | Engineer | P0 | M2 | Reuse | — | API-001 | WO: `internal/protocol/fixtures_test.go`, `plugin/test/protocol.test.ts` |
| FEAT-API-003 | Generated TypeScript SDK | Client, Plugin author | P0 | M2 | New | W D | API-001 | WO: hand-typed `plugin/src/runtime/types.ts`; WP: raw fetch hooks |
| FEAT-API-004 | Worker protocol | Worker | P1 | M5 | New | SH C E | ADR-0029, API-001 | WO: `Resumable`/`Detachable`, `weave.provider/v1` as precedent |
| FEAT-API-005 | Compatibility negotiation between clients and Instances | Client | P0 | M3 | Reuse | L SH C | API-001 | WO: `protocolVersion` in discovery, `/v1/health` |

### 2.6 Agents (AGT)
| ID | Feature | Actor | P | Milestone | Class | Avail | Depends on | Evidence |
|---|---|---|---|---|---|---|---|---|
| FEAT-AGT-001 | Agent definitions | Engineer | P0 | M3 | Reuse | L SH C | — | WO: `domain/agent.go`, ADR-010 |
| FEAT-AGT-002 | Provider preference and resolution | Engineer | P0 | M3 | Reuse | L SH C | AGT-001, CAP-001 | WO: `ResolvedAgent.Resolution` |
| FEAT-AGT-003 | Hermetic agent runs and tool scope | Platform | P0 | M3 | Reuse | L SH C E | AGT-001 | WO: ADR-023, `provider/claude/claude.go` flags |
| FEAT-AGT-004 | Agent and Skill snapshots per Step | Auditor | P0 | M3 | Reuse | L SH C | AGT-001 | WO: `StepExecution.Agent` |
| FEAT-AGT-005 | Teams | Lead | P1 | M7 | Reuse | SH C | IAM-004 | WO: `domain/team.go`, ADR-019; WP: `domain/team.go` |
| FEAT-AGT-006 | Organization-level Agent library | Lead | P2 | Backlog | Future | SH C | IAM-004 | WO: deferred in ROADMAP |

### 2.7 Skills and context (SKL)
| ID | Feature | Actor | P | Milestone | Class | Avail | Depends on | Evidence |
|---|---|---|---|---|---|---|---|---|
| FEAT-SKL-001 | Trusted, hash-pinned Skills | Engineer | P0 | M3 | Reuse | L SH C | PRJ-003 | WO: ADR-011, ADR-017 |
| FEAT-SKL-002 | Deterministic, recorded context assembly | Auditor | P0 | M3 | Reuse | L SH C | SKL-001, AGT-004 | WO: `runtime/executions/*/context.md` |
| FEAT-SKL-003 | Opt-in import of tool-native Skills | Engineer | P1 | M7 | Reuse | L SH | SKL-001 | WO: ADR-017 (`.claude/skills` import) |
| FEAT-SKL-004 | Context budget and least-context composition | Engineer | P1 | M5 | New | L SH C | SKL-002, OBS-002 | — (README principle "least context necessary") |
| FEAT-SKL-005 | Playbooks as knowledge notes | Engineer | P2 | Backlog | Future | L SH | PRJ-007 | WO: ADR-018 |

### 2.8 Capabilities and providers (CAP)
| ID | Feature | Actor | P | Milestone | Class | Avail | Depends on | Evidence |
|---|---|---|---|---|---|---|---|---|
| FEAT-CAP-001 | Typed Capability catalog | Platform | P0 | M3 | Reuse | L SH C | — | WO: `domain/capabilities.go`, ADR-012 |
| FEAT-CAP-002 | Built-in agent providers (Claude, Codex, OpenCode) | Engineer | P0 | M3 | Improve | L E | CAP-001 | WO: `provider/{claude,codex,opencode}`; WP: `runtime/cli_driver.go` |
| FEAT-CAP-003 | External provider protocol | Provider author | P0 | M3 | Improve | L SH E | API-001 | WO: `protocol/provider.schema.json`, `weave.provider/v1` |
| FEAT-CAP-004 | Remote-job providers (resumable, detachable) | Platform | P0 | M3 | Reuse | L SH C | EXE-002 | WO: `provider/remotejob`, `Resumable`, `Detachable` |
| FEAT-CAP-005 | Provider health and discovery | Engineer | P0 | M3 | Reuse | L SH E | CAP-001 | WO: probes, toolbox page, tool cache (F-9) |
| FEAT-CAP-006 | MCP client provider behind a gateway | Engineer | P1 | M5 | Improve | L SH E | POL-001 | WO: ADR-016 (built), ADR-025 gateway (not built) |
| FEAT-CAP-007 | Media and asset providers as plugins | Engineer | P2 | Backlog | Future | E | PLG-001 | WO: Blender, Meshy, Tripo, fal, Replicate, ElevenLabs, Pandoc, ffmpeg adapters |

### 2.9 Execution environments and workers (ENV)
| ID | Feature | Actor | P | Milestone | Class | Avail | Depends on | Evidence |
|---|---|---|---|---|---|---|---|---|
| FEAT-ENV-001 | Embedded local Worker | Platform | P0 | M3 | Improve | L | EXE-001 | WO: single process Runtime |
| FEAT-ENV-002 | Worker process and registration | Operator | P1 | M5 | New | SH C E | API-004, IAM-006 | WP: `INVESTIGATION-EXECUTION-BOUNDARY.md` (executor, not built) |
| FEAT-ENV-003 | Environment capability and health manifest | Operator | P1 | M5 | New | SH C E | ENV-002 | — |
| FEAT-ENV-004 | Leases, heartbeats, reassignment | Platform | P1 | M5 | New | SH C E | ENV-002 | — |
| FEAT-ENV-005 | Capability and label routing | Engineer | P1 | M5 | New | SH C E | ENV-003 | — |
| FEAT-ENV-006 | Project revision delivery to Workers | Platform | P1 | M5 | New | SH C E | ENV-002 | — |
| FEAT-ENV-007 | Artifact upload from Workers | Worker | P1 | M5 | New | SH C E | ENV-002, ART-004 | — |
| FEAT-ENV-008 | Container sandbox environment | Operator | P1 | M5 | Improve | SH C E | ENV-002 | WP: `runtime/podman.go` (`--cap-drop ALL`, limits); WP: `INVESTIGATION.md` (nested Podman blocked) |
| FEAT-ENV-009 | Windows workstation Worker (UE5 + MCP pilot) | Environment owner | P1 | M5 | New | E | ENV-002, EXE-003, ENV-011, ENV-013 | WO: GAP-05 (Windows unvalidated), F-31 Windows stress branch |
| FEAT-ENV-010 | Worktree isolation per Work Item | Platform | P0 | M3 | Reuse | L SH E | PRJ-001 | WO: `provider/gitrepo` (`git worktree add -B`) |
| FEAT-ENV-011 | Interactive desktop-session tools on Workers | Environment owner | P1 | M5 | New | E | ENV-002 | — |
| FEAT-ENV-012 | Managed Cloud Workers | Cloud customer | P2 | M9 | Future | C | ENV-008, OPS-008 | — |
| FEAT-ENV-013 | Environment-hosted MCP servers | Engineer, Environment owner | P1 | M5 | New | SH C E | ENV-003, CAP-006 | WO: MCP client (ADR-016), gateway not built (ADR-025) |
| FEAT-ENV-014 | Environment provisioning and lifecycle | Operator | P1 | M7 | New | SH C E | ENV-003, PLG-009 | — |

### 2.10 Validation and quality gates (VAL)
| ID | Feature | Actor | P | Milestone | Class | Avail | Depends on | Evidence |
|---|---|---|---|---|---|---|---|---|
| FEAT-VAL-001 | Commit-bound Local Checks | Engineer | P0 | M4 | Reuse | L SH E | ENV-010 | WO: `domain/localcheck.go`, ADR-029 |
| FEAT-VAL-002 | Hosted CI observation and classification | Engineer | P0 | M4 | Reuse | L SH C | INT-001 | WO: CI classification (`ci_unavailable`, `ambiguous`), F-24/26/28, F-36 |
| FEAT-VAL-003 | Quality-gate policy per Work Package | Lead | P0 | M4 | Improve | L SH C | VAL-001 | WO: `AutoloopPolicy.RequireChecks`, `LocalChecks`; F-43 (no editor) |
| FEAT-VAL-004 | Sandboxed Local Checks | Engineer | P1 | M5 | New | L SH E | VAL-001, ENV-008 | WO: residual risk "local checks have no sandbox" (`docs/security.md`) |
| FEAT-VAL-005 | Artifact Verification Runs | Engineer | P1 | M5 | Improve | SH C E | ENV-008 | WP: `domain/verification_run.go`, `PodmanRuntime.RunCommand` |
| FEAT-VAL-006 | Bounded CI repair loop | Engineer | P1 | M5 | Reuse | L SH C | VAL-002 | WO: `MaxRepairs`, `app/autoloop_evidence.go` (`selectEvidence`), F-35 |

### 2.11 Review and approval (REV)
| ID | Feature | Actor | P | Milestone | Class | Avail | Depends on | Evidence |
|---|---|---|---|---|---|---|---|---|
| FEAT-REV-001 | Durable Approval Step | Approver | P0 | M3 | Improve | L SH C | EXE-002, POL-002 | WO: `StepApproval`, `waitApproval`, F-31 |
| FEAT-REV-002 | Approval inbox | Approver | P0 | M4 | Improve | W D | REV-001, CLI-001 | WO: approvals inbox, F-23 stale notices |
| FEAT-REV-003 | Evidence-bound approvals | Approver, Auditor | P0 | M3 | Improve | L SH C | REV-001, ART-002 | WP: `domain/approval.go` (`ExecutionID`, `VerificationRunID`), migration 030 |
| FEAT-REV-004 | Agent review with structured verdict | Engineer | P0 | M4 | Reuse | L SH C | VAL-001 | WO: `VERDICT:` parsing, F-12 criteria table, F-38 `{{validation}}` |
| FEAT-REV-005 | Draft pull request delivery, human merge | Engineer | P0 | M4 | Reuse | L SH C | INT-001, PRJ-004 | WO: ADR-026 (`merge=false`, `awaiting_merge`), F-45 |
| FEAT-REV-006 | Change diff view | Approver | P0 | M4 | Improve | W D | ART-001 | WP: `computeDiff`, `Artifact.DiffAvailable`, ExecutionDetailPage |
| FEAT-REV-007 | Separation of duties | Lead | P1 | M7 | Reuse | SH C | AGT-005, IAM-002 | WO: Teams `separationOfDuties`; WP: `RequiredReviewerTeam` |
| FEAT-REV-008 | Approval expiry and escalation | Lead | P2 | Backlog | Future | SH C | REV-001 | — |

### 2.12 Artifacts and evidence (ART)
| ID | Feature | Actor | P | Milestone | Class | Avail | Depends on | Evidence |
|---|---|---|---|---|---|---|---|---|
| FEAT-ART-001 | Artifacts with Platform-computed hashes | Auditor | P0 | M3 | Reuse | L SH C | EXE-001 | WO: ADR-012 output files with role and hash |
| FEAT-ART-002 | Evidence bundle per Execution | Auditor | P0 | M3 | Improve | L SH C | ART-001, SKL-002 | WO: `context.md`, `events.jsonl`; WP: diff + verification |
| FEAT-ART-003 | Retention and redaction | Operator | P0 | M3 | New | L SH C | ART-002, REV-003 | WO: GAP-07 |
| FEAT-ART-004 | Artifact store port and object storage | Operator | P1 | M5 | New | SH C | ADR-0017 | WP: local filesystem `ArtifactRoot` |
| FEAT-ART-005 | Project note artifact index | Engineer | P2 | Backlog | Future | L | PRJ-007 | WO: ADR-005 (Obsidian notes) |
| FEAT-ART-006 | Artifact viewers (3D, diagrams, media) | Engineer | P2 | Backlog | Future | W D | PLG-004 | WO: `plugin/src/ui/viewer` (three.js, mermaid) |

### 2.13 Observability and cost (OBS)
| ID | Feature | Actor | P | Milestone | Class | Avail | Depends on | Evidence |
|---|---|---|---|---|---|---|---|---|
| FEAT-OBS-001 | Structured logs without secrets | Operator | P0 | M3 | Reuse | L SH C | — | WO: secrets port rule (`ports.go`), fixtures `invalid-secret-value` |
| FEAT-OBS-002 | Token and cost telemetry per Step | Engineer | P0 | M4 | New | L SH C | CAP-002 | WO: F-41 (`parse.go` drops `usage`, `total_cost_usd`); `StepExecution.Usage` exists |
| FEAT-OBS-003 | Execution timeline and logs view | Engineer | P0 | M4 | Improve | W D | EXE-004, CLI-001 | WO: plugin execution pages; WP: ExecutionDetailPage |
| FEAT-OBS-004 | Cost attribution per Work Item, Package, Project | Lead | P1 | M7 | New | SH C | OBS-002 | — |
| FEAT-OBS-005 | Metrics and tracing | Operator | P1 | M7 | New | SH C | — | — |
| FEAT-OBS-006 | Support diagnostics bundle | Operator | P1 | M7 | New | L SH | OBS-001 | — |
| FEAT-OBS-007 | Budgets and spend caps | Lead | P2 | Backlog | Future | SH C | OBS-004 | — |

### 2.14 Identity and access (IAM)
| ID | Feature | Actor | P | Milestone | Class | Avail | Depends on | Evidence |
|---|---|---|---|---|---|---|---|---|
| FEAT-IAM-001 | Local Instance authentication | Engineer | P0 | M3 | Improve | L | ADR-0034 | WO: `auth/token.go`, discovery 0600, Origin allowlist `app://obsidian.md` |
| FEAT-IAM-002 | User accounts and sessions | User | P1 | M7 | Improve | SH C W D | ADR-0028 | WP: `auth/auth.go` (bcrypt, HS256 JWT; no register/reset) |
| FEAT-IAM-003 | Organizations and membership | Lead | P1 | M7 | Improve | SH C | IAM-002 | WP: migrations 026/027; tenancy not enforced (`ListProjects`) |
| FEAT-IAM-004 | Roles and permissions | Lead | P1 | M7 | New | SH C | IAM-003 | WP: `admin`/`user` roles only |
| FEAT-IAM-005 | External identity provider (OIDC) | Operator | P1 | M7 | New | SH C | IAM-002, ADR-0028 | — |
| FEAT-IAM-006 | Machine identities and API tokens | Operator | P1 | M5 | New | SH C E | IAM-001 | — |
| FEAT-IAM-007 | Instance selection in clients | Engineer | P1 | M8 | New | W D | IAM-002 | WO: plugin settings endpoint override |

### 2.15 Policy, security and audit (POL)
| ID | Feature | Actor | P | Milestone | Class | Avail | Depends on | Evidence |
|---|---|---|---|---|---|---|---|---|
| FEAT-POL-001 | Capability policy and grants | Engineer | P0 | M3 | Improve | L SH C | CAP-001 | WO: grants ADR-024 (`needs_grant`), GAP-09 |
| FEAT-POL-002 | Consequential-action classification | Platform | P0 | M3 | New | L SH C | POL-001 | Amaru ADR-0006 list |
| FEAT-POL-003 | Audit trail of approvals, grants and policy changes | Auditor | P0 | M3 | New | L SH C | EXE-004 | — |
| FEAT-POL-004 | Egress consent | Engineer | P0 | M3 | Reuse | L SH C | POL-001 | WO: `transport/webhook/sender.go` private-host block, per-URL consent |
| FEAT-POL-005 | Workspace command sandbox | Engineer | P0 | M3 | Reuse | L E | AGT-003 | WO: `ValidWorkspaceCommand`, denylist `domain/agent.go`, `ShellConfiner` |
| FEAT-POL-006 | Provider confinement refusal | Platform | P0 | M3 | Improve | L E | CAP-002, AGT-003 | WO: Codex and OpenCode ignore `WorkspaceWrite` |
| FEAT-POL-007 | Secrets port and backends | Operator | P0 | M3 | Reuse | L SH C | — | WO: `userstore/credentials.json`, `WEAVE_SECRET_*` |
| FEAT-POL-008 | Vulnerability disclosure process | Security researcher | P1 | M7 | New | — | — | SECURITY_MODEL "Reporting" |
| FEAT-POL-009 | Tenant isolation | Cloud customer | P2 | M9 | Future | C | IAM-003, ADR-0022 | — |

### 2.16 Plugins (PLG)
| ID | Feature | Actor | P | Milestone | Class | Avail | Depends on | Evidence |
|---|---|---|---|---|---|---|---|---|
| FEAT-PLG-001 | Plugin package and manifest | Plugin author | P1 | M6 | New | L SH C | API-001 | WO: `provider.yaml` per user |
| FEAT-PLG-002 | Permission declaration and consent | Engineer | P1 | M6 | New | L SH C W D | PLG-001, POL-001 | WO: ADR-024 grants |
| FEAT-PLG-003 | TypeScript plugin SDK | Plugin author | P1 | M6 | New | — | API-003 | — |
| FEAT-PLG-004 | UI extension slots | Plugin author | P1 | M6 | New | W D | PLG-001, CLI-001 | — |
| FEAT-PLG-005 | Sandboxed TypeScript logic host | Plugin author | P1 | M6 | New | L SH C | PLG-001, ADR-0040 | — |
| FEAT-PLG-006 | Worker and local adapter components | Plugin author | P1 | M6 | Improve | E | PLG-001, API-004 | WO: external providers |
| FEAT-PLG-007 | Plugin lifecycle (install, update, disable, remove) | Engineer | P1 | M6 | New | L SH C | PLG-001 | — |
| FEAT-PLG-008 | Plugin configuration and secrets | Engineer | P1 | M6 | New | L SH C | PLG-001, POL-007 | — |
| FEAT-PLG-009 | Service plugins in self-hosted deployments | Operator | P1 | M7 | New | SH | PLG-001 | — |
| FEAT-PLG-010 | Plugin registry, provenance and signing | Plugin author | P2 | M10 | Future | SH C | PLG-001, ADR-0021 | — |
| FEAT-PLG-011 | Commercial plugin licensing | Plugin author | P3 | — | Future | C | PLG-010 | — |
| FEAT-PLG-012 | Third-party native Desktop extensions | Plugin author | P3 | — | Out of scope | D | — | — |

### 2.17 Clients (CLI)
| ID | Feature | Actor | P | Milestone | Class | Avail | Depends on | Evidence |
|---|---|---|---|---|---|---|---|---|
| FEAT-CLI-001 | Web client served by the Platform | Engineer | P0 | M4 | Improve | W | API-003, IAM-001 | WP: `dashboard/` (React 19, TanStack, shadcn) |
| FEAT-CLI-002 | Shared design system | Engineer | P0 | M4 | Reuse | W D | — | WP: `dashboard/src/components/ui`, `index.css` tokens, `CONVENTIONS.md` |
| FEAT-CLI-003 | Live updates from the event stream | Engineer | P0 | M4 | Improve | W D | EXE-004 | WO: `plugin/src/runtime/connection.ts` (reconnect with `since`) |
| FEAT-CLI-004 | Navigation and information architecture | Engineer | P0 | M4 | Improve | W D | CLI-001 | WP: `docs/IA_REVIEW.md` |
| FEAT-CLI-005 | Workflow text editor with validation | Engineer | P0 | M4 | Reuse | W D | WFL-001 | WP: `components/ui/yaml-editor.tsx` |
| FEAT-CLI-006 | Settings, providers and Policy screens | Engineer | P0 | M4 | Improve | W D | POL-001 | WO: settings/toolbox pages; F-6, F-43 |
| FEAT-CLI-007 | Accessibility baseline | Engineer | P0 | M4 | New | W D | CLI-002 | WP: Radix primitives |
| FEAT-CLI-008 | Desktop host | Engineer | P1 | M8 | New | D | ADR-0018, CLI-001 | — |
| FEAT-CLI-009 | Desktop supervision of a local Platform | Engineer | P1 | M8 | New | D | CLI-008 | — |
| FEAT-CLI-010 | Desktop native host API | Engineer | P1 | M8 | New | D | CLI-008 | — |
| FEAT-CLI-011 | Client updates and compatibility checks | Engineer | P1 | M8 | New | W D | API-005 | — |
| FEAT-CLI-012 | Command-line client | Engineer, Integrator | P1 | M7 | New | L SH C | API-001 | — |
| FEAT-CLI-013 | Obsidian plugin as an Amaru client | Engineer | P3 | — | Out of scope | — | ADR-0031 | WO: `plugin/` |
| FEAT-CLI-014 | Remote workstation view | Engineer | P2 | Backlog | Future | W D | ENV-003 | — |

### 2.18 Integrations (INT)
| ID | Feature | Actor | P | Milestone | Class | Avail | Depends on | Evidence |
|---|---|---|---|---|---|---|---|---|
| FEAT-INT-001 | GitHub forge integration | Engineer | P0 | M4 | Improve | L SH C | POL-007 | WO: `provider/github` via `gh` (`ports.GitHost`); WP: `capability/github.go` |
| FEAT-INT-002 | Periodic forge reconcile | Platform | P0 | M4 | Reuse | L SH C | INT-001 | WO: ADR-027, `--reconcile-interval` |
| FEAT-INT-003 | GitHub issue intake | Lead | P1 | M7 | Reuse | L SH C | INT-001, WRK-001 | WO: Intake, ADR-027, F-11 |
| FEAT-INT-004 | Outbound webhooks | Integrator | P1 | M7 | Reuse | SH C | POL-004 | WO: `app/hooks.go` (5 retries) |
| FEAT-INT-005 | Other forges (GitLab, Gitea) | Engineer | P2 | Backlog | Future | L SH C | PLG-001 | — |
| FEAT-INT-006 | Issue tracker links (Jira, Linear) | Lead | P2 | Backlog | Future | SH C | PLG-001 | — |
| FEAT-INT-007 | Amaru as an MCP server for agents | Engineer | P2 | Backlog | Future | L SH | POL-001 | WO: ADR-025 (not built) |

### 2.19 Operations and deployment (OPS)
| ID | Feature | Actor | P | Milestone | Class | Avail | Depends on | Evidence |
|---|---|---|---|---|---|---|---|---|
| FEAT-OPS-001 | Local install on Windows, macOS, Linux | Engineer | P0 | M4 | Improve | L | ENV-001 | WO: CI matrix (`.github/workflows/ci.yml`), GAP-05 |
| FEAT-OPS-002 | Persisted schema versioning and migrations | Operator | P0 | M3 | New | L SH C | ADR-0023 | WO: GAP-07, tolerant loaders; WP: `store/migrations.go` |
| FEAT-OPS-003 | Configuration scopes | Engineer | P0 | M3 | Improve | L SH C | ADR-0039 | WO: `.weave/config.yaml`, `settings.json`, user state dir |
| FEAT-OPS-004 | Compact self-hosted server | Operator | P1 | M7 | New | SH | OPS-002, IAM-002 | WP: `docker-compose.dev.yml` (dev only) |
| FEAT-OPS-005 | Backup and restore | Operator | P1 | M7 | New | L SH | OPS-002 | — |
| FEAT-OPS-006 | Upgrade with compatibility checks | Operator | P1 | M7 | New | L SH | OPS-002, API-005 | — |
| FEAT-OPS-007 | Distributed self-hosted deployment | Operator | P2 | M9 | Future | SH | ENV-004, ADR-0016 | — |
| FEAT-OPS-008 | Cloud tenancy and operations | Vendor | P2 | M9 | Future | C | POL-009 | — |
| FEAT-OPS-009 | Usage metering and billing | Vendor | P3 | — | Future | C | OBS-004 | WP: billing pages without backend |

### 2.20 Administration (ADM)
| ID | Feature | Actor | P | Milestone | Class | Avail | Depends on | Evidence |
|---|---|---|---|---|---|---|---|---|
| FEAT-ADM-001 | Instance health and status | Engineer, Operator | P0 | M4 | Improve | W D | API-005 | WO: `/v1/health`, runtime page (CPU/memory stub) |
| FEAT-ADM-002 | User and Organization administration | Operator | P1 | M7 | Improve | W | IAM-003 | WP: admin summary/users endpoints |
| FEAT-ADM-003 | Data export and deletion | Operator, User | P1 | M7 | New | SH C | IAM-003 | PRD PRIV-2 |

## 3. P0 and P1 feature details

Format per feature: **Outcome** · **Acceptance** (testable) · **Validation** · **Extension** (plugin involvement) · **Risks/open**.

### 3.1 Projects and work
- **FEAT-PRJ-001** Attach a Project. *Outcome:* an engineer points Amaru at an existing folder and gets a stable Project ID without any user file moving. *Acceptance:* attaching writes only to the metadata location; the ID is stable across restarts and path spelling; attaching an untrusted path never executes project content. *Validation:* unit + integration tests ported from Weave `project` package; manual attach on each OS. *Extension:* none. *Risks:* none beyond the metadata split of [ADR-0043](../architecture/adr/0043-project-definitions-in-git-teams-and-operational-state-in-platform.md) (attach writes nothing into the Project; `.amaru/` holds only committed definitions).
- **FEAT-PRJ-002** Many Projects per Instance. *Outcome:* one Instance serves all of a user's Projects. *Acceptance:* per-Project event streams, Executions and settings are isolated; one Project's failure does not stop others. *Validation:* integration test with two Projects; F-10 regression (secrets not shared across Projects). *Risks:* none known.
- **FEAT-PRJ-003** Project trust. *Outcome:* Amaru does nothing with project-supplied Skills, repositories or commands until the user trusts the Project. *Acceptance:* trust is content-bound; changes to trusted content require re-trust; untrusted Projects are read-only to Agents. *Validation:* ported tests for `trust.json` semantics. *Risks:* UX friction (F-1).
- **FEAT-PRJ-004** Linked repositories. *Outcome:* a Project can deliver to repositories outside its folder. *Acceptance:* only trusted Projects can link; each repository has a base branch and delivery Policy. *Validation:* integration test with a local bare repository. *Decision:* each linked repository gets its own scoped credential (a GitHub App installation token limited to that repository, or a per-repository deploy key), delivered per Lease and never stored on the Worker ([ADR-0024](../architecture/adr/0024-remote-worker-registration-identity-and-delivery.md)). *Risks:* hosts without per-repository tokens fall back to a user token, which is flagged in Evidence.
- **FEAT-PRJ-005** Project metadata store. *Outcome:* a decided, documented place for Amaru's per-Project state that separates knowledge in Git from operational state. *Acceptance:* the [data architecture](../architecture/DATA_ARCHITECTURE.md) classes are honored; operational state is not committed to user repositories by default. *Validation:* review against the data architecture. *Risks:* migration of existing `.weave/` data ([migration plan](../migration/MIGRATION_PLAN.md#8-persistence-and-state-migration)).
- **FEAT-PRJ-006** Detach and archive. *Outcome:* remove a Project from an Instance without touching its files. *Acceptance:* running Executions must be cancelled or finished first; history is retained per retention policy. *Validation:* integration test. *Risks:* none.
- **FEAT-WRK-001** Work Packages and Work Items. *Outcome:* human-owned units of intended work linked to Executions. *Acceptance:* item status is set only by humans or by an explicit Policy-governed delivery loop transition; package progress is derived; IDs are stable: a ULID internally and a display key `<PROJECTKEY>-<n>` unique per Instance; references across Instances use the Instance URL + ULID. *Validation:* ported domain tests. *Risks:* importing WO `WI-001` numbers needs a key mapping (kept as an alias).
- **FEAT-WRK-002** Dependencies. *Outcome:* an item does not start until its dependencies are done. *Acceptance:* cycles rejected; blocked state explains the dependency; external issue dependencies supported via INT-003 later. *Validation:* ported F-17 tests.
- **FEAT-WRK-003** Change events. *Outcome:* every client sees Work, Workflow and Agent edits without reloading. *Acceptance:* each mutation emits a typed event with a cursor; edits by other clients or by files on disk are detected. *Validation:* contract fixtures + integration test with two clients. *Risks:* closes Weave GAP-06 and F-7 (lost update).
- **FEAT-WRK-004** Supervised delivery loop. *Outcome:* the UC-1 flow runs with the human only approving and merging. *Acceptance:* opt-in per Work Package; every side effect is recorded before the next; never merges; draft PR only; a missing human decision parks one item, not the loop; restart-safe. *Validation:* ported Autoloop tests; a recorded supervised run on a real Go repository on each OS (Weave Cycle 7 shape). *Extension:* reviewers and checks are Capabilities, so plugins can supply them. *Risks:* Weave verdict "not ready for unattended engineering"; MVP scope is supervised only.
- **FEAT-WRK-005** Attempt and budget accounting. *Outcome:* runs are spent only on real attempts. *Acceptance:* a no-op implementation does not consume an attempt (F-39); unblocking resumes review when a commit exists (F-40); restart recoveries do not consume attempts (ADR-029). *Validation:* tests reproducing F-39/F-40; recorded run meeting "≤1 implementation and ≤1 review per accepted change". *Risks:* F-39/F-40 are Amaru acceptance tests, not Weave prerequisites ([ADR-0032](../architecture/adr/0032-migration-baseline-is-orkestra-dogfood-c8a2f6e.md)).

### 3.2 Workflows and execution
- **FEAT-WFL-001** Text Workflow definitions. *Outcome:* Workflows are reviewable YAML graphs. *Acceptance:* schema-validated; limits enforced (steps, edges, prompt size); legacy formats loaded only by an explicit migration. *Validation:* fixture tests. *Risks:* none.
- **FEAT-WFL-002** Versioning and run pinning. *Outcome:* every Execution records the exact definition (version and hash) it ran. *Acceptance:* editing a Workflow never changes a running or past Execution; resume after restart uses the pinned definition. *Decision:* versions are the Git history of `.amaru/`; each Execution stores the commit SHA, the content hash and an immutable snapshot of the definitions it ran, so resume never depends on Git. *Validation:* tests. *Risks:* snapshot size on large Workflows.
- **FEAT-WFL-003** Variables. *Acceptance:* substitution is typed and size-limited consistently with Work Item intent limits. *Validation:* F-15 regression test.
- **FEAT-WFL-004** Per-Step timeouts. *Outcome:* no Step runs forever. *Acceptance:* every Step has a default and overridable wall-clock timeout; on expiry the process tree is terminated and the Step fails with `timeout`. *Validation:* fault tests with a hanging fake provider, including Windows. *Defaults:* agent Steps 60 min, tool and check Steps 30 min, overridable up to an Organization maximum (default 8 h) ([RESILIENCE](../architecture/RESILIENCE.md)). *Risks:* long legitimate agent runs need overrides.
- **FEAT-WFL-005** Retries with backoff. *Acceptance:* per-Step retry policy only for Steps declared idempotent; attempts recorded as Events. *Validation:* tests. *Risks:* retry timers are PostgreSQL rows ([ADR-0042](../architecture/adr/0042-durable-execution-with-explicit-restart-semantics.md)).
- **FEAT-WFL-006** Parallel branches. *Acceptance:* independent DAG branches run concurrently within concurrency limits; join waits for all; cancellation propagates. *Decision:* Evidence is ordered by the Platform-assigned per-Execution Event `seq`, never by Worker clocks. *Validation:* tests.
- **FEAT-WFL-007** Workflow templates. *Acceptance:* a template instantiates into a Project with parameters; missing referenced Teams/Agents produce warnings (F-44). *Validation:* tests.
- **FEAT-WFL-008** Visual workflow editor. *Acceptance:* round-trips to the text format without loss; validation errors shown inline. *Validation:* component tests + manual. *Extension:* plugins can contribute Step types' forms. *Risks:* the editor is reimplemented in Amaru Web, using the Weave React Flow editor only as a behavioral reference ([ADR-0031](../architecture/adr/0031-obsidian-plugin-is-retired.md)).
- **FEAT-WFL-009** Inbound webhook trigger. *Acceptance:* HMAC with replay protection; only starts Workflows through Policy; rate-limited. *Validation:* ported hook tests.
- **FEAT-EXE-001** State machines. *Acceptance:* only documented transitions are possible ([domain model](../architecture/DOMAIN_MODEL.md#5-lifecycles)); invalid transitions error. *Validation:* ported domain tests.
- **FEAT-EXE-002** Durable state and recovery. *Outcome:* restarts never silently lose or duplicate work. *Acceptance:* each transition persisted before acknowledgement; on restart, waiting Steps keep waiting, resumable Steps resume, idempotent Steps retry as a new Lease and others end `interrupted` with a reason [ADR-0042](../architecture/adr/0042-durable-execution-with-explicit-restart-semantics.md); Approval waits survive. *Validation:* crash-injection suite killing the process in every Step state ([resilience](../architecture/RESILIENCE.md)). *Risks:* Amaru owns engine correctness ([ADR-0042](../architecture/adr/0042-durable-execution-with-explicit-restart-semantics.md)); the crash suite is mandatory.
- **FEAT-EXE-003** Cancellation. *Acceptance:* cancel terminates the whole process tree within a grace period on all three OSes. *Validation:* OS matrix tests; Windows validated for the first time (GAP-05).
- **FEAT-EXE-004** Event stream with replay. *Acceptance:* events ordered per Project and per Execution; clients resume from a cursor; a lost window yields an explicit resync signal. *Validation:* contract fixtures; reconnect tests. *Decision:* replay window 7 days compact, 30 days server and Cloud; transport is WebSocket with a resumable cursor ([ADR-0025](../architecture/adr/0025-api-and-protocol-versioning-policy.md)).
- **FEAT-EXE-005** Idempotent commands. *Acceptance:* every mutating command accepts an idempotency key; duplicates return the original result. *Validation:* tests.
- **FEAT-EXE-006** Concurrency limits. *Acceptance:* global, per-Project (default 2) and per-Environment agent slots, checked by the Platform before an assignment is published; queued Steps visible as queued with the reason. *Validation:* ported tests.
- **FEAT-EXE-007** Scheduling queue. *Acceptance:* priorities and per-Project fairness across Workers. *Validation:* simulation tests. *Risks:* fairness over JetStream work-queue streams per Worker pool ([ADR-0016](../architecture/adr/0016-nats-jetstream-message-bus-embedded-in-compact-installs.md)).
- **FEAT-EXE-008** Quota pause. *Acceptance:* a provider quota refusal pauses that provider until the stated reset time, without consuming attempts. *Validation:* F-27 regression test.

### 3.3 API and contracts
- **FEAT-API-001** Versioned public API. *Outcome:* one contract for every client, plugin and Worker. *Acceptance:* OpenAPI + JSON Schema in `packages/contracts`; every route registered by the Platform appears in the contract (drift check in tests); additive within a major version. *Validation:* contract test enumerating server routes against the spec. *Risks:* drift; versioning policy [ADR-0025](../architecture/adr/0025-api-and-protocol-versioning-policy.md).
- **FEAT-API-002** Fixtures in Go and TypeScript. *Acceptance:* each schema has valid and invalid fixtures validated in both languages in `make check`. *Validation:* the tests themselves.
- **FEAT-API-003** Generated TypeScript SDK. *Acceptance:* generated from the OpenAPI contract with `openapi-typescript` (types) and `openapi-fetch` (client); no hand-written request types in clients; committed generated files carry a header and a staleness check. *Validation:* staleness check in `make check`. *Extension:* the plugin SDK builds on it.
- **FEAT-API-004** Worker protocol. *Acceptance:* versioned schema for register, heartbeat, lease, assignment, events, output upload, cancel; fixtures. *Validation:* fixtures + M5 spike; Protobuf over WebSocket ([ADR-0029](../architecture/adr/0029-worker-protocol-over-websocket-and-protobuf.md)).
- **FEAT-API-005** Compatibility negotiation. *Acceptance:* clients learn the Instance's contract version before calling; incompatible versions produce an explicit UI state. *Validation:* tests with a mismatched fixture.

### 3.4 Agents, Skills, Capabilities
- **FEAT-AGT-001** Agent definitions. *Acceptance:* schema-validated; edits detected and versioned; deleting an Agent never rewrites history. *Validation:* ported tests; F-7 lost-update fix.
- **FEAT-AGT-002** Provider resolution. *Acceptance:* resolution recorded as preference/fallback/default per Step. *Validation:* ported tests.
- **FEAT-AGT-003** Hermetic runs and tool scope. *Acceptance:* agents get only the tools their scope allows; read-only Agents cannot write; MCP servers only to strict-isolation providers. *Validation:* ported provider argv tests; recorded real runs.
- **FEAT-AGT-004** Snapshots. *Acceptance:* every Step Execution stores the Agent and Skill versions used. *Validation:* tests.
- **FEAT-AGT-005** Teams. *Acceptance:* Teams have members (humans, Agents) with roles; referenced by review and approval Steps. *Decision:* Teams are Organization-scoped Platform state referenced by slug from definitions ([ADR-0043](../architecture/adr/0043-project-definitions-in-git-teams-and-operational-state-in-platform.md)); Agents are principals without login that act only through Executions ([ADR-0038](../architecture/adr/0038-roles-teams-and-separation-of-duties.md)). *Validation:* ported tests.
- **FEAT-SKL-001** Trusted Skills. *Acceptance:* a bad or untrusted Skill reference refuses the run before start; pinned hashes enforced. *Validation:* ported tests.
- **FEAT-SKL-002** Context assembly. *Acceptance:* the same inputs produce byte-identical context; the context is stored as Evidence. *Validation:* golden tests.
- **FEAT-SKL-003** Tool-native Skill import. *Acceptance:* opt-in per Project; imported Skills go through the same trust. *Validation:* ported tests.
- **FEAT-SKL-004** Context budget. *Acceptance:* each Step's context has a size budget and a recorded list of included items with sizes. *Default:* 8,000 tokens of Skills and Knowledge per Agent Step, configurable per Agent. *Validation:* tests; benefit measured as cost per accepted change from usage records ([ADR-0033](../architecture/adr/0033-usage-and-cost-telemetry-model.md)) on recorded runs with and without the budget.
- **FEAT-CAP-001** Capability catalog. *Acceptance:* specs are data; adding a Capability changes no core interface. *Validation:* the PRD §12 Provider test.
- **FEAT-CAP-002** Agent providers. *Acceptance:* each provider declares, for every confinement flag, whether it enforces it, and enforces the ones it declares (refusing the Step when a requested flag is not enforceable is POL-006); usage parsed (OBS-002). *Validation:* captured-output tests + recorded real runs per provider. *Risks:* vendor CLI changes.
- **FEAT-CAP-003** External provider protocol. *Acceptance:* `amaru.provider/v1`, adapted from `weave.provider/v1`; no compatibility shim (Weave Providers are ported); fixtures. *Validation:* a provider built outside the Platform passes the conformance fixtures.
- **FEAT-CAP-004** Remote-job providers. *Acceptance:* a remote job survives a Platform restart and is reattached. *Validation:* ported resume tests.
- **FEAT-CAP-005** Provider health. *Acceptance:* each provider reports availability and version; the UI shows why a provider is unavailable. *Validation:* tests.
- **FEAT-CAP-006** MCP behind a gateway (M5, needed by FEAT-ENV-013). *Acceptance:* agents reach MCP servers only through the audited Worker-side MCP gateway ([ADR-0035](../architecture/adr/0035-agents-run-beside-environment-hosted-tools.md)); secrets never passed to agents. *Validation:* tests + recorded run. *Extension:* MCP servers can be plugin components. *Risks:* the Weave ADR-025 design is not built; Amaru builds it on the Worker.

### 3.5 Environments and validation
- **FEAT-ENV-001** Embedded Worker. *Acceptance:* in compact mode the Worker runs in the Platform process but talks to the coordinator only through the Worker interface used by remote Workers. *Validation:* the same assignment tests run against embedded and separate Workers.
- **FEAT-ENV-002** Worker process and registration. *Acceptance:* a Worker connects outbound, authenticates with a machine identity, and appears in the environment registry. *Validation:* spike then integration test on two machines.
- **FEAT-ENV-003** Capability and health manifest. *Acceptance:* Workers declare OS, tools, Capabilities, labels, capacity; health updates by heartbeat. *Validation:* tests.
- **FEAT-ENV-004** Leases. *Acceptance:* Step assignment is leased; lost Workers' Steps are retried as a new Lease only if their Capability is declared idempotent, otherwise end `interrupted` [ADR-0042](../architecture/adr/0042-durable-execution-with-explicit-restart-semantics.md); duplicated completion reports are ignored. *Validation:* network-partition fault tests ([resilience](../architecture/RESILIENCE.md)).
- **FEAT-ENV-005** Routing. *Acceptance:* Steps route by required Capability and labels; no eligible Worker yields a visible waiting state. *Validation:* tests.
- **FEAT-ENV-006** Revision delivery. *Acceptance:* a Worker checks out the exact commit the Step targets; it never pushes except through granted Capabilities. *Validation:* integration test.
- **FEAT-ENV-007** Artifact upload. *Acceptance:* outputs uploaded with hashes verified by the Platform; partial uploads resumed or discarded. *Validation:* tests.
- **FEAT-ENV-008** Container sandbox. *Acceptance:* single-level rootless containers with dropped capabilities and resource limits; no nested containers required. *Validation:* integration test on Linux. *Risks:* Weave prototype `INVESTIGATION.md` shows nested Podman blocked.
- **FEAT-ENV-009** Windows Worker, UE5 + MCP pilot. *Outcome:* a Windows VM or workstation with Unreal Engine 5 and an Unreal MCP server does UE-related work for a Work Item. *Acceptance:* the Worker advertises `engine.*` Capabilities and its MCP servers; an agent Step routed by label (e.g. `ue5`) uses the editor's MCP tools under Policy; build/test Steps run with cancellation and process-tree kill; outputs (assets, logs, screenshots) return as hashed Artifacts. *Validation:* recorded manual run on a Windows VM. *Risks:* editor stability, license seats, GPU availability; the agent runs on the Worker beside the editor behind a Worker-side MCP gateway ([ADR-0035](../architecture/adr/0035-agents-run-beside-environment-hosted-tools.md)); the pilot uses Epic's Unreal MCP plugin (UE 5.8, experimental) for engine versions that ship it; a community MCP server is used only for older engine versions.
- **FEAT-ENV-011** Interactive desktop-session tools. *Outcome:* tools that need a logged-in desktop session (the Unreal Editor, Blender UI) can be driven by Steps. *Acceptance:* a Worker can run inside an interactive user session, report session and editor health (running, responsive, project loaded), start or attach to the editor before a Step and refuse Steps when the session is unavailable. *Validation:* recorded run with a locked and unlocked session. *Risks:* OS session policies; editor crashes mid-Step.
- **FEAT-ENV-013** Environment-hosted MCP servers. *Outcome:* MCP servers that live on a remote Environment (e.g. inside the Unreal Editor) are usable by agents. *Acceptance:* the Worker manifest lists MCP servers with their tools; agents running on that Worker reach them through the Worker-side MCP gateway ([ADR-0035](../architecture/adr/0035-agents-run-beside-environment-hosted-tools.md)); every tool call is Policy-checked per tool and recorded as an Event; MCP credentials never leave the Environment. *Validation:* integration test with a fake MCP server on a remote Worker; recorded UE5 run. *Extension:* MCP servers may be shipped as plugin `worker-adapter` components. *Risks:* MCP protocol versions.
- **FEAT-ENV-014** Environment provisioning and lifecycle. *Outcome:* operators define Environment templates (e.g. "Windows + UE5.x + MCP plugin + project") and Amaru starts them on demand. *Acceptance:* templates declare image, Capabilities, labels and cost class; Environments start when a Step needs them and stop after an idle timeout; a reset-to-snapshot option gives clean runs; provisioners (Proxmox, libvirt, Hyper-V, cloud APIs) are plugin `service` components; start and stop are Events with cost attribution. *Validation:* integration test with a fake provisioner; recorded run with one real hypervisor. *Risks:* provisioner diversity; boot time. License activation is part of the template, with keys delivered as per-Lease secrets ([execution environments §9](../architecture/EXECUTION_ENVIRONMENTS.md)).
- **FEAT-ENV-010** Worktree isolation. *Acceptance:* one worktree per Work Item; agents write only inside it; base refresh merges, never rebases or force-pushes (WO ADR-028). *Validation:* ported tests.
- **FEAT-VAL-001** Local Checks. *Acceptance:* bare executables, no shell, bounded concurrency, confined to the worktree; results bound to head commit and config hash. *Validation:* ported ADR-029 tests.
- **FEAT-VAL-002** Hosted CI observation. *Acceptance:* CI states classified (passed, failed, unavailable, ambiguous) with the cause shown (F-36). *Validation:* ported tests with fakes.
- **FEAT-VAL-003** Quality-gate policy. *Acceptance:* a Work Package declares required checks before review; editable in the UI (F-43). *Validation:* tests + UI test.
- **FEAT-VAL-004** Sandboxed Local Checks. *Acceptance:* Local Checks run in an OS or container sandbox when the environment offers one; otherwise the UI states they are unsandboxed. *Validation:* tests per OS.
- **FEAT-VAL-005** Verification Runs. *Acceptance:* a check runs against a stored Artifact in a sandbox and produces Evidence linked to approvals. *Validation:* ported prototype tests.
- **FEAT-VAL-006** CI repair loop. *Acceptance:* bounded repairs; evidence selection keeps failure windows (F-35). *Validation:* ported tests.

### 3.6 Review, approval and evidence
- **FEAT-REV-001** Durable Approval Step. *Outcome:* consequential actions wait for a human. *Acceptance:* Approval state survives restarts; decisions record identity, time and the Evidence shown; rejection ends or reroutes per Workflow. *Validation:* crash tests during waits; F-31 regression.
- **FEAT-REV-002** Approval inbox. *Acceptance:* all pending approvals across Projects in one place; stale notices disappear (F-23). *Validation:* UI tests.
- **FEAT-REV-003** Evidence-bound approvals. *Acceptance:* an approval references the exact Execution, commit, diff and check results; approving after new commits is refused. *Validation:* tests.
- **FEAT-REV-004** Agent review. *Acceptance:* the review prompt includes validation Evidence of the reviewed head; verdict is structured; missing verdict fails the Step. *Validation:* ported tests; recorded runs.
- **FEAT-REV-005** Draft PR delivery. *Acceptance:* PRs are drafts unless Policy says ready-for-review; marked ready after approval (F-45); never merged by Amaru in MVP. *Validation:* tests with a fake forge; recorded run.
- **FEAT-REV-006** Diff view. *Acceptance:* the change set of a Step or Work Item is shown as a unified diff with file list. *Validation:* UI tests.
- **FEAT-REV-007** Separation of duties. *Acceptance:* per gate ([ADR-0038](../architecture/adr/0038-roles-teams-and-separation-of-duties.md)): the approver differs from the initiator and from the human sponsor of the Agent that produced the Evidence; a gate may require a named reviewer Team; Agents never approve; disabled and recorded in single-user compact installs. *Validation:* tests.
- **FEAT-ART-001** Hashed Artifacts. *Acceptance:* the Platform, not the Provider, computes hashes; outputs outside the output location are rejected. *Validation:* ported tests.
- **FEAT-ART-002** Evidence bundle. *Acceptance:* one query returns context, snapshots, diffs, logs, checks, approvals and usage for an Execution. *Validation:* contract fixtures + audit walkthrough (PRD §12).
- **FEAT-ART-003** Retention and redaction. *Acceptance:* configurable retention per data class; known secret patterns redacted from stored provider output. *Note:* the retention classes keep Artifacts while an Approval or accepted Work Item references them ([DATA_ARCHITECTURE §5](../architecture/DATA_ARCHITECTURE.md#5-retention-and-redaction)), so the Evidence bundle (ART-002) and Approval-to-Evidence bindings (REV-003) come first. *Validation:* tests. *Risks:* redaction completeness.
- **FEAT-ART-004** Artifact store port and object storage. *Acceptance:* an Artifact store port with a filesystem adapter by M5 (needed for Worker uploads) and an S3-compatible adapter by M7. *Validation:* adapter conformance tests. *Risks:* S3 compatibility of the reference backend RustFS ([ADR-0017](../architecture/adr/0017-content-addressed-artifact-store-filesystem-or-s3.md)).

### 3.7 Observability, identity, policy
- **FEAT-OBS-001** Structured logs. *Acceptance:* JSON logs with correlation IDs; secret fixtures prove absence. *Validation:* tests.
- **FEAT-OBS-002** Token and cost telemetry. *Acceptance:* each agent Step records input/output tokens, model and cost when the provider reports them, and "unknown" otherwise. *Validation:* captured-output tests per provider. *Risks:* providers report differently.
- **FEAT-OBS-003** Timeline and logs view. *Acceptance:* live Step timeline with logs, state and Evidence links. *Validation:* UI tests + manual.
- **FEAT-OBS-004** Cost attribution. *Acceptance:* totals per Work Item, Package, Project and accepted change, including wasted runs. *Validation:* tests over recorded data.
- **FEAT-OBS-005** Metrics and tracing. *Acceptance:* OpenTelemetry-compatible export, off by default. *Validation:* integration test with a collector.
- **FEAT-OBS-006** Diagnostics bundle. *Acceptance:* one command collects versions, config (secrets removed) and recent logs. *Validation:* test that the bundle has no secrets.
- **FEAT-IAM-001** Local authentication. *Acceptance:* loopback only; token required; the browser client is protected against cross-origin and DNS-rebinding requests (launch code + session cookie, [ADR-0034](../architecture/adr/0034-local-web-client-authentication-with-launch-code-and-cookie.md)). *Validation:* security tests.
- **FEAT-IAM-002** User accounts. *Acceptance:* sign-in, sessions, revocation; password reset where local accounts are enabled. *Validation:* tests. *Risks:* IdP interoperability ([ADR-0028](../architecture/adr/0028-identity-local-accounts-oidc-and-opaque-tokens.md)).
- **FEAT-IAM-003** Organizations. *Acceptance:* every Project belongs to one Organization; all queries are scoped. *Validation:* tests proving no cross-Organization reads.
- **FEAT-IAM-004** Roles. *Acceptance:* role-based permissions for the fixed Organization roles Owner, Admin, Maintainer, Contributor and Viewer, narrowed or raised per Project ([ADR-0038](../architecture/adr/0038-roles-teams-and-separation-of-duties.md)). *Validation:* tests.
- **FEAT-IAM-005** OIDC. *Acceptance:* sign-in through an external OIDC provider. *Validation:* integration test with a test IdP.
- **FEAT-IAM-006** Machine identities. *Acceptance:* Workers use Ed25519 keys enrolled with one-time tokens ([ADR-0024](../architecture/adr/0024-remote-worker-registration-identity-and-delivery.md)); CLI automation and service plugins use scoped, revocable opaque tokens ([ADR-0028](../architecture/adr/0028-identity-local-accounts-oidc-and-opaque-tokens.md)). *Validation:* tests.
- **FEAT-IAM-007** Instance selection. *Acceptance:* clients store several Instances and show which one is active. *Validation:* UI tests.
- **FEAT-POL-001** Capability policy. *Outcome:* nothing runs that Policy does not allow. *Acceptance:* ungranted use refused before start with an actionable message; grants are recorded, revocable and attributed. *Validation:* tests + recorded attempt without a grant.
- **FEAT-POL-002** Consequential-action classification. *Acceptance:* the ADR-0006 list (push/publish, deletion outside outputs, spend, new egress, permission grants) is enforced for every trigger. *Validation:* tests per action and trigger.
- **FEAT-POL-003** Audit trail. *Acceptance:* every Approval, Grant and Policy change is an Event with actor and reason. *Validation:* tests.
- **FEAT-POL-004** Egress consent. *Acceptance:* per-URL consent; private and loopback destinations blocked unless allowlisted. *Validation:* ported tests.
- **FEAT-POL-005** Workspace command sandbox. *Acceptance:* allowlist format and denylist preserved; commands refused if the provider cannot confine them. *Validation:* ported tests.
- **FEAT-POL-006** Confinement refusal. *Note:* refusal is implemented in the adapted providers ([MIGRATION_PLAN §3](../migration/MIGRATION_PLAN.md#3-components-and-likely-dispositions)), so it needs the providers' enforcement declarations (CAP-002) and the requested write and tool restrictions (AGT-003). *Acceptance:* a provider that cannot enforce write or tool restrictions refuses the Step. *Validation:* tests for Codex and OpenCode.
- **FEAT-POL-007** Secrets port. *Acceptance:* secrets write-only from clients; visible at Organization or Project scope, encrypted with the per-Organization data key ([ADR-0022](../architecture/adr/0022-cloud-tenancy-row-level-isolation-by-organization.md)); never in events or logs. *Validation:* ported fixture tests.
- **FEAT-POL-008** Disclosure process. *Acceptance:* `SECURITY.md` with a contact and response process before the first public release. *Validation:* review.

### 3.8 Plugins
- **FEAT-PLG-001** Package and manifest. *Acceptance:* one manifest declares identity, version, compatibility range, components with targets, permissions and configuration schema ([extensibility](../architecture/EXTENSIBILITY.md)). *Validation:* schema fixtures. *Risks:* one manifest for all components may be heavy for authors ([ADR-0012](../architecture/adr/0012-plugin-package-with-targeted-components.md); validation A6 at M6).
- **FEAT-PLG-002** Permissions and consent. *Acceptance:* permissions shown before enabling; enforced by the Platform for every component. *Validation:* tests with a plugin exceeding its permissions.
- **FEAT-PLG-003** TypeScript SDK. *Acceptance:* typed APIs for commands, events, UI contributions, Capability handlers; versioned with the contract. *Validation:* example plugins in tests.
- **FEAT-PLG-004** UI extension slots. *Acceptance:* extensions render in isolated frames in Web and Desktop identically, and reach the Platform only through the SDK bridge. *Validation:* the same example plugin works in both clients.
- **FEAT-PLG-005** Sandboxed logic host. *Acceptance:* TypeScript logic runs in QuickJS-in-Wasm on wazero with no ambient filesystem or network; resource limits; crash isolated. *Validation:* escape tests. *Risks:* QuickJS performance ([ADR-0040](../architecture/adr/0040-plugin-logic-runs-in-a-separate-plugin-host-process-built-with-javy.md)).
- **FEAT-PLG-006** Worker and local adapters. *Acceptance:* a plugin adds a Provider on a Worker through the provider protocol; local adapters declare they need a user machine. *Validation:* a plugin Provider runs on a remote Worker.
- **FEAT-PLG-007** Lifecycle. *Acceptance:* install, update, disable, remove without restarting the Platform; failed updates roll back. *Validation:* tests.
- **FEAT-PLG-008** Configuration and secrets. *Acceptance:* configuration validated against the manifest schema; secrets through the secrets port. *Validation:* tests.
- **FEAT-PLG-009** Service plugins. *Acceptance:* an operator deploys a plugin service next to the Platform; it authenticates with a machine identity and receives only declared events. *Validation:* integration test.

### 3.9 Clients
- **FEAT-CLI-001** Web client. *Outcome:* the MVP UI, opened from the local Platform. *Acceptance:* every action goes through the generated SDK; no page shows data without a backend (no placeholders presented as real); works in current Chromium, Firefox and Safari/WebKit. *Validation:* component tests + end-to-end browser tests + recorded manual runs. *Risks:* prototype pages built without backend need pruning ([prototype inventory](../migration/weave-prototype-inventory.md)).
- **FEAT-CLI-002** Design system. *Acceptance:* `packages/ui` with tokens, light/dark themes and shadcn components used by Web and Desktop. *Validation:* visual review + component tests.
- **FEAT-CLI-003** Live updates. *Acceptance:* UI reflects events within one round trip; reconnects resume from cursor. *Validation:* tests ported from `connection.ts`.
- **FEAT-CLI-004** Navigation. *Acceptance:* scopes (Instance → Project → Work) are unambiguous, per `IA_REVIEW.md` findings. *Validation:* review + usability walkthrough.
- **FEAT-CLI-005** Workflow text editor. *Acceptance:* schema-aware validation before save. *Validation:* component tests.
- **FEAT-CLI-006** Settings, providers, Policy. *Acceptance:* providers, Local Checks, Policy grants and workflow variables editable (F-6, F-43). *Validation:* UI tests.
- **FEAT-CLI-007** Accessibility. *Acceptance:* keyboard operable, labelled controls, contrast meeting WCAG 2.2 AA on core flows. *Validation:* automated accessibility checks + manual keyboard pass.
- **FEAT-CLI-008** Desktop host. *Acceptance:* installable builds on three OSes wrapping the shared UI. *Validation:* recorded install per OS. *Risks:* WebView differences and signing (Tauri v2, [ADR-0018](../architecture/adr/0018-desktop-shell-is-tauri-v2.md)).
- **FEAT-CLI-009** Local supervision. *Acceptance:* Desktop can start, stop and update a local Platform and still talks to it only through the public API. *Validation:* tests + manual.
- **FEAT-CLI-010** Native host API. *Acceptance:* notifications, file pickers, OS keychain exposed through a declared, permission-checked API. *Validation:* tests.
- **FEAT-CLI-011** Updates and compatibility. *Acceptance:* clients refuse to operate against incompatible Instances with an explanation. *Validation:* tests.
- **FEAT-CLI-012** Command-line client. *Acceptance:* the remote CLI covers Work, Executions, Approvals and administration via the SDK contract. Local operator subcommands of the Platform binary (`open`, `migrate`, `backup`, `import weave`, `admin check`, `worker`) are not this feature; they ship with the Platform from M3/M4. *Validation:* tests.

### 3.10 Integrations, operations, administration
- **FEAT-INT-001** GitHub integration. *Acceptance:* PRs, checks, logs, issues via a forge port; credentials only to forge processes. *Validation:* ported tests with fakes; recorded run.
- **FEAT-INT-002** Reconcile. *Acceptance:* periodic reconcile catches missed webhooks. *Validation:* ported tests.
- **FEAT-INT-003** Issue intake. *Acceptance:* labelled issues become Work Items with links back; dependencies imported. *Validation:* ported tests.
- **FEAT-INT-004** Outbound webhooks. *Acceptance:* retries with backoff; consent per URL. *Validation:* ported tests.
- **FEAT-OPS-001** Local install. *Acceptance:* one signed archive per OS (no admin rights; native installers come with Desktop at M8); no Docker; the Platform starts and serves the Web client. *Validation:* recorded install per OS.
- **FEAT-OPS-002** Schema migrations. *Acceptance:* every persisted format carries a version; migrations are tested forward on fixtures; downgrade refused with a message. *Validation:* migration tests.
- **FEAT-OPS-003** Configuration scopes. *Acceptance:* resolution order deployment < Organization < Project < user preferences with locks ([ADR-0039](../architecture/adr/0039-configuration-scopes-and-resolution-order.md)). *Validation:* tests.
- **FEAT-OPS-004** Compact self-hosted server. *Acceptance:* one process plus its database on one host serves several users. *Validation:* recorded install on a reference host.
- **FEAT-OPS-005** Backup and restore. *Acceptance:* documented backup; a restore drill reproduces Projects, history and Artifacts. *Validation:* recorded drill.
- **FEAT-OPS-006** Upgrade. *Acceptance:* in-place upgrade with migrations; incompatible clients and Workers detected. *Validation:* upgrade test from the previous release.
- **FEAT-ADM-001** Instance health. *Acceptance:* health, version, storage use and provider availability visible; no stub metrics shown as real. *Validation:* UI tests.
- **FEAT-ADM-002** User and Organization administration. *Acceptance:* invite, remove, change roles; audit events. *Validation:* tests.
- **FEAT-ADM-003** Data export and deletion. *Acceptance:* export an Organization's data in documented formats; deletion removes it within the retention rules. *Validation:* tests.

## 4. Requirement traceability

Every FR and NFR is realized by at least one feature. NFR details: [NON_FUNCTIONAL_REQUIREMENTS](NON_FUNCTIONAL_REQUIREMENTS.md).

| Requirement | Features |
|---|---|
| FR-1 | PRJ-001, PRJ-003, PRJ-005 |
| FR-2 | WRK-001, WRK-002, WRK-003 |
| FR-3 | INT-003, INT-006 |
| FR-4 | PRJ-004, REV-005, INT-001 |
| FR-10 | WFL-001, WFL-002, CLI-005, WFL-008 |
| FR-11 | EXE-001, EXE-003, EXE-004, EXE-005 |
| FR-12 | EXE-002, CAP-004 |
| FR-13 | REV-001, REV-002, REV-003 |
| FR-14 | WFL-004, WFL-005, WFL-006, WFL-011 |
| FR-15 | WFL-007 |
| FR-16 | WRK-004, VAL-002, INT-002 |
| FR-17 | WRK-005, EXE-008 |
| FR-20 | AGT-001, AGT-002, AGT-003, AGT-004 |
| FR-21 | SKL-001, SKL-002, SKL-003 |
| FR-22 | CAP-001, CAP-005 |
| FR-23 | CAP-003 |
| FR-24 | POL-001, POL-002, POL-006 |
| FR-25 | REV-004 |
| FR-30 | ART-001 |
| FR-31 | ART-002, REV-006, OBS-003 |
| FR-32 | ART-004, ART-003 |
| FR-33 | OBS-002, OBS-004 |
| FR-34 | VAL-001, VAL-003, ENV-010 |
| FR-40 | API-001, API-002, API-003 |
| FR-41 | CLI-001, CLI-002, CLI-003, CLI-004, CLI-006, CLI-007 |
| FR-42 | PLG-001 … PLG-009 |
| FR-43 | ENV-002 … ENV-009, ENV-011, ENV-013, API-004 |
| FR-47 | ENV-014 |
| FR-44 | AGT-005, REV-007, IAM-004 |
| FR-45 | CLI-008, CLI-009, CLI-010, CLI-011, IAM-007 |
| FR-46 | IAM-002, IAM-003, IAM-005, IAM-006, ADM-002 |
| NFR-1 | OPS-001, EXE-003 |
| NFR-2 | API-001, API-002, API-005 |
| NFR-3 | EXE-002, REV-001 |
| NFR-4 | OBS-001, OBS-003, POL-003 |
| NFR-6 | ART-003, OPS-002 |
| NFR-7 | OPS-004, OPS-005, OPS-006 |
| NFR-8 | OPS-008, POL-009, ENV-012 |

Feature-to-milestone traceability and the reverse mapping are in the [roadmap](../architecture/ROADMAP.md#feature-traceability).

## 5. Out of scope

| Item | Reason | Revisit when |
|---|---|---|
| Replacing Git, GitHub, IDEs, CI or issue trackers | Not the product (PRD §1). | Never. |
| Automatic merging of pull requests | Human authority ([ADR-0006](../architecture/adr/0006-human-authorization-for-consequential-actions.md)); Weave Cycle 7 verdict. | A Policy-governed merge is proposed with evidence from supervised runs. |
| Autonomous creation of Agents, Skills or Workflows by agents | Human authority; Weave ADR-020 declined agent write access. | Evidence and an ADR. |
| In-process third-party plugins in the Platform or Desktop host | Isolation ([ADR-0005](../architecture/adr/0005-plugin-extensibility-independent-of-desktop.md)). | Never for third parties; first-party only. |
| Third-party native Desktop extensions (FEAT-PLG-012) | Trust and portability; Tauri plugins are framework-level. | A use case the host API cannot serve. |
| Docker or Podman as a requirement for local installs | NFR-1. | Never for local; allowed for self-hosted. |
| Mobile clients | No identified need. | User research. |
| Billing (FEAT-OPS-009) | Only meaningful with Cloud. | Cloud pilot. |
| Real-time collaborative editing | No identified need. | Team usage evidence. |
| Obsidian plugin as an Amaru client (FEAT-CLI-013) | Retired ([ADR-0031](../architecture/adr/0031-obsidian-plugin-is-retired.md)). | Never. |
| Unity/Unreal-specific product features beyond Worker capability | Delivered as plugins on Workers. | — |

## 6. Priority distribution

| Priority | Count | Milestones |
|---|---|---|
| P0 | 73 | M2 (3), M3 (47), M4 (23) |
| P1 | 60 | M5 (22), M6 (8), M7 (25), M8 (5) |
| P2 | 21 | M9 (4), M10 (1), Backlog (16) |
| P3 | 5 | unscheduled |
| **Total** | **159** | |

Counts are maintained by hand; the [roadmap traceability check](../architecture/ROADMAP.md#feature-traceability) is the source for per-milestone lists.
