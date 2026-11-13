package coordinator_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rluders/amaru/services/platform/internal/coordinator"
	"github.com/rluders/amaru/services/platform/internal/ports"
	"github.com/rluders/amaru/services/platform/internal/ports/fakes"
)

type receiveLink struct {
	ports.PlatformLink
	receive func(context.Context) (ports.WorkerMessage, error)
}

func (l receiveLink) Receive(ctx context.Context) (ports.WorkerMessage, error) { return l.receive(ctx) }

func TestServeJoinsReceivePump(t *testing.T) {
	entered, cancelled, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
	released := false
	defer func() {
		if !released {
			close(release)
		}
	}()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	link := receiveLink{receive: func(ctx context.Context) (ports.WorkerMessage, error) {
		close(entered)
		<-ctx.Done()
		close(cancelled)
		<-release
		return nil, ctx.Err()
	}}
	done := make(chan error, 1)
	go func() { done <- coordinator.New(fakes.NewExecutionStore(), time.Now).Serve(ctx, link, "embedded") }()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("pump not entered")
	}
	cancel()
	select {
	case <-cancelled:
	case <-time.After(time.Second):
		t.Fatal("pump not cancelled")
	}
	select {
	case <-done:
		t.Fatal("Serve returned before its receive pump joined")
	case <-time.After(25 * time.Millisecond):
	}
	close(release)
	released = true
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("Serve did not join released pump")
	}
}

func TestServeReceiveFailureDuringCancellation(t *testing.T) {
	for _, panics := range []bool{false, true} {
		t.Run(map[bool]string{false: "error", true: "panic"}[panics], func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			failure := errors.New("transport failed")
			link := receiveLink{receive: func(context.Context) (ports.WorkerMessage, error) {
				cancel()
				if panics {
					panic("private transport value")
				}
				return nil, failure
			}}
			err := coordinator.New(fakes.NewExecutionStore(), time.Now).Serve(ctx, link, "embedded")
			if panics {
				if err == nil || err.Error() != "Worker receive pump panicked" {
					t.Fatal("pump panic escaped or was lost")
				}
			} else if !errors.Is(err, failure) {
				t.Fatal("transport failure was lost during cancellation")
			}
		})
	}
}
