---
title: "SQL driver panics require bounded process failure, not a graceful cleanup claim"
author: Ricardo
date: 2026-11-13
branch: feat/18-storage-lifecycle
id: l-2c82b2cb
scope: repo
status: active
src: agent
touches:
  - docs/architecture/ARCHITECTURE.md
  - docs/engineering/repository-layout.md
  - docs/engineering/validation.md
  - services/platform/README.md
---

## Decided
In services/platform/internal/platform/storage_test.go, keep SQL driver panic cases in disposable subprocesses. Assert the panic actually ran, no dependent stage starts, Lane reports failure under a cancellation deadline and shutdown budget, and canary diagnostics stay absent. These cases establish failure containment, not repaired driver state or graceful connection cleanup. Production still needs ADR-0068's independent aggregate startup watchdog and nonzero exit.

## Tried and ruled out
An in-process fake Pinger panic did not close its physical connection; a QueryContext panic stranded migrations.Runner.Apply's deferred sql.Conn.Close and prevented the panic reaching Lane. Go 1.27 database/sql pingDC/queryDC release connections after driver calls; a panic may bypass release. Expecting Lane recovery alone to unwind SQL internals failed both tests. Do not add retries, a second supervisor, or compatibility code to hide this.

## Watch out
services/platform/internal/platform/storage.go keeps pools alive until Stop and joins schema completion within its cleanup context. That protects normal errors/cancellation, not a broken driver's internals. Initializer deadlines do not independently cancel the outer Lane if an initializer is stranded in a deferred close. Always retain the aggregate watchdog, bounded failure exit and the distinction between owner recreation and real process restart.
