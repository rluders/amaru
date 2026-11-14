package coordtest

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/rluders/amaru/services/platform/internal/coordinator"
	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/evidence"
	"github.com/rluders/amaru/services/platform/internal/ports"
	"github.com/rluders/amaru/services/platform/internal/ports/fakes"
)

// UsageSuite exercises identical delivery and restart cases on both adapters.
func UsageSuite(t *testing.T, factory func(*testing.T) ports.ExecutionStore) {
	ctx := context.Background()
	for _, outcome := range []ports.ResultOutcome{ports.OutcomeSucceeded, ports.OutcomeFailed, ports.OutcomeCancelled} {
		for _, faultAt := range []string{"none", "before", "after", "empty", "stale", "concurrent", "invalid"} {
			t.Run(map[ports.ResultOutcome]string{1: "success", 2: "failure", 3: "cancel"}[outcome]+"/"+faultAt, func(t *testing.T) {
				s := factory(t)
				e := domain.Execution{ID: "e1", ProjectID: "p1", EnvironmentID: "env1", State: domain.ExecQueued, CreatedAt: t0, RequestedBy: Requester, Trigger: domain.TriggerManual}
				e, steps, err := s.Create(ctx, e, []domain.StepExecution{{ID: "s1", ExecutionID: "e1", State: domain.StepQueued}})
				if err != nil {
					t.Fatal(err)
				}
				st := steps[0]
				if err := st.Lease("w1", "att-1", t0.Add(time.Hour)); err != nil {
					t.Fatal(err)
				}
				if err := st.Start(t0); err != nil {
					t.Fatal(err)
				}
				if err := e.Start(t0); err != nil {
					t.Fatal(err)
				}
				if _, err := s.Commit(ctx, ports.ExecutionChange{Execution: &e, Steps: []domain.StepExecution{st}}); err != nil {
					t.Fatal(err)
				}
				zero := uint64(0)
				m := ports.Result{AttemptID: "att-1", FencingToken: 1, Outcome: outcome, ErrorCode: "runner_failed", ErrorDetail: "token=abcdefghijk", Usage: []ports.UsageReport{{Provider: "fake", Model: "token=abcdefghijk", InputTokens: &ports.IntMeasurement{Source: ports.UsageReported, Value: &zero}, OutputTokens: &ports.IntMeasurement{Source: ports.UsageUnknown}, Cost: &ports.Cost{Source: ports.UsageEstimated, Amount: "0.001230", Currency: "USD", PriceTableVersion: "v1"}}}}
				deliver := func(store ports.ExecutionStore, m ports.Result, want ports.ResultDisposition, wantErr error) {
					t.Helper()
					run, cancel := context.WithTimeout(ctx, wait)
					defer cancel()
					acks := 0
					link := &protocolLink{messages: []ports.WorkerMessage{m}, send: func(msg ports.PlatformMessage) error {
						if ack, ok := msg.(ports.ResultAck); ok {
							acks++
							if want != 0 && ack.Disposition != want {
								t.Fatalf("ack=%v want=%v", ack.Disposition, want)
							}
							if want != ports.ResultRejectedStale {
								if _, err := store.ResultCapture(ctx, m.AttemptID); err != nil {
									t.Fatalf("ack before capture: %v", err)
								}
							}
							cancel()
						}
						return nil
					}}
					err := coordinator.New(store, clock, Permissive).Serve(run, link, "w1")
					if !errors.Is(err, wantErr) {
						t.Fatalf("serve=%v want=%v", err, wantErr)
					}
					if wantErr != nil && acks != 0 || wantErr == nil && acks != 1 {
						t.Fatalf("acks=%d", acks)
					}
				}
				if faultAt == "invalid" {
					m.Usage[0].InputTokens = &ports.IntMeasurement{Source: ports.UsageUnknown, Value: &zero}
					deliver(s, m, 0, domain.ErrInvalid)
					if _, err := s.ResultCapture(ctx, m.AttemptID); !errors.Is(err, domain.ErrNotFound) || readStep(t, s).State != domain.StepRunning {
						t.Fatal("invalid usage completed/captured")
					}
					return
				}
				if faultAt == "stale" {
					m.FencingToken = 2
					deliver(s, m, ports.ResultRejectedStale, nil)
					cs, err := s.UsageCaptures(ctx, "e1")
					if err != nil || len(cs) != 0 {
						t.Fatal("stale captured")
					}
					return
				}
				if faultAt == "empty" {
					m.Usage = nil
				}
				if faultAt == "before" || faultAt == "after" {
					fault := &boundaryStore{ExecutionStore: s}
					inject := func(int) error { return errCrash }
					if faultAt == "before" {
						fault.before = inject
					} else {
						fault.after = inject
					}
					deliver(fault, m, 0, errCrash)
					cs, err := s.UsageCaptures(ctx, "e1")
					if err != nil {
						t.Fatal(err)
					}
					if faultAt == "before" {
						if len(cs) != 0 || readStep(t, s).State != domain.StepRunning {
							t.Fatal("partial write")
						}
					} else {
						if len(cs) != 1 {
							t.Fatal("lost commit")
						}
					}
				}
				if faultAt == "concurrent" {
					var wg sync.WaitGroup
					for range 2 {
						wg.Add(1)
						go func() { defer wg.Done(); deliver(s, m, 0, nil) }()
					}
					wg.Wait()
				}
				want := ports.ResultAccepted
				if faultAt == "after" || faultAt == "concurrent" {
					want = ports.ResultDuplicate
				}
				deliver(s, m, want, nil)
				cs, err := s.UsageCaptures(ctx, "e1")
				if err != nil || len(cs) != 1 {
					t.Fatalf("captures=%v err=%v", cs, err)
				}
				c := cs[0]
				if c.ProjectID != "p1" || c.EnvironmentID != "env1" || c.StepID != "s1" || c.AttemptID != "att-1" {
					t.Fatal("attribution lost")
				}
				if len(c.Reports) > 0 {
					r := c.Reports[0]
					if r.Model != "token=[REDACTED]" || r.InputTokens.Value == nil || *r.InputTokens.Value != 0 || r.OutputTokens.Value != nil || r.Cost.Amount != "0.001230" {
						t.Fatal("redaction/precision/presence lost")
					}
				}
				events := readEvents(t, s)
				deliver(s, m, ports.ResultDuplicate, nil)
				next, _ := s.UsageCaptures(ctx, "e1")
				if !reflect.DeepEqual(cs, next) || !reflect.DeepEqual(events, readEvents(t, s)) {
					t.Fatal("replay changed evidence")
				}
				conflict := m
				conflict.Usage = append(append([]ports.UsageReport{}, m.Usage...), ports.UsageReport{})
				deliver(s, conflict, 0, domain.ErrInvalid)
				conflict = m
				conflict.Outcome = ports.OutcomeFailed
				if outcome == ports.OutcomeFailed {
					conflict.Outcome = ports.OutcomeSucceeded
				}
				deliver(s, conflict, 0, domain.ErrInvalid)
				b, err := (evidence.Reader{Executions: s, Contexts: fakes.NewContextStore(), Artifacts: fakes.NewArtifactIndex()}).Bundle(ctx, "e1")
				if err != nil || len(b.Steps) != 1 || len(b.Steps[0].Usage) != 1 {
					t.Fatalf("bundle usage: %v", err)
				}
				if faultAt == "empty" && b.Steps[0].UsageGap == "" {
					t.Fatal("unknown became zero")
				}
			})
		}
	}
}
