---
title: Adapt the prototype remote-job HTTP helpers into the Worker providers, and write remote-job resume in Amaru
status: accepted
date: 2026-10-28
deciders: Ricardo Lüders (product owner)
---

# 0054. Adapt the prototype remote-job HTTP helpers into the Worker providers, and write remote-job resume in Amaru

## Context
- [ADR-0007](0007-reuse-of-earlier-internal-code-is-a-separate-activity.md) bars moving code from the earlier internal implementation until a human approves a migration ADR for that component. This is the draft migration ADR for the `provider/remotejob` package of its Runtime (`runtime/internal/provider/remotejob`; `MADR-provider-remotejob`, issue #79, sub-issue of WP-M3-03 / #15). Every issue that moves this component is gated on it ([delivery §8](../../engineering/delivery.md#8-human-approval-gates)); in M3 that is FEAT-CAP-004 (#130, remote-job providers).
- **Source (verified 2026-10-28):** the Runtime repository of the earlier internal implementation (named in the private migration notes, not part of this repository), baseline commit **`c8a2f6e`** ([ADR-0032](0032-migration-baseline-is-a-pinned-commit.md)). From a `git archive` of that commit the package is three files, 373 lines: `remotejob.go` (202), `files.go` (111) and one test file (60, two test functions). It imports the standard library and `domain` (one function, `CleanRelPath`). `go test -count=1 ./internal/provider/remotejob/...` passes on the extracted tree (2026-10-28, Linux).
- **Planned disposition** (migration plan, `MADR-provider-remotejob`): **Adapt** into the Worker providers.
- **What the package is, and is not.** Its own header says "a helper, not a framework: adapters own their flow". It holds: a JSON client with typed non-2xx errors (`HTTPError`, body truncated to 500 bytes, `Retry-After`); `Retry` with the server's `Retry-After` or exponential backoff; `Sleep`; bounded `Download` of a signed result URL (https only, no credentials, size limit, never truncated); `ReadProjectFile` (project-confined, symlinks resolved, extension and size allow-list); and multipart `Upload`. It does **not** hold the remote-job lifecycle. Resumable and detachable jobs, recording the job id, and re-attaching after a restart live elsewhere in the prototype (the `Resumable` and `Detachable` provider interfaces, the `External` event and the manager's detach and resume paths). Six vendor adapters (`meshy`, `fal`, `tripo`, `elevenlabs`, `replicate`, `localtools`) import it; none of them is moved by this ADR.
- **Consequence for FEAT-CAP-004.** The acceptance criterion, "a remote job survives a Platform restart and is reattached", is mostly coordinator and port work. The domain already records a remote job on the Step (`RemoteJob{ID, Resumable}`) and the restart rule applies it ([ADR-0042](0042-durable-execution-with-explicit-restart-semantics.md) rule 2, [ADR-0044](0044-restart-semantics-clarifications-from-the-m1-spike.md)). What is missing is the Provider port that reports a job id, declares `Resumable` and `Detachable`, and re-attaches, and the coordinator behavior that uses it. That is written in Amaru against the accepted rules; this package contributes only its transport helpers.
- **Things in the source that need a decision or a fix before they move.**
  - **Redirects and SSRF.** `Download` accepts only an https URL but uses a client that follows redirects, with no check of the destination address. A signed URL from a remote service could redirect to an internal address. The source does not address it.
  - **Project file reads.** `ReadProjectFile` resolves symlinks and checks the prefix, then opens the file: a path swap between the check and the read is possible (time-of-check to time-of-use). Reading Project files belongs behind the same confinement the Worker applies to any file access, not in an HTTP helper.
  - **`domain.CleanRelPath`.** The function belongs to the Artifact path cleaning that ADR-0049 adapts into the domain; the helper must use Amaru's equivalent, not the prototype's.
  - **Defaults.** The client timeout (60 s API, 10 min download) and the 16 MiB response cap are constants of the prototype; they must be configurable and sit under the Step's timeout ([#212](https://github.com/rluders/amaru/issues/212)).
  - **Retry.** `Retry` retries on 429 and 5xx only. FEAT-WFL-005 (retries with backoff, M5) owns Step-level retry; a helper that retries an HTTP call inside one attempt is not a Step retry and must not repeat a non-idempotent remote call that was accepted (a submit that returned 5xx after the remote created the job).
