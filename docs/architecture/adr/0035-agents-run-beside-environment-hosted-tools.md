---
title: Agents run on the Worker beside Environment-hosted tools, behind a Worker-side MCP gateway
status: accepted
date: 2026-10-25
deciders: Ricardo Lüders
---

# 0035. Agents run on the Worker beside Environment-hosted tools, behind a Worker-side MCP gateway

## Context
- Question: [Q22](../OPEN_QUESTIONS.md#q22-agent-placement-relative-to-environment-hosted-tools). Product-owner answer: "You can decide."
- Verified 2026-10-25: Epic's Unreal MCP plugin (UE 5.8, Experimental) serves HTTP/SSE **only on `127.0.0.1:8000/mcp`, without authentication**, and runs tool calls one at a time on the game thread (https://dev.epicgames.com/documentation/en-us/unreal-engine/unreal-mcp-in-unreal-editor). Community UE MCP servers are early-stage.
- Weave ADR-025 (accepted in direction, not built) plans a loopback MCP gateway per agent session that proxies an allow-listed set of tools, keeps secrets in the Runtime and audits every call by argument hash. Weave ADR-023 runs agents hermetically (`--strict-mcp-config`, explicit allowed tools).

## Decision
- The agent process runs **on the Worker in the same Execution Environment** as the tools it uses; MCP traffic stays on loopback.
- The Worker runs an **MCP gateway** per agent session (the ADR-025 design moved to the Worker): the agent sees one gateway server; the gateway exposes only tools allowed by Policy for this Step, forwards calls to local MCP servers, and emits an Event per call (tool, argument hash, duration, outcome) to the Platform.
- Agent credentials are delivered with the Lease ([ADR-0024](0024-remote-worker-registration-identity-and-delivery.md)) and wiped after it.
- Environments whose tools are single-threaded editors are **leased exclusively**: one Step at a time per editor instance.

## Consequences
- Low latency and no exposure of unauthenticated editor endpoints beyond the VM.
- Audit and Policy are enforced on the Worker, which the Platform trusts only within its enrolled identity.
- Validation (M5 UE5 pilot, assumption A8): call latency, disconnect behavior and audit completeness on a Windows VM.

## Alternatives considered
- **Agent elsewhere, MCP proxied through the Platform**: adds round trips to every fine-grained editor call and requires exposing the editor endpoint.

## Approval
Proposed by: Claude (open-question resolution, 2026-10-25) · Approved by: Ricardo Lüders on 2026-10-25, product-owner planning session (decision delegated to Claude in OPEN_QUESTIONS Q22 and accepted in the same session).
