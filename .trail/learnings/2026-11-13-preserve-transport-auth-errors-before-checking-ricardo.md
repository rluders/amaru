---
title: Preserve transport auth errors before checking request cancellation
author: Ricardo
date: 2026-11-13
branch: feat/191-project-application
id: l-03a75706
scope: repo
status: active
src: agent
touches:
  - docs/engineering/project-access.md
  - services/platform/internal/attach/attach.go
  - services/platform/internal/coordinator/project.go
  - services/platform/internal/ports/project_clone.go
---

## Decided
ProjectAccess's shared administrator helper must invoke its trusted authentication callback before its own ctx.Err check. Live transport guards determine their established logout/expiry/cancellation error, and ADR-0067 explicitly preserves authentication guard errors. A callback that accepts a canceled context is still followed by ctx.Err before storage or inspection.

## Tried and ruled out
Checking ctx.Err first in services/platform/internal/coordinator/project.go passed the new fake-backed Project tests but failed make check in localauth TestAuthenticatedSecretResolution/cancellation and TestWorkflowWithholdsContentAfterRevocation/cancellation. It replaced the guard's established error with context.Canceled. Moving cancellation ahead of the transport guard changes unrelated existing Project operations too.

## Watch out
Keep post-I/O authority checks, cancellation propagation and atomic RegisterClone intact. This ordering preserves errors, not permission to access storage after cancellation, and does not make authentication and a database commit atomic. The localauth regressions cover real guard semantics that permissive application fakes alone cannot establish.
