package operational

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rluders/amaru/services/platform/internal/ports"
	"github.com/rluders/amaru/services/platform/internal/worker/embedded"
	"github.com/rluders/amaru/services/platform/internal/worker/inproc"
)

func TestRunnerDiagnosticsPreserveOutcomeAndOmitPayloads(t *testing.T) {
	for _, tc := range []struct {
		result ports.ResultOutcome
		want   string
	}{{ports.OutcomeSucceeded, "succeeded"}, {ports.OutcomeFailed, "failed"}, {ports.OutcomeCancelled, "cancelled"}, {0, "unknown_outcome"}} {
		t.Run(tc.want, func(t *testing.T) {
			secret := "private-provider-fixture"
			q := make(chan RunnerRecord, 2)
			a := ports.Assignment{AttemptID: "attempt", ExecutionID: "execution", StepID: "step", InputsJSON: []byte(secret), IdempotencyKey: secret, RemoteJob: &ports.RemoteJob{ID: secret}}
			want := embedded.Outcome{Result: tc.result, ErrorCode: secret, ErrorDetail: secret}
			ctx := context.Background()
			next := embedded.RunnerFunc(func(got context.Context, gotA ports.Assignment) embedded.Outcome {
				if got != ctx || !reflect.DeepEqual(gotA, a) {
					t.Fatal("Runner arguments changed")
				}
				return want
			})
			runner, err := ObserveRunner("instance", next, q)
			if err != nil {
				t.Fatal(err)
			}
			if out := runner.Run(ctx, a); !reflect.DeepEqual(out, want) {
				t.Fatal("Runner outcome changed")
			}
			var buf bytes.Buffer
			logger, _ := New(&buf) // No registered secret: payloads must be omitted.
			for _, outcome := range []string{"started", tc.want} {
				rec := <-q
				if rec.Identity != (Identity{InstanceID: "instance", CorrelationID: "attempt", ExecutionID: "execution", StepID: "step"}) || rec.Record.Outcome != outcome {
					t.Fatal("incorrect identity or outcome")
				}
				if err := logger.Write(WithIdentity(ctx, rec.Identity), rec.Record); err != nil {
					t.Fatal(err)
				}
			}
			if len(records(t, &buf)) != 2 || strings.Contains(buf.String(), secret) {
				t.Fatal("missing JSON or leaked payload")
			}
		})
	}
}

type unformattablePanic struct{ t *testing.T }

func (p *unformattablePanic) String() string {
	p.t.Error("diagnostics formatted panic")
	return "private panic"
}
func TestRunnerDiagnosticsPreservePanic(t *testing.T) {
	q := make(chan RunnerRecord, 2)
	value := &unformattablePanic{t}
	runner, _ := ObserveRunner("instance", embedded.RunnerFunc(func(context.Context, ports.Assignment) embedded.Outcome { panic(value) }), q)
	func() {
		defer func() {
			if recover() != value {
				t.Error("panic changed")
			}
		}()
		runner.Run(context.Background(), ports.Assignment{AttemptID: "a", ExecutionID: "e", StepID: "s"})
	}()
	if (<-q).Record.Outcome != "started" || (<-q).Record.Outcome != "panicked" {
		t.Fatal("missing panic diagnostic")
	}
}
func TestRunnerDiagnosticsNeverWaitForConsumer(t *testing.T) {
	next := embedded.RunnerFunc(func(context.Context, ports.Assignment) embedded.Outcome {
		return embedded.Outcome{Result: ports.OutcomeSucceeded}
	})
	for _, q := range []chan RunnerRecord{nil, make(chan RunnerRecord), make(chan RunnerRecord, 1)} {
		runner, _ := ObserveRunner("instance", next, q)
		done := make(chan struct{})
		go func() {
			defer close(done)
			runner.Run(context.Background(), ports.Assignment{AttemptID: "a", ExecutionID: "e", StepID: "s"})
		}()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("diagnostics blocked Runner")
		}
	}
	if _, err := ObserveRunner("", next, nil); err == nil {
		t.Fatal("missing Instance accepted")
	}
	if _, err := ObserveRunner("instance", nil, nil); err == nil {
		t.Fatal("missing Runner accepted")
	}
	q := make(chan RunnerRecord, 2)
	runner, _ := ObserveRunner("instance", next, q)
	runner.Run(context.Background(), ports.Assignment{})
	if len(q) != 0 {
		t.Fatal("uncorrelatable record")
	}
}

