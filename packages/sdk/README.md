# packages/sdk — TypeScript SDK

**Status: implemented-and-verified locally, 0.x, private (not published).** Generated types, the typed HTTP client and the event-stream client are tested against fakes. HTTP compatibility negotiation is also integration-tested against a loopback fixture server (#106). Project settings GET/PATCH are also integration-tested against the real Go auth/application/HTTP adapters with fake Project storage. Full Platform startup/API and browser SDK acceptance remain unvalidated.

**License:** Apache-2.0 for SDK packages is decided in [ADR-0026](../../docs/architecture/adr/0026-fair-code-community-license-with-commercial-licensing.md). The package [`LICENSE`](LICENSE) is a byte-identical copy of the unmodified Apache License 2.0 text already in [`packages/contracts/LICENSE`](../contracts/LICENSE) (from <https://www.apache.org/licenses/LICENSE-2.0.txt>), added at the product owner's explicit instruction on 2026-10-26. `package.json` stays `private` because nothing is published yet.

## Layout

| Path | What |
|---|---|
| `src/generated/schema.ts` | **Generated, never edited.** Types for every path and schema of `packages/contracts/openapi/openapi.yaml`, made by `openapi-typescript`. Carries a header saying so. |
| `scripts/generate.ts` | The generation command; `--check` is the staleness check. |
| `src/client.ts` | `createClient({ baseUrl, token?, csrfToken?, fetch? })`: an `openapi-fetch` client typed with the generated `paths`. Paths, parameters, bodies and responses are checked at compile time; product requests first check health compatibility. |
| `src/events.ts` | `EventStream`: the Project event stream over WebSocket with cursor resume (below). |
| `test/` | Unit and loopback integration tests (`node --test`), including a Linux Go auth fixture. |

There are no hand-written request or response types: everything comes from the contract, including the stream frames (`components.schemas.StreamFrame`, declared in the OpenAPI document for this purpose).

## Commands

```sh
make sdk-ts            # install from the lockfile, staleness check, tsc --noEmit, tests (part of `make check`)
pnpm run generate      # in packages/sdk: regenerate src/generated/schema.ts, then commit it
```

Needs Node ≥ 22.18 and pnpm (verified with Node 25.7.0 and pnpm 11.5.0; Node 22 not run).

## Compatibility negotiation

Before every HTTP product request, the client reads typed `GET /health` at that request's API base using its selected transport. Protocol major 1 is supported, as defined by Health.protocolVersion and [ADR-0025](../../docs/architecture/adr/0025-api-and-protocol-versioning-policy.md). platformVersion is a build string, not the wire version. Additional health fields and new build/minor versions do not make a compatible major incompatible.

Health probes omit browser credentials and send no administrator bearer header, disable redirects and caching, and use the product request's abort signal. They must succeed before the product request is sent. Each operation rechecks to detect a replaced/restarted Instance; this costs an extra HTTP round trip per operation. Direct `GET /health` remains available for diagnostics without a preliminary check.

```ts
import { CompatibilityError, createClient } from "./src/index.ts";

const client = createClient({ baseUrl: "http://127.0.0.1:7777/v1", token });
try {
  const result = await client.GET("/projects");
  // Render result.data or the typed product error.
} catch (error) {
  if (!(error instanceof CompatibilityError)) throw error;
  // Render error.message with state error.kind:
  // "incompatible", "invalid-health", or "connection".
}
```

These exported failure states and safe explanations are available to clients; no UI is implemented by this package. Invalid/missing health fields, malformed JSON, failed HTTP probes and unreachable Instances dispatch no product request. A later request retries the check, permitting recovery. The existing EventStream independently checks its Hello protocol major and exposes `incompatible` before delivering events. Worker/Provider handshake ranges remain separate under ADR-0025; this HTTP client does not negotiate those protocols.

Local auth/session composition is tracked by #92 and combined public API validation remains #18. Compatibility is not proof of endpoint identity or authorization.

## Event stream

```ts
import { EventStream } from "@amaru/sdk"; // not published yet; import from src/ inside the repository

const stream = new EventStream({
  baseUrl: "http://127.0.0.1:7777/v1",
  projectId,
  // Browser: ambient same-origin session cookie. Native: inject a header-capable webSocket factory.
  since: savedCursor,          // optional: resume after the last seq you processed
  onEvent: async (event) => { /* apply; the cursor advances after this resolves */ },
  onResync: async ({ reason, resumeFrom }) => { /* reload state through the HTTP API */ },
  onState: (s) => { /* connecting | connected | reconnecting | unavailable | incompatible | stopped */ },
});
await stream.start();          // resolves after stop() or on an incompatible protocol
```

Behavior, all from the contract text and covered by `test/events.test.ts`:

- Reconnects after a drop with `since=<last delivered seq>`; events with `seq` at or below the cursor are dropped (delivery is at-least-once, the handler may see a replay only if it threw).
- A gap in a Project stream (`seq` not `cursor + 1`) closes the connection and replays from the cursor. With `executionId` set, `seq` is not gap-free and the check is off.
- `ResyncRequired`, or a Hello whose `lastSeq` is below the cursor (Platform restarted or restored): `onResync` runs first (events are not delivered meanwhile), then the stream resumes from `resumeFrom`.
- A Hello with another `protocolVersion` than 1 stops the stream (`incompatible`); no retry.
- Backoff `min(500 · 2^n, 15000)` ms, `maxAttempts` (default 8) consecutive failures give `unavailable`; `retryNow()` skips the wait or leaves that state. A connection that received Hello resets the count.
- Frames of unknown type are ignored (additive evolution); a malformed frame reconnects.

The earlier plugin's `connection.ts` was read as a behavioral reference only ([ADR-0031](../../docs/architecture/adr/0031-obsidian-plugin-is-retired.md)); no code is shared with it.

## Evidence

Unit-tested with fake `fetch` and fake sockets; compatibility also integration-tested with a Node HTTP server bound to Linux loopback, including mismatch and redirect rejection with no mutation. See the [#106 evidence record](../../docs/validation/m3/2026-11-12-linux-api-005-compatibility.md). **Not validated** against the full composed Platform API, a real WebSocket server, a browser, native macOS/Windows, or Node 22. Query-token WebSocket transport was removed under #92/ADR-0034. Native callers provide a header-capable `webSocket` factory; browsers use ambient same-origin session cookies.

## Dependencies

| Dependency | Why | Version | License |
|---|---|---|---|
| `openapi-fetch` | The typed client required by FEAT-API-003 (runtime). | 0.17.0 | MIT |
| `openapi-typescript` | Generator required by FEAT-API-003; dev only, nothing of it ships. | 7.13.0 | MIT |
| `typescript`, `@types/node` | Typecheck and Node typings, dev only; same versions as `packages/contracts`. | 5.9.3, 22.20.5 | Apache-2.0, MIT |

The lockfile is `pnpm-lock.yaml`; `make sdk-ts` installs with `--frozen-lockfile`.

## Local authentication

[ADR-0061](../../docs/architecture/adr/0061-local-browser-authentication-wire-contract.md) defines the two transports. CLI/Desktop set `token` for Authorization and omit cookies. A same-origin browser omits `token`, reads `GET /auth/session`, retains `data.csrfToken` in memory, then supplies `csrfToken: () => csrf` to the client. Mutations fail before product dispatch when that value is missing. Never persist a session credential or administrator secret in browser storage. The server enforces Origin and CSRF; the browser supplies Origin. Product redirects are rejected to avoid forwarding credentials. The launch exchange uses its unversioned path and a manual, credential-free redirect response; actual browser handoff is a navigation to the one-use URL, followed by the server's code-free redirect.

`test/auth.test.ts` builds the test-only Go adapter fixture and uses private discovery on Linux. It exercises issuance, unversioned launch, cookie/session read, CSRF logout and replay rejection over real loopback HTTP. Node injects Cookie/Origin because it has no browser cookie jar. This is SDK integration evidence, not a browser run of the SDK. Native Firefox validation of the Go adapter is recorded separately in the [IAM-001 evidence](../../docs/validation/m3/2026-11-12-linux-iam-001.md). The fixture also composes the real Project settings GET/PATCH adapter and ProjectAccess over two same-name fake-attached Projects. Native and cookie/CSRF SDK clients verify default false, explicit false, repeated and concurrent updates, cross-client reads, encoded remote/local Project keys, missing Project denial, missing/wrong credentials and post-logout denial with unchanged settings. See the [settings SDK evidence](../../docs/validation/m3/2026-11-12-linux-prj-002-settings-sdk.md). This is a test-only resource composition; no product startup or UI is claimed.
