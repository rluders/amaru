---
title: Linux is the required acceptance platform for remaining M3 including exit
author: Ricardo
date: 2026-11-12
branch: feat/129-exe-002-durable-state-and-restart
id: l-3cca1934
scope: repo
status: active
src: agent
touches:
  - docs/architecture/RESILIENCE.md
  - docs/architecture/ROADMAP.md
  - docs/architecture/WORK_PACKAGES.md
  - docs/architecture/adr/0042-durable-execution-with-explicit-restart-semantics.md
  - docs/engineering/backlog-map.md
  - docs/engineering/validation.md
  - services/platform/internal/coordinator/coordtest/crash.go
---

## Decided
Ricardo explicitly selected Linux-first acceptance for all remaining M3 work, including exit #20, in the 2026-11-12 #129 planning session. macOS and Windows are best effort, with missing native evidence recorded as not validated. docs/engineering/validation.md and docs/architecture/ROADMAP.md record the decision.

## Tried and ruled out
Treating the 2026-10-28 Work Package deferral as requiring macOS/Windows runs before M3 exit no longer reflects the owner's approved scope.

## Watch out
This exception is M3 only. M4 native validation requirements and security/confinement restrictions remain unchanged. Cross-compilation is never native evidence.
