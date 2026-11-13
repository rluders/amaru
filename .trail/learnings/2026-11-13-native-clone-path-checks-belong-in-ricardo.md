---
title: "Native clone path checks belong in attach, not storage"
author: Ricardo
date: 2026-11-13
branch: feat/191-clone-binding
id: l-a36b40d4
scope: repo
status: active
src: agent
touches:
  - docs/engineering/project-access.md
  - services/platform/internal/attach/attach.go
  - services/platform/internal/attach/attach_test.go
  - services/platform/internal/ports/fakes/project.go
  - services/platform/internal/store/migrations/sql/migrations.sum
---

## Decided
Keep native root validation and symlink canonicalization in services/platform/internal/attach/attach.go. ProjectCloneStore receives a trusted canonical root and checks only record shape (matching Project key, nonempty and NUL-free text). The Platform store owns the binding but does not inspect the filesystem or authorize callers.

## Tried and ruled out
Using filepath.IsAbs/Clean in fake and PostgreSQL clone stores looked like harmless lexical validation, but internal/arch/arch_test.go TestStorageDoesNotTouchFiles rejects the path/filepath import for all storage packages, including internal/ports/porttest helpers. Do not weaken that rule or duplicate native path normalization with string heuristics. Existing Project-only stores/callers remain usable; attach requires atomic RegisterClone rather than split Create/Bind.

## Watch out
A stored root is neither current filesystem identity nor authorization. ADR-0066 still requires authorized lookup, identity revalidation and immutable Execution location provenance in follow-up composition. Internal relative-path normalization does not satisfy the public attach contract's absolute-root requirement. Storage tests validate records; real Git tests exercise native path inspection.
