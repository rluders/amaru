---
title: Cloud tenancy is shared infrastructure with row-level isolation per Organization
status: accepted
date: 2026-10-25
deciders: Ricardo Lüders
---

# 0022. Cloud tenancy is shared infrastructure with row-level isolation per Organization

## Context
- Question: [Q8](../OPEN_QUESTIONS.md#q8-cloud-tenant-isolation). Product-owner answer: "Row-level isolation. Every tenant has its own Workspace with multiple projects; this is how we scope secrets and artifacts." The tenant is the [glossary](../../product/GLOSSARY.md)'s **Organization** (the product owner's "Workspace"); the term Workspace stays unused.
- Verified from the PostgreSQL 18 docs (2026-10-25): enabling RLS with no policy denies all; table owners bypass RLS unless `FORCE ROW LEVEL SECURITY`; `set_config(name, value, true)` lasts for the transaction; `current_setting(name, true)` returns NULL when unset. Session-level settings leak across clients behind transaction pooling.

## Decision
- Cloud runs **shared Platform instances on a shared PostgreSQL** cluster. Every tenant-owned table has `organization_id`.
- Isolation is enforced twice:
  - **Application**: every repository call takes an Organization scope from the authenticated principal.
  - **Database**: RLS policies `organization_id = current_setting('app.organization_id', true)::uuid` on every tenant table, `FORCE ROW LEVEL SECURITY`, the application connects as a non-owner role without `BYPASSRLS`, and the tenant is set with `set_config(..., true)` at the start of every transaction. Migrations run as a separate owner role.
- **Secrets**: envelope encryption; one data key per Organization, wrapped by a KMS key; secrets are decrypted only to deliver them to a Lease ([ADR-0024](0024-remote-worker-registration-identity-and-delivery.md)).
- **Artifacts**: keys prefixed by Organization id in the S3 store ([ADR-0017](0017-content-addressed-artifact-store-filesystem-or-s3.md)); presigned URLs scoped to one object.
- **NATS**: subjects prefixed by Organization; Worker gateway credentials limited to their Organization's subjects.
- **Workers**: vendor-run Workers serve one Organization per Lease and are reset between Organizations; customer-run Workers are the default for access to private code.
- Self-hosted multi-Organization deployments use the same mechanism.

## Consequences
- Lowest cost per tenant; one schema to migrate.
- A missing `organization_id` filter is caught by RLS; tests must assert cross-tenant denial for every table.
- Large tenants needing physical isolation get a dedicated Cloud deployment of the same image (commercial option), not a different architecture.
- Validation (M9): cross-tenant penetration test and RLS test suite.

## Alternatives considered
- **Schema or database per tenant**: stronger isolation, migration and connection fan-out cost.
- **Instance per tenant**: highest cost; kept only as a dedicated-deployment option.

## Approval
Proposed by: Claude (open-question resolution, 2026-10-25) · Approved by: Ricardo Lüders on 2026-10-25, product-owner planning session (answer recorded in OPEN_QUESTIONS Q8; tenant term Organization confirmed in the same session).
