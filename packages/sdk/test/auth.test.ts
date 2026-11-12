import assert from "node:assert/strict";
import { test } from "node:test";
import { spawn, execFileSync } from "node:child_process";
import { mkdtemp, readFile, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { once } from "node:events";
import { createClient } from "../src/index.ts";

test("SDK negotiates and authenticates against the Go loopback adapter", { skip: process.platform !== "linux", timeout: 60_000 }, async t => {
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
  assert.equal((await browser.DELETE("/auth/session")).response.status, 204);
  assert.equal((await browser.GET("/auth/session")).response.status, 401);
  assert.equal((await native.GET("/auth/launch", { params: { query: { code: issued.data.code } } })).response.status, 401);
});