// Actual Worker and in-process transport, fake Provider. Process shutdown must
// not become a synthetic Result; explicit Cancel retains its existing Result.
func TestRunnerDiagnosticsWithWorker(t *testing.T) {
	for _, mode := range []string{"success", "failure", "panic", "cancel", "shutdown", "deadline"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			p, w := inproc.Pair()
			defer p.Close()
			entered := make(chan struct{})
			q := make(chan RunnerRecord, 2)
			secret := "provider-sensitive-fixture"
			next := embedded.RunnerFunc(func(ctx context.Context, a ports.Assignment) embedded.Outcome {
				close(entered)
				switch mode {
				case "panic":
					panic(secret)
				case "cancel", "shutdown", "deadline":
					<-ctx.Done()
					return embedded.Outcome{Result: ports.OutcomeFailed, ErrorDetail: secret}
				case "failure":
					return embedded.Outcome{Result: ports.OutcomeFailed, ErrorCode: "unavailable", ErrorDetail: secret}
				}
				return embedded.Outcome{Result: ports.OutcomeSucceeded}
			})
			runner, _ := ObserveRunner("instance", next, q)
			worker := embedded.Worker{Runner: runner}
			done := make(chan error, 1)
			go func() { done <- worker.Run(ctx, w) }()
			receive := func() ports.WorkerMessage {
				m, err := p.Receive(ctx)
				if err != nil {
					t.Fatal("Worker receive failed")
				}
				return m
			}
			if _, ok := receive().(ports.Offer); !ok {
				t.Fatal("missing Offer")
			}
			a := ports.Assignment{AttemptID: "a", ExecutionID: "e", StepID: "s", Lease: ports.Lease{FencingToken: 7}}
			if mode == "deadline" {
				a.Deadline = time.Now().Add(-time.Second)
			}
			if err := p.Send(ctx, a); err != nil {
				t.Fatal(err)
			}
			if _, ok := receive().(ports.AcceptAttempt); !ok {
				t.Fatal("missing Accept")
			}
			select {
			case <-entered:
			case <-ctx.Done():
				t.Fatal("Runner did not enter")
			}
			if mode == "cancel" {
				if err := p.Send(ctx, ports.Cancel{AttemptID: "a"}); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "shutdown" {
				cancel()
			} else {
				result, ok := receive().(ports.Result)
				if !ok {
					t.Fatal("missing Result")
				}
				want := ports.OutcomeSucceeded
				switch mode {
				case "failure", "panic", "deadline":
					want = ports.OutcomeFailed
				case "cancel":
					want = ports.OutcomeCancelled
				}
				if result.Outcome != want || result.AttemptID != "a" || result.FencingToken != 7 {
					t.Fatal("Worker Result changed")
				}
				if mode == "deadline" && result.ErrorCode != "timeout" {
					t.Fatal("timeout classification changed")
				}
				cancel()
			}
			select {
			case err := <-done:
				if err != nil {
					t.Fatal("Worker failed")
				}
			case <-time.After(time.Second):
				t.Fatal("Worker did not join")
			}
			if err := p.Close(); err != nil {
				t.Fatal(err)
			}
			for {
				m, err := p.Receive(context.Background())
				if errors.Is(err, ports.ErrLinkClosed) {
					break
				}
				if err != nil {
					t.Fatal(err)
				}
				if _, ok := m.(ports.Result); ok {
					t.Fatal("unexpected extra Result")
				}
			}
			var buf bytes.Buffer
			logger, _ := New(&buf)
			if len(q) != 2 {
				t.Fatal("missing Runner diagnostics")
			}
			last := ""
			for range 2 {
				rec := <-q
				last = rec.Record.Outcome
				if err := logger.Write(WithIdentity(context.Background(), rec.Identity), rec.Record); err != nil {
					t.Fatal(err)
				}
			}
			expected := map[string]string{"success": "succeeded", "failure": "failed", "panic": "panicked", "cancel": "context_cancelled", "shutdown": "context_cancelled", "deadline": "timed_out"}[mode]
			if last != expected {
				t.Fatal("incorrect Runner classification")
			}
			if strings.Contains(buf.String(), secret) || len(records(t, &buf)) != 2 {
				t.Fatal("secret leaked or invalid JSON")
			}
		})
	}
}
