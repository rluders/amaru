package fakes

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
)

// TrustStore is an in-memory ports.TrustStore.
type TrustStore struct {
	mu   sync.Mutex
	cmds map[[2]string]domain.CommandRecord
	rows map[trustKey]domain.Trust
	log  *AuditLog
	// projects, when set, makes Save refuse a Trust for a Project it does not hold, as the
	// PostgreSQL store does through its foreign key.
	projects ports.ProjectStore
}

type trustKey struct {
	project domain.ProjectKey
	kind    domain.TrustKind
	ref     string
}

var _ ports.TrustStore = (*TrustStore)(nil)

// NewTrustStore returns an empty store. projects may be nil.
func NewTrustStore(projects ports.ProjectStore) *TrustStore {
	return &TrustStore{rows: map[trustKey]domain.Trust{}, cmds: map[[2]string]domain.CommandRecord{}, log: &AuditLog{}, projects: projects}
}

func (s *TrustStore) Save(ctx context.Context, t domain.Trust) error {
	return s.save(ctx, t, nil)
}

func (s *TrustStore) SaveOnce(ctx context.Context, rec domain.CommandRecord, t domain.Trust) error {
	if err := rec.Valid(); err != nil {
		return err
	}
	return s.save(ctx, t, &rec)
}

func (s *TrustStore) save(ctx context.Context, t domain.Trust, rec *domain.CommandRecord) error {
	if err := t.Validate(); err != nil {
		return err
	}
	if s.projects != nil {
		if _, err := s.projects.Get(ctx, t.ProjectKey); err != nil {
			return err
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if rec != nil {
		if _, exists := s.cmds[[2]string{rec.Name, rec.Key}]; exists {
			return domain.ErrDuplicateCommand
		}
	}
	k := trustKey{t.ProjectKey, t.Kind, t.Ref}
	var prev *domain.Trust
	if old, ok := s.rows[k]; ok {
		prev = &old
	}
	if rec != nil && t.RevokedAt != nil && prev != nil &&
		(t.Hash != prev.Hash || t.TrustedBy != prev.TrustedBy || !t.TrustedAt.Equal(prev.TrustedAt)) {
		return domain.ErrConflict
	}
	ev, err := domain.TrustAuditEvent(prev, t)
	if err != nil {
		return err
	}
	if rec != nil {
		s.cmds[[2]string{rec.Name, rec.Key}] = clone(*rec)
	}
	s.rows[k] = cloneTrust(t)
	s.log.append(ev)
	return nil
}

func (s *TrustStore) Get(_ context.Context, key domain.ProjectKey, kind domain.TrustKind, ref string) (domain.Trust, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.rows[trustKey{key, kind, ref}]
	if !ok {
		return domain.Trust{}, fmt.Errorf("%w: trust %s %s %s", domain.ErrNotFound, key, kind, ref)
	}
	return cloneTrust(t), nil
}

func (s *TrustStore) List(_ context.Context, key domain.ProjectKey) ([]domain.Trust, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []domain.Trust{}
	for k, t := range s.rows {
		if k.project == key {
			out = append(out, cloneTrust(t))
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return out[i].Ref < out[j].Ref
	})
	return out, nil
}

// Audit implements ports.AuditLog.
func (s *TrustStore) Audit(ctx context.Context, after int64, limit int) ([]domain.AuditEvent, error) {
	return s.log.Audit(ctx, after, limit)
}

func cloneTrust(t domain.Trust) domain.Trust {
	if t.RevokedAt != nil {
		r := *t.RevokedAt
		t.RevokedAt = &r
	}
	return t
}

func (s *TrustStore) Command(_ context.Context, name, key string) (domain.CommandRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.cmds[[2]string{name, key}]
	if !ok {
		return domain.CommandRecord{}, domain.ErrNotFound
	}
	return clone(rec), nil
}
