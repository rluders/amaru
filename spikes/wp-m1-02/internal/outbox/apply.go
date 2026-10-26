package outbox

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go/jetstream"
)

// Applier is an idempotent consumer: the idempotency key (Nats-Msg-Id) and the
// effect commit in one PostgreSQL transaction, and the message is acked only
// afterwards (ADR-0016: "every consumer is idempotent using keys stored in
// PostgreSQL; redelivery is expected").
type Applier struct {
	Pool *pgxpool.Pool
	// Naive disables the PostgreSQL key, to show what the bus alone guarantees.
	Naive bool
	// BeforeAck models a crash after the effect committed and before the ack.
	BeforeAck func(msgID string) error
	Skipped   int
}

func (a *Applier) Handle(ctx context.Context, m jetstream.Msg) error {
	id := m.Headers().Get("Nats-Msg-Id")
	tx, err := a.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if !a.Naive {
		tag, err := tx.Exec(ctx, `insert into consumer_dedup(msg_id) values ($1) on conflict do nothing`, id)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			a.Skipped++
			return m.Ack()
		}
	}
	if _, err := tx.Exec(ctx, `insert into effects(msg_id) values ($1)`, id); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	if a.BeforeAck != nil {
		if err := a.BeforeAck(id); err != nil {
			return errors.Join(err)
		}
	}
	return m.Ack()
}
