package wf

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/rluders/amaru/spikes/wp-m1-02/internal/bus"
)

// recoverStep applies ADR-0042 to one Step whose Worker is gone (platform
// restart) or whose lease expired. Caller holds the transaction.
//
//	assigned  nothing ran            -> pending again (re-leased)         [interpretation, ADR speaks of running Steps]
//	running   resumable + job id     -> resumed under a new lease          (rule 2)
//	running   idempotent             -> pending again, new lease           (rule 3)
//	running   anything else          -> interrupted, never re-run          (rule 4)
func (c *Coord) recoverStep(ctx context.Context, tx pgx.Tx, id, reason string) error {
	var execID, kind, state string
	var ver int
	var job *string
	err := tx.QueryRow(ctx, `select exec_id, kind, state, version, remote_job from steps
		where id=$1 and state in ('assigned','running') for update`, id).Scan(&execID, &kind, &state, &ver, &job)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	now, err := c.Clock.Now(ctx)
	if err != nil {
		return err
	}
	switch {
	case state == Assigned:
		if _, err := tx.Exec(ctx, `update steps set state='pending', version=version+1, lease_owner=null, lease_expires_at=null where id=$1`, id); err != nil {
			return err
		}
		if err := c.event(ctx, tx, execID, "step.reassigned", id, reason); err != nil {
			return err
		}
	case kind == KindResumable && job != nil:
		if err := c.assign(ctx, tx, id, execID, ver, now); err != nil {
			return err
		}
		if err := c.event(ctx, tx, execID, "step.resumed", id, reason); err != nil {
			return err
		}
	case kind == KindIdempotent:
		if _, err := tx.Exec(ctx, `update steps set state='pending', version=version+1, lease_owner=null, lease_expires_at=null where id=$1`, id); err != nil {
			return err
		}
		if err := c.event(ctx, tx, execID, "step.retried", id, reason); err != nil {
			return err
		}
	default:
		if _, err := tx.Exec(ctx, `update steps set state='interrupted', version=version+1, lease_owner=null, lease_expires_at=null, reason=$2 where id=$1`, id, reason); err != nil {
			return err
		}
		if err := c.event(ctx, tx, execID, "step.interrupted", id, reason); err != nil {
			return err
		}
	}
	if err := c.dispatchTx(ctx, tx); err != nil {
		return err
	}
	return c.refreshExec(ctx, tx, execID)
}

// Reconcile runs at start. Running Steps whose Worker is not this process's
// (the embedded Worker died with the previous Platform process) are recovered
// by the ADR-0042 rule. Assigned Steps with a valid lease keep waiting and have
// their assignment re-enqueued if no unsent copy exists (the bus store may have
// been lost); the compare-and-set in Execute makes a surplus copy harmless.
func (c *Coord) Reconcile(ctx context.Context) error {
	ids, err := c.ids(ctx, `select id from steps where state='running' and lease_owner is distinct from $1 order by id`, c.WorkerID)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err := c.tx(ctx, "recover", func(tx pgx.Tx) error { return c.recoverStep(ctx, tx, id, "platform_restarted") }); err != nil {
			return err
		}
	}
	rows, err := c.Pool.Query(ctx, `select id, version from steps where state='assigned'`)
	if err != nil {
		return err
	}
	type as struct {
		id  string
		ver int
	}
	var list []as
	for rows.Next() {
		var a as
		if err := rows.Scan(&a.id, &a.ver); err != nil {
			return err
		}
		list = append(list, a)
	}
	rows.Close()
	for _, a := range list {
		payload, _ := json.Marshal(Assignment{Step: a.id, Version: a.ver})
		if _, err := c.Pool.Exec(ctx, `insert into outbox(subject,payload) select $1,$2::bytea
			where not exists (select 1 from outbox where sent_at is null and payload=$2::bytea)`, bus.SubjectBase+".pool", payload); err != nil {
			return err
		}
	}
	return nil
}

