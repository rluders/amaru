package platform

import (
	"context"
	"errors"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/rluders/amaru/services/platform/internal/adapter/operational"
	"github.com/rluders/lane/v2"
)

const (
	lifecycleStarting uint32 = iota
	lifecycleReady
	lifecycleStopping
	lifecycleExpired
)

// Lifecycle adds the aggregate startup deadline and admission latch to one Lane.
// It is single-use; the caller supplies actual resource owners in stage order.
type Lifecycle struct {
	deadline    time.Time
	shutdown    time.Duration
	diagnostics chan<- operational.Record
	started     atomic.Bool
	state       atomic.Uint32
}

func NewLifecycle(deadline time.Time, shutdown time.Duration, diagnostics chan<- operational.Record) (*Lifecycle, error) {
	if deadline.IsZero() || shutdown <= 0 {
		return nil, errors.New("lifecycle requires a startup deadline and positive shutdown budget")
	}
	return &Lifecycle{deadline: deadline, shutdown: shutdown, diagnostics: diagnostics}, nil
}

func (l *Lifecycle) Ready() bool { return l.state.Load() == lifecycleReady }

// Run returns unsafe underlying errors for classification only, never for logging.
// The entrypoint must exit unsuccessfully on error; abandoned goroutines cannot
// be terminated here and must never be followed by an in-process restart.
func (l *Lifecycle) Run(ctx context.Context, stages ...[]lane.Runner) error {
	if !l.started.CompareAndSwap(false, true) {
		return errors.New("lifecycle already run")
	}
	parent := ctx
	// Forward caller cancellation only after closing admission.
	ctx, cancel := context.WithCancel(context.WithoutCancel(parent))
	defer cancel()
	expire := func() {
		if l.state.CompareAndSwap(lifecycleStarting, lifecycleExpired) {
			cancel()
		}
	}
	if !time.Now().Before(l.deadline) {
		expire()
		return context.DeadlineExceeded
	}
	timer := time.AfterFunc(time.Until(l.deadline), expire)
	defer timer.Stop()
	stop := func() {
		l.state.CompareAndSwap(lifecycleStarting, lifecycleStopping)
		l.state.CompareAndSwap(lifecycleReady, lifecycleStopping)
		timer.Stop()
	}
	defer stop()
	stopParent := context.AfterFunc(parent, func() { stop(); cancel() })
	defer stopParent()
	if parent.Err() != nil {
		stop()
		cancel()
	}
	runtime := lane.New(
		lane.WithShutdownTimeout(l.shutdown),
		lane.WithLogger(slog.New(operational.LifecycleHandler(l.diagnostics))),
		lane.WithObserver(func(event lane.Event) {
			switch event.Kind {
			case lane.EventLaneReady:
				if time.Now().Before(l.deadline) {
					if l.state.CompareAndSwap(lifecycleStarting, lifecycleReady) {
						timer.Stop()
					}
				} else {
					expire()
				}
			case lane.EventLaneStopping:
				stop()
			case lane.EventRunnerExited:
				if event.Err != nil {
					stop()
				}
			}
		}),
	)
	for _, stage := range stages {
		runtime.AddStage(stage...)
	}
	err := runtime.Run(ctx)
	if l.state.Load() == lifecycleExpired {
		// Lane treats context cancellation as intentional; startup expiry is fatal.
		return errors.Join(context.DeadlineExceeded, err)
	}
	if errors.Is(parent.Err(), context.DeadlineExceeded) {
		return errors.Join(context.DeadlineExceeded, err)
	}
	return err
}
