# IAM-001 session-bound CSRF verification — Linux

Date: 2026-11-13 America/Sao_Paulo (2026-11-14 UTC). Executor: Codex for
Ricardo Lüders. Base: `d1b75c29a890803be6d5544fcbdf7ff9930512b7`, after #371,
on designated #18 integration `feat/18-wp-m3-06-public-api-events`.
Candidate: accompanying PR commit. Local Linux 7.2.11-100.fc43.x86_64 amd64,
host Go 1.26.0, Platform toolchain Go 1.27.0, Node v25.7.0. Local results are
not hosted CI. The existing shared checkout and its uncommitted work are preserved;
the candidate uses a separate `/tmp/amaru-92-security` worktree.

## Security acceptance audit on the integration base

Trail map/query/skeleton preceded source inspection. The following tests already
exist at the base; their scope is adapter verification, not product startup.
Source references below name test functions so they remain useful as lines move.

| #92 criterion | Existing evidence on the base | Limit |
|---|---|---|
| Loopback only | `TestLoopbackAndRandomTokens` rejects wildcard/non-loopback addresses and port zero; `TestRunningLoopbackAuth` uses a real IPv4 loopback listener. | Product listener ownership/startup is not composed; no native IPv6 listener/browser claim. |
| Token required and successful launch | `TestLaunchGuardsAndNoRejectedDispatch` requires credentials; `launch` checks 32-byte token encoding, host-only Secure/HttpOnly/Strict Path=/ cookie and code-free redirect. `TestRunningLoopbackAuth` exercises issuance/exchange over HTTP. | No `amaru open` executable wiring or canonical startup path. |
| Expired/replayed launch and session | `TestExactExpiryReplayAndConcurrentExchange` checks exact 60-second and eight-hour boundaries and one winner among 20 concurrent exchanges. `TestLogoutRestartAndSocketLifetime` rejects old credentials after Auth recreation and isolates logout. | In-process Auth recreation is not product process restart. |
| Host / DNS rebinding | `TestLaunchGuardsAndNoRejectedDispatch` rejects hostile names, wrong/malformed ports and forwarded-host spoofing; accepts configured loopback names/port. | Handler guards do not establish the eventual production bind. |
| Origin | Guard tests reject foreign, null and missing mutation/upgrade Origin; `TestOriginMustBeExactSerializedLoopbackOrigin` rejects paths, userinfo, fragments, queries and alternate schemes. Duplicate headers are rejected. | No complete browser attack/security audit is claimed. |
| CSRF | Guard tests reject missing, arbitrary wrong and duplicate CSRF; valid session CSRF permits mutation. | Base lacks a direct test using another live session's valid CSRF token; this change fills that gap. |
| WebSocket auth and lifetime | Guard tests cover cookie upgrades with/without Origin, native bearer upgrades and forbidden query tokens. Socket tests cover logout, expiry and Auth shutdown. | Fixture handlers/watch callbacks are not composed Project Event transport/replay. |
| State-file permissions | Linux `TestPrivateAtomicPublicationAndOwnership` checks directory 0700, discovery/lock files 0600 and same-/cross-process owner exclusion; `TestRefuseUnsafeExistingPaths` rejects unsafe modes and symlinks. Failure test preserves old publication. | Native Linux only; no Windows SID ACL proof, macOS proof, canonical product state-directory selection or power-loss durability. |

Auth tests: `services/platform/internal/adapter/localauth/auth_test.go`.
Native permissions tests:
`services/platform/internal/adapter/discovery/discovery_linux_test.go`.
The [original IAM-001 record](2026-11-12-linux-iam-001.md) separately records
native Firefox localhost/IPv4 Secure-cookie, CSRF, actual WebSocket upgrade and
logout validation. That browser run is historical evidence, not rerun here.
The SDK's test-only Go server also supplies auth HTTP integration in `make check`.

## Bounded change

`TestCSRFTokenCannotCrossSessions` creates two live browser sessions. In both
directions it sends the other session's valid CSRF token with a valid cookie and
allowed Origin to a command and to logout. Each request must return 403, dispatch
no command and emit no cookie. Both sessions must still return their original
CSRF token after each rejection. Each session's own token must dispatch exactly
one command and still permit logout; logout revokes that cookie while preserving
the other live session.

This specifically detects accepting any known token instead of the token bound
to the requesting session. Arbitrary-invalid-token coverage alone cannot detect
that regression. It also checks that rejected logout preserves authentication.
The test composes real Auth middleware with a counting fake product handler;
no production composition, OS extension, contract, dependency, architecture
decision or implementation change is needed. Verdict: within accepted
ADR-0034/0061 session and CSRF semantics. Credentials are never printed.

## Validation

