# M2 Contracts v0: exit validation

**Date:** 2026-10-27 · **Task:** [#12](https://github.com/rluders/amaru/issues/12), EXIT-M2 · **Source:** [ROADMAP M2](../ROADMAP.md#m2-contracts-v0) · **Tree:** `main` at `6279d07` (contains #9, #10 and #11 merged)

## Result
**Two of four exit criteria are met; one is met only in part; one is not met as written.** Nothing here is CI evidence and nothing ran against a Platform, because none exists. The two open points were left to my judgment by the product owner on 2026-10-27 ("Your choice"), with the rule that `/v1` stays the route prefix until 1.0.0; the resolution is under "Decision".

| # | Criterion | Result | Evidence level |
|---|---|---|---|
| 1 | Every prototype route needed for M3/M4 is in the contract | **Partly met; needs an owner decision** (see "Criterion 1") | Compared by script, not tested |
| 2 | A drift test compares server routes with the spec | **Not met as written**: the guard exists and is tested, but no server exists to compare | Unit-tested (fake route list); not integration-tested |
| 3 | The SDK generation staleness check is in `make check` | **Met** | Unit-tested: `make sdk-ts` is a `check` prerequisite; a tampered generated file made `--check` exit 1 (2026-10-26, PR #257) |
| 4 | Fixture tests pass in Go and TypeScript | **Met** | Unit-tested: 315 fixtures, identical per-fixture outcomes in Go and TypeScript |

## Environment and commands
Fedora Linux 43, kernel 7.2.11, Go 1.26.0, Node 25.7.0, pnpm 11.5.0, buf 1.69.0. Linux only; Node 22 (the `engines` floor) and other OSes were not run.

`make check`, run on `6279d07`: **exit 0**. Relevant lines of its output:
- `ok …/packages/contracts/routeguard`
- `ℹ pass 317 / ℹ fail 0` (contracts TypeScript suite)
- `contracts parity: 315 fixtures, identical outcomes in Go and TypeScript`
- `generated SDK types are up to date`
- `ℹ pass 16 / ℹ fail 0` (SDK)
- `doccheck: ok`

## Criterion 1: prototype routes in the contract
Method: the prototype's `protocol/openapi.yaml` at the pinned baseline `c8a2f6e` (ADR-0032) has 79 operations. Each was compared (method plus path with parameter names ignored, `/v1` prefix added, `/autoloop` read as `/delivery-loop`) with the operations in `packages/contracts/openapi/openapi.yaml`.

- **41 of 79** are in the contract with the same method and path.
- **38 are not.** Each belongs to a group that [PROVENANCE](../../../packages/contracts/PROVENANCE.md#not-carried-over-dropped-with-reason) records as deliberately dropped:

| Dropped group | Operations | Recorded reason | Planned feature (catalog) |
|---|---|---|---|
| MCP servers, agent MCP grants, connector validation, agent grants | 14 | Prototype configuration model; tool access goes through Capabilities and Policy (ADR-0037) | none scheduled in M3/M4 |
| Outbound hooks and hook egress | 10 | Webhook Triggers go through Policy | WFL-009 (M7) |
| Skills, Playbooks, their trust, Workflow templates | 10 | "Later features" | **SKL-001, SKL-002 are P0 in M3** |
| Provider enable and secret routes (`PATCH /providers/{id}`, `PUT …/secrets/{name}`) | 2 | Separate secret-handling design | **POL-007 (secrets port) is P0 in M3**; WP-M4-02 lists "settings (providers…)" |
| Intake sync | 1 | Tracker integration not in M2 | INT-001 (M4), INT-003 |
| `POST …/steps/{stepId}/approval` | 1 | Replaced by the `Approval` resource and decision route (ADR-0036) | REV-001/REV-003 |

- **3 operations are new** (not in the prototype): `GET …/approvals/{id}`, `POST …/approvals/{id}/decision`, and `GET …/work-packages/{id}/delivery-loop` (the prototype registered the last one but never declared it).

**Why only "partly met":** the criterion is "needed for M3/M4", and the drop list was decided while writing the contract. Two groups are tied to P0 M3 features in the catalog: Skill listing and trust (SKL-001 "bad or untrusted Skill reference refuses the run before start; pinned hashes enforced", SKL-002 context assembly) and provider secrets (POL-007). Whether those need HTTP routes in `/v1` before M3 starts is a scope judgment I did not make. The MCP group may also matter for AGT-003 ("MCP servers only to strict-isolation providers"), which ADR-0037 may already cover through Capabilities; I did not verify that.

## Criterion 2: drift guard
`packages/contracts/routeguard` ([WP-M2-03](../WORK_PACKAGES.md#wp-m2-03-contract-drift-guard), #11) fails in both directions: `TestRegisteredButUndeclaredRouteFails` (the prototype's undeclared `…/autoloop` shape) and `TestDeclaredButUnregisteredRouteFails`. It reads the contract and compares two lists; it has never read a router's real registrations. The ROADMAP wording ("compares server routes with the spec") needs the Platform, so the integration part is [WP-M3-06](../WORK_PACKAGES.md) ("Drift guard").

## Not validated
- Any run against a Platform, a real WebSocket server or a browser.
- The OpenAPI file with an OpenAPI validator (only structural Go checks, `openapi-typescript` parsing it, and fixtures).
- Node 22, macOS, Windows.
- Hosted CI: none runs `make check`.
- `buf breaking` against a released baseline: the baseline is the first commit of the Worker protocol, so no change has been compared with an earlier release.

## Decision (2026-10-27, delegated by the product owner, not an ADR)
The owner left both points to the agent and stated that `/v1` is the route prefix until 1.0.0. The package is `0.x` and may change incompatibly until the Local MVP release ([ROADMAP M2 risks](../ROADMAP.md#m2-contracts-v0)), so adding a route later costs a fixture and a schema, not a version.

1. **Criterion 1: the drop list stands for the M2 exit; no routes are added now.** Reason: the dropped Skill-trust and provider-secret routes are not copies to restore. Provider secrets were dropped because their handling needs a security design that does not exist yet, and Skill trust must follow ADR-0037 and the Skill model that M3 adapts. Adding prototype-shaped routes now would freeze the wrong shapes. They are added to `/v1` by the Work Package that implements them: Skills and context in [WP-M3-05](../WORK_PACKAGES.md#wp-m3-05-providers-and-context-assembly), secrets in POL-007's Work Package. Each must add its OpenAPI operations and fixtures in that change; the exit of M3 should check this. MCP routes: not added; whether AGT-003 needs any is for WP-M3-05 to show against ADR-0037.
2. **Criterion 2: M2 closes with the guard unit-tested.** The integration evidence (the guard run against the Platform's real route registrations) is an acceptance item of WP-M3-06 and part of the M3 exit, not of M2.

Neither choice changes a contract file. If the owner disagrees with either, the record should be amended before #12 is closed.
