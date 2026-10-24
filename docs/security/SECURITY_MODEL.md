# Security Model (proposed)

Status: **proposed.** No Amaru component enforces anything yet. Items marked **[Weave]** are implemented in the predecessor and are the baseline Amaru must not weaken.

## Assets
User source code and project files · secrets (API keys, tokens) · execution history and evidence · approvals and policies · the user's machine (process execution) · in Cloud: other tenants' data.

## Trust boundaries
| Boundary | Trusted side | Untrusted side |
|---|---|---|
| Client → Platform | Platform | Any local process or remote caller until authenticated |
| Platform → Worker | Platform (state, policy) | Worker reports are validated, not trusted blindly |
| Worker → Provider process | Worker | Provider process and its output |
| Agent context | Platform-assembled instructions | Project files, Skills (until trusted), tool output — prompt-injection sources |
| Plugin | Platform policy | Plugin code, always |

## Baseline rules
1. **Local Platform** binds to loopback only, authenticates every request except health, keeps its token in a user-only file, and checks `Origin` **[Weave]**. Windows ACLs for that file are unverified in Weave.
2. **No auto-approval** of agent permissions; consequential actions need Policy or Approval ([ADR-0006](../architecture/adr/0006-human-authorization-for-consequential-actions.md), accepted).
3. **Process execution**: argv arrays, explicit cwd, environment allowlist, prompts over stdin, no shell **[Weave]**.
4. **Secrets** reach Providers only through a secrets port; never in Events, logs, Artifacts, errors or UI notices **[Weave]**. Write-only from clients **[Weave]**.
5. **Project content is untrusted**: nothing project-supplied executes implicitly; Skills require explicit trust and may be hash-pinned **[Weave]**.
6. **Writes** by the Platform into a Project are confined to its metadata location and the run's output location **[Weave: `.weave/`]**.
7. **Egress**: remote Providers and outbound webhooks are disabled until enabled by the user; webhook egress consent is bound to the exact URL; private destinations blocked **[Weave]**.
8. **Inbound triggers** (webhooks) authenticate with HMAC + replay protection **[Weave]**, and go through Policy like any other start [Proposed].

## Known gaps to close before the MVP
- No capability Policy (Weave GAP-09) → PRD FR-24.
- No Approval step (Weave Q2) → PRD FR-13.
- Provider output and stderr stored unredacted; no retention (Weave GAP-07).
- Any process with the local token can attach any directory as a project (documented residual risk in Weave `docs/security.md`).

## Future boundaries (not designed)
User identity and roles; remote Worker authentication and secret delivery ([Q10](../architecture/OPEN_QUESTIONS.md#q10-remote-execution-environments)); plugin sandboxing and signing ([Q6](../architecture/OPEN_QUESTIONS.md#q6-plugin-sandboxing-and-execution-isolation), [Q7](../architecture/OPEN_QUESTIONS.md#q7-plugin-distribution-signing-permissions-and-compatibility)); tenant isolation ([Q8](../architecture/OPEN_QUESTIONS.md#q8-cloud-tenant-isolation)).

## Reporting
No vulnerability disclosure process exists yet; one is needed before any public release.
