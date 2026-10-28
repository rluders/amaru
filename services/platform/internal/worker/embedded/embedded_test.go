package embedded_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rluders/amaru/services/platform/internal/coordinator"
	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
	"github.com/rluders/amaru/services/platform/internal/ports/fakes"
	"github.com/rluders/amaru/services/platform/internal/worker/embedded"
	"github.com/rluders/amaru/services/platform/internal/worker/inproc"
)

var created = time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)

type rig struct {
	t     *testing.T
	store *fakes.ExecutionStore
	c     *coordinator.Coordinator
	pl    ports.PlatformLink
	wl    ports.WorkerLink
	ctx   context.Context
	done  chan error // the Worker's Run result
}

// newRig serves a coordinator and an embedded Worker over an in-process link, with the real clock.
func newRig(t *testing.T, w *embedded.Worker, lease time.Duration) *rig {
	t.Helper()
	store := fakes.NewExecutionStore()
	c := coordinator.New(store, time.Now, coordinator.WithLeasePeriod(lease))
	pl, wl := inproc.Pair()
	ctx, cancel := context.WithCancel(context.Background())
	r := &rig{t: t, store: store, c: c, pl: pl, wl: wl, ctx: ctx, done: make(chan error, 1)}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); _ = c.Serve(ctx, pl, "embedded") }()
	go func() { defer wg.Done(); r.done <- w.Run(ctx, wl) }()
	t.Cleanup(func() { cancel(); pl.Close(); wg.Wait() })
	return r
}

func (r *rig) queue(execID string, steps ...string) {
	r.t.Helper()
	e := domain.Execution{ID: execID, State: domain.ExecQueued, CreatedAt: created}
	ss := make([]domain.StepExecution, len(steps))
	for i, id := range steps {
		ss[i] = domain.StepExecution{ID: id, ExecutionID: execID, State: domain.StepPending, Capability: "code.test"}
	}
	_, ss, err := r.store.Create(r.ctx, e, ss)
	if err != nil {
		r.t.Fatal(err)
	}
	for i := range ss {
		_ = ss[i].Plan()
	}
	if _, err := r.store.Commit(r.ctx, ports.ExecutionChange{Steps: ss}); err != nil {
		r.t.Fatal(err)
	}
	r.c.Wake()
}

