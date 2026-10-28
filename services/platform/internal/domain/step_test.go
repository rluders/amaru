package domain

import (
	"errors"
	"testing"
	"time"
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
	{StepRunning, StepTimedOut}:         true,
	{StepInterrupted, StepQueued}:       true,
	{StepInterrupted, StepCancelled}:    true,
	{StepTimedOut, StepQueued}:          true,
	{StepTimedOut, StepCancelled}:       true,
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
	for _, s := range []StepState{StepSkipped, StepRefused, StepSucceeded, StepFailed, StepCancelled} {
		if !s.Terminal() {
			t.Errorf("%s must be terminal", s)
		}
	}
	for _, s := range []StepState{StepPending, StepAwaitingApproval, StepQueued, StepLeased, StepRunning, StepInterrupted, StepTimedOut} {
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
		"Lease":           {StepQueued, func(s *StepExecution) error { return s.Lease("w1", "a1", t0.Add(time.Minute)) }},
		"ExpireLease":     {StepLeased, func(s *StepExecution) error { return s.ExpireLease() }},
		"Start":           {StepLeased, func(s *StepExecution) error { return s.Start(t0) }},
		"Cancel":          {StepQueued, func(s *StepExecution) error { return s.Cancel(t0) }},
		"Finish":          {StepRunning, func(s *StepExecution) error { return s.Finish(t0, StepFailed, boom) }},
		"RecoverIdem":     {StepRunning, func(s *StepExecution) error { return s.Recover(t0, true, ReasonLeaseExpired) }},
		"TimeOut":         {StepRunning, func(s *StepExecution) error { return s.TimeOut(t0) }},
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
	if err := non.Lease("w1", "a1", t0.Add(time.Minute)); !errors.Is(err, ErrInvalidTransition) {
		t.Fatal("an interrupted Step must not be re-run automatically")
	}
	if err := non.Resume(Actor{Kind: ActorHuman}); err != nil || non.State != StepQueued || non.EndedAt != nil {
		t.Fatalf("a human resumes it: %v %+v", err, non)
	}
	// ADR-0044 rule 1: leased but never started is re-leased regardless of idempotency.
	l := StepExecution{State: StepLeased}
	if err := l.ExpireLease(); err != nil || l.State != StepQueued {
		t.Fatalf("expire lease: %v", err)
	}
	if err := l.Lease("w1", "a1", t0.Add(time.Minute)); err != nil {
		t.Fatalf("re-lease: %v", err)
	}
}

func TestStepHappyPathWithApproval(t *testing.T) {
	s := StepExecution{State: StepPending}
	for _, step := range []func() error{
		s.RequireApproval, s.Approve, func() error { return s.Lease("w1", "a1", t0.Add(time.Minute)) }, func() error { return s.Start(t0) },
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

// ADR-0042/0044: interrupted and timed_out wait for a human or a named Policy rule; they are
// never re-run automatically.
func TestStepResumeAndAbandon(t *testing.T) {
	human := Actor{Kind: ActorHuman}
	policy := Actor{Kind: ActorPolicy, Authority: "rule-resume-build"}
	for _, from := range []StepState{StepInterrupted, StepTimedOut} {
		for name, by := range map[string]Actor{"human": human, "policy": policy} {
			s := StepExecution{State: from, Reason: "x", EndedAt: &t0}
			if err := s.Resume(by); err != nil || s.State != StepQueued || s.Reason != ReasonResumed || s.EndedAt != nil {
				t.Errorf("%s resume %s: %v %+v", name, from, err, s)
			}
			a := StepExecution{State: from}
			if err := a.Abandon(t0, by); err != nil || a.State != StepCancelled || a.EndedAt == nil {
				t.Errorf("%s abandon %s: %v %+v", name, from, err, a)
			}
		}
		for name, by := range map[string]Actor{
			"agent": {Kind: "agent"}, "step": {Kind: "step"}, "zero": {},
			"loop":           {Kind: ActorDeliveryLoop, Authority: "p"},
			"policy no rule": {Kind: ActorPolicy},
		} {
			s := StepExecution{State: from}
			if err := s.Resume(by); !errors.Is(err, ErrNotPermitted) || s.State != from {
				t.Errorf("%s must not resume %s: %v", name, from, err)
			}
			if err := s.Abandon(t0, by); !errors.Is(err, ErrNotPermitted) || s.State != from {
				t.Errorf("%s must not abandon %s: %v", name, from, err)
			}
		}
	}
	for _, state := range StepStates {
		if state == StepInterrupted || state == StepTimedOut {
			continue
		}
		s := StepExecution{State: state}
		if err := s.Resume(human); !errors.Is(err, ErrInvalidTransition) || s.State != state {
			t.Errorf("Resume from %s: %v", state, err)
		}
		if err := s.Abandon(t0, human); !errors.Is(err, ErrInvalidTransition) || s.State != state {
			t.Errorf("Abandon from %s: %v", state, err)
		}
	}
}

func TestStepTimeOut(t *testing.T) {
	s := StepExecution{State: StepRunning}
	if err := s.TimeOut(t0); err != nil || s.State != StepTimedOut || s.Reason != ReasonStepTimeout || s.EndedAt == nil {
		t.Fatalf("timeout: %v %+v", err, s)
	}
	if err := s.Lease("w1", "a1", t0.Add(time.Minute)); !errors.Is(err, ErrInvalidTransition) {
		t.Fatal("a timed-out Step must not be re-run automatically")
	}
}

// reconcileCase builds a Step in the state under test.
func reconcileStep(t *testing.T, state StepState, mut func(*StepExecution)) StepExecution {
	t.Helper()
	s := StepExecution{ID: "s1", ExecutionID: "e1", State: state}
	if mut != nil {
		mut(&s)
	}
	return s
}

// TestReconcileRestartRule is ADR-0042's restart rule, one row per Step state and declaration.
func TestReconcileRestartRule(t *testing.T) {
	now := t0.Add(time.Hour)
	valid := &LeaseInfo{Owner: "w1", ExpiresAt: now.Add(time.Minute)}
	ended := &LeaseInfo{Owner: "w1", ExpiresAt: now.Add(-time.Minute)}
	tests := []struct {
		name   string
		step   StepExecution
		action RestartAction
		state  StepState
		reason string
	}{
		{"pending keeps waiting", reconcileStep(t, StepPending, nil), RestartKept, StepPending, ""},
		{"awaiting approval keeps waiting", reconcileStep(t, StepAwaitingApproval, nil), RestartKept, StepAwaitingApproval, ""},
		{"queued keeps waiting", reconcileStep(t, StepQueued, nil), RestartKept, StepQueued, ""},
		{"interrupted waits for a person", reconcileStep(t, StepInterrupted, nil), RestartKept, StepInterrupted, ""},
		{"timed out waits for a person", reconcileStep(t, StepTimedOut, nil), RestartKept, StepTimedOut, ""},
		{"leased with a valid lease keeps waiting", reconcileStep(t, StepLeased, func(s *StepExecution) { s.CurrentLease = valid }), RestartKept, StepLeased, ""},
		{"leased with an ended lease is retried", reconcileStep(t, StepLeased, func(s *StepExecution) { s.CurrentLease = ended }), RestartRetried, StepQueued, ReasonLeaseExpired},
		{"leased without a lease record is retried", reconcileStep(t, StepLeased, nil), RestartRetried, StepQueued, ReasonLeaseExpired},
		{"leased and not idempotent still retried: nothing ran", reconcileStep(t, StepLeased, func(s *StepExecution) { s.CurrentLease = ended; s.Idempotent = false }), RestartRetried, StepQueued, ReasonLeaseExpired},
		{"running resumable remote job resumes", reconcileStep(t, StepRunning, func(s *StepExecution) { s.RemoteJob = &RemoteJob{ID: "j1", Resumable: true} }), RestartResumed, StepQueued, ReasonPlatformRestarted},
		{"running with a non-resumable remote job is interrupted", reconcileStep(t, StepRunning, func(s *StepExecution) { s.RemoteJob = &RemoteJob{ID: "j1"} }), RestartInterrupted, StepInterrupted, ReasonPlatformRestarted},
		{"running idempotent is retried", reconcileStep(t, StepRunning, func(s *StepExecution) { s.Idempotent = true }), RestartRetried, StepQueued, ReasonPlatformRestarted},
		{"running other is interrupted", reconcileStep(t, StepRunning, nil), RestartInterrupted, StepInterrupted, ReasonPlatformRestarted},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := tc.step
			got, err := s.Reconcile(now, ReasonPlatformRestarted)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.action || s.State != tc.state || s.Reason != tc.reason {
				t.Fatalf("got %s → %s (%q), want %s → %s (%q)", got, s.State, s.Reason, tc.action, tc.state, tc.reason)
			}
			if s.State != StepLeased && s.State != StepRunning && s.CurrentLease != nil {
				t.Fatalf("lease kept after leaving leased/running: %+v", s.CurrentLease)
			}
			if got == RestartInterrupted && s.EndedAt == nil {
				t.Fatal("interrupted Step has no end time")
			}
		})
	}
}

func TestReconcileKeepsRemoteJobAndNeverSpendsInterruptionAsRetry(t *testing.T) {
	s := reconcileStep(t, StepRunning, func(s *StepExecution) { s.RemoteJob = &RemoteJob{ID: "j1", Resumable: true} })
	if _, err := s.Reconcile(t0, ReasonWorkerLost); err != nil {
		t.Fatal(err)
	}
	if s.RemoteJob == nil || s.RemoteJob.ID != "j1" || s.Reason != ReasonWorkerLost {
		t.Fatalf("remote job or reason lost: %+v", s)
	}
}

func TestReconcileTerminalRefused(t *testing.T) {
	for _, st := range []StepState{StepSkipped, StepRefused, StepSucceeded, StepFailed, StepCancelled} {
		s := reconcileStep(t, st, nil)
		if _, err := s.Reconcile(t0, ReasonPlatformRestarted); !errors.Is(err, ErrInvalidTransition) {
			t.Errorf("%s: got %v, want ErrInvalidTransition", st, err)
		}
	}
}

func TestLeaseNeedsOwnerAndClearsOnFinish(t *testing.T) {
	s := reconcileStep(t, StepQueued, nil)
	if err := s.Lease("", "a1", t0); !errors.Is(err, ErrInvalid) {
		t.Fatalf("empty owner: got %v", err)
	}
	if err := s.Lease("w1", "a1", t0.Add(time.Minute)); err != nil || s.CurrentLease == nil || s.CurrentLease.Owner != "w1" {
		t.Fatalf("lease not recorded: %v %+v", err, s.CurrentLease)
	}
	if err := s.Start(t0); err != nil {
		t.Fatal(err)
	}
	if err := s.Finish(t0, StepSucceeded, nil); err != nil || s.CurrentLease != nil {
		t.Fatalf("lease kept after finish: %v %+v", err, s.CurrentLease)
	}
}

func TestFencingTokenGrowsPerLeaseAndSurvivesIt(t *testing.T) {
	s := reconcileStep(t, StepQueued, nil)
	exp := t0.Add(time.Minute)
	if err := s.Lease("w1", "a1", exp); err != nil || s.FenceToken != 1 || s.AttemptID != "a1" {
		t.Fatalf("first lease: %v token=%d attempt=%q", err, s.FenceToken, s.AttemptID)
	}
	if err := s.ReleaseLease(ReasonAttemptRejected); err != nil || s.State != StepQueued || s.CurrentLease != nil || s.Reason != ReasonAttemptRejected {
		t.Fatalf("release: %v %+v", err, s)
	}
	if s.FenceToken != 1 || s.AttemptID != "a1" {
		t.Fatalf("token or attempt lost on release: %+v", s)
	}
	if err := s.Lease("w2", "a2", exp); err != nil || s.FenceToken != 2 || s.AttemptID != "a2" {
		t.Fatalf("second lease: %v token=%d attempt=%q", err, s.FenceToken, s.AttemptID)
	}
	if s.HoldsLease("a1", 1) || s.HoldsLease("a2", 1) || !s.HoldsLease("a2", 2) {
		t.Fatal("only the latest attempt with the latest token holds the lease")
	}
}

func TestRenewLeaseOnlyForTheHolder(t *testing.T) {
	s := reconcileStep(t, StepQueued, nil)
	_ = s.Lease("w1", "a1", t0.Add(time.Minute))
	later := t0.Add(2 * time.Minute)
	if err := s.RenewLease("a1", 1, later); err != nil || !s.CurrentLease.ExpiresAt.Equal(later) {
		t.Fatalf("renew: %v %+v", err, s.CurrentLease)
	}
	for _, c := range []struct {
		attempt string
		token   uint64
	}{{"a0", 1}, {"a1", 0}, {"a1", 2}} {
		if err := s.RenewLease(c.attempt, c.token, later); !errors.Is(err, ErrStaleLease) {
			t.Errorf("%s/%d: got %v, want ErrStaleLease", c.attempt, c.token, err)
		}
	}
	_ = s.Start(t0)
	_ = s.Finish(t0, StepSucceeded, nil)
	if err := s.RenewLease("a1", 1, later); !errors.Is(err, ErrStaleLease) {
		t.Fatalf("finished step renewed: %v", err)
	}
	if s.AttemptID != "a1" {
		t.Fatal("attempt id must survive the end of the step so a late duplicate can be recognised")
	}
}

func TestReleaseLeaseNeedsReasonAndLeasedStep(t *testing.T) {
	s := reconcileStep(t, StepQueued, nil)
	if err := s.ReleaseLease("why"); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("queued: got %v", err)
	}
	_ = s.Lease("w1", "a1", t0)
	if err := s.ReleaseLease(""); !errors.Is(err, ErrReasonRequired) {
		t.Fatalf("no reason: got %v", err)
	}
	if err := s.Lease("w1", "", t0); !errors.Is(err, ErrInvalidTransition) && !errors.Is(err, ErrInvalid) {
		t.Fatalf("empty attempt: got %v", err)
	}
}
