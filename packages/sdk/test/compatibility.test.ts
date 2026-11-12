import assert from "node:assert/strict";
import { test } from "node:test";
import { createServer } from "node:http";
import { once } from "node:events";
import { CompatibilityError, createClient } from "../src/index.ts";

const health = {
  status: "ok", protocolVersion: 1, platformVersion: "1.2.3",
  startedAt: "2026-11-12T00:00:00Z", uptimeSec: 0,
};
const command = {
  params: { path: { projectId: "p", id: "e" } },
  body: { idempotencyKey: "cancel-1", reason: "Owner stopped" },
};

test("health precedes commands, accepts additive fields and ignores build semver", async () => {
  const seen: Request[] = [];
  const client = createClient({
    baseUrl: "http://127.0.0.1:7777/v1", token: "test-secret",
    fetch: async (input) => {
      const req = input as Request;
      seen.push(req);
      return Response.json(req.url.endsWith("/health")
        ? { ...health, platformVersion: "999.0.0+build", futureOptional: true } : {});
    },
  });
  await client.POST("/projects/{projectId}/executions/{id}/cancel", command);
  assert.deepEqual(seen.map(r => new URL(r.url).pathname), ["/v1/health", "/v1/projects/p/executions/e/cancel"]);
  assert.equal(seen[0]!.headers.get("authorization"), null);
  assert.equal(seen[0]!.redirect, "error");
  assert.equal(seen[0]!.cache, "no-store");
  assert.equal(seen[1]!.headers.get("authorization"), "Bearer test-secret");
});

const failures: [string, () => Response, string][] = [
  ["other major", () => Response.json({ ...health, protocolVersion: 2 }), "incompatible"],
  ["missing major", () => Response.json({ ...health, protocolVersion: undefined }), "invalid-health"],
  ["string version", () => Response.json({ ...health, protocolVersion: "1.0.0" }), "invalid-health"],
  ["fractional version", () => Response.json({ ...health, protocolVersion: 1.1 }), "invalid-health"],
  ["zero version", () => Response.json({ ...health, protocolVersion: 0 }), "invalid-health"],
  ["null", () => Response.json(null), "invalid-health"],
  ["missing health fields", () => Response.json({ protocolVersion: 1 }), "invalid-health"],
  ["invalid status", () => Response.json({ ...health, status: "broken" }), "invalid-health"],
  ["invalid uptime", () => Response.json({ ...health, uptimeSec: -1 }), "invalid-health"],
  ["invalid timestamp", () => Response.json({ ...health, startedAt: "yesterday" }), "invalid-health"],
  ["malformed JSON", () => new Response("{not-json"), "invalid-health"],
  ["HTTP failure", () => Response.json({ message: "sensitive-body" }, { status: 503 }), "connection"],
  ["unreachable", () => { throw new Error("sensitive-transport-error"); }, "connection"],
];

for (const [name, respond, kind] of failures) {
  test(`${name}: explicit safe failure state and no product mutation`, async () => {
    let calls = 0;
    const client = createClient({
      baseUrl: "http://127.0.0.1:7777/v1", token: "sensitive-token",
      fetch: async (input) => {
        calls++;
        assert.equal(new URL((input as Request).url).pathname, "/v1/health");
        return respond();
      },
    });
    await assert.rejects(client.POST("/projects/{projectId}/executions/{id}/cancel", command), error => {
      assert.ok(error instanceof CompatibilityError);
      assert.equal(error.kind, kind);
      assert.doesNotMatch(error.message, /sensitive|127\.0\.0\.1/);
      return true;
    });
    assert.equal(calls, 1);
  });
}

test("replacement Instance is checked again and a failed check can recover", async () => {
  let version = 1;
  let commands = 0;
  const client = createClient({
    baseUrl: "http://x.test/v1",
    fetch: async input => {
      if ((input as Request).url.endsWith("/health")) return Response.json({ ...health, protocolVersion: version });
      commands++;
      return Response.json({});
    },
  });
  await client.POST("/projects/{projectId}/executions/{id}/cancel", command);
  version = 2;
  await assert.rejects(client.POST("/projects/{projectId}/executions/{id}/cancel", command), { kind: "incompatible" });
  assert.equal(commands, 1);
  version = 1;
  await client.POST("/projects/{projectId}/executions/{id}/cancel", command);
  assert.equal(commands, 2);
});

test("request-specific base URL and transport are negotiated, health stays accessible", async () => {
  const seen: string[] = [];
  const client = createClient({ baseUrl: "http://unused.test/v1", fetch: async () => { throw new Error("wrong transport"); } });
  const fetch = async (input: Request | string | URL) => {
    seen.push(String(input instanceof Request ? input.url : input));
    return Response.json(health);
  };
  await client.GET("/projects", { baseUrl: "http://other.test/v1", fetch });
  assert.deepEqual(seen, ["http://other.test/v1/health", "http://other.test/v1/projects"]);
  await client.GET("/health", { baseUrl: "http://other.test/v1", fetch });
  assert.equal(seen.length, 3);
});

test("loopback HTTP negotiation rejects redirects and prevents mutation on mismatch", async t => {
  let mode = "compatible";
  let mutations = 0;
  let redirected = 0;
  const server = createServer((req, res) => {
    if (req.url === "/v1/health") {
      if (mode === "redirect") {
        res.writeHead(302, { Location: "/foreign" }).end();
        return;
      }
      res.setHeader("Content-Type", "application/json");
      res.end(JSON.stringify({ ...health, protocolVersion: mode === "compatible" ? 1 : 2 }));
    } else if (req.url === "/foreign") {
      redirected++;
      res.end(JSON.stringify(health));
    } else {
      mutations++;
      res.setHeader("Content-Type", "application/json");
      res.end("{}");
    }
  });
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  t.after(() => new Promise<void>((resolve, reject) => server.close(error => error ? reject(error) : resolve())));
  const address = server.address();
  assert.ok(address && typeof address !== "string");
  const client = createClient({ baseUrl: `http://127.0.0.1:${address.port}/v1` });
  await client.POST("/projects/{projectId}/executions/{id}/cancel", command);
  mode = "incompatible";
  await assert.rejects(client.POST("/projects/{projectId}/executions/{id}/cancel", command), { kind: "incompatible" });
  mode = "redirect";
  await assert.rejects(client.POST("/projects/{projectId}/executions/{id}/cancel", command), { kind: "connection" });
  assert.equal(mutations, 1);
  assert.equal(redirected, 0);
});
