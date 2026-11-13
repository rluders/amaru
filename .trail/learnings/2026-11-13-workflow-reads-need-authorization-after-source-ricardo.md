---
title: Workflow reads need authorization after source IO
author: Ricardo
date: 2026-11-13
branch: feat/191-persisted-workflow-clone
id: l-34fbbdc3
scope: repo
status: active
src: agent
touches:
  - docs/engineering/project-access.md
  - services/platform/internal/attach/workflow.go
  - services/platform/internal/attach/workflow_test.go
  - services/platform/internal/coordinator/project_workflow.go
  - services/platform/internal/coordinator/project_workflow_test.go
  - services/platform/internal/store/postgres/project_clone_pg_test.go
---

## Decided
In services/platform/internal/coordinator/project_workflow.go, recheck current Project authorization after ProjectWorkflowSource.Workflow returns, before exposing a definition or source error. RunWorkflow's later Submit check protects creation but cannot protect direct Workflow queries.

## Tried and ruled out
Relying only on RunWorkflow -> Submit authorization leaves direct Workflow reads able to return loaded content after logout/expiry during Git or clone-store IO. Rechecking only before lookup cannot observe revocation during the read.

## Watch out
This is a read boundary, not immutable Execution clone provenance. services/platform/internal/attach/workflow.go captures one stored binding, but Execution storage/recovery still needs its own provenance implementation under ADR-0066. Keep lookup dependencies trusted and never take a replacement path from a public request.
