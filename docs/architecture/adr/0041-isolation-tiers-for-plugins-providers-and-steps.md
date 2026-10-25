---
title: Plugins, Providers and Steps run in fixed isolation tiers, restated
status: accepted
date: 2026-10-25
deciders: Ricardo Lüders
supersedes: 0019
---

# 0041. Plugins, Providers and Steps run in fixed isolation tiers, restated

## Context
- [ADR-0019](0019-isolation-tiers-for-plugins-and-providers.md) fixed one isolation tier per component target. The product-owner answer to [Q6](../OPEN_QUESTIONS.md#q6-plugin-sandboxing-and-execution-isolation) was "WebAssembly is a good stuff, and server deployments could be even better". ADR-0019 left three items inconsistent or open:
  - `logic` ran "inside the Platform", which contradicts [ADR-0009](0009-modular-monolith-platform-with-separate-worker-role.md). This is now resolved by [ADR-0040](0040-plugin-logic-runs-in-a-separate-plugin-host-process-built-with-javy.md).
  - Server/Cloud containers were "per Step (or per Lease)" ([Q25](../OPEN_QUESTIONS.md#q25-container-granularity-and-environment-sandbox-class)).
  - The Environment sandbox was called "optional" without saying who chooses it or what the default is.
- A Lease is one Worker's claim on one Step attempt ([glossary](../../product/GLOSSARY.md)). A retry is a new Lease.
- Delegated in the 2026-10-25 planning prompt.

## Decision
Each component target has exactly one isolation tier:

| Target | Tier |
|---|---|
| `ui` | Sandboxed `<iframe sandbox="allow-scripts">` on an opaque origin. It talks to the client only through a versioned `postMessage` bridge that proxies the public API with the plugin's granted permissions. No direct network. |
| `logic` | Wasm module in the separate `amaru plugin-host` process ([ADR-0040](0040-plugin-logic-runs-in-a-separate-plugin-host-process-built-with-javy.md)). Host calls are deny-by-default. |
| `worker-adapter`, `local-adapter`, Providers | A separate OS process started by a Worker with argv, an explicit cwd, a minimal env and only granted secrets; never loaded in-process. Linux adds Landlock and namespaces when available. Other OSes rely on the process boundary plus Policy (best effort, documented). |
| `service`, and every Step in a `container`-class Environment | One rootless OCI container **per Lease**: a fresh container for each Step attempt, destroyed when the Lease ends. Read-only base, no host mounts except the Worktree, egress limited by Policy. Never reused across Steps, Executions or Organizations. |

- **Environment sandbox class** is an Environment attribute set by an Owner or Admin: `container` or `none`.
  - Default: `container` for Environments of server and Cloud deployments (vendor-managed Cloud Environments are always `container`).
  - Default: `none` for a user's workstation Environment, including the compact embedded Worker. These Steps run as processes under the user's account, with Provider confinement and Policy.
  - Policy may require `container` for a Capability. A Step that requires it is never scheduled on a `none` Environment.
- Native code is never loaded into the Platform or client process.
- Every capability a component uses passes through Policy ([ADR-0037](0037-one-policy-evaluates-capabilities-grants-and-approvals.md)).

## Consequences
- Plugin authors know the sandbox from the target alone, and operators know it from the Environment class.
- Retries never inherit container state. Some speed is lost: no warm container is reused between attempts.
- Validation: an escape-surface review per tier before M6; Cloud container isolation reviewed before M9 (cross-tenant test suite).
- Revisit if: container start-up per Lease dominates Step time (measured at M5). A per-Execution warm pool within one Organization would then need its own ADR.

## Alternatives considered
- **Container per Execution**: faster, but Steps share state and a compromised Step affects later ones.
- **Containers everywhere, including workstations**: impossible in compact installs without Docker (NFR-1).
- **In-process Go plugins**: no isolation, fragile ABI.

## Approval
Proposed by: Claude (second-pass open-question resolution, 2026-10-25) · Approved by: Ricardo Lüders on 2026-10-25, planning prompt that delegated every remaining decision (status "accepted, delegated" confirmed in the same session).
