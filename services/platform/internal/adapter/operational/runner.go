package operational

import (
	"context"
	"errors"

	"github.com/rluders/amaru/services/platform/internal/ports"
	"github.com/rluders/amaru/services/platform/internal/worker/embedded"
)

// RunnerRecord describes local Runner activity, not a committed Step result.
// Composition drains the queue through Logger.Write. It may enrich ProjectID
// from authoritative Execution state; Assignments do not carry a Project ID.
type RunnerRecord struct {
	Identity Identity
	Record   Record
}

// ObserveRunner instruments an existing Runner without importing logging into
// Worker execution logic. instanceID is trusted process identity. Assignment IDs
// supply attempt correlation, Execution and Step identity; inputs, remote-job
// metadata, Provider output, error strings and panic values are never inspected.
// Keep queue open until Worker attempts join. A nil/full queue drops diagnostics;
// no sink IO, callback or domain mutation runs on the attempt goroutine.
func ObserveRunner(instanceID string, next embedded.Runner, queue chan<- RunnerRecord) (embedded.Runner, error) {
	if instanceID == "" || next == nil {
		return nil, errors.New("Runner diagnostics require Instance identity and Runner")
	}
	return embedded.RunnerFunc(func(ctx context.Context, a ports.Assignment) (out embedded.Outcome) {
		id := Identity{InstanceID: instanceID, CorrelationID: a.AttemptID, ExecutionID: a.ExecutionID, StepID: a.StepID}
		emit := func(outcome string) {
			// Malformed internal Assignments must not create uncorrelatable records.
			if id.CorrelationID == "" || id.ExecutionID == "" || id.StepID == "" {
				return
			}
			select {
			case queue <- RunnerRecord{Identity: id, Record: Record{Operation: "worker.runner", Outcome: outcome}}:
			default:
			}
		}
		emit("started")
		returned := false
		defer func() {
			// Do not recover/format the panic: the Worker's existing panic boundary owns
			// its Result. Even a hostile String method must never run for diagnostics.
			outcome := "panicked"
			if returned {
				switch {
				case ctx.Err() == context.DeadlineExceeded && out.Result != ports.OutcomeSucceeded:
					outcome = "timed_out"
				case ctx.Err() != nil:
					outcome = "context_cancelled"
				case out.Result == ports.OutcomeSucceeded:
					outcome = "succeeded"
				case out.Result == ports.OutcomeFailed:
					outcome = "failed"
				case out.Result == ports.OutcomeCancelled:
					outcome = "cancelled"
				default:
					outcome = "unknown_outcome"
				}
			}
			emit(outcome)
		}()
		out = next.Run(ctx, a)
		returned = true
		return out
	}), nil
}