// ExpireLeases recovers every Step whose lease is past due (Worker lost).
func (c *Coord) ExpireLeases(ctx context.Context) error {
	now, err := c.Clock.Now(ctx)
	if err != nil {
		return err
	}
	ids, err := c.ids(ctx, `select id from steps where state in ('assigned','running') and lease_expires_at<=$1 order by id`, now)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err := c.tx(ctx, "recover", func(tx pgx.Tx) error { return c.recoverStep(ctx, tx, id, "lease_expired") }); err != nil {
			return err
		}
	}
	return nil
}

// FireTimers applies every due timer. A timer carries the Step version it was
// created for and does nothing if the Step has moved on. Timers are claimed
// with FOR UPDATE SKIP LOCKED.
func (c *Coord) FireTimers(ctx context.Context) error {
	for {
		fired := false
		err := c.tx(ctx, "timer", func(tx pgx.Tx) error {
			now, err := c.Clock.Now(ctx)
			if err != nil {
				return err
			}
			var tid int64
			var kind, step string
			var ver int
			err = tx.QueryRow(ctx, `select id, kind, step_id, step_version from timers where not fired and due<=$1 order by due, id limit 1 for update skip locked`, now).Scan(&tid, &kind, &step, &ver)
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			if err != nil {
				return err
			}
			fired = true
			if _, err := tx.Exec(ctx, `update timers set fired=true where id=$1`, tid); err != nil {
				return err
			}
			var execID string
			switch kind {
			case "retry":
				err = tx.QueryRow(ctx, `update steps set state='pending', version=version+1 where id=$1 and state='retry_wait' and version=$2 returning exec_id`, step, ver).Scan(&execID)
			case "approval_deadline":
				err = tx.QueryRow(ctx, `update steps set state='failed', version=version+1, reason='approval_expired' where id=$1 and state='waiting_approval' and version=$2 returning exec_id`, step, ver).Scan(&execID)
			case "step_timeout":
				err = tx.QueryRow(ctx, `update steps set state='timed_out', version=version+1, lease_owner=null, lease_expires_at=null, reason='step_timeout' where id=$1 and state='running' and version=$2 returning exec_id`, step, ver).Scan(&execID)
			}
			if errors.Is(err, pgx.ErrNoRows) {
				return nil // stale timer
			}
			if err != nil {
				return err
			}
			if err := c.event(ctx, tx, execID, "timer."+kind, step, ""); err != nil {
				return err
			}
			if err := c.dispatchTx(ctx, tx); err != nil {
				return err
			}
			return c.refreshExec(ctx, tx, execID)
		})
		if err != nil {
			return err
		}
		if !fired {
			return nil
		}
	}
}

// Approve decides a waiting gate. A second decision is a no-op.
func (c *Coord) Approve(ctx context.Context, step string, approve bool) (bool, error) {
	applied := false
	err := c.tx(ctx, "approve", func(tx pgx.Tx) error {
		to, kind := Succeeded, "approval.approved"
		if !approve {
			to, kind = Failed, "approval.rejected"
		}
		var execID string
		err := tx.QueryRow(ctx, `update steps set state=$2, version=version+1 where id=$1 and state='waiting_approval' returning exec_id`, step, to).Scan(&execID)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		applied = true
		if err := c.event(ctx, tx, execID, kind, step, ""); err != nil {
			return err
		}
		if err := c.dispatchTx(ctx, tx); err != nil {
			return err
		}
		return c.refreshExec(ctx, tx, execID)
	})
	return applied, err
}

// Resume is the explicit human decision on an interrupted or timed-out Step:
// it goes back to pending and may run its side effect again.
func (c *Coord) Resume(ctx context.Context, step string) (bool, error) {
	applied := false
	err := c.tx(ctx, "resume", func(tx pgx.Tx) error {
		var execID string
		err := tx.QueryRow(ctx, `update steps set state='pending', version=version+1, reason=null where id=$1 and state in ('interrupted','timed_out') returning exec_id`, step).Scan(&execID)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		applied = true
		if err := c.event(ctx, tx, execID, "step.resumed_by_human", step, ""); err != nil {
			return err
		}
		if err := c.dispatchTx(ctx, tx); err != nil {
			return err
		}
		return c.refreshExec(ctx, tx, execID)
	})
	return applied, err
}

func (c *Coord) ids(ctx context.Context, q string, args ...any) ([]string, error) {
	rows, err := c.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