- **Placement constraint.** The core imports no network package (`internal/arch`). Providers live outside the core and depend only on `ports` and `domain`; the HTTP helper is in the provider layer. The vendor adapters are WP-M3-05 or plugins ([FEAT-CAP-002/003](../../product/FEATURE_CATALOG.md#34-agents-skills-capabilities)).
- **Unverified:** that the 2 prototype tests cover the retry and download edge cases (they do not cover redirects, a body over the limit on the error path, or `Upload`); how a real remote service behaves; macOS and Windows.

## Decision
1. **Component and source.** Migrate `remotejob.go` and the transport parts of `files.go` of `internal/provider/remotejob`, with their tests, at commit `c8a2f6e` (ADR-0032). No later commit is used.
2. **Disposition: Adapt.** The prototype is a behavioral reference and migration source. `services/platform` is canonical from the first commit; the prototype is not kept in sync. The six vendor adapters are **not** moved by this ADR.
3. **History option: fresh import with provenance** (migration plan default), as in ADR-0049: the source commit is recorded here, in `services/platform/README.md` and in `PROVENANCE.md` with each file's source path and blob id. No `git subtree` or `filter-repo`.
4. **File disposition.**

   | Source (`internal/provider/remotejob/`) | Disposition | Target / note |
   |---|---|---|
   | `remotejob.go`: `Client`, `Do`, `DoRaw`, `HTTPError`, `RetryAfter`, `Sleep` | Adapt | JSON transport with typed errors for HTTP Providers. Timeouts and response caps become configuration; error bodies stay truncated and are never logged with request data |
   | `remotejob.go`: `Retry` | Adapt | Kept for transient transport errors inside one attempt, with the rule that a call that may have been accepted by the remote is not repeated unless the Provider marks it idempotent (an idempotency key from the Step) |
   | `remotejob.go`: `Download` | Adapt, **with fixes** | https only, no credentials, bounded, never truncated (kept). Added: redirects limited and re-checked (https only, no private, loopback or link-local destination), and the check tested |
   | `files.go`: `Upload` | Adapt | Multipart upload of bytes the caller already holds |
   | `files.go`: `ReadProjectFile` | **Not moved** | Reading Project files is a Worker confinement concern (POL-006, FEAT-ENV-*), not an HTTP helper. The Provider receives the bytes or an Artifact reference. The extension and size checks are re-derived there |
   | `remotejob_test.go` (2 tests) | Adapt | See item 6 |
   | Vendor adapters `meshy`, `fal`, `tripo`, `elevenlabs`, `replicate`, `localtools` | **Out of this ADR** | WP-M3-05 or plugins; each needs its own decision (they name vendors) |
   | Remote-job lifecycle (`Resumable`, `Detachable`, `External` event, detach and resume) | **Not a file of this package** | Written in Amaru for FEAT-CAP-004 (#130) on ADR-0042/0044 and the existing `RemoteJob` on the Step; ADR-0052 item 5 defines the Worker port with its first implementation |
   | Source ADRs, dogfood records | Reference only | cited, not copied |

5. **Remote-job resume is in scope of #130, not of this package.** #130 defines the Provider port (job id report, `Resumable`, `Detachable`, re-attach), the coordinator use of it after a restart, and a fake remote Provider for the tests. A Provider that wants an HTTP transport uses the helpers adapted here, but is not required to.
6. **Tests that move.** The prototype tests `TestDoAndRetry` and `TestDownload` are the specification of the transport behavior. They run with a local `httptest` server and fakes in `make check` on Linux, macOS and Windows best-effort. **New**, listed in `PROVENANCE.md`: redirect to a non-https or private address is refused; a body over the limit on the error path is bounded; `Upload` round trip; a non-idempotent call that returned 5xx is not repeated. For #130: a fake remote Provider whose job survives a coordinator restart and is re-attached, and one that is not `Resumable` and ends `interrupted` (ADR-0042 rule 2). None is edited merely to pass.
7. **Renames** follow the [glossary term map](../../product/GLOSSARY.md#2-term-mapping-from-predecessor-repositories); `weave`, `orkestra` identifiers are not carried. No such string remains in `services/platform` except in `PROVENANCE.md`.
8. **Gate.** No file from the source is added to this repository until this ADR is `accepted` with a recorded approval. Only then does #130 start moving code. Merging this draft is not acceptance; issue #79 stays open until the decision.

## Readiness criteria (migration plan §5)
| # | Criterion | State | Evidence |
|---|---|---|---|
| 1 | Source side stable at `c8a2f6e` | **Met** | ADR-0032; commit verified; no later commit used |
| 2 | Inventory refreshed with tests run | **Met** | WP-M1-07 (#8) closed; package tests re-run at `c8a2f6e` on 2026-10-28 (see Context) |
| 3 | Target ADRs accepted | **Met** | ADR-0013, 0042, 0044, 0049, 0052 accepted. The vendor adapters are excluded so that no vendor decision is pre-empted |
| 4 | Contracts stable, with fixtures, in Amaru | **Met** | M2 exit validation (#12) closed; the Step already carries `RemoteJob`. A Provider-facing contract for remote jobs would be added with #130 and WP-M3-05 (`amaru.provider/v1`, FEAT-CAP-003), not here |
| 5 | Behavior covered by tests that run in Amaru | **Met by plan** | Item 6; becomes evidence in #130 |
| 6 | Verified execution | **Gap, listed** | The helpers were exercised by two unit tests and by the prototype's paid vendor runs, which are not reproducible evidence here. No real remote service is called in `make check`. Accepting this ADR accepts that gap |
| 7 | License resolved | **Met** | The source is MIT, © Ricardo Lüders, and the holder consents to relicensing ([ADR-0026](0026-fair-code-community-license-with-commercial-licensing.md)). No license file is added by this work |
| 8 | Explicit product-owner approval | **Met** | Accepted by the product owner on 2026-10-28 (see Approval) |

## Consequences
- **Easier:** HTTP Providers start from a small transport with typed errors and `Retry-After` handling, and the download path is hardened before the first vendor uses it.
- **Harder:** the catalog describes CAP-004 as a reuse of this package, but the acceptance criterion is the restart-and-reattach behavior, which is new coordinator and port work. #130 is larger than "move the package".
- **Security and compatibility:** results are fetched without credentials and from https only; redirect and destination checks are added. Credentials for a remote service are a Provider concern and never appear in logs or error bodies. No secrets or personal data are transferred (scanned in each PR). Amaru promises no compatibility with the prototype (no importer, owner decision of 2026-10-27).
- **Maintenance:** the helpers have a single owner (the Platform owner); the prototype copy is frozen.
- **Follow-up:** after acceptance, update the migration inventory in the private notes and mark these files migrated (separate approval in the source repository). Each vendor adapter needs its own decision before it moves. Revisit if the external provider protocol (FEAT-CAP-003) makes the in-process HTTP helper unnecessary.

## Alternatives considered
- **Move the package unchanged:** simplest, but ships a download path that follows redirects to any address, and a file reader with a check-then-read gap.
- **Reimplement from the documents only:** loses the small amount of tested behavior (`Retry-After`, bounded download); the package is 373 lines and can be reviewed whole.
- **Use an HTTP client library with built-in retry and backoff:** a new dependency for a feature of about 60 lines; not evaluated against the idempotency rule above, which a generic retry would break.
- **Move the vendor adapters together with the helper:** pulls six vendor decisions and paid-service evidence into M3 and contradicts "Workflows reference Capabilities, never vendors".

## Approval
Proposed by: Claude (migration ADR draft for #79, 2026-10-28). Approved by: Ricardo Lüders (product owner) on 2026-10-28, in the WP-M3-03 session, after reviewing PR #286: "Self-review, approve, merge. Do any rationale to take de decisions." The Not-moved and Out-of-this-ADR rows of item 4 stay unmoved as drafted; the criterion 6 gap is accepted with the ADR. Windows and macOS remain best-effort (owner decision of 2026-10-28), so cross-OS runs of the new tests are not required.
