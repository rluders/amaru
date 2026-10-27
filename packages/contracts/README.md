# packages/contracts

The versioned, UI-agnostic contract of the Amaru Platform: the public HTTP API, the event stream, the external Provider protocol and the Worker protocol. Clients, Workers and plugins depend on this package; the Platform (M3) must serve exactly what it declares.

**Status: proposed, 0.x.** Nothing serves these contracts yet: there is no Platform, SDK or Worker in the repository. What exists and is checked here is the contract files, the route-drift guard and the shared fixture corpus validated in Go and TypeScript. Versioning rules: [CONTRACT_VERSIONING](../../docs/architecture/CONTRACT_VERSIONING.md), [ADR-0025](../../docs/architecture/adr/0025-api-and-protocol-versioning-policy.md). Origin of the files: [PROVENANCE](PROVENANCE.md) ([ADR-0047](../../docs/architecture/adr/0047-migrate-the-prototype-protocol-into-packages-contracts.md)). Owner: the Platform contract owner. Work: WP-M2-01 ([#9](https://github.com/rluders/amaru/issues/9)), API-001 ([#68](https://github.com/rluders/amaru/issues/68)), fixtures API-002 ([#117](https://github.com/rluders/amaru/issues/117)).

License: Apache-2.0 for contract packages is decided in [ADR-0026](../../docs/architecture/adr/0026-fair-code-community-license-with-commercial-licensing.md). The package [`LICENSE`](LICENSE) is the unmodified Apache License 2.0 text from <https://www.apache.org/licenses/LICENSE-2.0.txt>, added at the product owner's explicit instruction on 2026-10-26.

## Layout

| Path | Content |
|---|---|
| `openapi/openapi.yaml` | OpenAPI 3.1, routes under `/v1` (29 paths, 44 operations). Shapes are `$ref`s into `schemas/`. |
| `schemas/*.json` | JSON Schema 2020-12, one file per type: resources, requests, Approval, Policy, Grant, Principal, usage. |
| `schemas/events/*.json` | Event stream frames (`Hello`, `ResyncRequired`, `Event` and each event type). |
| `schemas/provider/*.json` | External Provider protocol `amaru.provider/v1` (stdio, one JSON object per line). |
| `proto/amaru/worker/v1/worker.proto` | Worker protocol `amaru.worker.v1` (WebSocket + Protobuf, [ADR-0029](../../docs/architecture/adr/0029-worker-protocol-over-websocket-and-protobuf.md)); design in [WORKER_PROTOCOL](../../docs/architecture/WORKER_PROTOCOL.md). |
| `baseline/worker-v1.binpb` | Buf image that `buf breaking` compares against (see below). |
| `routeguard/` | Reusable route-drift guard for the Platform's tests (M3). |
| `fixtures/` | Shared fixture corpus (`manifest.json` and `<group>/v1/<Type>/<name>.json`), see [Fixtures](#fixtures). |
| `fixtures_test.go`, `test/fixtures.test.ts` | The Go and TypeScript suites that read the corpus. `package.json`, `pnpm-lock.yaml` and `tsconfig.json` are their test tooling only (not published; the SDK is WP-M2-02). |
| `contract_test.go` | Structural checks: unique `$id`s, every `$ref` resolves, unique `operationId`s, no prototype names. |

## Decisions made in this package

- **Delivery-loop route:** `GET /v1/projects/{projectId}/work-packages/{id}/delivery-loop` returns `DeliveryLoopLedger`. The prototype registered `/work-packages/{id}/autoloop` without declaring it; the name follows the glossary (Autoloop → Delivery loop) and the kebab-case of the other path segments.
- **Events (WRK-003):** every event has `id` (deduplication), `seq` (the per-Project cursor, also the resume point), `resource`, optional `correlationId`. A cursor that cannot be served yields a `ResyncRequired` frame, not a socket close code.
- **Usage (OBS-002, ADR-0033):** `UsageRecord` and the Protobuf `UsageReport` carry each number with a `source` (`reported`, `estimated`, `unknown`); unknown has no value, never zero. Cost is a decimal string plus currency.
- **No merge anywhere:** the delivery-loop policy has no merge switch and the ledger no merge phase (baseline rule 2).
- **`buf breaking` baseline:** a committed Buf image, `baseline/worker-v1.binpb`, built with `buf build -o baseline/worker-v1.binpb` from the proto sources. It is the frozen state of the last released contract and is rebuilt only at a contracts release; until the first release it equals the current sources, so the gate is real (it fails on a breaking edit, shown below) but compares against the draft. This answers the open point in [CONTRACT_VERSIONING](../../docs/architecture/CONTRACT_VERSIONING.md#2-buf-breaking-for-protobuf-packages); the choice is proposed for review.

### Pieces decided by ADR-0048

[ADR-0048](../../docs/architecture/adr/0048-approval-reuse-uncovered-capabilities-and-policy-defaults.md) was **accepted by the product owner on 2026-10-26**; Q56–Q58 are closed. The contract carries its shapes:

- `Approval.occurrence` and `ApprovalOccurrence`, `replaces`, and the use-time digest rule (Q56);
- reason codes `policy.explicit_deny` and `policy.uncovered`, and `Principal` kind `trigger` requiring a `sponsor` (Q57). The reason `code` is an open string pattern;
- no merge Capability and the `scm.pr.create` default (Q58) are defaults in configuration, not contract shapes. The delivery-loop policy has no merge switch.

These shapes are decided and have fixtures (an Approval and its decision, a replacement after a changed digest, a Trigger with and without a sponsor, explicit-deny and uncovered decisions, `needs_grant` rejected as a Policy decision), but no Platform implements them yet; the M3 crash tests are still to come.

## Not in this package yet

| Item | Where |
|---|---|
| Replay of *released* fixtures against the current schema (the additive-only check) | The first contracts release; until then the corpus describes the draft |
| Consumer behavior with unknown fields, unknown enum values and unknown Worker `oneof` bodies | SDK (WP-M2-02), Platform (M3); a schema validator cannot show it |
| Generated TypeScript SDK | WP-M2-02 ([#10](https://github.com/rluders/amaru/issues/10)) |
| The guard wired to a real router (the Platform does not exist) | WP-M2-03 ([#11](https://github.com/rluders/amaru/issues/11)), M3 |
| Plugin manifest schema | M6 |
| Identity endpoints | M7 |
| Routes for Grants, Policy evaluation, usage roll-ups, Skills, templates, hooks, intake | later features; shapes for Grant, Policy and usage exist, routes do not |
| Receiver behavior for unknown Worker `oneof` bodies across minor versions; `bytes` vs `Struct` for `inputs_json` | open points of [WORKER_PROTOCOL §8](../../docs/architecture/WORKER_PROTOCOL.md#8-open-points-for-the-m2-contract-work) |

## Commands

From the repository root: `make check` runs everything below among other checks.

```
make contracts-test    # go vet + go test in packages/contracts (structural checks, routeguard)
make contracts-proto   # buf lint, buf format -d --exit-code, buf breaking against the baseline image
make contracts-ts      # pnpm install --frozen-lockfile, tsc --noEmit, the fixtures with Ajv, and Go/TypeScript parity
```

Requirements: Go (the version in `go.mod`), Node >= 22.18 and pnpm for `contracts-ts` (verified with Node 25.7 and pnpm 11.5.0), and the `buf` CLI (verified with 1.69.0 on 2026-10-26; configuration key names checked against the [buf documentation](https://buf.build/docs/breaking/quickstart), read 2026-10-26).

Breaking-change gate shown to work (not part of `make check`): changing `uint64 fencing_token = 2` to `uint32` in a copy of the sources makes `buf breaking --against baseline/worker-v1.binpb` report `Field "2" with name "fencing_token" on message "Lease" changed type from "uint64" to "uint32"` and exit non-zero.

## Using the route guard (M3)

```go
func TestRoutesMatchContract(t *testing.T) {
	f, _ := os.Open("…/packages/contracts/openapi/openapi.yaml")
	defer f.Close()
	routeguard.AssertMatch(t, f, registeredPatterns) // "METHOD /v1/path/{param}", e.g. from the mux setup
}
```

It fails in both directions: a registered route missing from the contract (as `/autoloop` was) and a contract route nobody serves. Path parameter names are ignored; methods are not. Routes deliberately not yet served are passed to `AssertMatch` as `allowUnserved`, and each must still exist in the contract. The guard has not been run against a real router (none exists); its tests use the contract itself and mutations of it.

## Dependencies

| Dependency | Why | Version | License |
|---|---|---|---|
| `gopkg.in/yaml.v3` | The Go standard library cannot read YAML, and OpenAPI is authored in YAML. Used by `routeguard` and `contract_test.go` only. | v3.0.1 (pinned in `go.mod`, hashes in `go.sum`) | MIT and Apache-2.0 |
| `buf` CLI (tool, not a Go dependency) | Lint, format and breaking-change detection for Protobuf. | verified with 1.69.0 | Apache-2.0 |

| `github.com/santhosh-tekuri/jsonschema/v6` | Go validator for Draft 2020-12, with format assertion. Already used by the prototype's Go consumer. Used by `fixtures_test.go` only. | v6.0.3 (`go.mod`, hashes in `go.sum`) | Apache-2.0 |
| `google.golang.org/protobuf` | Reads the descriptors made by `buf build` and decodes Worker fixtures as protobuf JSON (`dynamicpb`, `protojson`) without generated code. Test only. | v1.36.12 | BSD-3-Clause |
| `golang.org/x/text` (indirect) | Required by the JSON Schema library. | v0.14.0 | BSD-3-Clause |
| `ajv` and `ajv-formats` | The TypeScript Draft 2020-12 validator (`ajv/dist/2020`) and the `date-time` format. Test tooling only. | 8.17.1 and 3.0.1 (exact in `package.json`, resolved in `pnpm-lock.yaml`) | MIT |
| `@bufbuild/protobuf` | Reads the `buf build` descriptor set and decodes Worker fixtures (`createFileRegistry`, `fromJson`) without generated code. Test tooling only. | 2.16.0 | Apache-2.0 and BSD-3-Clause |
| `typescript`, `@types/node` | `tsc --noEmit` on the test; Node runs the `.ts` test by type stripping. | 5.9.3 and 22.20.5 | Apache-2.0 and MIT |

Docs read for the APIs used: [Ajv](https://ajv.js.org/) (`Ajv2020`, `addSchema`, `getSchema`, error objects), [santhosh-tekuri/jsonschema](https://github.com/santhosh-tekuri/jsonschema) (`AddResource`, `Compile`, `ValidationError`), [protobuf-es](https://github.com/bufbuild/protobuf-es) (`createFileRegistry`, `fromJson`), [protobuf-go](https://github.com/protocolbuffers/protobuf-go) (`protodesc.NewFiles`, `dynamicpb`, `protojson`), read 2026-10-26 through the documentation index; the behavior listed under Fixtures was then confirmed by running the suites.

## Fixtures

One corpus, two consumers: `fixtures_test.go` (Go) and `test/fixtures.test.ts` (TypeScript) read the same `fixtures/manifest.json`. Layout: `fixtures/<group>/v1/<Type>/<name>.json`, `group` being `api`, `events`, `provider-protocol` or `worker`. Each manifest entry:

| Field | Meaning |
|---|---|
| `file` | Path under `fixtures/`. The manifest and the directory must list the same files; a file missing from either fails. |
| `schema` | File under `schemas/` (for example `Approval.json`, `events/Event.json`), or `proto:amaru.worker.v1.Envelope` for Worker fixtures. |
| `kind` / `expect` | `valid` and `forward-compat` expect `valid`; `invalid` expects `invalid`; `malformed` (not one JSON document: truncated, trailing comma, empty, two documents) expects both parsers to refuse it. |
| `error` | Required for `invalid`: the stable error id `keyword@/instance/path`. Both validators must report it. Composition failures (`allOf`, `if`/`then`, `not`, `oneOf`, `anyOf`) normalize to `condition`; Worker fixtures use `decode@/`. |
| `note` | Required for `forward-compat`; used where the reason is a decision (ADR-0033, ADR-0044, ADR-0048). |
| `source` | For imported fixtures, the prototype path (see [PROVENANCE](PROVENANCE.md)). |

Counts at this commit: 315 fixtures (154 valid, 154 invalid, 1 forward-compat, 6 malformed); 208 re-validated from the prototype, 107 new. The new ones cover Approval (states, decision, voiding and replacement, mismatched outcome, bad digest, agent requester without sponsor), `DecideApproval`, Principal (Trigger with and without sponsor), `PolicyRequest`, `PolicyDecision` (allow, `require_approval`, `policy.explicit_deny`, `policy.uncovered`, `needs_grant` rejected as a decision), `Grant`, usage (each number with its source, unknown never zero, cost as a decimal string with currency), Step Execution `timed_out`, `interrupted` and `refused`, the events added in WRK-003 and ADR-0033/0036/0044/0048, `ResyncRequired` and the `Frame`, and the Worker envelope (assignment with Lease and fencing token, renew, revoke, reject, result with usage, result failed on timeout, two bodies set, bad enum, bad number, bad timestamp, truncated JSON).

Checks beyond validation: every area in the list in `TestEveryAreaIsCovered` has at least one valid and one invalid fixture; no file name differs only by case (the corpus must check out on macOS and Windows); a scan rejects secret-like values (private keys, common token prefixes, `password`/`token` assignments) and the prototype names.

`make contracts-ts` runs both suites with `CONTRACTS_OUTCOMES_OUT`, each writing one `fixture<TAB>outcome` line per fixture, and fails on any difference: the parity check. It was shown to catch a broken fixture (changing a valid Approval's `state` to an invalid value fails both suites).

What the corpus does not show: the unknown-field behavior of Worker receivers (open point of [WORKER_PROTOCOL §8](../../docs/architecture/WORKER_PROTOCOL.md#8-open-points-for-the-m2-contract-work), so no fixture settles it); that a Platform produces these payloads; and `format` assertions beyond `date-time` (the only format the schemas use). Ajv runs with strict mode on except `strictRequired` and `strictTypes`, which reject the `if`/`then` and `unevaluatedProperties` idioms used here (valid Draft 2020-12). Worker fixtures are checked against descriptors built by `buf build` from the current sources, not the baseline image.

Rules for changes: a fixture file is not edited to make it pass; fix the schema or delete the fixture with a reason in the PR. A new field adds a fixture. Fixtures contain placeholder values only.

## Generated files

None. Everything here is hand-maintained; `baseline/worker-v1.binpb` is a release artifact built by the command above, not hand-edited.
