package fakes

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"

	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
)

// ExecutionStore is an in-memory ports.ExecutionStore. Rows are copied on the way in and out, so
// a caller never shares memory with the store.
type ExecutionStore struct {
	mu     sync.Mutex
	execs  map[string]domain.Execution
	steps  map[string][]domain.StepExecution // by Execution id, creation order
	events map[string][]domain.ExecutionEvent
}

var _ ports.ExecutionStore = (*ExecutionStore)(nil)

func NewExecutionStore() *ExecutionStore {
	return &ExecutionStore{
		execs:  map[string]domain.Execution{},
		steps:  map[string][]domain.StepExecution{},
		events: map[string][]domain.ExecutionEvent{},
	}
}

func clone[T any](v T) T {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err) // domain types are plain data
	}
	var out T
	if err := json.Unmarshal(b, &out); err != nil {
		panic(err)
	}
	return out
}

func (s *ExecutionStore) Create(_ context.Context, e domain.Execution, steps []domain.StepExecution) (domain.Execution, []domain.StepExecution, error) {
	if e.ID == "" {
		return domain.Execution{}, nil, fmt.Errorf("%w: execution needs an id", domain.ErrInvalid)
	}
	seen := map[string]bool{}
	for _, st := range steps {
		if st.ID == "" || st.ExecutionID != e.ID || seen[st.ID] {
			return domain.Execution{}, nil, fmt.Errorf("%w: step %q does not belong to execution %s", domain.ErrInvalid, st.ID, e.ID)
		}
		seen[st.ID] = true
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.execs[e.ID]; ok {
		return domain.Execution{}, nil, fmt.Errorf("%w: execution %s", domain.ErrExists, e.ID)
	}
	for _, other := range s.steps {
		for _, o := range other {
			if seen[o.ID] {
				return domain.Execution{}, nil, fmt.Errorf("%w: step %s", domain.ErrExists, o.ID)
			}
		}
	}
	e = clone(e)
	e.Version = 1
	out := make([]domain.StepExecution, len(steps))
	for i, st := range steps {
		st = clone(st)
		st.Version = 1
		out[i] = st
	}
	s.execs[e.ID] = e
	s.steps[e.ID] = out
	return clone(e), clone(out), nil
}

func (s *ExecutionStore) Commit(_ context.Context, c ports.ExecutionChange) (ports.ExecutionChange, error) {
	evExec, err := c.ExecutionID()
	if err != nil {
		return ports.ExecutionChange{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	// Validate everything before writing anything.
	if c.Execution != nil {
		cur, ok := s.execs[c.Execution.ID]
		if !ok {
			return ports.ExecutionChange{}, fmt.Errorf("%w: execution %s", domain.ErrNotFound, c.Execution.ID)
		}
		if cur.Version != c.Execution.Version {
			return ports.ExecutionChange{}, fmt.Errorf("%w: execution %s is at version %d, change read %d", domain.ErrConflict, cur.ID, cur.Version, c.Execution.Version)
		}
	}
	index := func(st domain.StepExecution) (int, error) {
		for i, cur := range s.steps[st.ExecutionID] {
			if cur.ID == st.ID {
				if cur.Version != st.Version {
					return 0, fmt.Errorf("%w: step %s is at version %d, change read %d", domain.ErrConflict, cur.ID, cur.Version, st.Version)
				}
				return i, nil
			}
		}
		return 0, fmt.Errorf("%w: step %s", domain.ErrNotFound, st.ID)
	}
	idx := make([]int, len(c.Steps))
	for i, st := range c.Steps {
		n, err := index(st)
		if err != nil {
			return ports.ExecutionChange{}, err
		}
		idx[i] = n
	}
	// An attempt id names one Step.
	changing := map[string]bool{}
	for _, st := range c.Steps {
		changing[st.ID] = true
	}
	claimed := map[string]string{}
	for _, st := range c.Steps {
		if st.AttemptID == "" {
			continue
		}
		if other, dup := claimed[st.AttemptID]; dup && other != st.ID {
			return ports.ExecutionChange{}, fmt.Errorf("%w: attempt %s", domain.ErrExists, st.AttemptID)
		}
		claimed[st.AttemptID] = st.ID
	}
	for _, steps := range s.steps {
		for _, cur := range steps {
			if owner, ok := claimed[cur.AttemptID]; ok && cur.AttemptID != "" && owner != cur.ID && !changing[cur.ID] {
				return ports.ExecutionChange{}, fmt.Errorf("%w: attempt %s", domain.ErrExists, cur.AttemptID)
			}
		}
	}

	out := ports.ExecutionChange{}
	if c.Execution != nil {
		e := clone(*c.Execution)
		e.Version++
		s.execs[e.ID] = e
		cp := clone(e)
		out.Execution = &cp
	}
	for i, st := range c.Steps {
		st = clone(st)
		st.Version++
		s.steps[st.ExecutionID][idx[i]] = st
		out.Steps = append(out.Steps, clone(st))
	}
	for _, ev := range c.Events {
		ev = clone(ev)
		ev.ExecutionID = evExec
		ev.Seq = int64(len(s.events[evExec])) + 1
		s.events[evExec] = append(s.events[evExec], ev)
		out.Events = append(out.Events, clone(ev))
	}
	return out, nil
}

func (s *ExecutionStore) Get(_ context.Context, id string) (domain.Execution, []domain.StepExecution, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.execs[id]
	if !ok {
		return domain.Execution{}, nil, fmt.Errorf("%w: execution %s", domain.ErrNotFound, id)
	}
	return clone(e), clone(s.steps[id]), nil
}

func (s *ExecutionStore) Open(_ context.Context) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []string{}
	for id, e := range s.execs {
		if !e.State.Terminal() {
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out, nil
}

func (s *ExecutionStore) Events(_ context.Context, id string) ([]domain.ExecutionEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.execs[id]; !ok {
		return nil, fmt.Errorf("%w: execution %s", domain.ErrNotFound, id)
	}
	return clone(append([]domain.ExecutionEvent{}, s.events[id]...)), nil
}

func (s *ExecutionStore) Queued(_ context.Context, limit int) ([]ports.StepRef, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ids := make([]string, 0, len(s.execs))
	for id, e := range s.execs {
		if !e.State.Terminal() {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(a, b int) bool {
		ea, eb := s.execs[ids[a]], s.execs[ids[b]]
		if !ea.CreatedAt.Equal(eb.CreatedAt) {
			return ea.CreatedAt.Before(eb.CreatedAt)
		}
		return ids[a] < ids[b]
	})
	out := []ports.StepRef{}
	for _, id := range ids {
		for _, st := range s.steps[id] {
			if st.State == domain.StepQueued && len(out) < limit {
				out = append(out, ports.StepRef{ExecutionID: id, StepID: st.ID})
			}
		}
	}
	return out, nil
}

func (s *ExecutionStore) ByAttempt(_ context.Context, attemptID string) (ports.StepRef, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if attemptID != "" {
		for id, steps := range s.steps {
			for _, st := range steps {
				if st.AttemptID == attemptID {
					return ports.StepRef{ExecutionID: id, StepID: st.ID}, nil
				}
			}
		}
	}
	return ports.StepRef{}, fmt.Errorf("%w: attempt %q", domain.ErrNotFound, attemptID)
}
