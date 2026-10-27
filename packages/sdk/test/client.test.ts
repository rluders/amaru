import assert from "node:assert/strict";
import { test } from "node:test";
import { createClient } from "../src/index.ts";

function fakeFetch(respond: (req: Request) => Response) {
  const seen: Request[] = [];
  const f = async (input: Request | URL | string, init?: RequestInit): Promise<Response> => {
    const req = input instanceof Request ? input : new Request(input, init);
    seen.push(req);
    return respond(req);
  };
  return { f: f as typeof globalThis.fetch, seen };
}

test("typed GET uses the base URL, path parameters and the bearer header", async () => {
  const { f, seen } = fakeFetch(() => Response.json({ events: [], nextCursor: null }));
  const client = createClient({ baseUrl: "http://127.0.0.1:7777/v1", token: "t-fake", fetch: f });
  const { data, error } = await client.GET("/projects/{projectId}/executions/{id}/events", {
    params: { path: { projectId: "p1", id: "e1" }, query: { after: 5, limit: 10 } },
  });
  assert.equal(error, undefined);
  assert.deepEqual(data, { events: [], nextCursor: null });
  assert.equal(seen.length, 1);
  assert.equal(seen[0]!.method, "GET");
  assert.equal(seen[0]!.url, "http://127.0.0.1:7777/v1/projects/p1/executions/e1/events?after=5&limit=10");
  assert.equal(seen[0]!.headers.get("authorization"), "Bearer t-fake");
});

test("without a token no Authorization header is sent", async () => {
  const { f, seen } = fakeFetch(() => Response.json({}));
  const client = createClient({ baseUrl: "http://x.test/v1", fetch: f });
  await client.GET("/projects/{projectId}/executions/{id}/events", { params: { path: { projectId: "p", id: "e" } } });
  assert.equal(seen[0]!.headers.get("authorization"), null);
});

test("an error response is returned as typed error data, not thrown", async () => {
  const { f } = fakeFetch(() => Response.json({ error: { code: "not_found", message: "no such execution" } }, { status: 404 }));
  const client = createClient({ baseUrl: "http://x.test/v1", fetch: f });
  const { data, error, response } = await client.GET("/projects/{projectId}/executions/{id}/events", {
    params: { path: { projectId: "p", id: "e" } },
  });
  assert.equal(data, undefined);
  assert.equal(response.status, 404);
  assert.equal(error?.error.code, "not_found");
});

// Compile-time check, run by `pnpm run typecheck`: an unknown path or a wrong parameter is a type error.
void (() => {
  const client = createClient({ baseUrl: "http://x.test/v1" });
  // @ts-expect-error not a path of the contract
  void client.GET("/projects/{projectId}/not-a-route", {});
  // @ts-expect-error path parameters are required
  void client.GET("/projects/{projectId}/executions/{id}/events", {});
});
