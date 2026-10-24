---
title: Monorepo layout introduced incrementally, by evidence
status: proposed
date: 2026-10-24
---

# 0008. Monorepo layout introduced incrementally, by evidence

## Context
The target product has a Go Platform, React/TS clients, a native Desktop host, shared contracts, SDKs and UI components. Creating every directory now would produce empty packages and fake scaffolds that look like progress and rot. Weave grew successfully from one Go module and one TS package.

## Decision
Amaru is a single monorepo whose target layout is documented in [repository-layout](../../engineering/repository-layout.md). A top-level area (`apps/`, `services/`, `packages/`, `tools/`) or a package inside it is created only when it has real content, a documented purpose, an owner and a validation command wired into `make check`. Infrastructure dependencies (workflow engines, brokers, databases, object storage) are added only with an accepted ADR.

## Consequences
- The bootstrap contains docs, governance, Skills and `tools/doccheck` only.
- Each new component brings its own build/test target in the root Makefile.
- Workspace tooling (Go workspace, pnpm workspace, Cargo workspace) is introduced with the second module of that language, not before.

## Alternatives considered
- *Scaffold the full tree now*: looks complete; adds unvalidated code and maintenance.
- *Polyrepo*: separates release cadences but makes cross-cutting contract changes expensive.

## Approval
Proposed by Claude in the bootstrap. Awaiting product-owner decision. (The bootstrap follows it provisionally because the approved plan chose the same scope.)
