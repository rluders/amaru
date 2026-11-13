# Linux PRJ-002 Artifact access validation — 2026-11-12

## Scope and evidence

This #191 slice implements ADR-0062's Project ownership boundary for direct runtime Artifact version/content queries. Base: #18 integration `5710444`, following accepted ADR-0065 in merged PR #344. Linux is required; other OSes remain best effort. No predecessor source was inspected or imported.

Changed runtime: `services/platform/internal/coordinator/project_artifact.go`; regression: `project_artifact_test.go` in the same directory. Both use existing ProjectAccess, ExecutionStore, ArtifactIndex and BlobStore contracts. No callers are migrated: there was no direct Artifact application boundary or serving product Artifact handler. Trusted ingestion and Evidence readers retain their existing internal interfaces.

The regression uses two fake-attached Projects named identically, separate authoritative Execution records, an immutable Artifact index and content-addressed blobs. Both Projects deliberately use `report` as the Artifact ID. It verifies only owned metadata/content is returned, mixed histories preserve owned version numbers, and foreign-only, missing, orphaned or unknown versions expose no result. A nil BlobStore proves rejected version selectors never reach content IO. Probes establish authentication before index IO, revocation after index read, closure of an opened reader on revocation, storage failure without partial metadata/content and unchanged version history.

## Architecture review

Verdict: **sound within accepted ADR-0062**. Platform state stays authoritative; Clients receive no raw stores and Workers gain no state. Existing Instance storage is a trusted composition dependency. No domain import, infrastructure, dependency, permission, trust, migration or license change. This is read-only, so no new transaction, duplicate-write, crash-recovery or Event ordering semantics arise. Existing global version numbers are retained; filtering is not renumbering or concealment of numbering gaps. Organization membership remains a separate gate before multi-Organization exposure.

The smaller option of trusting an Artifact ID or a caller-supplied hash cannot establish Project ownership. This boundary instead derives ownership from the persisted producing Execution and content identity from its indexed version. A returned reader is owned by its authorized caller; ongoing stream revocation is a future transport concern.

## Commands and outcomes

- `cd services/platform && go test -race -count=1 -run TestProjectArtifactAccess -v ./internal/coordinator`: **PASS**, exit 0; parent and seven subtests, no skips. Fake-backed application integration with the race detector.
- `make check`: **PASS**, exit 0, on this candidate. Includes 42 SDK tests, 348 TypeScript contract tests, 346 matching Go/TypeScript fixture outcomes, Platform tests, vet and documentation checks. Log: `/tmp/amaru-191-artifact-check.log`.
- `make check` on merged #344 (`5710444`): **PASS**, exit 0. Log: `/tmp/amaru-344-merged-check.log`.
- `git diff --cached --check`: **PASS**.
- `trail check --base origin/feat/18-wp-m3-06-public-api-events`: **PASS**, no conflicts. Lexical advisories concern historical GitHub fields, Linux scope, PostgreSQL limits and a previous fixture-runner failure; none changes those decisions or claims unrun validation.

Local results are not hosted CI. No PostgreSQL runtime or native non-Linux validation is claimed by cross-target vet.

## Limits

Fake-backed application integration only. No product HTTP/SDK Artifact route, real browser/Provider, PostgreSQL/filesystem blob integration, process restart or native macOS/Windows run. No durable storage semantics changed, so PostgreSQL checks are not required for this slice. Public schemas still describe Project-relative notes; runtime versioned-Artifact mapping remains a contract/composition gate. Full Project Work/dependencies, exact-scope secrets and encrypted authorized delivery, definition loading, clone lookup and composed product acceptance remain open. #191, #18 and M3 remain open.
