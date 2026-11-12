# SDK authentication fixture

Test-only Go command, built with `-tags authfixture` by `packages/sdk/test/auth.test.ts` (part of `make sdk-ts` and `make check`). It binds Linux loopback, initializes the real auth/private-discovery adapters, serves contract-compatible health and stops on SIGTERM. The test owns its temporary private state directory and reads credentials there; the command prints no credentials or request URLs. It has no product routes or domain mutation. Owner: Platform API/auth maintainers. It is not a deployable Platform server.
