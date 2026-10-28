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

var human = domain.Actor{Kind: domain.ActorHuman}

// CancelSuite checks cancellation (FEAT-EXE-003) through the Worker link: Steps nobody holds end
// at once, a Worker is told to stop the Steps it holds and the Execution ends when the last one
// has, the command is idempotent, and a restart settles a cancel whose Worker is gone. Ending the
// process tree is the Worker's part and is checked in internal/worker/osproc.
func CancelSuite(t *testing.T, newStore func(t *testing.T) ports.ExecutionStore, nl NewLink, reopen Restart) {
	t.Helper()
	if reopen == nil {
		reopen = func(_ *testing.T, s ports.ExecutionStore) ports.ExecutionStore { return s }
	}
	ctx := context.Background()
	cancel := func(c *coordinator.Coordinator, key, exec string) (coordinator.Cancelled, bool, error) {
		return c.Cancel(ctx, coordinator.CancelRequest{Key: key, ExecutionID: exec, Reason: "no longer needed", By: human})
	}
	state := func(t *testing.T, s ports.ExecutionStore, exec string) (domain.Execution, map[string]domain.StepState) {
		t.Helper()
		e, steps, err := s.Get(ctx, exec)
		if err != nil {
			t.Fatal(err)
		}
		m := map[string]domain.StepState{}
		for _, st := range steps {
			m[st.ID] = st.State
		}
		return e, m
	}
	kinds := func(t *testing.T, s ports.ExecutionStore, exec string) []string {
		t.Helper()
		evs, err := s.Events(ctx, exec)
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, ev := range evs {
			out = append(out, ev.Kind)
		}
		return out
	}
	has := func(ks []string, k string) bool {
		for _, x := range ks {
			if x == k {
				return true
			}
		}
		return false
	}

	t.Run("an Execution with nothing held is cancelled at once", func(t *testing.T) {
		store := newStore(t)
		c := limited(store)
		got := submit(t, c, "k1", "p1", "", 3)
		out, replayed, err := cancel(c, "c1", got.Execution.ID)
		if err != nil || replayed || !out.ExecutionCancelled || len(out.Ended) != 3 || len(out.Stopping) != 0 {
			t.Fatalf("%+v replayed=%v err=%v", out, replayed, err)
		}
		e, steps := state(t, store, got.Execution.ID)
		if e.State != domain.ExecCancelled || e.EndedAt == nil {
			t.Fatalf("execution %+v", e)
		}
		for id, s := range steps {
			if s != domain.StepCancelled {
				t.Errorf("%s is %s", id, s)
			}
		}
		ks := kinds(t, store, got.Execution.ID)
		if !has(ks, domain.EventExecCancelRequested) || !has(ks, domain.EventExecCancelled) {
			t.Fatalf("events %v", ks)
		}
	})

	t.Run("the same key replays, another request under it is refused, an ended Execution is refused", func(t *testing.T) {
		store := newStore(t)
		c := limited(store)
		got := submit(t, c, "k1", "p1", "", 1)
		first, _, err := cancel(c, "c1", got.Execution.ID)
		if err != nil {
			t.Fatal(err)
		}
		again, replayed, err := cancel(c, "c1", got.Execution.ID)
		if err != nil || !replayed || again.ExecutionID != first.ExecutionID || !again.ExecutionCancelled {
			t.Fatalf("replay %+v replayed=%v err=%v", again, replayed, err)
		}
		before := kinds(t, store, got.Execution.ID)
		if _, _, err := c.Cancel(ctx, coordinator.CancelRequest{Key: "c1", ExecutionID: got.Execution.ID, Reason: "other", By: human}); !errors.Is(err, domain.ErrKeyReused) {
			t.Fatalf("got %v, want ErrKeyReused", err)
		}
		if _, _, err := cancel(c, "c2", got.Execution.ID); !errors.Is(err, domain.ErrInvalidTransition) {
			t.Fatalf("an ended Execution: got %v, want ErrInvalidTransition", err)
		}
		if after := kinds(t, store, got.Execution.ID); len(after) != len(before) {
			t.Fatalf("a replay or a refusal wrote events: %v -> %v", before, after)
		}
	})

	t.Run("only a human or a named Policy rule may cancel", func(t *testing.T) {
		store := newStore(t)
		c := limited(store)
		got := submit(t, c, "k1", "p1", "", 1)
		for name, by := range map[string]domain.Actor{
			"none": {}, "delivery loop": {Kind: domain.ActorDeliveryLoop, Authority: "x"}, "policy without a rule": {Kind: domain.ActorPolicy},
		} {
			_, _, err := c.Cancel(ctx, coordinator.CancelRequest{Key: "c-" + name, ExecutionID: got.Execution.ID, By: by})
			if !errors.Is(err, domain.ErrNotPermitted) {
				t.Errorf("%s: got %v, want ErrNotPermitted", name, err)
			}
		}
		if e, _ := state(t, store, got.Execution.ID); e.State != domain.ExecQueued {
			t.Fatalf("a refused cancel changed the Execution to %s", e.State)
		}
		if _, _, err := c.Cancel(ctx, coordinator.CancelRequest{Key: "p", ExecutionID: got.Execution.ID, By: domain.Actor{Kind: domain.ActorPolicy, Authority: "rule-1"}}); err != nil {
			t.Fatalf("a named Policy rule may cancel: %v", err)
		}
	})

	t.Run("a Worker is told to stop what it holds, and the Execution ends when it reports", func(t *testing.T) {
		store := newStore(t)
		c := limited(store)
		got := submit(t, c, "k1", "p1", "", 3) // s1 and s2 are leased under the default limit of two; s3 waits
		s := startSession(t, store, c, nl, "w1")
		s.send(ports.Offer{FreeSlots: 10})
		as := s.drain()
		if len(as) != 2 {
			t.Fatalf("%d assignments", len(as))
		}
		// The first attempt is running; the second is leased and not yet accepted.
		s.send(ports.AcceptAttempt{AttemptID: as[0].AttemptID, FencingToken: as[0].Lease.FencingToken})
		waitFor(t, "the first Step to run", func() bool { _, m := state(t, store, got.Execution.ID); return m[as[0].StepID] == domain.StepRunning })

		out, _, err := cancel(c, "c1", got.Execution.ID)
		if err != nil || out.ExecutionCancelled || len(out.Stopping) != 2 || len(out.Ended) != 1 {
			t.Fatalf("%+v %v", out, err)
		}
		e, steps := state(t, store, got.Execution.ID)
		if e.State == domain.ExecCancelled || steps[as[0].StepID] != domain.StepRunning {
			t.Fatalf("held Steps must stay until the Worker reports: %s %v", e.State, steps)
		}
		told := map[string]bool{}
		for range 2 {
			m, ok := s.recv().(ports.Cancel)
			if !ok {
				t.Fatal("expected a Cancel message")
			}
			told[m.AttemptID] = true
			if m.Reason != "no longer needed" {
				t.Errorf("reason %q", m.Reason)
			}
		}
		if !told[as[0].AttemptID] || !told[as[1].AttemptID] {
			t.Fatalf("told %v", told)
		}
		// The Worker accepts the second, then reports both cancelled.
		s.send(ports.AcceptAttempt{AttemptID: as[1].AttemptID, FencingToken: as[1].Lease.FencingToken})
		for _, a := range as {
			s.send(ports.Result{AttemptID: a.AttemptID, FencingToken: a.Lease.FencingToken, Outcome: ports.OutcomeCancelled})
			if ack, ok := s.recv().(ports.ResultAck); !ok || ack.Disposition != ports.ResultAccepted {
				t.Fatalf("ack %+v", ack)
			}
		}
		e, steps = state(t, store, got.Execution.ID)
		if e.State != domain.ExecCancelled {
			t.Fatalf("execution %s, want cancelled", e.State)
		}
		for id, st := range steps {
			if st != domain.StepCancelled {
				t.Errorf("%s is %s", id, st)
			}
		}
		// Nothing is offered again.
		s.send(ports.Offer{FreeSlots: 5})
		s.expectNothing()
	})

	t.Run("a Step that finished before the Cancel arrived still lets the Execution end", func(t *testing.T) {
		store := newStore(t)
		c := limited(store)
		got := submit(t, c, "k1", "p1", "", 1)
		s := startSession(t, store, c, nl, "w1")
		s.send(ports.Offer{FreeSlots: 1})
		a := s.assignment()
		s.send(ports.AcceptAttempt{AttemptID: a.AttemptID, FencingToken: a.Lease.FencingToken})
		waitFor(t, "the Step to run", func() bool { _, m := state(t, store, got.Execution.ID); return m[a.StepID] == domain.StepRunning })
		if _, _, err := cancel(c, "c1", got.Execution.ID); err != nil {
			t.Fatal(err)
		}
		s.recv() // the Cancel
		s.send(ports.Result{AttemptID: a.AttemptID, FencingToken: a.Lease.FencingToken, Outcome: ports.OutcomeSucceeded})
		s.recv() // the ack
		e, steps := state(t, store, got.Execution.ID)
		if e.State != domain.ExecCancelled || steps[a.StepID] != domain.StepSucceeded {
			t.Fatalf("%s %v: a finished Step keeps its result and the Execution is cancelled", e.State, steps)
		}
	})

	t.Run("a restart settles a cancel whose Worker is gone", func(t *testing.T) {
		store := newStore(t)
		c := limited(store)
		got := submit(t, c, "k1", "p1", "", 2)
		s := startSession(t, store, c, nl, "w1")
		s.send(ports.Offer{FreeSlots: 2})
		as := s.drain()
		for _, a := range as {
			s.send(ports.AcceptAttempt{AttemptID: a.AttemptID, FencingToken: a.Lease.FencingToken})
		}
		waitFor(t, "both Steps to run", func() bool {
			_, m := state(t, store, got.Execution.ID)
			return m[as[0].StepID] == domain.StepRunning && m[as[1].StepID] == domain.StepRunning
		})
		out, _, err := cancel(c, "c1", got.Execution.ID)
		if err != nil || len(out.Stopping) != 2 {
			t.Fatalf("%+v %v", out, err)
		}
		// The Platform restarts before the Worker reported: a new coordinator over the stored data.
		restarted := limited(reopen(t, store))
		if _, err := restarted.Reconcile(ctx); err != nil {
			t.Fatal(err)
		}
		e, steps := state(t, reopen(t, store), got.Execution.ID)
		if e.State != domain.ExecCancelled {
			t.Fatalf("execution %s after restart, want cancelled; steps %v", e.State, steps)
		}
		for id, st := range steps {
			if !st.Terminal() {
				t.Errorf("%s is %s", id, st)
			}
		}
	})
}

// waitFor polls until cond holds; the coordinator applies a message after the test sent it.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// submitOne is a request for one Step; idem marks the Step idempotent.
func submitOne(key string, idem bool) coordinator.SubmitRequest {
	return coordinator.SubmitRequest{By: Requester, Key: key, ProjectID: "p1", Steps: []coordinator.StepSpec{{Name: "s1", Capability: "code.test", Idempotent: idem}}}
}
