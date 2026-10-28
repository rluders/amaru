package fakes

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
)

// GrantStore is an in-memory ports.GrantStore.
type GrantStore struct {
	mu   sync.Mutex
	byID map[string]domain.Grant
	log  []domain.AuditEvent
}

var (
	_ ports.GrantStore = (*GrantStore)(nil)
	_ ports.AuditLog   = (*GrantStore)(nil)
)

func NewGrantStore() *GrantStore { return &GrantStore{byID: map[string]domain.Grant{}} }

func (s *GrantStore) Create(_ context.Context, g domain.Grant) (domain.Grant, error) {
	if err := g.Validate(); err != nil {
		return domain.Grant{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.byID[g.ID]; ok {
		return domain.Grant{}, fmt.Errorf("%w: grant %s", domain.ErrExists, g.ID)
	}
	ev, err := domain.GrantAuditEvent(nil, g, time.Now().UTC())
	if err != nil {
		return domain.Grant{}, err
	}
	g.Version = 1
	s.byID[g.ID] = cloneGrant(g)
	s.appendAudit(ev)
	return cloneGrant(g), nil
}

func (s *GrantStore) Get(_ context.Context, id string) (domain.Grant, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	g, ok := s.byID[id]
	if !ok {
		return domain.Grant{}, fmt.Errorf("%w: grant %s", domain.ErrNotFound, id)
	}
	return cloneGrant(g), nil
}

func (s *GrantStore) Update(_ context.Context, g domain.Grant) (domain.Grant, error) {
	if err := g.Validate(); err != nil {
		return domain.Grant{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	old, ok := s.byID[g.ID]
	if !ok {
		return domain.Grant{}, fmt.Errorf("%w: grant %s", domain.ErrNotFound, g.ID)
	}
	if old.Version != g.Version {
		return domain.Grant{}, fmt.Errorf("%w: grant %s is at version %d, not %d", domain.ErrConflict, g.ID, old.Version, g.Version)
	}
	next := cloneGrant(g)
	// Only the status and the revocation record may differ from what is stored.
	next.Status, next.RevokedBy, next.RevokedAt, next.RevokeReason, next.Version = old.Status, old.RevokedBy, old.RevokedAt, old.RevokeReason, old.Version
	if !sameGrant(next, old) {
		return domain.Grant{}, fmt.Errorf("%w: grant %s: only its status and revocation may change", domain.ErrInvalid, g.ID)
	}
	if old.Status != domain.GrantActive && g.Status != old.Status {
		return domain.Grant{}, fmt.Errorf("%w: grant %s is %s", domain.ErrInvalidTransition, g.ID, old.Status)
	}
	if g.Status != old.Status {
		ev, err := domain.GrantAuditEvent(&old, g, time.Now().UTC())
		if err != nil {
			return domain.Grant{}, err
		}
		s.appendAudit(ev)
	}
	g.Version = old.Version + 1
	s.byID[g.ID] = cloneGrant(g)
	return cloneGrant(g), nil
}

func (s *GrantStore) Active(_ context.Context, projectID string) ([]domain.Grant, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []domain.Grant
	for _, g := range s.byID {
		if g.Status == domain.GrantActive && (g.Level == domain.GrantOrganization || g.ProjectID == projectID) {
			out = append(out, cloneGrant(g))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func cloneGrant(g domain.Grant) domain.Grant {
	g.Capabilities = append([]domain.Capability(nil), g.Capabilities...)
	if g.ExpiresAt != nil {
		t := *g.ExpiresAt
		g.ExpiresAt = &t
	}
	if g.RevokedAt != nil {
		t := *g.RevokedAt
		g.RevokedAt = &t
	}
	return g
}

// sameGrant compares the fields a human set.
func sameGrant(a, b domain.Grant) bool {
	if len(a.Capabilities) != len(b.Capabilities) {
		return false
	}
	for i := range a.Capabilities {
		if a.Capabilities[i] != b.Capabilities[i] {
			return false
		}
	}
	return a.ID == b.ID && a.Level == b.Level && a.ProjectID == b.ProjectID && a.Subject == b.Subject &&
		a.Resource == b.Resource && a.DefinitionHash == b.DefinitionHash && a.GrantedBy == b.GrantedBy &&
		a.CreatedAt.Equal(b.CreatedAt) && ((a.ExpiresAt == nil) == (b.ExpiresAt == nil)) &&
		(a.ExpiresAt == nil || a.ExpiresAt.Equal(*b.ExpiresAt))
}

// appendAudit stores ev with the next Seq. The caller holds s.mu and has already checked the
// write it records, so the Grant and its event change together.
func (s *GrantStore) appendAudit(ev domain.AuditEvent) {
	ev.Seq = int64(len(s.log)) + 1
	s.log = append(s.log, ev)
}

// Audit implements ports.AuditLog.
func (s *GrantStore) Audit(_ context.Context, after int64, limit int) ([]domain.AuditEvent, error) {
	if limit <= 0 {
		limit = 1000
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []domain.AuditEvent{}
	for _, ev := range s.log {
		if ev.Seq > after && len(out) < limit {
			out = append(out, ev)
		}
	}
	return out, nil
}
