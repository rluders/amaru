package platform

import (
	"context"
	"database/sql/driver"
	"errors"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rluders/amaru/services/platform/internal/adapter/operational"
	"github.com/rluders/lane/v2"
)

type fakeConnector struct {
	opens  atomic.Int32
	closes atomic.Int32
	ping   func(context.Context) error
	query  func(context.Context) error
	close  func() error
}

func (f *fakeConnector) Connect(context.Context) (driver.Conn, error) {
	f.opens.Add(1)
	return fakeConnection{f}, nil
}
func (*fakeConnector) Driver() driver.Driver { return fakeDriver{} }

type fakeDriver struct{}

func (fakeDriver) Open(string) (driver.Conn, error) { return nil, errors.New("use connector") }

type fakeConnection struct{ f *fakeConnector }

func (c fakeConnection) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("not implemented by fake")
}
func (c fakeConnection) Begin() (driver.Tx, error) { return nil, errors.New("not implemented by fake") }
func (c fakeConnection) Close() error {
	c.f.closes.Add(1)
	if c.f.close != nil {
		return c.f.close()
	}
	return nil
}
func (c fakeConnection) Ping(ctx context.Context) error {
	if c.f.ping != nil {
		return c.f.ping(ctx)
	}
	return ctx.Err()
}
func (c fakeConnection) QueryContext(ctx context.Context, _ string, _ []driver.NamedValue) (driver.Rows, error) {
	if c.f.query != nil {
		return nil, c.f.query(ctx)
	}
	return nil, errors.New("query failed")
}

type dependent struct {
	start func(context.Context) error
	ready func(context.Context) error
	stop  func(context.Context) error
}

func (dependent) Name() string { return "dependent" }
func (d dependent) Start(ctx context.Context) error {
	if d.start != nil {
		return d.start(ctx)
	}
	<-ctx.Done()
	return nil
}
func (d dependent) WaitReady(ctx context.Context) error {
	if d.ready != nil {
		return d.ready(ctx)
	}
	return nil
}
func (d dependent) Stop(ctx context.Context) error {
	if d.stop != nil {
		return d.stop(ctx)
	}
	return nil
}

