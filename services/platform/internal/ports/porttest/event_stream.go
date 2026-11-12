package porttest

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
)

// EventStream checks the same transactional replay contract for fake and PostgreSQL stores.
func EventStream(t *testing.T, factory func(*testing.T) (ports.ExecutionStore, ports.EventStream)) {
	ctx := context.Background()
	t.Run("ordered resumable isolated and atomic", func(t *testing.T) {
		s, stream := factory(t)
		create := func(id, project string) domain.Execution {
			e, steps := newExec(id)
			e.ProjectID = project
			got, _, err := s.Create(ctx, e, steps)
			if err != nil {
				t.Fatal(err)
			}
			return got
		}
		a, b, other := create("a", "p"), create("b", "p"), create("other", "q")
		appendEvents := func(e domain.Execution, kinds ...string) {
			c := ports.ExecutionChange{Execution: &e}
			for _, k := range kinds {
				// Deliberately reverse occurrence times: the Platform commit orders Events.
				c.Events = append(c.Events, domain.ExecutionEvent{Kind: k, At: t0.Add(-time.Duration(len(c.Events)) * time.Hour)})
			}
			if _, err := s.Commit(ctx, c); err != nil {
				t.Fatal(err)
			}
		}
		appendEvents(a, "step.started", "step.succeeded")
		appendEvents(other, "step.started")
		appendEvents(b, "step.started")
		a, _, _ = s.Get(ctx, a.ID)
		appendEvents(a, "execution.waiting")
		q := domain.EventQuery{ProjectID: "p", Limit: 2}
		read := func(query domain.EventQuery, now time.Time, window time.Duration) domain.EventPage {
			p, err := stream.ReadEvents(ctx, query, now, window)
			if err != nil {
				t.Fatal(err)
			}
			return p
		}
		now := time.Now().UTC()
		first := read(q, now, domain.CompactReplayWindow)
		if first.Resync != nil || first.Cursor != 2 || len(first.Events) != 2 || first.Events[0].Event.Seq != 1 || first.Events[1].Event.Seq != 2 {
			t.Fatalf("first: %+v", first)
		}
		if repeated := read(q, now, domain.CompactReplayWindow); !reflect.DeepEqual(first, repeated) {
			t.Fatalf("redelivery changed: %+v", repeated)
		}
		q.After = first.Cursor
		second := read(q, now, domain.CompactReplayWindow)
		if second.Resync != nil || second.Cursor != 4 || len(second.Events) != 2 || second.Events[0].Event.ExecutionID != "b" || second.Events[1].Event.Seq != 3 {
			t.Fatalf("second: %+v", second)
		}
		for i, e := range append(first.Events, second.Events...) {
			if e.Seq != int64(i+1) || e.ProjectID != "p" {
				t.Fatalf("order/isolation: %+v", e)
			}
		}
		q.After = 4
		if p := read(q, now, domain.CompactReplayWindow); len(p.Events) != 0 || p.Cursor != 4 || p.Resync != nil {
			t.Fatalf("current: %+v", p)
		}
		q.After, q.ExecutionID, q.Limit = 0, "a", 10
		if p := read(q, now, domain.CompactReplayWindow); len(p.Events) != 3 || p.Cursor != 4 {
			t.Fatalf("Execution filter: %+v", p)
		}
		q.ExecutionID = "other"
		if p := read(q, now, domain.CompactReplayWindow); len(p.Events) != 0 || p.Cursor != 4 {
			t.Fatalf("foreign Execution: %+v", p)
		}
		q.ExecutionID = ""
		// Old Worker event times above must not expire freshly committed Events.
		if p := read(q, now.Add(8*24*time.Hour), domain.CompactReplayWindow); p.Resync == nil || p.Resync.Reason != "cursor_too_old" || p.Resync.ResumeFrom != 4 || len(p.Events) != 0 {
			t.Fatalf("expired: %+v", p)
		}
		if p := read(q, now.Add(8*24*time.Hour), domain.ServerReplayWindow); p.Resync != nil || len(p.Events) != 4 {
			t.Fatalf("server window: %+v", p)
		}
		if p := read(q, now.Add(31*24*time.Hour), domain.ServerReplayWindow); p.Resync == nil || p.Resync.Reason != "cursor_too_old" {
			t.Fatalf("server expired: %+v", p)
		}
		q.After = 5
		if p := read(q, now, domain.CompactReplayWindow); p.Resync == nil || p.Resync.Reason != "cursor_ahead" {
			t.Fatalf("ahead: %+v", p)
		}
		q.ProjectID, q.After = "empty", 0
		if p := read(q, now, domain.CompactReplayWindow); len(p.Events) != 0 || p.Cursor != 0 || p.Resync != nil {
			t.Fatalf("empty: %+v", p)
		}
		q = domain.EventQuery{ProjectID: "p", Limit: 10}
		before := read(q, now, domain.CompactReplayWindow)
		_, err := s.Commit(ctx, ports.ExecutionChange{Execution: &b, Events: []domain.ExecutionEvent{{Kind: "step.failed", At: now}}})
		if !errors.Is(err, domain.ErrConflict) {
			t.Fatalf("stale commit: %v", err)
		}
		if after := read(q, now, domain.CompactReplayWindow); !reflect.DeepEqual(before, after) {
			t.Fatalf("failed commit leaked: %+v", after)
		}
		first.Events[0].Event.Kind = "mutated"
		if p := read(q, now, domain.CompactReplayWindow); p.Events[0].Event.Kind == "mutated" {
			t.Fatal("shared mutable record")
		}
	})
	t.Run("concurrent writers have gap-free Project order", func(t *testing.T) {
		s, stream := factory(t)
		const n = 12
		for i := range n {
			e, steps := newExec(string(rune('a' + i)))
			e.ProjectID = "p"
			if _, _, err := s.Create(ctx, e, steps); err != nil {
				t.Fatal(err)
			}
		}
		var wg sync.WaitGroup
		for i := range n {
			wg.Go(func() {
				id := string(rune('a' + i))
				e, _, err := s.Get(ctx, id)
				if err != nil {
					t.Error(err)
					return
				}
				_, err = s.Commit(ctx, ports.ExecutionChange{Execution: &e, Events: []domain.ExecutionEvent{{Kind: "step.started", At: t0}}})
				if err != nil {
					t.Error(err)
				}
			})
		}
		wg.Wait()
		p, err := stream.ReadEvents(ctx, domain.EventQuery{ProjectID: "p", Limit: 100}, time.Now(), domain.CompactReplayWindow)
		if err != nil || len(p.Events) != n || p.Resync != nil {
			t.Fatalf("concurrent: %+v %v", p, err)
		}
		for i, e := range p.Events {
			if e.Seq != int64(i+1) || e.Event.Seq != 1 {
				t.Fatalf("sequence: %+v", e)
			}
		}
	})
	t.Run("invalid queries", func(t *testing.T) {
		_, stream := factory(t)
		for _, q := range []domain.EventQuery{{Limit: 1}, {ProjectID: "p", After: -1, Limit: 1}, {ProjectID: "p"}, {ProjectID: "p", Limit: 1001}} {
			if _, err := stream.ReadEvents(ctx, q, time.Now(), domain.CompactReplayWindow); !errors.Is(err, domain.ErrInvalid) {
				t.Fatalf("query %+v: %v", q, err)
			}
		}
		q := domain.EventQuery{ProjectID: "p", Limit: 1}
		if _, err := stream.ReadEvents(ctx, q, time.Time{}, domain.CompactReplayWindow); !errors.Is(err, domain.ErrInvalid) {
			t.Fatal(err)
		}
		if _, err := stream.ReadEvents(ctx, q, time.Now(), 0); !errors.Is(err, domain.ErrInvalid) {
			t.Fatal(err)
		}
	})
}
