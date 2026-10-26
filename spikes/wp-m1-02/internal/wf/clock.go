package wf

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Clock is the time source of leases, timers and deadlines. Seconds since epoch.
type Clock interface {
	Now(ctx context.Context) (int64, error)
}

// Wall is the real clock.
type Wall struct{}

func (Wall) Now(context.Context) (int64, error) { return time.Now().Unix(), nil }

// Sim is a controlled clock stored in PostgreSQL, so it survives process kills
// and every process of a scenario sees the same time. Time moves only on Advance.
type Sim struct{ Pool *pgxpool.Pool }

func (s Sim) Now(ctx context.Context) (int64, error) {
	var n int64
	err := s.Pool.QueryRow(ctx, `select now from sim_clock where id=1`).Scan(&n)
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
