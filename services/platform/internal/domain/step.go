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
	StepInterrupted      StepState = "interrupted" // restart or lost Lease of a non-idempotent Step (ADR-0042)
	StepCancelled        StepState = "cancelled"
)

// StepStates lists every Step state, in lifecycle order.
var StepStates = []StepState{StepPending, StepSkipped, StepRefused, StepAwaitingApproval, StepQueued,
	StepLeased, StepRunning, StepSucceeded, StepFailed, StepInterrupted, StepCancelled}

// stepTransitions is the complete set of documented transitions. Anything absent is invalid.
var stepTransitions = map[StepState][]StepState{
	StepPending:          {StepSkipped, StepRefused, StepAwaitingApproval, StepQueued},
	StepAwaitingApproval: {StepQueued, StepFailed},
	StepQueued:           {StepLeased, StepCancelled},
	StepLeased:           {StepRunning, StepQueued},
	StepRunning:          {StepSucceeded, StepFailed, StepQueued, StepInterrupted, StepCancelled},
}

func (s StepState) Valid() bool { return slices.Contains(StepStates, s) }

// Terminal reports whether the state is final: no transition leaves it.
func (s StepState) Terminal() bool { return s.Valid() && len(stepTransitions[s]) == 0 }

// CanTransitionTo reports whether the lifecycle documents the move s → to.
func (s StepState) CanTransitionTo(to StepState) bool { return slices.Contains(stepTransitions[s], to) }

// Reasons recorded when a running Step is recovered (ADR-0042, ADR-0044).
const (
	ReasonPlatformRestarted = "platform_restarted"
	ReasonLeaseExpired      = "lease_expired"
	ReasonWorkerLost        = "worker_lost"
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
	// Agent is the snapshot of the Agent (and its Skills) an agent Step runs with. Nil for other
	// Steps. Set once, while the Step is pending, and never changed: a later edit of the Agent
	// definition does not rewrite what this Step used.
	Agent *ResolvedAgent `json:"agent,omitempty"`
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

// Lease moves queued → leased when a Worker accepts the Step.
func (s *StepExecution) Lease() error { return s.move(StepQueued, StepLeased) }

// ExpireLease moves leased → queued: the Lease ended before the Step started, so nothing ran
// and it is re-leased whatever its idempotency (ADR-0044).
func (s *StepExecution) ExpireLease() error { return s.move(StepLeased, StepQueued) }

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
		s.EndedAt = &now
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
	s.State, s.Error, s.EndedAt = StepFailed, err, &now
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
	s.State, s.Reason = to, reason
	if to == StepInterrupted {
		s.EndedAt = &now
	}
	return nil
}
