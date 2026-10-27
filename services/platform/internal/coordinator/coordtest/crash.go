// Package coordtest holds the crash-injection suite for the Execution coordinator. The in-memory
// fake and the PostgreSQL adapter run the same suite, so the restart rule is checked against both
// (ADR-0042, WP-M3-03 acceptance).
//
// A "crash" here is a restart: the process state is thrown away and a new Coordinator is built over
// a store that holds only what was committed. The suite does not kill an OS process; killing the
// real Platform process is validated separately (see docs/engineering/validation.md).
package coordtest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rluders/amaru/services/platform/internal/coordinator"
	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
)

var (
	t0  = time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	now = t0.Add(time.Hour) // the restart happens here
)

func clock() time.Time { return now }

// Restart returns a Coordinator over a store that has only the committed data.
type Restart func(t *testing.T, s ports.ExecutionStore) ports.ExecutionStore

// step drives one Step into a state through documented transitions, committing after each move,
// the way the coordinator will: a transition is durable before the next one starts.
type scenario struct {
	name string
	// path moves the Step (and Execution) from pending/queued to the state under test.
	path func(e *domain.Execution, s *domain.StepExecution) error
	// want
	step   domain.StepState
	action domain.RestartAction // "" for a Step that must not change
	reason string
	exec   domain.ExecState
}

func lease(valid bool) func(*domain.StepExecution) error {
	return func(s *domain.StepExecution) error {
		exp := now.Add(time.Minute)
		if !valid {
			exp = now.Add(-time.Minute)
		}
		return s.Lease("worker-1", exp)
	}
}

func chain(fs ...func(*domain.StepExecution) error) func(*domain.StepExecution) error {
	return func(s *domain.StepExecution) error {
		for _, f := range fs {
			if err := f(s); err != nil {
				return err
			}
		}
		return nil
	}
}

func plan(s *domain.StepExecution) error  { return s.Plan() }
func start(s *domain.StepExecution) error { return s.Start(t0) }

func running(opts ...func(*domain.StepExecution)) func(*domain.Execution, *domain.StepExecution) error {
	return func(e *domain.Execution, s *domain.StepExecution) error {
		for _, o := range opts {
			o(s)
		}
		if err := chain(plan, lease(true), start)(s); err != nil {
			return err
		}
		return e.Start(t0)
	}
}

func waiting(f func(*domain.StepExecution) error) func(*domain.Execution, *domain.StepExecution) error {
	return func(e *domain.Execution, s *domain.StepExecution) error {
		if err := e.Start(t0); err != nil {
			return err
		}
		if err := f(s); err != nil {
			return err
		}
		return e.Wait()
	}
}

func idempotent(s *domain.StepExecution) { s.Idempotent = true }
func resumable(s *domain.StepExecution) {
	s.RemoteJob = &domain.RemoteJob{ID: "job-1", Resumable: true}
}
func plainJob(s *domain.StepExecution) { s.RemoteJob = &domain.RemoteJob{ID: "job-1"} }

