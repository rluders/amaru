package fakes

import (
	"context"

	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
)

var _ ports.SettingsStore = (*ProjectStore)(nil)

func (s *ProjectStore) Settings(ctx context.Context, key domain.ProjectKey) (domain.ProjectSettings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return domain.ProjectSettings{}, err
	}
	if _, ok := s.byID[key]; !ok {
		return domain.ProjectSettings{}, domain.ErrNotFound
	}
	return s.settings[key], nil
}

func (s *ProjectStore) UpdateSettings(ctx context.Context, key domain.ProjectKey, patch domain.UpdateSettings) (domain.ProjectSettings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return domain.ProjectSettings{}, err
	}
	if _, ok := s.byID[key]; !ok {
		return domain.ProjectSettings{}, domain.ErrNotFound
	}
	if err := patch.Validate(); err != nil {
		return domain.ProjectSettings{}, err
	}
	value := s.settings[key]
	value.ImportClaudeSkills = *patch.ImportClaudeSkills
	if s.settings == nil {
		s.settings = map[domain.ProjectKey]domain.ProjectSettings{}
	}
	s.settings[key] = value
	return value, nil
}
