# MADR-auth-discovery source review — Linux, 2026-11-12

## Scope and environment

Runner: Codex for Ricardo Lüders. Linux `7.2.11-100.fc43.x86_64`, Go `go1.26.0 linux/amd64`. Amaru base: `56d6b7f31ddd2e189f9591e79383fc04447b0e4b`, #18 integration branch. Decision: accepted [ADR-0060](../../architecture/adr/0060-adapt-the-prototype-auth-and-discovery-for-compact-installs.md), issue [#84](https://github.com/rluders/amaru/issues/84).

Source: `rluders/weave-obsidian` at immutable commit `c8a2f6e983beba59edc446f36df2f442edaa9855`. Inspected the four files below using `git show` at that SHA. No source repository was changed and no code was moved into Amaru.

| Source path | Blob ID |
|---|---|
| `runtime/internal/auth/token.go` | `9d9ed33a557126de54b8b5ca4a160d98dbcb12d4` |
| `runtime/internal/auth/token_test.go` | `cd4c0d3dad55c922540832690b06e89809562481` |
| `runtime/internal/discovery/discovery.go` | `8768e5145cbdf5a73e15fcfaedc4c1b864d8e7a8` |
| `runtime/internal/discovery/discovery_test.go` | `62424052aca02c5d5296aa61af85c09bb429ef2a` |

## Commands and evidence

- `git ls-tree -r c8a2f6e983beba59edc446f36df2f442edaa9855 runtime/internal/auth runtime/internal/discovery`: exit 0, four files and blob IDs above.
- In the existing isolated extraction `/tmp/amaru-83-baseline/runtime` (runtime and sibling protocol archived at the same SHA): `go test -count=1 -json ./internal/auth/... ./internal/discovery/... > /tmp/amaru-84-source-tests.jsonl`: exit 0, **5 passed, 0 failed, 0 skipped**, two packages. JSON results counted by test action, excluding package summaries.
- `make check` on the complete Amaru documentation proposal: exit 0. Local Linux repository checks only, not hosted CI or proof of target authentication.
- `git diff --check`: exit 0. `trail check`: exit 0; lexical advisories reviewed against the proposal, no learning conflict.

Tests executed: `TestTokenUniqueAndComparable`, `TestWriteIsAtomicPrivateAndRemovable`, `TestWriteRequiresEndpoint`, `TestRemoveIfOwnerLeavesAnotherRuntimesRecord`, `TestReadAndLive`. Evidence level: predecessor unit-tested and loopback integration-tested. No real agents or paid APIs used. Source tests use temporary files and an `httptest` HTTP server.

## Review findings and required changes

- Token generation uses 32 bytes from `crypto/rand`; comparison uses `crypto/subtle`. The source test can print token values on failure: replace those diagnostics in Amaru.
- Publication uses a same-directory temporary file and rename; the test checks Unix 0600, replacement and no leftover temporary file. Existing state-directory permissions, injected write/close/rename failures, hostile path substitution and crash durability are not established by that test. Atomic visibility is not evidence of power-loss durability.
- Owner cleanup compares PID/token then removes, without synchronization. The source test proves sequential owner/non-owner behavior only, not safety against replacement between read and removal. Amaru must test the interleaving and prevent replacement deletion.
- Discovery liveness accepts HTTP 200 from the default client's health request; neither protocol compatibility nor process identity is established. Validate destinations and redirect behavior before sending credentials. Test stale files, reused PID, unrelated HTTP service and incompatible health separately.
- Windows source relies on per-user directory ACL assumptions; no user-SID ACL implementation or native evidence was found. Unix file-mode results are not Windows security evidence.
- Source HTTP middleware (`server.go` lines 197–243) allows missing Origin and Event-stream query tokens. It cannot supply ADR-0034's launch/session/Host/CSRF boundary; ADR-0059 excludes it from adaptation.
- Current OpenAPI `x-protocol.origin-policy` is token-only for missing Origin. #92 must reconcile browser-session metadata, auth operations and CSRF wire details through the existing contract review/fixture/SDK process; no endpoint was invented in this proposal.

Architecture review: Ricardo Lüders approved selective adaptation, exact source/history and evidence gaps on 2026-11-12; see ADR-0060 Approval. No domain/client/Worker boundary change, dependency or infrastructure introduced. Implementation constraints and self-contained behavior tests are in ADR-0060.

## Limits and approval

No Amaru authentication/discovery code or new tests are implemented here. Amaru browser launch, session lifecycle, Host/Origin/CSRF rejection, concurrent discovery cleanup, runtime log redaction and native OS permissions are **not validated**. No complete security audit was performed. Linux is required for remaining M3; macOS/Windows are best effort and not validated here. Windows security requirements remain requirements even though native validation is deferred.

Readiness 1–4, 7 and 8 have decision-stage evidence; 5 (Amaru tests) and 6 (real validation) remain unmet implementation requirements. Ricardo Lüders explicitly approved the component, selected Adapt boundary, exact commit, fresh import with provenance and stated gaps on 2026-11-12 in the implementation conversation: “Proposal approved. Update, and merge. Sync, and continue next task.” Accepted ADR-0060 authorizes only its selected adaptation scope; #84 closes after merge, while #92 and #18 remain open.
