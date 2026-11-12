import assert from "node:assert/strict";
import { test } from "node:test";
import { spawn, execFileSync } from "node:child_process";
import { mkdtemp, readFile, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { once } from "node:events";
import { createClient } from "../src/index.ts";

test("SDK authenticates and isolates Project settings against the Go loopback adapter", { skip: process.platform !== "linux", timeout: 60_000 }, async t => {
  const dir = await mkdtemp(join(tmpdir(), "amaru-sdk-auth-"));
  t.after(() => rm(dir, { recursive: true, force: true }));
  const binary = join(dir, "authfixture");
  execFileSync("go", ["build", "-tags", "authfixture", "-o", binary, "./internal/adapter/localauth/testserver"], { cwd: new URL("../../../services/platform", import.meta.url), stdio: "pipe", timeout: 45_000 });
  const state = join(dir, "state");
  const server = spawn(binary, [state], { stdio: "ignore" });
  t.after(async () => { if (server.exitCode === null) { const exit = once(server, "exit"); server.kill("SIGTERM"); await exit; } });
  let info: { endpoint: string; administratorSecret: string } | undefined;
  for (let i = 0; i < 100; i++) {
    try { info = JSON.parse(await readFile(join(state, "instance.json"), "utf8")); break; } catch { await new Promise(resolve => setTimeout(resolve, 20)); }
  }
  if (!info) throw new Error("Auth fixture did not publish discovery.");
  const baseUrl = `${info.endpoint}/v1`;
  const native = createClient({ baseUrl, token: info.administratorSecret });
  const issued = await native.POST("/auth/launch-codes");
  assert.equal(issued.response.status, 201);
  if (!issued.data) throw new Error("Missing launch response.");
  const launch = await native.GET("/auth/launch", { params: { query: { code: issued.data.code } } });
  assert.equal(launch.response.status, 303);
  assert.equal(launch.response.headers.get("Location"), "/");
  const cookie = launch.response.headers.get("Set-Cookie")?.split(";")[0];
  if (!cookie) throw new Error("Missing session cookie.");
  // Node has no browser cookie jar/Origin. This bridge tests the SDK's wire
  // requests; the separate native Firefox run proves actual browser behavior.
  const transport: typeof fetch = async (input, init) => {
    const req = new Request(input, init);
    if (req.credentials !== "omit") req.headers.set("Cookie", cookie);
    if (req.method !== "GET") req.headers.set("Origin", info!.endpoint);
    return fetch(req);
  };
  let csrf: string | undefined;
  const browser = createClient({ baseUrl, csrfToken: () => csrf, fetch: transport });
  const session = await browser.GET("/auth/session");
  assert.equal(session.response.status, 200);
  csrf = session.data?.csrfToken;
  if (!csrf) throw new Error("Missing CSRF response.");
  assert.ok(!JSON.stringify(session.data).includes(info.administratorSecret), "Session must not expose administrator credentials");
  const projectA = "ssh://github.com/acme/a#0123456789abcdef0123456789abcdef01234567";
  const projectB = "id:01ARZ3NDEKTSV4RRFFQ69G5FAV";
  const path = (projectId: string) => ({ path: { projectId } });
  const read = async (client: typeof native, projectId: string, expected: boolean) => {
    const result = await client.GET("/projects/{projectId}/settings", { params: path(projectId) });
    assert.equal(result.response.status, 200);
    assert.deepEqual(result.data, { importClaudeSkills: expected });
    assert.equal(result.response.headers.get("Cache-Control"), "no-store");
  };
  const patch = async (client: typeof native, projectId: string, value: boolean) => {
    const result = await client.PATCH("/projects/{projectId}/settings", {
      params: path(projectId), body: { importClaudeSkills: value },
    });
    assert.equal(result.response.status, 200);
    assert.deepEqual(result.data, { importClaudeSkills: value });
  };
  await t.test("two SDK clients share Project state and keep encoded Project keys isolated", async () => {
    await read(native, projectA, false);
    await read(browser, projectB, false);
    await patch(browser, projectA, true);
    await read(native, projectA, true);
    await read(native, projectB, false);
    await Promise.all([patch(native, projectA, false), patch(browser, projectB, true)]);
    await read(browser, projectA, false);
    await read(native, projectB, true);
    await patch(browser, projectB, false);
    await patch(native, projectB, false);
    await read(browser, projectB, false);
  });
  await t.test("missing Project and invalid credentials cannot update settings", async () => {
    const missing = "id:01ARZ3NDEKTSV4RRFFQ69G5FAW";
    assert.equal((await native.GET("/projects/{projectId}/settings", { params: path(missing) })).response.status, 404);
    assert.equal((await native.PATCH("/projects/{projectId}/settings", { params: path(missing), body: { importClaudeSkills: true } })).response.status, 404);
    const anonymous = createClient({ baseUrl });
    assert.equal((await anonymous.GET("/projects/{projectId}/settings", { params: path(projectA) })).response.status, 401);
    await assert.rejects(anonymous.PATCH("/projects/{projectId}/settings", { params: path(projectA), body: { importClaudeSkills: true } }), /requires a session CSRF token/);
    const wrongCSRF = createClient({ baseUrl, fetch: transport, csrfToken: () => "invalid" });
    assert.equal((await wrongCSRF.PATCH("/projects/{projectId}/settings", { params: path(projectA), body: { importClaudeSkills: true } })).response.status, 403);
    await read(native, projectA, false);
    await read(native, projectB, false);
  });
  assert.equal((await browser.DELETE("/auth/session")).response.status, 204);
  assert.equal((await browser.GET("/auth/session")).response.status, 401);
  assert.equal((await browser.GET("/projects/{projectId}/settings", { params: path(projectA) })).response.status, 401);
  assert.equal((await browser.PATCH("/projects/{projectId}/settings", { params: path(projectA), body: { importClaudeSkills: true } })).response.status, 401);
  await read(native, projectA, false);
  assert.equal((await native.GET("/auth/launch", { params: { query: { code: issued.data.code } } })).response.status, 401);
});
