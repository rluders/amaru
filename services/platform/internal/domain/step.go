package domain

import (
	"fmt"
	"slices"
	"time"
)

// StepState is the lifecycle state of a Step Execution (DOMAIN_MODEL §5.2).
type StepState string

const (
	StepPending          StepState = "pending"
	StepSkipped          StepState = "skipped"           // trigger or condition not met; always has a reason
	StepRefused          StepState = "refused"           // Policy or confinement refusal; always has a reason
	StepAwaitingApproval StepState = "awaiting_approval" // nothing runs while a Step is here
	StepQueued           StepState = "queued"
	StepLeased           StepState = "leased" // a Worker accepted it and has not reported it running
	StepRunning          StepState = "running"
	StepSucceeded        StepState = "succeeded"
	StepFailed           StepState = "failed"
	StepInterrupted      StepState = "interrupted" // restart or lost Lease of a non-idempotent Step (ADR-0042); a human or Policy rule resumes or cancels it
	StepTimedOut         StepState = "timed_out"   // exceeded its declared timeout (ADR-0044); resumed or cancelled like interrupted, never retried
	StepCancelled        StepState = "cancelled"
)

// StepStates lists every Step state, in lifecycle order.
var StepStates = []StepState{StepPending, StepSkipped, StepRefused, StepAwaitingApproval, StepQueued,
	StepLeased, StepRunning, StepSucceeded, StepFailed, StepTimedOut, StepInterrupted, StepCancelled}

// stepTransitions is the complete set of documented transitions. Anything absent is invalid.
var stepTransitions = map[StepState][]StepState{
	StepPending:          {StepSkipped, StepRefused, StepAwaitingApproval, StepQueued},
	StepAwaitingApproval: {StepQueued, StepFailed},
	StepQueued:           {StepLeased, StepCancelled},
	StepLeased:           {StepRunning, StepQueued},
	StepRunning:          {StepSucceeded, StepFailed, StepTimedOut, StepQueued, StepInterrupted, StepCancelled},
	StepInterrupted:      {StepQueued, StepCancelled},
	StepTimedOut:         {StepQueued, StepCancelled},
}

func (s StepState) Valid() bool { return slices.Contains(StepStates, s) }

// Terminal reports whether the state is final: no transition leaves it. interrupted and
// timed_out are not terminal: a person (or a Policy rule naming the Step) resumes or cancels them.
func (s StepState) Terminal() bool { return s.Valid() && len(stepTransitions[s]) == 0 }

// CanTransitionTo reports whether the lifecycle documents the move s → to.
func (s StepState) CanTransitionTo(to StepState) bool { return slices.Contains(stepTransitions[s], to) }

// Reasons recorded when a running Step is recovered (ADR-0042, ADR-0044).
const (
	ReasonPlatformRestarted = "platform_restarted"
	ReasonLeaseExpired      = "lease_expired"
	ReasonWorkerLost        = "worker_lost"
	ReasonStepTimeout       = "step_timeout"
	ReasonResumed           = "resumed"
	ReasonAttemptRejected   = "attempt_rejected"
)

// StepExecution is one Step of an Execution. Only its lifecycle is modelled here.
type StepExecution struct {
	ID          string     `json:"id"`
	ExecutionID string     `json:"executionId"`
	State       StepState  `json:"state"`
	Reason      string     `json:"reason,omitempty"` // skipped, refused, interrupted, re-queued
	StartedAt   *time.Time `json:"startedAt,omitempty"`
	EndedAt     *time.Time `json:"endedAt,omitempty"`
	Error       *ExecError `json:"error,omitempty"`
	// Version is the optimistic-concurrency counter of the stored row. The store sets it; a
	// change is accepted only against the version it was read at.
	Version int64 `json:"version"`
	// Idempotent is the Capability's declaration, copied when the Step is planned: its side
	// effects are guarded by idempotency keys, so a restart may re-run it on a new Lease (ADR-0042 rule 3).
	Idempotent bool `json:"idempotent,omitempty"`
	// RemoteJob is the remote job a Worker recorded for this Step, if any (ADR-0042 rule 2).
	RemoteJob *RemoteJob `json:"remoteJob,omitempty"`
	// Capability is the vendor-neutral operation the Step runs (e.g. "code.test"). InputsJSON is
	// its resolved inputs as canonical JSON. Both are set when the Step is planned.
	Capability string `json:"capability,omitempty"`
	InputsJSON []byte `json:"inputs,omitempty"`
	// AttemptID names the latest attempt (one per Lease) and FenceToken is the fencing token
	// issued with it, strictly increasing per Step. Both survive the Lease so that a late result
	// can be told from a duplicate and from a stale one (ADR-0044 point 4).
	AttemptID  string `json:"attemptId,omitempty"`
	FenceToken uint64 `json:"fenceToken,omitempty"`
	// CurrentLease is the Lease held while the Step is leased or running; nil otherwise.
	CurrentLease *LeaseInfo `json:"lease,omitempty"`
	// Agent is the snapshot of the Agent (and its Skills) an agent Step runs with. Nil for other
	// Steps. Set once, while the Step is pending, and never changed: a later edit of the Agent
	// definition does not rewrite what this Step used.
	Agent *ResolvedAgent `json:"agent,omitempty"`
}

