/**
 * GENERATED FILE. DO NOT EDIT.
 * Source: packages/contracts/openapi/openapi.yaml, by openapi-typescript.
 * Regenerate with `pnpm run generate` in packages/sdk; `make sdk-ts` fails when this file is stale.
 */
export interface paths {
    "/health": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Liveness + versions. The only route reachable without a token. */
        get: operations["getHealth"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/projects": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Projects attached to this Platform. */
        get: operations["listProjects"];
        put?: never;
        /** Attach a project root (idempotent). 400 invalid_request when root is not an absolute existing directory. */
        post: operations["attachProject"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/projects/{projectId}": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                /** @description Id returned by POST /projects. */
                projectId: components["parameters"]["ProjectId"];
            };
            cookie?: never;
        };
        /** @description `git.remote` is the origin URL read from `.git/config` with credentials removed. It is read from the file system only (no git process, no network) and is untrusted text. */
        get: operations["getProject"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/providers": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: operations["listProviders"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/capabilities": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** @description Capabilities usable in workflow tool/system steps - the built-in vendor-neutral catalog plus every capability a provider offers, each with its canonical spec (inputs, output roles) and the providers offering it. */
        get: operations["listCapabilities"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/projects/{projectId}/executions": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                /** @description Id returned by POST /projects. */
                projectId: components["parameters"]["ProjectId"];
            };
            cookie?: never;
        };
        get: operations["listExecutions"];
        put?: never;
        /**
         * @description Resolves `capability` (and optional `providerId`) to a provider and starts the execution.
         *     `idempotencyKey` makes retries safe. `input` is `{prompt}` for agent.run or `{artifactId}` for asset.generate. Optional `workItemId` links the execution to a Work Item (not_found if unknown). Errors: invalid_request, provider_unavailable (no provider / not available), conflict.
         */
        post: operations["createExecution"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/projects/{projectId}/executions/{id}": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                /** @description Id returned by POST /projects. */
                projectId: components["parameters"]["ProjectId"];
            };
            cookie?: never;
        };
        get: operations["getExecution"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/projects/{projectId}/executions/{id}/cancel": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                /** @description Id returned by POST /projects. */
                projectId: components["parameters"]["ProjectId"];
            };
            cookie?: never;
        };
        get?: never;
        put?: never;
        /** @description Idempotent. Cancelling a terminal execution returns it unchanged. */
        post: operations["cancelExecution"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/projects/{projectId}/executions/{id}/events": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                /** @description Id returned by POST /projects. */
                projectId: components["parameters"]["ProjectId"];
            };
            cookie?: never;
        };
        /** @description Persisted events of one Execution, oldest first, up to `limit` (default 1000, max 5000) after the cursor `after` (a `seq`). Unlike the live stream this survives Platform restarts and buffer eviction. Page with `nextCursor`. */
        get: operations["listExecutionEvents"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/projects/{projectId}/events": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                /** @description Id returned by POST /projects. */
                projectId: components["parameters"]["ProjectId"];
            };
            cookie?: never;
        };
        /** @description WebSocket upgrade, one stream per Project. The first frame is `Hello`, then `Event` frames with strictly increasing `seq` (the per-Project cursor; every event carries `id`, `projectId`, `resource` and optional `correlationId`). `since=<seq>` replays retained events with seq > since. When the cursor cannot be served the server sends one `ResyncRequired` frame and the client reloads state through the HTTP queries, then resumes from `resumeFrom`. The token is sent as an `Authorization: Bearer` header or `?token=` (browsers cannot set WebSocket headers). */
        get: operations["events"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/projects/{projectId}/work-packages": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                /** @description Id returned by POST /projects. */
                projectId: components["parameters"]["ProjectId"];
            };
            cookie?: never;
        };
        get: operations["listWorkPackages"];
        put?: never;
        /** @description The Platform assigns the id (WP-001, WP-002, …). New packages start as `planned`. */
        post: operations["createWorkPackage"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/projects/{projectId}/work-packages/{id}": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                /** @description Id returned by POST /projects. */
                projectId: components["parameters"]["ProjectId"];
            };
            cookie?: never;
        };
        get: operations["getWorkPackage"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch: operations["updateWorkPackage"];
        trace?: never;
    };
    "/projects/{projectId}/work-items": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                /** @description Id returned by POST /projects. */
                projectId: components["parameters"]["ProjectId"];
            };
            cookie?: never;
        };
        get: operations["listWorkItems"];
        put?: never;
        /** @description The Platform assigns the id (WI-001, … global in the project). Errors - invalid_request, not_found (unknown package). */
        post: operations["createWorkItem"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/projects/{projectId}/work-items/{id}": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                /** @description Id returned by POST /projects. */
                projectId: components["parameters"]["ProjectId"];
            };
            cookie?: never;
        };
        get: operations["getWorkItem"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        /** @description Status is set by the user and never derived from executions. `blockedReason` is only kept while status is `blocked`. */
        patch: operations["updateWorkItem"];
        trace?: never;
    };
    "/projects/{projectId}/workflows": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                /** @description Id returned by POST /projects. */
                projectId: components["parameters"]["ProjectId"];
            };
            cookie?: never;
        };
        get: operations["listWorkflows"];
        put?: never;
        /** @description The Platform assigns the id (WF-001, …). Persisted under `.Amaru/workflows/`. */
        post: operations["createWorkflow"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/projects/{projectId}/workflows/{id}": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                /** @description Id returned by POST /projects. */
                projectId: components["parameters"]["ProjectId"];
            };
            cookie?: never;
        };
        get: operations["getWorkflow"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch: operations["updateWorkflow"];
        trace?: never;
    };
    "/projects/{projectId}/workflows/{id}/source": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                /** @description Id returned by POST /projects. */
                projectId: components["parameters"]["ProjectId"];
            };
            cookie?: never;
        };
        /** @description The workflow file exactly as stored (`.Amaru/workflows/<id>.yaml`). Read-only preview. */
        get: operations["getWorkflowSource"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/projects/{projectId}/workflows/{id}/run": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                /** @description Id returned by POST /projects. */
                projectId: components["parameters"]["ProjectId"];
            };
            cookie?: never;
        };
        get?: never;
        put?: never;
        /** @description Starts one Execution through the normal execution path, carrying a Step Execution per workflow step (`steps`). Executable steps run sequentially, one provider run each; the first failure stops later steps. Trigger steps are skipped and tool/system steps without an available capability provider are unsupported - never reported as successful. The Execution carries `workflowId`. `workItemId` is linked only when given, never inferred. Errors - not_found (workflow or work item), conflict (workflow not active, or a step's Agent is disabled), invalid_request (no executable steps, or a step's Agent does not exist), provider_unavailable. A step with an `agentId` runs on its Agent's resolved provider and records an Agent snapshot on its Step Execution; the run-level `providerId` applies only to agent steps without an `agentId`. */
        post: operations["runWorkflow"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/projects/{projectId}/agents": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                /** @description Id returned by POST /projects. */
                projectId: components["parameters"]["ProjectId"];
            };
            cookie?: never;
        };
        /** @description The project's Agent catalog, ordered by id. */
        get: operations["listAgents"];
        put?: never;
        /** @description Persisted at `.Amaru/agents/<id>.json`. Errors - invalid_request, conflict (id already exists). Retire an Agent in use with status `disabled`; an Agent no Workflow refers to can be deleted. */
        post: operations["createAgent"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/projects/{projectId}/agents/{id}": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                /** @description Id returned by POST /projects. */
                projectId: components["parameters"]["ProjectId"];
            };
            cookie?: never;
        };
        get: operations["getAgent"];
        put?: never;
        post?: never;
        /** @description Removes `.Amaru/agents/<id>.json`. Errors - not_found, conflict (a Workflow step still refers to the Agent; disable it instead). */
        delete: operations["deleteAgent"];
        options?: never;
        head?: never;
        patch: operations["updateAgent"];
        trace?: never;
    };
    "/projects/{projectId}/teams": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                /** @description Id returned by POST /projects. */
                projectId: components["parameters"]["ProjectId"];
            };
            cookie?: never;
        };
        /** @description The project's Teams, ordered by id. */
        get: operations["listTeams"];
        put?: never;
        /** @description Persisted at `.Amaru/teams/<id>.json`. Errors - invalid_request, conflict (id exists). */
        post: operations["createTeam"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/projects/{projectId}/teams/{id}": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                /** @description Id returned by POST /projects. */
                projectId: components["parameters"]["ProjectId"];
            };
            cookie?: never;
        };
        get: operations["getTeam"];
        put?: never;
        post?: never;
        /** @description Errors - not_found, conflict (a workflow step still uses the team). */
        delete: operations["deleteTeam"];
        options?: never;
        head?: never;
        patch: operations["updateTeam"];
        trace?: never;
    };
    "/projects/{projectId}/settings": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                /** @description Id returned by POST /projects. */
                projectId: components["parameters"]["ProjectId"];
            };
            cookie?: never;
        };
        /** @description The project's Amaru switches (`.Amaru/settings.json`); all off by default. */
        get: operations["getSettings"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch: operations["updateSettings"];
        trace?: never;
    };
    "/projects/{projectId}/artifacts": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                /** @description Id returned by POST /projects. */
                projectId: components["parameters"]["ProjectId"];
            };
            cookie?: never;
        };
        /** @description Markdown notes with a `type` frontmatter key, scanned read-only from the project. Filters combine with AND. */
        get: operations["listArtifacts"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/projects/{projectId}/artifacts/{id}": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                /** @description Id returned by POST /projects. */
                projectId: components["parameters"]["ProjectId"];
            };
            cookie?: never;
        };
        /** @description The id is the project-relative note path (slashes are not escaped), e.g. `/artifacts/Assets/Player.md`. */
        get: operations["getArtifact"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/projects/{projectId}/work-packages/{id}/delivery-loop": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                /** @description Id returned by POST /projects. */
                projectId: components["parameters"]["ProjectId"];
                id: components["schemas"]["WorkPackageId"];
            };
            cookie?: never;
        };
        /** @description The delivery loop ledger of a Work Package: per-Work-Item phase, attempts, runs, pull request and decisions. Platform state; Work Items and Executions stay authoritative. Replaces the prototype route `/work-packages/{id}/delivery loop`, which was registered by the server but missing from the contract. 404 `not_found` for an unknown package or one without a delivery loop. */
        get: operations["getDeliveryLoop"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/projects/{projectId}/approvals": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                /** @description Id returned by POST /projects. */
                projectId: components["parameters"]["ProjectId"];
            };
            cookie?: never;
        };
        /** @description Approvals of the Project, newest request first. Filter by `state` (default `requested`). Read-only; deciding stays on the decision route. */
        get: operations["listApprovals"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/projects/{projectId}/approvals/{approvalId}": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                /** @description Id returned by POST /projects. */
                projectId: components["parameters"]["ProjectId"];
                approvalId: components["schemas"]["Ulid"];
            };
            cookie?: never;
        };
        /** @description One Approval with the Evidence it binds. */
        get: operations["getApproval"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/projects/{projectId}/approvals/{approvalId}/decision": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                /** @description Id returned by POST /projects. */
                projectId: components["parameters"]["ProjectId"];
                approvalId: components["schemas"]["Ulid"];
            };
            cookie?: never;
        };
        get?: never;
        put?: never;
        /** @description A human principal approves or rejects a `requested` Approval. Only humans may decide (ADR-0036); the decider is the authenticated principal, never a body field. Errors: `not_found`; `forbidden` when the role, Team or separation-of-duties rule is not met (recorded as an Event, the Approval is unchanged); `conflict` when the Approval is not `requested` or `evidenceDigest` differs from its current digest. Use-time reuse and re-request rules are decided by ADR-0048 (accepted 2026-10-26). */
        post: operations["decideApproval"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
}
export type webhooks = Record<string, never>;
export interface components {
    schemas: {
        StreamFrame: components["schemas"]["Frame"];
        /**
         * ProtocolVersion
         * @constant
         */
        ProtocolVersion: 1;
        /**
         * Timestamp
         * Format: date-time
         */
        Timestamp: string;
        /** Health */
        Health: {
            /** @constant */
            status: "ok";
            platformVersion: string;
            protocolVersion: components["schemas"]["ProtocolVersion"];
            startedAt: components["schemas"]["Timestamp"];
            uptimeSec: number;
        };
        /**
         * ErrorCode
         * @enum {unknown}
         */
        ErrorCode: "invalid_request" | "unauthorized" | "forbidden_origin" | "forbidden" | "not_found" | "provider_unavailable" | "protocol_mismatch" | "conflict" | "rate_limited" | "needs_grant" | "internal";
        /** Error */
        Error: {
            error: {
                code: components["schemas"]["ErrorCode"];
                message: string;
                details?: Record<string, never>;
            };
        };
        /** Project */
        Project: {
            id: string;
            name: string;
            /** @description Absolute native path of the project root. Informational; identity is `id`. */
            root: string;
            /** @description External Git repositories linked in .amaru/project.yaml. An entry with `error` is never used. */
            repositories?: {
                id: string;
                path: string;
                remote?: string;
                error?: string;
            }[];
            config: {
                present: boolean;
                path?: string;
            };
            git: {
                present: boolean;
                branch?: string;
                /** @description URL of the origin remote read from .git/config, credentials removed. Untrusted project text; never contacted. */
                remote?: string;
            };
        };
        /**
         * ProjectList
         * @description Projects attached to this Platform (GET /projects), ordered by name.
         */
        ProjectList: components["schemas"]["Project"][];
        /**
         * AttachProject
         * @description POST /projects body. Idempotent: the same canonical root always yields the same project.
         */
        AttachProject: {
            /** @description Absolute native path of an existing directory (the vault root). */
            root: string;
        };
        /**
         * ProviderKind
         * @enum {unknown}
         */
        ProviderKind: "agent" | "tool" | "pipeline";
        /**
         * CapabilityId
         * @description Dotted identifier, e.g. agent.run. Data, not an enum: new capabilities are additive.
         */
        CapabilityId: string;
        /** ProviderStatus */
        ProviderStatus: {
            available: boolean;
            version?: string;
            /** @description Why unavailable, e.g. "executable not found". */
            reason?: string;
        };
        /** PortName */
        PortName: string;
        /** MediaType */
        MediaType: string;
        /**
         * PortSpec
         * @description One named input or output of a capability. Output ports are files; their name is the output role.
         */
        PortSpec: {
            name: components["schemas"]["PortName"];
            /** @enum {unknown} */
            kind: "text" | "number" | "bool" | "enum" | "file";
            mediaTypes?: components["schemas"]["MediaType"][];
            required?: boolean;
            enum?: string[];
            description?: string;
        };
        /**
         * OutputPortSpec
         * @description An output port: always a file; its name is the output role.
         */
        OutputPortSpec: components["schemas"]["PortSpec"] & {
            /** @constant */
            kind?: "file";
        };
        /**
         * CapabilitySpec
         * @description Vendor-neutral contract of a capability: named inputs and output roles.
         */
        CapabilitySpec: {
            id: components["schemas"]["CapabilityId"];
            title: string;
            description?: string;
            inputs: components["schemas"]["PortSpec"][];
            outputs: components["schemas"]["OutputPortSpec"][];
        };
        /**
         * Connection
         * @description User-level configuration of a provider that needs one. Secret values are never returned, only whether each is set.
         */
        Connection: {
            enabled: boolean;
            secrets: {
                [key: string]: boolean;
            };
        };
        /** Provider */
        Provider: {
            id: string;
            kind: components["schemas"]["ProviderKind"];
            name: string;
            capabilities: components["schemas"]["CapabilityId"][];
            status: components["schemas"]["ProviderStatus"];
            /** @description Capability specs the provider publishes (custom capabilities or extra optional inputs). Others use the built-in catalog. */
            specs?: components["schemas"]["CapabilitySpec"][];
            /** @description Secret names the provider needs (values are write-only). */
            secrets?: string[];
            /** @description Sends project data to a remote service; disabled until the user enables it. */
            network?: boolean;
            connection?: components["schemas"]["Connection"];
        };
        /** ProviderList */
        ProviderList: components["schemas"]["Provider"][];
        /** ProviderId */
        ProviderId: string;
        /** CapabilityOffer */
        CapabilityOffer: {
            providerId: components["schemas"]["ProviderId"];
            name: string;
            available: boolean;
            spec?: components["schemas"]["CapabilitySpec"];
        };
        /**
         * CapabilityInfo
         * @description A capability usable in workflow tool/system steps: canonical spec plus the providers offering it (GET /v1/capabilities).
         */
        CapabilityInfo: {
            id: components["schemas"]["CapabilityId"];
            title: string;
            description?: string;
            inputs: components["schemas"]["PortSpec"][];
            outputs: components["schemas"]["OutputPortSpec"][];
            builtin: boolean;
            providers: components["schemas"]["CapabilityOffer"][];
        };
        /** CapabilityList */
        CapabilityList: components["schemas"]["CapabilityInfo"][];
        /**
         * Checkout
         * @description A Platform-managed Git worktree; path is project-relative.
         */
        Checkout: {
            path: string;
            branch: string;
            base: string;
        };
        /**
         * ExecutionState
         * @description Execution state (DOMAIN_MODEL 5.1). `waiting`: a Step awaits an Approval or a Worker. Clients must tolerate new values.
         * @enum {unknown}
         */
        ExecutionState: "queued" | "running" | "waiting" | "succeeded" | "failed" | "cancelled";
        /** ExecutionError */
        ExecutionError: {
            code: string;
            message: string;
        };
        /** WorkItemId */
        WorkItemId: string;
        /** WorkflowId */
        WorkflowId: string;
        /**
         * StepType
         * @enum {unknown}
         */
        StepType: "trigger" | "agent" | "tool" | "system" | "review" | "approval";
        /**
         * StepState
         * @description Step Execution state (DOMAIN_MODEL 5.2, ADR-0042, ADR-0044). Prototype `unsupported` is `refused` with a reason. Clients must tolerate new values.
         * @enum {unknown}
         */
        StepState: "pending" | "skipped" | "refused" | "awaiting_approval" | "queued" | "leased" | "running" | "succeeded" | "failed" | "timed_out" | "interrupted" | "cancelled";
        /**
         * AgentId
         * @description Human-referenceable slug, e.g. godot-developer. Immutable.
         */
        AgentId: string;
        /**
         * AgentResolution
         * @description preference: a providerPreference entry; fallback: preference set but unmet, first available used; default: no preference, first available used.
         * @enum {unknown}
         */
        AgentResolution: "preference" | "fallback" | "default";
        /**
         * ResolvedAgent
         * @description Snapshot of the Agent a step ran with, recorded when the run is planned. A full copy: the run stays reconstructable after the Agent changes.
         */
        ResolvedAgent: {
            /**
             * @description strict: the provider ran with its own MCP servers, connectors and native skills shut out. ambient: it also saw whatever the user configured for that CLI. Absent on snapshots recorded before.
             * @enum {string}
             */
            toolIsolation?: "strict" | "ambient";
            worktreeWrite?: boolean;
            /** @description Shell command prefixes run in an OS sandbox in the worktree. Needs worktreeWrite. */
            worktreeCommands?: string[];
            id: components["schemas"]["AgentId"];
            name: string;
            role?: string;
            instructions: string;
            model?: string;
            providerPreference: components["schemas"]["ProviderId"][];
            providerId: components["schemas"]["ProviderId"];
            resolution: components["schemas"]["AgentResolution"];
        };
        /**
         * TeamRole
         * @description A role label inside a Team, unique per Team.
         */
        TeamRole: string;
        /** StepTeam */
        StepTeam: {
            teamId: components["schemas"]["AgentId"];
            role: components["schemas"]["TeamRole"];
        };
        /**
         * StepVerdict
         * @description A review step's outcome. `changes_requested` fails the step, which ends the run.
         */
        StepVerdict: {
            /** @enum {unknown} */
            outcome: "approved" | "changes_requested";
            notes?: string;
        };
        /**
         * ApprovalState
         * @description The human gate of an approval step. While `pending` the step stays `running` and the run waits for a person.
         */
        ApprovalState: {
            /** @enum {unknown} */
            status: "pending" | "approved" | "rejected";
            message: string;
            note?: string;
            requestedAt: components["schemas"]["Timestamp"];
            decidedAt?: components["schemas"]["Timestamp"];
        };
        /** Sha256Hex */
        Sha256Hex: string;
        /**
         * OutputFile
         * @description A file a capability step produced. sha256/bytes are computed by the Platform (empty sha256 only when no inspector is configured).
         */
        OutputFile: {
            /** @description Where asset storage holds a copy (asset.upload). */
            url?: string;
            role: components["schemas"]["PortName"];
            path: string;
            mediaType: string;
            sha256: components["schemas"]["Sha256Hex"] | "";
            bytes: number;
        };
        /**
         * UsageSource
         * @description Provenance of a usage number (ADR-0033): reported by the provider, estimated by Amaru, or unknown. Unknown is never stored as zero.
         * @enum {unknown}
         */
        UsageSource: "reported" | "estimated" | "unknown";
        /**
         * UsageReport
         * @description The measurements of one usage record, without Platform identity. This is what a Provider or Worker reports (ADR-0033). Every number is a measurement with a source; `value` is absent when the source is `unknown`.
         */
        UsageReport: {
            provider?: string;
            model?: string;
            inputTokens?: {
                value?: number;
                source: components["schemas"]["UsageSource"];
            } & (unknown & unknown);
            outputTokens?: {
                value?: number;
                source: components["schemas"]["UsageSource"];
            } & (unknown & unknown);
            cacheReadTokens?: {
                value?: number;
                source: components["schemas"]["UsageSource"];
            } & (unknown & unknown);
            cacheWriteTokens?: {
                value?: number;
                source: components["schemas"]["UsageSource"];
            } & (unknown & unknown);
            credits?: {
                value?: number;
                source: components["schemas"]["UsageSource"];
            } & (unknown & unknown);
            wallDurationMs?: {
                value?: number;
                source: components["schemas"]["UsageSource"];
            } & (unknown & unknown);
            cost?: {
                amount?: string;
                currency?: string;
                source: components["schemas"]["UsageSource"];
                /** @description Required when source is estimated. */
                priceTableVersion?: string;
            } & (unknown & unknown & unknown);
            environmentId?: string;
            costClass?: string;
        };
        /**
         * UsageRecord
         * @description One usage record of a Step Execution (OBS-002, ADR-0033). Every number is a measurement with a source; `value` is absent when the source is `unknown`. Replaces the prototype `Usage` shape. Cost is a decimal string with its currency, never a float.
         */
        UsageRecord: {
            id?: string;
            stepExecutionId: string;
            capturedAt: components["schemas"]["Timestamp"];
        } & components["schemas"]["UsageReport"];
        /**
         * ExternalRef
         * @description Provider-side job of a remote step.
         */
        ExternalRef: {
            providerId: components["schemas"]["ProviderId"];
            jobId: string;
            submittedAt: components["schemas"]["Timestamp"];
        };
        /**
         * StepExecution
         * @description Execution state of one workflow step inside a workflow run. The parent Execution stays the canonical overall run.
         */
        StepExecution: {
            stepId: string;
            name: string;
            type: components["schemas"]["StepType"];
            capability?: components["schemas"]["CapabilityId"];
            /** @description Decided when the run is planned: whether this step can run at all. Progress counts only executable steps. */
            executable: boolean;
            state: components["schemas"]["StepState"];
            reason?: string;
            startedAt?: components["schemas"]["Timestamp"];
            endedAt?: components["schemas"]["Timestamp"];
            error?: components["schemas"]["ExecutionError"];
            outputs?: string[];
            agent?: components["schemas"]["ResolvedAgent"];
            team?: components["schemas"]["StepTeam"];
            /** @description The agent's final text for an agent or review step (capped); what a review step reads. */
            answer?: string;
            /** @description The step a review or approval step looks at. */
            reviews?: string;
            verdict?: components["schemas"]["StepVerdict"];
            approval?: components["schemas"]["ApprovalState"];
            providerId?: components["schemas"]["ProviderId"];
            /** @description Capability input the step ran with (variables and data edges substituted). Provenance; never credentials. */
            input?: {
                [key: string]: string;
            };
            files?: components["schemas"]["OutputFile"][];
            /** @description Usage records of this Step Execution (ADR-0033); empty or absent when the provider reported nothing. */
            usage?: components["schemas"]["UsageRecord"][];
            external?: components["schemas"]["ExternalRef"];
        } & (unknown & unknown & unknown);
        /** Execution */
        Execution: {
            checkout?: components["schemas"]["Checkout"];
            id: string;
            projectId: string;
            capability: components["schemas"]["CapabilityId"];
            providerId: string;
            /** @description What was requested. Present on executions created by this Platform; user-authored text, never credentials. */
            input?: {
                prompt?: string;
                /** @description Artifact id (project-relative note path) for asset.generate. */
                artifactId?: string;
            } & (unknown | unknown);
            state: components["schemas"]["ExecutionState"];
            createdAt: components["schemas"]["Timestamp"];
            startedAt?: components["schemas"]["Timestamp"];
            endedAt?: components["schemas"]["Timestamp"];
            error?: components["schemas"]["ExecutionError"];
            /** @description Work Item this execution was started for. Absent for executions not tied to a Work Item. Not a status: a Work Item's status never mirrors its executions. */
            workItemId?: components["schemas"]["WorkItemId"];
            /** @description Workflow this execution was started from (POST /workflows/{id}/run). Absent otherwise. */
            workflowId?: components["schemas"]["WorkflowId"];
            /** @description Project-relative files the execution produced (asset.generate; workflow runs: the union of step outputs). Absent otherwise. */
            outputs?: string[];
            /** @description Ordered Step Executions of a workflow run. Absent for executions not started from a workflow. */
            steps?: components["schemas"]["StepExecution"][];
        } & (unknown & unknown & unknown);
        /** ExecutionList */
        ExecutionList: components["schemas"]["Execution"][];
        /** CreateExecution */
        CreateExecution: {
            capability: components["schemas"]["CapabilityId"];
            providerId?: string;
            input: {
                prompt?: string;
                /** @description Artifact id (project-relative note path) for asset.generate. */
                artifactId?: string;
            } & (unknown | unknown);
            idempotencyKey?: string;
            workItemId?: components["schemas"]["WorkItemId"];
        };
        /**
         * Ulid
         * @description Crockford base32 ULID (POLICY_AND_APPROVAL_SHAPES conventions).
         */
        Ulid: string;
        /**
         * EventBase
         * @description Common fields of every persisted event (WRK-003). `seq` is the per-Project cursor: strictly increasing, gap-free per Project, and the value a client sends to resume. `id` is stable across redelivery (clients deduplicate on it). `resource` names what the event is about; `correlationId` links events caused by the same request or Execution.
         */
        EventBase: {
            id: components["schemas"]["Ulid"];
            seq: number;
            type: string;
            time: components["schemas"]["Timestamp"];
            projectId: string;
            executionId?: string;
            resource: {
                type: string;
                id: string;
            };
            correlationId?: string;
            data: Record<string, never>;
        };
        /**
         * ExecutionLifecycle
         * @description created / started / waiting / completed / cancelled. Terminal events are final.
         */
        ExecutionLifecycle: components["schemas"]["EventBase"] & {
            /** @enum {unknown} */
            type?: "execution.created" | "execution.started" | "execution.waiting" | "execution.completed" | "execution.cancelled";
            data?: {
                /** @enum {unknown} */
                state: "queued" | "running" | "waiting" | "succeeded" | "cancelled";
            };
        };
        /** ExecutionFailed */
        ExecutionFailed: components["schemas"]["EventBase"] & {
            /** @constant */
            type?: "execution.failed";
            data?: {
                /** @constant */
                state: "failed";
                error: {
                    code: string;
                    message: string;
                };
            };
        };
        /**
         * ExecutionOutput
         * @description Provider-neutral output chunk. `stream` is a role, not a pipe: message = agent prose, log = diagnostics.
         */
        ExecutionOutput: components["schemas"]["EventBase"] & {
            /** @constant */
            type?: "execution.output";
            data?: {
                /** @enum {unknown} */
                stream: "message" | "log";
                text: string;
                /** @description Workflow step this output belongs to; absent outside workflow runs. */
                stepId?: string;
            };
        };
        /** ExecutionProgress */
        ExecutionProgress: components["schemas"]["EventBase"] & {
            /** @constant */
            type?: "execution.progress";
            data?: {
                message: string;
                percent?: number;
            };
        };
        /**
         * ExecutionStep
         * @description A Step Execution of a workflow run changed state. `failed` carries `error`; `skipped`/`refused` carry `reason`; `succeeded` may carry `outputs`.
         */
        ExecutionStep: components["schemas"]["EventBase"] & {
            /** @constant */
            type?: "execution.step";
            data?: {
                stepId: string;
                /** @enum {unknown} */
                state: "pending" | "skipped" | "refused" | "awaiting_approval" | "queued" | "leased" | "running" | "succeeded" | "failed" | "timed_out" | "interrupted" | "cancelled";
                reason?: string;
                error?: {
                    code: string;
                    message: string;
                };
                outputs?: string[];
                providerId?: string;
                files?: components["schemas"]["OutputFile"][];
                usage?: components["schemas"]["UsageReport"][];
                external?: components["schemas"]["ExternalRef"];
                verdict?: components["schemas"]["StepVerdict"];
                approval?: components["schemas"]["ApprovalState"];
            } & (unknown & unknown);
        };
        /**
         * ArtifactGenerated
         * @description Files produced by an asset.generate execution, published just before it completes.
         */
        ArtifactGenerated: components["schemas"]["EventBase"] & {
            /** @constant */
            type?: "artifact.generated";
            data?: {
                paths: string[];
            };
        };
        /** PlatformState */
        PlatformState: components["schemas"]["EventBase"] & {
            /** @constant */
            type?: "platform.state";
            data?: {
                /** @enum {unknown} */
                state: "ready" | "shutting_down";
            };
        };
        /**
         * CatalogChanged
         * @description Agent or Team files were edited, added or removed outside the Platform; `ids` are the ones that changed. Clients refetch the list.
         */
        CatalogChanged: components["schemas"]["EventBase"] & {
            /** @enum {unknown} */
            type?: "agent.changed" | "team.changed";
            data?: {
                ids: string[];
            };
        };
        /**
         * WorkChanged
         * @description A Work Package or Work Item was created or updated.
         */
        WorkChanged: components["schemas"]["EventBase"] & {
            /** @enum {unknown} */
            type?: "work.package.changed" | "work.item.changed";
            data?: {
                id: string;
                packageId?: string;
                status: string;
            };
        };
        /**
         * DeliveryLoopDecisionEvent
         * @description The delivery loop recorded a decision.
         */
        DeliveryLoopDecisionEvent: components["schemas"]["EventBase"] & {
            /** @constant */
            type?: "delivery_loop.decision";
            data?: {
                packageId: string;
                decision: Record<string, never>;
            };
        };
        /**
         * CheckoutEvent
         * @description A Platform worktree was prepared or removed.
         */
        CheckoutEvent: components["schemas"]["EventBase"] & {
            /** @enum {unknown} */
            type?: "checkout.prepared" | "checkout.removed";
            data?: {
                workItemId: string;
                checkout: Record<string, never>;
            };
        };
        /**
         * PullRequestEvent
         * @description A normalized pull request event from the Git host. Never the raw payload.
         */
        PullRequestEvent: components["schemas"]["EventBase"] & {
            /** @enum {unknown} */
            type?: "pr.opened" | "pr.closed" | "pr.merged" | "pr.reviewed" | "pr.commented" | "pr.updated";
            data?: {
                repo: string;
                number: number;
                action: string;
                branch?: string;
                url?: string;
                merged?: boolean;
                reviewState?: string;
                /** @enum {unknown} */
                source: "webhook" | "resync" | "reconcile";
                deliveryId?: string;
            };
        };
        /**
         * CheckEvent
         * @description A check suite, check run or workflow run finished on the Git host (or the startup resync re-read a pull request awaiting CI). Never the raw payload.
         */
        CheckEvent: components["schemas"]["EventBase"] & {
            /** @constant */
            type?: "pr.checks";
            data?: {
                repo: string;
                sha: string;
                branch?: string;
                /** @enum {unknown} */
                source: "webhook" | "resync" | "reconcile";
                deliveryId?: string;
            };
        };
        /**
         * IssueEvent
         * @description A normalized issue event from the Git host; issue intake re-syncs the repository. Never the raw payload.
         */
        IssueEvent: components["schemas"]["EventBase"] & {
            /** @constant */
            type?: "issue.changed";
            data?: {
                repo: string;
                number: number;
                /** @enum {unknown} */
                action: "opened" | "edited" | "closed" | "reopened" | "labeled" | "unlabeled";
                /** @enum {unknown} */
                source: "webhook";
                deliveryId?: string;
            };
        };
        /**
         * ReconcileCompleted
         * @description One periodic (or startup) reconcile pass against the Git host finished. error is at most 200 characters and never a secret.
         */
        ReconcileCompleted: components["schemas"]["EventBase"] & {
            /** @constant */
            type?: "reconcile.completed";
            data?: {
                /** @enum {unknown} */
                scope: "delivery_loop";
                ok: boolean;
                partial: boolean;
                created: number;
                updated: number;
                unchanged: number;
                errors: number;
                error?: string;
                durationMs: number;
            };
        };
        /**
         * Digest
         * @description `sha256:` followed by 64 lowercase hex characters.
         */
        Digest: string;
        /**
         * ApprovalEvent
         * @description Approval state change: previous and new state plus the evidence digest (POLICY_AND_APPROVAL_SHAPES 3.3). No rationale text and no secrets.
         */
        ApprovalEvent: components["schemas"]["EventBase"] & {
            /** @enum {unknown} */
            type?: "approval.requested" | "approval.approved" | "approval.rejected" | "approval.expired" | "approval.voided";
            data?: {
                approvalId: components["schemas"]["Ulid"];
                /** @enum {unknown} */
                previousState?: "requested" | "approved" | "rejected" | "expired" | "voided";
                /** @enum {unknown} */
                state: "requested" | "approved" | "rejected" | "expired" | "voided";
                evidenceDigest: components["schemas"]["Digest"];
                replaces?: components["schemas"]["Ulid"];
            };
        };
        /**
         * PolicyEvaluated
         * @description Every Policy evaluation is recorded (ADR-0037): the input hash, the decision and the reason codes.
         */
        PolicyEvaluated: components["schemas"]["EventBase"] & {
            /** @constant */
            type?: "policy.evaluated";
            data?: {
                requestId: components["schemas"]["Ulid"];
                /** @enum {unknown} */
                decision: "allow" | "require_approval" | "deny";
                inputHash: components["schemas"]["Digest"];
                reasonCodes: string[];
            };
        };
        /**
         * GrantChanged
         * @description A Grant was created, lapsed or revoked (ADR-0037).
         */
        GrantChanged: components["schemas"]["EventBase"] & {
            /** @enum {unknown} */
            type?: "grant.created" | "grant.lapsed" | "grant.revoked";
            data?: {
                grantId: components["schemas"]["Ulid"];
                /** @enum {unknown} */
                status: "active" | "lapsed" | "revoked" | "expired";
            };
        };
        /**
         * UsageRecorded
         * @description A usage record was captured for a Step Execution (OBS-002, ADR-0033). The numbers are on the UsageRecord; the event carries only the reference.
         */
        UsageRecorded: components["schemas"]["EventBase"] & {
            /** @constant */
            type?: "usage.recorded";
            data?: {
                stepExecutionId: string;
                usageRecordId: string;
            };
        };
        /**
         * ResultRejectedStale
         * @description A Worker result carried a fencing token below the Step's current one and was rejected (ADR-0044 point 4).
         */
        ResultRejectedStale: components["schemas"]["EventBase"] & {
            /** @constant */
            type?: "result.rejected_stale";
            data?: {
                attemptId: string;
                presentedToken: number;
                currentToken: number;
            };
        };
        /**
         * Event
         * @description A seq-numbered event frame. Clients must ignore unknown event types and unknown fields (additive evolution within v1, ADR-0025).
         */
        Event: components["schemas"]["ExecutionLifecycle"] | components["schemas"]["ExecutionFailed"] | components["schemas"]["ExecutionOutput"] | components["schemas"]["ExecutionProgress"] | components["schemas"]["ExecutionStep"] | components["schemas"]["ArtifactGenerated"] | components["schemas"]["PlatformState"] | components["schemas"]["CatalogChanged"] | components["schemas"]["WorkChanged"] | components["schemas"]["DeliveryLoopDecisionEvent"] | components["schemas"]["CheckoutEvent"] | components["schemas"]["PullRequestEvent"] | components["schemas"]["CheckEvent"] | components["schemas"]["IssueEvent"] | components["schemas"]["ReconcileCompleted"] | components["schemas"]["ApprovalEvent"] | components["schemas"]["PolicyEvaluated"] | components["schemas"]["GrantChanged"] | components["schemas"]["UsageRecorded"] | components["schemas"]["ResultRejectedStale"];
        /**
         * EventList
         * @description Persisted events of one Project or Execution, oldest first. `nextCursor` is the `seq` to resume from.
         */
        EventList: {
            events: components["schemas"]["Event"][];
            nextCursor?: number;
        };
        /** LocalCheck */
        LocalCheck: {
            name: string;
            /** @description argv. command[0] is a bare executable name found on the Platform's PATH; no shell, no expansion. */
            command: string[];
            /** @description Worktree-relative directory; default the worktree root. */
            dir?: string;
            /** @description 0 = 600. */
            timeoutSec?: number;
        };
        /**
         * LocalChecksPolicy
         * @description Validation commands the Platform runs in the item's worktree against the committed head before the review (Cycle 6). Local results are never recorded as hosted CI. The command lines come only from this policy.
         */
        LocalChecksPolicy: {
            checks: components["schemas"]["LocalCheck"][];
            /** @description Starts allowed per commit and configuration, restarts included. 0 = 3. */
            maxRuns?: number;
        };
        /**
         * DeliveryLoopPolicy
         * @description Opts a Work Package into autonomous execution.
         */
        DeliveryLoopPolicy: {
            enabled: boolean;
            maxAttempts?: number;
            reviewWorkflowId?: components["schemas"]["WorkflowId"];
            /** @description Project-relative Git repository directory; empty or . = the project root. */
            repo?: string;
            /** @description Wait for the host's CI checks before the review; repair recoverable failures (bounded by maxRepairs). */
            requireChecks?: boolean;
            maxRepairs?: number;
            /** @description Open the pull request ready for review instead of as a draft (default). */
            readyForReview?: boolean;
            /** @description Id of a repository linked in .amaru/project.yaml; exclusive with repo. Used only for a trusted project. */
            repository?: string;
            base?: string;
            /** @description Merge a moved base branch into an open pull request waiting for checks or a merge, and push the merge commit. Default false. */
            refreshBase?: boolean;
            /** @description Patterns of new files that look like agent scratch but are deliverables; a trailing / covers a directory. Set by a person. Without a match, such files stay out of the commit and are reported. */
            commitInclude?: string[];
            localChecks?: components["schemas"]["LocalChecksPolicy"];
        };
        /** WorkPackageId */
        WorkPackageId: string;
        /**
         * WorkPackageStatus
         * @enum {unknown}
         */
        WorkPackageStatus: "planned" | "active" | "completed" | "cancelled";
        /** WorkPackage */
        WorkPackage: {
            deliveryLoop?: components["schemas"]["DeliveryLoopPolicy"];
            id: components["schemas"]["WorkPackageId"];
            title: string;
            summary: string;
            status: components["schemas"]["WorkPackageStatus"];
            createdAt: components["schemas"]["Timestamp"];
            updatedAt: components["schemas"]["Timestamp"];
        };
        /** WorkPackageList */
        WorkPackageList: components["schemas"]["WorkPackage"][];
        /** CreateWorkPackage */
        CreateWorkPackage: {
            title: string;
            summary?: string;
        };
        /** UpdateWorkPackage */
        UpdateWorkPackage: {
            deliveryLoop?: components["schemas"]["DeliveryLoopPolicy"];
            title?: string;
            summary?: string;
            status?: components["schemas"]["WorkPackageStatus"];
        };
        /**
         * IssueDep
         * @description One issue another issue is blocked by. Only a blocker closed as completed is resolved.
         */
        IssueDep: {
            repo: string;
            number: number;
            /** @enum {unknown} */
            state: "open" | "closed";
            /** @description completed | not_planned | duplicate | reopened */
            stateReason?: string;
        };
        /**
         * IssueRef
         * @description The issue a Work Item was imported from (WorkItem.external); (repo, number) identifies it.
         */
        IssueRef: {
            /** @constant */
            provider: "github";
            repo: string;
            number: number;
            url?: string;
            updatedAt: components["schemas"]["Timestamp"];
            /** @enum {unknown} */
            state: "open" | "closed";
            labels?: string[];
            /** @description GitHub issue dependencies: the issues this one is blocked by. */
            blockedBy?: components["schemas"]["IssueDep"][];
        };
        /**
         * WorkItemStatus
         * @enum {unknown}
         */
        WorkItemStatus: "todo" | "in_progress" | "blocked" | "done";
        /** WorkOwner */
        WorkOwner: {
            /** @enum {unknown} */
            kind: "human" | "agent";
            label?: string;
        };
        /** WorkItem */
        WorkItem: {
            dependsOn?: components["schemas"]["WorkItemId"][];
            workflowId?: components["schemas"]["WorkflowId"];
            external?: components["schemas"]["IssueRef"];
            id: components["schemas"]["WorkItemId"];
            packageId: components["schemas"]["WorkPackageId"];
            title: string;
            intent: string;
            status: components["schemas"]["WorkItemStatus"];
            blockedReason?: string;
            owner: components["schemas"]["WorkOwner"];
            createdAt: components["schemas"]["Timestamp"];
            updatedAt: components["schemas"]["Timestamp"];
        } & unknown;
        /** WorkItemList */
        WorkItemList: components["schemas"]["WorkItem"][];
        /** CreateWorkItem */
        CreateWorkItem: {
            dependsOn?: components["schemas"]["WorkItemId"][];
            workflowId?: components["schemas"]["WorkflowId"] | "";
            packageId: components["schemas"]["WorkPackageId"];
            title: string;
            intent?: string;
            owner?: components["schemas"]["WorkOwner"];
        };
        /** UpdateWorkItem */
        UpdateWorkItem: {
            dependsOn?: components["schemas"]["WorkItemId"][];
            workflowId?: components["schemas"]["WorkflowId"] | "";
            title?: string;
            intent?: string;
            status?: components["schemas"]["WorkItemStatus"];
            blockedReason?: string;
            owner?: components["schemas"]["WorkOwner"];
        };
        /**
         * WorkflowStatus
         * @enum {unknown}
         */
        WorkflowStatus: "draft" | "active" | "archived";
        /**
         * WorkflowType
         * @enum {unknown}
         */
        WorkflowType: "pipeline" | "task";
        /**
         * WorkflowStep
         * @description WHAT happens. `tool` is a display label for the intended agent/tool; providers decide HOW. Agent steps run; tool/system steps run only when they declare a `capability` a provider offers; trigger steps are control steps. `id` is optional on input; the Platform assigns step-N.
         */
        WorkflowStep: {
            id?: string;
            name: string;
            type: components["schemas"]["StepType"];
            tool?: string;
            prompt?: string;
            /** @description Semantic operation for a tool/system step (e.g. model.validate). Names WHAT, never a provider or MCP server. */
            capability?: components["schemas"]["CapabilityId"];
            /** @description Agent step only: the project Agent that runs this step. `prompt` is then the task; persona, provider and model come from the Agent. Existence and status are checked when a run starts. */
            agentId?: components["schemas"]["AgentId"];
            /** @description Agent and review steps: select the Agent through a Team role instead of agentId. On a review step they name the reviewer. */
            teamId?: components["schemas"]["AgentId"];
            role?: components["schemas"]["TeamRole"];
            /** @description Review and approval steps: the step whose output is reviewed. Required on a review step. */
            reviews?: string;
            /** @description Explicit named input for the capability (e.g. {"file": "models/hero.blend"}); values may use {{variable}}. Providers validate it. */
            input?: {
                [key: string]: string;
            };
        } & (unknown & unknown & unknown & unknown & unknown & unknown);
        /** WorkflowVariable */
        WorkflowVariable: {
            name: string;
            value: string;
        };
        /**
         * WorkflowEdge
         * @description `from` runs before `to`. With fromPort+toPort it is a data edge: input toPort of `to` receives the path of `from`'s output with role fromPort.
         */
        WorkflowEdge: {
            from: string;
            fromPort?: components["schemas"]["PortName"];
            to: string;
            toPort?: components["schemas"]["PortName"];
        };
        /**
         * WorkflowLayout
         * @description Editor node positions by step id. UI data only.
         */
        WorkflowLayout: {
            nodes: {
                [key: string]: {
                    x: number;
                    y: number;
                };
            };
        };
        /** Workflow */
        Workflow: {
            id: components["schemas"]["WorkflowId"];
            name: string;
            description: string;
            status: components["schemas"]["WorkflowStatus"];
            type: components["schemas"]["WorkflowType"];
            steps: (components["schemas"]["WorkflowStep"] & Record<string, never>)[];
            variables: components["schemas"]["WorkflowVariable"][];
            createdAt: components["schemas"]["Timestamp"];
            updatedAt: components["schemas"]["Timestamp"];
            edges?: components["schemas"]["WorkflowEdge"][];
            layout?: components["schemas"]["WorkflowLayout"];
        };
        /** WorkflowList */
        WorkflowList: components["schemas"]["Workflow"][];
        /**
         * CreateWorkflow
         * @description The Platform assigns the id (WF-001, …). Defaults: status active, type pipeline.
         */
        CreateWorkflow: {
            name: string;
            description?: string;
            status?: components["schemas"]["WorkflowStatus"];
            type?: components["schemas"]["WorkflowType"];
            steps?: components["schemas"]["WorkflowStep"][];
            variables?: components["schemas"]["WorkflowVariable"][];
            edges?: components["schemas"]["WorkflowEdge"][];
            layout?: components["schemas"]["WorkflowLayout"];
        };
        /**
         * UpdateWorkflow
         * @description Fields present replace the stored value; `steps`/`variables` replace the whole list.
         */
        UpdateWorkflow: {
            name?: string;
            description?: string;
            status?: components["schemas"]["WorkflowStatus"];
            type?: components["schemas"]["WorkflowType"];
            steps?: components["schemas"]["WorkflowStep"][];
            variables?: components["schemas"]["WorkflowVariable"][];
            edges?: components["schemas"]["WorkflowEdge"][];
            layout?: components["schemas"]["WorkflowLayout"];
        };
        /** RunWorkflow */
        RunWorkflow: {
            providerId?: string;
            workItemId?: components["schemas"]["WorkItemId"];
            idempotencyKey?: string;
        };
        /**
         * AgentModel
         * @description Opaque to the Platform; the provider decides whether it is valid.
         */
        AgentModel: string;
        /**
         * AgentStatus
         * @enum {unknown}
         */
        AgentStatus: "active" | "disabled";
        /**
         * Agent
         * @description A reusable participant definition, stored at `.amaru/agents/<id>.json`. `skills` are Skill names, each resolved to a trusted Skill when a run starts; `requestedCapabilities` is a declaration, never enforced.
         */
        Agent: {
            worktreeWrite?: boolean;
            /** @description Shell command prefixes run in an OS sandbox in the worktree. Needs worktreeWrite. */
            worktreeCommands?: string[];
            id: components["schemas"]["AgentId"];
            name: string;
            role?: string;
            instructions: string;
            requestedCapabilities: components["schemas"]["CapabilityId"][];
            providerPreference: components["schemas"]["ProviderId"][];
            /** @description Model per agent provider id; wins over `model` for that provider. */
            models?: {
                [key: string]: components["schemas"]["AgentModel"];
            };
            model?: components["schemas"]["AgentModel"];
            status: components["schemas"]["AgentStatus"];
            createdAt: components["schemas"]["Timestamp"];
            updatedAt: components["schemas"]["Timestamp"];
        };
        /** AgentList */
        AgentList: components["schemas"]["Agent"][];
        /**
         * CreateAgent
         * @description `id` is optional: derived from `name` when absent. A duplicate id is a conflict.
         */
        CreateAgent: {
            worktreeWrite?: boolean;
            /** @description Shell command prefixes run in an OS sandbox in the worktree. Needs worktreeWrite. */
            worktreeCommands?: string[];
            id?: components["schemas"]["AgentId"];
            name: string;
            role?: string;
            instructions: string;
            requestedCapabilities?: components["schemas"]["CapabilityId"][];
            providerPreference?: components["schemas"]["ProviderId"][];
            /** @description Model per agent provider id; wins over `model` for that provider. */
            models?: {
                [key: string]: components["schemas"]["AgentModel"];
            };
            model?: components["schemas"]["AgentModel"];
            status?: components["schemas"]["AgentStatus"];
        };
        /**
         * UpdateAgent
         * @description Fields present replace the stored value; lists replace the whole list. The id is immutable (not accepted).
         */
        UpdateAgent: {
            worktreeWrite?: boolean;
            /** @description Shell command prefixes run in an OS sandbox in the worktree. Needs worktreeWrite. */
            worktreeCommands?: string[];
            name?: string;
            role?: string;
            instructions?: string;
            requestedCapabilities?: components["schemas"]["CapabilityId"][];
            providerPreference?: components["schemas"]["ProviderId"][];
            /** @description Model per agent provider id; wins over `model` for that provider. */
            models?: {
                [key: string]: components["schemas"]["AgentModel"];
            };
            /** @description Empty string clears the model. */
            model?: string;
            status?: components["schemas"]["AgentStatus"];
        };
        /** TeamMember */
        TeamMember: {
            agentId: components["schemas"]["AgentId"];
            role: components["schemas"]["TeamRole"];
        };
        /**
         * Team
         * @description A named set of Agents with roles, stored at `.amaru/teams/<id>.json`. Runs nothing by itself.
         */
        Team: {
            id: components["schemas"]["AgentId"];
            name: string;
            description?: string;
            members: components["schemas"]["TeamMember"][];
            /** @description When true the Platform refuses a plan in which a review step uses the Agent that produced the reviewed work. */
            separationOfDuties: boolean;
            createdAt: components["schemas"]["Timestamp"];
            updatedAt: components["schemas"]["Timestamp"];
        };
        /** TeamList */
        TeamList: components["schemas"]["Team"][];
        /** CreateTeam */
        CreateTeam: {
            id?: components["schemas"]["AgentId"];
            name: string;
            description?: string;
            members?: components["schemas"]["TeamMember"][];
            separationOfDuties?: boolean;
        };
        /** UpdateTeam */
        UpdateTeam: {
            name?: string;
            description?: string;
            members?: components["schemas"]["TeamMember"][];
            separationOfDuties?: boolean;
        };
        /**
         * ProjectSettings
         * @description The project's Amaru switches (.amaru/settings.json). Every switch is off by default and only widens what Amaru reads.
         */
        ProjectSettings: {
            /** @description List .claude/skills/*\/SKILL.md as read-only Skills, untrusted until the user trusts their content. */
            importClaudeSkills: boolean;
        };
        /** UpdateSettings */
        UpdateSettings: {
            importClaudeSkills?: boolean;
        };
        /**
         * ArtifactId
         * @description Project-relative slash path of the note, e.g. Assets/Player.md.
         */
        ArtifactId: string;
        /**
         * Artifact
         * @description A user-owned Markdown note with frontmatter, indexed read-only by the Platform.
         */
        Artifact: {
            id: components["schemas"]["ArtifactId"];
            path: string;
            title: string;
            type: string;
            status?: string;
            tags: string[];
            engines: string[];
            format?: string;
            files: string[];
            workItems: components["schemas"]["WorkItemId"][];
            modifiedAt: components["schemas"]["Timestamp"];
        };
        /** ArtifactList */
        ArtifactList: components["schemas"]["Artifact"][];
        /** DeliveryLoopRun */
        DeliveryLoopRun: {
            /** @enum {unknown} */
            kind: "implement" | "review";
            attempt: number;
            executionId: string;
            state: components["schemas"]["ExecutionState"];
            error?: components["schemas"]["ExecutionError"];
            /** @description The item was blocked while this run was live; the run is cancelled and its result ignored. */
            abandoned?: boolean;
        };
        /** PullRequest */
        PullRequest: {
            number: number;
            url: string;
            branch: string;
            /** @enum {unknown} */
            state: "open" | "merged" | "closed";
            commit?: string;
            draft?: boolean;
        };
        /**
         * CIState
         * @description What the delivery loop knows about the CI checks of an item's pull request (requireChecks).
         */
        CIState: {
            headSha: string;
            /** @enum {unknown} */
            state: "pending" | "success" | "failure";
            repairs: number;
            handled?: string[];
            lastFailure?: string;
            /**
             * @description The classification behind lastFailure; ci_unavailable means the host never ran the job (quota, billing), so no repair or rerun was spent.
             * @enum {unknown}
             */
            lastFailureClass?: "recoverable" | "pre_existing" | "infra" | "ambiguous" | "ci_unavailable";
            /** @description Earlier heads the delivery loop waited on; their checks never validate the current head. */
            superseded?: string[];
        };
        /**
         * BaseState
         * @description The last outcome of bringing an item's branch up to date with the base branch.
         */
        BaseState: {
            sha: string;
            /** @enum {unknown} */
            state: "current" | "refreshed" | "conflict" | "diverged" | "error";
            detail?: string;
            conflicts?: string[];
            at: components["schemas"]["Timestamp"];
        };
        /** LocalCheckResult */
        LocalCheckResult: {
            name: string;
            command: string[];
            /** @enum {unknown} */
            outcome: "passed" | "failed" | "unavailable" | "timeout" | "invalid";
            exitCode: number;
            startedAt: components["schemas"]["Timestamp"];
            endedAt: components["schemas"]["Timestamp"];
            durationMs: number;
            /** @description Head and tail of the output, bounded; truncated says the middle was cut. */
            output?: string;
            truncated?: boolean;
            detail?: string;
        };
        /**
         * LocalCheckState
         * @description What the delivery loop knows about the local checks of an item's pull request head. Source is always local: this is not hosted CI.
         */
        LocalCheckState: {
            /** @constant */
            source: "local";
            headSha: string;
            configHash: string;
            /** @enum {unknown} */
            state: "running" | "interrupted" | "cancelled" | "passed" | "failed" | "unavailable" | "timeout" | "invalid";
            runs: number;
            repairs: number;
            results?: components["schemas"]["LocalCheckResult"][];
            handled?: string[];
            runKey?: string;
        };
        /** ExcludedFile */
        ExcludedFile: {
            path: string;
            reason: string;
        };
        /** DeliveryLoopItem */
        DeliveryLoopItem: {
            /** @enum {unknown} */
            phase: "waiting" | "implementing" | "validating" | "reviewing" | "awaiting_ci" | "awaiting_merge" | "done" | "blocked" | "needs_grant" | "paused";
            attempt: number;
            runs?: components["schemas"]["DeliveryLoopRun"][];
            checkout?: components["schemas"]["Checkout"];
            pr?: components["schemas"]["PullRequest"];
            feedback?: string;
            /**
             * @description resume: an unblocked item is checking its delivered pull request before anything is redone.
             * @enum {unknown}
             */
            parkedFor?: "implement" | "review" | "preflight" | "resume" | "";
            /** @description Attempts granted when a person moved a blocked item back. */
            extraAttempts?: number;
            ci?: components["schemas"]["CIState"];
            /** @description When a paused item (provider quota or session limit) tries again. */
            resumeAt?: components["schemas"]["Timestamp"];
            /**
             * @description The phase the item was blocked in; unblocking resumes from it when the delivered work is still usable.
             * @enum {unknown}
             */
            blockedIn?: "waiting" | "implementing" | "validating" | "reviewing" | "awaiting_ci" | "awaiting_merge" | "needs_grant" | "paused";
            /** @description The pull request head an approved review covered. */
            reviewedSha?: string;
            base?: components["schemas"]["BaseState"];
            local?: components["schemas"]["LocalCheckState"];
            /** @description New files the last commit left out as agent scratch; they stay in the worktree. */
            excluded?: components["schemas"]["ExcludedFile"][];
            /** @description Scratch-looking files committed because the policy's commitInclude names them. */
            allowed?: string[];
            /** @description Runs of the current attempt a Platform restart interrupted; they spend no attempt, bounded at 3. */
            restartRecoveries?: number;
            /** @description Index into runs from which review runs count toward the review budget; a person's unblock moves it. */
            reviewFloor?: number;
        };
        /**
         * DeliveryLoopDecision
         * @description One thing the delivery loop did and the event that caused it.
         */
        DeliveryLoopDecision: {
            at: components["schemas"]["Timestamp"];
            triggerSeq?: number;
            triggerType: string;
            /** @enum {unknown} */
            source?: "event" | "webhook" | "startup" | "resync" | "reconcile" | "grant" | "provider_pause" | "local_check";
            workItemId?: components["schemas"]["WorkItemId"];
            action: string;
            reason: string;
            executionId?: string;
        };
        /**
         * DeliveryLoopLedger
         * @description GET /v1/projects/{projectId}/work-packages/{id}/delivery-loop. Platform state; Work Items and Executions stay authoritative.
         */
        DeliveryLoopLedger: {
            packageId: components["schemas"]["WorkPackageId"];
            items: {
                [key: string]: components["schemas"]["DeliveryLoopItem"];
            };
            decisions: components["schemas"]["DeliveryLoopDecision"][];
            completedAt?: components["schemas"]["Timestamp"];
        };
        /** @enum {unknown} */
        state: "requested" | "approved" | "rejected" | "expired" | "voided";
        /**
         * ApprovalGate
         * @description What the Approval gates. `capability` is present for `policy_required`.
         */
        ApprovalGate: {
            /** @enum {unknown} */
            kind: "step_approval" | "policy_required" | "work_item_acceptance";
            stepExecutionId?: string;
            workItemId?: components["schemas"]["WorkItemId"];
            capability?: string;
            policyEvaluationId?: string;
        };
        /**
         * PolicyResource
         * @description What the action touches; `type` selects the matching rule. Secrets appear by name only.
         */
        PolicyResource: {
            /** @enum {unknown} */
            type: "git_ref" | "url" | "host" | "path" | "plugin" | "secret" | "tracker_item" | "deploy_target" | "pull_request" | "repository";
        } & {
            [key: string]: unknown;
        };
        /**
         * ApprovalOccurrence
         * @description The one occurrence an Approval authorizes: (Step Execution id, Capability, resource). Decided by ADR-0048 (accepted 2026-10-26, Q56).
         */
        ApprovalOccurrence: {
            stepExecutionId: string;
            capability: string;
            resource: components["schemas"]["PolicyResource"];
        };
        /**
         * Role
         * @description Fixed role set, no custom roles (ADR-0038). Clients must tolerate new values.
         * @enum {unknown}
         */
        Role: "owner" | "admin" | "maintainer" | "contributor" | "viewer";
        /**
         * Principal
         * @description Whoever or whatever a request or decision is attributed to (ADR-0038). Only `human` may decide an Approval. `agent` and `trigger` require a human `sponsor`: for `trigger` that is ADR-0048 (accepted 2026-10-26, Q57); the `agent` rule is in POLICY_AND_APPROVAL_SHAPES section 2.
         */
        Principal: {
            /** @enum {unknown} */
            kind: "human" | "agent" | "trigger" | "plugin";
            id: string;
            displayName?: string;
            organizationRole?: components["schemas"]["Role"];
            projectRole?: components["schemas"]["Role"] | null;
            /** @description Id of the accountable human principal. */
            sponsor?: string;
            teams?: string[];
        } & (unknown & unknown);
        /**
         * ApprovalRequirement
         * @description Copied from the Policy output at request time so a later configuration change does not rewrite what the approver was asked to satisfy.
         */
        ApprovalRequirement: {
            minRole: components["schemas"]["Role"];
            requiredTeam?: string;
            separationOfDuties?: {
                enabled: boolean;
                differFrom?: ("initiator" | "agentSponsor")[];
                /** @enum {unknown} */
                reason?: "single_user_install";
            };
            expiresAt?: components["schemas"]["Timestamp"];
        };
        /**
         * EvidenceItem
         * @description One reference into the Evidence view an Approval was shown (POLICY_AND_APPROVAL_SHAPES 3.2). References and hashes only; never content or secrets.
         */
        EvidenceItem: {
            /** @constant */
            kind: "execution";
            id: string;
        } | {
            /** @constant */
            kind: "step_execution";
            id: string;
        } | {
            /** @constant */
            kind: "commit";
            sha: string;
            repository: string;
        } | {
            /** @constant */
            kind: "diff";
            hash: components["schemas"]["Digest"];
        } | {
            /** @constant */
            kind: "artifact";
            id: string;
            hash: components["schemas"]["Digest"];
        } | {
            /** @constant */
            kind: "check_result";
            id: string;
            hash: components["schemas"]["Digest"];
            /** @enum {unknown} */
            outcome: "passed" | "failed" | "errored" | "skipped";
        } | {
            /** @constant */
            kind: "definition";
            commit: string;
            contentHash: components["schemas"]["Digest"];
            dirty: boolean;
        };
        /** ApprovalDecisionRecord */
        ApprovalDecisionRecord: {
            /** @enum {unknown} */
            outcome: "approved" | "rejected";
            approver: components["schemas"]["Principal"];
            /** @description Free text. Must not contain secret values. */
            rationale?: string;
            decidedAt: components["schemas"]["Timestamp"];
        };
        /**
         * Approval
         * @description A human decision bound to specific Evidence (ADR-0036). `approved` and `rejected` are terminal and immutable. `occurrence` and the use-time digest check are decided by ADR-0048 (accepted 2026-10-26, Q56).
         */
        Approval: {
            /** @constant */
            schemaVersion: 1;
            id: components["schemas"]["Ulid"];
            organizationId?: components["schemas"]["Ulid"];
            projectId: string;
            /** @enum {unknown} */
            state: "requested" | "approved" | "rejected" | "expired" | "voided";
            gate: components["schemas"]["ApprovalGate"];
            occurrence?: components["schemas"]["ApprovalOccurrence"];
            requester: components["schemas"]["Principal"];
            requirement: components["schemas"]["ApprovalRequirement"];
            evidence: {
                digest: components["schemas"]["Digest"];
                items: components["schemas"]["EvidenceItem"][];
            };
            decision?: components["schemas"]["ApprovalDecisionRecord"];
            requestedAt: components["schemas"]["Timestamp"];
            replaces?: components["schemas"]["Ulid"] | null;
            voidedBy?: string | null;
        } & (unknown & unknown & unknown & unknown & unknown);
        /** ApprovalList */
        ApprovalList: components["schemas"]["Approval"][];
        /**
         * DecideApproval
         * @description Body of the decision endpoint. Only a human principal may decide (ADR-0036); the decider is the authenticated principal, never a body field.
         */
        DecideApproval: {
            /** @enum {unknown} */
            outcome: "approved" | "rejected";
            /** @description The digest the approver was shown; a mismatch with the Approval is `conflict`. */
            evidenceDigest: components["schemas"]["Digest"];
            rationale?: string;
        };
        /**
         * Hello
         * @description First frame on every stream connection. Not part of the seq stream. `lastSeq` is the highest seq emitted so far (0 if none); a client whose cursor is above it must resync.
         */
        Hello: {
            /** @constant */
            type: "hello";
            /** @constant */
            protocolVersion: 1;
            platformVersion: string;
            lastSeq: number;
        };
        /**
         * ResyncRequired
         * @description Sent instead of replay when the requested cursor cannot be served (older than retained history, or ahead of `lastSeq`). The client must reload state through the HTTP API and resume from `resumeFrom`.
         */
        ResyncRequired: {
            /** @constant */
            type: "resync_required";
            /** @enum {unknown} */
            reason: "cursor_too_old" | "cursor_ahead" | "history_unavailable";
            resumeFrom: number;
        };
        /**
         * Frame
         * @description Any frame on the event stream.
         */
        Frame: components["schemas"]["Hello"] | components["schemas"]["ResyncRequired"] | components["schemas"]["Event"];
    };
    responses: {
        /** @description Error. Status mapping: invalid_request 400, unauthorized 401, forbidden_origin 403, forbidden 403, not_found 404, conflict 409, rate_limited 429, needs_grant 403 (the action needs a Grant), provider_unavailable 422, protocol_mismatch 426, internal 500. */
        Error: {
            headers: {
                [name: string]: unknown;
            };
            content: {
                "application/json": components["schemas"]["Error"];
            };
        };
    };
    parameters: {
        /** @description Id returned by POST /projects. */
        ProjectId: string;
        ExecutionId: string;
    };
    requestBodies: never;
    headers: never;
    pathItems: never;
}
export type $defs = Record<string, never>;
export interface operations {
    getHealth: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Health"];
                };
            };
        };
    };
    listProjects: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ProjectList"];
                };
            };
            default: components["responses"]["Error"];
        };
    };
    attachProject: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["AttachProject"];
            };
        };
        responses: {
            /** @description The attached project */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Project"];
                };
            };
            default: components["responses"]["Error"];
        };
    };
    getProject: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                /** @description Id returned by POST /projects. */
                projectId: components["parameters"]["ProjectId"];
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Project"];
                };
            };
            default: components["responses"]["Error"];
        };
    };
    listProviders: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ProviderList"];
                };
            };
            default: components["responses"]["Error"];
        };
    };
    listCapabilities: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["CapabilityList"];
                };
            };
            default: components["responses"]["Error"];
        };
    };
    listExecutions: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                /** @description Id returned by POST /projects. */
                projectId: components["parameters"]["ProjectId"];
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ExecutionList"];
                };
            };
            default: components["responses"]["Error"];
        };
    };
    createExecution: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                /** @description Id returned by POST /projects. */
                projectId: components["parameters"]["ProjectId"];
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["CreateExecution"];
            };
        };
        responses: {
            /** @description Created */
            201: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Execution"];
                };
            };
            default: components["responses"]["Error"];
        };
    };
    getExecution: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                /** @description Id returned by POST /projects. */
                projectId: components["parameters"]["ProjectId"];
                id: components["parameters"]["ExecutionId"];
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Execution"];
                };
            };
            default: components["responses"]["Error"];
        };
    };
    cancelExecution: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                /** @description Id returned by POST /projects. */
                projectId: components["parameters"]["ProjectId"];
                id: components["parameters"]["ExecutionId"];
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Execution"];
                };
            };
            default: components["responses"]["Error"];
        };
    };
    listExecutionEvents: {
        parameters: {
            query?: {
                limit?: number;
                after?: number;
            };
            header?: never;
            path: {
                /** @description Id returned by POST /projects. */
                projectId: components["parameters"]["ProjectId"];
                id: components["parameters"]["ExecutionId"];
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["EventList"];
                };
            };
            default: components["responses"]["Error"];
        };
    };
    events: {
        parameters: {
            query?: {
                since?: number;
                executionId?: string;
            };
            header?: never;
            path: {
                /** @description Id returned by POST /projects. */
                projectId: components["parameters"]["ProjectId"];
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Switching Protocols */
            101: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
            default: components["responses"]["Error"];
        };
    };
    listWorkPackages: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                /** @description Id returned by POST /projects. */
                projectId: components["parameters"]["ProjectId"];
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["WorkPackageList"];
                };
            };
            default: components["responses"]["Error"];
        };
    };
    createWorkPackage: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                /** @description Id returned by POST /projects. */
                projectId: components["parameters"]["ProjectId"];
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["CreateWorkPackage"];
            };
        };
        responses: {
            /** @description Created */
            201: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["WorkPackage"];
                };
            };
            default: components["responses"]["Error"];
        };
    };
    getWorkPackage: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                /** @description Id returned by POST /projects. */
                projectId: components["parameters"]["ProjectId"];
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["WorkPackage"];
                };
            };
            default: components["responses"]["Error"];
        };
    };
    updateWorkPackage: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                /** @description Id returned by POST /projects. */
                projectId: components["parameters"]["ProjectId"];
                id: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["UpdateWorkPackage"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["WorkPackage"];
                };
            };
            default: components["responses"]["Error"];
        };
    };
    listWorkItems: {
        parameters: {
            query?: {
                packageId?: string;
            };
            header?: never;
            path: {
                /** @description Id returned by POST /projects. */
                projectId: components["parameters"]["ProjectId"];
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["WorkItemList"];
                };
            };
            default: components["responses"]["Error"];
        };
    };
    createWorkItem: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                /** @description Id returned by POST /projects. */
                projectId: components["parameters"]["ProjectId"];
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["CreateWorkItem"];
            };
        };
        responses: {
            /** @description Created */
            201: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["WorkItem"];
                };
            };
            default: components["responses"]["Error"];
        };
    };
    getWorkItem: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                /** @description Id returned by POST /projects. */
                projectId: components["parameters"]["ProjectId"];
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["WorkItem"];
                };
            };
            default: components["responses"]["Error"];
        };
    };
    updateWorkItem: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                /** @description Id returned by POST /projects. */
                projectId: components["parameters"]["ProjectId"];
                id: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["UpdateWorkItem"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["WorkItem"];
                };
            };
            default: components["responses"]["Error"];
        };
    };
    listWorkflows: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                /** @description Id returned by POST /projects. */
                projectId: components["parameters"]["ProjectId"];
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["WorkflowList"];
                };
            };
            default: components["responses"]["Error"];
        };
    };
    createWorkflow: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                /** @description Id returned by POST /projects. */
                projectId: components["parameters"]["ProjectId"];
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["CreateWorkflow"];
            };
        };
        responses: {
            /** @description Created */
            201: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Workflow"];
                };
            };
            default: components["responses"]["Error"];
        };
    };
    getWorkflow: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                /** @description Id returned by POST /projects. */
                projectId: components["parameters"]["ProjectId"];
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Workflow"];
                };
            };
            default: components["responses"]["Error"];
        };
    };
    updateWorkflow: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                /** @description Id returned by POST /projects. */
                projectId: components["parameters"]["ProjectId"];
                id: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["UpdateWorkflow"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Workflow"];
                };
            };
            default: components["responses"]["Error"];
        };
    };
    getWorkflowSource: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                /** @description Id returned by POST /projects. */
                projectId: components["parameters"]["ProjectId"];
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/yaml": string;
                };
            };
            default: components["responses"]["Error"];
        };
    };
    runWorkflow: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                /** @description Id returned by POST /projects. */
                projectId: components["parameters"]["ProjectId"];
                id: string;
            };
            cookie?: never;
        };
        requestBody?: {
            content: {
                "application/json": components["schemas"]["RunWorkflow"];
            };
        };
        responses: {
            /** @description Created */
            201: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Execution"];
                };
            };
            default: components["responses"]["Error"];
        };
    };
    listAgents: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                /** @description Id returned by POST /projects. */
                projectId: components["parameters"]["ProjectId"];
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["AgentList"];
                };
            };
            default: components["responses"]["Error"];
        };
    };
    createAgent: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                /** @description Id returned by POST /projects. */
                projectId: components["parameters"]["ProjectId"];
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["CreateAgent"];
            };
        };
        responses: {
            /** @description Created */
            201: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Agent"];
                };
            };
            default: components["responses"]["Error"];
        };
    };
    getAgent: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                /** @description Id returned by POST /projects. */
                projectId: components["parameters"]["ProjectId"];
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Agent"];
                };
            };
            default: components["responses"]["Error"];
        };
    };
    deleteAgent: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                /** @description Id returned by POST /projects. */
                projectId: components["parameters"]["ProjectId"];
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Deleted */
            204: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
            default: components["responses"]["Error"];
        };
    };
    updateAgent: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                /** @description Id returned by POST /projects. */
                projectId: components["parameters"]["ProjectId"];
                id: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["UpdateAgent"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Agent"];
                };
            };
            default: components["responses"]["Error"];
        };
    };
    listTeams: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                /** @description Id returned by POST /projects. */
                projectId: components["parameters"]["ProjectId"];
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["TeamList"];
                };
            };
            default: components["responses"]["Error"];
        };
    };
    createTeam: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                /** @description Id returned by POST /projects. */
                projectId: components["parameters"]["ProjectId"];
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["CreateTeam"];
            };
        };
        responses: {
            /** @description Created */
            201: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Team"];
                };
            };
            default: components["responses"]["Error"];
        };
    };
    getTeam: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                /** @description Id returned by POST /projects. */
                projectId: components["parameters"]["ProjectId"];
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Team"];
                };
            };
            default: components["responses"]["Error"];
        };
    };
    deleteTeam: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                /** @description Id returned by POST /projects. */
                projectId: components["parameters"]["ProjectId"];
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Deleted */
            204: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
            default: components["responses"]["Error"];
        };
    };
    updateTeam: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                /** @description Id returned by POST /projects. */
                projectId: components["parameters"]["ProjectId"];
                id: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["UpdateTeam"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Team"];
                };
            };
            default: components["responses"]["Error"];
        };
    };
    getSettings: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                /** @description Id returned by POST /projects. */
                projectId: components["parameters"]["ProjectId"];
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ProjectSettings"];
                };
            };
            default: components["responses"]["Error"];
        };
    };
    updateSettings: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                /** @description Id returned by POST /projects. */
                projectId: components["parameters"]["ProjectId"];
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["UpdateSettings"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ProjectSettings"];
                };
            };
            default: components["responses"]["Error"];
        };
    };
    listArtifacts: {
        parameters: {
            query?: {
                type?: string;
                workItemId?: string;
            };
            header?: never;
            path: {
                /** @description Id returned by POST /projects. */
                projectId: components["parameters"]["ProjectId"];
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ArtifactList"];
                };
            };
            default: components["responses"]["Error"];
        };
    };
    getArtifact: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                /** @description Id returned by POST /projects. */
                projectId: components["parameters"]["ProjectId"];
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Artifact"];
                };
            };
            default: components["responses"]["Error"];
        };
    };
    getDeliveryLoop: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                /** @description Id returned by POST /projects. */
                projectId: components["parameters"]["ProjectId"];
                id: components["schemas"]["WorkPackageId"];
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["DeliveryLoopLedger"];
                };
            };
            default: components["responses"]["Error"];
        };
    };
    listApprovals: {
        parameters: {
            query?: {
                state?: components["schemas"]["state"];
            };
            header?: never;
            path: {
                /** @description Id returned by POST /projects. */
                projectId: components["parameters"]["ProjectId"];
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ApprovalList"];
                };
            };
            default: components["responses"]["Error"];
        };
    };
    getApproval: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                /** @description Id returned by POST /projects. */
                projectId: components["parameters"]["ProjectId"];
                approvalId: components["schemas"]["Ulid"];
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Approval"];
                };
            };
            default: components["responses"]["Error"];
        };
    };
    decideApproval: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                /** @description Id returned by POST /projects. */
                projectId: components["parameters"]["ProjectId"];
                approvalId: components["schemas"]["Ulid"];
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["DecideApproval"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Approval"];
                };
            };
            default: components["responses"]["Error"];
        };
    };
}
