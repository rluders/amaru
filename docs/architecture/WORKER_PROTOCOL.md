# Worker Protocol `amaru.worker.v1` (Draft Schemas)

Status: **Proposed** (2026-10-26), not implemented. Design deliverable of [WP-M1-06](WORK_PACKAGES.md#wp-m1-06-worker-protocol-schemas) ([#7](https://github.com/rluders/amaru/issues/7)). Implementation is M5; the contract package that will hold the final `.proto` files and fixtures is created at M2 ([repository layout](../engineering/repository-layout.md), [CONTRACT_VERSIONING](CONTRACT_VERSIONING.md)). No `.proto` file or directory is added to the repository now ([AGENTS.md](../../AGENTS.md) rule 5); the draft lives here as reviewable text.

Applies [ADR-0029](adr/0029-worker-protocol-over-websocket-and-protobuf.md) (WebSocket + Protobuf, package `amaru.worker.v1`), [ADR-0024](adr/0024-remote-worker-registration-identity-and-delivery.md) (enrollment, Ed25519 identity, revision, secrets), [ADR-0042](adr/0042-durable-execution-with-explicit-restart-semantics.md) and [ADR-0044](adr/0044-restart-semantics-clarifications-from-the-m1-spike.md) (leases, idempotency, fenced results). Lifecycle: [EXECUTION_ENVIRONMENTS §3](EXECUTION_ENVIRONMENTS.md#3-worker-lifecycle-and-protocol-draft).

## 1. Message map

| Family | Messages | Direction | Source rule |
|---|---|---|---|
| Handshake and enrollment | `Hello`, `Welcome`, `ChallengeResponse`, `Authenticated` | W→P, P→W, W→P, P→W | ADR-0024: one-time token, Ed25519 signed challenge, short-lived session; version negotiation (ADR-0029). |
| Offer | `Offer` | W→P | Pull-based dispatch: the Worker announces free slots. |
| Assignment | `Assignment`, `AcceptAttempt`, `RejectAttempt` | P→W, W→P | Lease starts on accept. Carries `attempt_id`, `Revision`, `idempotency_key`, `capability_idempotent`. |
| Lease | `LeaseRenew`, `LeaseRelease`, `LeaseRevoked` | W→P, W→P, P→W | Heartbeat 10 s, Lease 30 s by default (periods are set in `Authenticated`). Expiry is decided in PostgreSQL, not by the Worker. |
| Events and result | `StepEvents`, `Result`, `ResultAck` | W→P, W→P, P→W | Platform assigns `seq`; Worker ordinals deduplicate only. Duplicate completions are ignored by `attempt_id`. |
| Cancel and drain | `Cancel`, `Drain` | P→W, W→P | `Cancel` asks the Worker to end the attempt's whole process tree within a grace period; the attempt and its Lease end with the Worker's `cancelled` Result (implemented, FEAT-EXE-003). |
| Secrets | `SecretDelivery` | P→W | Scoped to the Step, memory only, wiped at Lease end, redacted from Events. |
| Errors | `ProtocolError` | both | Unsupported version, authentication failure, revocation, malformed frame. |

Large payloads (Artifacts, logs over a threshold) do not use this socket: resumable HTTP uploads or presigned S3 URLs ([ADR-0017](adr/0017-content-addressed-artifact-store-filesystem-or-s3.md)). `OutputRef.upload_ref` points to them.

## 2. Reliability and idempotency fields

- **Application acks.** `Envelope.message_id` is strictly increasing per sender and session; `ack_id` is the highest received. On reconnect the Worker sends `Hello.session_id` and `last_ack_id`; the Platform resends unacknowledged messages (ADR-0029).
- **Attempt identity.** `attempt_id` scopes results and events. A second `Result` for the same `attempt_id` gets `RESULT_DISPOSITION_DUPLICATE` and changes nothing.
- **Fencing.** `Lease.fencing_token` increases on every lease of a Step. `Result`, `StepEvents`, `LeaseRenew` and `SecretDelivery` carry it. A message with a token below the Step's current one is rejected; for a `Result` the Platform answers `RESULT_DISPOSITION_REJECTED_STALE` and records `result.rejected_stale` (ADR-0044 point 4). The Worker must stop work on `LeaseRevoked`.
- **Side-effect keys.** `Assignment.idempotency_key` is stable across attempts of the same Step and is the key the Capability must pass to anything externally visible. `capability_idempotent` reflects the Capability catalog flag the Platform used to decide a retry ([ADR-0042](adr/0042-durable-execution-with-explicit-restart-semantics.md)); the Worker does not decide retries.
- **Attempts.** Which outcomes consume an attempt is Platform policy (ADR-0044 point 5), not a protocol field. `RESULT_OUTCOME_FAILED` is a reported failure; interruption, timeout and lease loss are Platform-observed and have no `Result`.
- **Timeouts.** `Assignment.deadline` is informative for the Worker; the Platform enforces `timed_out` (ADR-0044 point 3).

## 3. In-process transport (embedded Worker)

The embedded Worker uses the same `Envelope` messages over an in-process, ordered, reliable channel. Differences, all deliberate:
- No WebSocket framing, TLS, enrollment or key challenge: the process boundary is the trust boundary; `Hello`/`Welcome` still run so version negotiation and `Manifest` are exercised by the same code.
- The embedded Worker lives and dies with the Platform: after a Platform restart its `running` Steps are recovered at once ([ADR-0044](adr/0044-restart-semantics-clarifications-from-the-m1-spike.md) point 2). Acks and resume (`last_ack_id`) therefore do not span restarts for it.
- Messages are still encoded (or at least pass through the same types) so the in-process path cannot drift from the wire path; conformance fixtures run against both ([CONTRACT_VERSIONING](CONTRACT_VERSIONING.md)).

## 4. Security notes

- Secrets only in `SecretDelivery`, never in `Assignment`, `StepEvents`, `Result` or any error text. `Revision.credential_secret_ref` names a secret delivered separately; it is not the credential.
- `SecretValue.value` is `bytes` so it can be zeroed; implementations must not log protocol frames that can contain it.
- Events carry text already redacted on the Worker (known secret values removed before leaving it) and are redacted again on the Platform ([DATA_ARCHITECTURE §5](DATA_ARCHITECTURE.md#5-retention-and-redaction)).
- `OutputRef.claimed_sha256` is a claim; the Platform hashes content itself.
- Authorization stays on the Platform: a Worker only receives Capabilities its Environment Policy allows, and consequential Capabilities need Grants or Approvals before assignment (EXECUTION_ENVIRONMENTS §3).
- Unknown fields are ignored by receivers; unknown `oneof` bodies produce `ERROR_CODE_MALFORMED` only if the version was negotiated as equal, otherwise they are skipped (additive evolution, [ADR-0025](adr/0025-api-and-protocol-versioning-policy.md)). Exact behavior is a decision for the M2 contract work.

## 5. Schema draft

Source for the Protobuf below was extracted unchanged and checked with `buf` 1.69.0 on 2026-10-26 (`buf lint` with the `STANDARD` rule set, and `buf build`; both exit 0, no findings) from a scratch module with the path `amaru/worker/v1/worker.proto`. No `buf.yaml` is committed.

```protobuf
syntax = "proto3";

package amaru.worker.v1;

import "google/protobuf/duration.proto";
import "google/protobuf/timestamp.proto";

// Envelope wraps every frame (ADR-0029). Transport: WebSocket for remote
// Workers, an in-process channel for the embedded Worker; same messages.
message Envelope {
  // Sender-assigned, unique per session, strictly increasing.
  uint64 message_id = 1;
  // Highest message_id received from the peer (application-level ack).
  uint64 ack_id = 2;
  oneof body {
    Hello hello = 10;
    Welcome welcome = 11;
    ChallengeResponse challenge_response = 12;
    Authenticated authenticated = 13;
    Offer offer = 20;
    Assignment assignment = 21;
    AcceptAttempt accept_attempt = 22;
    RejectAttempt reject_attempt = 23;
    LeaseRenew lease_renew = 30;
    LeaseRelease lease_release = 31;
    LeaseRevoked lease_revoked = 32;
    StepEvents step_events = 40;
    Result result = 41;
    ResultAck result_ack = 42;
    Cancel cancel = 50;
    SecretDelivery secret_delivery = 60;
    Drain drain = 70;
    ProtocolError error = 99;
  }
}

// ---- Handshake and enrollment (ADR-0024) ----

message Hello {
  // Protocol versions the Worker speaks, e.g. "amaru.worker.v1".
  repeated string protocol_versions = 1;
  // Empty on first enrollment.
  string worker_id = 2;
  // One-time token; empty after enrollment.
  string enrollment_token = 3;
  // Ed25519 public key; sent only at enrollment.
  bytes public_key = 4;
  string worker_version = 5;
  Manifest manifest = 6;
  // Session resume: last message_id received from the Platform.
  uint64 last_ack_id = 7;
  // Empty for a new session.
  string session_id = 8;
}

message Manifest {
  string os = 1;
  string architecture = 2;
  repeated ToolVersion tools = 3;
  repeated string capabilities = 4;
  map<string, string> labels = 5;
  uint32 slots = 6;
  bool sandbox_supported = 7;
}

message ToolVersion {
  string name = 1;
  string version = 2;
}

message Welcome {
  string selected_protocol_version = 1;
  // Random nonce the Worker signs with its private key.
  bytes challenge = 2;
  string session_id = 3;
}

message ChallengeResponse {
  // Ed25519 signature over challenge || session_id.
  bytes signature = 1;
}

message Authenticated {
  string worker_id = 1;
  google.protobuf.Duration heartbeat_period = 2;
  google.protobuf.Duration lease_period = 3;
}

// ---- Dispatch (pull-based) ----

message Offer {
  uint32 free_slots = 1;
}

message Assignment {
  string execution_id = 1;
  string step_id = 2;
  // Unique per attempt; the idempotency scope for results and events.
  string attempt_id = 3;
  string capability = 4;
  // Resolved inputs as canonical JSON (UTF-8).
  bytes inputs_json = 5;
  // Repository and commit the Worker must check out (ADR-0024).
  Revision revision = 6;
  google.protobuf.Timestamp deadline = 7;
  Lease lease = 8;
  // Platform-assigned key for side effects of idempotent Capabilities.
  string idempotency_key = 9;
  // Declared by the Capability catalog (ADR-0042).
  bool capability_idempotent = 10;
  // Grants the Worker may rely on; names only, never secret values.
  repeated string grant_ids = 11;
  // Set when the Step already has a remote job recorded (FEAT-CAP-004): the Worker
  // re-attaches to it and must not submit the work again.
  RemoteJob remote_job = 12;
}

// RemoteJob identifies a job that runs outside the Platform and the Worker, for
// example at a remote service. A Worker records it with a StepEvent of type
// "remote_job" (data_json: {"id": "...", "resumable": true|false}) as soon as the
// service accepts the work (ADR-0042 rule 2).
message RemoteJob {
  // The service's job id. Opaque; never a secret or a signed URL.
  string id = 1;
  // The Provider declares that a new Lease can re-attach to this job.
  bool resumable = 2;
}

message Revision {
  string repository = 1;
  string commit_sha = 2;
  // Short-lived, repository-scoped credential, if the Platform issues one.
  // Delivered through SecretDelivery, never inline.
  string credential_secret_ref = 3;
}

message Lease {
  string lease_id = 1;
  // Fencing token: strictly increasing per Step. A result carrying a lower
  // token than the Step's current token is rejected (ADR-0044 point 4).
  uint64 fencing_token = 2;
  google.protobuf.Timestamp expires_at = 3;
}

message AcceptAttempt {
  string attempt_id = 1;
  uint64 fencing_token = 2;
}

message RejectAttempt {
  string attempt_id = 1;
  RejectReason reason = 2;
  string detail = 3;
}

enum RejectReason {
  REJECT_REASON_UNSPECIFIED = 0;
  REJECT_REASON_NO_CAPACITY = 1;
  REJECT_REASON_CAPABILITY_UNAVAILABLE = 2;
  REJECT_REASON_REVISION_UNAVAILABLE = 3;
  REJECT_REASON_DRAINING = 4;
}

// ---- Lease ----

// Heartbeat: renews the Lease. Sent every heartbeat_period.
message LeaseRenew {
  string attempt_id = 1;
  uint64 fencing_token = 2;
}

message LeaseRelease {
  string attempt_id = 1;
  uint64 fencing_token = 2;
}

// Platform to Worker: the Worker no longer holds the Lease.
message LeaseRevoked {
  string attempt_id = 1;
  string reason = 2;
}

// ---- Events and result ----

message StepEvents {
  string attempt_id = 1;
  uint64 fencing_token = 2;
  // Worker-local ordinal, used only to deduplicate; the Platform assigns the
  // authoritative per-Execution seq.
  repeated StepEvent events = 3;
}

message StepEvent {
  uint64 worker_ordinal = 1;
  // "remote_job" records the Step's RemoteJob (see RemoteJob); a Provider may send it
  // again when a multi-phase job gets a new id. Other types are free-form.
  string type = 2;
  // Already redacted on the Worker before leaving it.
  string message = 3;
  bytes data_json = 4;
}

message Result {
  string attempt_id = 1;
  uint64 fencing_token = 2;
  ResultOutcome outcome = 3;
  repeated OutputRef outputs = 4;
  Usage usage = 5;
  string error_code = 6;
  string error_detail = 7;
}

enum ResultOutcome {
  RESULT_OUTCOME_UNSPECIFIED = 0;
  RESULT_OUTCOME_SUCCEEDED = 1;
  RESULT_OUTCOME_FAILED = 2;
  RESULT_OUTCOME_CANCELLED = 3;
}

message OutputRef {
  string role = 1;
  // Project-relative path, or an uploaded Artifact reference.
  string path = 2;
  // Hash is computed again by the Platform; this value is a claim.
  string claimed_sha256 = 3;
  string upload_ref = 4;
}

message Usage {
  uint64 input_tokens = 1;
  uint64 output_tokens = 2;
  google.protobuf.Duration wall_time = 3;
}

message ResultAck {
  string attempt_id = 1;
  ResultDisposition disposition = 2;
}

enum ResultDisposition {
  RESULT_DISPOSITION_UNSPECIFIED = 0;
  RESULT_DISPOSITION_ACCEPTED = 1;
  // Duplicate completion for an attempt already completed: ignored.
  RESULT_DISPOSITION_DUPLICATE = 2;
  // Worker no longer holds the Lease; Event result.rejected_stale recorded.
  RESULT_DISPOSITION_REJECTED_STALE = 3;
}

// ---- Control ----

message Cancel {
  string attempt_id = 1;
  string reason = 2;
}

message SecretDelivery {
  string attempt_id = 1;
  uint64 fencing_token = 2;
  // Held in memory only, scoped to this Step, wiped at Lease end.
  repeated SecretValue secrets = 3;
}

message SecretValue {
  // Name only appears in logs; the value never does.
  string name = 1;
  bytes value = 2;
}

message Drain {}

message ProtocolError {
  ErrorCode code = 1;
  string detail = 2;
}

enum ErrorCode {
  ERROR_CODE_UNSPECIFIED = 0;
  ERROR_CODE_UNSUPPORTED_VERSION = 1;
  ERROR_CODE_AUTHENTICATION_FAILED = 2;
  ERROR_CODE_REVOKED = 3;
  ERROR_CODE_MALFORMED = 4;
}
```

## 6. Fixtures (examples, JSON form)

Fixtures are shown in Protobuf JSON mapping. They are synthetic, hold no secrets and contain placeholders only. Valid and invalid pairs ship with the M2 contract package; the list here is the required minimum.

```json
{"messageId": "1", "hello": {"protocolVersions": ["amaru.worker.v1"], "workerId": "", "enrollmentToken": "<one-time-token>", "publicKey": "<base64-ed25519-public-key>", "workerVersion": "0.1.0", "manifest": {"os": "windows", "architecture": "amd64", "tools": [{"name": "git", "version": "2.45"}], "capabilities": ["repo.checkout"], "labels": {"pool": "build"}, "slots": 2, "sandboxSupported": false}}}
```

```json
{"messageId": "7", "ackId": "6", "assignment": {"executionId": "<execution-id>", "stepId": "build", "attemptId": "<attempt-id>", "capability": "build.run", "inputsJson": "<base64-json>", "revision": {"repository": "<remote-url>", "commitSha": "<40-hex>", "credentialSecretRef": "repo-token"}, "lease": {"leaseId": "<lease-id>", "fencingToken": "3", "expiresAt": "2026-10-26T12:00:30Z"}, "idempotencyKey": "<key>", "capabilityIdempotent": true}}
```

```json
{"messageId": "12", "result": {"attemptId": "<attempt-id>", "fencingToken": "2", "outcome": "RESULT_OUTCOME_SUCCEEDED"}}
```

The last one is **valid on the wire but must be answered** with `RESULT_DISPOSITION_REJECTED_STALE` when the Step's current token is 3. Invalid fixtures required: missing `attempt_id`; `Hello` with no `protocol_versions`; a `SecretDelivery` inside an `Assignment`-only phase; `Result` after `LeaseRelease`; frame with `message_id` going backwards.

## 7. Evidence levels

| Criterion (issue #7) | Level |
|---|---|
| Messages cover handshake, enrollment challenge, offer, assignment, lease, events, result, cancel, secret delivery | Documented; **not validated** against a running Worker. |
| In-process transport for the embedded Worker | Documented (§3); **not validated**. The WP-M1-02 spike used its own in-process dispatch, not this schema. |
| Idempotency and lease fields | Documented (§2); **not validated**. The spike exercised leases and fencing semantics (86/86 crash cases) but not these messages. |
| `buf lint` | **Run, clean** on a scratch copy (command above). Not run in CI; no committed `buf` configuration. |
| Schemas and fixtures reviewed | **Requires human review**. Fixtures are examples, not machine-validated. |
| Wire behavior (reconnect, NAT, proxy, Windows process trees) | **Not validated**; M5 spike ([ADR-0029](adr/0029-worker-protocol-over-websocket-and-protobuf.md)). |

## 8. Open points for the M2 contract work

- Exact receiver behavior for unknown `oneof` bodies across minor versions.
- Whether `inputs_json`/`data_json` stay `bytes` or move to `google.protobuf.Struct`.
- Chunking threshold for logs before an upload is required.
- Where the `buf breaking` baseline is stored ([CONTRACT_VERSIONING](CONTRACT_VERSIONING.md)).

Related: [ADR-0029](adr/0029-worker-protocol-over-websocket-and-protobuf.md) · [ADR-0024](adr/0024-remote-worker-registration-identity-and-delivery.md) · [ADR-0042](adr/0042-durable-execution-with-explicit-restart-semantics.md) · [ADR-0044](adr/0044-restart-semantics-clarifications-from-the-m1-spike.md) · [EXECUTION_ENVIRONMENTS](EXECUTION_ENVIRONMENTS.md) · [WP-M1-06](WORK_PACKAGES.md#wp-m1-06-worker-protocol-schemas) (#7) · [WP-M1-02 report](investigations/2026-10-26-wp-m1-02-storage-and-execution.md) (#3).
