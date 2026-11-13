package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
)

var _ ports.ProjectCloneStore = (*ProjectStore)(nil)

func (s *ProjectStore) RegisterClone(ctx context.Context, p domain.Project, clone domain.ProjectClone) (domain.Project, bool, error) {
	if err := p.Validate(); err != nil {
		return domain.Project{}, false, err
	}
	if clone.ProjectKey != p.Key || clone.Root == "" || strings.ContainsRune(clone.Root, 0) {
		return domain.Project{}, false, domain.ErrInvalid
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.Project{}, false, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `INSERT INTO projects (key,name,created_at) VALUES ($1,$2,$3) ON CONFLICT (key) DO NOTHING`, string(p.Key), p.Name, p.CreatedAt.UTC())
	if err != nil {
		return domain.Project{}, false, fmt.Errorf("postgres: register project: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return domain.Project{}, false, err
	}
	// Serialize selection until commit so response order is not mistaken for selection order.
	err = tx.QueryRowContext(ctx, `SELECT name,created_at FROM projects WHERE key=$1 FOR UPDATE`, string(p.Key)).Scan(&p.Name, &p.CreatedAt)
	if err != nil {
		return domain.Project{}, false, fmt.Errorf("postgres: lock project: %w", err)
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO project_clones (project_key,root) VALUES ($1,$2) ON CONFLICT (project_key) DO UPDATE SET root=EXCLUDED.root`, string(p.Key), clone.Root)
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == uniqueViolation {
		return domain.Project{}, false, domain.ErrExists
	}
	if err != nil {
		return domain.Project{}, false, fmt.Errorf("postgres: select clone: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return domain.Project{}, false, err
	}
	p.CreatedAt = p.CreatedAt.UTC()
	return p, count == 1, nil
}

func (s *ProjectStore) ActiveClone(ctx context.Context, key domain.ProjectKey) (domain.ProjectClone, error) {
	clone := domain.ProjectClone{ProjectKey: key}
	err := s.db.QueryRowContext(ctx, `SELECT root FROM project_clones WHERE project_key=$1`, string(key)).Scan(&clone.Root)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ProjectClone{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.ProjectClone{}, fmt.Errorf("postgres: read active clone: %w", err)
	}
	return clone, nil
}
