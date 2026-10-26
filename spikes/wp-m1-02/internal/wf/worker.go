package wf

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// Execute is the embedded Worker running one assignment. Every stale or
// duplicated assignment is rejected by compare-and-set on the Step version, so
// redelivery is harmless. It returns true if this call ran the Step.
func (c *Coord) Execute(ctx context.Context, a Assignment) (bool, error) {
	now, err := c.Clock.Now(ctx)
	if err != nil {
		return false, err
	}
	var kind, execID string
	var failFirst bool
	var attempts int
	var timeout *int64
	var job *string
	var newVer int
	err = c.tx(ctx, "running", func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `update steps set state='running', version=version+1, lease_owner=$3, lease_expires_at=$4
			where id=$1 and state='assigned' and version=$2
			returning kind, exec_id, fail_first, attempts, timeout_s, remote_job, version`,
			a.Step, a.Version, c.WorkerID, now+c.LeaseTTL).Scan(&kind, &execID, &failFirst, &attempts, &timeout, &job, &newVer)
		if err != nil {
			return err
		}
		if timeout != nil {
			if _, err := tx.Exec(ctx, `insert into timers(kind,step_id,due,step_version) values ('step_timeout',$1,$2,$3)`, a.Step, now+*timeout, newVer); err != nil {
				return err
			}
		}
		return c.event(ctx, tx, execID, "step.running", a.Step, "")
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil // duplicate, stale or out-of-order assignment
	}
	if err != nil {
		return false, err
	}

	// capability
	switch kind {
	case KindResumable:
		if job == nil {
			j := "job-" + a.Step
			if _, err := c.Pool.Exec(ctx, `update steps set remote_job=$2 where id=$1 and version=$3`, a.Step, j, newVer); err != nil {
				return false, err
			}
			c.hit("worker.after_job_recorded")
		}
		// the remote job runs once per job id, whoever re-attaches to it
		if err := c.effect(ctx, a.Step, "job-"+a.Step); err != nil {
			return false, err
		}
	case KindIdempotent:
		if !(failFirst && attempts == 0) {
			if err := c.effect(ctx, a.Step, "key-"+a.Step); err != nil {
				return false, err
			}
		}
	case KindNonIdempotent:
		if err := c.effect(ctx, a.Step, ""); err != nil {
			return false, err
		}
	}
	c.hit("worker.after_effect")

	ok := !(kind == KindIdempotent && failFirst && attempts == 0)
	applied, err := c.Complete(ctx, a.Step, newVer, ok, "capability_failed")
	if err != nil {
		return false, err
	}
	return applied, nil
}

// effect records one external side effect. A non-empty key is an idempotency
// key: the world ignores a second effect with the same key.
func (c *Coord) effect(ctx context.Context, step, key string) error {
	_, err := c.Pool.Exec(ctx, `insert into world(step_id,key,run) values ($1,nullif($2,''),$3) on conflict (key) do nothing`, step, key, c.WorkerID)
	return err
}

// Complete records a Worker result, only if the Step is still running at the
// version the Worker was given (fencing: a Worker that lost its lease cannot
// complete a Step that was re-leased). On success the next Step is dispatched
// in the same transaction.
func (c *Coord) Complete(ctx context.Context, step string, version int, ok bool, reason string) (bool, error) {
	applied := false
	err := c.tx(ctx, "complete", func(tx pgx.Tx) error {
		now, err := c.Clock.Now(ctx)
		if err != nil {
			return err
		}
		var execID string
		var attempts, max int
		err = tx.QueryRow(ctx, `select exec_id, attempts, max_attempts from steps where id=$1 and state='running' and version=$2 for update`, step, version).Scan(&execID, &attempts, &max)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil // stale result: ignored, recorded nowhere
		}
		if err != nil {
			return err
		}
		applied = true
		switch {
		case ok:
			if _, err := tx.Exec(ctx, `update steps set state='succeeded', version=version+1, lease_owner=null, lease_expires_at=null where id=$1`, step); err != nil {
				return err
			}
			if err := c.event(ctx, tx, execID, "step.succeeded", step, ""); err != nil {
				return err
			}
		case attempts+1 < max:
			backoff := int64(10 * (attempts + 1))
			if _, err := tx.Exec(ctx, `update steps set state='retry_wait', attempts=attempts+1, version=version+1, lease_owner=null, lease_expires_at=null, reason=$2 where id=$1`, step, reason); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `insert into timers(kind,step_id,due,step_version) values ('retry',$1,$2,$3)`, step, now+backoff, version+1); err != nil {
				return err
			}
			if err := c.event(ctx, tx, execID, "step.retry_scheduled", step, fmt.Sprintf("in %ds", backoff)); err != nil {
				return err
			}
		default:
			if _, err := tx.Exec(ctx, `update steps set state='failed', attempts=attempts+1, version=version+1, lease_owner=null, lease_expires_at=null, reason=$2 where id=$1`, step, reason); err != nil {
				return err
			}
			if err := c.event(ctx, tx, execID, "step.failed", step, reason); err != nil {
				return err
			}
		}
		if err := c.dispatchTx(ctx, tx); err != nil {
			return err
		}
		return c.refreshExec(ctx, tx, execID)
	})
	return applied, err
}
