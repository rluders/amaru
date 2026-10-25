---
title: Project definitions live in Git under .amaru/; operational state lives in the Platform
status: superseded
date: 2026-10-25
deciders: Ricardo Lüders
superseded-by: 0043
---

# 0027. Project definitions live in Git under .amaru/; operational state lives in the Platform

## Context
- Question: [Q13](../OPEN_QUESTIONS.md#q13-project-metadata-location). Product-owner answer: "You can choose the best approach; no need to keep compatibility with Weave."
- Weave commits `.weave/{project.yaml,config.yaml,agents,workflows,teams,work,trust.json,settings.json}` and ignores `.weave/runtime/` (executions, Autoloop ledgers, worktrees). Committing Work Items and `trust.json` caused clone-level conflicts (a clone can import trust; ids are derived from files on disk).
- Remote and Cloud Platforms cannot write to a user's disk; Workflow and Agent definitions benefit from code review.

## Decision
- **Committed in the Project, under `.amaru/`** (reviewed through Git): Workflows, Agents, Teams, Workflow templates, Project configuration (`.amaru/config.yaml`, [ADR-0039](0039-configuration-scopes-and-resolution-order.md)), Local Check definitions, references to Skills and Playbooks (which stay wherever the Project keeps its Knowledge).
- **In the Platform's PostgreSQL, keyed by Project**: Work Packages, Work Items, Executions, Step Executions, Events, Approvals, Grants, trust decisions, delivery-loop ledgers, usage, Artifacts metadata, secrets.
- Every Execution records the **definition revision**: commit SHA (when clean) plus a content hash of the definitions used.
- A local Platform reads definitions from the working tree (uncommitted edits allowed, flagged as "dirty" in Evidence); remote and Cloud Platforms read them by Git fetch at a commit.
- Nothing Amaru writes at runtime lands in the Project tree; `.amaru/` contains no runtime directory.
- No `.weave/` compatibility. A one-shot `amaru import weave` (definitions and Work Items) is a roadmap task, not a runtime feature.

## Consequences
- Two clones of a repository share definitions and never conflict on operational state.
- Work Items become multi-user Platform data with ids that do not collide.
- Users who want Work Items in Git get export, not storage.
- Validation (M3): two clones against one Platform; Cloud reads definitions at a commit.

## Alternatives considered
- **Everything in the Project (Weave)**: conflicts across clones, impossible for Cloud.
- **Everything in the Platform**: loses review of definitions in Git.

## Approval
Proposed by: Claude (open-question resolution, 2026-10-25) · Approved by: Ricardo Lüders on 2026-10-25, product-owner planning session (decision delegated to Claude in OPEN_QUESTIONS Q13 and accepted in the same session).
