package ports

import (
	"context"
	"github.com/rluders/amaru/services/platform/internal/domain"
)

// SettingsStore keeps preferences for attached Projects (ADR-0064).
// Missing settings default false; missing Projects return bare ErrNotFound.
// Updates commit atomically before returning; concurrent explicit values serialize.
// Invalid patches and failed writes leave the previous value unchanged.
type SettingsStore interface {
	Settings(context.Context, domain.ProjectKey) (domain.ProjectSettings, error)
	UpdateSettings(context.Context, domain.ProjectKey, domain.UpdateSettings) (domain.ProjectSettings, error)
}
