package wf

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Querier is the pool or the caller's open transaction.
type Querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Clock is the time source of leases, timers and deadlines. Seconds since epoch.
// Inside a transaction the caller passes the transaction as q: reading the
// clock through the pool while a transaction holds a connection deadlocks once
// concurrent transactions use up the pool (observed on a 4-core CI runner).
type Clock interface {
	Now(ctx context.Context, q Querier) (int64, error)
}

// Wall is the real clock.
type Wall struct{}

func (Wall) Now(context.Context, Querier) (int64, error) { return time.Now().Unix(), nil }

// Sim is a controlled clock stored in PostgreSQL, so it survives process kills
// and every process of a scenario sees the same time. Time moves only on Advance.
type Sim struct{ Pool *pgxpool.Pool }

func (s Sim) Now(ctx context.Context, q Querier) (int64, error) {
	var n int64
	if q == nil {
		q = s.Pool
	}
	err := q.QueryRow(ctx, `select now from sim_clock where id=1`).Scan(&n)
	return n, err
}

// Init sets the epoch once; later calls keep the stored value.
func (s Sim) Init(ctx context.Context, start int64) error {
	_, err := s.Pool.Exec(ctx, `insert into sim_clock values (1,$1) on conflict do nothing`, start)
	return err
}

func (s Sim) Advance(ctx context.Context, secs int64) error {
	_, err := s.Pool.Exec(ctx, `update sim_clock set now=now+$1 where id=1`, secs)
	return err
}
