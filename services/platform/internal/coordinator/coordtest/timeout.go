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

// TimeoutSuite checks the Platform side of per-Step timeouts (FEAT-WFL-004) through the Worker
// link: every Assignment carries a Deadline (the default for its kind or the Step's override),
// an override above the Organization maximum is refused, and a Result that reports "timeout"
// ends the Step as timed_out. Killing the process tree is the Worker's part and is not checked here.
func TimeoutSuite(t *testing.T, newStore func(t *testing.T) ports.ExecutionStore, nl NewLink, reopen Restart) {
	t.Helper()
	if reopen == nil {
		reopen = func(_ *testing.T, s ports.ExecutionStore) ports.ExecutionStore { return s }
	}
	ctx := context.Background()
	two := func(t *testing.T, store ports.ExecutionStore, c *coordinator.Coordinator) (coordinator.Submitted, error) {
		t.Helper()
		got, _, err := c.Submit(ctx, coordinator.SubmitRequest{Key: "k1", ProjectID: "p1", Steps: []coordinator.StepSpec{
			{Name: "default", Capability: "code.test"},
			{Name: "short", Capability: "code.test", TimeoutSeconds: 90},
		}})
		return got, err
	}

	t.Run("the Assignment carries the default Deadline or the override", func(t *testing.T) {
		store := newStore(t)
		c := limited(store, coordinator.WithTimeouts(domain.Timeouts{Agent: time.Hour, Tool: 10 * time.Minute, Max: 2 * time.Hour}))
		got, err := two(t, store, c)
		if err != nil {
			t.Fatal(err)
		}
		s := startSession(t, store, c, nl, "w1")
		s.send(ports.Offer{FreeSlots: 10})
		want := map[string]time.Duration{got.Steps[0].ID: 10 * time.Minute, got.Steps[1].ID: 90 * time.Second}
		as := s.drain()
		if len(as) != 2 {
			t.Fatalf("%d assignments", len(as))
		}
		for _, a := range as {
			if d := a.Deadline.Sub(clock()); d != want[a.StepID] {
				t.Errorf("%s: deadline in %s, want %s", a.StepID, d, want[a.StepID])
			}
		}
	})

	t.Run("the override is stored with the Step", func(t *testing.T) {
		store := newStore(t)
		got, err := two(t, store, limited(store))
		if err != nil {
			t.Fatal(err)
		}
		_, steps, err := reopen(t, store).Get(ctx, got.Execution.ID)
		if err != nil || steps[0].TimeoutSeconds != 0 || steps[1].TimeoutSeconds != 90 {
			t.Fatalf("stored %+v, %v", steps, err)
		}
	})

	t.Run("an override above the Organization maximum is refused and creates nothing", func(t *testing.T) {
		store := newStore(t)
		c := limited(store, coordinator.WithTimeouts(domain.Timeouts{Agent: time.Hour, Tool: 10 * time.Minute, Max: time.Hour}))
		for name, secs := range map[string]int{"above the maximum": 3601, "negative": -1} {
			_, _, err := c.Submit(ctx, coordinator.SubmitRequest{Key: "k-" + name, ProjectID: "p1", Steps: []coordinator.StepSpec{{Name: "a", Capability: "code.test", TimeoutSeconds: secs}}})
			if !errors.Is(err, domain.ErrInvalid) {
				t.Fatalf("%s: got %v, want ErrInvalid", name, err)
			}
		}
		_, _, err := c.Submit(ctx, coordinator.SubmitRequest{Key: "ok", ProjectID: "p1", Steps: []coordinator.StepSpec{{Name: "a", Capability: "code.test", TimeoutSeconds: 3600}}})
		if err != nil {
			t.Fatalf("exactly the maximum must pass: %v", err)
		}
		if open, _ := store.Open(ctx); len(open) != 1 {
			t.Fatalf("%d Executions, want 1", len(open))
		}
	})

	t.Run("a Result with code timeout ends the Step as timed_out, and it is not run again", func(t *testing.T) {
		store := newStore(t)
		c := limited(store)
		got, _, err := c.Submit(ctx, coordinator.SubmitRequest{Key: "k1", ProjectID: "p1", Steps: []coordinator.StepSpec{{Name: "a", Capability: "code.test", Idempotent: true}}})
		if err != nil {
			t.Fatal(err)
		}
		s := startSession(t, store, c, nl, "w1")
		s.send(ports.Offer{FreeSlots: 1})
		a := s.assignment()
		s.send(ports.AcceptAttempt{AttemptID: a.AttemptID, FencingToken: a.Lease.FencingToken})
		s.send(ports.Result{AttemptID: a.AttemptID, FencingToken: a.Lease.FencingToken, Outcome: ports.OutcomeFailed, ErrorCode: "timeout", ErrorDetail: "ran past its deadline"})
		if ack, ok := s.recv().(ports.ResultAck); !ok || ack.Disposition != ports.ResultAccepted {
			t.Fatalf("ack %+v", ack)
		}
		_, st := s.step(got.Execution.ID, got.Steps[0].ID)
		if st.State != domain.StepTimedOut || st.Reason != domain.ReasonStepTimeout || st.Error == nil || st.Error.Code != domain.CodeTimeout || st.EndedAt == nil {
			t.Fatalf("step %+v", st)
		}
		var kinds []string
		for _, ev := range s.events(got.Execution.ID) {
			kinds = append(kinds, ev.Kind)
		}
		if kinds[len(kinds)-1] != domain.EventStepTimedOut {
			t.Fatalf("events %v", kinds)
		}
		// Even an idempotent Step is not offered again: a person resumes or cancels it.
		s.send(ports.Offer{FreeSlots: 1})
		s.expectNothing()
		// A restart keeps it as it is.
		if _, st, err := stepOf(ctx, reopen(t, store), got.Execution.ID); err != nil || st.State != domain.StepTimedOut {
			t.Fatalf("after reopen: %v, %v", st.State, err)
		}
	})
}

func stepOf(ctx context.Context, s ports.ExecutionStore, exec string) (domain.Execution, domain.StepExecution, error) {
	e, steps, err := s.Get(ctx, exec)
	if err != nil || len(steps) == 0 {
		return e, domain.StepExecution{}, err
	}
	return e, steps[0], nil
}
