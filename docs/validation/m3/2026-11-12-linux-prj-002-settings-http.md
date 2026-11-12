# Linux Project settings HTTP integration — 2026-11-12

Scope: PRJ-002 [#191](https://github.com/rluders/amaru/issues/191), parent [#18](https://github.com/rluders/amaru/issues/18). Base: `76cf2f75599ad00dd0f4c5c3d080c926a3408d6c` (merged PR #341). The reviewed PR head identifies this implementation and evidence together. This record does not complete either issue or M3.

## Environment

Linux `7.2.11-100.fc43.x86_64`, x86_64 GNU/Linux; host Go `1.26.0` with the module's Go 1.27 toolchain, Node `v25.7.0`, pnpm `11.5.0`. Physical/full-VM classification is unverified. Local results only, not hosted CI.

## Behavior and evidence level

**Integration-tested with fake storage:** `httpapi.SettingsHandler` composes real compact authentication, ProjectAccess and the fake Project/settings adapter for the existing GET/PATCH resource. Two registered Projects have matching display names; one uses a remote-derived key with encoded slashes and `#`, the other a local `id:` key. A browser session patches true, repeated true, false and repeated false; a native client reads committed responses and confirms the other Project stays false. Logout prevents further dispatch.

Rejected authentication, Host, Origin, query credentials and invalid/foreign Project selectors cannot read the body or settings adapter. Browser PATCH requires Origin and CSRF. Revocation during body IO is rechecked before writes. Strict parsing rejects empty/null/non-boolean/unknown-field/trailing input; the handler rejects non-JSON media, oversized bodies and body IO errors without writes. Read/write store failures return fixed internal-error envelopes; a refused write leaves the fake value unchanged. The six shared UpdateSettings fixtures are submitted to the handler with their declared outcomes. A scoped OpenAPI check compares exactly the two owned settings methods/paths; it is not a full API route-parity claim.

**Integration-tested over actual loopback:** two native HTTP clients PATCH through a temporary listener with real authentication. Cookie/CSRF tests dispatch real requests directly through the handler; they are not a native browser run. All storage here is fake. Existing PostgreSQL persistence evidence remains in the [settings store record](2026-11-12-linux-prj-002-settings.md).

## Commands and results

- Merged base: `make check` — PASS, exit 0, on #18 integration at `76cf2f7`; log `/tmp/amaru-341-merged-check.log`. Defect #339 was closed after this verification.
- `go test -race -count=10 ./internal/adapter/httpapi` from `services/platform` — PASS, exit 0. Repeated HTTP, loopback, body failure, authorization and shared-fixture cases; no race reported.
- Candidate `make check` — PASS, exit 0, including Platform formatting/vet/tests/architecture checks, docs, contracts and SDK validation; shared parity reports 346 identical Go/TypeScript fixture outcomes. Log `/tmp/amaru-191-settings-http-check.log`. Cross-compilation/vet is not native macOS/Windows validation.

Initial test setup used an incorrect relative path to the OpenAPI fixture, causing the scoped route test to fail; correcting that path made it pass. No runtime test was suppressed.

## Architecture review and limits

Verdict: **sound** within accepted ADR-0059/0062/0064. Fresh code maps existing contracts onto Platform-owned state; the domain stays pure and Clients/Workers acquire no authoritative settings. No copied predecessor material, new dependency, infrastructure, wire shape or authority grant. A preflight authorized read deliberately precedes PATCH body parsing, and the update rechecks identity after IO; the store applies the patch atomically without relying on that read's value. Unexpected failures produce no success response or raw diagnostics. Simpler permissive decoding without the Project application boundary would lose strict validation and live authorization; direct unguarded handlers would lose compact Host/Origin/CSRF protection.

Trusted composition must use the same Auth identity callback and authoritative Instance stores. Product startup, actual SDK/native browser calls, the remaining public routes, full routeguard parity, PostgreSQL HTTP/restart composition, actual attached-root Skill discovery, real Providers and native macOS/Windows are **not validated** by this slice. Linux is required; other OSes remain best effort for M3. The wider #191 Work/dependency, other-definition, exact-scope secret and clone-location gates remain open.
