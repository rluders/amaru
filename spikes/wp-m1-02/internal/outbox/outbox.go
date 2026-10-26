// Package outbox is the transactional outbox and its relay (ADR-0016 rules):
// every publish originates from an outbox row written in the same transaction
// as the state change; the relay sets Nats-Msg-Id to the row id and marks the
// row sent only after the JetStream ack.
package outbox

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

const Schema = `
create table if not exists outbox(
  id bigserial primary key,
  subject text not null,
  payload bytea not null,
  created_at timestamptz not null default now(),
  sent_at timestamptz,
  attempts int not null default 0,
  last_error text
);
create table if not exists consumer_dedup(msg_id text primary key, applied_at timestamptz not null default now());
create table if not exists effects(n bigserial primary key, msg_id text not null);
`

// MsgID is the Nats-Msg-Id of an outbox row.
func MsgID(id int64) string { return fmt.Sprintf("ob-%d", id) }

// Enqueue writes one outbox row inside the caller's transaction.
func Enqueue(ctx context.Context, tx pgx.Tx, subject string, payload []byte) (int64, error) {
	var id int64
	err := tx.QueryRow(ctx, `insert into outbox(subject,payload) values ($1,$2) returning id`, subject, payload).Scan(&id)
	return id, err
}

// Hooks inject faults at the boundaries that matter. A hook that returns an
// error makes the relay stop at that point exactly as a failure would.
type Hooks struct {
	BeforePublish func(id int64) error
	// AfterPublish runs after JetStream acked and before the row is marked sent;
	// an error models "crash or lost response between ack and mark".
	AfterPublish func(id int64, duplicate bool) error
}

// Stats counts what the relay saw.
type Stats struct{ Published, Duplicates, Failed int }

// Relay publishes unsent outbox rows in id order.
type Relay struct {
	Pool  *pgxpool.Pool
	JS    jetstream.JetStream
	Hooks Hooks
	Stats Stats
}

// RunOnce publishes up to limit rows. The batch is selected FOR UPDATE SKIP
// LOCKED in one transaction, so several relays never publish the same row and
// a relay killed mid-batch leaves its rows unsent.
func (r *Relay) RunOnce(ctx context.Context, limit int) (int, error) {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, `select id, subject, payload from outbox where sent_at is null order by id limit $1 for update skip locked`, limit)
	if err != nil {
		return 0, err
	}
	type row struct {
		id      int64
		subject string
		payload []byte
	}
	var batch []row
	for rows.Next() {
		var x row
		if err := rows.Scan(&x.id, &x.subject, &x.payload); err != nil {
			return 0, err
		}
		batch = append(batch, x)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	done := 0
	for _, x := range batch {
		if r.Hooks.BeforePublish != nil {
			if err := r.Hooks.BeforePublish(x.id); err != nil {
				return done, commitPartial(ctx, tx, err)
			}
		}
		ack, err := r.JS.PublishMsg(ctx, &nats.Msg{Subject: x.subject, Data: x.payload}, jetstream.WithMsgID(MsgID(x.id)))
		if err != nil {
			r.Stats.Failed++
			tx.Exec(ctx, `update outbox set attempts=attempts+1, last_error=$2 where id=$1`, x.id, err.Error())
			return done, errors.Join(err, tx.Commit(ctx))
		}
		r.Stats.Published++
		if ack.Duplicate {
			r.Stats.Duplicates++
		}
		if r.Hooks.AfterPublish != nil {
			if err := r.Hooks.AfterPublish(x.id, ack.Duplicate); err != nil {
				return done, commitPartial(ctx, tx, err)
			}
		}
		if _, err := tx.Exec(ctx, `update outbox set sent_at=now() where id=$1`, x.id); err != nil {
			return done, err
		}
		done++
	}
	return done, tx.Commit(ctx)
}

// commitPartial keeps the rows already marked sent in this batch and returns err.
func commitPartial(ctx context.Context, tx pgx.Tx, err error) error {
	return errors.Join(err, tx.Commit(ctx))
}
