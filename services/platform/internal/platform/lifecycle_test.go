package platform

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rluders/lane/v2"
)

func TestLifecycleWatchdogBoundsUncooperativeStartup(t *testing.T) {
	for _, blocked := range []string{"start", "readiness", "cleanup"} {
		t.Run(blocked, func(t *testing.T) {
			release := make(chan struct{})
			defer close(release)
			var admitted atomic.Bool
			l, _ := NewLifecycle(time.Now().Add(75*time.Millisecond), 50*time.Millisecond, nil)
			runner := dependent{
				start: func(ctx context.Context) error {
					if blocked == "start" {
						<-release
					} else {
						<-ctx.Done()
					}
					return nil
				},
				ready: func(ctx context.Context) error {
					if blocked == "readiness" {
						<-release
					} else {
						<-ctx.Done()
					}
					return ctx.Err()
				},
				stop: func(ctx context.Context) error {
					if l.Ready() {
						return errors.New("admission remained open during Stop")
					}
					if blocked == "cleanup" {
						<-release
					}
					return nil
				},
			}
			started := time.Now()
			err := l.Run(context.Background(), []lane.Runner{runner}, []lane.Runner{dependent{
				start: func(ctx context.Context) error { admitted.Store(true); <-ctx.Done(); return nil },
			}})
			if !errors.Is(err, context.DeadlineExceeded) || admitted.Load() || l.Ready() {
				t.Fatal("startup expiry did not fail closed")
			}
			if time.Since(started) > time.Second {
				t.Fatal("startup/cleanup escaped aggregate budgets")
			}
		})
	}
}

func TestLifecycleHealthyLifetimeAndShutdownLatch(t *testing.T) {
	deadline := time.Now().Add(200 * time.Millisecond)
	l, _ := NewLifecycle(deadline, time.Second, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	entered, cancelled := make(chan struct{}), make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- l.Run(ctx, []lane.Runner{dependent{
			start: func(ctx context.Context) error {
				close(entered)
				<-ctx.Done()
				if l.Ready() {
					return errors.New("ready after Start cancellation")
				}
				close(cancelled)
				return nil
			},
			stop: func(ctx context.Context) error {
				select {
				case <-cancelled:
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			},
		}})
	}()
	wait(t, entered)
	limit := time.After(time.Second)
	for !l.Ready() {
		select {
		case err := <-done:
			t.Fatalf("early lifecycle exit: %v", err)
		case <-limit:
			t.Fatal("never ready")
		default:
			time.Sleep(time.Millisecond)
		}
	}
	<-time.After(time.Until(deadline) + 25*time.Millisecond)
	if !l.Ready() {
		t.Fatal("startup deadline cancelled healthy lifetime")
	}
	select {
	case <-done:
		t.Fatal("healthy process exited")
	default:
	}
	cancel()
	if err := result(t, done); err != nil {
		t.Fatal(err)
	}
	if l.Ready() {
		t.Fatal("ready after shutdown")
	}
	if err := l.Run(context.Background()); err == nil {
		t.Fatal("in-process reuse accepted")
	}
}

func TestLifecycleFailuresAndIntentionalCancellation(t *testing.T) {
	for _, failure := range []string{"start", "readiness", "panic", "stop", "cancel", "deadline"} {
		t.Run(failure, func(t *testing.T) {
			l, _ := NewLifecycle(time.Now().Add(time.Second), time.Second, nil)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if failure == "deadline" {
				var stop context.CancelFunc
				ctx, stop = context.WithTimeout(ctx, 50*time.Millisecond)
				defer stop()
			}
			boom := errors.New("private error")
			var stopped atomic.Bool
			err := l.Run(ctx, []lane.Runner{dependent{
				start: func(ctx context.Context) error {
					if failure == "start" {
						return boom
					}
					if failure == "panic" {
						panic("private panic")
					}
					<-ctx.Done()
					return nil
				},
				ready: func(context.Context) error {
					if failure == "readiness" {
						return boom
					}
					if failure == "cancel" || failure == "stop" {
						cancel()
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
			}})
			if (err == nil) != (failure == "cancel") || !stopped.Load() || l.Ready() {
				t.Fatal("failure/cancellation classification or cleanup incorrect")
			}
			if failure == "deadline" && !errors.Is(err, context.DeadlineExceeded) {
				t.Fatal("caller deadline was treated as intentional cancellation")
			}
		})
	}
}

func TestLifecycleConfigurationAndExpiredDeadline(t *testing.T) {
	for _, config := range []struct {
		deadline time.Time
		shutdown time.Duration
	}{
		{time.Time{}, time.Second}, {time.Now(), 0}, {time.Now(), -time.Second},
	} {
		if _, err := NewLifecycle(config.deadline, config.shutdown, nil); err == nil {
			t.Fatal("invalid configuration accepted")
		}
	}
	l, _ := NewLifecycle(time.Now().Add(-time.Second), time.Second, nil)
	var acquired atomic.Bool
	err := l.Run(context.Background(), []lane.Runner{dependent{start: func(context.Context) error { acquired.Store(true); return nil }}})
	if !errors.Is(err, context.DeadlineExceeded) || acquired.Load() || l.Ready() {
		t.Fatal("expired startup acquired resources")
	}
}

func TestLifecycleStartupRollbackKeepsStorageUntilDependentJoins(t *testing.T) {
	deadline := time.Now().Add(100 * time.Millisecond)
	l, _ := NewLifecycle(deadline, time.Second, nil)
	f := &fakeConnector{}
	s, _ := NewStorage(f, deadline)
	var joined atomic.Bool
	err := l.Run(context.Background(), []lane.Runner{s}, []lane.Runner{dependent{
		ready: func(ctx context.Context) error { <-ctx.Done(); return ctx.Err() },
		stop: func(ctx context.Context) error {
			db, err := s.DB(ctx)
			if err != nil {
				return err
			}
			if err := db.PingContext(ctx); err != nil {
				return err
			}
			joined.Store(f.closes.Load() == 0)
			return nil
		},
	}})
	if !errors.Is(err, context.DeadlineExceeded) || !joined.Load() || f.closes.Load() != 1 || l.Ready() {
		t.Fatal("watchdog rollback did not join dependent before closing storage")
	}
}

func TestLifecycleRunnerFailureClosesAdmissionBeforePeerCancellation(t *testing.T) {
	l, _ := NewLifecycle(time.Now().Add(time.Second), time.Second, nil)
	boom := errors.New("essential loop failed")
	var observedClosed atomic.Bool
	err := l.Run(context.Background(), []lane.Runner{
		dependent{start: func(ctx context.Context) error {
			for !l.Ready() {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(time.Millisecond):
				}
			}
			return boom
		}},
		dependent{start: func(ctx context.Context) error {
			<-ctx.Done()
			observedClosed.Store(!l.Ready())
			return nil
		}},
	})
	if !errors.Is(err, boom) || !observedClosed.Load() || l.Ready() {
		t.Fatal("failed essential loop left admission open when peer was cancelled")
	}
}