func (r *rig) until(what string, cond func() bool) {
	r.t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			r.t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func (r *rig) states(execID string) map[string]domain.StepState {
	_, steps, err := r.store.Get(r.ctx, execID)
	if err != nil {
		r.t.Fatal(err)
	}
	m := map[string]domain.StepState{}
	for _, s := range steps {
		m[s.ID] = s.State
	}
	return m
}

func succeed(context.Context, ports.Assignment) embedded.Outcome {
	return embedded.Outcome{Result: ports.OutcomeSucceeded}
}

func TestStepRunsEndToEndThroughTheWorkerInterface(t *testing.T) {
	var got ports.Assignment
	r := newRig(t, &embedded.Worker{HeartbeatPeriod: time.Hour, Runner: embedded.RunnerFunc(
		func(_ context.Context, a ports.Assignment) embedded.Outcome { got = a; return succeed(nil, a) })}, time.Minute)
	r.queue("e1", "s1")
	r.until("the Step to succeed", func() bool { return r.states("e1")["s1"] == domain.StepSucceeded })
	if got.StepID != "s1" || got.Capability != "code.test" || got.Lease.FencingToken != 1 || got.IdempotencyKey != "e1/s1" {
		t.Fatalf("the Runner saw %+v", got)
	}
	e, _, _ := r.store.Get(r.ctx, "e1")
	if e.State != domain.ExecRunning {
		t.Fatalf("execution %s", e.State)
	}
}

func TestFailureAndPanicAreReported(t *testing.T) {
	r := newRig(t, &embedded.Worker{Slots: 2, HeartbeatPeriod: time.Hour, Runner: embedded.RunnerFunc(
		func(_ context.Context, a ports.Assignment) embedded.Outcome {
			if a.StepID == "bad" {
				panic("boom")
			}
			return embedded.Outcome{Result: ports.OutcomeFailed, ErrorCode: "tests_failed", ErrorDetail: "1 failed"}
		})}, time.Minute)
	r.queue("e1", "bad", "ok")
	r.until("both Steps to fail", func() bool {
		s := r.states("e1")
		return s["bad"] == domain.StepFailed && s["ok"] == domain.StepFailed
	})
	_, steps, _ := r.store.Get(r.ctx, "e1")
	for _, s := range steps {
		want := "tests_failed"
		if s.ID == "bad" {
			want = "runner_panic"
		}
		if s.Error == nil || s.Error.Code != want {
			t.Errorf("%s: error %+v, want code %s", s.ID, s.Error, want)
		}
	}
}

func TestSlotsLimitConcurrency(t *testing.T) {
	var cur, peak atomic.Int32
	r := newRig(t, &embedded.Worker{Slots: 2, HeartbeatPeriod: time.Hour, Runner: embedded.RunnerFunc(
		func(_ context.Context, a ports.Assignment) embedded.Outcome {
			n := cur.Add(1)
			for {
				p := peak.Load()
				if n <= p || peak.CompareAndSwap(p, n) {
					break
				}
			}
			time.Sleep(30 * time.Millisecond)
			cur.Add(-1)
			return succeed(nil, a)
		})}, time.Minute)
	r.queue("e1", "s1", "s2", "s3", "s4", "s5")
	r.until("all Steps to succeed", func() bool {
		for _, st := range r.states("e1") {
			if st != domain.StepSucceeded {
				return false
			}
		}
		return true
	})
	if peak.Load() != 2 {
		t.Fatalf("peak concurrency %d, want 2", peak.Load())
	}
}

func TestHeartbeatRenewsTheLeaseWhileTheRunnerWorks(t *testing.T) {
	release := make(chan struct{})
	r := newRig(t, &embedded.Worker{HeartbeatPeriod: 10 * time.Millisecond, Runner: embedded.RunnerFunc(
		func(ctx context.Context, a ports.Assignment) embedded.Outcome {
			select {
			case <-release:
			case <-ctx.Done():
			}
			return succeed(nil, a)
		})}, time.Minute)
	r.queue("e1", "s1")
	var first time.Time
	r.until("a Lease", func() bool {
		_, steps, _ := r.store.Get(r.ctx, "e1")
		if steps[0].State == domain.StepRunning && steps[0].CurrentLease != nil {
			first = steps[0].CurrentLease.ExpiresAt
			return true
		}
		return false
	})
	r.until("the Lease to be renewed", func() bool {
		_, steps, _ := r.store.Get(r.ctx, "e1")
		return steps[0].CurrentLease != nil && steps[0].CurrentLease.ExpiresAt.After(first)
	})
	close(release)
	r.until("the Step to succeed", func() bool { return r.states("e1")["s1"] == domain.StepSucceeded })
}

func TestClosingTheLinkCancelsRunningWorkAndReportsNothing(t *testing.T) {
	started := make(chan struct{})
	var cancelled atomic.Bool
	r := newRig(t, &embedded.Worker{HeartbeatPeriod: time.Hour, Runner: embedded.RunnerFunc(
		func(ctx context.Context, a ports.Assignment) embedded.Outcome {
			close(started)
			<-ctx.Done()
			cancelled.Store(true)
			return embedded.Outcome{Result: ports.OutcomeCancelled}
		})}, time.Minute)
	r.queue("e1", "s1")
	<-started
	r.until("the Platform to record the start", func() bool { return r.states("e1")["s1"] == domain.StepRunning })
	r.pl.Close()
	select {
	case err := <-r.done:
		if err != nil {
			t.Fatalf("Run returned %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after the link closed")
	}
	if !cancelled.Load() {
		t.Fatal("the Runner was not cancelled")
	}
	// The Worker reported no Result: what becomes of the Step is the restart rule's business.
	if st := r.states("e1")["s1"]; st != domain.StepRunning {
		t.Fatalf("Step is %s, want it left running for the restart rule", st)
	}
}

func TestWorkerNeedsARunner(t *testing.T) {
	_, wl := inproc.Pair()
	if err := (&embedded.Worker{}).Run(context.Background(), wl); err == nil {
		t.Fatal("a Worker with no Runner started")
	}
}
