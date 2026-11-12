package fakes

import (
	"context"
	"fmt"
	"time"

	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
)

var _ ports.EventStream = (*ExecutionStore)(nil)

type recordedEvent struct {
	event domain.StreamEvent
	at    time.Time
}

func (s *ExecutionStore) ReadEvents(_ context.Context, q domain.EventQuery, now time.Time, window time.Duration) (domain.EventPage, error) {
	if err := q.Validate(); err != nil {
		return domain.EventPage{}, err
	}
	if now.IsZero() || window <= 0 {
		return domain.EventPage{}, fmt.Errorf("%w: event replay needs a Platform clock and positive window", domain.ErrInvalid)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	events := s.stream[q.ProjectID]
	var expired int64
	for _, r := range events {
		if r.at.Before(now.Add(-window)) {
			expired = r.event.Seq
		}
	}
	last := int64(len(events))
	page := domain.EventPage{Events: []domain.StreamEvent{}, Cursor: last, Resync: domain.ReplayResync(q.After, last, 0, expired)}
	if page.Resync != nil {
		return page, nil
	}
	for _, r := range events {
		if r.event.Seq > q.After && (q.ExecutionID == "" || r.event.Event.ExecutionID == q.ExecutionID) {
			page.Events = append(page.Events, clone(r.event))
			if len(page.Events) == q.Limit {
				page.Cursor = r.event.Seq
				break
			}
		}
	}
	return page, nil
}
