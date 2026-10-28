package embedded_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/rluders/amaru/services/platform/internal/coordinator"
	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
	"github.com/rluders/amaru/services/platform/internal/ports/fakes"
	"github.com/rluders/amaru/services/platform/internal/worker/embedded"
	"github.com/rluders/amaru/services/platform/internal/worker/inproc"
)

// fakeService is a remote service that outlives the Platform: it keeps its jobs when the Platform
// "restarts". It stands in for a vendor's API; no real service is called.
type fakeService struct {
	mu        sync.Mutex
	submitted []string // idempotency keys, one per accepted submit
	done      map[string]chan struct{}
	attached  []string // job ids a Worker re-attached to
}

func newFakeService() *fakeService { return &fakeService{done: map[string]chan struct{}{}} }

func (s *fakeService) submit(key string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := fmt.Sprintf("job-%d", len(s.submitted)+1)
	s.submitted = append(s.submitted, key)
	s.done[id] = make(chan struct{})
	return id
}

func (s *fakeService) finish(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	close(s.done[id])
}

func (s *fakeService) wait(ctx context.Context, id string) error {
	s.mu.Lock()
	ch := s.done[id]
	s.mu.Unlock()
	select {
	case <-ch:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// runner is a remote-job Provider: it re-attaches to the job in the Assignment, else submits.
func (s *fakeService) runner(resumable bool) embedded.Runner {
	return embedded.RunnerFunc(func(ctx context.Context, a ports.Assignment) embedded.Outcome {
		var id string
		if a.RemoteJob != nil {
			id = a.RemoteJob.ID
			s.mu.Lock()
			s.attached = append(s.attached, id)
			s.mu.Unlock()
		} else {
			id = s.submit(a.IdempotencyKey)
			if err := embedded.ReportRemoteJob(ctx, id, resumable); err != nil {
				return embedded.Outcome{Result: ports.OutcomeFailed, ErrorCode: "report_failed", ErrorDetail: err.Error()}
			}
		}
		if err := s.wait(ctx, id); err != nil {
			return embedded.Outcome{Result: ports.OutcomeFailed, ErrorCode: "cancelled"}
		}
		return embedded.Outcome{Result: ports.OutcomeSucceeded}
	})
}

// platform is one run of the Platform over a store: a coordinator, an embedded Worker, a link.
type platform struct {
	c    *coordinator.Coordinator
	stop func()
}

func startPlatform(t *testing.T, store *fakes.ExecutionStore, run embedded.Runner) *platform {
	t.Helper()
	c := coordinator.New(store, time.Now)
	ctx, cancel := context.WithCancel(context.Background())
	if _, err := c.Reconcile(ctx); err != nil {
		t.Fatal(err)
	}
	pl, wl := inproc.Pair()
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); _ = c.Serve(ctx, pl, "embedded") }()
	go func() { defer wg.Done(); _ = (&embedded.Worker{HeartbeatPeriod: time.Hour, Runner: run}).Run(ctx, wl) }()
	p := &platform{c: c, stop: func() { cancel(); pl.Close(); wg.Wait() }}
	t.Cleanup(p.stop)
	return p
}

func waitUntil(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func stepOf(t *testing.T, store *fakes.ExecutionStore, exec string) domain.StepExecution {
	t.Helper()
	_, steps, err := store.Get(context.Background(), exec)
	if err != nil {
		t.Fatal(err)
	}
	return steps[0]
}

func submitJob(t *testing.T, p *platform) string {
	t.Helper()
	got, _, err := p.c.Submit(context.Background(), coordinator.SubmitRequest{Key: "k1", ProjectID: "p1",
		Steps: []coordinator.StepSpec{{Name: "render", Capability: "code.test"}}})
	if err != nil {
		t.Fatal(err)
	}
	return got.Execution.ID
}

func TestAResumableRemoteJobSurvivesAPlatformRestartAndIsReattached(t *testing.T) {
	store, svc := fakes.NewExecutionStore(), newFakeService()
	p1 := startPlatform(t, store, svc.runner(true))
	exec := submitJob(t, p1)
	waitUntil(t, "the remote job to be recorded", func() bool { return stepOf(t, store, exec).RemoteJob != nil })
	if j := stepOf(t, store, exec).RemoteJob; j.ID != "job-1" || !j.Resumable || stepOf(t, store, exec).State != domain.StepRunning {
		t.Fatalf("recorded %+v, state %s", j, stepOf(t, store, exec).State)
	}

	// The Platform restarts while the job runs at the service: nothing reports, the Step is left running.
	p1.stop()
	if s := stepOf(t, store, exec); s.State != domain.StepRunning {
		t.Fatalf("a stopped Platform must leave the Step as it was, got %s", s.State)
	}

	// The new Platform reconciles, leases the Step again and the Provider re-attaches.
	p2 := startPlatform(t, store, svc.runner(true))
	waitUntil(t, "the Step to be leased again with its job", func() bool {
		svc.mu.Lock()
		defer svc.mu.Unlock()
		return len(svc.attached) == 1
	})
	svc.finish("job-1")
	waitUntil(t, "the Step to succeed", func() bool { return stepOf(t, store, exec).State == domain.StepSucceeded })
	_ = p2

	svc.mu.Lock()
	defer svc.mu.Unlock()
	if len(svc.submitted) != 1 || svc.attached[0] != "job-1" {
		t.Fatalf("the work was submitted %d times and attached to %v; want one submit and a re-attach to job-1", len(svc.submitted), svc.attached)
	}
}

func TestAJobThatIsNotResumableEndsInterruptedAndIsNotSubmittedAgain(t *testing.T) {
	store, svc := fakes.NewExecutionStore(), newFakeService()
	p1 := startPlatform(t, store, svc.runner(false))
	exec := submitJob(t, p1)
	waitUntil(t, "the remote job to be recorded", func() bool { return stepOf(t, store, exec).RemoteJob != nil })
	p1.stop()

	startPlatform(t, store, svc.runner(false))
	waitUntil(t, "the Step to be interrupted", func() bool { return stepOf(t, store, exec).State == domain.StepInterrupted })
	time.Sleep(150 * time.Millisecond)
	svc.mu.Lock()
	defer svc.mu.Unlock()
	if len(svc.submitted) != 1 || len(svc.attached) != 0 {
		t.Fatalf("submitted %d, attached %v: a job that cannot be re-attached must not be submitted again", len(svc.submitted), svc.attached)
	}
}
