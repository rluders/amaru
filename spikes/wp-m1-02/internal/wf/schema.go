package wf

// Schema is the execution state of the spike: Execution and Step rows with a
// monotonic version (optimistic concurrency), an ordered Event log, timers with
// a due time, and the simulated external world used to count side effects.
const Schema = `
create table if not exists executions(
  id text primary key, state text not null, next_seq int not null default 0);
create table if not exists steps(
  id text primary key, exec_id text not null references executions(id), ord int not null,
  name text not null, kind text not null, state text not null,
  version int not null default 0, attempts int not null default 0, max_attempts int not null default 3,
  timeout_s bigint, approval_deadline_s bigint, fail_first boolean not null default false,
  lease_owner text, lease_expires_at bigint, remote_job text, reason text);
create table if not exists events(
  exec_id text not null, seq int not null, kind text not null, step_id text, data text, at bigint not null,
  primary key(exec_id, seq));
create table if not exists timers(
  id bigserial primary key, kind text not null, step_id text not null, due bigint not null,
  step_version int not null, fired boolean not null default false);
create table if not exists sim_clock(id int primary key check (id=1), now bigint not null);
-- the simulated external world: one row per executed side effect
create table if not exists world(
  n bigserial primary key, step_id text not null, key text unique, run text);
`

// Step kinds.
const (
	KindIdempotent    = "idempotent"    // side effect guarded by an idempotency key
	KindNonIdempotent = "nonidempotent" // side effect with no key (agent pushes a branch)
	KindResumable     = "resumable"     // Provider declares Resumable and records a remote job id
	KindApproval      = "approval"      // persisted gate, holds no process
)

// Step states.
const (
	Pending         = "pending"
	Assigned        = "assigned"
	Running         = "running"
	WaitingApproval = "waiting_approval"
	RetryWait       = "retry_wait"
	Succeeded       = "succeeded"
	Failed          = "failed"
	Interrupted     = "interrupted"
	TimedOut        = "timed_out"
)
