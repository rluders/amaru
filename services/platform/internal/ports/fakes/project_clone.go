package fakes

import (
	"context"
	"strings"

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
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return domain.Project{}, false, err
	}
	for key, root := range s.clones {
		if root == clone.Root && key != p.Key {
			return domain.Project{}, false, domain.ErrExists
		}
	}
	existing, ok := s.byID[p.Key]
	if ok {
		p = existing
	}
	if s.clones == nil {
		s.clones = map[domain.ProjectKey]string{}
	}
	s.byID[p.Key] = p
	s.clones[p.Key] = clone.Root
	return p, !ok, nil
}

func (s *ProjectStore) ActiveClone(ctx context.Context, key domain.ProjectKey) (domain.ProjectClone, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return domain.ProjectClone{}, err
	}
	root, ok := s.clones[key]
	if !ok {
		return domain.ProjectClone{}, domain.ErrNotFound
	}
	return domain.ProjectClone{ProjectKey: key, Root: root}, nil
}
