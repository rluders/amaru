package ports

import (
	"context"

	"github.com/rluders/amaru/services/platform/internal/domain"
)

// SecretIndex checks only existence, never decrypts or returns a secret value.
// Trusted composition must bind it to the Instance's authoritative secret metadata.
// Production encryption, Policy/Approval and assignment delivery remain separate gates.
type SecretIndex interface {
	// Has checks exactly this identity: no user, Organization or other Project fallback.
	// Missing or inaccessible identities return false. Errors must not be logged by callers.
	Has(context.Context, domain.SecretIdentity) (bool, error)
}
