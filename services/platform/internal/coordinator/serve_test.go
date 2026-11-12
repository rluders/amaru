package coordinator_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/rluders/amaru/services/platform/internal/coordinator"
	"github.com/rluders/amaru/services/platform/internal/coordinator/coordtest"
	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
	"github.com/rluders/amaru/services/platform/internal/ports/fakes"
)

type shutdownStore struct {
	ports.ExecutionStore
	access func(context.Context) error
}

func (s shutdownStore) Queued(ctx context.Context, _ int) ([]ports.StepRef, error) {
	return nil, s.access(ctx)
}
func (s shutdownStore) ByAttempt(ctx context.Context, _ string) (ports.StepRef, error) {
	return ports.StepRef{}, s.access(ctx)
}

type shutdownLink struct {
	message ports.WorkerMessage
	send    func(context.Context) error
}

func (l *shutdownLink) Receive(ctx context.Context) (ports.WorkerMessage, error) {
	if l.message != nil {
		m := l.message
		l.message = nil
		return m, nil
	}
	<-ctx.Done()
	return nil, ctx.Err()
}
func (l *shutdownLink) Send(ctx context.Context, _ ports.PlatformMessage) error { return l.send(ctx) }
func (*shutdownLink) Close() error                                              { return nil }

func TestServeShutdownDuringStoreAccess(t *testing.T) {
	failure := errors.New("store unavailable")
	for _, message := range []ports.WorkerMessage{
		ports.Offer{FreeSlots: 1}, ports.AcceptAttempt{}, ports.RejectAttempt{},
		ports.LeaseRenew{}, ports.StepEvents{}, ports.Result{},
	} {
		for _, mode := range []string{"cancelled", "unrelated error during cancellation", "foreign cancellation"} {
			t.Run(fmt.Sprintf("%T/%s", message, mode), func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				entered := make(chan struct{})
				done := make(chan error, 1)
				store := shutdownStore{ExecutionStore: fakes.NewExecutionStore(), access: func(ctx context.Context) error {
					if mode == "foreign cancellation" {
						return fmt.Errorf("store: %w", context.Canceled)
					}
					close(entered)
					<-ctx.Done()
					if mode == "unrelated error during cancellation" {
						return failure
					}
					return fmt.Errorf("store: %w", ctx.Err())
				}}
				link := &shutdownLink{message: message, send: func(context.Context) error { t.Error("unexpected send after failed store access"); return nil }}
				go func() { done <- coordinator.New(store, time.Now).Serve(ctx, link, "w1") }()
				if mode != "foreign cancellation" {
					select {
					case <-entered:
						cancel()
					case <-time.After(5 * time.Second):
						t.Fatal("store was not entered")
					}
				}
				select {
				case err := <-done:
					switch mode {
					case "cancelled":
						if err != nil {
							t.Fatalf("normal shutdown: %v", err)
						}
					case "unrelated error during cancellation":
						if !errors.Is(err, failure) {
							t.Fatalf("lost store error: %v", err)
						}
					case "foreign cancellation":
						if !errors.Is(err, context.Canceled) {
							t.Fatalf("lost foreign cancellation: %v", err)
						}
					}
				case <-time.After(5 * time.Second):
					t.Fatal("Serve did not return")
				}
			})
		}
	}
}

func TestServeShutdownDuringSend(t *testing.T) {
	failure := errors.New("transport failed")
	for _, assignment := range []bool{false, true} {
		for _, unrelated := range []bool{false, true} {
			t.Run(fmt.Sprintf("assignment=%v/unrelated=%v", assignment, unrelated), func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				store := fakes.NewExecutionStore()
				var message ports.WorkerMessage = ports.Result{}
				if assignment {
					_, _, err := store.Create(ctx,
						domain.Execution{ID: "e1", State: domain.ExecQueued, RequestedBy: coordtest.Requester, CreatedAt: time.Now(), Trigger: domain.TriggerManual},
						[]domain.StepExecution{{ID: "s1", ExecutionID: "e1", State: domain.StepQueued, Capability: "code.test", InputsJSON: []byte(`{}`)}},
					)
					if err != nil {
						t.Fatal(err)
					}
					message = ports.Offer{FreeSlots: 1}
				}
				sent := false
				link := &shutdownLink{message: message, send: func(context.Context) error {
					sent = true
					if assignment {
						_, steps, err := store.Get(context.Background(), "e1")
						if err != nil || len(steps) != 1 || steps[0].State != domain.StepLeased {
							t.Errorf("lease not committed before send: %v %v", steps, err)
						}
					}
					cancel()
					if unrelated {
						return failure
					}
					return fmt.Errorf("send: %w", ctx.Err())
				}}
				err := coordinator.New(store, time.Now, coordtest.Permissive).Serve(ctx, link, "w1")
				if !sent {
					t.Fatal("send not reached")
				}
				if unrelated && !errors.Is(err, failure) {
					t.Fatalf("lost transport error: %v", err)
				}
				if !unrelated && err != nil {
					t.Fatalf("normal shutdown: %v", err)
				}
			})
		}
	}
}
