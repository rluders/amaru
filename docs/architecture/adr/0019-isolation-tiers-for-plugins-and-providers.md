---
title: Plugins and Providers run in fixed isolation tiers
status: superseded
date: 2026-10-25
deciders: Ricardo Lüders
superseded-by: 0041
---

# 0019. Plugins and Providers run in fixed isolation tiers

## Context
- Question: [Q6](../OPEN_QUESTIONS.md#q6-plugin-sandboxing-and-execution-isolation). Product-owner answer: "WebAssembly is a good stuff, and server deployments could be even better."
- Plugins run next to users' code and secrets; Cloud must isolate tenants. [ADR-0012](0012-plugin-package-with-targeted-components.md) defines component targets `ui`, `logic`, `service`, `worker-adapter`, `local-adapter`.
- The prototype runs external Providers as separate processes speaking JSON Lines over stdio; nothing runs in-process.
- OS sandboxes without admin rights differ widely across Windows, macOS and Linux; containers are not available in compact installs (NFR-1).

## Decision
Each component target has one isolation tier:

| Target | Tier |
|---|---|
| `ui` | Sandboxed `<iframe sandbox="allow-scripts">` on an opaque origin; talks to the client only via a versioned `postMessage` bridge that proxies the public API with the plugin's granted permissions. No direct network. |
| `logic` | WebAssembly sandbox inside the Platform ([ADR-0020](0020-plugin-logic-runs-in-quickjs-on-wazero.md)); deny-by-default host functions. |
| `worker-adapter`, `local-adapter`, Providers | Separate OS process started by a Worker with argv, explicit cwd, minimal env and only granted secrets; never loaded in-process. Linux uses Landlock/namespaces when available; other OSes rely on process boundaries plus Policy (best effort, documented). |
| `service` and all Steps in server/Cloud Workers | Rootless OCI container per Step (or per Lease) with read-only base, no host mounts except the Worktree, egress limited by Policy. Containers never mix Organizations. |

- Native code is never loaded into the Platform or client process.
- Every capability a component uses passes through Policy ([ADR-0037](0037-one-policy-evaluates-capabilities-grants-and-approvals.md)).

## Consequences
- Compact installs get strong isolation for `ui` and `logic`, process isolation for adapters; server/Cloud add containers.
- Plugin authors know the sandbox from the target alone.
- Validation: escape-surface review per tier before M6; Cloud container isolation reviewed before M9.
- Revisit if: a target needs capabilities its tier cannot mediate.

## Alternatives considered
- **In-process Go plugins**: no isolation, ABI fragility.
- **Containers everywhere**: impossible in compact installs without Docker.
- **OS sandbox as the primary boundary on all OSes**: not portable without admin rights.

## Approval
Proposed by: Claude (open-question resolution, 2026-10-25) · Approved by: Ricardo Lüders on 2026-10-25, product-owner planning session (answer recorded in OPEN_QUESTIONS Q6).
