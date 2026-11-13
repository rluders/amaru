# Project SDK rejected-attachment authority

Status: integration validation slice for #191 (PRJ-002), parent #18; neither issue nor M3 is complete.

## Scope

The generated SDK now tests rejected attachment through the existing Go loopback fixture. Invalid bearer credentials, absent authentication, missing browser Origin and foreign Origin cannot register either of two real Git Projects. A valid session with incorrect CSRF cannot reselect an attached Project to a replacement clone. Logout prevents attachment while the independent native client remains usable. Before/after native list queries assert unchanged registration and active-clone roots; settings remain unchanged after the rejected reselection. Subsequent authorized attachment and reselection still succeed.

Only tests and evidence change. Node supplies Cookie/Origin; fake Platform storage and real temporary Git repositories are used. This does not validate a native browser, PostgreSQL, product startup/restart, real Provider, Work HTTP transport, or remaining definition/secret acceptance. Existing adapters, contracts and authority decisions are unchanged.

## Delivery refresh

GitHub read on 2026-11-13: PR #366 merged at `f87733abb81fe69c97553233bd04b8c95b10a35c`, the current #18 base `feat/18-wp-m3-06-public-api-events`. #191 blocker #120 and inherited #15/#83/#84 are closed. ADR-0062/0067 already authorize this mapping. No human architecture decision is pending for these regression cases.

Open native children refreshed:

| Parent | Open children | Dependency assessment |
|---|---|---|
| #16 | #188, #197 | #188 blocker #135 closed, but stream code must reach its base; #197 awaits #124 |
| #17 | #80, #81, #82, #112, #113, #127, #128, #131, #163, #189, #190 | Migration owner decisions #80–#82 remain open; dependent Provider/confinement work remains blocked |
| #18 | #92, #170, #191, #219 | Native blockers #83/#84/#120/#135 closed; remaining acceptance is not implied |
| #19 | #124, #125, #154 | #124 blockers #119/#123 closed; #154 blockers #85/#120 closed; #125 awaits #197 and #124 |

#16–#19 inherited #15 is closed. #20 still has all four open Work Packages as native blockers. Designated bases remain `feat/16-wp-m3-04-policy-grants-and`, `feat/17-wp-m3-05-providers-and-context`, `feat/18-wp-m3-06-public-api-events`, and `feat/19-wp-m3-07-worktrees-configuration-scopes`. Remote #16 is `d2c393a`; only #16/#18 designated branches currently exist remotely. Do not substitute the stale `feat/18-execution-lifecycle` branch. No existing branch/worktree or dirty shared checkout was changed.

## Local validation

Linux x86_64, Node 25.7.0, pnpm 11.5.0; Platform module selects Go 1.27. Local results are not hosted CI.

- `node --test packages/sdk/test/auth.test.ts`: PASS, exit 0, 5 test/subtest passes, zero failures/skips.
- `pnpm -C packages/sdk run typecheck`: PASS, exit 0.
- `make check`: PASS, exit 0. 346 matching Go/TypeScript fixture outcomes; 348 contract TypeScript passes; 46 SDK passes, zero failures/skips. Default Platform tests and host/cross-vet plus PostgreSQL-tag compile/vet passed. Log: `/tmp/amaru-191-sdk-authority-check.log`.
- `git diff --check`: PASS, exit 0.
- `trail check --base origin/feat/18-wp-m3-06-public-api-events`: exit 1, one lexical conflict on active-clone wording in the operation inventory. The new text describes test readback; it does not add a second ActiveClone lookup after Workflow IO or alter captured definition provenance. Its informational fingerprint warning in the SDK test also introduces no fingerprint change. No learning or check was disabled.

Initial sandbox dependency installation/cache access prevented the test from starting; cache-enabled frozen-lockfile installation resolved that environment limitation. The first authored foreign-Origin transport also affected the SDK health preflight, so it did not reach POST; restricting the foreign Origin to mutations made the case exercise the intended adapter. Final focused and full checks passed after that correction. No production changes required a PostgreSQL/crash suite rerun. Native macOS/Windows, real browser and all product gates listed above remain not validated by this slice.