func newStorage(t *testing.T, f *fakeConnector) *Storage {
	t.Helper()
	s, err := NewStorage(f, time.Now().Add(3*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func wait(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(3 * time.Second):
		t.Fatal("barrier did not settle")
	}
}
func result(t *testing.T, ch <-chan error) error {
	t.Helper()
	select {
	case err := <-ch:
		return err
	case <-time.After(3 * time.Second):
		t.Fatal("lifecycle did not settle")
		return nil
	}
}
func newLane() *lane.Lane {
	return lane.New(lane.WithLogger(slog.New(operational.LifecycleHandler(nil))), lane.WithShutdownTimeout(time.Second))
}

func TestStorageConfigurationAndReadinessBeforeStart(t *testing.T) {
	f := &fakeConnector{}
	if _, err := NewStorage(nil, time.Now()); err == nil {
		t.Fatal("nil connector accepted")
	}
	if _, err := NewStorage(f, time.Time{}); err == nil {
		t.Fatal("missing deadline accepted")
	}
	if _, err := NewSchema(nil); err == nil {
		t.Fatal("nil storage accepted")
	}
	s := newStorage(t, f)
	schema, _ := NewSchema(s)
	ctx, cancel := context.WithCancel(context.Background())
	readiness := make(chan error, 1)
	go func() { readiness <- s.WaitReady(ctx) }()
	cancel()
	if !errors.Is(result(t, readiness), context.Canceled) {
		t.Fatal("pre-Start readiness ignored cancellation")
	}
	if err := schema.WaitReady(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal("pre-Start schema readiness ignored cancellation")
	}
	if _, err := s.DB(ctx); !errors.Is(err, context.Canceled) || f.opens.Load() != 0 {
		t.Fatal("constructor/readiness acquired a connection")
	}
}

func TestStorageBarrierAndDependentCleanup(t *testing.T) {
	entered, release, admitted, cancelled := make(chan struct{}), make(chan struct{}), make(chan struct{}), make(chan struct{})
	startupContext := make(chan context.Context, 1)
	var first atomic.Bool
	f := &fakeConnector{ping: func(ctx context.Context) error {
		if first.CompareAndSwap(false, true) {
			startupContext <- ctx
			close(entered)
			select {
			case <-release:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		return ctx.Err()
	}}
	s := newStorage(t, f)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	l := newLane()
	l.AddStage(s)
	l.AddStage(dependent{
		start: func(ctx context.Context) error { close(admitted); <-ctx.Done(); close(cancelled); return nil },
		stop: func(ctx context.Context) error {
			select {
			case <-cancelled:
			case <-ctx.Done():
				return ctx.Err()
			}
			db, err := s.DB(ctx)
			if err != nil {
				return err
			}
			if f.closes.Load() != 0 {
				return errors.New("storage closed before dependent joined")
			}
			return db.PingContext(ctx)
		},
	})
	done := make(chan error, 1)
	go func() { done <- l.Run(ctx) }()
	wait(t, entered)
	select {
	case <-admitted:
		t.Fatal("dependent started before connectivity completed")
	default:
	}
	close(release)
	wait(t, admitted)
	if err := (<-startupContext).Err(); !errors.Is(err, context.Canceled) {
		t.Fatal("initialization context remained attached to healthy lifetime")
	}
	cancel()
	if err := result(t, done); err != nil {
		t.Fatal("dependent cleanup could not use storage")
	}
	if f.closes.Load() != 1 {
		t.Fatal("pool connection not closed exactly once")
	}
	if err := s.Stop(context.Background()); err != nil || f.closes.Load() != 1 {
		t.Fatal("repeat Stop changed cleanup")
	}
}

func TestStoragePartialInitializationCleanupAfterBudget(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	f := &fakeConnector{ping: func(ctx context.Context) error {
		close(entered)
		<-release // deliberately delayed driver; cleanup must remain bounded
		return ctx.Err()
	}}
	s := newStorage(t, f)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started := make(chan error, 1)
	go func() { started <- s.Start(ctx) }()
	wait(t, entered)
	cancel()
	expired, expire := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer expire()
	if err := s.Stop(expired); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("partial cleanup ignored its deadline")
	}
	close(release)
	if err := result(t, started); !errors.Is(err, context.Canceled) {
		t.Fatal("partial initialization lost cancellation")
	}
	wait(t, s.stopped)
	if f.closes.Load() != 1 {
		t.Fatal("late initialization leaked its connection")
	}
}

func TestStorageFailurePreventsLaterStages(t *testing.T) {
	for _, mode := range []string{"ping failure", "deadline"} {
		t.Run(mode, func(t *testing.T) {
			want := errors.New("unregistered-storage-secret")
			f := &fakeConnector{ping: func(context.Context) error {
				return want
			}}
			s := newStorage(t, f)
			if mode == "deadline" {
				s.deadline = time.Now().Add(-time.Second)
			}
			var later atomic.Bool
			l := newLane()
			l.AddStage(s)
			l.AddStage(dependent{start: func(context.Context) error { later.Store(true); return nil }})
			done := make(chan error, 1)
			go func() { done <- l.Run(context.Background()) }()
			err := result(t, done)
			if err == nil || later.Load() {
				t.Fatal("failed storage admitted later stage")
			}
			if mode == "ping failure" && !errors.Is(err, want) {
				t.Fatal("lost startup failure")
			}
			if mode == "deadline" && !errors.Is(err, context.DeadlineExceeded) {
				t.Fatal("lost startup deadline failure")
			}
			if _, err := s.DB(context.Background()); err == nil || f.opens.Load() != f.closes.Load() {
				t.Fatal("partial initialization exposed or leaked pool")
			}
		})
	}
}

func TestStorageCleanupBudgetAndPanic(t *testing.T) {
	for _, panics := range []bool{false, true} {
		entered, release := make(chan struct{}), make(chan struct{})
		f := &fakeConnector{close: func() error {
			close(entered)
			<-release
			if panics {
				panic("unregistered-storage-secret")
			}
			return nil
		}}
		s := newStorage(t, f)
		ctx, cancel := context.WithCancel(context.Background())
		started := make(chan error, 1)
		go func() { started <- s.Start(ctx) }()
		if err := s.WaitReady(context.Background()); err != nil {
			t.Fatal("storage initialization failed")
		}
		cancel()
		if err := result(t, started); err != nil {
			t.Fatal("Start cancellation failed")
		}
		cleanup, stop := context.WithCancel(context.Background())
		stopped := make(chan error, 1)
		go func() { stopped <- s.Stop(cleanup) }()
		wait(t, entered)
		stop()
		if !errors.Is(result(t, stopped), context.Canceled) {
			t.Fatal("blocked cleanup ignored its budget")
		}
		close(release)
		wait(t, s.stopped)
		err := s.Stop(context.Background())
		if (err != nil) != panics || (err != nil && err.Error() != "storage cleanup panicked") || f.closes.Load() != 1 {
			t.Fatal("cleanup was duplicated or panic data escaped")
		}
	}
}

func TestSchemaFailureJoinsBeforeStorageCleanup(t *testing.T) {
	for _, mode := range []string{"failure", "cancellation", "deadline"} {
		t.Run(mode, func(t *testing.T) {
			entered, exited := make(chan struct{}), make(chan struct{})
			want := errors.New("schema-unregistered-secret")
			f := &fakeConnector{query: func(ctx context.Context) error {
				close(entered)
				defer close(exited)
				if mode == "cancellation" || mode == "deadline" {
					<-ctx.Done()
					return ctx.Err()
				}
				return want
			}, close: func() error {
				select {
				case <-exited:
					return nil
				default:
					return errors.New("storage closed before schema joined")
				}
			}}
			s := newStorage(t, f)
			if mode == "deadline" {
				s.deadline = time.Now().Add(500 * time.Millisecond)
			}
			schema, _ := NewSchema(s)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			l := newLane()
			l.AddStage(s)
			l.AddStage(schema)
			var later atomic.Bool
			l.AddStage(dependent{start: func(context.Context) error { later.Store(true); return nil }})
			done := make(chan error, 1)
			go func() { done <- l.Run(ctx) }()
			wait(t, entered)
			if mode == "cancellation" {
				cancel()
			}
			err := result(t, done)
			if (err != nil) != (mode != "cancellation") || later.Load() || f.closes.Load() != 1 {
				t.Fatal("schema failure/cancellation broke rollback ordering")
			}
			if mode == "deadline" && !errors.Is(err, context.DeadlineExceeded) {
				t.Fatal("schema ignored the shared startup deadline")
			}
			if err := schema.WaitReady(context.Background()); err == nil {
				t.Fatal("failed schema reported ready")
			}
		})
	}
}

// database/sql does not promise to unwind its internal state after driver
// panics. Isolate the intentionally stranded calls in a disposable process.
func TestDriverPanicFailsClosed(t *testing.T) {
	if mode := os.Getenv("AMARU_LIFECYCLE_PANIC_TEST"); mode != "" {
		f := &fakeConnector{}
		var panicked atomic.Bool
		if mode == "storage" {
			f.ping = func(context.Context) error { panicked.Store(true); panic("unregistered-driver-secret") }
		} else {
			f.query = func(context.Context) error { panicked.Store(true); panic("unregistered-driver-secret") }
		}
		s := newStorage(t, f)
		schema, _ := NewSchema(s)
		l, _ := NewLifecycle(time.Now().Add(50*time.Millisecond), 50*time.Millisecond, nil)
		var later atomic.Bool
		if err := l.Run(context.Background(), []lane.Runner{s}, []lane.Runner{schema},
			[]lane.Runner{dependent{start: func(context.Context) error { later.Store(true); return nil }}}); err == nil || later.Load() || !panicked.Load() || l.Ready() {
			t.Fatal("driver panic did not fail closed")
		}
		if mode == "storage" {
			if _, err := s.DB(context.Background()); err == nil {
				t.Fatal("panicking storage reported ready")
			}
		}
		return
	}
	for _, mode := range []string{"storage", "schema"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestDriverPanicFailsClosed$")
			cmd.Env = append(os.Environ(), "AMARU_LIFECYCLE_PANIC_TEST="+mode)
			out, err := cmd.CombinedOutput()
			if err != nil || strings.Contains(string(out), "unregistered-driver-secret") {
				t.Fatal("bounded driver panic subprocess failed or leaked diagnostics")
			}
		})
	}
}
