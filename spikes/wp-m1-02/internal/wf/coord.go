// Package wf is the spike's durable execution state machine (ADR-0042): all
// state in PostgreSQL, every transition committed together with the messages it
// causes (transactional outbox), restart and lost-Worker handling by explicit
// rule. It is an experiment, not the Platform coordinator.
package wf

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rluders/amaru/spikes/wp-m1-02/internal/bus"
	"github.com/rluders/amaru/spikes/wp-m1-02/internal/outbox"
)

// StepDef declares one Step of a Workflow.
type StepDef struct {
	Name              string
	Kind              string
	TimeoutS          int64 // running Step timeout (0 = none)
	ApprovalDeadlineS int64 // approval gate deadline (0 = none)
	FailFirst         bool  // capability fails its first attempt (retry/back-off)
	MaxAttempts       int
}

// Assignment is the outbox payload that offers a Step to a Worker.
type Assignment struct {
	Step    string `json:"step"`
	Version int    `json:"v"`
}

// Coord is the coordinator. WorkerID identifies this process's embedded Worker.
type Coord struct {
	Pool     *pgxpool.Pool
	Clock    Clock
	Crash    Crasher
	WorkerID string
	LeaseTTL int64
}

func (c *Coord) hit(p string) {
	if c.Crash != nil {
		c.Crash.Hit(p)
	}
}

// StartExecution creates the Execution and its Steps and dispatches the first
// Step, all in one transaction. Re-running it for an existing id is a no-op.
func (c *Coord) StartExecution(ctx context.Context, id string, defs []StepDef) error {
	return c.tx(ctx, "start", func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `insert into executions(id,state) values ($1,'running') on conflict do nothing`, id)
		if err != nil || tag.RowsAffected() == 0 {
			return err
		}
		for i, d := range defs {
			max := d.MaxAttempts
			if max == 0 {
				max = 3
			}
			if _, err := tx.Exec(ctx, `insert into steps(id,exec_id,ord,name,kind,state,max_attempts,timeout_s,approval_deadline_s,fail_first)
				values ($1,$2,$3,$4,$5,'pending',$6,nullif($7,0),nullif($8,0),$9)`,
				fmt.Sprintf("%s/%s", id, d.Name), id, i, d.Name, d.Kind, max, d.TimeoutS, d.ApprovalDeadlineS, d.FailFirst); err != nil {
				return err
			}
		}
		if err := c.event(ctx, tx, id, "execution.started", "", ""); err != nil {
			return err
		}
		return c.dispatchTx(ctx, tx)
	})
}

// Dispatch assigns the next ready Step, if any. Safe to call concurrently:
// the candidate is claimed with FOR UPDATE SKIP LOCKED.
func (c *Coord) Dispatch(ctx context.Context) error {
	return c.tx(ctx, "dispatch", func(tx pgx.Tx) error { return c.dispatchTx(ctx, tx) })
}

// dispatchTx moves the next ready Step of any Execution out of pending, inside
// the caller's transaction, so the state change and its outbox row commit together.
func (c *Coord) dispatchTx(ctx context.Context, tx pgx.Tx) error {
	for {
		var id, execID, kind string
		var ver int
		var deadline *int64
		err := tx.QueryRow(ctx, `select s.id, s.exec_id, s.kind, s.version, s.approval_deadline_s from steps s
			where s.state='pending'
			  and not exists (select 1 from steps p where p.exec_id=s.exec_id and p.ord<s.ord and p.state<>'succeeded')
			order by s.exec_id, s.ord limit 1 for update skip locked`).Scan(&id, &execID, &kind, &ver, &deadline)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		now, err := c.Clock.Now(ctx, tx)
		if err != nil {
			return err
		}
		if kind == KindApproval {
			if _, err := tx.Exec(ctx, `update steps set state=$2, version=version+1 where id=$1`, id, WaitingApproval); err != nil {
				return err
			}
			if deadline != nil {
				if _, err := tx.Exec(ctx, `insert into timers(kind,step_id,due,step_version) values ('approval_deadline',$1,$2,$3)`, id, now+*deadline, ver+1); err != nil {
					return err
				}
			}
			if err := c.event(ctx, tx, execID, "step.waiting_approval", id, ""); err != nil {
				return err
			}
			continue // a gate holds no process; look for more ready work
		}
		if err := c.assign(ctx, tx, id, execID, ver, now); err != nil {
			return err
		}
	}
}

// assign leases a Step and writes the assignment to the outbox.
func (c *Coord) assign(ctx context.Context, tx pgx.Tx, id, execID string, ver int, now int64) error {
	if _, err := tx.Exec(ctx, `update steps set state='assigned', version=version+1, lease_owner='pool', lease_expires_at=$2 where id=$1`, id, now+c.LeaseTTL); err != nil {
		return err
	}
	payload, _ := json.Marshal(Assignment{Step: id, Version: ver + 1})
	if _, err := outbox.Enqueue(ctx, tx, bus.SubjectBase+".pool", payload); err != nil {
		return err
	}
	return c.event(ctx, tx, execID, "step.assigned", id, "")
}

func (c *Coord) event(ctx context.Context, tx pgx.Tx, execID, kind, stepID, data string) error {
	var seq int
	if err := tx.QueryRow(ctx, `update executions set next_seq=next_seq+1 where id=$1 returning next_seq`, execID).Scan(&seq); err != nil {
		return err
	}
	now, err := c.Clock.Now(ctx, tx)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `insert into events(exec_id,seq,kind,step_id,data,at) values ($1,$2,$3,nullif($4,''),nullif($5,''),$6)`, execID, seq, kind, stepID, data, now)
	return err
}

// refreshExec derives the Execution state from its Steps.
func (c *Coord) refreshExec(ctx context.Context, tx pgx.Tx, execID string) error {
	_, err := tx.Exec(ctx, `update executions e set state = case
		when exists (select 1 from steps where exec_id=e.id and state='failed') then 'failed'
		when exists (select 1 from steps where exec_id=e.id and state in ('interrupted','timed_out')) then 'needs_attention'
		when not exists (select 1 from steps where exec_id=e.id and state<>'succeeded') then 'succeeded'
		else 'running' end where e.id=$1`, execID)
	return err
}

// tx runs f in a transaction and calls the crash points "<point>.before_commit"
// (everything written, nothing durable) and "<point>.after_commit".
func (c *Coord) tx(ctx context.Context, point string, f func(pgx.Tx) error) error {
	tx, err := c.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := f(tx); err != nil {
		return err
	}
	c.hit(point + ".before_commit")
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	c.hit(point + ".after_commit")
	return nil
}
