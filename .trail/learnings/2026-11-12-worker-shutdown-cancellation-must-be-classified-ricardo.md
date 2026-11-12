---
title: Worker shutdown cancellation must be classified at the Serve boundary
author: Ricardo
date: 2026-11-12
branch: fix/339-worker-shutdown
id: l-ac423085
scope: repo
status: active
src: agent
touches:
  - services/platform/internal/coordinator/serve.go
---

## Decided
Normalize errors caused by a Worker session's own context at the Coordinator.Serve boundary in services/platform/internal/coordinator/serve.go. Require both ctx.Err() and errors.Is(returnedError, ctx.Err()); unrelated store/transport failures must remain visible even during cancellation.

## Tried and ruled out
A passing rerun of PostgreSQL concurrency-limit tests did not establish correctness. Issue #339 was independently reproduced on unchanged integration aca5091, four failures in twenty runs. Deterministic fake store callbacks reproduce cancellation during assignment and all five message handlers without PostgreSQL or sleeps.

## Watch out
Handling cancellation only in the Receive select misses store operations already executing. Conversely, ignoring every send error whenever ctx.Err() is non-nil hides genuine transport failures. Do not suppress failures in coordtest/startSession or infer a durable commit from a nil shutdown return; existing protocol durability and recovery tests remain required.
