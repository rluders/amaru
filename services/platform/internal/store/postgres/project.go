package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
)

// uniqueViolation is the PostgreSQL SQLSTATE for a unique or primary key violation.
const uniqueViolation = "23505"

// ProjectStore is the PostgreSQL ports.ProjectStore.
type ProjectStore struct{ db *sql.DB }

var _ ports.ProjectStore = (*ProjectStore)(nil)

// NewProjectStore returns a store over db, which must be connected through the pgx stdlib driver
// to a database at the current schema.
func NewProjectStore(db *sql.DB) *ProjectStore { return &ProjectStore{db: db} }

func (s *ProjectStore) Create(ctx context.Context, p domain.Project) error {
	if err := p.Validate(); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO projects (key, name, created_at) VALUES ($1, $2, $3)`,
		string(p.Key), p.Name, p.CreatedAt.UTC())
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == uniqueViolation {
		return fmt.Errorf("%w: project %s", domain.ErrExists, p.Key)
	}
	if err != nil {
		return fmt.Errorf("postgres: create project: %w", err)
	}
	return nil
}

func (s *ProjectStore) Get(ctx context.Context, key domain.ProjectKey) (domain.Project, error) {
	var p domain.Project
	var k string
	err := s.db.QueryRowContext(ctx,
		`SELECT key, name, created_at FROM projects WHERE key = $1`, string(key)).
		Scan(&k, &p.Name, &p.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Project{}, fmt.Errorf("%w: project %s", domain.ErrNotFound, key)
	}
	if err != nil {
		return domain.Project{}, fmt.Errorf("postgres: get project: %w", err)
	}
	p.Key = domain.ProjectKey(k)
	return p, nil
}

func (s *ProjectStore) List(ctx context.Context) ([]domain.Project, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT key, name, created_at FROM projects ORDER BY key COLLATE "C"`)
	if err != nil {
		return nil, fmt.Errorf("postgres: list projects: %w", err)
	}
	defer rows.Close()
	out := []domain.Project{}
	for rows.Next() {
		var p domain.Project
		var k string
		if err := rows.Scan(&k, &p.Name, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("postgres: list projects: %w", err)
		}
		p.Key = domain.ProjectKey(k)
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: list projects: %w", err)
	}
	return out, nil
}