func scenarios() []scenario {
	return []scenario{
		// Rule 1: nothing ran, nothing is lost.
		{"pending", func(e *domain.Execution, s *domain.StepExecution) error { return e.Start(t0) }, domain.StepPending, "", "", domain.ExecRunning},
		{"awaiting_approval survives (Approval wait)", waiting(func(s *domain.StepExecution) error { return s.RequireApproval() }), domain.StepAwaitingApproval, "", "", domain.ExecWaiting},
		{"queued", func(e *domain.Execution, s *domain.StepExecution) error { _ = e.Start(t0); return s.Plan() }, domain.StepQueued, "", "", domain.ExecRunning},
		{"leased, lease still valid", func(e *domain.Execution, s *domain.StepExecution) error {
			_ = e.Start(t0)
			return chain(plan, lease(true))(s)
		}, domain.StepLeased, "", "", domain.ExecRunning},
		// A Lease that ended before the Step started: nothing ran, so it is re-leased whatever its kind.
		{"leased, lease ended", func(e *domain.Execution, s *domain.StepExecution) error {
			_ = e.Start(t0)
			return chain(plan, lease(false))(s)
		}, domain.StepQueued, domain.RestartRetried, domain.ReasonLeaseExpired, domain.ExecRunning},
		// A person owns these.
		{"interrupted stays interrupted", waiting(chain(plan, lease(true), start, func(s *domain.StepExecution) error {
			return s.Recover(t0, false, domain.ReasonPlatformRestarted)
		})), domain.StepInterrupted, "", "", domain.ExecWaiting},
		{"timed_out stays timed_out", waiting(chain(plan, lease(true), start, func(s *domain.StepExecution) error { return s.TimeOut(t0) })),
			domain.StepTimedOut, "", "", domain.ExecWaiting},

		// Running: rules 2, 3 and 4.
		{"running, resumable remote job: resumed", running(resumable), domain.StepQueued, domain.RestartResumed, domain.ReasonPlatformRestarted, domain.ExecRunning},
		{"running, idempotent: retried on a new lease", running(idempotent), domain.StepQueued, domain.RestartRetried, domain.ReasonPlatformRestarted, domain.ExecRunning},
		{"running, remote job not resumable: interrupted", running(plainJob), domain.StepInterrupted, domain.RestartInterrupted, domain.ReasonPlatformRestarted, domain.ExecWaiting},
		{"running, neither: interrupted", running(), domain.StepInterrupted, domain.RestartInterrupted, domain.ReasonPlatformRestarted, domain.ExecWaiting},

		// Finished Steps are left alone.
		{"succeeded", func(e *domain.Execution, s *domain.StepExecution) error {
			_ = e.Start(t0)
			return chain(plan, lease(true), start, func(s *domain.StepExecution) error { return s.Finish(t0, domain.StepSucceeded, nil) })(s)
		}, domain.StepSucceeded, "", "", domain.ExecRunning},
		{"failed", func(e *domain.Execution, s *domain.StepExecution) error {
			_ = e.Start(t0)
			return chain(plan, lease(true), start, func(s *domain.StepExecution) error {
				return s.Finish(t0, domain.StepFailed, &domain.ExecError{Code: "x", Message: "y"})
			})(s)
		}, domain.StepFailed, "", "", domain.ExecRunning},
		{"cancelled", func(e *domain.Execution, s *domain.StepExecution) error {
			_ = e.Start(t0)
			return chain(plan, func(s *domain.StepExecution) error { return s.Cancel(t0) })(s)
		},
			domain.StepCancelled, "", "", domain.ExecRunning},
		{"skipped", func(e *domain.Execution, s *domain.StepExecution) error {
			_ = e.Start(t0)
			return s.Skip(t0, "not needed")
		}, domain.StepSkipped, "", "", domain.ExecRunning},
		{"refused", func(e *domain.Execution, s *domain.StepExecution) error {
			_ = e.Start(t0)
			return s.Refuse(t0, "policy")
		}, domain.StepRefused, "", "", domain.ExecRunning},
	}
}

func named(t *testing.T, name string) scenario {
	t.Helper()
	for _, sc := range scenarios() {
		if sc.name == name {
			return sc
		}
	}
	t.Fatalf("no scenario %q", name)
	return scenario{}
}

// seed stores the scenario as the Platform would have left it before the crash: every move was
// committed on its own, in order.
func seed(t *testing.T, s ports.ExecutionStore, sc scenario) (domain.Execution, domain.StepExecution) {
	t.Helper()
	ctx := context.Background()
	e := domain.Execution{ID: "e1", State: domain.ExecQueued, CreatedAt: t0}
	st := domain.StepExecution{ID: "s1", ExecutionID: "e1", State: domain.StepPending}
	e, steps, err := s.Create(ctx, e, []domain.StepExecution{st})
	if err != nil {
		t.Fatal(err)
	}
	st = steps[0]
	// Apply the path, then persist. The path mutates both values; commit once at the end is not
	// enough to prove per-transition durability, so the porttest suite covers that; here the
	// state before the crash is what matters.
	if err := sc.path(&e, &st); err != nil {
		t.Fatalf("seed %s: %v", sc.name, err)
	}
	out, err := s.Commit(ctx, ports.ExecutionChange{Execution: &e, Steps: []domain.StepExecution{st}})
	if err != nil {
		t.Fatal(err)
	}
	return *out.Execution, out.Steps[0]
}

