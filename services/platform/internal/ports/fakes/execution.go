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
	cmds   map[[2]string]domain.CommandRecord
	apprs  map[string]domain.Approval // by id
	byExec map[string][]string        // Approval ids by Execution id, request order
	log    *AuditLog
}

var (
	_ ports.ExecutionStore = (*ExecutionStore)(nil)
	_ ports.AuditLog       = (*ExecutionStore)(nil)
)

func NewExecutionStore() *ExecutionStore {
	return &ExecutionStore{
		execs:  map[string]domain.Execution{},
		steps:  map[string][]domain.StepExecution{},
		events: map[string][]domain.ExecutionEvent{},
		cmds:   map[[2]string]domain.CommandRecord{},
		apprs:  map[string]domain.Approval{},
		byExec: map[string][]string{},
		log:    &AuditLog{},
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
	return s.create(nil, e, steps, nil)
}

func (s *ExecutionStore) CreateOnce(_ context.Context, rec domain.CommandRecord, e domain.Execution, steps []domain.StepExecution, approvals ...domain.Approval) (domain.Execution, []domain.StepExecution, error) {
	if err := rec.Valid(); err != nil {
		return domain.Execution{}, nil, err
	}
	return s.create(&rec, e, steps, approvals)
}

func (s *ExecutionStore) Command(_ context.Context, name, key string) (domain.CommandRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.cmds[[2]string{name, key}]
	if !ok {
		return domain.CommandRecord{}, fmt.Errorf("%w: command %s %q", domain.ErrNotFound, name, key)
	}
	return clone(rec), nil
}

func (s *ExecutionStore) create(rec *domain.CommandRecord, e domain.Execution, steps []domain.StepExecution, approvals []domain.Approval) (domain.Execution, []domain.StepExecution, error) {
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
	var requested []domain.AuditEvent
	for _, a := range approvals {
		ev, err := domain.ApprovalAuditEvent(nil, a)
		if err == nil {
			err = a.Validate()
		}
		if err == nil && (a.Gate.ExecutionID != e.ID || !seen[a.Gate.StepID]) {
			err = fmt.Errorf("%w: approval %s does not gate a step of execution %s", domain.ErrInvalid, a.ID, e.ID)
		}
		if err != nil {
			return domain.Execution{}, nil, err
		}
		requested = append(requested, ev)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, a := range approvals {
		if _, ok := s.apprs[a.ID]; ok {
			return domain.Execution{}, nil, fmt.Errorf("%w: approval %s", domain.ErrExists, a.ID)
		}
	}
	if rec != nil {
		if _, dup := s.cmds[[2]string{rec.Name, rec.Key}]; dup {
			return domain.Execution{}, nil, fmt.Errorf("%w: %s %q", domain.ErrDuplicateCommand, rec.Name, rec.Key)
		}
	}
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
	if rec != nil {
		s.cmds[[2]string{rec.Name, rec.Key}] = clone(*rec)
	}
	for i, a := range approvals {
		a = clone(a)
		a.Version = 1
		s.apprs[a.ID] = a
		s.byExec[e.ID] = append(s.byExec[e.ID], a.ID)
		s.log.append(requested[i])
	}
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
	if c.Command != nil {
		if err := c.Command.Valid(); err != nil {
			return ports.ExecutionChange{}, err
		}
		if _, dup := s.cmds[[2]string{c.Command.Name, c.Command.Key}]; dup {
			return ports.ExecutionChange{}, fmt.Errorf("%w: %s %q", domain.ErrDuplicateCommand, c.Command.Name, c.Command.Key)
		}
	}
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
	var apprEvents []domain.AuditEvent
	for _, a := range c.Approvals {
		cur, ok := s.apprs[a.ID]
		if !ok || cur.Gate.ExecutionID != evExec {
			return ports.ExecutionChange{}, fmt.Errorf("%w: approval %s", domain.ErrNotFound, a.ID)
		}
		if cur.Version != a.Version {
			return ports.ExecutionChange{}, fmt.Errorf("%w: approval %s is at version %d, change read %d", domain.ErrConflict, a.ID, cur.Version, a.Version)
		}
		ev, err := domain.ApprovalAuditEvent(&cur, a)
		if err == nil {
			err = a.Validate()
		}
		if err == nil && (a.Gate != cur.Gate || a.Requester != cur.Requester || a.Evidence.Digest != cur.Evidence.Digest || a.MinRole != cur.MinRole || a.ProjectID != cur.ProjectID) {
			err = fmt.Errorf("%w: approval %s: only its state, decision and void record may change", domain.ErrInvalid, a.ID)
		}
		if err != nil {
			return ports.ExecutionChange{}, err
		}
		apprEvents = append(apprEvents, ev)
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
		e.ProjectID, e.EnvironmentID = s.execs[e.ID].ProjectID, s.execs[e.ID].EnvironmentID // scopes never change
		e.Definition = s.execs[e.ID].Definition                                             // the pinned definition never changes
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
	if c.Command != nil {
		s.cmds[[2]string{c.Command.Name, c.Command.Key}] = clone(*c.Command)
	}
	for i, a := range c.Approvals {
		a = clone(a)
		a.Version++
		s.apprs[a.ID] = a
		s.log.append(apprEvents[i])
		out.Approvals = append(out.Approvals, clone(a))
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
				out = append(out, ports.StepRef{ExecutionID: id, StepID: st.ID, ProjectID: s.execs[id].ProjectID, EnvironmentID: s.execs[id].EnvironmentID})
			}
		}
	}
	return out, nil
}

func (s *ExecutionStore) Active(_ context.Context) ([]ports.StepRef, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []ports.StepRef{}
	for id, e := range s.execs {
		if e.State.Terminal() {
			continue
		}
		for _, st := range s.steps[id] {
			if st.State == domain.StepLeased || st.State == domain.StepRunning {
				out = append(out, ports.StepRef{ExecutionID: id, StepID: st.ID, ProjectID: e.ProjectID, EnvironmentID: e.EnvironmentID})
			}
		}
	}
	sort.Slice(out, func(a, b int) bool {
		return out[a].ExecutionID+"/"+out[a].StepID < out[b].ExecutionID+"/"+out[b].StepID
	})
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

func (s *ExecutionStore) Approval(_ context.Context, id string) (domain.Approval, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.apprs[id]
	if !ok {
		return domain.Approval{}, fmt.Errorf("%w: approval %s", domain.ErrNotFound, id)
	}
	return clone(a), nil
}

func (s *ExecutionStore) Approvals(_ context.Context, executionID string) ([]domain.Approval, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.execs[executionID]; !ok {
		return nil, fmt.Errorf("%w: execution %s", domain.ErrNotFound, executionID)
	}
	out := []domain.Approval{}
	for _, id := range s.byExec[executionID] {
		out = append(out, clone(s.apprs[id]))
	}
	return out, nil
}

// Audit implements ports.AuditLog.
func (s *ExecutionStore) Audit(ctx context.Context, after int64, limit int) ([]domain.AuditEvent, error) {
	return s.log.Audit(ctx, after, limit)
}
