package fakes

import (
	"context"
	"fmt"
	"sync"

	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
)

// SkillStore is an in-memory ports.SkillStore.
type SkillStore struct {
	mu   sync.Mutex
	rows map[skillKey]domain.Skill
}

type skillKey struct {
	scope   domain.SkillScope
	project domain.ProjectKey
	name    string
	version string
}

var _ ports.SkillStore = (*SkillStore)(nil)

func NewSkillStore() *SkillStore { return &SkillStore{rows: map[skillKey]domain.Skill{}} }

func (s *SkillStore) Add(_ context.Context, sk domain.Skill) error {
	if err := sk.Validate(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	k := skillKey{sk.Scope, sk.ProjectKey, sk.Name, sk.Version}
	if old, ok := s.rows[k]; ok {
		if old.Hash != sk.Hash {
			return fmt.Errorf("%w: skill %s %s already exists with other content", domain.ErrConflict, sk.Name, sk.Version)
		}
		return nil
	}
	s.rows[k] = sk
	return nil
}

func (s *SkillStore) Get(_ context.Context, project domain.ProjectKey, name, version string) (domain.Skill, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if project != "" {
		if sk, ok := s.rows[skillKey{domain.SkillProject, project, name, version}]; ok {
			return sk, nil
		}
	}
	if sk, ok := s.rows[skillKey{domain.SkillOrganization, "", name, version}]; ok {
		return sk, nil
	}
	return domain.Skill{}, fmt.Errorf("%w: skill %s %s", domain.ErrNotFound, name, version)
}
