package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
)

// ExecutionStore is the PostgreSQL ports.ExecutionStore. Every write is one transaction that
// commits before the call returns.
type ExecutionStore struct{ db *sql.DB }

var _ ports.ExecutionStore = (*ExecutionStore)(nil)

// NewExecutionStore returns a store over db, which must be connected through the pgx stdlib driver
// to a database at the current schema.
func NewExecutionStore(db *sql.DB) *ExecutionStore { return &ExecutionStore{db: db} }

func nullTime(t *time.Time) sql.NullTime {
	if t == nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: t.UTC(), Valid: true}
}

func timePtr(n sql.NullTime) *time.Time {
	if !n.Valid {
		return nil
	}
	t := n.Time
	return &t
}

func nullString(s string) sql.NullString { return sql.NullString{String: s, Valid: s != ""} }

func (s *ExecutionStore) Create(ctx context.Context, e domain.Execution, steps []domain.StepExecution) (domain.Execution, []domain.StepExecution, error) {
	if e.ID == "" {
		return domain.Execution{}, nil, fmt.Errorf("%w: execution needs an id", domain.ErrInvalid)
	}
	seen := map[string]bool{}
	for _, st := range steps {
		if st.ID == "" || st.ExecutionID != e.ID || seen[st.ID] {
			return domain.Execution{}, nil, fmt.Errorf("%w: step %q does not belong to execution %s", domain.ErrInvalid, st.ID, e.ID)
		}
		seen[st.ID] = true
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.Execution{}, nil, fmt.Errorf("postgres: create execution: %w", err)
	}
	defer tx.Rollback()

	e.Version = 1
	var code, msg sql.NullString
	if e.Error != nil {
		code, msg = nullString(e.Error.Code), nullString(e.Error.Message)
	}
	_, err = tx.ExecContext(ctx,
		`INSERT INTO executions (id, version, state, created_at, started_at, ended_at, error_code, error_message)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		e.ID, e.Version, string(e.State), e.CreatedAt.UTC(), nullTime(e.StartedAt), nullTime(e.EndedAt), code, msg)
	if err != nil {
		return domain.Execution{}, nil, mapCreateErr(err, "execution", e.ID)
	}
	out := make([]domain.StepExecution, len(steps))
	for i, st := range steps {
		st.Version = 1
		if err := insertStep(ctx, tx, i, st); err != nil {
			return domain.Execution{}, nil, mapCreateErr(err, "step", st.ID)
		}
		out[i] = st
	}
	if err := tx.Commit(); err != nil {
		return domain.Execution{}, nil, fmt.Errorf("postgres: create execution: %w", err)
	}
	return e, out, nil
}

func mapCreateErr(err error, what, id string) error {
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == uniqueViolation {
		return fmt.Errorf("%w: %s %s", domain.ErrExists, what, id)
	}
	return fmt.Errorf("postgres: create %s %s: %w", what, id, err)
}

func insertStep(ctx context.Context, tx *sql.Tx, ord int, st domain.StepExecution) error {
	args, err := stepArgs(st)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx,
		`INSERT INTO step_executions (id, execution_id, ord, version, state, reason, started_at, ended_at, error_code, error_message,
		   idempotent, remote_job_id, remote_job_resumable, lease_owner, lease_expires_at, agent,
		   capability, inputs, attempt_id, fence_token)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20)`,
		append([]any{st.ID, st.ExecutionID, ord, st.Version}, args...)...)
	return err
}

// stepArgs are the mutable columns state … agent, in column order.
func stepArgs(st domain.StepExecution) ([]any, error) {
	var code, msg, job, owner sql.NullString
	var leaseExp sql.NullTime
	var resumable bool
	if st.Error != nil {
		code, msg = nullString(st.Error.Code), nullString(st.Error.Message)
	}
	if st.RemoteJob != nil {
		job, resumable = nullString(st.RemoteJob.ID), st.RemoteJob.Resumable
	}
	if st.CurrentLease != nil {
		owner = nullString(st.CurrentLease.Owner)
		leaseExp = sql.NullTime{Time: st.CurrentLease.ExpiresAt.UTC(), Valid: true}
	}
	var agent []byte
	if st.Agent != nil {
		b, err := json.Marshal(st.Agent)
		if err != nil {
			return nil, fmt.Errorf("postgres: encode agent snapshot: %w", err)
		}
		agent = b
	}
	return []any{string(st.State), st.Reason, nullTime(st.StartedAt), nullTime(st.EndedAt), code, msg,
		st.Idempotent, job, resumable, owner, leaseExp, agent,
		st.Capability, st.InputsJSON, nullString(st.AttemptID), int64(st.FenceToken)}, nil
}

func (s *ExecutionStore) Commit(ctx context.Context, c ports.ExecutionChange) (ports.ExecutionChange, error) {
	execID, err := c.ExecutionID()
	if err != nil {
		return ports.ExecutionChange{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ports.ExecutionChange{}, fmt.Errorf("postgres: commit: %w", err)
	}
	defer tx.Rollback()

	out := ports.ExecutionChange{}
	var lastSeq int64
	// The Execution row is locked first, always, so two commits to one Execution never deadlock
	// on row order and the event counter is serialized.
	switch {
	case c.Execution != nil:
		e := *c.Execution
		var code, msg sql.NullString
		if e.Error != nil {
			code, msg = nullString(e.Error.Code), nullString(e.Error.Message)
		}
		err := tx.QueryRowContext(ctx,
			`UPDATE executions SET version = version + 1, state = $3, started_at = $4, ended_at = $5,
			   error_code = $6, error_message = $7, next_seq = next_seq + $8
			 WHERE id = $1 AND version = $2 RETURNING version, next_seq`,
			e.ID, e.Version, string(e.State), nullTime(e.StartedAt), nullTime(e.EndedAt), code, msg, len(c.Events)).
			Scan(&e.Version, &lastSeq)
		if err != nil {
			return ports.ExecutionChange{}, s.missOrStale(ctx, tx, err, "execution", "executions", e.ID, e.Version)
		}
		out.Execution = &e
	case len(c.Events) > 0:
		err := tx.QueryRowContext(ctx,
			`UPDATE executions SET next_seq = next_seq + $2 WHERE id = $1 RETURNING next_seq`, execID, len(c.Events)).Scan(&lastSeq)
		if errors.Is(err, sql.ErrNoRows) {
			return ports.ExecutionChange{}, fmt.Errorf("%w: execution %s", domain.ErrNotFound, execID)
		}
		if err != nil {
			return ports.ExecutionChange{}, fmt.Errorf("postgres: commit: %w", err)
		}
	}

	// Steps are written in id order (a stable lock order); the result keeps the caller's order.
	order := make([]int, len(c.Steps))
	for i := range order {
		order[i] = i
	}
	sort.Slice(order, func(a, b int) bool { return c.Steps[order[a]].ID < c.Steps[order[b]].ID })
	out.Steps = make([]domain.StepExecution, len(c.Steps))
	for _, i := range order {
		st := c.Steps[i]
		args, err := stepArgs(st)
		if err != nil {
			return ports.ExecutionChange{}, err
		}
		err = tx.QueryRowContext(ctx,
			`UPDATE step_executions SET version = version + 1, state = $4, reason = $5, started_at = $6, ended_at = $7,
			   error_code = $8, error_message = $9, idempotent = $10, remote_job_id = $11, remote_job_resumable = $12,
			   lease_owner = $13, lease_expires_at = $14, agent = $15,
			   capability = $16, inputs = $17, attempt_id = $18, fence_token = $19
			 WHERE id = $1 AND execution_id = $2 AND version = $3 RETURNING version`,
			append([]any{st.ID, st.ExecutionID, st.Version}, args...)...).Scan(&st.Version)
		if err != nil {
			return ports.ExecutionChange{}, s.missOrStale(ctx, tx, err, "step", "step_executions", st.ID, st.Version)
		}
		out.Steps[i] = st
	}

	first := lastSeq - int64(len(c.Events))
	for i, ev := range c.Events {
		ev.ExecutionID, ev.Seq = execID, first+int64(i)+1
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO execution_events (execution_id, seq, kind, step_id, reason, at) VALUES ($1, $2, $3, $4, $5, $6)`,
			ev.ExecutionID, ev.Seq, ev.Kind, ev.StepID, ev.Reason, ev.At.UTC()); err != nil {
			return ports.ExecutionChange{}, fmt.Errorf("postgres: append event: %w", err)
		}
		out.Events = append(out.Events, ev)
	}
	if err := tx.Commit(); err != nil {
		return ports.ExecutionChange{}, fmt.Errorf("postgres: commit: %w", err)
	}
	return out, nil
}

