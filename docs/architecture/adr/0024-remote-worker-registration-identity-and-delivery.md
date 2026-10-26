---
title: Remote Workers are enrolled per Organization with key-based identity and per-Lease delivery
status: accepted
date: 2026-10-25
deciders: Ricardo Lüders
---

# 0024. Remote Workers are enrolled per Organization with key-based identity and per-Lease delivery

## Context
- Question: [Q10](../OPEN_QUESTIONS.md#q10-remote-execution-environments). Product-owner answer: "VMs and remote machines need to be registered by workspace or projects; configuration should be provided manually by the user. You can spike and decide the best approach." Workspace = Organization ([ADR-0022](0022-cloud-tenancy-row-level-isolation-by-organization.md)).
- [ADR-0013](0013-outbound-pull-based-workers-with-leases.md): Workers connect outbound and hold Leases. Transport: [ADR-0029](0029-worker-protocol-over-websocket-and-protobuf.md).
- The prototype's Provider contract (stdio provider protocol, `Resumable`, `Detachable`) is the protocol seed; Windows process-tree termination is unvalidated in the prototype (GAP-05).
- Assumption A5: a Worker run inside an interactive user session satisfies Windows/UE5 use cases.

## Decision
- **Registration**: an Organization admin creates a Worker registration (name, optional Project restriction, labels) and receives a **one-time enrollment token** (short TTL). Configuration is manual: the Worker's local config file declares Platform URL, labels, Environment description and Capabilities/Providers it offers.
- **Identity**: on enrollment the Worker generates an **Ed25519 key pair**; the Platform stores only the public key. Each connection authenticates with a signed challenge and receives a short-lived session; keys can be rotated and the Worker revoked at any time.
- **Project revision**: by default the Worker checks out the **Git commit SHA** recorded in the assignment into a per-Lease Worktree, using either its own Git credentials or a short-lived, repository-scoped credential the Platform issues for that Lease. Non-Git inputs come as a content-addressed bundle from the Artifact store ([ADR-0017](0017-content-addressed-artifact-store-filesystem-or-s3.md)).
- **Secrets**: delivered inside the Lease, only those granted to the Step, held in memory, passed to Providers via env or stdin, never written to disk, wiped at Lease end.
- **Interactive tools**: the Worker runs as a user process inside the logged-in session of the machine (Windows service mode is not used for GUI tools).
- **Report back**: Events, heartbeats and results over the Worker protocol; large outputs via Artifact upload.
- **Process control**: Windows Job Objects (kill-on-close), POSIX process groups.

## Consequences
- No inbound ports on remote machines; revocation is immediate.
- Users configure machines explicitly; no auto-discovery.
- Validation (M5 spike): one Worker on a second Windows machine with forced disconnects, process-tree kill and revocation; UE5 pilot (A5).

## Alternatives considered
- **Shared static tokens**: no per-Worker revocation or rotation.
- **mTLS with a private CA**: stronger PKI story but heavier for users; may be added for Cloud-run Workers.
- **File sync of the working tree**: drift and size issues; kept only as Artifact bundles.

## Approval
Proposed by: Claude (open-question resolution, 2026-10-25) · Approved by: Ricardo Lüders on 2026-10-25, product-owner planning session (answer recorded in OPEN_QUESTIONS Q10; detailed approach delegated).
