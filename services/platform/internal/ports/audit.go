package ports

import (
	"context"

	"github.com/rluders/amaru/services/platform/internal/domain"
)

// AuditLog reads the Platform's append-only audit log. Events are written by the store that makes
// the change they record, in the same transaction; there is no way to append one on its own.
type AuditLog interface {
	// Audit returns up to limit events with Seq greater than after, in Seq order, so a reader
	// replays the log from the start (after 0) or resumes from the last Seq it saw. A limit of
	// zero or less means 1000.
	Audit(ctx context.Context, after int64, limit int) ([]domain.AuditEvent, error)
}

// GrantAuditStore is a GrantStore whose every write is audited, and whose audit log can be read.
type GrantAuditStore interface {
	GrantStore
	AuditLog
}