// missOrStale turns "no row matched" into ErrNotFound (no such row) or ErrConflict (stale version).
// table is a fixed name from this file, never caller input.
func (s *ExecutionStore) missOrStale(ctx context.Context, tx *sql.Tx, err error, what, table, id string, read int64) error {
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == uniqueViolation {
		return fmt.Errorf("%w: %s %s", domain.ErrExists, what, id)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("postgres: commit %s %s: %w", what, id, err)
	}
	var cur int64
	switch err := tx.QueryRowContext(ctx, `SELECT version FROM `+table+` WHERE id = $1`, id).Scan(&cur); {
	case errors.Is(err, sql.ErrNoRows):
		return fmt.Errorf("%w: %s %s", domain.ErrNotFound, what, id)
	case err != nil:
		return fmt.Errorf("postgres: commit %s %s: %w", what, id, err)
	}
	return fmt.Errorf("%w: %s %s is at version %d, change read %d", domain.ErrConflict, what, id, cur, read)
}

func (s *ExecutionStore) Get(ctx context.Context, id string) (domain.Execution, []domain.StepExecution, error) {
	// One snapshot for both reads, so the Execution and its Steps are consistent.
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return domain.Execution{}, nil, fmt.Errorf("postgres: get execution: %w", err)
	}
	defer tx.Rollback()

	var e domain.Execution
	var state string
	var started, ended sql.NullTime
	var code, msg sql.NullString
	err = tx.QueryRowContext(ctx,
		`SELECT id, version, state, created_at, started_at, ended_at, error_code, error_message FROM executions WHERE id = $1`, id).
		Scan(&e.ID, &e.Version, &state, &e.CreatedAt, &started, &ended, &code, &msg)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Execution{}, nil, fmt.Errorf("%w: execution %s", domain.ErrNotFound, id)
	}
	if err != nil {
		return domain.Execution{}, nil, fmt.Errorf("postgres: get execution: %w", err)
	}
	e.State, e.StartedAt, e.EndedAt = domain.ExecState(state), timePtr(started), timePtr(ended)
	if code.Valid {
		e.Error = &domain.ExecError{Code: code.String, Message: msg.String}
	}

	rows, err := tx.QueryContext(ctx,
		`SELECT id, execution_id, version, state, reason, started_at, ended_at, error_code, error_message,
		   idempotent, remote_job_id, remote_job_resumable, lease_owner, lease_expires_at, agent,
		   capability, inputs, attempt_id, fence_token
		 FROM step_executions WHERE execution_id = $1 ORDER BY ord`, id)
	if err != nil {
		return domain.Execution{}, nil, fmt.Errorf("postgres: get steps: %w", err)
	}
	defer rows.Close()
	steps := []domain.StepExecution{}
	for rows.Next() {
		var st domain.StepExecution
		var state string
		var started, ended, leaseExp sql.NullTime
		var code, msg, job, owner sql.NullString
		var resumable bool
		var agent []byte
		var attempt sql.NullString
		var fence int64
		if err := rows.Scan(&st.ID, &st.ExecutionID, &st.Version, &state, &st.Reason, &started, &ended, &code, &msg,
			&st.Idempotent, &job, &resumable, &owner, &leaseExp, &agent,
			&st.Capability, &st.InputsJSON, &attempt, &fence); err != nil {
			return domain.Execution{}, nil, fmt.Errorf("postgres: get steps: %w", err)
		}
		st.State, st.StartedAt, st.EndedAt = domain.StepState(state), timePtr(started), timePtr(ended)
		if code.Valid {
			st.Error = &domain.ExecError{Code: code.String, Message: msg.String}
		}
		st.AttemptID, st.FenceToken = attempt.String, uint64(fence)
		if job.Valid {
			st.RemoteJob = &domain.RemoteJob{ID: job.String, Resumable: resumable}
		}
		if owner.Valid {
			st.CurrentLease = &domain.LeaseInfo{Owner: owner.String, ExpiresAt: leaseExp.Time}
		}
		if agent != nil {
			var a domain.ResolvedAgent
			if err := json.Unmarshal(agent, &a); err != nil {
				return domain.Execution{}, nil, fmt.Errorf("postgres: decode agent snapshot of step %s: %w", st.ID, err)
			}
			st.Agent = &a
		}
		steps = append(steps, st)
	}
	if err := rows.Err(); err != nil {
		return domain.Execution{}, nil, fmt.Errorf("postgres: get steps: %w", err)
	}
	return e, steps, nil
}

