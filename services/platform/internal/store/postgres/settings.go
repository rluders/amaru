package postgres

import (
	"context"
	"database/sql"
	"errors"

	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
)

var _ ports.SettingsStore = (*ProjectStore)(nil)

func (s *ProjectStore) Settings(ctx context.Context, key domain.ProjectKey) (domain.ProjectSettings, error) {
	var value domain.ProjectSettings
	err := s.db.QueryRowContext(ctx, `SELECT COALESCE(s.import_claude_skills, false)
 FROM projects p LEFT JOIN project_settings s ON s.project_key = p.key WHERE p.key = $1`, string(key)).Scan(&value.ImportClaudeSkills)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ProjectSettings{}, domain.ErrNotFound
	}
	return value, err
}

func (s *ProjectStore) UpdateSettings(ctx context.Context, key domain.ProjectKey, patch domain.UpdateSettings) (domain.ProjectSettings, error) {
	if err := patch.Validate(); err != nil {
		return domain.ProjectSettings{}, err
	}
	var value domain.ProjectSettings
	// The upsert serializes competing writes, including concurrent first updates.
	err := s.db.QueryRowContext(ctx, `INSERT INTO project_settings (project_key, import_claude_skills)
 SELECT key, $2 FROM projects WHERE key = $1
 ON CONFLICT (project_key) DO UPDATE SET import_claude_skills = EXCLUDED.import_claude_skills
 RETURNING import_claude_skills`, string(key), *patch.ImportClaudeSkills).Scan(&value.ImportClaudeSkills)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ProjectSettings{}, domain.ErrNotFound
	}
	return value, err
}
