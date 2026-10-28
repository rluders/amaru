package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
)

// SkillStore is the PostgreSQL ports.SkillStore.
type SkillStore struct{ db *sql.DB }

var _ ports.SkillStore = (*SkillStore)(nil)

// NewSkillStore returns a store over db, which must be connected through the pgx stdlib driver to
// a database at the current schema.
func NewSkillStore(db *sql.DB) *SkillStore { return &SkillStore{db: db} }

const skillColumns = `scope, project_key, name, version, body, hash, added_by, added_at`

func (s *SkillStore) Add(ctx context.Context, sk domain.Skill) error {
	if err := sk.Validate(); err != nil {
		return err
	}
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO skills (`+skillColumns+`) VALUES ($1, $2, $3, $4, $5, $6, $7, $8) ON CONFLICT DO NOTHING`,
		string(sk.Scope), string(sk.ProjectKey), sk.Name, sk.Version, sk.Body, sk.Hash, sk.AddedBy, sk.AddedAt.UTC())
	if err != nil {
		return fmt.Errorf("postgres: add skill: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 1 {
		return nil
	}
	// The key exists: the same body is a no-op, another body is a conflict.
	var hash string
	if err := s.db.QueryRowContext(ctx,
		`SELECT hash FROM skills WHERE scope = $1 AND project_key = $2 AND name = $3 AND version = $4`,
		string(sk.Scope), string(sk.ProjectKey), sk.Name, sk.Version).Scan(&hash); err != nil {
		return fmt.Errorf("postgres: add skill: %w", err)
	}
	if hash != sk.Hash {
		return fmt.Errorf("%w: skill %s %s already exists with other content", domain.ErrConflict, sk.Name, sk.Version)
	}
	return nil
}

func (s *SkillStore) Get(ctx context.Context, project domain.ProjectKey, name, version string) (domain.Skill, error) {
	// A Project's own Skill sorts before the Organization's: 'project' > 'organization', so
	// order descending by scope.
	row := s.db.QueryRowContext(ctx,
		`SELECT `+skillColumns+` FROM skills
		 WHERE name = $2 AND version = $3 AND (scope = 'organization' OR (scope = 'project' AND project_key = $1 AND $1 <> ''))
		 ORDER BY scope DESC LIMIT 1`, string(project), name, version)
	var sk domain.Skill
	var scope, key string
	err := row.Scan(&scope, &key, &sk.Name, &sk.Version, &sk.Body, &sk.Hash, &sk.AddedBy, &sk.AddedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Skill{}, fmt.Errorf("%w: skill %s %s", domain.ErrNotFound, name, version)
	}
	if err != nil {
		return domain.Skill{}, fmt.Errorf("postgres: get skill: %w", err)
	}
	sk.Scope, sk.ProjectKey, sk.AddedAt = domain.SkillScope(scope), domain.ProjectKey(key), sk.AddedAt.UTC()
	return sk, nil
}
