package coordtest

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rluders/amaru/services/platform/internal/coordinator"
	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
)

// NewLink returns the two ends of a new Worker link. The suite runs once per implementation of the
// link (in-process, and a stand-in that serializes every message), so the coordinator is shown to
// depend on the Worker interface and on nothing a Worker shares with it.
type NewLink func(t *testing.T) (ports.PlatformLink, ports.WorkerLink)

const (
	wait        = 5 * time.Second
	quiet       = 150 * time.Millisecond
	leasePeriod = time.Minute
)

type session struct {
	t     *testing.T
	store ports.ExecutionStore
	c     *coordinator.Coordinator
	w     ports.WorkerLink
	ctx   context.Context
}

// start serves one Worker session for the test and returns the Worker's end.
func startSession(t *testing.T, store ports.ExecutionStore, c *coordinator.Coordinator, nl NewLink, id string) *session {
	t.Helper()
	pl, wl := nl(t)
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := c.Serve(ctx, pl, id); err != nil {
			t.Errorf("serve: %v", err)
		}
	}()
	t.Cleanup(func() { cancel(); wl.Close(); wg.Wait() })
	return &session{t: t, store: store, c: c, w: wl, ctx: ctx}
}

func (s *session) send(m ports.WorkerMessage) {
	s.t.Helper()
	if err := s.w.Send(s.ctx, m); err != nil {
		s.t.Fatal(err)
	}
}

func (s *session) recv() ports.PlatformMessage {
	s.t.Helper()
	ctx, cancel := context.WithTimeout(s.ctx, wait)
	defer cancel()
	m, err := s.w.Receive(ctx)
	if err != nil {
		s.t.Fatalf("expected a message from the Platform: %v", err)
	}
	return m
}

func (s *session) assignment() ports.Assignment {
	s.t.Helper()
	a, ok := s.recv().(ports.Assignment)
	if !ok {
		s.t.Fatal("expected an Assignment")
	}
	return a
}

func (s *session) expectNothing() {
	s.t.Helper()
	ctx, cancel := context.WithTimeout(s.ctx, quiet)
	defer cancel()
	if m, err := s.w.Receive(ctx); err == nil {
		s.t.Fatalf("unexpected message: %#v", m)
	}
}

func (s *session) step(exec, step string) (domain.Execution, domain.StepExecution) {
	s.t.Helper()
	e, steps, err := s.store.Get(context.Background(), exec)
	if err != nil {
		s.t.Fatal(err)
	}
	for _, st := range steps {
		if st.ID == step {
			return e, st
		}
	}
	s.t.Fatalf("no step %s", step)
	return e, domain.StepExecution{}
}

func (s *session) events(exec string) []domain.ExecutionEvent {
	s.t.Helper()
	evs, err := s.store.Events(context.Background(), exec)
	if err != nil {
		s.t.Fatal(err)
	}
	return evs
}

