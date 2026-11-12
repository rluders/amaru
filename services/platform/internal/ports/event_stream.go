package ports

import (
	"context"
	"time"

	"github.com/rluders/amaru/services/platform/internal/domain"
)

// EventStream reads one consistent committed snapshot. now is supplied by the Platform,
// never a Worker or client; window is chosen by trusted deployment composition.
// Events are ordered by Project cursor; a full page resumes at its last returned cursor.
// A short/empty page advances to the snapshot's high-water mark, even with a filter.
type EventStream interface {
	ReadEvents(ctx context.Context, q domain.EventQuery, now time.Time, window time.Duration) (domain.EventPage, error)
}
