---
title: Project definitions live in Git under .amaru/; Team membership and operational state live in the Platform
status: accepted
date: 2026-10-25
deciders: Ricardo Lüders
supersedes: 0027
---

# 0043. Project definitions live in Git under .amaru/; Team membership and operational state live in the Platform

## Context
- [ADR-0027](0027-project-definitions-in-git-operational-state-in-platform.md) answered [Q13](../OPEN_QUESTIONS.md#q13-project-metadata-location); the product owner delegated it: "You can choose the best approach; no need to keep compatibility with Weave". It listed **Teams** among the definitions committed under `.amaru/`.
- [ADR-0038](0038-roles-teams-and-separation-of-duties.md) later made Teams sets of humans and Agents that **gate Approvals** (required reviewer Team, separation of duties). Keeping that membership in Git would let anyone with write access to the repository add themselves to a reviewer Team. Authorization data would then follow repository permissions instead of Organization roles ([Q27](../OPEN_QUESTIONS.md#q27-where-teams-live)).
- Weave's `.weave/teams` holds agent compositions, not human authorization.
- Delegated in the 2026-10-25 planning prompt.

## Decision
- **Committed in the Project, under `.amaru/`** (reviewed through Git):
  - Workflows, Agents and Workflow templates.
  - Project configuration (`.amaru/config.yaml`, [ADR-0039](0039-configuration-scopes-and-resolution-order.md)).
  - Local Check definitions.
  - References to Skills and Playbooks.
  - **References to Teams by slug**, e.g. a gate's required reviewer Team or a Workflow's implementer Team.
- **In the Platform's PostgreSQL**:
  - **Teams**, Organization-scoped: name, slug, members (humans and Agents) and team roles. They are managed by Owners and Admins through the public API.
  - Keyed by Project: Work Packages, Work Items, Executions, Step Executions, Events, Approvals, Grants, trust decisions, delivery-loop ledgers, usage, Artifact metadata and secrets.
- A definition that references an unknown Team slug fails validation and does not start.
- The rest of ADR-0027 is unchanged:
  - Every Execution records the definition revision: the commit SHA when the tree is clean, plus a content hash.
  - A local Platform reads the working tree, and dirty edits are flagged in Evidence. Remote and Cloud Platforms fetch at a commit.
  - Nothing Amaru writes at runtime lands in the Project tree.
  - There is no `.weave/` compatibility, only a one-shot `amaru import weave`.
- The importer maps Weave `teams/` (agent compositions) to Platform Teams that hold the same Agents, after the user confirms.

## Consequences
- Repository write access never grants approval authority.
- Two clones of a repository share definitions and never conflict on operational state. This is validated at M3 with two clones against one Platform; Cloud reads at a commit.
- Team changes are audited as Platform Events, not Git commits.

## Alternatives considered
- **Teams in Git (ADR-0027 as written)**: reviewable, but it ties authorization to repository permissions.
- **Split: agent-only Teams in Git, human Teams in the Platform**: two Team concepts for one glossary term.

## Approval
Proposed by: Claude (second-pass open-question resolution, 2026-10-25) · Approved by: Ricardo Lüders on 2026-10-25, planning prompt that delegated every remaining decision (status "accepted, delegated" confirmed in the same session).
