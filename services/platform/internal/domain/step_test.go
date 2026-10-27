package domain

import (
	"errors"
	"testing"
)

// documentedStep restates DOMAIN_MODEL §5.2 independently of the implementation table.
var documentedStep = map[[2]StepState]bool{
	{StepPending, StepSkipped}:          true,
	{StepPending, StepRefused}:          true,
	{StepPending, StepAwaitingApproval}: true,
	{StepPending, StepQueued}:           true,
	{StepAwaitingApproval, StepQueued}:  true,
	{StepAwaitingApproval, StepFailed}:  true,
	{StepQueued, StepLeased}:            true,
	{StepLeased, StepRunning}:           true,
	{StepRunning, StepSucceeded}:        true,
	{StepRunning, StepFailed}:           true,
	{StepRunning, StepQueued}:           true,
	{StepRunning, StepInterrupted}:      true,
	{StepLeased, StepQueued}:            true,
	{StepRunning, StepCancelled}:        true,
	{StepQueued, StepCancelled}:         true,
}

func TestStepOnlyDocumentedTransitions(t *testing.T) {
	for _, from := range StepStates {
		for _, to := range StepStates {
			if got, want := from.CanTransitionTo(to), documentedStep[[2]StepState{from, to}]; got != want {
				t.Errorf("%s -> %s: allowed=%v, documented=%v", from, to, got, want)
			}
		}
	}
	for _, s := range []StepState{StepSkipped, StepRefused, StepSucceeded, StepFailed, StepInterrupted, StepCancelled} {
		if !s.Terminal() {
			t.Errorf("%s must be terminal", s)
		}
	}
	for _, s := range []StepState{StepPending, StepAwaitingApproval, StepQueued, StepLeased, StepRunning} {
		if s.Terminal() {
			t.Errorf("%s must not be terminal", s)
		}
	}
}

func TestStepMethodsRejectEveryOtherState(t *testing.T) {
	boom := &ExecError{Code: "x", Message: "y"}
	methods := map[string]struct {
		from StepState
		call func(*StepExecution) error
	}{
		"Plan":            {StepPending, func(s *StepExecution) error { return s.Plan() }},
		"Skip":            {StepPending, func(s *StepExecution) error { return s.Skip(t0, "r") }},
		"Refuse":          {StepPending, func(s *StepExecution) error { return s.Refuse(t0, "r") }},
		"RequireApproval": {StepPending, func(s *StepExecution) error { return s.RequireApproval() }},
		"Approve":         {StepAwaitingApproval, func(s *StepExecution) error { return s.Approve() }},
		"Reject":          {StepAwaitingApproval, func(s *StepExecution) error { return s.Reject(t0, boom) }},
		"Lease":           {StepQueued, func(s *StepExecution) error { return s.Lease() }},
		"ExpireLease":     {StepLeased, func(s *StepExecution) error { return s.ExpireLease() }},
		"Start":           {StepLeased, func(s *StepExecution) error { return s.Start(t0) }},
		"Cancel":          {StepQueued, func(s *StepExecution) error { return s.Cancel(t0) }},
		"Finish":          {StepRunning, func(s *StepExecution) error { return s.Finish(t0, StepFailed, boom) }},
		"RecoverIdem":     {StepRunning, func(s *StepExecution) error { return s.Recover(t0, true, ReasonLeaseExpired) }},
		"RecoverNonIdem":  {StepRunning, func(s *StepExecution) error { return s.Recover(t0, false, ReasonPlatformRestarted) }},
	}
	for name, m := range methods {
		for _, state := range StepStates {
			s := StepExecution{State: state}
			err := m.call(&s)
			if state == m.from {
				if err != nil {
					t.Errorf("%s from %s: %v", name, state, err)
				}
				continue
			}
			if !errors.Is(err, ErrInvalidTransition) || s.State != state {
				t.Errorf("%s from %s: want ErrInvalidTransition and no change, got %v (state %s)", name, state, err, s.State)
			}
		}
	}
}

func TestStepRequiredDetails(t *testing.T) {
	p := StepExecution{State: StepPending}
	if err := p.Skip(t0, ""); !errors.Is(err, ErrReasonRequired) || p.State != StepPending {
		t.Errorf("skip needs a reason: %v", err)
	}
	if err := p.Refuse(t0, ""); !errors.Is(err, ErrReasonRequired) || p.State != StepPending {
		t.Errorf("refuse needs a reason: %v", err)
	}
	if err := p.Skip(t0, "condition false"); err != nil || p.Reason != "condition false" || p.EndedAt == nil {
		t.Errorf("skip: %v %+v", err, p)
	}
	a := StepExecution{State: StepAwaitingApproval}
	if err := a.Reject(t0, nil); !errors.Is(err, ErrErrorRequired) || a.State != StepAwaitingApproval {
		t.Errorf("reject needs an error: %v", err)
	}
	r := StepExecution{State: StepRunning}
	if err := r.Finish(t0, StepFailed, nil); !errors.Is(err, ErrErrorRequired) || r.State != StepRunning {
		t.Errorf("failed needs an error: %v", err)
	}
	for _, to := range []StepState{StepQueued, StepLeased, StepInterrupted, StepSkipped, "bogus"} {
		r := StepExecution{State: StepRunning}
		if err := r.Finish(t0, to, nil); !errors.Is(err, ErrInvalidTransition) || r.State != StepRunning {
			t.Errorf("Finish must not produce %s: %v", to, err)
		}
	}
	rr := StepExecution{State: StepRunning}
	if err := rr.Recover(t0, false, ""); !errors.Is(err, ErrReasonRequired) || rr.State != StepRunning {
		t.Errorf("recover needs a reason: %v", err)
	}
}

// ADR-0042 rule 3/4: idempotent → re-queued; otherwise interrupted, never re-run automatically.
func TestStepRestartRule(t *testing.T) {
	idem := StepExecution{State: StepRunning}
	if err := idem.Recover(t0, true, ReasonPlatformRestarted); err != nil ||
		idem.State != StepQueued || idem.Reason != ReasonPlatformRestarted || idem.EndedAt != nil {
		t.Fatalf("idempotent: %v %+v", err, idem)
	}
	non := StepExecution{State: StepRunning}
	if err := non.Recover(t0, false, ReasonLeaseExpired); err != nil ||
		non.State != StepInterrupted || non.Reason != ReasonLeaseExpired || non.EndedAt == nil {
		t.Fatalf("non-idempotent: %v %+v", err, non)
	}
	if err := non.Lease(); !errors.Is(err, ErrInvalidTransition) {
		t.Fatal("an interrupted Step must not be re-run automatically")
	}
	// ADR-0044 rule 1: leased but never started is re-leased regardless of idempotency.
	l := StepExecution{State: StepLeased}
	if err := l.ExpireLease(); err != nil || l.State != StepQueued {
		t.Fatalf("expire lease: %v", err)
	}
	if err := l.Lease(); err != nil {
		t.Fatalf("re-lease: %v", err)
	}
}

func TestStepHappyPathWithApproval(t *testing.T) {
	s := StepExecution{State: StepPending}
	for _, step := range []func() error{
		s.RequireApproval, s.Approve, s.Lease, func() error { return s.Start(t0) },
		func() error { return s.Finish(t0, StepSucceeded, nil) },
	} {
		if err := step(); err != nil {
			t.Fatal(err)
		}
	}
	if s.State != StepSucceeded || s.StartedAt == nil || s.EndedAt == nil {
		t.Fatalf("%+v", s)
	}
}
