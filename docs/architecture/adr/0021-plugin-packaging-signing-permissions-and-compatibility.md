---
title: Plugins are signed OCI-distributed packages with declared permissions and engine ranges
status: accepted
date: 2026-10-25
deciders: Ricardo Lüders
---

# 0021. Plugins are signed OCI-distributed packages with declared permissions and engine ranges

## Context
- Question: [Q7](../OPEN_QUESTIONS.md#q7-plugin-distribution-signing-permissions-and-compatibility). Product-owner answer: "SDK + API versioning. Not sure, we need to investigate." The investigation is this ADR.
- Verified 2026-10-25: cosign v3.1.4 supports keyless signing (OIDC identity) and offline verification of protobuf bundles with a trusted root; private PKI via `--certificate-chain`/`--ca-roots` (https://docs.sigstore.dev/cosign/verifying/verify/). ORAS v1.3.6 (Apache-2.0) pushes and pulls arbitrary OCI artifacts with custom media types (https://oras.land/docs/).
- Weave installs Providers per user in `<stateDir>/providers/` with no signature and no compatibility check.
- Contract versioning: [ADR-0025](0025-api-and-protocol-versioning-policy.md).

## Decision
- **Manifest** `amaru-plugin.json`, schema `amaru.plugin/v1`: id (reverse-DNS), version (semver), publisher, components with targets ([ADR-0012](0012-plugin-package-with-targeted-components.md)), declared **permissions** (Capabilities, API scopes, Event subjects, network egress hosts for `service`/adapters), and **`engines`**: semver ranges for the Platform API major and the plugin SDK major.
- **Package**: a deterministic `.tar.zst` with manifest, components and an SBOM; distributed as an **OCI artifact** (pushed/pulled with ORAS-compatible tooling) or as a file.
- **Signing**: every package has a cosign signature bundle. Public publishers sign keyless; self-hosted and air-gapped operators can use their own keys or private PKI and verify offline. The Platform verifies the signature and the publisher identity against the Organization's trust list before install.
- **Unsigned packages** install only when an Organization admin enables developer mode for that Organization; they are marked in the UI and audit log.
- **Permissions**: shown before install and granted by a human per Organization as Grants ([ADR-0006](0006-human-authorization-for-consequential-actions.md), [ADR-0037](0037-one-policy-evaluates-capabilities-grants-and-approvals.md)); an upgrade that adds permissions needs a new grant.
- **Compatibility**: the Platform refuses to enable a plugin whose `engines` range excludes it; Workflows referencing a disabled plugin fail validation before execution, never silently.
- A public registry/marketplace is out of scope until M10; it will serve the same OCI artifacts.

## Consequences
- Supply-chain controls work online and offline with standard tooling.
- Plugin authors need a signing step in their build; the SDK CLI provides it.
- Validation (M6): sign, distribute via a local OCI registry, verify offline, reject tampered and out-of-range packages.

## Alternatives considered
- **Vendor-managed keys only**: blocks self-hosted publishers.
- **npm-style registry**: no signatures by default, Node-centric.
- **No compatibility ranges**: silent breakage on upgrade.

## Approval
Proposed by: Claude (open-question resolution, 2026-10-25) · Approved by: Ricardo Lüders on 2026-10-25, product-owner planning session (answer recorded in OPEN_QUESTIONS Q7; investigation delegated).
