# packages/sdk — TypeScript SDK

**Status: proposed, 0.x, private (not published).** Nothing serves the API yet: there is no Platform in the repository. What exists and is checked here is the generated types, a typed HTTP client and an event-stream client, tested against fakes (WP-M2-02, [#10](https://github.com/rluders/amaru/issues/10), FEAT-API-003).

**License:** Apache-2.0 for SDK packages is decided in [ADR-0026](../../docs/architecture/adr/0026-fair-code-community-license-with-commercial-licensing.md). The package [`LICENSE`](LICENSE) is a byte-identical copy of the unmodified Apache License 2.0 text already in [`packages/contracts/LICENSE`](../contracts/LICENSE) (from <https://www.apache.org/licenses/LICENSE-2.0.txt>), added at the product owner's explicit instruction on 2026-10-26. `package.json` stays `private` because nothing is published yet.

## Layout

| Path | What |
|---|---|
| `src/generated/schema.ts` | **Generated, never edited.** Types for every path and schema of `packages/contracts/openapi/openapi.yaml`, made by `openapi-typescript`. Carries a header saying so. |
| `scripts/generate.ts` | The generation command; `--check` is the staleness check. |
| `src/client.ts` | `createClient({ baseUrl, token?, fetch? })`: an `openapi-fetch` client typed with the generated `paths`. Paths, parameters, bodies and responses are checked at compile time. |
| `src/events.ts` | `EventStream`: the Project event stream over WebSocket with cursor resume (below). |
| `test/` | Unit tests (`node --test`), fakes only. |

There are no hand-written request or response types: everything comes from the contract, including the stream frames (`components.schemas.StreamFrame`, declared in the OpenAPI document for this purpose).

## Commands

```sh
make sdk-ts            # install from the lockfile, staleness check, tsc --noEmit, tests (part of `make check`)
pnpm run generate      # in packages/sdk: regenerate src/generated/schema.ts, then commit it
```

Needs Node ≥ 22.18 and pnpm (verified with Node 25.7.0 and pnpm 11.5.0; Node 22 not run).

## Event stream

```ts
import { EventStream } from "@amaru/sdk"; // not published yet; import from src/ inside the repository

const stream = new EventStream({
  baseUrl: "http://127.0.0.1:7777/v1",
  projectId,
  token,                       // sent as ?token= (browsers cannot set WebSocket headers); never logged by the SDK
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

Unit-tested with a fake `fetch` and fake sockets. **Not validated** against a real Platform or a real WebSocket server (none exists), in a browser, or on Node 22.

## Dependencies

| Dependency | Why | Version | License |
|---|---|---|---|
| `openapi-fetch` | The typed client required by FEAT-API-003 (runtime). | 0.17.0 | MIT |
| `openapi-typescript` | Generator required by FEAT-API-003; dev only, nothing of it ships. | 7.13.0 | MIT |
| `typescript`, `@types/node` | Typecheck and Node typings, dev only; same versions as `packages/contracts`. | 5.9.3, 22.20.5 | Apache-2.0, MIT |

The lockfile is `pnpm-lock.yaml`; `make sdk-ts` installs with `--frozen-lockfile`.
