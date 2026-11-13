# SDK authentication and Project fixture

Test-only Go command, built with `-tags authfixture` by `packages/sdk/test/auth.test.ts` (part of `make sdk-ts` and `make check`). It binds Linux loopback, initializes real auth/private-discovery adapters and serves contract-compatible health, Project attach/list/detail and Project settings through their existing guarded HTTP adapters. One fake Project/clone/settings store and one Auth instance are shared by these adapters. The SDK test creates and attaches temporary real Git repositories; the fixture starts with no registered Projects.

The test owns its temporary private state directory and reads credentials there; the command prints no credentials or request URLs and stops on SIGTERM. It uses no database or agent. Node supplies Cookie/Origin for the browser transport; this is not a native browser run. Owner: Platform API/auth maintainers. It is not a deployable Platform server or evidence of product startup. See the [Project SDK validation record](../../../../../../docs/validation/m3/2026-11-13-linux-prj-002-project-sdk.md).

Rejected SDK attachments also cover credentials, browser Origin, CSRF, clone-selection preservation and logout; see the [authority record](../../../../../../docs/validation/m3/2026-11-13-linux-prj-002-sdk-authority.md).
