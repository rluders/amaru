# Compact Project access boundary

[ADR-0062](../architecture/adr/0062-project-bound-public-operations-and-secret-resolution.md) is accepted following owner approval in [PR #330](https://github.com/rluders/amaru/pull/330). `coordinator.ProjectAccess` implements the first #191 application slice around existing Execution operations. It does not implement a product HTTP server or complete [#191](https://github.com/rluders/amaru/issues/191).

Construct `NewProjectAccess` with the existing coordinator, the Instance's attached Project store and `localauth.Auth.Principal`. Every call rechecks authentication and attached Project identity. Compact composition has one configured Organization; its local administrator has owner access to those Projects. No membership database or multi-Organization authorization is implemented. The authentication callback is a trusted composition dependency, never a principal decoded from a request body or headers. A custom callback must enforce current authorization on every call.

`Auth.Principal` accepts only context established by that Auth instance's guarded downstream dispatch. Credential-free health/root requests and another Instance's context confer no identity. It refuses cancelled requests, logged-out/expired browser sessions and shutdown native contexts. Identity is the Instance-local `local-administrator`; session IDs and bearer values never become domain actor IDs. Origin/CSRF checks still precede authenticated dispatch.

`ProjectAccess` uses the coordinator's own authoritative store, preserving the trusted internal APIs for scheduling and recovery. It refuses invalid/unattached Project selectors before resource lookup. A foreign or missing Execution/Approval returns exactly `domain.ErrNotFound` with no returned child records, replay outcome or cursor. Approval ownership checks both its Project and its gating Execution. Ownership is checked before calling command replay or Evidence's context/artifact/audit readers. Existing stores already preserve immutable Execution Project identity; this slice changes no persisted storage contract, migration or transaction semantics. A read-only `ports.EvidenceReader` keeps the coordinator dependent only on domain/ports; trusted composition supplies the existing Evidence reader.

Manual Submit derives Project, principal and trigger from the trusted boundary. Cancel derives the human Actor and ActorID; Approval decisions derive their principal. Existing coordinator Policy, Approval, version checks, atomic writes and `(command name, key)` command namespace remain in force. Same-key requests for different Projects conflict; they do not receive independent records. A current owner can replay the same request after state changes; a revoked session cannot replay it.

## Operation inventory and remaining composition

Inspected against `packages/contracts/openapi/amaru.yaml` and the [coordinator command inventory](command-idempotency.md). This is an application/source mapping, not a claim that these HTTP routes are served.

| Public operation family | This slice | Remaining delivery gate |
|---|---|---|
| Execution create; Workflow run | Authorized manual `Submit`; `RunWorkflow` loads a named committed Workflow through `ports.ProjectWorkflowSource`, then binds Project/actor before replay | Agent/Skill/Environment resolution and wire-to-domain mapping; do not pass client definitions directly into the internal Submit request |
| Execution detail/cancel | Project-bound `Execution` and `Cancel` | HTTP handlers and shared contract/SDK acceptance |
| Execution definition/Evidence | Ownership before pinned definition return or Evidence assembly | Public mapping; direct Artifact ID/content access still needs its own ownership boundary |
| Project and Execution Event replay | Project authorization plus ownership of any Execution filter before the existing journal read; trusted clock/window | Typed envelope mapping, WebSocket lifecycle and cursor wire mapping |
| Approval detail/decision | Project-bound `Approval` and `DecideApproval` before replay | List/query adapters, HTTP handlers and contract/SDK acceptance |
| Project list/detail/attach | Existing Project/attach components are unchanged | Authenticated product composition |
| Execution lists, Work Packages/Items and dependencies | No new implementation | Project-selected authoritative query/mutation paths; reject cross-Project dependencies |
| Workflow/Agent/Team CRUD/source, settings | `Workflow` reads the selected Project's committed Workflow at an explicit revision | Settings and other definition types, CRUD, versioned API/disk reconciliation (#219) |
| Artifact list/detail/content; delivery-loop query | No new implementation | Project-bound access; delivery-loop implementation remains M4 |
| Health/auth, Provider/Capability discovery | Existing auth subset remains; identity now available to composition | Product startup, authenticated discovery and applicable protocol mapping |
| Grant and trust mutations (internal command inventory) | Existing trusted commands unchanged | Any future public exposure must authorize the target Project/Grant before replay; never expose raw coordinator methods |

Event stream and Evidence reader passed to application calls are trusted deployment dependencies. They must use the same authoritative Instance data as the coordinator. Event replay windows are deployment configuration, never client input. The Evidence reader retains its existing sequential-read consistency limits. Neither auth nor this boundary stores product state or changes Worker/Provider contracts.

## Committed Workflow selection

`attach.WorkflowReader` implements `ports.ProjectWorkflowSource`, reusing the Attacher's hardened, bounded Git process path and existing strict `workflowdef` parser. Trusted composition supplies an absolute attached-root lookup keyed by Project and the same Instance's Project store. It checks the root's current Project identity, refuses shallow clones, requires a full lowercase commit ID naming a commit, and checks its root commit against the current history using ADR-0046's smallest-root rule. Missing/foreign content returns bare `ErrNotFound`. Only a regular committed `.amaru/workflows/<id>.yaml` blob is read; symlinks, path selectors, mismatched IDs, invalid/oversized definitions and unknown format versions are refused. Repository/parser errors omit raw diagnostics and values. Working-tree edits are not loaded, repository content is not executed, and no Project file or operational record is written by loading.

`ProjectAccess.Workflow` checks live authentication and attached Project membership before calling the trusted source. `RunWorkflowRequest` carries only the command key, Workflow ID, explicit commit ID and variable overrides. `RunWorkflow` loads that definition and rechecks authentication in `Submit`, then reuses existing validation, Policy/Approval, snapshot pinning and atomic command outcomes. The explicit revision keeps retries stable after HEAD changes; changing it with an existing key conflicts. Loading makes no trust decision and grants no execution permission. Draft definitions can be queried but cannot run. No cache is introduced, so Projects and revisions share no mutable definition entry.

The requested Git object must remain available to load before replay. If it is removed, this application call fails closed; the existing Execution's persisted snapshot remains queryable. Product HTTP replay semantics still require composition acceptance.

The compact clone-location lookup is an injected trusted dependency, not a new persisted clone registry or public path parameter. Product composition still needs that registry and the HTTP contract mapping; this internal application request is not a new wire contract. Other definition types and settings remain pending. The existing settings format is still proposed; this slice introduces no configuration-file syntax or change to `ResolveConfig` precedence. Public Environment selection also remains a separate gate.

## Evidence and limits

[Linux validation record](../validation/m3/2026-11-12-linux-prj-002-access.md) covers two registered Projects, matching Step names, foreign/missing IDs, replay, conflicting keys, child-read ordering, cursor withholding, concurrent duplicate cancellation and authentication revocation. The tests compose the real auth guard and application boundary with fake stores. They establish application behavior, not a running public API, real Project attachment, durable-adapter restart or full two-client transport acceptance.

The [committed Workflow validation record](../validation/m3/2026-11-12-linux-prj-002-workflows.md) adds two attached temporary Git Projects with matching Workflow/Step names, separate pinned definitions, foreign commit/root rejection, concurrent old-revision replay and invalid-content isolation. It does not exercise dispatch or a public transport.

Remaining #191 work includes Project-selected settings and other definitions, Work dependencies, matching secret names with exact-scope resolution and Policy/Approval delivery, production Provider-unavailability handling and composed API/SDK acceptance. An encrypted production secret backend and authorized assignment delivery remain open gates; fake secret resolution alone cannot close #191. #191, #18 and M3 remain open. Linux is required; macOS/Windows are best effort and unvalidated by this slice.

## Independent Step progress

`TestProjectsProgressIndependently` composes the actual coordinator and single-slot embedded Worker with fake storage and Runner outcomes over both in-process and JSON serialization links. One Project retains a pending `git.push` Approval while another Execution in that Project fails, reports a fake unavailable Provider, or panics. A second Project completes its Step, then completes another after coordinator/Worker recreation over retained fake state. Matching Step names stay separate. Default Policy remains active; trust is an explicit test-only precondition.

Every result is delivered twice and both accepted/duplicate acknowledgements are observed. Existing Step records, Approvals and Events remain unchanged after restart and unrelated work; each completed Step has one terminal Event. This is fake-backed integration evidence for Step progress, not real Provider discovery, a remote transport, durable process recovery or public API acceptance. See the [Linux progress record](../validation/m3/2026-11-12-linux-prj-002-progress.md).

The verification identified the missing terminal parent aggregation tracked in [#335](https://github.com/rluders/amaru/issues/335). The subsequent [Execution completion fix](execution-lifecycle.md) implements accepted ADR-0063, with separate regression and PostgreSQL evidence. The original #334 slice changed no runtime behavior and did not establish full lifecycle acceptance. #191 remains open for its other gates.
