// Generates src/generated/schema.ts from the OpenAPI contract in packages/contracts.
//   node scripts/generate.ts          rewrite the file
//   node scripts/generate.ts --check  fail if the committed file differs from a fresh generation (staleness check)
import { readFileSync, writeFileSync } from "node:fs";
import openapiTS, { astToString } from "openapi-typescript";

const source = new URL("../../contracts/openapi/openapi.yaml", import.meta.url);
const target = new URL("../src/generated/schema.ts", import.meta.url);

const header = [
  "/**",
  " * GENERATED FILE. DO NOT EDIT.",
  " * Source: packages/contracts/openapi/openapi.yaml, by openapi-typescript.",
  " * Regenerate with `pnpm run generate` in packages/sdk; `make sdk-ts` fails when this file is stale.",
  " */",
  "",
].join("\n");

const generated = header + astToString(await openapiTS(source));

if (process.argv.includes("--check")) {
  let committed = "";
  try {
    committed = readFileSync(target, "utf8");
  } catch {
    // missing file is stale
  }
  if (committed !== generated) {
    console.error("packages/sdk/src/generated/schema.ts is stale: run `pnpm run generate` in packages/sdk and commit the result.");
    process.exit(1);
  }
  console.log("generated SDK types are up to date");
} else {
  writeFileSync(target, generated);
  console.log(`wrote ${target.pathname}`);
}
