package operational

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rluders/lane/v2"
)

type forbiddenLogValue struct{ calls *atomic.Int32 }

func (v forbiddenLogValue) LogValue() slog.Value {
	v.calls.Add(1)
	panic("must not resolve untrusted LogValuer")
}
func (v forbiddenLogValue) String() string {
	v.calls.Add(1)
	panic("must not format untrusted Stringer")
}
func (v forbiddenLogValue) Error() string {
	v.calls.Add(1)
	panic("must not format untrusted error")
}

func TestLifecycleDropsRawDataBeforeFormatting(t *testing.T) {
	queue := make(chan Record, 8)
	var calls atomic.Int32
	raw := forbiddenLogValue{&calls}
	const secret = "unregistered-lifecycle-secret"
	logger := slog.New(LifecycleHandler(queue)).With("secret", secret, "value", raw).WithGroup(secret)
	for _, level := range []slog.Level{-100, slog.LevelInfo, slog.LevelWarn, slog.LevelError, 100} {
		logger.Log(context.Background(), level, secret,
			"error", raw, "stringer", raw, "value", slog.AnyValue(raw),
			"stack", secret, "runner", secret, "op", secret,
			slog.Group(secret, slog.Any("nested", raw)))
	}
	var sink bytes.Buffer
	l, _ := New(&sink)
	ctx := WithIdentity(context.Background(), Identity{InstanceID: "instance", CorrelationID: "lifecycle"})
	want := []string{"debug", "info", "warning", "error", "error"}
	for _, outcome := range want {
		select {
		case rec := <-queue:
			if rec != (Record{Operation: "process.lifecycle", Outcome: outcome}) {
				t.Fatal("record contains unexpected data")
			}
			if err := l.Write(ctx, rec); err != nil {
				t.Fatal(err)
			}
		default:
			t.Fatal("missing lifecycle record")
		}
	}
	if calls.Load() != 0 || strings.Contains(sink.String(), secret) || len(records(t, &sink)) != len(want) {
		t.Fatal("unsafe or missing lifecycle output")
	}
}

func TestLifecycleHandoffDoesNotWaitForConsumer(t *testing.T) {
	for _, queue := range []chan Record{nil, make(chan Record), make(chan Record, 1)} {
		done := make(chan struct{})
		go func() {
			defer close(done)
			logger := slog.New(LifecycleHandler(queue))
			var wg sync.WaitGroup
			for range 8 {
				wg.Go(func() {
					for range 100 {
						logger.Error("raw error")
					}
				})
			}
			wg.Wait()
		}()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Fatal("lifecycle logging blocked without a consumer")
		}
		if len(queue) != cap(queue) {
			t.Fatal("handoff did not retain the available bounded capacity")
		}
	}
}

type failingLifecycleRunner struct{ panic bool }

func (failingLifecycleRunner) Name() string { return "unregistered-lifecycle-secret" }
func (r failingLifecycleRunner) Start(context.Context) error {
	if r.panic {
		panic("unregistered-lifecycle-secret")
	}
	return errors.New("unregistered-lifecycle-secret")
}
func (failingLifecycleRunner) Stop(context.Context) error {
	return errors.New("unregistered-lifecycle-secret")
}

func TestLaneFailuresUseSafeLifecycleHandoff(t *testing.T) {
	for _, panics := range []bool{false, true} {
		queue := make(chan Record, 32)
		l := lane.New(lane.WithLogger(slog.New(LifecycleHandler(queue))))
		l.AddStage(failingLifecycleRunner{panic: panics})
		done := make(chan error, 1)
		go func() { done <- l.Run(context.Background()) }()
		select {
		case err := <-done:
			// Run errors remain unsafe; check presence/type without formatting.
			var panicErr *lane.PanicError
			if err == nil || errors.As(err, &panicErr) != panics {
				t.Fatal("Lane did not preserve the failure")
			}
		case <-time.After(3 * time.Second):
			t.Fatal("Lane failure did not settle")
		}
		var sink bytes.Buffer
		logger, _ := New(&sink)
		ctx := WithIdentity(context.Background(), Identity{InstanceID: "instance", CorrelationID: "lifecycle"})
		close(queue) // Run and all diagnostic producers have joined.
		var foundError bool
		for rec := range queue {
			foundError = foundError || rec.Outcome == "error"
			if err := logger.Write(ctx, rec); err != nil {
				t.Fatal(err)
			}
		}
		if !foundError || strings.Contains(sink.String(), "unregistered-lifecycle-secret") || strings.Contains(sink.String(), "stack") {
			t.Fatal("Lane failure diagnostics missing or unsafe")
		}
	}
}
