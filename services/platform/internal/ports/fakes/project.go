// Package fakes holds in-memory implementations of the ports, for tests that must not need a
// database.
package fakes

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
)

// ProjectStore is an in-memory ports.ProjectStore.
type ProjectStore struct {
	mu   sync.Mutex
	byID map[domain.ProjectKey]domain.Project
}

var _ ports.ProjectStore = (*ProjectStore)(nil)

func NewProjectStore() *ProjectStore {
	return &ProjectStore{byID: map[domain.ProjectKey]domain.Project{}}
}

func (s *ProjectStore) Create(_ context.Context, p domain.Project) error {
	if err := p.Validate(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.byID[p.Key]; ok {
		return fmt.Errorf("%w: project %s", domain.ErrExists, p.Key)
	}
	s.byID[p.Key] = p
	return nil
}

func (s *ProjectStore) Get(_ context.Context, key domain.ProjectKey) (domain.Project, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.byID[key]
	if !ok {
		return domain.Project{}, fmt.Errorf("%w: project %s", domain.ErrNotFound, key)
	}
	return p, nil
}

func (s *ProjectStore) List(_ context.Context) ([]domain.Project, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]domain.Project, 0, len(s.byID))
	for _, p := range s.byID {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out, nil
}
