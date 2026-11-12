package embedded_test

import (
	"context"
	"reflect"
	"sync"
	"sync/atomic"
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

// Real coordinator/Worker dispatch with fake Provider outcomes and storage; no vendor is called.
func TestProjectsProgressIndependently(t *testing.T) {
	for _, transport := range []string{"inproc", "serialized"} {
		for _, failure := range []string{"provider_failure", "provider_unavailable", "panic"} {
			t.Run(transport+"/"+failure, func(t *testing.T) {
				ctx := context.Background()
				store := fakes.NewExecutionStore()
				var badRuns, goodRuns, forbiddenRuns atomic.Int64
				acks := make(chan ports.ResultAck, 6)
				checkAcks := func() {
					for _, want := range []ports.ResultDisposition{ports.ResultAccepted, ports.ResultDuplicate} {
						select {
						case ack := <-acks:
							if ack.Disposition != want {
								t.Fatalf("result disposition %v, want %v", ack.Disposition, want)
							}
						case <-time.After(5 * time.Second):
							t.Fatal("missing result acknowledgement")
						}
					}
				}
				runner := embedded.RunnerFunc(func(_ context.Context, a ports.Assignment) embedded.Outcome {
					switch a.Capability {
					case "check.run":
						badRuns.Add(1)
						if failure == "panic" {
							panic("fake Provider panic")
						}
						return embedded.Outcome{Result: ports.OutcomeFailed, ErrorCode: failure, ErrorDetail: "fake Provider unavailable or failed"}
					case "code.test":
						goodRuns.Add(1)
						return embedded.Outcome{Result: ports.OutcomeSucceeded}
					default:
						forbiddenRuns.Add(1)
						return embedded.Outcome{Result: ports.OutcomeFailed, ErrorCode: "unexpected_capability"}
					}
				})
				start := func() (*coordinator.Coordinator, func()) {
					// Trust is a test precondition; default Policy still requires Approval for git.push.
					c := coordinator.New(store, time.Now, coordtest.NewOpenTrust().Option())
					if _, err := c.Reconcile(ctx); err != nil {
						t.Fatal(err)
					}
					pl, wl := inproc.Pair()
					if transport == "serialized" {
						pl, wl = coordtest.SerializedLink(pl, wl)
					}
					runCtx, cancel := context.WithCancel(ctx)
					var wg sync.WaitGroup
					wg.Add(2)
					go func() {
						defer wg.Done()
						if err := c.Serve(runCtx, observeAcks{pl, acks}, "embedded"); err != nil {
							t.Errorf("Serve: %v", err)
						}
					}()
					go func() {
						defer wg.Done()
						if err := (&embedded.Worker{Slots: 1, HeartbeatPeriod: time.Hour, Runner: runner}).Run(runCtx, duplicateResults{wl}); err != nil {
							t.Errorf("Worker: %v", err)
						}
					}()
					stop := func() { cancel(); pl.Close(); wg.Wait() }
					t.Cleanup(stop)
					return c, stop
				}
				submit := func(c *coordinator.Coordinator, key, project, capability string) coordinator.Submitted {
					got, replay, err := c.Submit(ctx, coordinator.SubmitRequest{Key: key, By: coordtest.Requester, ProjectID: project,
						Steps: []coordinator.StepSpec{{Name: "same-step", Capability: capability}}})
					if err != nil || replay {
						t.Fatalf("submit %s: replay %v, %v", key, replay, err)
					}
					return got
				}
				c, stop := start()
				waiting := submit(c, "wait", "affected", "git.push")
				if len(waiting.Approvals) != 1 || waiting.Steps[0].State != domain.StepAwaitingApproval {
					t.Fatal("git.push did not wait for Approval")
				}
				initialApprovals, err := store.Approvals(ctx, waiting.Execution.ID)
				if err != nil {
					t.Fatal(err)
				}
				bad := submit(c, "bad", "affected", "check.run")
				if bad.Steps[0].State != domain.StepQueued {
					t.Fatalf("failure probe was not eligible: %+v", bad.Steps[0])
				}
				waitUntil(t, "affected Project failure", func() bool { return stepOf(t, store, bad.Execution.ID).State == domain.StepFailed })
				checkAcks()
				failed := stepOf(t, store, bad.Execution.ID)
				wantCode := failure
				if failure == "panic" {
					wantCode = "runner_panic"
				}
				if failed.Error == nil || failed.Error.Code != wantCode {
					t.Fatalf("missing failure reason: %+v", failed.Error)
				}
				good := submit(c, "good", "healthy", "code.test")
				waitUntil(t, "healthy Project success", func() bool { return stepOf(t, store, good.Execution.ID).State == domain.StepSucceeded })
				checkAcks()
				stop()
				beforeWait := stepOf(t, store, waiting.Execution.ID)
				beforeBad := stepOf(t, store, bad.Execution.ID)
				beforeGood := stepOf(t, store, good.Execution.ID)
				approvals, err := store.Approvals(ctx, waiting.Execution.ID)
				if err != nil || !reflect.DeepEqual(approvals, initialApprovals) || beforeWait.State != domain.StepAwaitingApproval || beforeWait.AttemptID != "" {
					t.Fatalf("waiting Project changed: %+v, %v", approvals, err)
				}
				events := func(id string) []domain.ExecutionEvent {
					evs, err := store.Events(ctx, id)
					if err != nil {
						t.Fatal(err)
					}
					return evs
				}
				waitEvents, badEvents, goodEvents := events(waiting.Execution.ID), events(bad.Execution.ID), events(good.Execution.ID)
				// Restart over retained fake state, then run more healthy work while Approval still waits.
				c, stop = start()
				next := submit(c, "next", "healthy", "code.test")
				waitUntil(t, "healthy Project success after restart", func() bool { return stepOf(t, store, next.Execution.ID).State == domain.StepSucceeded })
				checkAcks()
				stop()
				for _, snapshot := range []struct {
					id     string
					step   domain.StepExecution
					events []domain.ExecutionEvent
				}{
					{waiting.Execution.ID, beforeWait, waitEvents},
					{bad.Execution.ID, beforeBad, badEvents},
					{good.Execution.ID, beforeGood, goodEvents},
				} {
					if !reflect.DeepEqual(stepOf(t, store, snapshot.id), snapshot.step) || !reflect.DeepEqual(events(snapshot.id), snapshot.events) {
						t.Fatalf("restart or unrelated work changed %s", snapshot.id)
					}
				}
				approvals, err = store.Approvals(ctx, waiting.Execution.ID)
				if err != nil || !reflect.DeepEqual(approvals, initialApprovals) {
					t.Fatalf("restart changed Approval: %+v, %v", approvals, err)
				}
				if badRuns.Load() != 1 || goodRuns.Load() != 2 || forbiddenRuns.Load() != 0 {
					t.Fatalf("Runner calls: bad=%d healthy=%d forbidden=%d", badRuns.Load(), goodRuns.Load(), forbiddenRuns.Load())
				}
				for _, run := range []struct {
					got   coordinator.Submitted
					state domain.StepState
					kind  string
				}{
					{bad, domain.StepFailed, domain.EventStepFailed},
					{good, domain.StepSucceeded, domain.EventStepSucceeded},
					{next, domain.StepSucceeded, domain.EventStepSucceeded},
				} {
					e, steps, err := store.Get(ctx, run.got.Execution.ID)
					if err != nil || e.ProjectID != run.got.Execution.ProjectID || len(steps) != 1 || steps[0].State != run.state {
						t.Fatalf("Execution: %+v, %v", e, err)
					}
					count := 0
					for _, ev := range events(e.ID) {
						if ev.Kind == run.kind {
							count++
						}
					}
					if count != 1 {
						t.Fatalf("%s: %d terminal Step events, want one", e.ID, count)
					}
				}
			})
		}
	}
}
