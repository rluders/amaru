---
title: "Clone selection is provenance, not command intent"
author: Ricardo
date: 2026-11-13
branch: feat/191-execution-clone
id: l-6f491232
scope: repo
status: active
src: agent
touches:
  - docs/engineering/command-idempotency.md
  - docs/engineering/project-access.md
  - services/platform/internal/adapter/localauth/workflow_test.go
  - services/platform/internal/attach/workflow.go
  - services/platform/internal/attach/workflow_test.go
  - services/platform/internal/coordinator/command.go
  - services/platform/internal/coordinator/coordtest/definition.go
  - services/platform/internal/coordinator/project_workflow.go
  - services/platform/internal/coordinator/project_workflow_test.go
  - services/platform/internal/domain/definition.go
---

## Decided
In services/platform/internal/coordinator/command.go, omit the trusted DefinitionPin.CloneRoot from the Submit fingerprint while persisting it in the original atomic outcome. Reattaching a Project changes server selection, not the caller's requested Workflow/revision/variables. An identical retry must retain its original Execution and clone; new command keys capture the newly selected clone. docs/engineering/command-idempotency.md records the distinction.

## Tried and ruled out
Hashing the whole pin including CloneRoot would turn an unchanged request into ErrKeyReused merely because another authenticated attach selected a clone. Looking up ActiveClone again after reading the Workflow can instead pin the wrong source during concurrent reattach. ports.ProjectWorkflow returns definition and captured clone together to avoid that race.

## Watch out
RunWorkflow still loads and authorizes requested content before command replay, so an unavailable active revision fails closed; durable Execution definition recovery does not read Git. CloneRoot is compact-Instance operational provenance, not remote Worker placement or a public root override. No pre-v1 schema upgrade/backfill is added.
