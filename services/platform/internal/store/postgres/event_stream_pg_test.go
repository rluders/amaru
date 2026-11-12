//go:build pg

package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
	"github.com/rluders/amaru/services/platform/internal/ports/porttest"
)

func TestEventStream(t *testing.T) {
	porttest.EventStream(t, func(t *testing.T) (ports.ExecutionStore, ports.EventStream) {
		s := NewExecutionStore(migratedDB(t))
		return s, s
	})
}

func TestEventStreamVisibilityAndRestart(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db := migratedDB(t)
	s := NewExecutionStore(db)
	e, _, err := s.Create(ctx, domain.Execution{ID: "e", ProjectID: "p", State: domain.ExecQueued, CreatedAt: time.Now()}, nil)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE executions SET next_seq = 1 WHERE id = 'e'; INSERT INTO execution_events (execution_id, seq, kind, at) VALUES ('e', 1, 'step.started', now())`); err != nil {
		t.Fatal(err)
	}
	if err := appendProjectEvents(ctx, tx, e.ID, []domain.ExecutionEvent{{Seq: 1}}); err != nil {
		t.Fatal(err)
	}
	q := domain.EventQuery{ProjectID: "p", Limit: 10}
	if page, err := s.ReadEvents(ctx, q, time.Now(), domain.CompactReplayWindow); err != nil || len(page.Events) != 0 || page.Cursor != 0 {
		t.Fatalf("uncommitted: %+v %v", page, err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	restarted := NewExecutionStore(db)
	if page, err := restarted.ReadEvents(ctx, q, time.Now(), domain.CompactReplayWindow); err != nil || len(page.Events) != 1 || page.Cursor != 1 {
		t.Fatalf("restart: %+v %v", page, err)
	}
}

func TestEventStreamWriteFailureRollsBackStateAndCursor(t *testing.T) {
	ctx := context.Background()
	db := migratedDB(t)
	s := NewExecutionStore(db)
	e, _, err := s.Create(ctx, domain.Execution{ID: "e", ProjectID: "p", State: domain.ExecQueued, CreatedAt: time.Now()}, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Fail after the Project counter was advanced, exercising the real Commit rollback.
	if _, err := db.Exec(`ALTER TABLE project_events ADD CONSTRAINT injected_failure CHECK (seq <> 1)`); err != nil {
		t.Fatal(err)
	}
	e.State = domain.ExecRunning
	change := ports.ExecutionChange{Execution: &e, Events: []domain.ExecutionEvent{{Kind: "step.started", At: time.Now()}}}
	if _, err := s.Commit(ctx, change); err == nil {
		t.Fatal("expected injected journal failure")
	}
	got, _, err := s.Get(ctx, e.ID)
	if err != nil || got.State != domain.ExecQueued || got.Version != e.Version {
		t.Fatalf("state leaked: %+v %v", got, err)
	}
	if events, err := s.Events(ctx, e.ID); err != nil || len(events) != 0 {
		t.Fatalf("Execution log leaked: %+v %v", events, err)
	}
	q := domain.EventQuery{ProjectID: "p", Limit: 10}
	if page, err := s.ReadEvents(ctx, q, time.Now(), domain.CompactReplayWindow); err != nil || len(page.Events) != 0 || page.Cursor != 0 {
		t.Fatalf("Project cursor leaked: %+v %v", page, err)
	}
	if _, err := db.Exec(`ALTER TABLE project_events DROP CONSTRAINT injected_failure`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Commit(ctx, change); err != nil {
		t.Fatal(err)
	}
	if page, err := s.ReadEvents(ctx, q, time.Now(), domain.CompactReplayWindow); err != nil || len(page.Events) != 1 || page.Events[0].Seq != 1 {
		t.Fatalf("retry left a hole: %+v %v", page, err)
	}
}

func TestEventStreamRetainedSuffixAndMissingHistory(t *testing.T) {
	ctx := context.Background()
	db := migratedDB(t)
	s := NewExecutionStore(db)
	now := time.Now().UTC()
	e, _, err := s.Create(ctx, domain.Execution{ID: "e", ProjectID: "p", State: domain.ExecQueued, CreatedAt: now}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Commit(ctx, ports.ExecutionChange{Execution: &e, Events: []domain.ExecutionEvent{
		{Kind: "step.started", At: now}, {Kind: "step.succeeded", At: now}, {Kind: "execution.waiting", At: now},
	}}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE project_events SET recorded_at = $1 WHERE project_id = 'p' AND seq = 1`, now.Add(-8*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	q := domain.EventQuery{ProjectID: "p", Limit: 10}
	page, err := s.ReadEvents(ctx, q, now, domain.CompactReplayWindow)
	if err != nil || page.Resync == nil || page.Resync.Reason != "cursor_too_old" || page.Resync.ResumeFrom != 3 {
		t.Fatalf("expired prefix: %+v %v", page, err)
	}
	q.After = 1
	page, err = s.ReadEvents(ctx, q, now, domain.CompactReplayWindow)
	if err != nil || page.Resync != nil || len(page.Events) != 2 || page.Events[0].Seq != 2 || page.Cursor != 3 {
		t.Fatalf("retained suffix: %+v %v", page, err)
	}
	if _, err := db.Exec(`INSERT INTO project_event_counters (project_id, last_seq, unavailable_through) VALUES ('legacy', 1, 1)`); err != nil {
		t.Fatal(err)
	}
	q.ProjectID, q.After = "legacy", 0
	page, err = s.ReadEvents(ctx, q, now, domain.CompactReplayWindow)
	if err != nil || page.Resync == nil || page.Resync.Type != "resync_required" || page.Resync.Reason != "history_unavailable" || page.Resync.ResumeFrom != 1 {
		t.Fatalf("missing history: %+v %v", page, err)
	}
	q.After = page.Resync.ResumeFrom
	page, err = s.ReadEvents(ctx, q, now, domain.CompactReplayWindow)
	if err != nil || page.Resync != nil || len(page.Events) != 0 || page.Cursor != 1 {
		t.Fatalf("resume after reload: %+v %v", page, err)
	}
}
