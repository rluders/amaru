---
title: Coordinator session shutdown must join its receive pump
author: Ricardo
date: 2026-11-13
branch: feat/18-execution-lifecycle
id: l-f47c8b70
scope: repo
status: active
src: agent
touches:
  - docs/architecture/ARCHITECTURE.md
  - docs/engineering/repository-layout.md
  - docs/engineering/validation.md
  - services/platform/README.md
  - services/platform/internal/coordinator/serve.go
  - services/platform/internal/platform/README.md
---

## Decided
services/platform/internal/coordinator/serve.go must cancel and join its receive pump before Serve returns. Session lifecycle Stop relies on that completion to release the in-process link only after Worker attempts and Coordinator protocol work have joined. Capture receive errors through the join so unrelated errors are not lost if cancellation wins select; catch pump panics locally and return a static failure without formatting the panic value.

## Tried and ruled out
The original defer stop() only cancelled the child pump. TestServeJoinsReceivePump in services/platform/internal/coordinator/receive_test.go reproduced Serve returning while Receive still waited for its controlled release. Wrapping Serve in Lane does not join a goroutine Serve itself abandons. Closing the session link on Start cancellation would also release a dependency before Worker cleanup finishes.

## Watch out
A Receive implementation must honor context cancellation (ports.Link's existing contract). A broken adapter can still strand Serve; Lane's shared shutdown budget bounds the process-level wait, and the eventual entrypoint must exit on failure. This is not graceful completion or an in-process restart path. Worker Provider panics remain per-attempt failures under existing Worker semantics; critical session/transport failures are lifecycle failures. No synthetic Result or domain Cancel is added by process shutdown.
