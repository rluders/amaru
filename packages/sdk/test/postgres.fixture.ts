// Invoked only by the PostgreSQL-tagged Go acceptance test, outside the default Node suite.
import assert from "node:assert/strict";
import { readFile, writeFile } from "node:fs/promises";
import { createClient } from "../src/index.ts";
import type { components } from "../src/index.ts";

const env = (name: string) => {
  const value = process.env[name];
  if (!value) throw new Error(`Missing fixture input: ${name}`);
  return value;
};
const client = createClient({ baseUrl: env("AMARU_SDK_ENDPOINT"), token: env("AMARU_SDK_TOKEN") });
const snapshot = env("AMARU_SDK_SNAPSHOT");
type Project = components["schemas"]["Project"];
const list = async () => {
  const result = await client.GET("/projects");
  assert.equal(result.response.status, 200);
  assert.ok(result.data);
  return result.data;
};
const settings = async (projectId: string, expected: boolean) => {
  const result = await client.GET("/projects/{projectId}/settings", { params: { path: { projectId } } });
  assert.equal(result.response.status, 200);
  assert.deepEqual(result.data, { importClaudeSkills: expected });
};
let projects: Project[];
if (env("AMARU_SDK_PHASE") === "write") {
  assert.deepEqual(await list(), []);
  projects = [];
  for (const name of ["AMARU_SDK_ROOT_A", "AMARU_SDK_ROOT_B"]) {
    const result = await client.POST("/projects", { body: { root: env(name) } });
    assert.equal(result.response.status, 200);
    assert.ok(result.data);
    projects.push(result.data);
  }
  assert.notEqual(projects[0]!.id, projects[1]!.id);
  await settings(projects[0]!.id, false);
  await settings(projects[1]!.id, false);
  const before = await list();
  const result = await client.PATCH("/projects/{projectId}/settings", {
    params: { path: { projectId: projects[0]!.id } }, body: { importClaudeSkills: true },
  });
  assert.equal(result.response.status, 200);
  assert.deepEqual(result.data, { importClaudeSkills: true });
  assert.deepEqual(await list(), before, "Settings update must preserve both Project resources");
  await writeFile(snapshot, JSON.stringify({ projects, listed: before }), { mode: 0o600 });
} else {
  const saved: { projects: Project[]; listed: Project[] } = JSON.parse(await readFile(snapshot, "utf8"));
  projects = saved.projects;
  assert.deepEqual(await list(), saved.listed, "Project metadata and clone bindings must survive fixture recreation");
}
for (const project of projects) {
  const detail = await client.GET("/projects/{projectId}", { params: { path: { projectId: project.id } } });
  assert.equal(detail.response.status, 200);
  assert.deepEqual(detail.data, project);
}
await settings(projects[0]!.id, true);
await settings(projects[1]!.id, false);
