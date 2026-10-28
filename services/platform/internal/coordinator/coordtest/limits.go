package coordtest

import (
	"context"
	"fmt"
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
	got, _, err := c.Submit(context.Background(), coordinator.SubmitRequest{Key: key, ProjectID: project, EnvironmentID: env, Steps: steps})
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
		got := submit(t, c, "k1", "p1", "", 4)
		s := startSession(t, store, c, nl, "w1")
		s.send(ports.Offer{FreeSlots: 10})
		as := s.drain()
		if len(as) != 2 {
			t.Fatalf("%d assignments, want 2", len(as))
		}
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
		// Four Steps: one failed, the refused one is queued again, so three compete for two slots.
		if w := queuedWithReason(t, store, got.Execution.ID); len(w) != 1 {
			t.Fatalf("waiting %v, want exactly one Step", w)
		}
	})
}
