package coordinator

import (
	"context"
	"strings"

	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
)

// SecretResolver resolves metadata only. Its Organization and index are trusted Instance
// dependencies, never request parameters. It exposes no plaintext or delivery operation.
type SecretResolver struct {
	access       *ProjectAccess
	organization string
	index        ports.SecretIndex
}

func NewSecretResolver(access *ProjectAccess, organization string, index ports.SecretIndex) (*SecretResolver, error) {
	if access == nil || strings.TrimSpace(organization) == "" || index == nil {
		return nil, domain.ErrInvalid
	}
	return &SecretResolver{access, organization, index}, nil
}

// Resolve requires current Project access even for an explicit Organization reference.
// A successful identity still needs Policy/first-use Approval before any value delivery.
func (r *SecretResolver) Resolve(ctx context.Context, project domain.ProjectKey, ref domain.SecretReference) (domain.SecretIdentity, error) {
	if _, err := r.access.authorize(ctx, project); err != nil {
		return domain.SecretIdentity{}, err
	}
	id, err := ref.Bind(r.organization, project)
	if err != nil {
		return domain.SecretIdentity{}, err
	}
	found, lookupErr := r.index.Has(ctx, id)
	if _, err := r.access.authorize(ctx, project); err != nil {
		return domain.SecretIdentity{}, err
	}
	if lookupErr != nil {
		return domain.SecretIdentity{}, domain.ErrSecretUnavailable
	}
	if !found {
		return domain.SecretIdentity{}, domain.ErrNotFound
	}
	return id, nil
}
