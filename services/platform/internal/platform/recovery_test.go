package platform

import (
	"context"
	"database/sql/driver"
	"errors"
	"io"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rluders/amaru/services/platform/internal/coordinator"
)

type recoveryConnector struct {
	*fakeConnector
	query func(context.Context) (driver.Rows, error)
}

func (f *recoveryConnector) Connect(ctx context.Context) (driver.Conn, error) {
	conn, err := f.fakeConnector.Connect(ctx)
	return recoveryConnection{Conn: conn, query: f.query}, err
}

type recoveryConnection struct {
	driver.Conn
	query func(context.Context) (driver.Rows, error)
}

func (c recoveryConnection) Ping(ctx context.Context) error { return ctx.Err() }
func (c recoveryConnection) QueryContext(ctx context.Context, _ string, _ []driver.NamedValue) (driver.Rows, error) {
	return c.query(ctx)
}

type emptyExecutions struct{}

func (emptyExecutions) Columns() []string         { return []string{"id"} }
func (emptyExecutions) Close() error              { return nil }
func (emptyExecutions) Next([]driver.Value) error { return io.EOF }

func TestRecoveryCompletionGatesCoordinatorAndLaterStages(t *testing.T) {
	entered, release, admitted := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	connector := &recoveryConnector{fakeConnector: &fakeConnector{}, query: func(ctx context.Context) (driver.Rows, error) {
		calls.Add(1)
		close(entered)
		select {
		case <-release:
			return emptyExecutions{}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}}
	s, _ := NewStorage(connector, time.Now().Add(3*time.Second))
	r, _ := NewRecovery(s)
	waiting, done := make(chan error, 1), make(chan error, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { waiting <- r.WaitReady(ctx) }()
	l := newLane()
	l.AddStage(s)
	l.AddStage(r)
	l.AddStage(dependent{ready: func(ctx context.Context) error {
		first, err := r.Coordinator(ctx)
		if err != nil {
			return err
		}
		second, err := r.Coordinator(ctx)
		if err != nil {
			return err
		}
		if first == nil || first != second {
			return errors.New("Coordinator instance changed")
		}
		close(admitted)
		return nil
	}, stop: func(ctx context.Context) error {
		db, err := s.DB(ctx)
		if err != nil {
			return err
		}
		return db.PingContext(ctx)
	}})
	go func() { done <- l.Run(ctx) }()
	wait(t, entered)
	select {
	case <-waiting:
		t.Fatal("readiness completed before reconciliation")
	case <-admitted:
		t.Fatal("later stage admitted before reconciliation")
	default:
	}
	probe, stop := context.WithCancel(ctx)
	stop()
	if c, err := r.Coordinator(probe); c != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled accessor exposed Coordinator")
	}
	close(release)
	wait(t, admitted)
	if err := result(t, waiting); err != nil {
		t.Fatal("pre-Start readiness waiter failed")
	}
	cancel()
	if err := result(t, done); err != nil {
		t.Fatal("clean shutdown failed")
	}
	if calls.Load() != 1 || connector.closes.Load() != 1 {
		t.Fatal("reconciliation repeated or storage not closed")
	}
	if err := r.Stop(context.Background()); err != nil {
		t.Fatal("repeat Stop failed")
	}
}

func TestRecoveryFailurePreventsAdmission(t *testing.T) {
	failure := errors.New("recovery query failed")
	for _, mode := range []string{"error", "cancel", "deadline", "panic"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			deadline := time.Now().Add(3 * time.Second)
			if mode == "deadline" {
				deadline = time.Now().Add(50 * time.Millisecond)
			}
			connector := &recoveryConnector{fakeConnector: &fakeConnector{}, query: func(ctx context.Context) (driver.Rows, error) {
				switch mode {
				case "cancel":
					cancel()
					<-ctx.Done()
					return nil, ctx.Err()
				case "deadline":
					<-ctx.Done()
					return nil, ctx.Err()
				default:
					return nil, failure
				}
			}}
			s, _ := NewStorage(connector, deadline)
			var options []coordinator.Option
			if mode == "panic" {
				options = append(options, func(*coordinator.Coordinator) { panic("untrusted panic value") })
			}
			r, _ := NewRecovery(s, options...)
			var admitted atomic.Bool
			l := newLane()
			l.AddStage(s)
			l.AddStage(r)
			l.AddStage(dependent{ready: func(context.Context) error { admitted.Store(true); return nil }})
			if err := l.Run(ctx); err == nil && mode != "cancel" {
				t.Fatal("failed recovery reported success")
			}
			if c, err := r.Coordinator(context.Background()); err == nil || c != nil {
				t.Fatal("failed recovery exposed Coordinator")
			}
			if admitted.Load() || connector.closes.Load() != 1 {
				t.Fatal("later stage admitted or pool retained")
			}
		})
	}
}

func TestRecoveryConfigurationAndBoundedJoin(t *testing.T) {
	if _, err := NewRecovery(nil); err == nil {
		t.Fatal("nil storage accepted")
	}
	entered, release := make(chan struct{}), make(chan struct{})
	connector := &recoveryConnector{fakeConnector: &fakeConnector{}, query: func(context.Context) (driver.Rows, error) {
		close(entered)
		<-release // model a driver that ignores cancellation
		return emptyExecutions{}, nil
	}}
	s, _ := NewStorage(connector, time.Now().Add(3*time.Second))
	r, _ := NewRecovery(s)
	ctx, cancel := context.WithCancel(context.Background())
	storageDone, recoveryDone := make(chan error, 1), make(chan error, 1)
	go func() { storageDone <- s.Start(ctx) }()
	if err := s.WaitReady(ctx); err != nil {
		t.Fatal(err)
	}
	go func() { recoveryDone <- r.Start(ctx) }()
	wait(t, entered)
	cancel()
	expired, stop := context.WithCancel(context.Background())
	stop()
	if err := r.Stop(expired); !errors.Is(err, context.Canceled) {
		t.Fatal("Stop exceeded its cleanup budget")
	}
	close(release)
	if err := result(t, recoveryDone); !errors.Is(err, context.Canceled) {
		t.Fatal("late success ignored startup cancellation")
	}
	if c, err := r.Coordinator(context.Background()); c != nil || err == nil {
		t.Fatal("late completion exposed Coordinator")
	}
	if err := r.Stop(context.Background()); err != nil {
		t.Fatal("Stop did not join late completion")
	}
	if err := result(t, storageDone); err != nil {
		t.Fatal(err)
	}
	if err := s.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
}
