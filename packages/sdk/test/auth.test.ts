import assert from "node:assert/strict";
import { test } from "node:test";
import { spawn, execFileSync } from "node:child_process";
import { mkdtemp, mkdir, readFile, writeFile, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { once } from "node:events";
import { createClient } from "../src/index.ts";
import type { components } from "../src/index.ts";

test("SDK attaches real Git Projects and isolates settings against the Go loopback adapters", { skip: process.platform !== "linux", timeout: 60_000 }, async t => {
  const dir = await mkdtemp(join(tmpdir(), "amaru-sdk-auth-"));
  t.after(() => rm(dir, { recursive: true, force: true }));
  const binary = join(dir, "authfixture");
  execFileSync("go", ["build", "-tags", "authfixture", "-o", binary, "./internal/adapter/localauth/testserver"], { cwd: new URL("../../../services/platform", import.meta.url), stdio: "pipe", timeout: 45_000 });
  const state = join(dir, "state");
  const server = spawn(binary, [state], { stdio: ["ignore", "pipe", "pipe"] });
  const stdout: Buffer[] = [];
  const stderr: Buffer[] = [];
  server.stdout.on("data", chunk => stdout.push(Buffer.from(chunk)));
  server.stderr.on("data", chunk => stderr.push(Buffer.from(chunk)));
  // Await close, rather than exit, so both diagnostic streams are fully drained.
  const closed = once(server, "close");
  const stop = async () => {
    if (server.exitCode === null && server.signalCode === null) server.kill("SIGTERM");
    await closed;
  };
  t.after(stop);
  let info: { endpoint: string; administratorSecret: string } | undefined;
  for (let i = 0; i < 100; i++) {
    try { info = JSON.parse(await readFile(join(state, "instance.json"), "utf8")); break; } catch { await new Promise(resolve => setTimeout(resolve, 20)); }
  }
  if (!info) throw new Error("Auth fixture did not publish discovery.");
  const baseUrl = `${info.endpoint}/v1`;
  const statuses: number[] = [];
  const untrustedCorrelation = "sdk-untrusted-correlation-marker";
  const trackedFetch: typeof fetch = async (input, init) => {
    const request = new Request(input, init);
    request.headers.set("X-Correlation-ID", untrustedCorrelation);
    const response = await fetch(request);
    statuses.push(response.status);
    return response;
  };
  const native = createClient({ baseUrl, fetch: trackedFetch, token: info.administratorSecret });
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
    return trackedFetch(req);
  };
  let csrf: string | undefined;
  const browser = createClient({ baseUrl, csrfToken: () => csrf, fetch: transport });
  const session = await browser.GET("/auth/session");
  assert.equal(session.response.status, 200);
  csrf = session.data?.csrfToken;
  if (!csrf) throw new Error("Missing CSRF response.");
  assert.ok(!JSON.stringify(session.data).includes(info.administratorSecret), "Session must not expose administrator credentials");
  const git = (root: string, ...args: string[]) => execFileSync("git", args, {
    cwd: root, stdio: "pipe", timeout: 5_000,
    env: { ...process.env, GIT_CONFIG_GLOBAL: "/dev/null", GIT_CONFIG_NOSYSTEM: "1",
      GIT_AUTHOR_NAME: "SDK fixture", GIT_AUTHOR_EMAIL: "sdk@example.com",
      GIT_COMMITTER_NAME: "SDK fixture", GIT_COMMITTER_EMAIL: "sdk@example.com" },
  });
  const roots = [join(dir, "project-a"), join(dir, "project-b")];
  for (const [i, root] of roots.entries()) {
    await mkdir(join(root, ".amaru"), { recursive: true });
    git(root, "init", "-q", "-b", "main");
    await writeFile(join(root, ".amaru/config.yaml"), "project:\n  name: same\n" +
      (i === 1 ? "  id: 01ARZ3NDEKTSV4RRFFQ69G5FAV\n" : ""));
    git(root, "add", ".");
    git(root, "commit", "-q", "-m", "initial");
    if (i === 0) git(root, "remote", "add", "origin", "https://github.com/acme/a.git");
  }
  const rejectedAttach = async (client: typeof native, root: string, status: number) => {
    const before = await native.GET("/projects");
    assert.equal(before.response.status, 200);
    const result = await client.POST("/projects", { body: { root } });
    assert.equal(result.response.status, status);
    assert.equal(result.data, undefined);
    const after = await native.GET("/projects");
    assert.equal(after.response.status, 200);
    assert.deepEqual(after.data, before.data, "Rejected attach must preserve registration and active clone");
  };
  await t.test("rejected credentials and browser Origins cannot register Projects", async () => {
    await rejectedAttach(createClient({ baseUrl, fetch: trackedFetch, token: "invalid" }), roots[0]!, 401);
    // Supplying CSRF satisfies the SDK preflight; the server must still refuse
    // a request without authentication rather than register the inspected root.
    await rejectedAttach(createClient({ baseUrl, fetch: trackedFetch, csrfToken: () => csrf }), roots[0]!, 401);
    for (const origin of [undefined, "https://foreign.example"]) {
      const fetchOrigin: typeof fetch = async (input, init) => {
        const req = new Request(input, init);
        req.headers.set("Cookie", cookie);
        if (origin && req.method !== "GET") req.headers.set("Origin", origin);
        else req.headers.delete("Origin");
        return trackedFetch(req);
      };
      await rejectedAttach(createClient({ baseUrl, csrfToken: () => csrf, fetch: fetchOrigin }), roots[1]!, 403);
    }
  });
  assert.deepEqual((await native.GET("/projects")).data, []);
  const attached: components["schemas"]["Project"][] = [];
  for (const [i, client] of [native, browser].entries()) {
    const result = await client.POST("/projects", { body: { root: join(roots[i]!, ".amaru") } });
    assert.equal(result.response.status, 200);
    assert.ok(result.data);
    assert.equal(result.data.name, "same");
    assert.equal(result.data.root, roots[i]);
    assert.deepEqual(result.data.git, { present: true });
    assert.deepEqual(result.data.config, { present: true, path: ".amaru/config.yaml" });
    assert.equal(result.data.repositories, undefined);
    attached.push(result.data);
  }
  const projectA = attached[0]!.id;
  const projectB = attached[1]!.id;
  assert.equal(projectA, `ssh://github.com/acme/a#${git(roots[0]!, "rev-parse", "HEAD").toString().trim()}`);
  assert.equal(projectB, "id:01ARZ3NDEKTSV4RRFFQ69G5FAV");
  assert.notEqual(projectA, projectB);
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
    const missingProject = await native.GET("/projects/{projectId}", { params: path(missing) });
    assert.equal(missingProject.response.status, 404);
    assert.deepEqual(missingProject.error, { error: { code: "not_found", message: "Request rejected." } });
    const invalidRoot = await native.POST("/projects", { body: { root: "relative" } });
    assert.equal(invalidRoot.response.status, 400);
    assert.deepEqual(invalidRoot.error, { error: { code: "invalid_request", message: "Request rejected." } });
    const anonymous = createClient({ baseUrl, fetch: trackedFetch });
    assert.equal((await anonymous.GET("/projects/{projectId}/settings", { params: path(projectA) })).response.status, 401);
    await assert.rejects(anonymous.PATCH("/projects/{projectId}/settings", { params: path(projectA), body: { importClaudeSkills: true } }), /requires a session CSRF token/);
    const wrongCSRF = createClient({ baseUrl, fetch: transport, csrfToken: () => "invalid" });
    assert.equal((await wrongCSRF.PATCH("/projects/{projectId}/settings", { params: path(projectA), body: { importClaudeSkills: true } })).response.status, 403);
    assert.equal((await wrongCSRF.POST("/projects", { body: { root: roots[0]! } })).response.status, 403);
    await read(native, projectA, false);
    await read(native, projectB, false);
  });
  await t.test("Project SDK readback, clone reselection and redacted failures", async () => {
    const expected = [...attached].sort((a, b) => a.id < b.id ? -1 : a.id > b.id ? 1 : 0);
    // Committed metadata wins over a dirty malformed working file.
    await writeFile(join(roots[0]!, ".amaru/config.yaml"), "[invalid");
    for (const client of [native, browser]) {
      const listed = await client.GET("/projects");
      assert.equal(listed.response.status, 200);
      assert.deepEqual(listed.data, expected);
      for (const project of attached) {
        const detail = await client.GET("/projects/{projectId}", { params: path(project.id) });
        assert.equal(detail.response.status, 200);
        assert.deepEqual(detail.data, project);
      }
    }
    const clone = join(dir, "replacement");
    git(dir, "clone", "-q", roots[0]!, clone);
    git(clone, "remote", "set-url", "origin", "https://github.com/acme/a.git");
    await writeFile(join(clone, ".amaru/config.yaml"), "project:\n  name: changed suggestion\n");
    git(clone, "add", ".");
    git(clone, "commit", "-q", "-m", "different suggested name");
    const wrongCSRF = createClient({ baseUrl, fetch: transport, csrfToken: () => "invalid" });
    await rejectedAttach(wrongCSRF, clone, 403);
    await read(native, projectA, false);
    await read(browser, projectB, false);
    await patch(browser, projectA, true);
    for (const client of [native, browser]) {
      const result = await client.POST("/projects", { body: { root: clone } });
      assert.equal(result.response.status, 200);
      assert.deepEqual(result.data, { ...attached[0], root: clone });
    }
    await read(native, projectA, true);
    await read(browser, projectB, false);
    git(clone, "remote", "set-url", "origin", "https://github.com/acme/foreign.git");
    const failures = [
      await browser.POST("/projects", { body: { root: clone } }),
      await native.GET("/projects/{projectId}", { params: path(projectA) }),
      await browser.GET("/projects"),
    ];
    for (const result of failures) {
      assert.equal(result.response.status, 409);
      assert.equal(result.data, undefined);
      assert.deepEqual(result.error, { error: { code: "conflict", message: "Request rejected." } });
    }
    await rm(clone, { recursive: true });
    assert.equal((await native.GET("/projects/{projectId}", { params: path(projectA) })).response.status, 409);
    // Recovery requires explicit reattach, even though the original root remains.
    assert.equal((await native.POST("/projects", { body: { root: roots[0]! } })).response.status, 200);
    assert.deepEqual((await browser.GET("/projects/{projectId}", { params: path(projectA) })).data, attached[0]);
    await read(native, projectA, true);
    await patch(native, projectA, false);
  });
  const rejectedSecrets = {
    bearer: "rejected-bearer-sensitive-marker",
    launch: "rejected-launch-sensitive/marker+value",
    csrf: "rejected-csrf-sensitive-marker",
    cookie: "rejected-cookie-sensitive-marker",
  };
  await t.test("rejected auth credentials never enter response bodies or diagnostic channels", async () => {
    const invalidBearer = createClient({ baseUrl, fetch: trackedFetch, token: rejectedSecrets.bearer });
    const invalidCSRF = createClient({ baseUrl, fetch: transport, csrfToken: () => rejectedSecrets.csrf });
    const cookieFetch: typeof fetch = async (input, init) => {
      const req = new Request(input, init);
      req.headers.set("Cookie", `${cookie.split("=")[0]}=${rejectedSecrets.cookie}`);
      return trackedFetch(req);
    };
    const invalidCookie = createClient({ baseUrl, fetch: cookieFetch });
    const results = [
      [await invalidBearer.GET("/auth/session"), 401],
      [await native.GET("/auth/launch", { params: { query: { code: rejectedSecrets.launch } } }), 401],
      [await invalidCSRF.DELETE("/auth/session"), 403],
      [await invalidCookie.GET("/auth/session"), 401],
    ] as const;
    for (const [result, status] of results) {
      assert.equal(result.response.status, status);
      assert.equal(result.data, undefined);
      const body = JSON.stringify(result.error);
      for (const sensitive of Object.values(rejectedSecrets)) {
        assert.ok(!body.includes(sensitive), "Rejected credential echoed in response body");
      }
    }
    // A rejected logout must leave the legitimate session live.
    assert.equal((await browser.GET("/auth/session")).response.status, 200);
  });
  assert.equal((await browser.DELETE("/auth/session")).response.status, 204);
  assert.equal((await browser.GET("/auth/session")).response.status, 401);
  assert.equal((await browser.GET("/projects/{projectId}/settings", { params: path(projectA) })).response.status, 401);
  assert.equal((await browser.PATCH("/projects/{projectId}/settings", { params: path(projectA), body: { importClaudeSkills: true } })).response.status, 401);
  await rejectedAttach(browser, roots[0]!, 401);
  assert.equal((await browser.GET("/projects")).response.status, 401);
  assert.equal((await browser.GET("/projects/{projectId}", { params: path(projectA) })).response.status, 401);
  await read(native, projectA, false);
  assert.deepEqual((await native.GET("/projects")).data, [...attached].sort((a, b) => a.id < b.id ? -1 : 1));
  assert.equal((await native.GET("/auth/launch", { params: { query: { code: issued.data.code } } })).response.status, 401);
  await t.test("Project SDK diagnostics correlate every request without exposing credentials or input", async () => {
    await stop();
    assert.equal(server.exitCode, 0, "Fixture must shut down successfully");
    const output = Buffer.concat(stdout).toString("utf8");
    const errors = Buffer.concat(stderr).toString("utf8");
    for (const sensitive of [info.administratorSecret, issued.data!.code, cookie, cookie.split("=").slice(1).join("="), csrf!,
      ...Object.values(rejectedSecrets), encodeURIComponent(rejectedSecrets.launch),
      untrustedCorrelation, dir, ...roots, projectA, projectB, "https://foreign.example", "relative", "changed suggestion"]) {
      assert.ok(sensitive.length > 0);
      // Do not include captured output or secret values in assertion diagnostics.
      assert.ok(!output.includes(sensitive) && !errors.includes(sensitive), "Captured diagnostics contain sensitive input");
    }
    assert.equal(errors.length, 0, "Unexpected fixture stderr diagnostics");
    const lines = output.trim().split("\n");
    const correlations = new Set<string>();
    const loggedStatuses: number[] = [];
    for (const line of lines) {
      let record: Record<string, unknown>;
      try { record = JSON.parse(line); } catch { throw new Error("Fixture emitted non-JSON operational output"); }
      assert.deepEqual(Object.keys(record).sort(), ["correlationId", "instanceId", "operation", "outcome", "status", "time"]);
      assert.equal(record.instanceId, "sdk-fixture");
      assert.equal(record.operation, "http.request");
      assert.equal(typeof record.time, "string");
      assert.ok(Number.isFinite(Date.parse(record.time as string)));
      assert.equal(typeof record.correlationId, "string");
      const id = record.correlationId as string;
      assert.ok(id.length > 0 && !correlations.has(id), "Each HTTP request needs a fresh trusted correlation");
      correlations.add(id);
      assert.equal(typeof record.status, "number");
      const status = record.status as number;
      assert.equal(record.outcome, status >= 400 ? "rejected" : "succeeded");
      loggedStatuses.push(status);
    }
    // Compare multisets: concurrent settings writes need not log in response order.
    assert.deepEqual(loggedStatuses.sort((a, b) => a - b), [...statuses].sort((a, b) => a - b));
    for (const status of [200, 201, 204, 303, 400, 401, 403, 404, 409]) {
      assert.ok(loggedStatuses.includes(status), "Missing successful or rejected request coverage");
    }
  });
});
