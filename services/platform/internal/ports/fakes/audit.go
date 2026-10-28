package fakes

import (
	"context"
	"sync"

	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
)

// AuditLog is the in-memory audit log. Each store holds one; LinkAudit makes two stores share
// one, as the PostgreSQL stores do through one table.
type AuditLog struct {
	mu     sync.Mutex
	events []domain.AuditEvent
}

var _ ports.AuditLog = (*AuditLog)(nil)

// append stores ev with the next Seq. The calling store has already checked the write it records.
func (l *AuditLog) append(ev domain.AuditEvent) {
	l.mu.Lock()
	defer l.mu.Unlock()
	ev.Seq = int64(len(l.events)) + 1
	l.events = append(l.events, ev)
}

// Audit implements ports.AuditLog.
func (l *AuditLog) Audit(_ context.Context, after int64, limit int) ([]domain.AuditEvent, error) {
	if limit <= 0 {
		limit = 1000
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	out := []domain.AuditEvent{}
	for _, ev := range l.events {
		if ev.Seq > after && len(out) < limit {
			out = append(out, ev)
		}
	}
	return out, nil
}

// LinkAudit makes the Grant store write to the Execution store's audit log, so one log holds the
// Grant and Approval events and Seq runs across both.
func LinkAudit(e *ExecutionStore, g *GrantStore) { g.log = e.log }
