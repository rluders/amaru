package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
)

var _ ports.EventStream = (*ExecutionStore)(nil)

// appendProjectEvents is part of the Execution state transaction. The counter lock is
// held until commit, so a reader never resumes past a lower uncommitted cursor.
func appendProjectEvents(ctx context.Context, tx *sql.Tx, execID string, events []domain.ExecutionEvent) error {
	if len(events) == 0 {
		return nil
	}
	var project string
	if err := tx.QueryRowContext(ctx, `SELECT project_id FROM executions WHERE id = $1`, execID).Scan(&project); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO project_event_counters (project_id) VALUES ($1) ON CONFLICT DO NOTHING`, project); err != nil {
		return err
	}
	var last int64
	if err := tx.QueryRowContext(ctx, `UPDATE project_event_counters SET last_seq = last_seq + $2 WHERE project_id = $1 RETURNING last_seq`, project, len(events)).Scan(&last); err != nil {
		return err
	}
	for i, ev := range events {
		if _, err := tx.ExecContext(ctx, `INSERT INTO project_events (project_id, seq, execution_id, execution_seq) VALUES ($1, $2, $3, $4)`,
			project, last-int64(len(events))+int64(i)+1, execID, ev.Seq); err != nil {
			return err
		}
	}
	return nil
}

func (s *ExecutionStore) ReadEvents(ctx context.Context, q domain.EventQuery, now time.Time, window time.Duration) (domain.EventPage, error) {
	if err := q.Validate(); err != nil {
		return domain.EventPage{}, err
	}
	if now.IsZero() || window <= 0 {
		return domain.EventPage{}, fmt.Errorf("%w: event replay needs a Platform clock and positive window", domain.ErrInvalid)
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return domain.EventPage{}, err
	}
	defer tx.Rollback()
	var last, unavailable, expired int64
	err = tx.QueryRowContext(ctx, `SELECT last_seq, unavailable_through FROM project_event_counters WHERE project_id = $1`, q.ProjectID).Scan(&last, &unavailable)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return domain.EventPage{}, err
	}
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(seq), 0) FROM project_events WHERE project_id = $1 AND recorded_at < $2`, q.ProjectID, now.Add(-window)).Scan(&expired); err != nil {
		return domain.EventPage{}, err
	}
	page := domain.EventPage{Events: []domain.StreamEvent{}, Cursor: last, Resync: domain.ReplayResync(q.After, last, unavailable, expired)}
	if page.Resync == nil {
		rows, err := tx.QueryContext(ctx, `SELECT p.seq, e.execution_id, e.seq, e.kind, e.step_id, e.reason, e.at
			FROM project_events p JOIN execution_events e ON e.execution_id = p.execution_id AND e.seq = p.execution_seq
			WHERE p.project_id = $1 AND p.seq > $2 AND ($3 = '' OR p.execution_id = $3)
			ORDER BY p.seq LIMIT $4`, q.ProjectID, q.After, q.ExecutionID, q.Limit)
		if err != nil {
			return domain.EventPage{}, err
		}
		for rows.Next() {
			ev := domain.StreamEvent{ProjectID: q.ProjectID}
			if err := rows.Scan(&ev.Seq, &ev.Event.ExecutionID, &ev.Event.Seq, &ev.Event.Kind, &ev.Event.StepID, &ev.Event.Reason, &ev.Event.At); err != nil {
				rows.Close()
				return domain.EventPage{}, err
			}
			ev.Event.At = ev.Event.At.UTC()
			page.Events = append(page.Events, ev)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return domain.EventPage{}, err
		}
		if len(page.Events) == q.Limit {
			page.Cursor = page.Events[len(page.Events)-1].Seq
		}
	}
	if err := tx.Commit(); err != nil {
		return domain.EventPage{}, err
	}
	return page, nil
}
