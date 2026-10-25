---
title: Artifacts live in a content-addressed store on the filesystem or S3-compatible storage
status: accepted
date: 2026-10-25
deciders: Ricardo Lüders
---

# 0017. Artifacts live in a content-addressed store on the filesystem or S3-compatible storage

## Context
- Question: [Q4](../OPEN_QUESTIONS.md#q4-artifact-and-object-storage-strategy). Product-owner answer: "RustFS is a good alternative, 'cause it can handle artifacts versions/history."
- Verified 2026-10-25: RustFS is Apache-2.0, GA since 1.0.0 (2026-09-16), latest 1.0.4; versioning, object lock and replication listed as available; a Windows x86_64 binary is published but Windows support is not stated in the README (https://github.com/rustfs/rustfs). MinIO's repository is archived and no longer maintained (https://github.com/minio/minio). SeaweedFS is Apache-2.0; Garage is AGPL-3.0.
- Weave stores generated files in the Project tree or a per-user `asset-store/`; the Weave prototype copies whole workspaces to `ARTIFACT_ROOT` with no versions or hashes.
- Compact installs must work without an object-storage service ([ADR-0015](0015-postgresql-is-the-only-database-bundled-in-compact-installs.md) NFR-34 restatement).

## Decision
- The Platform stores Artifact content through an **`ArtifactStore` port**, addressed by SHA-256 of the content. Blobs are immutable.
- **Versions and history are Platform data** in PostgreSQL: an Artifact has a stable id, and each version row records the content hash, size, media type, producing Step Execution and creation time. History does not depend on bucket features.
- Adapters:
  - **Filesystem** (compact default): a content-addressed directory under the data directory.
  - **S3-compatible**: any S3 API store. **RustFS** is the reference backend used in the self-hosted compose file, in tests and in Cloud; bucket versioning and object lock are enabled as defense in depth.
- Generated Artifacts are **never written into the Project tree** unless a human or an approved Step explicitly promotes them (as a Git change).
- Remote Workers upload through the Platform with chunked, resumable HTTP uploads, verified against the declared hash; with the S3 adapter the Platform issues short-lived presigned URLs instead.
- Keys are prefixed by Organization ([ADR-0022](0022-cloud-tenancy-row-level-isolation-by-organization.md)). Retention is a Policy setting per Organization; blobs are garbage-collected only when no version references them.

## Consequences
- Local installs need no object store; server and Cloud scale storage independently of the database.
- Hashes give audit-grade Evidence and deduplicate identical outputs.
- Validation (M5): a remote Worker uploads a multi-GB output over an interrupted connection; the hash verifies and resumes. Self-hosted compose runs against RustFS in CI (M7).
- Revisit if: RustFS shows S3-compatibility gaps for multipart upload or presigned URLs; any other S3-compatible store can replace it without code change.

## Alternatives considered
- **RustFS mandatory everywhere**: would add a service to compact installs and tie history to one product's versioning.
- **Project tree / Git LFS**: fills user repositories; LFS stays an option only through explicit promotion.
- **MinIO**: no longer maintained as open source.

## Approval
Proposed by: Claude (open-question resolution, 2026-10-25) · Approved by: Ricardo Lüders on 2026-10-25, product-owner planning session (answer recorded in OPEN_QUESTIONS Q4; RustFS kept as the reference S3 backend).
