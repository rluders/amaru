package platform

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rluders/amaru/services/platform/internal/adapter/operational"
	"github.com/rluders/lane/v2"
)

type diagnosticRunner struct{ dependent }

func (diagnosticRunner) Name() string { return "raw-stage-unregistered" }

func TestLifecycleDiagnosticComposition(t *testing.T) {
	for _, mode := range []string{"buffered", "full", "nil"} {
		for _, failure := range []string{"start", "readiness", "panic", "stop", "runtime", "cancel"} {
			t.Run(mode+"/"+failure, func(t *testing.T) {
				var queue chan operational.Record
				sentinel := operational.Record{Operation: "test.sentinel", Outcome: "info"}
				if mode == "buffered" {
					queue = make(chan operational.Record, 128)
				}
				if mode == "full" {
					queue = make(chan operational.Record, 1)
					queue <- sentinel
				}
				l, err := NewLifecycle(time.Now().Add(2*time.Second), 250*time.Millisecond, queue)
				if err != nil {
					t.Fatal(err)
				}
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				ctx = operational.WithIdentity(ctx, operational.Identity{InstanceID: "raw-producer-instance", CorrelationID: "raw-producer-correlation"})
				boom := errors.New("raw-error-unregistered opaque-fixture-credential")
				var joined, stopped, admitted, closedOnCancel atomic.Bool
				runner := diagnosticRunner{dependent{
					start: func(ctx context.Context) error {
						defer joined.Store(true)
						switch failure {
						case "start":
							return boom
						case "panic":
							panic("raw-panic-unregistered opaque-fixture-credential")
						case "runtime":
							for !l.Ready() {
								select {
								case <-ctx.Done():
									return ctx.Err()
								case <-time.After(time.Millisecond):
								}
							}
							return boom
						}
						<-ctx.Done()
						return nil
					},
					ready: func(ctx context.Context) error {
						if failure == "start" || failure == "panic" {
							<-ctx.Done()
							return ctx.Err()
						}
						if failure == "readiness" {
							return boom
						}
						return nil
					},
					stop: func(context.Context) error {
						stopped.Store(true)
						if failure == "stop" {
							return boom
						}
						return nil
					},
				}}
				peer := dependent{start: func(ctx context.Context) error {
					admitted.Store(true)
					if failure == "stop" || failure == "cancel" {
						cancel()
					}
					<-ctx.Done()
					closedOnCancel.Store(!l.Ready())
					return nil
				}}
				done := make(chan error, 1)
				go func() { done <- l.Run(ctx, []lane.Runner{runner}, []lane.Runner{peer}) }()
				select {
				case err = <-done:
				case <-time.After(3 * time.Second):
					t.Fatal("diagnostics blocked lifecycle completion")
				}
				var panicErr *lane.PanicError
				if (err == nil) != (failure == "cancel") || errors.As(err, &panicErr) != (failure == "panic") {
					t.Fatal("failure classification changed")
				}
				if failure != "panic" && failure != "cancel" && !errors.Is(err, boom) {
					t.Fatal("underlying failure lost")
				}
				if !joined.Load() || !stopped.Load() || l.Ready() {
					t.Fatal("cleanup incomplete or admission open")
				}
				if failure == "start" || failure == "readiness" || failure == "panic" {
					if admitted.Load() {
						t.Fatal("failed startup admitted dependent")
					}
				} else if !admitted.Load() || !closedOnCancel.Load() {
					t.Fatal("admission/cancellation ordering changed")
				}
				// Run and cooperative producers have joined; only now close and drain.
				// No consumer runs during Lifecycle execution, including the full queue case.
				if queue == nil {
					return
				}
				close(queue)
				var sink bytes.Buffer
				logger, err := operational.New(&sink, "opaque-fixture-credential")
				if err != nil {
					t.Fatal(err)
				}
				trusted := operational.WithIdentity(context.Background(), operational.Identity{InstanceID: "trusted-instance", CorrelationID: "trusted-lifecycle"})
				count, foundError := 0, false
				for rec := range queue {
					count++
					if mode == "full" {
						if rec != sentinel {
							t.Fatal("full queue contents changed")
						}
						continue
					}
					if rec.Operation != "process.lifecycle" || rec.Cause != "" || rec.Status != 0 {
						t.Fatal("raw lifecycle data reached queue")
					}
					switch rec.Outcome {
					case "debug", "info", "warning", "error":
					default:
						t.Fatal("unexpected outcome")
					}
					foundError = foundError || rec.Outcome == "error"
					if err := logger.Write(trusted, rec); err != nil {
						t.Fatal(err)
					}
				}
				if mode == "full" {
					if count != 1 {
						t.Fatal("full queue lost sentinel")
					}
					return
				}
				if count == 0 || count == cap(queue) || (failure != "cancel" && !foundError) {
					t.Fatal("missing diagnostics or insufficient capacity")
				}
				for _, raw := range []string{"raw-stage-unregistered", "raw-error-unregistered", "raw-panic-unregistered", "opaque-fixture-credential", "raw-producer", "stack"} {
					if strings.Contains(sink.String(), raw) {
						t.Fatal("raw data reached JSON sink")
					}
				}
				lines := bytes.Split(bytes.TrimSpace(sink.Bytes()), []byte("\n"))
				if len(lines) != count {
					t.Fatal("JSON count differs from drained queue")
				}
				for _, line := range lines {
					var record map[string]any
					if err := json.Unmarshal(line, &record); err != nil {
						t.Fatal(err)
					}
					if len(record) != 5 || record["instanceId"] != "trusted-instance" || record["correlationId"] != "trusted-lifecycle" || record["operation"] != "process.lifecycle" {
						t.Fatal("unexpected fields or untrusted identity")
					}
					timestamp, ok := record["time"].(string)
					if !ok {
						t.Fatal("missing timestamp")
					}
					if _, err := time.Parse(time.RFC3339Nano, timestamp); err != nil {
						t.Fatal("invalid timestamp")
					}
				}
			})
		}
	}
}
