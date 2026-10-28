package ports

import (
	"context"

	"github.com/rluders/amaru/services/platform/internal/domain"
)

// TrustStore keeps Trust records (FEAT-PRJ-003). There is at most one record per Project, kind
// and ref; it is never deleted, so a revoked record and the hash it held stay readable. Every
// Save is audited in the same transaction.
type TrustStore interface {
	AuditLog
	// Save stores t, replacing the record of the same Project, kind and ref. A Trust that is not
	// revoked is a grant of trust (a first one, a renewal at a new hash, or a trust after a
	// revoke); a revoked one must follow a record in force, or Save returns domain.ErrInvalid.
	// An invalid Trust returns domain.ErrInvalid.
	Save(ctx context.Context, t domain.Trust) error
	// Get returns the record, or domain.ErrNotFound.
	Get(ctx context.Context, key domain.ProjectKey, kind domain.TrustKind, ref string) (domain.Trust, error)
	// List returns every record of the Project, ordered by kind and ref.
	List(ctx context.Context, key domain.ProjectKey) ([]domain.Trust, error)
}