- From `services/platform`, `go test -race -count=1 -v ./internal/adapter/localauth ./internal/adapter/discovery`:
  PASS, exit 0, both packages, 57 passed test/subtest records, no skips.
  Log: `/tmp/amaru-92-focused.log`.
  The initial sandbox attempt failed before tests because the existing Go build
  cache was read-only; the authorized run with cache/loopback access passed.
- `make check`: PASS, exit 0; 348 contract tests, 346 identical Go/TypeScript
  fixture outcomes, 48 SDK tests (no failures/skips), generated-code/type checks,
  Platform tests, cross-platform vet and docs-check.
  Log: `/tmp/amaru-92-check.log`.
- Final `make docs-check`, `git diff --check` and `git diff --cached --check`:
  PASS, exit 0.
- `trail check --base origin/feat/18-wp-m3-06-public-api-events`: PASS, exit 0,
  no conflicts, 14 advisory lexical matches reviewed. They concern receive-pump
  joins, post-I/O live authorization, legacy migrations, captured clone/Git
  provenance, scheduling/PostgreSQL races, OS scope, Project-board fields,
  lifecycle context ordering and storage filesystem imports. None of those
  implementations change here, no rerun dismisses a race, and deferred OS runs
  remain explicitly unvalidated. The record describes those limits rather than
  reintroducing the rejected approaches.

Evidence: **integration-tested** Auth middleware and fake product handler;
discovery filesystem/ownership tests execute on native Linux. New browser,
Provider, PostgreSQL, native non-Linux, production startup/CLI and Project Event
transport behavior are **not validated** by this slice. No complete #92/#18/M3
acceptance or security audit is claimed. No new human architecture decision is
needed; implementation review and the remaining composed acceptance stay owed.

## Refreshed M3 children, blockers and bases

GitHub native child/dependency states, current issue acceptance/delivery bodies,
open/recent merged PRs and remote branch refs were refreshed after #371. No PR
was open at selection. #363–#371 merged into #18; most recently #369 auth-safe
rejection logs at `a0e379b`, #370 SDK/PostgreSQL isolation at `ad5f1bd`, and #371
SDK rollback at `d1b75c2`. These merges establish their bounded evidence, not
the remaining parents' integrated acceptance.

| Queue | Refreshed acceptance and blockers | Designated PR base |
|---|---|---|
| #18 / #92 | #83/#84/#106/#120/#135/#335/#339/#357 closed; #92/#170/#191/#219 open. Native #92 blockers #83/#84 and inherited #15 closed and present. Auth audit above supports this test-only selection; startup/discovery publication, CLI handoff and product/Event composition remain open. | `feat/18-wp-m3-06-public-api-events` at `d1b75c2`. |
| #170 | JSON/correlation, secret fixtures, Worker diagnostics and SDK success/rejected-auth capture merged. Full product-channel instrumentation/capture remains open. #83/#84/#15 closed. | #18 integration. |
| #191 | Two-Project access, settings/Execution/definition slices, HTTP/SDK/PostgreSQL isolation and rejected-attachment rollback merged. Full Work/definition/Provider/secret and running-product acceptance remains owed. Native #120 and inherited #15 closed. | #18 integration. |
| #219 | Native #135 closed and available, inherited #15/#83/#84 closed. Typed mutation Events, external-file detection and two-client conflicts/no silent overwrite remain required; ordered replay alone proves none of these. | #18 integration. |
| #16 | #121/#132/#168/#187/#195 closed; #188/#197 open. #188's native #135 closed, but prerequisite commit `fa8bb47` has not reached this base; #197 waits for open #124 (#195 closed). Integrated policy/approval acceptance remains open. | `feat/16-wp-m3-04-policy-grants-and` at `d2c393a`; bring reviewed prerequisite implementation to this base first. |
| #17 | #122/#123 closed. #80/#81/#82 owner migration gates and #112/#113/#127/#128/#131/#163/#189/#190 remain open. Provider/context integration and real Provider validation remain owed. | `feat/17-wp-m3-05-providers-and-context`, absent as a remote ref; resolve relevant migration gates and establish designated base before dependent reuse. |
| #19 | #85/#119/#178 closed; #124/#154/#125 open. #125 waits for #197. Full Evidence audit chain/worktree acceptance remains open. | `feat/19-wp-m3-07-worktrees-configuration-scopes`, absent as a remote ref; establish from main and verify prerequisite availability before implementation. |

M3 exit #20 stays open. Linux is required; other OSes remain best effort under
the recorded owner scope. No merge, issue closure, board-status mutation or M4
transition is performed. The previous [queue snapshot](2026-11-13-linux-prj-002-sdk-rollback.md)
remains historical; this record adds #371's merge and the security-test selection.
