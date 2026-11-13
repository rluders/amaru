package platform

import (
	"context"
	"errors"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rluders/amaru/services/platform/internal/coordinator"
	"github.com/rluders/amaru/services/platform/internal/coordinator/coordtest"
	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
	"github.com/rluders/amaru/services/platform/internal/ports/fakes"
	"github.com/rluders/amaru/services/platform/internal/worker/embedded"
	"github.com/rluders/lane/v2"
)

func recoveredCoordinator(t *testing.T, store ports.ExecutionStore) *Recovery {
	t.Helper()
	c := coordinator.New(store, time.Now, coordtest.Permissive)
	if _, err := c.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	r := &Recovery{coordinator: c, ready: make(chan struct{})}
	close(r.ready)
	return r
}

func TestExecutionReadinessWaitsForRecoveryAndInitialOffer(t *testing.T) {
	store := fakes.NewExecutionStore()
	r := recoveredCoordinator(t, store)
	r.ready = make(chan struct{})
	s, _ := NewSession(r, "embedded")
	w, _ := NewEmbeddedWorker(s, embedded.Worker{Runner: embedded.RunnerFunc(func(context.Context, ports.Assignment) embedded.Outcome {
		t.Error("empty store produced an assignment")
		return embedded.Outcome{}
	})})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	l, _ := NewLifecycle(time.Now().Add(3*time.Second), time.Second, nil)
	done, ready := make(chan error, 1), make(chan error, 1)
	go func() { ready <- w.WaitReady(ctx) }()
	admitted := make(chan struct{})
	go func() {
		done <- l.Run(ctx, []lane.Runner{s}, []lane.Runner{w}, []lane.Runner{dependent{ready: func(context.Context) error { close(admitted); return nil }}})
	}()
	probe, stop := context.WithCancel(ctx)
	stop()
	if err := s.WaitReady(probe); !errors.Is(err, context.Canceled) {
		t.Fatal("pre-Start readiness ignored cancellation")
	}
	select {
	case <-ready:
		t.Fatal("Worker ready before recovery")
	case <-admitted:
		t.Fatal("later stage admitted before recovery")
	default:
	}
	close(r.ready)
	wait(t, admitted)
	if err := result(t, ready); err != nil {
		t.Fatal(err)
	}
	cancel()
	if err := result(t, done); err != nil {
		t.Fatal(err)
	}
	if err := s.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := w.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestExecutionShutdownJoinsAttemptWithoutResult(t *testing.T) {
	store := fakes.NewExecutionStore()
	r := recoveredCoordinator(t, store)
	e := domain.Execution{ID: "e1", State: domain.ExecQueued, CreatedAt: time.Now(), RequestedBy: coordtest.Requester, Trigger: domain.TriggerManual}
	_, _, err := store.Create(context.Background(), e, []domain.StepExecution{
		{ID: "running", ExecutionID: e.ID, State: domain.StepQueued, Capability: "code.test", InputsJSON: []byte(`{}`)},
		{ID: "queued", ExecutionID: e.ID, State: domain.StepQueued, Capability: "code.test", InputsJSON: []byte(`{}`)},
	})
	if err != nil {
		t.Fatal(err)
	}
	started, cancelled, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var released atomic.Bool
	defer func() {
		if !released.Swap(true) {
			close(release)
		}
	}()
	s, _ := NewSession(r, "embedded")
	w, _ := NewEmbeddedWorker(s, embedded.Worker{HeartbeatPeriod: time.Hour, Runner: embedded.RunnerFunc(func(ctx context.Context, a ports.Assignment) embedded.Outcome {
		close(started)
		<-ctx.Done()
		close(cancelled)
		<-release
		return embedded.Outcome{Result: ports.OutcomeCancelled}
	})})
	l, _ := NewLifecycle(time.Now().Add(3*time.Second), time.Second, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- l.Run(ctx, []lane.Runner{s}, []lane.Runner{w}) }()
	wait(t, started)
	var before domain.Execution
	var steps []domain.StepExecution
	for deadline := time.Now().Add(time.Second); ; {
		before, steps, err = store.Get(context.Background(), e.ID)
		if err != nil {
			t.Fatal(err)
		}
		if steps[0].State == domain.StepRunning {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("Accept was not committed")
		}
		time.Sleep(time.Millisecond)
	}
	events, err := store.Events(context.Background(), e.ID)
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	wait(t, cancelled)
	if l.Ready() {
		t.Fatal("admission remained open after attempt cancellation")
	}
	cleanup, stop := context.WithCancel(context.Background())
	stop()
	if err := w.Stop(cleanup); !errors.Is(err, context.Canceled) {
		t.Fatal("Worker Stop did not bound its join")
	}
	select {
	case <-done:
		t.Fatal("lifecycle returned before attempt joined")
	default:
	}
	if err := s.platform.Send(context.Background(), ports.ResultAck{}); err != nil {
		t.Fatal("session link closed before Worker joined")
	}
	close(release)
	released.Store(true)
	if err := result(t, done); err != nil {
		t.Fatal(err)
	}
	after, afterSteps, err := store.Get(context.Background(), e.ID)
	afterEvents, eventsErr := store.Events(context.Background(), e.ID)
	if err != nil || eventsErr != nil || !reflect.DeepEqual(before, after) || !reflect.DeepEqual(steps, afterSteps) || !reflect.DeepEqual(events, afterEvents) {
		t.Fatal("shutdown synthesized Result, domain cancellation or terminal state")
	}
	if err := s.platform.Send(context.Background(), ports.ResultAck{}); !errors.Is(err, ports.ErrLinkClosed) {
		t.Fatal("session link not released")
	}
}

func TestExecutionUnexpectedLinkExitIsFatal(t *testing.T) {
	s, _ := NewSession(recoveredCoordinator(t, fakes.NewExecutionStore()), "embedded")
	w, _ := NewEmbeddedWorker(s, embedded.Worker{Runner: embedded.RunnerFunc(func(context.Context, ports.Assignment) embedded.Outcome { return embedded.Outcome{} })})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	l, _ := NewLifecycle(time.Now().Add(time.Second), time.Second, nil)
	done := make(chan error, 1)
	go func() { done <- l.Run(ctx, []lane.Runner{s}, []lane.Runner{w}) }()
	if err := w.WaitReady(ctx); err != nil {
		t.Fatal(err)
	}
	if err := s.platform.Close(); err != nil {
		t.Fatal(err)
	}
	if err := result(t, done); err == nil || l.Ready() {
		t.Fatal("unexpected nil loop exit did not fail the process")
	}
}

func TestExecutionConfigurationAndFailedRecovery(t *testing.T) {
	if _, err := NewSession(nil, "embedded"); err == nil {
		t.Fatal("nil recovery accepted")
	}
	r := &Recovery{ready: make(chan struct{}), initErr: errors.New("recovery failed")}
	if _, err := NewSession(r, "  "); err == nil {
		t.Fatal("empty Worker identity accepted")
	}
	s, _ := NewSession(r, "embedded")
	if _, err := NewEmbeddedWorker(nil, embedded.Worker{}); err == nil {
		t.Fatal("nil session accepted")
	}
	if _, err := NewEmbeddedWorker(s, embedded.Worker{}); err == nil {
		t.Fatal("nil Runner accepted")
	}
	close(r.ready)
	w, _ := NewEmbeddedWorker(s, embedded.Worker{Runner: embedded.RunnerFunc(func(context.Context, ports.Assignment) embedded.Outcome { return embedded.Outcome{} })})
	if err := s.Start(context.Background()); err == nil {
		t.Fatal("failed recovery admitted session")
	}
	if err := w.Start(context.Background()); err == nil {
		t.Fatal("failed session admitted Worker")
	}
	if err := s.WaitReady(context.Background()); err == nil {
		t.Fatal("failed session ready")
	}
	if err := w.WaitReady(context.Background()); err == nil {
		t.Fatal("failed Worker ready")
	}
	if s.platform != nil || s.worker != nil {
		t.Fatal("failed recovery acquired a link")
	}
	if err := w.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := s.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestExecutionCompletesThroughRecoveredCoordinator(t *testing.T) {
	store := fakes.NewExecutionStore()
	r := recoveredCoordinator(t, store)
	e := domain.Execution{ID: "success", State: domain.ExecQueued, CreatedAt: time.Now(), RequestedBy: coordtest.Requester, Trigger: domain.TriggerManual}
	if _, _, err := store.Create(context.Background(), e, []domain.StepExecution{{ID: "s1", ExecutionID: e.ID, State: domain.StepQueued, Capability: "code.test", InputsJSON: []byte(`{}`)}}); err != nil {
		t.Fatal(err)
	}
	s, _ := NewSession(r, "embedded")
	w, _ := NewEmbeddedWorker(s, embedded.Worker{Runner: embedded.RunnerFunc(func(context.Context, ports.Assignment) embedded.Outcome {
		return embedded.Outcome{Result: ports.OutcomeSucceeded}
	})})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	l, _ := NewLifecycle(time.Now().Add(time.Second), time.Second, nil)
	done := make(chan error, 1)
	go func() { done <- l.Run(ctx, []lane.Runner{s}, []lane.Runner{w}) }()
	for deadline := time.Now().Add(time.Second); ; {
		got, steps, err := store.Get(context.Background(), e.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got.State == domain.ExecSucceeded && l.Ready() {
			if steps[0].State != domain.StepSucceeded {
				t.Fatal("Execution completed without Step")
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("composed Worker did not complete Execution")
		}
		time.Sleep(time.Millisecond)
	}
	cancel()
	if err := result(t, done); err != nil {
		t.Fatal(err)
	}
}

func TestExecutionInitialOfferFailureWithholdsReadiness(t *testing.T) {
	s, _ := NewSession(recoveredCoordinator(t, fakes.NewExecutionStore()), "embedded")
	w, _ := NewEmbeddedWorker(s, embedded.Worker{Runner: embedded.RunnerFunc(func(context.Context, ports.Assignment) embedded.Outcome { return embedded.Outcome{} })})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- s.Start(ctx) }()
	if err := s.WaitReady(ctx); err != nil {
		t.Fatal(err)
	}
	if err := s.platform.Close(); err != nil {
		t.Fatal(err)
	}
	if err := w.Start(ctx); err == nil {
		t.Fatal("closed transport admitted Worker")
	}
	if err := w.WaitReady(ctx); !errors.Is(err, ports.ErrLinkClosed) {
		t.Fatal("failed initial Offer published readiness")
	}
	if err := result(t, done); err == nil {
		t.Fatal("closed session exited successfully")
	}
	if err := w.Stop(ctx); err != nil {
		t.Fatal(err)
	}
	if err := s.Stop(ctx); err != nil {
		t.Fatal(err)
	}
}
