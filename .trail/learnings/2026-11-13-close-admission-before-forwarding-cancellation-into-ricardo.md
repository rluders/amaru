---
title: Close admission before forwarding cancellation into Lane
author: Ricardo
date: 2026-11-13
branch: feat/18-startup-watchdog
id: l-73c141f1
scope: repo
status: active
src: agent
touches:
  - docs/architecture/ARCHITECTURE.md
  - docs/engineering/validation.md
  - services/platform/README.md
  - services/platform/internal/platform/README.md
  - services/platform/internal/platform/storage_test.go
---

## Decided
In services/platform/internal/platform/lifecycle.go, derive Lane's run context with context.WithoutCancel and forward caller cancellation only after closing the atomic readiness latch. Preserve context values, classify caller deadline expiry as failure, and leave signal ownership to Lane. A failed RunnerExited event closes the latch before errgroup cancellation; LaneStopping closes it for signals and teardown.

## Tried and ruled out
Passing a direct child of the caller context allowed Start to observe cancellation while Ready still returned true. TestLifecycleHealthyLifetimeAndShutdownLatch reproduced this on all ten initial race runs: the Lane stopping observer executes after cancellation has already propagated through parent contexts. Merely relying on that observer cannot order external cancellation.

## Watch out
Stop the startup timer after aggregate readiness and before forwarding intentional cancellation. Watchdog expiry must remain failure even if Lane returns nil for cancellation. This helper cannot kill abandoned goroutines; the eventual entrypoint must exit on failure, and child owners must prevent orphans. This does not add in-process restoration or replace resource-owner failure classification.
