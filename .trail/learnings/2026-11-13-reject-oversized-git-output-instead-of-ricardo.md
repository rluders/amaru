---
title: Reject oversized Git output instead of parsing a truncated prefix
author: Ricardo
date: 2026-11-13
branch: feat/191-project-inspection
id: l-5982b974
scope: repo
status: active
src: agent
touches:
  - services/platform/internal/attach/attach.go
---

## Decided
In services/platform/internal/attach/attach.go, Attacher.run reads maxOutput+1 stdout bytes, drains remaining output and waits for the process, then rejects overflow. The old LimitReader(maxOutput) silently returned a successful prefix. A committed config consisting of more than 1 MiB of YAML comments was accepted as empty valid config. TestCommittedConfigInspection/oversized reproduces that acceptance with the old limit and passes with explicit overflow rejection.

## Tried and ruled out
A memory cap alone is not input validation: valid YAML at the start of a longer blob can hide the suffix. Increasing the limit or relying on YAML parse errors does not fix this. Keep draining stdout/stderr and waiting; returning immediately on overflow could leave a blocked child process. Config ls-tree and cat-file now use one captured HEAD object so a changed HEAD cannot switch the checked file mode before reading bytes.

## Watch out
Attacher.run is shared by identity, attach and Workflow reads; keep their tests in the validation scope. The output cap is per command, not a total process-time guarantee, and caller cancellation is still required. Inspection is internal and unauthenticated; it neither selects a persisted binding nor redacts errors for HTTP. Capturing a commit for config does not establish immunity to concurrent filesystem replacement or atomic identity inspection across all Git commands.
