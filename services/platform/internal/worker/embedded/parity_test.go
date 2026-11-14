package embedded_test

import (
	"context"
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
	"github.com/rluders/amaru/services/platform/internal/worker/inproc"
)

// duplicateResults simulates repeated delivery without giving the Worker access to state.
type duplicateResults struct{ ports.WorkerLink }

func (l duplicateResults) Send(ctx context.Context, m ports.WorkerMessage) error {
	if err := l.WorkerLink.Send(ctx, m); err != nil {
		return err
	}
	if _, ok := m.(ports.Result); ok {
		return l.WorkerLink.Send(ctx, m)
	}
	return nil
}

type observeAcks struct {
	ports.PlatformLink
	acks chan ports.ResultAck
}

func (l observeAcks) Send(ctx context.Context, m ports.PlatformMessage) error {
	if err := l.PlatformLink.Send(ctx, m); err != nil {
		return err
	}
	if ack, ok := m.(ports.ResultAck); ok {
		select {
		case l.acks <- ack:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

// Both paths use the actual embedded Worker and fake Runner. The serialized path stands in for
// a separate Worker boundary; it does not validate a remote transport or enrollment.
func TestWorkerInterfaceParity(t *testing.T) {
	for _, transport := range []string{"inproc", "serialized"} {
		t.Run(transport, func(t *testing.T) {
			pair := func() (ports.PlatformLink, ports.WorkerLink) {
				pl, wl := inproc.Pair()
				if transport == "serialized" {
					return coordtest.SerializedLink(pl, wl)
				}
				return pl, wl
			}
			for _, tc := range []struct {
				name    string
				outcome ports.ResultOutcome
				state   domain.StepState
				event   string
			}{
				{"success", ports.OutcomeSucceeded, domain.StepSucceeded, domain.EventStepSucceeded},
				{"failure", ports.OutcomeFailed, domain.StepFailed, domain.EventStepFailed},
				{"cancel", ports.OutcomeCancelled, domain.StepCancelled, domain.EventStepCancelled},
			} {
				t.Run(tc.name, func(t *testing.T) {
					acks := make(chan ports.ResultAck, 4)
					runs := make(chan ports.Assignment, 2)
					stopped := make(chan struct{})
					runner := embedded.RunnerFunc(func(ctx context.Context, a ports.Assignment) embedded.Outcome {
						runs <- a
						if tc.outcome == ports.OutcomeCancelled {
							<-ctx.Done()
						}
						close(stopped)
						// A requested cancellation must override even a successful Runner return.
						outcome := tc.outcome
						if outcome == ports.OutcomeCancelled {
							outcome = ports.OutcomeSucceeded
						}
						return embedded.Outcome{Usage: []ports.UsageReport{{Provider: "fake", OutputTokens: &ports.IntMeasurement{Source: ports.UsageUnknown}}}, Result: outcome, ErrorCode: "fake_failure", ErrorDetail: "fake detail"}
					})
					r := newRigWithLink(t, &embedded.Worker{HeartbeatPeriod: time.Hour, Runner: runner}, time.Hour,
						func() (ports.PlatformLink, ports.WorkerLink) {
							pl, wl := pair()
							return observeAcks{pl, acks}, duplicateResults{wl}
						})
					got, _, err := r.c.Submit(r.ctx, coordinator.SubmitRequest{Key: "k1", By: coordtest.Requester,
						ProjectID: "p1", Steps: []coordinator.StepSpec{{Name: "s1", Capability: "code.test", Inputs: []byte(`{"n":1}`)}}})
					if err != nil {
						t.Fatal(err)
					}
					var a ports.Assignment
					select {
					case a = <-runs:
					case <-time.After(5 * time.Second):
						t.Fatal("Runner did not start")
					}
					if a.ExecutionID != got.Execution.ID || a.StepID != got.Execution.ID+".s1" ||
						a.Capability != "code.test" || string(a.InputsJSON) != `{"n":1}` ||
						a.IdempotencyKey != a.ExecutionID+"/"+a.StepID || a.Lease.FencingToken != 1 {
						t.Fatalf("assignment: %+v", a)
					}
					if tc.outcome == ports.OutcomeCancelled {
						r.until("running state", func() bool { return stepOf(t, r.store, a.ExecutionID).State == domain.StepRunning })
						if _, _, err := r.c.Cancel(r.ctx, coordinator.CancelRequest{ActorID: "tester", Key: "cancel", ExecutionID: a.ExecutionID,
							By: domain.Actor{Kind: domain.ActorHuman, Authority: "owner"}}); err != nil {
							t.Fatal(err)
						}
					}
					for _, disposition := range []ports.ResultDisposition{ports.ResultAccepted, ports.ResultDuplicate} {
						select {
						case ack := <-acks:
							if ack.AttemptID != a.AttemptID || ack.Disposition != disposition {
								t.Fatalf("ack: %+v, want %v", ack, disposition)
							}
						case <-time.After(5 * time.Second):
							t.Fatal("missing result acknowledgement")
						}
					}
					select {
					case <-stopped:
					default:
						t.Fatal("result acknowledged before Runner stopped")
					}
					capture, err := r.store.ResultCapture(r.ctx, a.AttemptID)
					if err != nil || len(capture.Reports) != 1 || capture.Reports[0].Provider != "fake" || capture.Reports[0].OutputTokens.Value != nil {
						t.Fatalf("embedded usage lost: %v", err)
					}
					st := stepOf(t, r.store, a.ExecutionID)
					if st.State != tc.state || st.CurrentLease != nil {
						t.Fatalf("step: %+v", st)
					}
					if tc.outcome == ports.OutcomeFailed && (st.Error == nil || st.Error.Code != "fake_failure" || st.Error.Message != "fake detail") {
						t.Fatalf("error: %+v", st.Error)
					}
					evs, err := r.store.Events(r.ctx, a.ExecutionID)
					if err != nil {
						t.Fatal(err)
					}
					count := 0
					for _, ev := range evs {
						if ev.Kind == tc.event {
							count++
						}
					}
					if count != 1 {
						t.Fatalf("terminal events: %d, want 1", count)
					}
					select {
					case extra := <-runs:
						t.Fatalf("duplicate Runner invocation: %+v", extra)
					default:
					}
					if tc.outcome == ports.OutcomeCancelled {
						e, _, err := r.store.Get(r.ctx, a.ExecutionID)
						if err != nil || e.State != domain.ExecCancelled {
							t.Fatalf("execution: %+v, %v", e, err)
						}
					}
				})
			}
			for _, idempotent := range []bool{false, true} {
				name := "restart_non_idempotent"
				if idempotent {
					name = "restart_idempotent"
				}
				t.Run(name, func(t *testing.T) {
					store := fakes.NewExecutionStore()
					started := make(chan ports.Assignment, 1)
					var stopped atomic.Bool
					p1 := startPlatformWithLink(t, store, embedded.RunnerFunc(func(ctx context.Context, a ports.Assignment) embedded.Outcome {
						started <- a
						<-ctx.Done()
						stopped.Store(true)
						return embedded.Outcome{Result: ports.OutcomeFailed}
					}), pair)
					got, _, err := p1.c.Submit(context.Background(), coordinator.SubmitRequest{Key: "k1", By: coordtest.Requester,
						ProjectID: "p1", Steps: []coordinator.StepSpec{{Name: "s1", Capability: "code.test", Idempotent: idempotent}}})
					if err != nil {
						t.Fatal(err)
					}
					var first ports.Assignment
					select {
					case first = <-started:
					case <-time.After(5 * time.Second):
						t.Fatal("first Runner did not start")
					}
					waitUntil(t, "durable running state", func() bool { return stepOf(t, store, got.Execution.ID).State == domain.StepRunning })
					p1.stop()
					before := stepOf(t, store, got.Execution.ID)
					if !stopped.Load() || before.State != domain.StepRunning || before.CurrentLease == nil || !before.CurrentLease.ExpiresAt.After(time.Now()) {
						t.Fatalf("stopped Worker must leave a live Lease for startup reconciliation: %+v", before)
					}
					afterRestart := make(chan ports.Assignment, 1)
					p2 := startPlatformWithLink(t, store, embedded.RunnerFunc(func(_ context.Context, a ports.Assignment) embedded.Outcome {
						afterRestart <- a
						return embedded.Outcome{Result: ports.OutcomeSucceeded}
					}), pair)
					if idempotent {
						select {
						case second := <-afterRestart:
							if second.AttemptID == first.AttemptID || second.Lease.FencingToken <= first.Lease.FencingToken || second.IdempotencyKey != first.IdempotencyKey {
								t.Fatalf("restart assignment: %+v, first %+v", second, first)
							}
						case <-time.After(5 * time.Second):
							t.Fatal("restart waited for the remote-heartbeat expiry")
						}
						waitUntil(t, "recovered Step success", func() bool { return stepOf(t, store, got.Execution.ID).State == domain.StepSucceeded })
					} else {
						st := stepOf(t, store, got.Execution.ID)
						e, _, err := store.Get(context.Background(), got.Execution.ID)
						if err != nil || st.State != domain.StepInterrupted || st.Reason != domain.ReasonPlatformRestarted || e.State != domain.ExecWaiting {
							t.Fatalf("step %+v, execution %+v, error %v", st, e, err)
						}
					}
					p2.stop()
					select {
					case extra := <-afterRestart:
						t.Fatalf("unexpected invocation: %+v", extra)
					default:
					}
					settled := stepOf(t, store, got.Execution.ID)
					events, err := store.Events(context.Background(), got.Execution.ID)
					if err != nil {
						t.Fatal(err)
					}
					if _, err := coordinator.New(store, time.Now, coordtest.Permissive).Reconcile(context.Background()); err != nil {
						t.Fatal(err)
					}
					again := stepOf(t, store, got.Execution.ID)
					nextEvents, err := store.Events(context.Background(), got.Execution.ID)
					if err != nil {
						t.Fatal(err)
					}
					if !reflect.DeepEqual(settled, again) || !reflect.DeepEqual(events, nextEvents) {
						t.Fatal("second reconciliation changed settled state/events")
					}
				})
			}
		})
	}
}
