package coordtest

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/rluders/amaru/services/platform/internal/coordinator"
	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
)

// boundaryStore injects faults at the store port, leaving the real adapter's atomic write intact.
type boundaryStore struct {
	ports.ExecutionStore
	before, after func(int) error
	commits       int
}

func (s *boundaryStore) Commit(ctx context.Context, ch ports.ExecutionChange) (ports.ExecutionChange, error) {
	s.commits++
	if s.before != nil {
		if err := s.before(s.commits); err != nil {
			return ports.ExecutionChange{}, err
		}
	}
	out, err := s.ExecutionStore.Commit(ctx, ch)
	if err == nil && s.after != nil {
		err = s.after(s.commits)
	}
	return out, err
}

// protocolLink supplies an ordered fake Worker. Exhaustion waits for cancellation so the receive
// pump cannot report closure before Serve has processed the last message.
type protocolLink struct {
	messages []ports.WorkerMessage
	send     func(ports.PlatformMessage) error
}

func (l *protocolLink) Receive(ctx context.Context) (ports.WorkerMessage, error) {
	if len(l.messages) == 0 {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	m := l.messages[0]
	l.messages = l.messages[1:]
	return m, nil
}
func (l *protocolLink) Send(_ context.Context, m ports.PlatformMessage) error { return l.send(m) }
func (l *protocolLink) Close() error                                          { return nil }

func protocolMessages() []ports.WorkerMessage {
	return []ports.WorkerMessage{
		ports.Offer{FreeSlots: 1},
		ports.AcceptAttempt{AttemptID: "att-1", FencingToken: 1},
		ports.StepEvents{AttemptID: "att-1", FencingToken: 1, Events: []ports.StepEvent{
			{Type: ports.StepEventRemoteJob, DataJSON: []byte(`{"id":"job-1","resumable":true}`)},
		}},
		ports.LeaseRenew{AttemptID: "att-1", FencingToken: 1},
		ports.Result{AttemptID: "att-1", FencingToken: 1, Outcome: ports.OutcomeSucceeded},
	}
}

func readStep(t *testing.T, s ports.ExecutionStore) domain.StepExecution {
	t.Helper()
	_, steps, err := s.Get(context.Background(), "e1")
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 1 {
		t.Fatalf("got %d Steps", len(steps))
	}
	return steps[0]
}

func readEvents(t *testing.T, s ports.ExecutionStore) []domain.ExecutionEvent {
	t.Helper()
	evs, err := s.Events(context.Background(), "e1")
	if err != nil {
		t.Fatal(err)
	}
	for i, ev := range evs {
		if ev.Seq != int64(i+1) {
			t.Fatalf("event sequence gap: %+v", evs)
		}
	}
	return evs
}

// ProtocolDurability exercises lease, start, remote-job recording, renewal and result commits
// through Serve, including a durable result whose acknowledgement never reaches the Worker.
func ProtocolDurability(t *testing.T, newStore func(*testing.T) ports.ExecutionStore, reopen Restart) {
	for _, after := range []bool{false, true} {
		for commit := 1; commit <= 5; commit++ {
			t.Run(fmt.Sprintf("protocol/commit-%d/after-%t", commit, after), func(t *testing.T) {
				store := newStore(t)
				queued(t, store, "e1", t0, "s1")
				fault := &boundaryStore{ExecutionStore: store}
				inject := func(n int) error {
					if n == commit {
						return errCrash
					}
					return nil
				}
				if after {
					fault.after = inject
				} else {
					fault.before = inject
				}
				acks := 0
				link := &protocolLink{messages: protocolMessages(), send: func(m ports.PlatformMessage) error {
					switch m.(type) {
					case ports.Assignment:
						if readStep(t, store).State != domain.StepLeased {
							t.Fatal("assignment before commit")
						}
					case ports.ResultAck:
						acks++
					}
					return nil
				}}
				ctx, cancel := context.WithTimeout(context.Background(), wait)
				defer cancel()
				if err := newCoordinator(fault).Serve(ctx, link, "w1"); !errors.Is(err, errCrash) {
					t.Fatalf("expected injected fault, got %v", err)
				}
				if acks != 0 {
					t.Fatal("result acknowledged despite failed commit")
				}
				store = reopen(t, store)
				durable := commit - 1
				if after {
					durable++
				}
				verifyProtocolCommit(t, store, durable)
				verifyRecovery(t, store, reopen)
			})
		}
	}
	t.Run("protocol/result committed but acknowledgement lost", func(t *testing.T) {
		store := newStore(t)
		queued(t, store, "e1", t0, "s1")
		link := &protocolLink{messages: protocolMessages(), send: func(m ports.PlatformMessage) error {
			if ack, ok := m.(ports.ResultAck); ok {
				if ack.Disposition != ports.ResultAccepted || readStep(t, store).State != domain.StepSucceeded {
					t.Fatal("acknowledgement before durable result")
				}
				return errCrash
			}
			return nil
		}}
		ctx, cancel := context.WithTimeout(context.Background(), wait)
		defer cancel()
		if err := newCoordinator(store).Serve(ctx, link, "w1"); !errors.Is(err, errCrash) {
			t.Fatalf("lost ack: %v", err)
		}
		verifyRecovery(t, store, reopen)
	})
}

func verifyProtocolCommit(t *testing.T, store ports.ExecutionStore, durable int) {
	t.Helper()
	st := readStep(t, store)
	want := domain.StepQueued
	if durable == 1 {
		want = domain.StepLeased
	}
	if durable >= 2 {
		want = domain.StepRunning
	}
	if durable == 5 {
		want = domain.StepSucceeded
	}
	// queued() creates v1, then plans to v2; each successful protocol commit adds one.
	if st.State != want || st.Version != int64(2+durable) {
		t.Fatalf("durable=%d: state/version %+v", durable, st)
	}
	if (st.RemoteJob != nil) != (durable >= 3) {
		t.Fatal("remote job and transition disagree")
	}
	wantEvents := durable
	if durable >= 4 {
		wantEvents--
	} // a renewal changes the Lease, not the event log
	if got := len(readEvents(t, store)); got != wantEvents {
		t.Fatalf("events=%d want=%d", got, wantEvents)
	}
}

func verifyRecovery(t *testing.T, store ports.ExecutionStore, reopen Restart) {
	t.Helper()
	ctx := context.Background()
	var settled domain.StepExecution
	var settledEvents []domain.ExecutionEvent
	for i := 0; i < 2; i++ {
		store = reopen(t, store)
		if _, err := coordinator.New(store, clock).Reconcile(ctx); err != nil {
			t.Fatal(err)
		}
		st, events := readStep(t, store), readEvents(t, store)
		if i == 0 {
			settled, settledEvents = st, events
			continue
		}
		if !reflect.DeepEqual(settled, st) || !reflect.DeepEqual(settledEvents, events) {
			t.Fatal("second restart changed settled Step or events")
		}
		// Recovery never manufactures a capability failure (there is no retry-budget counter yet).
		for _, ev := range events {
			if ev.Kind == domain.EventStepFailed {
				t.Fatal("recovery recorded a failure")
			}
		}
		before := st
		// Replaying an old result after recovery must be a duplicate or stale, never a new success.
		link := &protocolLink{messages: []ports.WorkerMessage{protocolMessages()[4]}, send: func(m ports.PlatformMessage) error {
			ack, ok := m.(ports.ResultAck)
			if !ok {
				t.Fatalf("unexpected reply: %T", m)
			}
			want := ports.ResultRejectedStale
			if before.State == domain.StepSucceeded {
				want = ports.ResultDuplicate
			}
			if ack.Disposition != want {
				t.Fatalf("disposition=%v want=%v", ack.Disposition, want)
			}
			return errCrash
		}}
		timeout, cancel := context.WithTimeout(ctx, wait)
		err := coordinator.New(store, clock, Permissive).Serve(timeout, link, "w2")
		cancel()
		if !errors.Is(err, errCrash) {
			t.Fatalf("replay: %v", err)
		}
		after := readStep(t, store)
		if before.State != domain.StepSucceeded && before.AttemptID == "att-1" {
			// A recognized stale result is audited in an atomic write that advances the row version.
			if after.Version != before.Version+1 {
				t.Fatal("stale-result audit did not advance version once")
			}
			after.Version = before.Version
			audited := readEvents(t, store)
			if len(audited) != len(events)+1 || audited[len(audited)-1].Kind != domain.EventResultStale {
				t.Fatal("missing stale-result audit event")
			}
		}
		if !reflect.DeepEqual(before, after) {
			t.Fatal("old result changed Step")
		}
		if before.State == domain.StepSucceeded && !reflect.DeepEqual(events, readEvents(t, store)) {
			t.Fatal("duplicate result appended events")
		}
		if before.State == domain.StepQueued {
			link = &protocolLink{messages: []ports.WorkerMessage{ports.Offer{FreeSlots: 1}}, send: func(m ports.PlatformMessage) error {
				a, ok := m.(ports.Assignment)
				if !ok || a.AttemptID == before.AttemptID || a.Lease.FencingToken <= before.FenceToken || a.IdempotencyKey != "e1/s1" || !reflect.DeepEqual(a.RemoteJob, remoteJob(before.RemoteJob)) {
					t.Fatalf("recovery assignment: %+v", m)
				}
				return errCrash
			}}
			timeout, cancel = context.WithTimeout(ctx, wait)
			err = coordinator.New(store, clock, Permissive).Serve(timeout, link, "w2")
			cancel()
			if !errors.Is(err, errCrash) {
				t.Fatalf("re-lease: %v", err)
			}
		}
	}
}

func remoteJob(j *domain.RemoteJob) *ports.RemoteJob {
	if j == nil {
		return nil
	}
	return &ports.RemoteJob{ID: j.ID, Resumable: j.Resumable}
}
