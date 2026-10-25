---
title: A plugin is one package of components with declared targets
status: accepted
date: 2026-10-25
deciders: Ricardo Lüders
---

# 0012. A plugin is one package of components with declared targets

## Context
- [ADR-0005](0005-plugin-extensibility-independent-of-desktop.md) makes plugins independent of the Desktop framework and out of process by default, but does not define the package shape.
- One extension often spans environments: a UI panel, logic reacting to events, a Provider on a remote Worker, an adapter on the user's machine, a backend service (PRD UC-7).
- Verified: Weave's only extension mechanism is external Providers installed per user (`<stateDir>/providers/<id>/provider.yaml`, `weave.provider/v1`); MCP servers are configured per user (Weave ADR-016).
- Deployments differ: Cloud cannot reach a user's filesystem; Desktop is not always online.

## Decision
- A **Plugin** is one versioned, signed-when-distributed package with a single **manifest** declaring:
  - identity, version and publisher
  - a compatibility range against contract versions
  - permissions and a configuration schema
  - one or more **components**
- Each component declares a **target**:
  - `ui`: rendered in an isolated frame in Web and Desktop
  - `logic`: sandboxed TypeScript in an extension host
  - `service`: a separate process or container for self-hosted and Cloud
  - `worker-adapter`: a Provider on Workers
  - `local-adapter`: needs a user machine
- Each component also declares its **availability** requirements.
- The Platform enables each component only where its target is available and its permissions are granted.
- The plugin as a whole stays usable with the components it can run, and the manifest states which components are required.

## Consequences
- One install, one consent screen and one version for a coherent feature; partial availability is explicit instead of implicit failures.
- The manifest schema becomes a contract (ADR-0004) and must be versioned.
- Each target needs its own host and isolation story:
  - isolation tiers: [ADR-0019](0019-isolation-tiers-for-plugins-and-providers.md)
  - logic sandbox: [ADR-0020](0020-plugin-logic-runs-in-quickjs-on-wazero.md)
  - packaging and signing: [ADR-0021](0021-plugin-packaging-signing-permissions-and-compatibility.md)
- Revisit if: packages become too large to distribute, or publishers need independent release cadences per component.

## Alternatives considered
- **Separate packages per component with dependencies**: flexible but multiplies consent, versioning and compatibility problems.
- **Single runtime for all plugin code (e.g. everything in WebAssembly)**: cannot cover OS-specific adapters and services.

## Approval
Proposed by: Claude (master roadmap planning, 2026-10-25) · Approved by: Ricardo Lüders on 2026-10-25, product-owner planning session that resolved all open questions (explicit acceptance of every pending ADR).