// RemoteJob identifies a job running outside the Platform. Resumable is the Provider's
// declaration that a new Lease can re-attach to it.
type RemoteJob struct {
	ID        string `json:"id"`
	Resumable bool   `json:"resumable"`
}

// LeaseInfo records who holds a Step and until when. A Lease nobody renews expires.
type LeaseInfo struct {
	Owner     string    `json:"owner"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// BindAgent records the Agent snapshot of a pending Step. A second bind is refused, so the
// snapshot cannot drift from what the Step ran with.
func (s *StepExecution) BindAgent(a ResolvedAgent) error {
	switch {
	case s.State != StepPending:
		return fmt.Errorf("%w: agent is bound while pending, step is %s", ErrInvalidTransition, s.State)
	case s.Agent != nil:
		return fmt.Errorf("%w: step already has an agent snapshot", ErrConflict)
	}
	a.ProviderPreference = append([]string{}, a.ProviderPreference...)
	a.Skills = append([]SkillUse(nil), a.Skills...)
	a.WorktreeCommands = append([]string(nil), a.WorktreeCommands...)
	s.Agent = &a
	return nil
}

// move changes state only from the one state the calling method is defined for, and only along
// a documented edge, so a method cannot reach a target from another source that also has an edge to it.
func (s *StepExecution) move(from, to StepState) error {
	if err := s.check(from, to); err != nil {
		return err
	}
	s.State = to
	return nil
}

func (s *StepExecution) check(from, to StepState) error {
	if s.State != from || !s.State.CanTransitionTo(to) {
		return fmt.Errorf("%w: step %s -> %s", ErrInvalidTransition, s.State, to)
	}
	return nil
}

// Plan moves pending → queued.
func (s *StepExecution) Plan() error { return s.move(StepPending, StepQueued) }

// Skip moves pending → skipped. A Step is never skipped silently.
func (s *StepExecution) Skip(now time.Time, reason string) error {
	return s.endWithReason(now, StepSkipped, reason)
}

// Refuse moves pending → refused because Policy or confinement refused it.
func (s *StepExecution) Refuse(now time.Time, reason string) error {
	return s.endWithReason(now, StepRefused, reason)
}

func (s *StepExecution) endWithReason(now time.Time, to StepState, reason string) error {
	if err := s.check(StepPending, to); err != nil {
		return err
	}
	if reason == "" {
		return ErrReasonRequired
	}
	s.State, s.Reason, s.EndedAt = to, reason, &now
	return nil
}

// RequireApproval moves pending → awaiting_approval.
func (s *StepExecution) RequireApproval() error { return s.move(StepPending, StepAwaitingApproval) }

// Approve moves awaiting_approval → queued.
func (s *StepExecution) Approve() error { return s.move(StepAwaitingApproval, StepQueued) }

// Reject moves awaiting_approval → failed because the Approval was rejected or expired.
func (s *StepExecution) Reject(now time.Time, err *ExecError) error {
	return s.fail(now, StepAwaitingApproval, err)
}

// Lease moves queued → leased when a Worker is offered the Step as attempt attemptID, until
// expiresAt. It issues the next fencing token (the previous one plus one).
func (s *StepExecution) Lease(owner, attemptID string, expiresAt time.Time) error {
	if owner == "" || attemptID == "" {
		return fmt.Errorf("%w: a lease needs an owner and an attempt id", ErrInvalid)
	}
	if err := s.move(StepQueued, StepLeased); err != nil {
		return err
	}
	s.AttemptID, s.FenceToken = attemptID, s.FenceToken+1
	s.CurrentLease = &LeaseInfo{Owner: owner, ExpiresAt: expiresAt}
	s.Reason = "" // whatever kept the Step queued no longer applies
	return nil
}

// Reasons a queued Step is not leased yet: the named scope has no free slot (FEAT-EXE-006).
const (
	ReasonLimitGlobal      = "limit:global"
	ReasonLimitProject     = "limit:project"
	ReasonLimitEnvironment = "limit:environment"
)

// Defer records why a queued Step waits. It changes nothing else and reports whether the reason
// changed, so a caller writes only when there is something new to show.
func (s *StepExecution) Defer(reason string) (changed bool, err error) {
	if s.State != StepQueued {
		return false, fmt.Errorf("%w: only a queued step can wait for a slot, this one is %s", ErrInvalidTransition, s.State)
	}
	if reason == "" {
		return false, fmt.Errorf("%w: a deferral needs a reason", ErrReasonRequired)
	}
	if s.Reason == reason {
		return false, nil
	}
	s.Reason = reason
	return true, nil
}

// HoldsLease reports whether the Step is leased or running under exactly this attempt and token.
// A holder with a lower token, or for an earlier attempt, is stale and must be refused.
func (s *StepExecution) HoldsLease(attemptID string, token uint64) bool {
	return (s.State == StepLeased || s.State == StepRunning) && s.CurrentLease != nil &&
		s.AttemptID == attemptID && s.FenceToken == token
}

// RenewLease extends the Lease of the attempt that holds it. A stale holder gets ErrStaleLease.
func (s *StepExecution) RenewLease(attemptID string, token uint64, expiresAt time.Time) error {
	if !s.HoldsLease(attemptID, token) {
		return fmt.Errorf("%w: attempt %s", ErrStaleLease, attemptID)
	}
	s.CurrentLease.ExpiresAt = expiresAt
	return nil
}

// ExpireLease moves leased → queued: the Lease ended before the Step started, so nothing ran
// and it is re-leased whatever its idempotency (ADR-0044).
func (s *StepExecution) ExpireLease() error { return s.ReleaseLease(ReasonLeaseExpired) }

// ReleaseLease moves leased → queued with a reason: the Lease ended, or the Worker refused the
// attempt, before the Step started. Nothing ran, so the Step is re-leased whatever its kind.
func (s *StepExecution) ReleaseLease(reason string) error {
	if reason == "" {
		return ErrReasonRequired
	}
	if err := s.move(StepLeased, StepQueued); err != nil {
		return err
	}
	s.CurrentLease, s.Reason = nil, reason
	return nil
}

// Start moves leased → running.
func (s *StepExecution) Start(now time.Time) error {
	if err := s.move(StepLeased, StepRunning); err != nil {
		return err
	}
	s.StartedAt = &now
	return nil
}

// Cancel moves queued → cancelled. A running Step is cancelled through Finish.
func (s *StepExecution) Cancel(now time.Time) error {
	if err := s.move(StepQueued, StepCancelled); err != nil {
		return err
	}
	s.EndedAt = &now
	return nil
}

// Finish moves running → succeeded, failed or cancelled. Failed requires err.
func (s *StepExecution) Finish(now time.Time, to StepState, err *ExecError) error {
	switch to {
	case StepFailed:
		return s.fail(now, StepRunning, err)
	case StepSucceeded, StepCancelled:
		if mv := s.move(StepRunning, to); mv != nil {
			return mv
		}
		s.EndedAt, s.CurrentLease = &now, nil
		return nil
	}
	return fmt.Errorf("%w: step %s -> %s", ErrInvalidTransition, s.State, to)
}

func (s *StepExecution) fail(now time.Time, from StepState, err *ExecError) error {
	if mv := s.check(from, StepFailed); mv != nil {
		return mv
	}
	if err == nil {
		return ErrErrorRequired
	}
	s.State, s.Error, s.EndedAt, s.CurrentLease = StepFailed, err, &now, nil
	return nil
}

// Recover applies the restart rule of ADR-0042 to a running Step whose Worker is gone
// (Platform restart, expired Lease, lost Worker): an idempotent Step goes back to queued to be
// re-leased; any other ends interrupted and is never re-run automatically. reason is one of
// the Reason* constants. Resumable remote jobs (ADR-0042 rule 2) are decided by the caller and
// do not pass through here.
func (s *StepExecution) Recover(now time.Time, idempotent bool, reason string) error {
	to := StepInterrupted
	if idempotent {
		to = StepQueued
	}
	if mv := s.check(StepRunning, to); mv != nil {
		return mv
	}
	if reason == "" {
		return ErrReasonRequired
	}
	s.State, s.Reason, s.CurrentLease = to, reason, nil
	if to == StepInterrupted {
		s.EndedAt = &now
	}
	return nil
}

// RestartAction is what Reconcile did to a Step.
type RestartAction string

const (
	RestartKept        RestartAction = "kept"        // nothing ran or a person must act: the Step keeps waiting (rule 1)
	RestartResumed     RestartAction = "resumed"     // queued again to re-attach to its resumable remote job (rule 2)
	RestartRetried     RestartAction = "retried"     // queued again to run on a new Lease (rule 3, and a Lease that ended before the Step started)
	RestartInterrupted RestartAction = "interrupted" // ended interrupted, never re-run automatically (rule 4)
)

// Reconcile applies the restart rule of ADR-0042 to one non-terminal Step when the Platform starts
// or a Worker is lost. reason is recorded on a running Step that is recovered (ReasonPlatformRestarted
// or ReasonWorkerLost). It never runs a Step: it only decides where the Step waits.
//
//	pending, awaiting_approval, queued, interrupted, timed_out   kept (nothing ran, or a person decides)
//	leased, Lease still valid at now                             kept
//	leased, Lease ended                                          retried (nothing ran, so any Step)
//	running, resumable remote job recorded                       resumed (remote job kept)
//	running, idempotent                                          retried
//	running, anything else                                       interrupted
//
// Terminal Steps return ErrInvalidTransition.
func (s *StepExecution) Reconcile(now time.Time, reason string) (RestartAction, error) {
	switch s.State {
	case StepPending, StepAwaitingApproval, StepQueued, StepInterrupted, StepTimedOut:
		return RestartKept, nil
	case StepLeased:
		if s.CurrentLease != nil && s.CurrentLease.ExpiresAt.After(now) {
			return RestartKept, nil
		}
		return RestartRetried, s.ExpireLease()
	case StepRunning:
		switch {
		case s.RemoteJob != nil && s.RemoteJob.Resumable:
			return RestartResumed, s.Recover(now, true, reason)
		case s.Idempotent:
			return RestartRetried, s.Recover(now, true, reason)
		}
		return RestartInterrupted, s.Recover(now, false, reason)
	}
	return "", fmt.Errorf("%w: step %s is terminal", ErrInvalidTransition, s.State)
}

// TimeOut moves running → timed_out: the Step exceeded its declared timeout (ADR-0044). It is
// never re-run automatically, whatever its kind, and a timeout is not a retry trigger.
func (s *StepExecution) TimeOut(now time.Time) error {
	if err := s.move(StepRunning, StepTimedOut); err != nil {
		return err
	}
	s.Reason, s.EndedAt, s.CurrentLease = ReasonStepTimeout, &now, nil
	return nil
}

// Resume moves interrupted or timed_out → queued so the Step is leased again (ADR-0042, ADR-0044).
// Only a human, or a Policy rule that names the Step (ActorPolicy with its Authority), may do
// it; anything else gets ErrNotPermitted. The caller checks that the Execution still runs the
// same Workflow version and records an Event with the actor. Resuming spends no attempt.
func (s *StepExecution) Resume(by Actor) error {
	if !by.mayResume() {
		return fmt.Errorf("%w: %q may not resume a step", ErrNotPermitted, by.Kind)
	}
	if s.State != StepInterrupted && s.State != StepTimedOut {
		return fmt.Errorf("%w: step %s -> %s", ErrInvalidTransition, s.State, StepQueued)
	}
	s.State, s.Reason, s.EndedAt = StepQueued, ReasonResumed, nil
	return nil
}

// Abandon moves interrupted or timed_out → cancelled: the person decided not to resume it.
func (s *StepExecution) Abandon(now time.Time, by Actor) error {
	if !by.mayResume() {
		return fmt.Errorf("%w: %q may not cancel a step", ErrNotPermitted, by.Kind)
	}
	if s.State != StepInterrupted && s.State != StepTimedOut {
		return fmt.Errorf("%w: step %s -> %s", ErrInvalidTransition, s.State, StepCancelled)
	}
	s.State, s.EndedAt = StepCancelled, &now
	return nil
}
