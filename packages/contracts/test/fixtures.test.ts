// API-002: the shared fixture corpus (fixtures/manifest.json) validated against
// the schemas with Ajv (Draft 2020-12). fixtures_test.go reads the same
// manifest; `make contracts-parity` compares the per-fixture outcomes of both.

import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { mkdtempSync, readdirSync, readFileSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, relative, sep } from "node:path";
import { test } from "node:test";
import { fileURLToPath } from "node:url";
import { Ajv2020 } from "ajv/dist/2020.js";
import addFormats from "ajv-formats";
import type { ErrorObject } from "ajv";
import { createFileRegistry, fromBinary, fromJson, type DescMessage, type JsonValue } from "@bufbuild/protobuf";
import { FileDescriptorSetSchema } from "@bufbuild/protobuf/wkt";

const root = fileURLToPath(new URL("..", import.meta.url));
const idBase = "https://amaru.invalid/contracts/v1/schemas/";

type Outcome = "valid" | "invalid" | "malformed";
interface Entry {
  file: string;
  schema: string;
  kind: string;
  expect: Outcome;
  error?: string;
  note?: string;
  source?: string;
}
interface Manifest {
  version: number;
  entries: Entry[];
}

function walk(dir: string): string[] {
  return readdirSync(dir, { withFileTypes: true }).flatMap((d) =>
    d.isDirectory() ? walk(join(dir, d.name)) : [join(dir, d.name)],
  );
}

const manifest = JSON.parse(readFileSync(join(root, "fixtures/manifest.json"), "utf8")) as Manifest;
assert.equal(manifest.version, 1);
assert.ok(manifest.entries.length > 0, "empty manifest");

// Strict mode stays on (unknown keywords, formats, ignored $ref siblings fail). Two of its style checks are off: strictRequired
// and strictTypes reject the if/then and unevaluatedProperties idioms these schemas use, which are valid Draft 2020-12.
const ajv = new Ajv2020({ strict: true, strictRequired: false, strictTypes: false, allErrors: true });
// ajv-formats is CommonJS with a default export; under NodeNext the callable is .default.
addFormats.default(ajv);
for (const p of walk(join(root, "schemas")).filter((f) => f.endsWith(".json"))) {
  ajv.addSchema(JSON.parse(readFileSync(p, "utf8")) as object);
}

// Error ids are "keyword@/instance/path". Composition failures (allOf, if/then,
// not, oneOf, anyOf) normalize to "condition"; this matches fixtures_test.go.
const conditionKeywords = new Set(["allOf", "anyOf", "oneOf", "not", "if", "then", "else"]);

function errorId(e: ErrorObject): string {
  const keyword = conditionKeywords.has(e.keyword) ? "condition" : e.keyword;
  return `${keyword}@${e.instancePath === "" ? "/" : e.instancePath}`;
}

function run(e: Entry): { outcome: Outcome; ids: Set<string> } {
  const text = readFileSync(join(root, "fixtures", e.file), "utf8");
  let doc: unknown;
  try {
    doc = JSON.parse(text);
  } catch {
    return { outcome: "malformed", ids: new Set() };
  }
  if (e.schema.startsWith(protoPrefix)) {
    try {
      fromJson(protoMessage(e.schema.slice(protoPrefix.length)), doc as JsonValue);
      return { outcome: "valid", ids: new Set() };
    } catch {
      return { outcome: "invalid", ids: new Set(["decode@/"]) };
    }
  }
  const validate = ajv.getSchema(idBase + e.schema);
  assert.ok(validate, `schema ${e.schema} not found`);
  if (validate(doc)) return { outcome: "valid", ids: new Set() };
  return { outcome: "invalid", ids: new Set((validate.errors ?? []).map(errorId)) };
}

// Worker fixtures are protobuf JSON validated against the message descriptors
// built from proto/ by `buf build` (same source as fixtures_test.go). The error
// id of a decode failure is "decode@/": the two libraries share no vocabulary.
const protoPrefix = "proto:";
let workerRegistry: ReturnType<typeof createFileRegistry> | undefined;

function protoMessage(name: string): DescMessage {
  if (!workerRegistry) {
    const out = join(mkdtempSync(join(tmpdir(), "amaru-contracts-")), "worker.binpb");
    execFileSync("buf", ["build", "-o", out], { cwd: root });
    workerRegistry = createFileRegistry(fromBinary(FileDescriptorSetSchema, readFileSync(out)));
  }
  const m = workerRegistry.getMessage(name);
  assert.ok(m, `message ${name} not found`);
  return m;
}

const outcomes = new Map<string, Outcome>();

test("every fixture has its declared outcome and error id", async (t) => {
  for (const e of manifest.entries) {
    await t.test(e.file, () => {
      const { outcome, ids } = run(e);
      outcomes.set(e.file, outcome);
      assert.equal(outcome, e.expect);
      if (e.expect === "invalid") {
        assert.ok(e.error && ids.has(e.error), `error id ${e.error} not reported; got ${[...ids].sort().join(", ")}`);
      }
    });
  }
  const out = process.env["CONTRACTS_OUTCOMES_OUT"];
  if (out) {
    const lines = [...outcomes.entries()].sort(([a], [b]) => (a < b ? -1 : a > b ? 1 : 0)).map(([f, o]) => `${f}\t${o}\n`);
    writeFileSync(out, lines.join(""));
  }
});

test("manifest and directory describe the same files", () => {
  const listed = new Set(manifest.entries.map((e) => e.file));
  assert.equal(listed.size, manifest.entries.length, "a file is listed twice");
  const onDisk = walk(join(root, "fixtures"))
    .map((p) => relative(join(root, "fixtures"), p).split(sep).join("/"))
    .filter((f) => f !== "manifest.json");
  assert.deepEqual(new Set(onDisk), listed);
});
