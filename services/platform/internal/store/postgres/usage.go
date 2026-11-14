package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/rluders/amaru/services/platform/internal/domain"
)

func insertCapture(ctx context.Context, tx *sql.Tx, c domain.ResultCapture) error {
	raw, err := json.Marshal(c)
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO result_captures(attempt_id,execution_id,step_id,captured_at,receipt) VALUES($1,$2,$3,$4,$5)`, c.AttemptID, c.ExecutionID, c.StepID, c.CapturedAt.UTC(), raw); err != nil {
		return fmt.Errorf("postgres: insert capture: %w", err)
	}
	for i := range c.Reports {
		if _, err = tx.ExecContext(ctx, `INSERT INTO usage_publication_intents(usage_id,attempt_id,ordinal) VALUES($1,$2,$3)`, c.UsageID(i), c.AttemptID, i); err != nil {
			return fmt.Errorf("postgres: usage publication intent: %w", err)
		}
	}
	return nil
}
func (s *ExecutionStore) ResultCapture(ctx context.Context, attempt string) (domain.ResultCapture, error) {
	var raw []byte
	err := s.db.QueryRowContext(ctx, `SELECT receipt FROM result_captures WHERE attempt_id=$1`, attempt).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ResultCapture{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.ResultCapture{}, err
	}
	var c domain.ResultCapture
	err = json.Unmarshal(raw, &c)
	return c, err
}
func (s *ExecutionStore) UsageCaptures(ctx context.Context, execution string) ([]domain.ResultCapture, error) {
	var exists bool
	if err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM executions WHERE id=$1)`, execution).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, domain.ErrNotFound
	}
	rows, err := s.db.QueryContext(ctx, `SELECT c.receipt FROM result_captures c JOIN step_executions st ON st.id=c.step_id WHERE c.execution_id=$1 ORDER BY st.ord,c.captured_at,c.attempt_id COLLATE "C"`, execution)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.ResultCapture
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var c domain.ResultCapture
		if err := json.Unmarshal(raw, &c); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
