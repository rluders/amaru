package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
)

// ArtifactIndex is the PostgreSQL ports.ArtifactIndex.
type ArtifactIndex struct{ db *sql.DB }

var _ ports.ArtifactIndex = (*ArtifactIndex)(nil)

// NewArtifactIndex returns an index over db, which must be connected through the pgx stdlib driver
// to a database at the current schema.
func NewArtifactIndex(db *sql.DB) *ArtifactIndex { return &ArtifactIndex{db: db} }

const artifactColumns = `artifact_id, version, hash, size, media_type, path, execution_id, step_id, attempt_id, created_at`

func (s *ArtifactIndex) Record(ctx context.Context, v domain.ArtifactVersion) (domain.ArtifactVersion, error) {
	if err := v.Validate(); err != nil {
		return domain.ArtifactVersion{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.ArtifactVersion{}, fmt.Errorf("postgres: record artifact: %w", err)
	}
	defer tx.Rollback()
	// One writer per Artifact at a time, so the next version number is not taken twice.
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, v.ArtifactID); err != nil {
		return domain.ArtifactVersion{}, fmt.Errorf("postgres: lock artifact %s: %w", v.ArtifactID, err)
	}
	old, err := scanArtifact(tx.QueryRowContext(ctx,
		`SELECT `+artifactColumns+` FROM artifact_versions
		 WHERE artifact_id = $1 AND execution_id = $2 AND step_id = $3 AND attempt_id = $4 AND path = $5`,
		v.ArtifactID, v.ExecutionID, v.StepID, v.AttemptID, v.Path))
	switch {
	case err == nil:
		return old, nil
	case !errors.Is(err, sql.ErrNoRows):
		return domain.ArtifactVersion{}, err
	}
	if err := tx.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(version), 0) + 1 FROM artifact_versions WHERE artifact_id = $1`, v.ArtifactID).Scan(&v.Version); err != nil {
		return domain.ArtifactVersion{}, fmt.Errorf("postgres: next artifact version: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO artifact_versions (`+artifactColumns+`) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		v.ArtifactID, v.Version, v.Hash, v.Size, v.MediaType, v.Path, v.ExecutionID, v.StepID, v.AttemptID, v.CreatedAt.UTC()); err != nil {
		return domain.ArtifactVersion{}, fmt.Errorf("postgres: record artifact %s: %w", v.ArtifactID, err)
	}
	if err := tx.Commit(); err != nil {
		return domain.ArtifactVersion{}, fmt.Errorf("postgres: record artifact: %w", err)
	}
	v.CreatedAt = v.CreatedAt.UTC()
	return v, nil
}

func (s *ArtifactIndex) Versions(ctx context.Context, id string) ([]domain.ArtifactVersion, error) {
	return s.query(ctx, `SELECT `+artifactColumns+` FROM artifact_versions WHERE artifact_id = $1 ORDER BY version`, id)
}

func (s *ArtifactIndex) ByExecution(ctx context.Context, exec string) ([]domain.ArtifactVersion, error) {
	return s.query(ctx, `SELECT `+artifactColumns+` FROM artifact_versions WHERE execution_id = $1
		ORDER BY step_id COLLATE "C", artifact_id COLLATE "C", version`, exec)
}

func (s *ArtifactIndex) query(ctx context.Context, q string, arg string) ([]domain.ArtifactVersion, error) {
	rows, err := s.db.QueryContext(ctx, q, arg)
	if err != nil {
		return nil, fmt.Errorf("postgres: list artifact versions: %w", err)
	}
	defer rows.Close()
	out := []domain.ArtifactVersion{}
	for rows.Next() {
		v, err := scanArtifact(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func scanArtifact(r rowScanner) (domain.ArtifactVersion, error) {
	var v domain.ArtifactVersion
	err := r.Scan(&v.ArtifactID, &v.Version, &v.Hash, &v.Size, &v.MediaType, &v.Path, &v.ExecutionID, &v.StepID, &v.AttemptID, &v.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.ArtifactVersion{}, err
		}
		return domain.ArtifactVersion{}, fmt.Errorf("postgres: read artifact version: %w", err)
	}
	v.CreatedAt = v.CreatedAt.UTC()
	return v, nil
}