// CrashInjection runs the restart matrix: one Step in each state (and each kind of running Step)
// is committed, the process "crashes", and Reconcile must apply ADR-0042 exactly.
func CrashInjection(t *testing.T, newStore func(t *testing.T) ports.ExecutionStore, restart Restart) {
	ctx := context.Background()

	for _, sc := range scenarios() {
		t.Run(sc.name, func(t *testing.T) {
			s := newStore(t)
			e0, st0 := seed(t, s, sc)

			s = restart(t, s) // the crash: only committed data remains
			rep, err := coordinator.New(s, clock).Reconcile(ctx)
			if err != nil {
				t.Fatal(err)
			}
			e, steps, err := s.Get(ctx, "e1")
			if err != nil {
				t.Fatal(err)
			}
			st := steps[0]
			if st.State != sc.step || e.State != sc.exec {
				t.Fatalf("got step %s / execution %s, want %s / %s", st.State, e.State, sc.step, sc.exec)
			}
			evs, _ := s.Events(ctx, "e1")
			if sc.action == "" {
				// Untouched: same versions, no event, no report entry.
				if st.Version != st0.Version || e.Version != e0.Version || len(evs) != 0 || rep.Retried+rep.Resumed+rep.Interrupted != 0 {
					t.Fatalf("a Step that must be left alone changed: step v%d→v%d, exec v%d→v%d, events %v, report %+v",
						st0.Version, st.Version, e0.Version, e.Version, evs, rep)
				}
				return
			}
			if st.Reason != sc.reason {
				t.Fatalf("reason %q, want %q", st.Reason, sc.reason)
			}
			if len(evs) == 0 || evs[0].StepID != "s1" || evs[0].Reason != sc.reason || !evs[0].At.Equal(now) {
				t.Fatalf("no restart event naming the Step and the reason: %+v", evs)
			}
			for i, ev := range evs {
				if ev.Seq != int64(i+1) {
					t.Fatalf("event log has a gap: %+v", evs)
				}
			}
			if st.State == domain.StepInterrupted && (st.EndedAt == nil || st.CurrentLease != nil) {
				t.Fatalf("interrupted Step keeps a lease or has no end: %+v", st)
			}
			if st.State == domain.StepQueued && st.CurrentLease != nil {
				t.Fatalf("re-queued Step still holds a lease: %+v", st.CurrentLease)
			}
			if sc.action == domain.RestartResumed && (st.RemoteJob == nil || st.RemoteJob.ID != "job-1") {
				t.Fatalf("resumed Step lost its remote job: %+v", st.RemoteJob)
			}
		})
	}

	// An interruption is not a failure: nothing is retried, nothing spends an attempt, the
	// Execution waits (ADR-0042 amendment 2026-10-27).
	t.Run("an interrupted Step leaves its Execution waiting, not failed", func(t *testing.T) {
		s := newStore(t)
		seed(t, s, named(t, "running, neither: interrupted"))
		s = restart(t, s)
		rep, err := coordinator.New(s, clock).Reconcile(ctx)
		if err != nil {
			t.Fatal(err)
		}
		e, _, _ := s.Get(ctx, "e1")
		if e.State != domain.ExecWaiting || rep.Waiting != 1 || rep.Interrupted != 1 {
			t.Fatalf("execution %s, report %+v", e.State, rep)
		}
		evs, _ := s.Events(ctx, "e1")
		if len(evs) != 2 || evs[1].Kind != domain.EventExecWaiting {
			t.Fatalf("events: %+v", evs)
		}
	})

	// A second restart changes nothing: the first already settled every Step.
	t.Run("reconcile twice is the same as once", func(t *testing.T) {
		s := newStore(t)
		seed(t, s, named(t, "running, idempotent: retried on a new lease"))
		s = restart(t, s)
		if _, err := coordinator.New(s, clock).Reconcile(ctx); err != nil {
			t.Fatal(err)
		}
		e1, steps1, _ := s.Get(ctx, "e1")
		ev1, _ := s.Events(ctx, "e1")
		s = restart(t, s)
		rep, err := coordinator.New(s, clock).Reconcile(ctx)
		if err != nil {
			t.Fatal(err)
		}
		e2, steps2, _ := s.Get(ctx, "e1")
		ev2, _ := s.Events(ctx, "e1")
		if e1.Version != e2.Version || steps1[0].Version != steps2[0].Version || len(ev1) != len(ev2) || rep.Retried+rep.Resumed+rep.Interrupted != 0 {
			t.Fatalf("second reconcile changed state: exec v%d→v%d, step v%d→v%d, events %d→%d, report %+v",
				e1.Version, e2.Version, steps1[0].Version, steps2[0].Version, len(ev1), len(ev2), rep)
		}
	})

	// The crash can come during Reconcile itself.
	t.Run("crash before the write is durable: nothing changed, next run converges", func(t *testing.T) {
		s := newStore(t)
		seed(t, s, named(t, "running, idempotent: retried on a new lease"))
		failing := &faultyStore{ExecutionStore: s, failBefore: 1}
		if _, err := coordinator.New(failing, clock).Reconcile(ctx); err == nil {
			t.Fatal("expected the injected crash to surface")
		}
		s = restart(t, s)
		e, steps, _ := s.Get(ctx, "e1")
		if steps[0].State != domain.StepRunning || e.State != domain.ExecRunning {
			t.Fatalf("a write that never became durable changed state: %s / %s", steps[0].State, e.State)
		}
		if evs, _ := s.Events(ctx, "e1"); len(evs) != 0 {
			t.Fatalf("events written without the change: %+v", evs)
		}
		if _, err := coordinator.New(s, clock).Reconcile(ctx); err != nil {
			t.Fatal(err)
		}
		_, steps, _ = s.Get(ctx, "e1")
		if steps[0].State != domain.StepQueued {
			t.Fatalf("next run did not converge: %s", steps[0].State)
		}
	})

	t.Run("crash after the write is durable but before the ack: no duplicate work", func(t *testing.T) {
		s := newStore(t)
		seed(t, s, named(t, "running, idempotent: retried on a new lease"))
		failing := &faultyStore{ExecutionStore: s, failAfter: 1}
		if _, err := coordinator.New(failing, clock).Reconcile(ctx); err == nil {
			t.Fatal("expected the injected crash to surface")
		}
		s = restart(t, s)
		_, steps, _ := s.Get(ctx, "e1")
		if steps[0].State != domain.StepQueued {
			t.Fatalf("durable write lost: %s", steps[0].State)
		}
		rep, err := coordinator.New(s, clock).Reconcile(ctx)
		if err != nil {
			t.Fatal(err)
		}
		evs, _ := s.Events(ctx, "e1")
		if len(evs) != 1 || rep.Retried != 0 {
			t.Fatalf("re-run repeated the recovery: %d events, report %+v", len(evs), rep)
		}
	})

	t.Run("several Executions are each settled", func(t *testing.T) {
		s := newStore(t)
		for _, id := range []string{"a", "b"} {
			e := domain.Execution{ID: id, State: domain.ExecQueued, CreatedAt: t0}
			st := domain.StepExecution{ID: id + "-s", ExecutionID: id, State: domain.StepPending}
			e, steps, err := s.Create(ctx, e, []domain.StepExecution{st})
			if err != nil {
				t.Fatal(err)
			}
			st = steps[0]
			if err := running()(&e, &st); err != nil {
				t.Fatal(err)
			}
			if _, err := s.Commit(ctx, ports.ExecutionChange{Execution: &e, Steps: []domain.StepExecution{st}}); err != nil {
				t.Fatal(err)
			}
		}
		s = restart(t, s)
		rep, err := coordinator.New(s, clock).Reconcile(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if rep.Executions != 2 || rep.Interrupted != 2 || rep.Waiting != 2 {
			t.Fatalf("report %+v", rep)
		}
	})
}

// faultyStore injects a crash around the Nth Commit: before it is durable (the call errors and
// nothing is written) or after (the write is durable and the caller is told it failed).
type faultyStore struct {
	ports.ExecutionStore
	failBefore, failAfter int
	commits               int
}

var errCrash = errors.New("injected crash")

func (f *faultyStore) Commit(ctx context.Context, c ports.ExecutionChange) (ports.ExecutionChange, error) {
	f.commits++
	if f.commits == f.failBefore {
		return ports.ExecutionChange{}, errCrash
	}
	out, err := f.ExecutionStore.Commit(ctx, c)
	if err == nil && f.commits == f.failAfter {
		return ports.ExecutionChange{}, errCrash
	}
	return out, err
}
