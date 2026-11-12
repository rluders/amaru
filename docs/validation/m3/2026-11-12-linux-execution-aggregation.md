# Linux Execution aggregation validation — #335

Date: 2026-11-12. Runner: Codex on the owner's Linux workstation. Delivery base: `7686cf2587840620404ee9cf58373fbe47251ea2` on `feat/18-wp-m3-06-public-api-events`. Implementation commit: `5b1788bacd7c3c109d3e60df11391c8cb2681b50`. The following documentation commit records evidence without changing runtime or test sources.

Environment: Linux `7.2.11-100.fc43.x86_64`, x86_64; host Go 1.26.0 with module-selected Go 1.27.0, Node 25.7.0, pnpm 11.5.0, PostgreSQL 17.5. The PostgreSQL binary came from the existing cached test bundle. A new disposable data directory under `/tmp/amaru-335-pgdata` listened on loopback port 55435 with test-only trust authentication; no existing service or credentials were used. The test harness created and dropped separate databases per test.

## Reproduction

Before the runtime change, adding a permanent parent-failure assertion to the assignment suite and running `go test -count=1 -run TestAssignment ./internal/coordinator` failed on both in-process and serialized links: `parent remains running after final Step`. The assertion remains in the passing suite.

## Checks

All results are local Linux evidence, not hosted CI or real-Provider validation.

| Command | Result |
|---|---|
| `make check` | Exit 0; formatting, documentation, contracts, SDK and Platform checks passed. Includes best-effort Windows/macOS vet, which is not native OS validation. |
| `AMARU_TEST_PG='postgres://postgres@127.0.0.1:55435/postgres?sslmode=disable' GOFLAGS=-json make platform-test-pg` | Exit 0; 1,395 passing tests/subtests across 19 packages, zero failed or skipped tests; four packages have no test files. Includes 31 process-kill subtests and 25 terminal cases per Worker link against PostgreSQL. |
| `go test -race -count=1 -timeout 120s -run 'TestTerminal\|TestAssignment\|TestCancel\|TestProjectAccess\|TestProjectsProgressIndependently' ./internal/coordinator ./internal/worker/embedded` (from `services/platform`) | Exit 0; focused concurrency and independent-Project checks passed. |

## Evidence level and scope

**Unit-tested:** domain transition pairs and timestamps; aggregation precedence, safe deterministic errors, empty/all-skipped runs, refusal, pending/queued drain, human resume/abandon, idempotency and duplicate/stale results. Both Worker links run the shared suite with fake Workers. The JSON serialization link is not a remote network transport.

**Integration-tested:** the same terminal suite over both Worker links against isolated PostgreSQL; read-only Step version guards reject stale decisions atomically; the crash suite covers every protocol commit including aggregation; the Linux process-kill matrix covers both sides of the parent commit and result acknowledgement. Real embedded Worker plus fake Runner independent-Project tests remain green.

Specific regressions cover failure with held work and a racing successful result, Approval withdrawal without a decision, pre-start rejection and sibling withdrawal, protected timeouts/interruption, resume versus sibling failure, healthy resume versus abandon, Approval decision versus sibling failure, lease versus sibling failure, explicit cancellation precedence, simultaneous final results, and the final-Step/parent crash window. Parent terminal state, timestamp and exactly one terminal Event survive repeat recovery and duplicate delivery. Acknowledgement waits for the parent write.

**Not validated:** real agents/Providers, remote Worker transport, remote process termination, composed product HTTP lifecycle routes, native macOS/Windows. Failure signals use the existing best-effort session cancellation port; durable terminal state is not evidence that an external process was killed. #191, #18 and M3 exit remain open for their other acceptance gates.
