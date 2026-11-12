package coordtest

import (
	"context"
	"fmt"
	"reflect"
	"sync/atomic"
	"testing"

	"github.com/rluders/amaru/services/platform/internal/coordinator"
	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
)

// limited is a coordinator with the given limits, counters for ids, and the fixed clock.
func limited(store ports.ExecutionStore, opts ...coordinator.Option) *coordinator.Coordinator {
	var attempts, execs atomic.Int64
	return coordinator.New(store, clock, append([]coordinator.Option{
		Permissive,
		coordinator.WithLeasePeriod(leasePeriod),
		coordinator.WithAttemptIDs(func() string { return fmt.Sprintf("att-%d", attempts.Add(1)) }),
		coordinator.WithExecutionIDs(func() string { return fmt.Sprintf("e-%d", execs.Add(1)) }),
	}, opts...)...)
}

func submit(t *testing.T, c *coordinator.Coordinator, key, project, env string, n int) coordinator.Submitted {
	t.Helper()
	steps := make([]coordinator.StepSpec, n)
	for i := range steps {
		steps[i] = coordinator.StepSpec{Name: fmt.Sprintf("s%d", i+1), Capability: "code.test"}
	}
	got, _, err := c.Submit(context.Background(), coordinator.SubmitRequest{By: Requester, Key: key, ProjectID: project, EnvironmentID: env, Steps: steps})
	if err != nil {
		t.Fatal(err)
	}
	return got
}

// drain returns every Assignment that arrives until the link is quiet.
func (s *session) drain() []ports.Assignment {
	s.t.Helper()
	var out []ports.Assignment
	for {
		ctx, cancel := context.WithTimeout(s.ctx, quiet)
		m, err := s.w.Receive(ctx)
		cancel()
		if err != nil {
			return out
		}
		a, ok := m.(ports.Assignment)
		if !ok {
			s.t.Fatalf("unexpected message: %#v", m)
		}
		out = append(out, a)
	}
}

// queuedWithReason returns the queued Steps of an Execution with the reason each one waits.
func queuedWithReason(t *testing.T, store ports.ExecutionStore, exec string) map[string]string {
	t.Helper()
	_, steps, err := store.Get(context.Background(), exec)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, st := range steps {
		if st.State == domain.StepQueued {
			out[st.ID] = st.Reason
		}
	}
	return out
}

func deferred(t *testing.T, store ports.ExecutionStore, exec string) int {
	t.Helper()
	evs, err := store.Events(context.Background(), exec)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, ev := range evs {
		if ev.Kind == domain.EventStepDeferred {
			n++
		}
	}
	return n
}

