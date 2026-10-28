package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
)

// ContextStore is the PostgreSQL ports.ContextStore.
type ContextStore struct{ db *sql.DB }

var _ ports.ContextStore = (*ContextStore)(nil)

// NewContextStore returns a store over db, which must be connected through the pgx stdlib driver
// to a database at the current schema.
func NewContextStore(db *sql.DB) *ContextStore { return &ContextStore{db: db} }

const contextColumns = `execution_id, step_id, format, hash, document, items, created_at`

func (s *ContextStore) Record(ctx context.Context, c domain.StepContext) (domain.StepContext, error) {
	if err := c.Validate(); err != nil {
		return domain.StepContext{}, err
	}
	items, err := json.Marshal(c.Items)
	if err != nil {
		return domain.StepContext{}, fmt.Errorf("postgres: encode context items: %w", err)
	}
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO step_contexts (`+contextColumns+`) VALUES ($1, $2, $3, $4, $5, $6, $7) ON CONFLICT DO NOTHING`,
		c.ExecutionID, c.StepID, c.Format, c.Hash, c.Document, items, c.CreatedAt.UTC())
	if err != nil {
		return domain.StepContext{}, fmt.Errorf("postgres: record context: %w", err)
	}
	old, err := s.Get(ctx, c.ExecutionID, c.StepID)
	if err != nil {
		return domain.StepContext{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 && old.Hash != c.Hash {
		return domain.StepContext{}, fmt.Errorf("%w: step %s already has a different context", domain.ErrConflict, c.StepID)
	}
	return old, nil
}

func (s *ContextStore) Get(ctx context.Context, executionID, stepID string) (domain.StepContext, error) {
	var c domain.StepContext
	var items []byte
	err := s.db.QueryRowContext(ctx, `SELECT `+contextColumns+` FROM step_contexts WHERE execution_id = $1 AND step_id = $2`, executionID, stepID).
		Scan(&c.ExecutionID, &c.StepID, &c.Format, &c.Hash, &c.Document, &items, &c.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.StepContext{}, fmt.Errorf("%w: context of step %s", domain.ErrNotFound, stepID)
	}
	if err != nil {
		return domain.StepContext{}, fmt.Errorf("postgres: get context: %w", err)
	}
	if err := json.Unmarshal(items, &c.Items); err != nil {
		return domain.StepContext{}, fmt.Errorf("postgres: decode context items of step %s: %w", stepID, err)
	}
	c.CreatedAt = c.CreatedAt.UTC()
	return c, nil
}
