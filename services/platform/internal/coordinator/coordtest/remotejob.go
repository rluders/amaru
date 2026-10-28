package coordtest

import (
	"context"
	"testing"
	"time"

	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
)

// RemoteJobSuite checks remote-job recording and re-attachment (FEAT-CAP-004) through the Worker
// link, against the store newStore returns: a Worker records the job of a running Step, the record
// is fenced and validated, it survives a restart, and a Step re-leased after a restart carries the
// job in its Assignment so the Provider re-attaches instead of submitting again.
func RemoteJobSuite(t *testing.T, newStore func(t *testing.T) ports.ExecutionStore, nl NewLink, reopen Restart) {
	t.Helper()
	if reopen == nil {
		reopen = func(_ *testing.T, s ports.ExecutionStore) ports.ExecutionStore { return s }
	}
	ctx := context.Background()
	job := func(id string, resumable bool) ports.StepEvent {
		d := `{"id":"` + id + `","resumable":` + map[bool]string{true: "true", false: "false"}[resumable] + `}`
		return ports.StepEvent{Type: ports.StepEventRemoteJob, DataJSON: []byte(d)}
	}
	// running leases and accepts the only Step and waits until it runs.
	running := func(t *testing.T, store ports.ExecutionStore, idem bool) (*session, ports.Assignment, string) {
		t.Helper()
		c := limited(store)
		got, _, err := c.Submit(ctx, submitOne("k1", idem))
		if err != nil {
			t.Fatal(err)
		}
		s := startSession(t, store, c, nl, "w1")
		s.send(ports.Offer{FreeSlots: 1})
		a := s.assignment()
		s.send(ports.AcceptAttempt{AttemptID: a.AttemptID, FencingToken: a.Lease.FencingToken})
		waitFor(t, "the Step to run", func() bool { _, st := s.step(got.Execution.ID, a.StepID); return st.State == domain.StepRunning })
		return s, a, got.Execution.ID
	}
	recorded := func(s *session, exec, step string) *domain.RemoteJob {
		_, st := s.step(exec, step)
		return st.RemoteJob
	}

	t.Run("a Worker records the job of a running Step, and a repeat changes nothing", func(t *testing.T) {
		store := newStore(t)
		s, a, exec := running(t, store, false)
		s.send(ports.StepEvents{AttemptID: a.AttemptID, FencingToken: a.Lease.FencingToken, Events: []ports.StepEvent{job("job-1", true)}})
		waitFor(t, "the job to be recorded", func() bool { return recorded(s, exec, a.StepID) != nil })
		if j := recorded(s, exec, a.StepID); j.ID != "job-1" || !j.Resumable {
			t.Fatalf("recorded %+v", j)
		}
		n := len(s.events(exec))
		s.send(ports.StepEvents{AttemptID: a.AttemptID, FencingToken: a.Lease.FencingToken, Events: []ports.StepEvent{job("job-1", true)}})
		// a second phase may report a new id
		s.send(ports.StepEvents{AttemptID: a.AttemptID, FencingToken: a.Lease.FencingToken, Events: []ports.StepEvent{job("job-2", true)}})
		waitFor(t, "the new id", func() bool { j := recorded(s, exec, a.StepID); return j != nil && j.ID == "job-2" })
		if got := len(s.events(exec)); got != n+1 {
			t.Fatalf("%d events, want %d: recording the same job again must not write an event", got, n+1)
		}
	})

	t.Run("a stale token, a malformed event and an unknown event type record nothing", func(t *testing.T) {
		store := newStore(t)
		s, a, exec := running(t, store, false)
		s.send(ports.StepEvents{AttemptID: a.AttemptID, FencingToken: a.Lease.FencingToken + 7, Events: []ports.StepEvent{job("job-x", true)}})
		if rv, ok := s.recv().(ports.LeaseRevoked); !ok || rv.Reason != "stale_lease" {
			t.Fatalf("a wrong token must revoke, got %+v", rv)
		}
		s.send(ports.StepEvents{AttemptID: "nope", FencingToken: 1, Events: []ports.StepEvent{job("job-x", true)}})
		if rv, ok := s.recv().(ports.LeaseRevoked); !ok || rv.Reason != "unknown_attempt" {
			t.Fatalf("an unknown attempt must revoke, got %+v", rv)
		}
		s.send(ports.StepEvents{AttemptID: a.AttemptID, FencingToken: a.Lease.FencingToken, Events: []ports.StepEvent{
			{Type: ports.StepEventRemoteJob, DataJSON: []byte(`{"id":"j","extra":1}`)},
			{Type: ports.StepEventRemoteJob, DataJSON: []byte(`not json`)},
			{Type: ports.StepEventRemoteJob, DataJSON: []byte(`{"id":""}`)},
			{Type: ports.StepEventRemoteJob, DataJSON: []byte("{\"id\":\"a\\u0007b\"}")},
			{Type: "progress", Message: "50%"},
		}})
		time.Sleep(100 * time.Millisecond)
		if j := recorded(s, exec, a.StepID); j != nil {
			t.Fatalf("recorded %+v from events that must be dropped", j)
		}
		// The session still works: a good event after the bad ones is applied.
		s.send(ports.StepEvents{AttemptID: a.AttemptID, FencingToken: a.Lease.FencingToken, Events: []ports.StepEvent{job("job-1", false)}})
		waitFor(t, "a good event after bad ones", func() bool { return recorded(s, exec, a.StepID) != nil })
	})

	t.Run("after a restart a resumable job is re-leased with its id; one that is not resumable is interrupted", func(t *testing.T) {
		for _, resumable := range []bool{true, false} {
			store := newStore(t)
			s, a, exec := running(t, store, false)
			s.send(ports.StepEvents{AttemptID: a.AttemptID, FencingToken: a.Lease.FencingToken, Events: []ports.StepEvent{job("job-1", resumable)}})
			waitFor(t, "the job to be recorded", func() bool { return recorded(s, exec, a.StepID) != nil })

			restarted := limited(reopen(t, store))
			rep, err := restarted.Reconcile(ctx)
			if err != nil {
				t.Fatal(err)
			}
			s2 := startSession(t, reopen(t, store), restarted, nl, "w2")
			s2.send(ports.Offer{FreeSlots: 1})
			if !resumable {
				if rep.Interrupted != 1 || rep.Resumed != 0 {
					t.Fatalf("report %+v", rep)
				}
				s2.expectNothing()
				if _, st := s2.step(exec, a.StepID); st.State != domain.StepInterrupted {
					t.Fatalf("step %s, want interrupted", st.State)
				}
				continue
			}
			if rep.Resumed != 1 {
				t.Fatalf("report %+v", rep)
			}
			b := s2.assignment()
			if b.RemoteJob == nil || b.RemoteJob.ID != "job-1" || !b.RemoteJob.Resumable {
				t.Fatalf("the new Assignment carries %+v, want job-1", b.RemoteJob)
			}
			if b.Lease.FencingToken <= a.Lease.FencingToken {
				t.Fatalf("fencing token %d did not grow past %d", b.Lease.FencingToken, a.Lease.FencingToken)
			}
		}
	})
}