// LimitsSuite checks the concurrency limits (FEAT-EXE-006) through the Worker link: the Platform
// leases no Step that would exceed a limit, and a Step that waits is queued with the reason.
func LimitsSuite(t *testing.T, newStore func(t *testing.T) ports.ExecutionStore, nl NewLink) {
	limitLifecycle(t, newStore, nl)
	t.Run("two slots per Project by default; the rest wait with the reason", func(t *testing.T) {
		store := newStore(t)
		c := limited(store)
		got := submit(t, c, "k1", "p1", "", 4)
		s := startSession(t, store, c, nl, "w1")
		s.send(ports.Offer{FreeSlots: 10})
		if as := s.drain(); len(as) != 2 {
			t.Fatalf("%d assignments, want 2", len(as))
		}
		want := map[string]string{got.Execution.ID + ".s3": domain.ReasonLimitProject, got.Execution.ID + ".s4": domain.ReasonLimitProject}
		if w := queuedWithReason(t, store, got.Execution.ID); fmt.Sprint(w) != fmt.Sprint(want) {
			t.Fatalf("waiting %v, want %v", w, want)
		}
		// Another pass does not write the same reason again.
		c.Wake()
		s.expectNothing()
		if n := deferred(t, store, got.Execution.ID); n != 2 {
			t.Fatalf("%d step.deferred events, want 2 (one per waiting Step)", n)
		}
	})

	t.Run("a Project at its limit does not hold up another Project", func(t *testing.T) {
		store := newStore(t)
		c := limited(store)
		submit(t, c, "k1", "p1", "", 4)
		other := submit(t, c, "k2", "p2", "", 1)
		s := startSession(t, store, c, nl, "w1")
		s.send(ports.Offer{FreeSlots: 10})
		as := s.drain()
		if len(as) != 3 {
			t.Fatalf("%d assignments, want 3 (two for p1, one for p2)", len(as))
		}
		if _, st := s.step(other.Execution.ID, other.Steps[0].ID); st.State != domain.StepLeased {
			t.Fatalf("the other Project's Step is %s", st.State)
		}
	})

	t.Run("a finished Step frees its slot for the next one", func(t *testing.T) {
		store := newStore(t)
		c := limited(store)
		got := submit(t, c, "k1", "p1", "", 3)
		s := startSession(t, store, c, nl, "w1")
		s.send(ports.Offer{FreeSlots: 10})
		as := s.drain()
		if len(as) != 2 {
			t.Fatalf("%d assignments, want 2", len(as))
		}
		a := as[0]
		s.send(ports.AcceptAttempt{AttemptID: a.AttemptID, FencingToken: a.Lease.FencingToken})
		s.send(ports.Result{AttemptID: a.AttemptID, FencingToken: a.Lease.FencingToken, Outcome: ports.OutcomeSucceeded})
		var third ports.Assignment
		for range 2 { // the Result is acknowledged, and the freed slot is used
			switch m := s.recv().(type) {
			case ports.Assignment:
				third = m
			case ports.ResultAck:
			default:
				t.Fatalf("unexpected message %#v", m)
			}
		}
		if third.StepID != got.Execution.ID+".s3" {
			t.Fatalf("third assignment %+v", third)
		}
		if w := queuedWithReason(t, store, got.Execution.ID); len(w) != 0 {
			t.Fatalf("still waiting: %v", w)
		}
		if _, st := s.step(got.Execution.ID, third.StepID); st.Reason != "" {
			t.Fatalf("a leased Step still shows the reason %q", st.Reason)
		}
	})

	t.Run("a global limit holds across Projects", func(t *testing.T) {
		store := newStore(t)
		c := limited(store, coordinator.WithLimits(coordinator.Limits{Global: 3}))
		var ids []string
		for i, p := range []string{"p1", "p2", "p3"} {
			ids = append(ids, submit(t, c, fmt.Sprint("k", i), p, "", 2).Execution.ID)
		}
		s := startSession(t, store, c, nl, "w1")
		s.send(ports.Offer{FreeSlots: 10})
		if as := s.drain(); len(as) != 3 {
			t.Fatalf("%d assignments, want 3", len(as))
		}
		n := 0
		for _, id := range ids {
			for _, reason := range queuedWithReason(t, store, id) {
				if reason != domain.ReasonLimitGlobal {
					t.Fatalf("reason %q, want %q", reason, domain.ReasonLimitGlobal)
				}
				n++
			}
		}
		if n != 3 {
			t.Fatalf("%d Steps waiting, want 3", n)
		}
	})

	t.Run("an Environment limit holds across Projects", func(t *testing.T) {
		store := newStore(t)
		c := limited(store, coordinator.WithLimits(coordinator.Limits{PerEnvironment: 1}))
		a := submit(t, c, "k1", "p1", "dev", 1)
		b := submit(t, c, "k2", "p2", "dev", 1)
		d := submit(t, c, "k3", "p3", "prod", 1)
		s := startSession(t, store, c, nl, "w1")
		s.send(ports.Offer{FreeSlots: 10})
		if as := s.drain(); len(as) != 2 {
			t.Fatalf("%d assignments, want 2", len(as))
		}
		if w := queuedWithReason(t, store, b.Execution.ID); w[b.Steps[0].ID] != domain.ReasonLimitEnvironment {
			t.Fatalf("second dev Step: %v", w)
		}
		for _, e := range []coordinator.Submitted{a, d} {
			if _, st := s.step(e.Execution.ID, e.Steps[0].ID); st.State != domain.StepLeased {
				t.Fatalf("%s is %s", st.ID, st.State)
			}
		}
	})

	t.Run("empty limits remove every cap", func(t *testing.T) {
		store := newStore(t)
		c := limited(store, coordinator.WithLimits(coordinator.Limits{}))
		submit(t, c, "k1", "p1", "dev", 5)
		s := startSession(t, store, c, nl, "w1")
		s.send(ports.Offer{FreeSlots: 10})
		if as := s.drain(); len(as) != 5 {
			t.Fatalf("%d assignments, want 5", len(as))
		}
	})

	t.Run("two Workers together never exceed a limit", func(t *testing.T) {
		store := newStore(t)
		c := limited(store)
		got := submit(t, c, "k1", "p1", "", 8)
		s1 := startSession(t, store, c, nl, "w1")
		s2 := startSession(t, store, c, nl, "w2")
		s1.send(ports.Offer{FreeSlots: 8})
		s2.send(ports.Offer{FreeSlots: 8})
		total := len(s1.drain()) + len(s2.drain())
		if total != 2 {
			t.Fatalf("%d assignments across two Workers, want 2", total)
		}
		active, err := store.Active(context.Background())
		if err != nil || len(active) != 2 {
			t.Fatalf("active %v, %v", active, err)
		}
		if w := queuedWithReason(t, store, got.Execution.ID); len(w) != 6 {
			t.Fatalf("%d Steps waiting, want 6", len(w))
		}
	})

	t.Run("a failed result and a refused attempt free their slots", func(t *testing.T) {
		store := newStore(t)
		c := limited(store)
		submit(t, c, "k1", "p1", "", 4)
		s := startSession(t, store, c, nl, "w1")
		s.send(ports.Offer{FreeSlots: 10})
		as := s.drain()
		if len(as) != 2 {
			t.Fatalf("%d assignments, want 2", len(as))
		}
		got := submit(t, c, "healthy", "p1", "", 3)
		s.send(ports.AcceptAttempt{AttemptID: as[0].AttemptID, FencingToken: as[0].Lease.FencingToken})
		s.send(ports.Result{AttemptID: as[0].AttemptID, FencingToken: as[0].Lease.FencingToken, Outcome: ports.OutcomeFailed, ErrorCode: "boom"})
		s.send(ports.RejectAttempt{AttemptID: as[1].AttemptID})
		s.send(ports.Offer{FreeSlots: 10}) // a refusing Worker offers again
		deadline := 0
		var more []ports.Assignment
		for len(more) < 2 && deadline < 20 {
			deadline++
			ctx, cancel := context.WithTimeout(s.ctx, wait/10)
			m, err := s.w.Receive(ctx)
			cancel()
			if a, ok := m.(ports.Assignment); err == nil && ok {
				more = append(more, a)
			}
		}
		if len(more) != 2 {
			t.Fatalf("%d assignments after both slots were freed, want 2", len(more))
		}
		// Failure drains its Execution; three healthy Steps compete for the freed slots.
		if w := queuedWithReason(t, store, got.Execution.ID); len(w) != 1 {
			t.Fatalf("waiting %v, want exactly one Step", w)
		}
	})
}

