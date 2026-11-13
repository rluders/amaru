import assert from "node:assert/strict";
import { test } from "node:test";
import { createClient } from "../src/index.ts";

function fakeFetch(respond: (req: Request) => Response) {
  const seen: Request[] = [];
  const f = async (input: Request | URL | string, init?: RequestInit): Promise<Response> => {
    const req = input instanceof Request ? input : new Request(input, init);
    if (new URL(req.url).pathname.endsWith("/health")) {
      return Response.json({ status: "ok", protocolVersion: 1, platformVersion: "test", startedAt: "2026-11-12T00:00:00Z", uptimeSec: 0 });
    }
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


test("mutating execution and Approval commands carry caller keys in their bodies", async () => {
  const { f, seen } = fakeFetch(() => Response.json({}));
  const client = createClient({ baseUrl: "http://x.test/v1", token: "native-fixture", fetch: f });
  await client.POST("/projects/{projectId}/executions/{id}/cancel", {
    params: { path: { projectId: "p", id: "e" } },
    body: { idempotencyKey: "cancel-1", reason: "Stopped by owner" },
  });
  await client.POST("/projects/{projectId}/approvals/{approvalId}/decision", {
    params: { path: { projectId: "p", approvalId: "a" } },
    body: { idempotencyKey: "decision-1", outcome: "approved", evidenceDigest: "sha256:abc" },
  });
  assert.deepEqual(await seen[0]!.json(), { idempotencyKey: "cancel-1", reason: "Stopped by owner" });
  assert.equal((await seen[1]!.json()).idempotencyKey, "decision-1");
});

test("browser cookie transport adds in-memory CSRF only to mutations", async () => {
  const seen: Request[] = [];
  const client = createClient({ baseUrl: "http://localhost:7777/v1", csrfToken: () => "csrf-fixture", fetch: async input => {
    const req = input as Request; seen.push(req);
    if (req.url.endsWith("/health")) return Response.json({ status: "ok", protocolVersion: 1, platformVersion: "test", startedAt: "2026-11-12T00:00:00Z", uptimeSec: 0 });
    return req.method === "DELETE" ? new Response(null, { status: 204 }) : Response.json({ csrfToken: "csrf-fixture", expiresAt: "2026-11-12T08:00:00Z" });
  } });
  await client.GET("/auth/session");
  await client.DELETE("/auth/session");
  assert.deepEqual(seen.map(r => r.credentials), ["omit", "same-origin", "omit", "same-origin"]);
  assert.ok(seen.every(r => !r.headers.has("Authorization")));
  assert.equal(seen[1]!.headers.get("X-Amaru-CSRF"), null);
  assert.equal(seen[3]!.headers.get("X-Amaru-CSRF"), "csrf-fixture");
});

test("browser mutation without CSRF fails before product dispatch; mixed modes rejected", async () => {
  const { f, seen } = fakeFetch(() => new Response(null, { status: 204 }));
  const client = createClient({ baseUrl: "http://localhost:7777/v1", fetch: f });
  await assert.rejects(client.DELETE("/auth/session"), /CSRF token/);
  assert.equal(seen.length, 0);
  assert.throws(() => createClient({ baseUrl: "http://localhost:7777/v1", token: "native-fixture", csrfToken: () => "csrf" }));
});

test("launch exchange uses unversioned path, manual redirect and no credentials", async () => {
  const { f, seen } = fakeFetch(() => new Response(null, { status: 303, headers: { Location: "/" } }));
  const client = createClient({ baseUrl: "http://localhost:7777/v1", token: "native-fixture", fetch: f });
  await client.GET("/auth/launch", { params: { query: { code: "fake-launch-code" } } });
  assert.equal(new URL(seen[0]!.url).pathname, "/auth/launch");
  assert.equal(seen[0]!.credentials, "omit");
  assert.equal(seen[0]!.redirect, "manual");
  assert.equal(seen[0]!.headers.get("Authorization"), null);
});

test("explicit health remains credential-free even on a native client", async () => {
  const seen: Request[] = [];
  const client = createClient({ baseUrl: "http://localhost:7777/v1", token: "native-fixture", fetch: async input => {seen.push(input as Request);return Response.json({});} });
  await client.GET("/health");
  assert.equal(seen[0]!.credentials, "omit");
  assert.equal(seen[0]!.headers.get("Authorization"), null);
});


test("Project settings keep false patches and selected Project identity on the wire", async () => {
  const { f, seen } = fakeFetch(() => Response.json({ importClaudeSkills: false }));
  const client = createClient({ baseUrl: "http://x.test/v1", token: "fake", fetch: f });
  const { data } = await client.PATCH("/projects/{projectId}/settings", {
    params: { path: { projectId: "project-a" } }, body: { importClaudeSkills: false },
  });
  assert.deepEqual(data, { importClaudeSkills: false });
  assert.equal(seen[0]!.method, "PATCH");
  assert.equal(seen[0]!.url, "http://x.test/v1/projects/project-a/settings");
  assert.deepEqual(await seen[0]!.json(), { importClaudeSkills: false });
  await client.GET("/projects/{projectId}/settings", { params: { path: { projectId: "project-b" } } });
  assert.equal(seen[1]!.url, "http://x.test/v1/projects/project-b/settings");
});

test("Project attach, detail and list preserve the current shared inspection fixture", async () => {
  const { readFileSync } = await import("node:fs");
  const project = JSON.parse(readFileSync(new URL("../../contracts/fixtures/api/v1/Project/valid.json", import.meta.url), "utf8"));
  const { f, seen } = fakeFetch(req => Response.json(req.url.endsWith("/projects") && req.method === "GET" ? [project] : project));
  const client = createClient({ baseUrl: "http://x.test/v1", token: "native-fixture", fetch: f });
  const attached = await client.POST("/projects", { body: { root: project.root } });
  assert.deepEqual(await seen[0]!.json(), { root: project.root });
  assert.deepEqual(attached.data, project);
  assert.equal(attached.data?.config.path, ".amaru/config.yaml");
  assert.equal(attached.data?.git.present, true);
  assert.equal(attached.data?.git.remote, undefined);
  assert.equal(attached.data?.git.branch, undefined);
  assert.equal(attached.data?.repositories, undefined);
  assert.deepEqual((await client.GET("/projects/{projectId}", { params: { path: { projectId: project.id } } })).data, project);
  assert.deepEqual((await client.GET("/projects")).data, [project]);
});

test("Project failures remain typed redacted errors", async () => {
  for (const [status, code] of [[400, "invalid_request"], [409, "conflict"], [500, "internal"]] as const) {
    const { f } = fakeFetch(() => Response.json({ error: { code, message: "Request rejected." } }, { status }));
    const client = createClient({ baseUrl: "http://x.test/v1", token: "native-fixture", fetch: f });
    const result = await client.POST("/projects", { body: { root: "/fixture/root" } });
    assert.equal(result.response.status, status);
    assert.equal(result.data, undefined);
    assert.deepEqual(result.error, { error: { code, message: "Request rejected." } });
  }
});
