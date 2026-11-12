# Linux MADR-httpapi proposal evidence — 2026-11-12

Scope: [#83](https://github.com/rluders/amaru/issues/83), accepted [ADR-0059](../../architecture/adr/0059-adapt-the-prototype-http-handlers-to-the-platform-public-api.md). Reviewer/runner: Codex for Ricardo Lüders. Target inspected: #18 integration at `fa8bb475979dc39e81d05fffd00dfa40eeb87ae7`. This record covers the documentation decision and source evidence; no HTTP implementation is delivered.

## Source and method

Source: `rluders/weave-obsidian` commit `c8a2f6e983beba59edc446f36df2f442edaa9855`. `git rev-parse c8a2f6e` resolved to that full SHA. `git ls-tree -r` verified the immutable component `runtime/internal/transport/httpapi/`: 26 Go files, 4,078 lines, 55 `Test` functions. Counts were computed from the extracted files; they include tests. Inspected `server.go` registration/authentication/Origin/Project/response code, `events.go`, `work.go` decoding and mutation handlers, `agents.go` updates, `settings.go`, and all test function names. No claim of a complete security audit is made.

The source checkout was not modified. Its immutable `runtime` and sibling `protocol` directories were extracted with `git archive` into a disposable directory outside Amaru. The sibling protocol schemas are required by its contract tests. No predecessor implementation was added to Amaru.

```sh
git archive c8a2f6e983beba59edc446f36df2f442edaa9855 runtime | tar -x -C /tmp/amaru-83-baseline
git archive c8a2f6e983beba59edc446f36df2f442edaa9855 protocol | tar -x -C /tmp/amaru-83-baseline
cd /tmp/amaru-83-baseline/runtime
go test -count=1 -json ./internal/transport/httpapi/...
```

**Passed:** exit 0; 55 tests passed, 0 failed, 0 skipped; one package passed. This is predecessor unit/integration evidence with local fixtures and loopback clients, not an Amaru API test or a real Provider/browser run. Initial extraction of `runtime` alone failed because `protocol/schema.json` was absent; adding the sibling fixtures from the same commit fixed the extraction. No source or tests were edited to pass.

## Findings and architecture review

Verdict: **sound selective adaptation; approved by the product owner on 2026-11-12**. Platform state ownership, pure domain, public client contracts and Worker isolation remain unchanged; no dependency or infrastructure is introduced. The simpler unchanged-package import fails the current authentication and durable replay requirements. Fresh implementations remain appropriate where contracts diverge.

| Source location | Observed behavior / risk | Required disposition |
|---|---|---|
| `server.go:26-33,194-244` | Desktop Origin default, missing-Origin bypass, bearer/query-token authentication; no Host middleware in the registered wrapper chain | Do not import; #84/#92 implement accepted ADR-0034 and negative Host/Origin/CSRF/launch tests |
| `server.go:87-181,270-281` | Many extra routes and direct predecessor app/Project context composition | Register current contract operations only; bind authenticated Organization/Project before command/query resolution; test real registrations with routeguard |
| `work.go:10-20` | Bounded strict decoder reads only the first JSON value and echoes decoder error text | Reject trailing content and redact error responses; verify no mutation on rejection |
| `events.go:24-90` | Replay from predecessor bus; close-on-replay-loss/overflow, ping and shutdown handling | Reference lifecycle tests, compose durable Amaru replay and public envelopes, preserve explicit resync and Project scope |
| `agents.go:61-101` | Pointer fields preserve partial edits, but this handler alone does not establish stale-write conflict protection | Retain patch semantics; #219 verifies conflicts at the authoritative mutation boundary |

The ADR maps all source file groups, source terminology, current contracts, tests, provenance and all eight readiness checks. #83's prerequisite #8 and #18's prerequisite #15 are closed. Approval is recorded in ADR-0059 following the owner's instruction in the PR #323 review session: "Proposal approved. Updated and get it merged." #84's separate migration/authentication decision remains open; this proposal does not waive it. The historical #83 egress gate is superseded by the owner's fresh #168 decision; the backlog map now records that explicitly.

## Amaru checks and limits

`make check` on PR #322's merged #18 result (`fa8bb475979dc39e81d05fffd00dfa40eeb87ae7`) passed, exit 0, on Linux. The proposal's `make check` also passed, exit 0, including documentation, formatting, contract/SDK and Platform checks. These are local results, not CI; no HTTP behavior is inferred from document checks.

Environment: Linux x86_64 on the local development machine; Platform/source Go 1.27 toolchain. No paid API or real agent was used. Native macOS/Windows, a running Amaru API/browser, end-to-end auth/security, transport load behavior and real Providers are **not validated**. Readiness checks 5 (Amaru implementation tests) and 6 (real validation) remain unmet and were explicitly accepted as evidence gaps for this decision. Check 8 (owner approval) is met: the approval covers the component, exact commit, selected Adapt boundary and fresh-import history. The separate #84 gate remains.
