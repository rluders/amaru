//go:build !windows

package osproc

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/rluders/amaru/services/platform/internal/coordinator"
	"github.com/rluders/amaru/services/platform/internal/coordinator/coordtest"
	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
	"github.com/rluders/amaru/services/platform/internal/ports/fakes"
	"github.com/rluders/amaru/services/platform/internal/worker/embedded"
	"github.com/rluders/amaru/services/platform/internal/worker/inproc"
)

// The whole path of FEAT-EXE-003 with nothing faked but the program: a person cancels an
// Execution, the coordinator tells the embedded Worker, the Worker ends its Runner's context, the
// OS layer ends the real process tree within the grace period, and the Worker reports it. The
// "program" is this test binary re-executed as a helper (a process that starts a grandchild).
func TestCancelEndsTheProcessTreeOfARunningStep(t *testing.T) {
	for _, mode := range []string{"tree", "stubborn-tree"} {
		t.Run(mode, func(t *testing.T) {
			const grace = 500 * time.Millisecond
			pids := make(chan int, 1)
			runner := embedded.RunnerFunc(func(ctx context.Context, a ports.Assignment) embedded.Outcome {
				exe, _ := os.Executable()
				p, err := Runner{Grace: grace, Environ: func() []string { return nil }}.Start(ctx,
					ports.ProcSpec{Path: exe, Dir: t.TempDir(), Env: []string{helperEnv + "=" + mode}})
				if err != nil {
					return embedded.Outcome{Result: ports.OutcomeFailed, ErrorCode: "start_failed", ErrorDetail: err.Error()}
				}
				pids <- grandchildPID(t, p)
				p.Wait() // returns once the root has ended, which is after the context ended it
				return embedded.Outcome{Result: ports.OutcomeFailed, ErrorCode: "killed"}
			})

			store := fakes.NewExecutionStore()
			c := coordinator.New(store, time.Now, coordtest.Permissive)
			pl, wl := inproc.Pair()
			ctx, stop := context.WithCancel(context.Background())
			var wg sync.WaitGroup
			wg.Add(2)
			go func() { defer wg.Done(); _ = c.Serve(ctx, pl, "embedded") }()
			go func() {
				defer wg.Done()
				_ = (&embedded.Worker{HeartbeatPeriod: time.Hour, Runner: runner}).Run(ctx, wl)
			}()
			t.Cleanup(func() { stop(); pl.Close(); wg.Wait() })

			got, _, err := c.Submit(ctx, coordinator.SubmitRequest{By: coordtest.Requester, Key: "k1", ProjectID: "p1", Steps: []coordinator.StepSpec{{Name: "run", Capability: "code.test"}}})
			if err != nil {
				t.Fatal(err)
			}
			var grandchild int
			select {
			case grandchild = <-pids:
			case <-time.After(5 * time.Second):
				t.Fatal("the Step never started a process")
			}
			if !alive(grandchild) {
				t.Fatal("the grandchild is not running")
			}

			began := time.Now()
			out, _, err := c.Cancel(ctx, coordinator.CancelRequest{Key: "c1", ExecutionID: got.Execution.ID, Reason: "stop", By: domain.Actor{Kind: domain.ActorHuman}})
			if err != nil || len(out.Stopping) != 1 {
				t.Fatalf("%+v %v", out, err)
			}
			eventually(t, 10*time.Second, "the Execution to be cancelled", func() bool {
				e, _, _ := store.Get(ctx, got.Execution.ID)
				return e.State == domain.ExecCancelled
			})
			eventually(t, 5*time.Second, "the grandchild to end", func() bool { return !alive(grandchild) })
			if mode == "stubborn-tree" && time.Since(began) < grace {
				t.Fatalf("a tree that ignores the polite signal ended after %s, before the %s grace period", time.Since(began), grace)
			}
			_, steps, _ := store.Get(ctx, got.Execution.ID)
			if steps[0].State != domain.StepCancelled {
				t.Fatalf("step %s", steps[0].State)
			}
		})
	}
}