func (s *ExecutionStore) Open(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id FROM executions WHERE state NOT IN ('succeeded', 'failed', 'cancelled') ORDER BY id COLLATE "C"`)
	if err != nil {
		return nil, fmt.Errorf("postgres: open executions: %w", err)
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("postgres: open executions: %w", err)
		}
		out = append(out, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: open executions: %w", err)
	}
	return out, nil
}

func (s *ExecutionStore) Events(ctx context.Context, id string) ([]domain.ExecutionEvent, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, fmt.Errorf("postgres: events: %w", err)
	}
	defer tx.Rollback()
	var one int
	if err := tx.QueryRowContext(ctx, `SELECT 1 FROM executions WHERE id = $1`, id).Scan(&one); errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%w: execution %s", domain.ErrNotFound, id)
	} else if err != nil {
		return nil, fmt.Errorf("postgres: events: %w", err)
	}
	rows, err := tx.QueryContext(ctx,
		`SELECT execution_id, seq, kind, step_id, reason, at FROM execution_events WHERE execution_id = $1 ORDER BY seq`, id)
	if err != nil {
		return nil, fmt.Errorf("postgres: events: %w", err)
	}
	defer rows.Close()
	out := []domain.ExecutionEvent{}
	for rows.Next() {
		var ev domain.ExecutionEvent
		if err := rows.Scan(&ev.ExecutionID, &ev.Seq, &ev.Kind, &ev.StepID, &ev.Reason, &ev.At); err != nil {
			return nil, fmt.Errorf("postgres: events: %w", err)
		}
		out = append(out, ev)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: events: %w", err)
	}
	return out, nil
}

func (s *ExecutionStore) Queued(ctx context.Context, limit int) ([]ports.StepRef, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT st.execution_id, st.id FROM step_executions st JOIN executions e ON e.id = st.execution_id
		 WHERE st.state = 'queued' AND e.state NOT IN ('succeeded', 'failed', 'cancelled')
		 ORDER BY e.created_at, e.id COLLATE "C", st.ord LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("postgres: queued steps: %w", err)
	}
	defer rows.Close()
	out := []ports.StepRef{}
	for rows.Next() {
		var r ports.StepRef
		if err := rows.Scan(&r.ExecutionID, &r.StepID); err != nil {
			return nil, fmt.Errorf("postgres: queued steps: %w", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: queued steps: %w", err)
	}
	return out, nil
}

func (s *ExecutionStore) ByAttempt(ctx context.Context, attemptID string) (ports.StepRef, error) {
	var r ports.StepRef
	err := s.db.QueryRowContext(ctx,
		`SELECT execution_id, id FROM step_executions WHERE attempt_id = $1`, attemptID).Scan(&r.ExecutionID, &r.StepID)
	if errors.Is(err, sql.ErrNoRows) || attemptID == "" {
		return ports.StepRef{}, fmt.Errorf("%w: attempt %q", domain.ErrNotFound, attemptID)
	}
	if err != nil {
		return ports.StepRef{}, fmt.Errorf("postgres: step by attempt: %w", err)
	}
	return r, nil
}
