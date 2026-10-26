---
title: Configuration resolves deployment, Organization, Project and user scopes in a fixed order
status: accepted
date: 2026-10-25
deciders: Ricardo Lüders
---

# 0039. Configuration resolves deployment, Organization, Project and user scopes in a fixed order

## Context
- Question: [W4](../OPEN_QUESTIONS.md#w4-configuration-scopes) (prototype Q8). Resolved under the session's delegation.
- the prototype: the project `config.yaml` holds only `schemaVersion: 1`; provider connections and MCP servers are per user; there is no resolution order.

## Decision
- Scopes, lowest to highest precedence:
  1. **Deployment** — environment variables and the Platform config file (ports, database, NATS, storage, KMS). Set by the operator.
  2. **Organization** — stored in the Platform (Policy rules, Provider connections, model defaults, retention, Agents/Skills shared across Projects).
  3. **Project** — `.amaru/config.yaml` committed in the Project ([ADR-0027](0027-project-definitions-in-git-operational-state-in-platform.md)): Workflow defaults, Local Checks, Knowledge globs, Project Agents.
  4. **User preferences** — UI-only settings (theme, notifications); never affect execution.
- The most specific scope wins, except keys a higher scope marks **locked** (e.g. an Organization forbids a Provider); security-relevant keys (Policy, egress, retention) can only be tightened by lower scopes.
- Organization and Project Agents/Skills coexist; a Project definition with the same id overrides the Organization one for that Project.
- **Secrets never appear in Project configuration**; configuration refers to them by name.
- The effective configuration and the scope of each value are visible in the UI and recorded in Execution Evidence.

## Consequences
- Predictable behavior across clones and deployments; reviewable Project settings.

## Alternatives considered
- **Project-first with no locks**: a repository could weaken Organization security.

## Approval
Proposed by: Claude (open-question resolution, 2026-10-25) · Approved by: Ricardo Lüders on 2026-10-25, product-owner planning session (resolution delegated to Claude; accepted in the same session).
