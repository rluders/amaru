---
title: Adapt selected prototype HTTP handlers to the Platform public API
status: proposed
date: 2026-11-12
deciders:
---

# 0059. Adapt selected prototype HTTP handlers to the Platform public API

## Context

This is the migration proposal for `MADR-httpapi` ([#83](https://github.com/rluders/amaru/issues/83), parent [#18](https://github.com/rluders/amaru/issues/18)). [ADR-0007](0007-reuse-of-earlier-internal-code-is-a-separate-activity.md) requires component approval before code moves. The planned disposition is **Adapt**, with a fresh import and provenance. This proposal neither moves code nor authorizes it before acceptance.

**Verified source:** `rluders/weave-obsidian`, commit `c8a2f6e983beba59edc446f36df2f442edaa9855`, the [ADR-0032](0032-migration-baseline-is-a-pinned-commit.md) baseline; component `runtime/internal/transport/httpapi/`. On 2026-11-12, its immutable Git tree contained 26 Go files, 4,078 lines and 55 test functions. The [source evidence record](../../validation/m3/2026-11-12-linux-madr-httpapi.md) records inspection and all 55 tests passing against that commit. These are predecessor tests, not evidence that Amaru serves an API.

The package couples handlers to the predecessor's `app.ProjectContext`, registry, managers and in-memory Event bus. Its middleware accepts the desktop Origin, permits requests without Origin, authenticates with a bearer token, and accepts a token in the Event-stream query. Those choices cannot implement Amaru's [ADR-0034](0034-local-web-client-authentication-with-launch-code-and-cookie.md) browser boundary. Its `decode` helper rejects unknown fields and caps body size, but only decodes one JSON value; adaptation must also reject trailing content without echoing sensitive input.

Amaru already owns [OpenAPI](../../../packages/contracts/openapi/openapi.yaml), [schemas](../../../packages/contracts/schemas/), [shared fixtures](../../../packages/contracts/fixtures/manifest.json), [routeguard](../../../packages/contracts/routeguard/routeguard.go), and a [typed SDK](../../../packages/sdk/README.md). Its [Event replay core](../../engineering/event-stream.md) is implemented on #18's integration branch by PR #322; it indexes Execution Events, not all public change Events. Current domain/store code and SDK tests do not establish a running HTTP API. #18's acceptance still requires the real router, contract fixtures and Host/Origin/CSRF tests.

## Decision

The following is proposed, subject to human approval:

1. **Source, disposition and history.** Adapt selected behavior and handler/test code from the exact component and commit above, using **fresh import with provenance**, without subtree history or a compatibility layer. Each implementation PR records source path and blob ID for copied/adapted material in `services/platform/PROVENANCE.md` and links this ADR from its README. Amaru becomes canonical; do not change or synchronize the predecessor. Any later source commit requires an explicit provenance review and owner approval.
2. **Placement and ownership.** HTTP adapters live outside the pure domain under `services/platform`, invoke Platform commands/queries through its ports, and own no authoritative state. They never use predecessor app managers, stores or Event buffers. Clients use only the public API; Workers remain Step runners. Keep the same public contracts across deployment models; authentication composition supplies the authenticated Organization/Project context. Local browser authentication is #92 and its migration decision #84, not granted by this ADR. No new package is created until a real implementation and validation target land.
3. **Selective adaptation.** The file groups below are the maximum reuse boundary, not a promise to expose every predecessor route. Register only current contract operations. An absent operation requires a separately reviewed contract change before a handler can be exposed.

   | Source in `runtime/internal/transport/httpapi/` | Disposition and Amaru target |
   |---|---|
   | `server.go`: handler registration, Project lookup, health, Provider/Capability queries, Execution and Artifact handlers, JSON/error response helpers | Adapt against OpenAPI and Platform ports; replace predecessor types/error mappings. Authentication must precede Project/resource resolution. Never use a globally resolved Execution ID without verifying its Project |
   | `work.go`, `workflow.go`, `agents.go`, `teams.go`, `settings.go` | Adapt request validation and command/query behavior for existing Work Package/Work Item, Workflow, Agent, Team and settings operations; preserve partial updates and reject stale conflicting writes at the mutation boundary |
   | `events.go` | Reference only; write the transport composition over Amaru's committed replay port. Preserve useful disconnect, bounded delivery and reconnect cases, but never import its in-memory bus or token-query authentication |
   | `server.go`: `authenticate`, `checkOrigin`, hook exemptions and predecessor auth/discovery wiring | Do not move; #84/#92 own the ADR-0034 replacement. No desktop Origin default, query bearer token or blanket missing-Origin browser bypass |
   | `grants.go`, `hooks.go`, `connectors.go`, `mcp.go`, `skills.go`, `playbooks.go`, `workflow_templates.go`; Provider mutation/secret and egress-host handlers in `server.go` | Reference only, outside this HTTP migration's import boundary. These routes are not permission to expand the current API; their owning features/contracts and applicable migration gates must be reviewed first |
   | `*_test.go` | Adapt only the selected behavior cases below into Amaru-owned tests/fixtures; no predecessor schema, app package or repository is a test prerequisite |

4. **Wire behavior.** OpenAPI and strict payload schemas control names, paths, responses and errors. Translate predecessor Runtime → Platform, autoloop → Delivery Loop, and project context → authorized Project under its Organization using the [glossary](../../product/GLOSSARY.md#2-term-mapping-from-predecessor-repositories). Do not add legacy aliases. Health reports the existing integer `protocolVersion` and separate `platformVersion`; negotiation (#106) must not interpret the build version as the protocol. Public Event envelopes, hello/resync messages and cursors follow the current contracts; #219 owns additional typed change producers. Do not expose the internal replay page as the public envelope.
5. **Security and failure behavior.** Check Host, browser Origin, session and CSRF under ADR-0034 before mutations or WebSocket upgrades; keep CLI/Desktop credentials distinct. Validate Project scope on every query, mutation and replay. Cap input bodies, reject unknown fields and trailing JSON, and return contract-safe errors without raw bodies, cookies, tokens, launch codes or Provider output. Reject invalid or unauthorized commands without state changes or Events. Publish only committed changes; duplicate delivery or reconnect must not repeat a command. Slow/disconnected clients release resources and receive explicit resync when continuity cannot be maintained; they never block durable commands or invent missed history. Restart replay reads durable Platform state. The HTTP layer introduces no broker, database, dependency or independent state owner.
6. **Tests before claiming the API implemented.** Use Amaru fixtures, fake ports and loopback HTTP/WebSocket clients, with no real agents or paid APIs. The table below is the minimum behavior plan; implementation PRs supply the actual results. Adapted source test names are evidence/navigation only, never required imports.
7. **Separate owners and gates.** #168's egress consent is fresh Amaru work under its recorded owner decision and merged PR #321 on #16's integration branch; this ADR neither gates it nor brings that code onto #18's base. Remaining webhook adaptation belongs to #95. #135's fresh replay core remains outside this migration gate. #84 remains a separate decision; accepting this proposal alone does not unblock work that also depends on #84. #83 closes only after explicit approval is recorded and this ADR is merged as accepted; merging a proposed document is insufficient.

## Behavior verification plan

All rows are **proposed / not validated in Amaru's HTTP adapter**.

| Behavior | Amaru verification; source reference |
|---|---|
| Route/fixture parity and errors | Feed the real registrations to `routeguard`, fail both undocumented and unserved operations, and run shared fixtures plus generated SDK requests against the running local server. Source: `TestHealthIsOpenAndMatchesContract`, CRUD contract/error tests |
| Validation without side effects | Unknown fields, malformed/trailing/oversized JSON, invalid IDs and failed commands leave state/Event cursors unchanged. Errors remain useful and redact secret fixtures. Source: `TestCreateExecutionErrors`, `TestWorkErrors`, `TestWorkflowRunRejections` |
| Two Projects and authorization | Attach two distinct roots, use the same definition and secret names, and exercise foreign Project/resource IDs, settings, Executions and Events. One failing/waiting Project does not block the healthy one. Source: `TestProjectsAreIsolated`, `TestAgentsAreIsolatedPerProject`; #191 owns completion |
| Browser boundary | Test allowed/forged Host, missing/foreign Origin, missing/invalid CSRF, expired/reused launch code, cookie/session expiry and WebSocket upgrade. Assert no command dispatch after rejection. Source `TestOriginPolicy` and `TestWebSocketRejectsBadOriginAndToken` are references, not the desired auth model; #84/#92 own it |
| Execution lifecycle | Success, failure, cancellation, unknown/foreign Execution and request cancellation map to existing contracts without leaking Provider output; no Worker/Provider runs directly in the handler. Source lifecycle/cancel/failure tests |
| Resume and committed changes | Two clients observe one attributable committed mutation; reconnect after cursor, duplicates, empty current stream, retained-window loss, invalid/ahead cursor, slow reader, disconnect and restart. Public resync is explicit; preserve Project isolation and original Execution order. Source replay/lost-window tests; use #135 and #219 rather than the old bus |
| Definition edits and conflicts | File and API edits generate strict typed changes; invalid external files retain the authoritative valid state; a stale full-field Agent edit cannot overwrite newer unrelated fields. #219 owns this, coordinated with mutation version checks |
| Compatibility and secret safety | Supported/incompatible/missing protocol and connection failure remain distinct; failed negotiation sends no mutation. Capture all operational channels for success/rejection/failure/cancellation with secret fixtures. #106/#170 own completion |

## Readiness criteria

| # | Criterion | State and evidence |
|---|---|---|
| 1 | Exact component and inspected commit | **Met:** immutable source and full SHA above, verified from local Git objects |
| 2 | Refreshed behavior inventory and actual tests | **Met for predecessor evidence:** 26 files, 55 passing tests, inspected middleware/handlers; extraction setup failure and corrected run recorded in the linked evidence. No source changed |
| 3 | Accepted target boundaries | **Met:** ADR-0003/0004 (client/contracts), ADR-0025 (version policy), ADR-0034 (browser security), ADR-0042/0044 (durability/restart), ADR-0043 (definitions/state), and ADR-0007/0032 (migration). #84's component approval is still pending |
| 4 | Existing contracts and fixtures | **Met for current target:** linked OpenAPI, schemas, fixtures, SDK and routeguard. This does not certify a running server or authorize new wire shapes |
| 5 | Self-contained Amaru behavior tests | **Unmet in implementation; plan supplied:** table above, local fakes/fixtures only. Required before implementation acceptance |
| 6 | Real validation | **Unmet:** no Amaru API/browser/security run or real Provider run. Proposal acceptance must explicitly acknowledge this gap, not mark the target verified. Linux is required for M3; native macOS/Windows are best effort and not validated here |
| 7 | Licensing and provenance | **Met for decision basis:** [ADR-0026](0026-fair-code-community-license-with-commercial-licensing.md). Fresh imports must record file/blob provenance; no license file or license text is changed by this proposal |
| 8 | Owner approval of component, disposition, commit and history | **Unmet:** awaiting explicit product-owner approval of this proposal and the listed evidence gaps. No code may move yet |

## Consequences

Selected validation and handler behavior can be retained without importing the predecessor's application architecture. Contract, security and durable Event integration require substantial Amaru work; 55 passing source tests do not complete any API feature. #18 remains open until all children and its combined running-server acceptance pass.

The shared Platform/Worker/client boundaries remain unchanged. No library or infrastructure is introduced here. Implementation PRs must justify and pin any dependency separately under repository policy. Revisit this decision if the source commit, selected import boundary or public contracts change enough to invalidate the mapping.

## Alternatives considered

- **Import the package unchanged:** brings obsolete auth, undocumented routes, predecessor managers and an in-memory replay source across the Platform boundary.
- **Write all handlers without source reuse:** viable, but discards useful contract/error and isolation test behavior. Prefer fresh code where the current port/contract diverges; the selected adaptation boundary allows that without wholesale copying.
- **Migrate auth, event bus and webhooks in one decision:** combines separate owners, widens the security review and reverses the recorded fresh-core decisions for #135/#168.

## Approval

Proposed by: Codex on 2026-11-12 for #83. Awaiting product-owner approval of the exact source commit, selected Adapt disposition, fresh import with provenance, and readiness gaps 5, 6 and 8. No approval recorded; `deciders` remains empty.