func queued(t *testing.T, store ports.ExecutionStore, execID string, created time.Time, steps ...string) {
	t.Helper()
	e := domain.Execution{ID: execID, State: domain.ExecQueued, CreatedAt: created, RequestedBy: Requester, Trigger: domain.TriggerManual}
	ss := make([]domain.StepExecution, len(steps))
	for i, id := range steps {
		ss[i] = domain.StepExecution{ID: id, ExecutionID: execID, State: domain.StepPending, Capability: "code.test", InputsJSON: []byte(`{"n":1}`), Idempotent: i == 0}
	}
	_, ss, err := store.Create(context.Background(), e, ss)
	if err != nil {
		t.Fatal(err)
	}
	for i := range ss {
		if err := ss[i].Plan(); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.Commit(context.Background(), ports.ExecutionChange{Steps: ss}); err != nil {
		t.Fatal(err)
	}
}

func newCoordinator(store ports.ExecutionStore) *coordinator.Coordinator {
	var n atomic.Int64
	return coordinator.New(store, clock, Permissive,
		coordinator.WithLeasePeriod(leasePeriod),
		coordinator.WithAttemptIDs(func() string { return fmt.Sprintf("att-%d", n.Add(1)) }))
}

// AssignmentSuite is the Worker-interface contract: what the coordinator does for every message a
// Worker can send, checked through the link only.
func AssignmentSuite(t *testing.T, newStore func(t *testing.T) ports.ExecutionStore, nl NewLink) {
	t.Run("an Offer is answered with an Assignment, and the Lease is durable before it is sent", func(t *testing.T) {
		store := newStore(t)
		queued(t, store, "e1", t0, "s1")
		s := startSession(t, store, newCoordinator(store), nl, "w1")
		s.send(ports.Offer{FreeSlots: 1})
		a := s.assignment()
		if a.ExecutionID != "e1" || a.StepID != "s1" || a.AttemptID != "att-1" || a.Capability != "code.test" ||
			string(a.InputsJSON) != `{"n":1}` || a.IdempotencyKey != "e1/s1" || !a.CapabilityIdempotent ||
			a.Lease.LeaseID != "att-1" || a.Lease.FencingToken != 1 || !a.Lease.ExpiresAt.Equal(now.Add(leasePeriod)) {
			t.Fatalf("assignment: %+v", a)
		}
		_, st := s.step("e1", "s1")
		if st.State != domain.StepLeased || st.CurrentLease == nil || st.CurrentLease.Owner != "w1" || st.FenceToken != 1 {
			t.Fatalf("the Assignment arrived before the Lease was stored: %+v", st)
		}
		if evs := s.events("e1"); len(evs) != 1 || evs[0].Kind != domain.EventStepLeased {
			t.Fatalf("events: %+v", evs)
		}
	})

	t.Run("accept starts the Step and the Execution; a result ends the Step", func(t *testing.T) {
		store := newStore(t)
		queued(t, store, "e1", t0, "s1")
		s := startSession(t, store, newCoordinator(store), nl, "w1")
		s.send(ports.Offer{FreeSlots: 1})
		a := s.assignment()
		s.send(ports.AcceptAttempt{AttemptID: a.AttemptID, FencingToken: a.Lease.FencingToken})
		s.send(ports.Result{AttemptID: a.AttemptID, FencingToken: a.Lease.FencingToken, Outcome: ports.OutcomeSucceeded})
		ack, ok := s.recv().(ports.ResultAck)
		if !ok || ack.Disposition != ports.ResultAccepted || ack.AttemptID != a.AttemptID {
			t.Fatalf("ack: %#v", ack)
		}
		e, st := s.step("e1", "s1")
		if st.State != domain.StepSucceeded || st.CurrentLease != nil || st.EndedAt == nil || e.State != domain.ExecRunning {
			t.Fatalf("step %+v, execution %s", st, e.State)
		}
		kinds := []string{}
		for _, ev := range s.events("e1") {
			kinds = append(kinds, ev.Kind)
		}
		want := []string{domain.EventStepLeased, domain.EventStepStarted, domain.EventStepSucceeded}
		if fmt.Sprint(kinds) != fmt.Sprint(want) {
			t.Fatalf("events %v, want %v", kinds, want)
		}
	})

	t.Run("a failed result keeps its error", func(t *testing.T) {
		store := newStore(t)
		queued(t, store, "e1", t0, "s1")
		s := startSession(t, store, newCoordinator(store), nl, "w1")
		s.send(ports.Offer{FreeSlots: 1})
		a := s.assignment()
		s.send(ports.AcceptAttempt{AttemptID: a.AttemptID, FencingToken: 1})
		s.send(ports.Result{AttemptID: a.AttemptID, FencingToken: 1, Outcome: ports.OutcomeFailed, ErrorCode: "tests_failed", ErrorDetail: "2 failed"})
		s.recv()
		_, st := s.step("e1", "s1")
		if st.State != domain.StepFailed || st.Error == nil || st.Error.Code != "tests_failed" || st.Error.Message != "2 failed" {
			t.Fatalf("step: %+v", st)
		}
	})

	t.Run("a second result for a completed attempt is a duplicate and changes nothing", func(t *testing.T) {
		store := newStore(t)
		queued(t, store, "e1", t0, "s1")
		s := startSession(t, store, newCoordinator(store), nl, "w1")
		s.send(ports.Offer{FreeSlots: 1})
		a := s.assignment()
		s.send(ports.AcceptAttempt{AttemptID: a.AttemptID, FencingToken: 1})
		res := ports.Result{AttemptID: a.AttemptID, FencingToken: 1, Outcome: ports.OutcomeSucceeded}
		s.send(res)
		s.recv()
		_, before := s.step("e1", "s1")
		n := len(s.events("e1"))
		s.send(res)
		if ack := s.recv().(ports.ResultAck); ack.Disposition != ports.ResultDuplicate {
			t.Fatalf("got %v, want duplicate", ack.Disposition)
		}
		_, after := s.step("e1", "s1")
		if after.Version != before.Version || len(s.events("e1")) != n {
			t.Fatal("a duplicate result changed state")
		}
	})

	t.Run("a rejected attempt returns to the queue and is not re-offered until the Worker offers again", func(t *testing.T) {
		store := newStore(t)
		queued(t, store, "e1", t0, "s1")
		s := startSession(t, store, newCoordinator(store), nl, "w1")
		s.send(ports.Offer{FreeSlots: 1})
		a := s.assignment()
		s.send(ports.RejectAttempt{AttemptID: a.AttemptID, Reason: ports.RejectNoCapacity, Detail: "busy"})
		s.expectNothing() // no loop of re-assigning the Step the Worker just refused
		_, st := s.step("e1", "s1")
		if st.State != domain.StepQueued || st.Reason != domain.ReasonAttemptRejected || st.CurrentLease != nil {
			t.Fatalf("step: %+v", st)
		}
		s.send(ports.Offer{FreeSlots: 1})
		b := s.assignment()
		if b.AttemptID != "att-2" || b.Lease.FencingToken != 2 {
			t.Fatalf("second attempt: %+v", b)
		}
	})

	t.Run("a result with an old fencing token is stale and does not end the Step", func(t *testing.T) {
		store := newStore(t)
		queued(t, store, "e1", t0, "s1")
		s := startSession(t, store, newCoordinator(store), nl, "w1")
		s.send(ports.Offer{FreeSlots: 1})
		a := s.assignment()
		s.send(ports.RejectAttempt{AttemptID: a.AttemptID, Reason: ports.RejectNoCapacity})
		s.expectNothing()
		s.send(ports.Offer{FreeSlots: 1})
		b := s.assignment()
		s.send(ports.AcceptAttempt{AttemptID: b.AttemptID, FencingToken: b.Lease.FencingToken})
		// the first attempt finishes late, with its own token
		s.send(ports.Result{AttemptID: a.AttemptID, FencingToken: 1, Outcome: ports.OutcomeSucceeded})
		if ack := s.recv().(ports.ResultAck); ack.Disposition != ports.ResultRejectedStale {
			t.Fatalf("old attempt: got %v, want rejected_stale", ack.Disposition)
		}
		// and the current attempt with the old token
		s.send(ports.Result{AttemptID: b.AttemptID, FencingToken: 1, Outcome: ports.OutcomeSucceeded})
		if ack := s.recv().(ports.ResultAck); ack.Disposition != ports.ResultRejectedStale {
			t.Fatalf("old token: got %v, want rejected_stale", ack.Disposition)
		}
		if _, st := s.step("e1", "s1"); st.State != domain.StepRunning {
			t.Fatalf("a stale result ended the Step: %s", st.State)
		}
		stale := 0
		for _, ev := range s.events("e1") {
			if ev.Kind == domain.EventResultStale {
				stale++
			}
		}
		// Only the latest attempt of a Step is remembered, so the first (superseded) attempt is
		// unknown: refused, but there is no Step to attach an Event to. The current attempt with a
		// stale token is recorded.
		if stale != 1 {
			t.Fatalf("%d result.rejected_stale events, want 1", stale)
		}
		s.send(ports.Result{AttemptID: "no-such-attempt", FencingToken: 1, Outcome: ports.OutcomeSucceeded})
		if ack := s.recv().(ports.ResultAck); ack.Disposition != ports.ResultRejectedStale {
			t.Fatalf("unknown attempt: got %v", ack.Disposition)
		}
	})

	t.Run("a heartbeat extends the Lease; a stale one is revoked", func(t *testing.T) {
		store := newStore(t)
		queued(t, store, "e1", t0, "s1")
		clk := now
		var mu sync.Mutex
		c := coordinator.New(store, func() time.Time { mu.Lock(); defer mu.Unlock(); return clk }, Permissive,
			coordinator.WithLeasePeriod(leasePeriod), coordinator.WithAttemptIDs(func() string { return "att-1" }))
		s := startSession(t, store, c, nl, "w1")
		s.send(ports.Offer{FreeSlots: 1})
		a := s.assignment()
		s.send(ports.AcceptAttempt{AttemptID: a.AttemptID, FencingToken: 1})
		mu.Lock()
		clk = clk.Add(20 * time.Second)
		mu.Unlock()
		s.send(ports.LeaseRenew{AttemptID: a.AttemptID, FencingToken: 1})
		s.send(ports.LeaseRenew{AttemptID: a.AttemptID, FencingToken: 9}) // wrong token: ordered after, so the first was applied
		rv, ok := s.recv().(ports.LeaseRevoked)
		if !ok || rv.AttemptID != a.AttemptID || rv.Reason != "stale_lease" {
			t.Fatalf("got %#v, want a revocation", rv)
		}
		_, st := s.step("e1", "s1")
		mu.Lock()
		want := clk.Add(leasePeriod)
		mu.Unlock()
		if st.CurrentLease == nil || !st.CurrentLease.ExpiresAt.Equal(want.Truncate(time.Microsecond)) {
			t.Fatalf("lease not extended: %+v, want %v", st.CurrentLease, want)
		}
	})

	t.Run("accept with a wrong token or an unknown attempt is revoked and starts nothing", func(t *testing.T) {
		store := newStore(t)
		queued(t, store, "e1", t0, "s1")
		s := startSession(t, store, newCoordinator(store), nl, "w1")
		s.send(ports.Offer{FreeSlots: 1})
		a := s.assignment()
		s.send(ports.AcceptAttempt{AttemptID: a.AttemptID, FencingToken: 7})
		if rv, ok := s.recv().(ports.LeaseRevoked); !ok || rv.Reason != "stale_lease" {
			t.Fatalf("wrong token: %#v", rv)
		}
		s.send(ports.AcceptAttempt{AttemptID: "nope", FencingToken: 1})
		if rv, ok := s.recv().(ports.LeaseRevoked); !ok || rv.Reason != "unknown_attempt" {
			t.Fatalf("unknown: %#v", rv)
		}
		e, st := s.step("e1", "s1")
		if st.State != domain.StepLeased || e.State != domain.ExecQueued {
			t.Fatalf("step %s, execution %s", st.State, e.State)
		}
	})

	t.Run("an Offer of n slots gets at most n Assignments", func(t *testing.T) {
		store := newStore(t)
		queued(t, store, "e1", t0, "s1", "s2", "s3")
		s := startSession(t, store, newCoordinator(store), nl, "w1")
		s.send(ports.Offer{FreeSlots: 2})
		a, b := s.assignment(), s.assignment()
		if a.StepID != "s1" || b.StepID != "s2" {
			t.Fatalf("order: %s, %s", a.StepID, b.StepID)
		}
		s.expectNothing()
		// Attempts sent and not yet accepted are not in the Worker's free count, so they come off
		// the next Offer: a Worker that has not accepted them is not handed more.
		s.send(ports.Offer{FreeSlots: 1})
		s.expectNothing()
		s.send(ports.AcceptAttempt{AttemptID: a.AttemptID, FencingToken: 1})
		s.send(ports.AcceptAttempt{AttemptID: b.AttemptID, FencingToken: 1})
		s.send(ports.Offer{FreeSlots: 1})
		if c := s.assignment(); c.StepID != "s3" {
			t.Fatalf("third: %s", c.StepID)
		}
	})

	t.Run("older Executions are served first", func(t *testing.T) {
		store := newStore(t)
		queued(t, store, "new", t0.Add(time.Hour), "n1")
		queued(t, store, "old", t0, "o1")
		s := startSession(t, store, newCoordinator(store), nl, "w1")
		s.send(ports.Offer{FreeSlots: 1})
		if a := s.assignment(); a.ExecutionID != "old" {
			t.Fatalf("got %s first", a.ExecutionID)
		}
	})

	t.Run("a Worker with credit is handed a Step that is queued later, when the coordinator is woken", func(t *testing.T) {
		store := newStore(t)
		c := newCoordinator(store)
		s := startSession(t, store, c, nl, "w1")
		s.send(ports.Offer{FreeSlots: 1})
		s.expectNothing()
		queued(t, store, "e1", t0, "s1")
		c.Wake()
		if a := s.assignment(); a.StepID != "s1" {
			t.Fatalf("got %s", a.StepID)
		}
	})

	t.Run("two Workers offering for one Step: exactly one gets it", func(t *testing.T) {
		store := newStore(t)
		queued(t, store, "e1", t0, "s1")
		c := newCoordinator(store)
		s1, s2 := startSession(t, store, c, nl, "w1"), startSession(t, store, c, nl, "w2")
		s1.send(ports.Offer{FreeSlots: 1})
		s2.send(ports.Offer{FreeSlots: 1})
		time.Sleep(quiet)
		got := 0
		for _, s := range []*session{s1, s2} {
			ctx, cancel := context.WithTimeout(s.ctx, quiet)
			if m, err := s.w.Receive(ctx); err == nil {
				if _, ok := m.(ports.Assignment); ok {
					got++
				}
			}
			cancel()
		}
		if got != 1 {
			t.Fatalf("%d Workers were assigned the Step, want 1", got)
		}
		if _, st := s1.step("e1", "s1"); st.FenceToken != 1 {
			t.Fatalf("token %d", st.FenceToken)
		}
	})
}
