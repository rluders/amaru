# Architecture Decisions

Amaru records significant decisions as Architecture Decision Records (ADRs) in [`adr/`](adr/). This file is the convention and the index. The convention itself is [ADR-0001](adr/0001-record-architecture-decisions.md).

## Convention

- **File:** `adr/NNNN-lowercase-title.md`, numbered sequentially, never renumbered or reused. Start from the [template](adr/0000-template.md).
- **Frontmatter** (single-line `key: value` pairs, checked by `make docs-check`):
  - `title`: the decision, as a short phrase
  - `status`: one of the statuses below
  - `date`: `YYYY-MM-DD` of the last status change
  - `deciders`: who approved it — **required when `accepted`**
  - optional: `supersedes`, `superseded-by` (ADR numbers)
- **Statuses:**
  | Status | Meaning |
  |---|---|
  | `proposed` | Written and recommended; not approved. Nothing may be built *only* on its authority. |
  | `under-investigation` | Evidence is being gathered (spike, research). |
  | `accepted` | Approved by a named human decider. Recorded where and when. |
  | `rejected` | Considered and declined; kept for the record. |
  | `superseded` | Replaced by a later ADR (`superseded-by`). |
  | `deprecated` | No longer applies, not replaced. |
- **Acceptance rule:** an ADR becomes `accepted` only with an explicit, recorded human approval (who, when, where). An agent never marks its own proposal accepted. Asking for an investigation is not approval of its outcome.
- **Change rule:** an accepted ADR is not edited in substance; write a new ADR that supersedes it. Typos and links may be fixed.
- **Index:** every ADR has exactly one row below, whose status matches the file (`make docs-check` enforces this).
- **Scope:** significant, hard-to-reverse choices (boundaries, contracts, dependencies on infrastructure, security model, licensing). Unresolved questions live in [OPEN_QUESTIONS](OPEN_QUESTIONS.md) until there is enough evidence for an ADR. Procedure: the `adr-authoring` Skill (`.agents/skills/adr-authoring/SKILL.md`).

## Index

| ADR | Title | Status | Date |
|---|---|---|---|
| [0001](adr/0001-record-architecture-decisions.md) | Record architecture decisions as ADRs | accepted | 2026-10-24 |
| [0002](adr/0002-one-platform-multiple-deployment-models.md) | One Platform, multiple deployment models | proposed | 2026-10-24 |
| [0003](adr/0003-desktop-is-a-client.md) | Desktop is a client, not the owner of execution state | proposed | 2026-10-24 |
| [0004](adr/0004-shared-versioned-contracts.md) | Shared, versioned contracts across deployments and clients | proposed | 2026-10-24 |
| [0005](adr/0005-plugin-extensibility-independent-of-desktop.md) | Plugin extensibility independent of the desktop framework | proposed | 2026-10-24 |
| [0006](adr/0006-human-authorization-for-consequential-actions.md) | Explicit human authorization for consequential actions | accepted | 2026-10-24 |
| [0007](adr/0007-weave-migration-is-a-separate-activity.md) | Migration of the Weave implementation is a separate, approval-gated activity | accepted | 2026-10-24 |
| [0008](adr/0008-incremental-monorepo-layout.md) | Monorepo layout introduced incrementally, by evidence | proposed | 2026-10-24 |

**Not ADRs yet** (open, see [OPEN_QUESTIONS](OPEN_QUESTIONS.md)): Tauri v2 / Rust desktop host, durable workflow engine (Temporal or alternatives), queues, databases, object storage, plugin sandboxing and signing, tenancy, licensing.

## Relationship to Weave ADRs

The predecessor project has its own ADR-001…020 in [`weave-obsidian` `docs/decisions/`](https://github.com/rluders/weave-obsidian/tree/main/docs/decisions). They govern that codebase and are **not** Amaru decisions. They are evidence: an Amaru ADR that keeps, changes or drops a Weave decision cites it. Their status at the snapshot is in the [Weave inventory](../migration/weave-inventory.md#architecture-decisions).
