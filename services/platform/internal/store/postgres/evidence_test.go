package postgres

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rluders/amaru/services/platform/internal/domain"
)

// Only transaction lifecycle is faked here; PostgreSQL snapshot behavior is tested
// in evidence_pg_test.go. No SQL semantics are simulated by this driver.
type scopeDriver struct{}
type scopeConn struct{ state *scopeState }
type scopeState struct {
	opts       driver.TxOptions
	rollback   atomic.Int32
	fail       error
	cancel     bool
	entered    chan struct{}
	rolledBack chan struct{}
}
type scopeConnector struct{ state *scopeState }

func (c scopeConnector) Connect(context.Context) (driver.Conn, error) {
	return &scopeConn{c.state}, nil
}
func (c scopeConnector) Driver() driver.Driver         { return scopeDriver{} }
func (scopeDriver) Open(string) (driver.Conn, error)   { return nil, errors.New("use connector") }
func (*scopeConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("unexpected prepare") }
func (*scopeConn) Close() error                        { return nil }
func (*scopeConn) Begin() (driver.Tx, error)           { return nil, errors.New("unexpected begin") }
func (c *scopeConn) BeginTx(_ context.Context, opts driver.TxOptions) (driver.Tx, error) {
	c.state.opts = opts
	return c, nil
}
func (*scopeConn) Commit() error     { return errors.New("unexpected commit after failed query") }
func (c *scopeConn) Rollback() error { c.state.rollback.Add(1); close(c.state.rolledBack); return nil }
func (c *scopeConn) QueryContext(ctx context.Context, _ string, _ []driver.NamedValue) (driver.Rows, error) {
	if c.state.cancel {
		close(c.state.entered)
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return nil, c.state.fail
}

func TestEvidenceReadScopeFailure(t *testing.T) {
	sentinel := errors.New("read failed")
	for _, cancel := range []bool{false, true} {
		t.Run(map[bool]string{false: "read failure", true: "cancellation"}[cancel], func(t *testing.T) {
			state := &scopeState{fail: sentinel, cancel: cancel, entered: make(chan struct{}), rolledBack: make(chan struct{})}
			db := sql.OpenDB(scopeConnector{state})
			defer db.Close()
			ctx, stop := context.WithCancel(context.Background())
			defer stop()
			if cancel {
				go func() { <-state.entered; stop() }()
			}
			b, err := NewEvidenceReader(db).Bundle(ctx, "e")
			want := sentinel
			if cancel {
				want = context.Canceled
			}
			if !errors.Is(err, want) || !reflect.DeepEqual(b, domain.ExecutionEvidence{}) {
				t.Fatalf("bundle=%+v err=%v", b, err)
			}
			select {
			case <-state.rolledBack:
			case <-time.After(time.Second):
				t.Fatal("rollback did not finish")
			}
			if state.opts.Isolation != driver.IsolationLevel(sql.LevelRepeatableRead) || !state.opts.ReadOnly {
				t.Fatalf("options=%+v", state.opts)
			}
			if state.rollback.Load() != 1 {
				t.Fatalf("rollbacks=%d", state.rollback.Load())
			}
		})
	}
}