// Check the same slot lifecycle in each scope, including competing Serve loops.
func limitLifecycle(t *testing.T, newStore func(*testing.T) ports.ExecutionStore, nl NewLink) {
	for _, scope := range []struct {
		name   string
		limits coordinator.Limits
		reason string
	}{
		{"global", coordinator.Limits{Global: 1, PerProject: 2, PerEnvironment: 3}, domain.ReasonLimitGlobal},
		{"project", coordinator.Limits{Global: 2, PerProject: 1, PerEnvironment: 3}, domain.ReasonLimitProject},
		{"environment", coordinator.Limits{Global: 3, PerProject: 2, PerEnvironment: 1}, domain.ReasonLimitEnvironment},
	} {
		t.Run(scope.name, func(t *testing.T) {
			t.Run("simultaneous offers cannot take the same slot", func(t *testing.T) {
				store := newStore(t)
				c := limited(store, coordinator.WithLimits(scope.limits))
				got := submit(t, c, "work", "p1", "dev", 4)
				sessions := []*session{startSession(t, store, c, nl, "w1"), startSession(t, store, c, nl, "w2")}
				for _, s := range sessions {
					s.send(ports.Offer{FreeSlots: 0})
					s.expectNothing()
				}
				start := make(chan struct{})
				errs := make(chan error, len(sessions))
				for _, s := range sessions {
					go func() {
						<-start
						errs <- s.w.Send(s.ctx, ports.Offer{FreeSlots: 4})
					}()
				}
				close(start)
				for range sessions {
					if err := <-errs; err != nil {
						t.Fatal(err)
					}
				}
				if n := len(sessions[0].drain()) + len(sessions[1].drain()); n != 1 {
					t.Fatalf("published %d assignments, want 1", n)
				}
				assertLimitWaiting(t, store, got.Execution.ID, 3, scope.reason)
				assertActiveSlots(t, store, 1)
			})

			for _, outcome := range []ports.ResultOutcome{ports.OutcomeSucceeded, ports.OutcomeFailed, ports.OutcomeCancelled} {
				t.Run(fmt.Sprintf("result %d releases once", outcome), func(t *testing.T) {
					store := newStore(t)
					c := limited(store, coordinator.WithLimits(scope.limits))
					first := submit(t, c, "first", "p1", "dev", 1)
					s := startSession(t, store, c, nl, "w1")
					s.send(ports.Offer{FreeSlots: 10})
					a := s.assignment()
					s.send(ports.AcceptAttempt{AttemptID: a.AttemptID, FencingToken: a.Lease.FencingToken})
					waitFor(t, "running Step", func() bool { _, st := s.step(a.ExecutionID, a.StepID); return st.State == domain.StepRunning })
					next := submit(t, c, "next", "p1", "dev", 2)
					s.expectNothing()
					assertLimitWaiting(t, store, next.Execution.ID, 2, scope.reason)
					if outcome == ports.OutcomeCancelled {
						if _, _, err := c.Cancel(s.ctx, coordinator.CancelRequest{ActorID: "tester", Key: "cancel", ExecutionID: first.Execution.ID, By: human}); err != nil {
							t.Fatal(err)
						}
						if m, ok := s.recv().(ports.Cancel); !ok || m.AttemptID != a.AttemptID {
							t.Fatalf("cancel message: %#v", m)
						}
						s.expectNothing() // a request alone must not release a held slot
						assertActiveSlots(t, store, 1)
					}
					result := ports.Result{AttemptID: a.AttemptID, FencingToken: a.Lease.FencingToken, Outcome: outcome}
					s.send(result)
					assertLimitAck(t, s, ports.ResultAccepted)
					if replacement := s.assignment(); replacement.ExecutionID != next.Execution.ID {
						t.Fatalf("replacement: %+v", replacement)
					}
					before := s.events(first.Execution.ID)
					s.send(result)
					assertLimitAck(t, s, ports.ResultDuplicate)
					s.expectNothing()
					assertActiveSlots(t, store, 1)
					assertLimitWaiting(t, store, next.Execution.ID, 1, scope.reason)
					if !reflect.DeepEqual(before, s.events(first.Execution.ID)) {
						t.Fatal("duplicate result changed Events")
					}
				})
			}

			t.Run("startup reconciliation frees a lost Lease once", func(t *testing.T) {
				store := newStore(t)
				var lost ports.Assignment
				var next coordinator.Submitted
				t.Run("before restart", func(t *testing.T) {
					c := limited(store, coordinator.WithLimits(scope.limits))
					submit(t, c, "first", "p1", "dev", 1)
					s := startSession(t, store, c, nl, "old-worker")
					s.send(ports.Offer{FreeSlots: 10})
					lost = s.assignment()
					s.send(ports.AcceptAttempt{AttemptID: lost.AttemptID, FencingToken: lost.Lease.FencingToken})
					waitFor(t, "running Step", func() bool { _, st := s.step(lost.ExecutionID, lost.StepID); return st.State == domain.StepRunning })
					next = submit(t, c, "next", "p1", "dev", 2)
					s.expectNothing()
					assertLimitWaiting(t, store, next.Execution.ID, 2, scope.reason)
				}) // old Serve has stopped before reconciliation
				c := coordinator.New(store, clock, Permissive, coordinator.WithLimits(scope.limits))
				report, err := c.Reconcile(context.Background())
				if err != nil || report.Interrupted != 1 {
					t.Fatalf("reconcile %+v: %v", report, err)
				}
				assertActiveSlots(t, store, 0)
				s := startSession(t, store, c, nl, "new-worker")
				before := s.events(lost.ExecutionID)
				if _, err := c.Reconcile(context.Background()); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(before, s.events(lost.ExecutionID)) {
					t.Fatal("repeated reconciliation changed Events")
				}
				s.send(ports.Offer{FreeSlots: 10})
				if a := s.assignment(); a.ExecutionID != next.Execution.ID {
					t.Fatalf("replacement: %+v", a)
				}
				s.expectNothing()
				assertActiveSlots(t, store, 1)
				assertLimitWaiting(t, store, next.Execution.ID, 1, scope.reason)
				_, st := s.step(lost.ExecutionID, lost.StepID)
				if st.State != domain.StepInterrupted || st.CurrentLease != nil {
					t.Fatalf("lost Step: %+v", st)
				}
			})
		})
	}
}

func assertActiveSlots(t *testing.T, store ports.ExecutionStore, want int) {
	t.Helper()
	active, err := store.Active(context.Background())
	if err != nil || len(active) != want {
		t.Fatalf("active slots %d, want %d: %v", len(active), want, err)
	}
}

func assertLimitWaiting(t *testing.T, store ports.ExecutionStore, exec string, want int, reason string) {
	t.Helper()
	waiting := queuedWithReason(t, store, exec)
	if len(waiting) != want {
		t.Fatalf("queued Steps %v, want %d", waiting, want)
	}
	for id, got := range waiting {
		if got != reason {
			t.Fatalf("%s waits for %q, want %q", id, got, reason)
		}
	}
}

func assertLimitAck(t *testing.T, s *session, want ports.ResultDisposition) {
	t.Helper()
	if ack, ok := s.recv().(ports.ResultAck); !ok || ack.Disposition != want {
		t.Fatalf("ack %+v, want %v", ack, want)
	}
}
