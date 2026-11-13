# Linux PRJ-002 Project SDK integration

Date: 2026-11-13. Contributor: Ricardo Lüders through Codex. Branch: `feat/191-sdk-project-acceptance`; exact base: `4ebcef1194065b2a2043489e13aa98fe97474c75` (merged PR #365) on `feat/18-wp-m3-06-public-api-events`. This record accompanies the bounded #191 slice; it does not close #191/#18 or establish M3 exit. #20 remains blocked by #16–#19.

Environment: native Linux x86_64 execution in a container (`systemd-detect-virt`: `container-other`), Fedora 43 userland, kernel `7.2.11-100.fc43.x86_64`; no physical-host claim. Go launcher 1.26.0, Platform module toolchain 1.27.0, Node 25.7.0, pnpm 11.5.0, Git 2.56.0. No real agent, paid API or PostgreSQL server is used. Local results are not hosted CI.

## Scope and evidence

**Integration-tested:** the generated TypeScript SDK sends requests to real guarded Go auth, Project and settings HTTP adapters over Linux loopback. An empty fake Project/clone/settings store replaces the former pre-seeded settings fixture. Real temporary Git Projects are attached through the public mapping: one remote-derived identity and one committed local identity, with matching names. The same Auth instance and authoritative fake store serve both clients.

Native bearer and simulated browser cookie/Origin/CSRF clients cover empty list, attach from a subdirectory, canonical identity/root, omitted optional metadata, deterministic list/detail readback and encoded selectors. Existing settings tests still cover false/default/repeated/concurrent isolated writes, missing Projects, anonymous/missing-CSRF/wrong-CSRF denial and unchanged settings. Additional cases cover ignored dirty config, clone replacement and retry preserving stored metadata/settings, foreign-root conflicts and all-or-error lists, unavailable clone with no fallback, explicit reattach recovery, and rejected Project/settings operations after logout while native access still works. Discovery credentials remain private, are read only by the test and are never printed.

No production runtime, contract, storage schema, dependency or architecture decision changes. The fixture remains behind `authfixture`; it is not product startup.

## Commands and outcomes

- `pnpm -C packages/sdk run typecheck`: PASS, exit 0 after adding the explicit generated Project type.
- `node --test packages/sdk/test/auth.test.ts`: PASS, exit 0; 4 test/subtest passes, zero failures/skips.
- `go -C services/platform vet -tags authfixture ./internal/adapter/localauth/testserver`: PASS, exit 0; explicitly includes the changed build-tagged fixture.
- Final `make check`: PASS, exit 0; 346 identical Go/TypeScript fixture outcomes, 348 contract TypeScript test passes and 45 SDK test passes with zero failures/skips; default Platform suites and host/cross-vet plus PostgreSQL-tag vet/compile passed. PostgreSQL runtime tests are not executed by this command. Log: `/tmp/amaru-191-project-sdk-check-final.log`.
- `make docs-check` and `git diff --check`: PASS, exit 0.
- `trail check --base origin/feat/18-wp-m3-06-public-api-events`: exit 1, five lexical documentation conflicts, reviewed below; no learning or check disabled.

Trail's five matches concern existing Workflow post-I/O authority rechecks, single captured clone binding, trusted transport guard before cancellation, native filesystem validation outside storage and the unreleased-product no-upgrade rule. This change adds SDK evidence and test-only handler composition; it alters none of those implementations or rules. It claims neither SQL panic recovery nor absence of unrelated scheduling races.

Initial sandboxed install failed on the read-only pnpm cache, leaving the SDK dependency unavailable. The cache-enabled frozen-lockfile install succeeded with all 37 packages reused and no new dependency/version. Initial authoring checks caught an implicit array type and an incorrect expected remote identity scheme; both were corrected to match the public generated type and existing canonical identity. The first full check stopped at documentation links to this not-yet-written record and a miscounted fixture README relative link; the links were corrected before the final run.

## Limits and remaining gates

**Not validated:** native browser behavior (Node injects Cookie/Origin), native macOS/Windows, Node 22, PostgreSQL SDK composition, product startup or server/process restart, real Providers, secret delivery, Work/remaining definitions, Execution/Event public HTTP transport or complete #191/#18 acceptance. Earlier PostgreSQL HTTP evidence used Go clients and is separate from this fake-backed SDK fixture. Tests do not establish immunity to concurrent filesystem replacement. Existing #170 product wiring and #219 semantics are unchanged. No issue is closed or merged by this slice.
