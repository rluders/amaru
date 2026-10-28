package fakes

import (
	"context"
	"fmt"
	"sync"

	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
)

// ContextStore is an in-memory ports.ContextStore.
type ContextStore struct {
	mu   sync.Mutex
	rows map[[2]string]domain.StepContext
}

var _ ports.ContextStore = (*ContextStore)(nil)

func NewContextStore() *ContextStore { return &ContextStore{rows: map[[2]string]domain.StepContext{}} }

func (s *ContextStore) Record(_ context.Context, c domain.StepContext) (domain.StepContext, error) {
	if err := c.Validate(); err != nil {
		return domain.StepContext{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	k := [2]string{c.ExecutionID, c.StepID}
	if old, ok := s.rows[k]; ok {
		if old.Hash != c.Hash {
			return domain.StepContext{}, fmt.Errorf("%w: step %s already has a different context", domain.ErrConflict, c.StepID)
		}
		return clone(old), nil
	}
	s.rows[k] = clone(c)
	return clone(c), nil
}

func (s *ContextStore) Get(_ context.Context, executionID, stepID string) (domain.StepContext, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.rows[[2]string{executionID, stepID}]
	if !ok {
		return domain.StepContext{}, fmt.Errorf("%w: context of step %s", domain.ErrNotFound, stepID)
	}
	return clone(c), nil
}
