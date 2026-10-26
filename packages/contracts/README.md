# packages/contracts

The versioned, UI-agnostic contract of the Amaru Platform: the public HTTP API, the event stream, the external Provider protocol and the Worker protocol. Clients, Workers and plugins depend on this package; the Platform (M3) must serve exactly what it declares.

**Status: proposed, 0.x.** Nothing serves these contracts yet: there is no Platform, SDK or Worker in the repository. What exists and is checked here is the contract files and the route-drift guard. Versioning rules: [CONTRACT_VERSIONING](../../docs/architecture/CONTRACT_VERSIONING.md), [ADR-0025](../../docs/architecture/adr/0025-api-and-protocol-versioning-policy.md). Origin of the files: [PROVENANCE](PROVENANCE.md) ([ADR-0047](../../docs/architecture/adr/0047-migrate-the-prototype-protocol-into-packages-contracts.md)). Owner: the Platform contract owner. Work: WP-M2-01 ([#9](https://github.com/rluders/amaru/issues/9)), API-001 ([#68](https://github.com/rluders/amaru/issues/68)), fixtures API-002 ([#117](https://github.com/rluders/amaru/issues/117)).

License: Apache-2.0 for contract packages is decided in [ADR-0026](../../docs/architecture/adr/0026-fair-code-community-license-with-commercial-licensing.md). The package `LICENSE` file is added by the product owner; it is not in this change.

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
| `contract_test.go` | Structural checks: unique `$id`s, every `$ref` resolves, unique `operationId`s, no prototype names. |

## Decisions made in this package

- **Delivery-loop route:** `GET /v1/projects/{projectId}/work-packages/{id}/delivery-loop` returns `DeliveryLoopLedger`. The prototype registered `/work-packages/{id}/autoloop` without declaring it; the name follows the glossary (Autoloop → Delivery loop) and the kebab-case of the other path segments.
- **Events (WRK-003):** every event has `id` (deduplication), `seq` (the per-Project cursor, also the resume point), `resource`, optional `correlationId`. A cursor that cannot be served yields a `ResyncRequired` frame, not a socket close code.
- **Usage (OBS-002, ADR-0033):** `UsageRecord` and the Protobuf `UsageReport` carry each number with a `source` (`reported`, `estimated`, `unknown`); unknown has no value, never zero. Cost is a decimal string plus currency.
- **No merge anywhere:** the delivery-loop policy has no merge switch and the ledger no merge phase (baseline rule 2).
- **`buf breaking` baseline:** a committed Buf image, `baseline/worker-v1.binpb`, built with `buf build -o baseline/worker-v1.binpb` from the proto sources. It is the frozen state of the last released contract and is rebuilt only at a contracts release; until the first release it equals the current sources, so the gate is real (it fails on a breaking edit, shown below) but compares against the draft. This answers the open point in [CONTRACT_VERSIONING](../../docs/architecture/CONTRACT_VERSIONING.md#2-buf-breaking-for-protobuf-packages); the choice is proposed for review.

### Pieces that depend on a proposed ADR

[ADR-0048](../../docs/architecture/adr/0048-approval-reuse-uncovered-capabilities-and-policy-defaults.md) is **proposed, not accepted**; [Q56–Q58](../../docs/architecture/OPEN_QUESTIONS.md) stay open until the product owner decides. The contract carries its shapes so they are reviewable, and they may change:

- `Approval.occurrence` and `ApprovalOccurrence` (Q56);
- reason codes `policy.explicit_deny` and `policy.uncovered`, and `Principal` kind `trigger` requiring a `sponsor` (Q57). The reason `code` is an open string pattern, so accepting or rejecting the ADR does not change the schema;
- no merge Capability and `scm.pr.create` default (Q58) are defaults in configuration, not contract shapes.

## Not in this package yet

| Item | Where |
|---|---|
| Fixtures and the Go + TypeScript validation of payloads against these schemas | API-002 ([#117](https://github.com/rluders/amaru/issues/117)) |
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
```

Requirements: Go (the version in `go.mod`) and the `buf` CLI (verified with 1.69.0 on 2026-10-26; configuration key names checked against the [buf documentation](https://buf.build/docs/breaking/quickstart), read 2026-10-26).

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

JSON Schema validators for the fixture suite are added by API-002, with their own reasons.

## Generated files

None. Everything here is hand-maintained; `baseline/worker-v1.binpb` is a release artifact built by the command above, not hand-edited.
