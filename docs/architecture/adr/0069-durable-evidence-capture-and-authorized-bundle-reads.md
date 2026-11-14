---
title: Durable Evidence capture and authorized bundle reads
status: accepted
date: 2026-11-13
deciders: Ricardo Lüders
---

# 0069. Durable Evidence capture and authorized bundle reads

## Context

[ART-002 #124](https://github.com/rluders/amaru/issues/124), under
[WP-M3-07 #19](https://github.com/rluders/amaru/issues/19), requires one query
for attributable Execution Evidence: definitions, context and Skill snapshots,
input/output hashes, Artifact versions, diffs, ordered Events/logs, checks,
Approvals, usage and workspace snapshots. Evidence should compose authoritative
records, not duplicate every entity. Missing, expired and not-yet-produced
content must remain visible.

This is an **accepted** decision. The [bounded Usage capture implementation](../../../services/platform/README.md#bounded-usage-capture-art-002) has local Linux evidence; the complete decision remains unimplemented. Source review used #19 head
`0ff1ef010f258edca005ebb373b785c588ebc326`. It neither implements recorders nor
adds a public contract. The owner approved how reports become durable
Evidence and the authorization and consistency boundary for one bundle query.
The explicit approval below resolves this integration decision; earlier domain
delegation was not its authority. See [Q61](../OPEN_QUESTIONS.md#q61-durable-evidence-capture-and-authorized-bundle-reads).

### Verified source facts at the reviewed head

- [Reader.Bundle](../../../services/platform/internal/evidence/evidence.go)
  (lines 15–68) reads Execution, Events, Approvals, audit, context and Artifacts
  sequentially. Its comment explicitly disclaims a shared snapshot. It has no
  principal, Project or Organization access check or production API composition.
  The audit helper (lines 71–97) scans the audit log and filters by Approval IDs;
  filtering after an unscoped read is not a tenant authorization boundary.
- [ExecutionEvidence](../../../services/platform/internal/domain/execution_evidence.go)
  (lines 20–96) explicitly lists check, diff, log, usage and workspace gaps;
  neither StepEvidence nor the builder input accepts those records. Final
  describes a terminal Execution state, not evidence completeness. Digest hashes
  the assembled representation; it does not establish an atomic read or freeze
  subsequent Artifact versions.
- [ADR-0033](0033-usage-and-cost-telemetry-model.md)
  already decides Usage provenance, unknown values and all-attempt accounting.
  [UsageReport](../../../packages/contracts/schemas/UsageReport.json) describes
  measurements without Platform identity;
  [UsageRecord](../../../packages/contracts/schemas/UsageRecord.json) requires
  stepExecutionId and capturedAt. These schemas are not a persistence path.
- [Worker Protobuf](../../../packages/contracts/proto/amaru/worker/v1/worker.proto)
  lines 203–210 carries repeated usage in Result. The
  [in-process port](../../../services/platform/internal/ports/worker.go)
  lines 17–20 and 163–169 omits outputs and usage;
  [the parity test](../../../services/platform/internal/ports/worker_proto_test.go)
  line 58 explicitly exempts both. The embedded Worker sends outcome/error only
  (worker/embedded/worker.go, lines 153–187). There is no Platform Usage
  model/store/composition consuming these schemas at this head.
- [Coordinator.onResult](../../../services/platform/internal/coordinator/serve.go)
  lines 304–364 fences and deduplicates completion and acknowledges after its
  edit, but has no Usage persistence. Adding a standalone Usage table would not
  complete Worker-to-Evidence delivery. Other StepEvent types are accepted and
  dropped; only remote_job is acted on (ports/worker.go, lines 83–94).
- [OpenAPI](../../../packages/contracts/openapi/openapi.yaml) defines scoped
  Execution read/events operations, but no Evidence bundle operation/schema.
  Trail's indexed Reader construction hits are in the port test suite; a public
  endpoint cannot be inferred from the internal builder's JSON tags.

Source inspection establishes these gaps; existing tests and this docs-only
change do not establish full ART-002 acceptance. The
[prior attribution record](../../validation/m3/2026-11-13-linux-art-002-step-attribution.md)
records a narrower implemented slice.

Number allocation: fetched main and all origin integration/topic refs were
inspected on 2026-11-13. Main and #19 end at ADR-0058; the #18 API integration
branch already contains ADR-0059–0068. **0069 is the next free number** across
those refs. Absence of 0059–0068 on this base is intentional; do not reuse them.

## Decision

**Accepted direction, not implemented:** use
Platform-owned capture with a transactionally consistent metadata view composed
by a Project/Organization-authorized application service. Start with a complete
Usage result-to-persistence slice; defer public exposure until
its contract and access/consistency tests are reviewed as part of M3. Do not
reinterpret raw StepEvents or lifecycle Events as durable checks, diffs or logs.

### Product direction and milestone sequence (accepted)

Choose the transactional metadata view as the long-term audit foundation, with
Usage as the first delivery slice, not the final product boundary. Best-effort
live views may supplement it; projections or sealed exports may be added later
when measured needs justify them. Neither substitutes for authoritative capture,
scoped access or the exact Evidence binding of a human Approval.

Planning sources at reviewed head `0ff1ef010f258edca005ebb373b785c588ebc326`:
[ROADMAP M3/M4](../ROADMAP.md#m3-platform-core), lines 109–155, assigns headless
API execution and ART-002 to M3 and the visible diff/Evidence/cost and end-to-end
audit walkthrough to M4. [WORK_PACKAGES](../WORK_PACKAGES.md), lines 235–247,
requires a full test-Execution audit query in WP-M3-07; lines 286–336 assign
checks/review, the inbox/Evidence view and recorded product walkthrough to M4.
This acceptance does not move those criteria or declare them met.

The owner-supplied `docs/architecture/architecture.png` was visually inspected
from the shared checkout on 2026-11-13. It is untracked and absent from this PR's
base, so it has no immutable repository URL. Inspected PNG SHA-256:
`cc5abaf9ff3167c7edd5ffc9be5a6129d177d77ae75e6823b4dfa23eadd35a0f`.
Its Control Plane labels Identity & Access, Execution Coordination, Artifact Management, Reviews & Approvals and
Policies & Governance; its Data Layer separates Artifact Storage from Platform
Database. This supports the proposed ownership direction. The Experience Layer
shows Web, Desktop and APIs & SDKs; it does not specify an Evidence protocol.
The Brain, optional vector store and distributed deployment pictures are vision,
not accepted M3 infrastructure or implemented behavior. Canonical authority
remains [ARCHITECTURE §4](../ARCHITECTURE.md#4-platform-and-worker-boundary)
and its public-API-only client rule (lines 154–158), not this illustration.

The owner approved the following sequence:

1. **M3 capture:** complete Usage end to end, including identity, failed/cancelled
   outcomes, redaction, transaction/outbox and replay tests. Then define and
   implement the remaining recorder inputs and attribution with bounded payloads.
   Use controlled producers/fakes to prove the full test-Execution query; do not
   pull the M4 delivery loop or M5 retries/remote transport into M3.
2. **M3 read/API foundation:** integrate transaction-aware metadata composition
   and scoped authorization with WP-M3-06; review a versioned public contract and
   fixtures before exposing the bundle. Prove concurrent reads, gaps and access
   isolation. Headless does not mean internal-only: Web must inherit a tested
   public boundary. Usage alone or an internal sequential Reader is insufficient
   to complete #124/#19. Any deliberate deferral needs an explicit owner scope
   change, not a silent move into M4.
3. **M4 product integration:** connect real delivery/check/review producers
   (WP-M4-03/04) and the inbox/diff/Evidence/cost view (WP-M4-06) to that API.
   Preserve exact Approval bindings and show unknown usage, gaps and unavailable
   bytes. WP-M4-07 records reconstruction of one change end to end and native
   three-OS supervised runs; M3's Linux-first exception does not waive M4 exit.

### Evidence identity and completeness (accepted direction)

A capture identity identifies a durable fact; a content hash identifies bytes;
a bundle digest identifies an assembled representation. Keep these distinct.
Use Platform-derived Execution/Step/attempt attribution plus producer identity
and stable ordinal for capture deduplication; a replay must preserve ordering
and payload, with conflicts rejected rather than overwritten. References pin
exact definition/context/Skill revisions, Artifact versions and hashes, and
ordered history positions. Do not resolve an audit reference to mutable latest
content. Existing accepted Approval bindings remain the review identity.

Completeness must be assessed per expected Evidence kind and applicable attempt
at the metadata read boundary: captured, pending, unavailable/expired, or
explicitly not applicable with reason. These are accepted semantic distinctions,
not agreed wire enum names. Recorder-not-implemented and producer-failed gaps
must stay distinguishable; an empty list is not proof that nothing was needed.
Terminal Execution state, complete metadata and currently retrievable bytes are
independent. Missing Usage means unknown, never zero (ADR-0033). A timeout or
store failure is an unsuccessful query, not a fabricated completeness gap.
Do not seal Evidence solely because the Execution terminated; late enrichment
remains a separate owner choice and must never rewrite a decided Approval.

### Capture and publication boundary (accepted direction)

1. Providers report through Workers; the Platform validates assignment identity,
   Execution, Step, attempt and fencing token and derives Project/Organization
   from authoritative state. Worker-supplied scope is never authority. Domain
   validation remains pure; adapters handle transport, blobs and transactions.
2. Complete the existing Usage Result path through embedded/in-process mapping,
   coordinator validation, durable records and Evidence composition. Preserve
   ADR-0033 sources, decimal cost and unknown values. Persist records for accepted
   failed/cancelled outcomes too; do not count duplicate deliveries twice.
   Recommend stable capture identity per attempt and report position, retaining
   a payload fingerprint to reject conflicting replay. This is a persistence
   identity recommendation, not a new UsageReport field or wire contract.
3. Commit accepted completion, its Usage records and any required publication
   intent atomically in the Platform store before acknowledging. Derive repeated
   acknowledgement from the committed result. Publish any existing applicable
   Event through the existing transactional outbox; do not publish first or
   invent new Events here. Review whether the existing UsageRecorded contract
   suffices in the implementation proposal. Late reports for obsolete attempts
   must not attach to the current attempt; retain existing stale-result handling.
   Late enrichment after accepted completion requires a separate owner choice.
4. Specify a producer, identity, capture point and bounded payload for each
   remaining kind before implementing it: checks from the check runner (command
   configuration revision, result and output hash); diffs from version-control
   capture (base/head and diff content hash); ordered redacted logs from the
   Worker stream (attempt and ordinal); workspace snapshots from workspace
   capture (revision/manifest and hashes). These are accepted attribution
   requirements, not available producers or agreed record shapes. Logs require
   durable incremental capture rather than an unbounded terminal Result.
5. Store large content through the accepted Artifact/blob boundary, computing
   hashes on the Platform. Make verified content durable before publishing its
   reference. A failed metadata commit may leave an orphan blob, never a dangling
   successful reference. Reconciliation and cleanup must respect retained
   references and Approval bindings; do not invent retention periods. Preserve
   compact/self-hosted/Cloud semantics with storage adapters only.

### Bundle access and read boundary (accepted direction)

- Expose one logical bundle query through the public API application layer only
  after a versioned contract review. No route, schema, CLI flag or exported port
  is added by this ADR. Reuse the Reader's pure assembly/identity checks inside
  a suitable consistent read scope; its current sequential store calls alone
  cannot supply the proposed guarantee.
- Authenticate, resolve the requested Project in the principal's Organization,
  authorize Evidence read, then constrain Execution lookup and every child query
  to that scope. Authorize at the metadata read boundary; a revocation committed
  afterwards applies to subsequent queries and downloads, without promising
  retroactive cancellation of an in-flight response. Check Approval audit,
  context, logs and blob references as well
  as the root Execution. Denied and nonexistent scoped resources must not leak
  cross-tenant existence; review the exact error mapping with the public contract.
  Do not serialize a partial unauthorized bundle. Blob retrieval reauthorizes
  access; a content hash or a cached bundle is not an access token. Cache entries
  must be scoped to principal permissions and Organization/Project.
- Recommend one repeatable-read transaction for all Platform metadata used in a
  bundle, including authorization-relevant scope, ordered history, capture
  references and explicit missing/expired/pending states. All internally fetched
  pages use that same read boundary. Bound the transaction duration and query size. Do not keep a
  database transaction open while a human reviews or a client fetches later
  pages; cross-request pagination would require a separately reviewed snapshot
  token/materialization contract, not ordinary cursors over changing data. This
  is an accepted adapter requirement, **not verified behavior**
  of the current ports. Existing separate adapter calls need transaction-aware
  composition; rereading only the Execution version is insufficient because
  context, Artifacts and audit may change independently.
- Blob bytes sit outside the metadata transaction. Return immutable hashed
  references and honest availability at read time; expiry/deletion during later
  download remains possible and must be explicit. Do not claim one atomic
  snapshot of PostgreSQL plus filesystem/S3. A digest identifies what was read,
  not perpetual availability. A future contract must distinguish terminal state,
  completeness, read boundary and availability without overloading Final.
- Keep Approvals bound to the exact Evidence they saw under
  [ADR-0036](0036-approvals-bind-to-evidence-and-gate-work.md) and
  [ADR-0048](0048-approval-reuse-uncovered-capabilities-and-policy-defaults.md).
  A later bundle cannot silently replace that binding. Secret removal and
  Platform redaction precede durable capture under the
  [security baseline](../../security/SECURITY_MODEL.md) and
  [data retention rules](../DATA_ARCHITECTURE.md#5-retention-and-redaction).

## Alternatives considered

| Alternative | Benefits | Costs and reason not recommended now |
|---|---|---|
| Finish Usage capture first; keep Reader internal and sequential | Smallest coherent implementation increment; reuses accepted measurements and completion fencing | Viable first phase, but leaves checks/diffs/logs/workspace and public acceptance open; it is not completion of ART-002 |
| Expose the current Reader with explicitly best-effort reads | Least read-side work; useful for an evolving live view | A single query can mix independently committed states; terminal Final and Digest can mislead reviewers. Viable only if the owner explicitly chooses this weaker promise and the contract communicates it |
| Platform capture plus transactional metadata view (recommended) | One authoritative write path; no duplicate entity store; consistent attribution/history at a read boundary | Requires coordinator transaction changes, durable capture identities and a read-scope adapter; blob availability remains separate; long audit reads can hold snapshots too long |
| Append durable capture Events and build a bundle projection | Replay and incremental stream ingestion; separates write load from read shape | Projection lag, ordering, duplicate delivery and rebuild semantics require an explicit completeness watermark; larger operational surface and possible contract work. Prefer only if scale/stream needs justify it |
| Materialize an immutable bundle at terminal completion or Approval | Stable review/export object and cheap repeat reads | Late evidence and retention/redaction changes need versions; terminal state does not mean capture is complete; duplicates sensitive data and needs sealing/reconciliation. Consider later for export or approved snapshots |

A Worker writing Platform tables or clients assembling private stores violates
[Platform/Worker/Client boundaries](../ARCHITECTURE.md#4-platform-and-worker-boundary)
and is not a viable shortcut. No new runtime/library/infrastructure dependency
is introduced here; the recommendation uses already-decided Platform storage,
Artifact storage and outbox boundaries.

## Failure modes and required follow-up verification

| Failure | Required outcome under the recommendation; not implemented by this ADR |
|---|---|
| Crash before result transaction commits | No accepted completion or Usage subset; replay can retry the same capture identity |
| Commit succeeds, acknowledgement is lost | Replay returns the durable disposition; no new Usage rows or totals |
| Conflicting duplicate / stale fencing token | Reject conflicting payload or stale attempt; never overwrite prior Evidence or reattribute costs |
| Worker disconnect / network partition before capture | Preserve already committed incremental logs; expose missing terminal reports; never infer zero usage or a passing check |
| Blob upload succeeds, metadata write fails | Retry safely or reconcile orphan content; do not expose an uncommitted reference |
| Missing/corrupt/expired blob, redaction failure | Explicit unavailable/expired state or error; never fabricate content; fail closed on unsafe persistence |
| Concurrent Artifact, Approval or audit write during pagination | One metadata read boundary includes or excludes the commit consistently; no mixed-page history |
| Store failure, timeout or cancellation mid-read | Return an error and no successful partial bundle; distinguish data gaps from infrastructure failure |
| Wrong Project/Organization or scope revocation | Deny under authorization at the read boundary; no child data or existence leak; reauthorize later downloads and cached reads |
| Long-running capture or excessive log volume | Bound size/time, stream redacted chunks, surface truncation/pending state; terminal Execution alone never seals Evidence |

Implementation tests must use fakes for identity, unknown-versus-zero, replay,
fencing, payload conflicts and scope denial. PostgreSQL integration must prove
atomic completion/capture/outbox and consistent concurrent reads; blob tests
must prove upload/commit failure and expiry behavior. Public-contract fixtures
and API tests must prove cross-Project and cross-Organization isolation,
permissions, revoked access, missing Execution and partial evidence. A recorded
multi-Step audit walkthrough must cover failed attempts, context, Artifacts,
checks, Approvals and usage before claiming full ART-002 acceptance. M5 retries
or remote Workers are not added to M3 scope by this proposal.

## Consequences

The decision makes ownership and missing integration work explicit and keeps
Evidence a view over durable facts. The recommended first slice is larger than
a new store: it spans Worker result mapping, coordinator persistence,
acknowledgement, publication and Reader composition. Future recorder and public
contract changes remain separate reviewed work within the approved boundaries.

Read transactions and bounded pagination need implementation design and tests;
there is no latency or scale claim. Revisit the recommendation if streaming
volume requires projections, if multiple authoritative stores prevent a common
metadata snapshot, or if stable exports require sealed bundles. No accepted ADR
is superseded; retention durations and Approval semantics remain as decided.

### Architecture review

**Historical pre-approval verdict: needs human decision.** The owner approval
below resolves finding 1 and the choice requested by finding 3; implementation
and verification obligations in all findings remain open.

The proposed Platform service composes durable
capture records behind public authorization; Workers report facts and clients
consume the API. This preserves domain purity, vendor-neutral Capabilities and
deployment parity without new infrastructure.

Findings, most severe first:

1. **Unapproved integration and public promise:** ports/worker.go:17–20,
   coordinator/serve.go:304–364 and OpenAPI currently provide neither complete
   capture nor exposure. Owner must choose capture/acknowledgement scope and
   access/read guarantee before implementation; schemas alone are insufficient.
2. **Authorization cannot be inferred from attribution:** evidence.go:30–97
   validates identities but performs unscoped store reads. Add scoped service and
   adapter composition and isolation tests before exposure; obey the security
   model's Organization and Project boundaries and ADR-0003/0004.
3. **Digest/Final cannot certify a snapshot:** evidence.go:26–29 and
   execution_evidence.go:75–82 describe current limits. Approve and test the read
   boundary and completeness representation before promising consistent audit
   evidence; preserve ADR-0036/0048 bindings.

## Approval

Proposed by: Codex, 2026-11-13, for ART-002 #124 on #19.

Approved by: **Ricardo Lüders, product owner**, on **2026-11-13
(America/Sao_Paulo)**, explicitly in the repository task conversation:
“You need to change the proposal to approved. Update, merge, and continue the orchestration.”
Codex records the owner's decision; this is not agent self-approval. The ADR
status vocabulary records that approval as **accepted**.

Approval scope: Platform-owned durable capture; atomic accepted completion,
Usage persistence and applicable outbox intent before acknowledgement; scoped
Project/Organization authorization for every referenced record; transactionally
consistent metadata reads with blob availability reported separately; and Usage
as the first end-to-end M3 slice. Remaining recorder design proceeds within these
constraints. M3 retains the authorized public read foundation and full
test-Execution query; M4 retains real producer/UI/audit-walkthrough integration.

This acceptance does not approve a new public wire contract, route or schema,
late enrichment, new infrastructure, retention periods, unrelated proposals or
changes to milestone criteria. Implementation and contract review remain
follow-up work. #124 and #19 and every unmet acceptance criterion stay open.
This PR records acceptance only and implements no Evidence behavior.

### Owner article context

Both local owner PDFs under `docs/articles/` were read as product context; they
are ignored/untracked inputs, intentionally neither modified nor committed.
Page numbers below refer to PDF pages (including cover and contents), not web
pagination. These articles inform the decision, not implementation evidence or
blanket approval of their broader architecture.

- Ricardo Lüders, *Amaru, Part 2: From Vision to Architecture — Designing a
  Distributed AI-Native Production Platform* (2026-11-12), local file
  `amaru-part-2-from-vision-to-architecture-designing-a-distributed-ai-native-production-platform.pdf`:
  pp. 6–10 separate clients, intelligence and authoritative control/data;
  pp. 10–11 require durable reconstruction linking intent, process and outputs.
  This supports Platform capture and authorized reads with distinct metadata
  and Artifact bytes. pp. 17–18 explicitly frame the architecture as direction
  to validate incrementally; Brain, vector storage and distributed environments
  do not become M3 implementation requirements through this acceptance.
- Ricardo Lüders, *Amaru: Why I’m Building an Engineering Execution Platform in
  the Age of AI Agents* (2026-10-29), local file
  `amaru-why-i-m-building-an-engineering-execution-platform-in-the-age-of-ai-agents.pdf`:
  pp. 6–8 separate reasoning from enforced state, permissions and human approval;
  pp. 13–14 distinguish a final agent message from attributable execution history;
  pp. 21–23 connect Evidence and cost to accepted outcomes and call for a smaller,
  validated core. Therefore Usage is a bounded first slice, not full ART-002
  acceptance; the later public/UI audit path must preserve gaps and Approval
  identity rather than equating activity or terminal state with success.
