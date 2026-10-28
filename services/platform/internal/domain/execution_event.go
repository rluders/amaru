package domain

import "time"

// Kinds of ExecutionEvent the coordinator records for restart recovery. Other kinds are added
// with the transitions that cause them.
const (
	EventStepRetried         = "step.retried"
	EventStepResumed         = "step.resumed"
	EventStepInterrupted     = "step.interrupted"
	EventExecWaiting         = "execution.waiting"
	EventExecCancelRequested = "execution.cancel_requested"
	EventExecCancelled       = "execution.cancelled"
	EventStepLeased          = "step.leased"
	EventStepStarted         = "step.started"
	EventStepRejected        = "step.attempt_rejected"
	EventStepSucceeded       = "step.succeeded"
	EventStepFailed          = "step.failed"
	EventStepRemoteJob       = "step.remote_job_recorded"
	EventStepTimedOut        = "step.timed_out"
	EventStepCancelled       = "step.cancelled"
	EventResultStale         = "result.rejected_stale"
	EventStepDeferred        = "step.deferred"
)

// ExecutionEvent is one entry of an Execution's ordered, append-only log. Seq starts at 1 and has
// no gaps; the store assigns it, in the same transaction as the state change the event records.
type ExecutionEvent struct {
	ExecutionID string    `json:"executionId"`
	Seq         int64     `json:"seq"`
	Kind        string    `json:"kind"`
	StepID      string    `json:"stepId,omitempty"`
	Reason      string    `json:"reason,omitempty"`
	At          time.Time `json:"at"`
}
