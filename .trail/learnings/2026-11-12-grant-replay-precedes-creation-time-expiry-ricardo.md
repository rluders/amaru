---
title: Grant replay precedes creation-time expiry validation
author: Ricardo
date: 2026-11-12
branch: feat/157-command-idempotency
id: l-a8c9a319
scope: repo
status: active
src: agent
touches:
  - docs/architecture/RESILIENCE.md
  - docs/architecture/adr/0058-command-idempotency-conflicts-and-retention.md
  - packages/contracts/README.md
  - packages/contracts/fixtures/api/v1/CreateExecution/invalid-both-inputs.json
  - packages/contracts/fixtures/api/v1/CreateExecution/invalid-capability.json
  - packages/contracts/fixtures/api/v1/CreateExecution/invalid-empty-artifact-id.json
  - packages/contracts/fixtures/api/v1/CreateExecution/invalid-empty-prompt.json
  - packages/contracts/fixtures/api/v1/CreateExecution/valid-asset-generate.json
  - packages/contracts/fixtures/api/v1/CreateExecution/valid-with-provider.json
  - packages/contracts/fixtures/api/v1/CreateExecution/valid-with-work-item.json
---

## Decided
In services/platform/internal/coordinator/policy.go, authorize CreateGrant and fingerprint caller-owned fields before replay. Run domain.NewGrant with the current clock only when no command record exists. ADR-0058 requires an expired or revoked Grant to replay its original creation outcome.

## Tried and ruled out
Normalization through NewGrant before lookup violates that guarantee: Grant.Validate compares ExpiresAt with CreatedAt, so replacing original creation time with retry time can reject an unchanged request after expiry. Merely excluding creation time from the fingerprint cannot fix this ordering error.

## Watch out
Check current authority before disclosing a replay and preserve expiry validation for new commands. coordtest.IdempotencySuite tests an expiring Grant with a one-year clock advance against fake stores and PostgreSQL, alongside conflicting actor and payload rejection.
