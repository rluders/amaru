//go:build pg

package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/rluders/amaru/services/platform/internal/coordinator/coordtest"
	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
	"strings"
	"testing"
	"time"
)

func TestUsageCapture(t *testing.T) {
	coordtest.UsageSuite(t, func(t *testing.T) ports.ExecutionStore {
		db := migratedDB(t)
		return usageCheckedStore{NewExecutionStore(db), db}
	})
}

func TestUsagePublicationAtomicity(t *testing.T) {
	db := migratedDB(t)
	s := NewExecutionStore(db)
	ctx := context.Background()
	now := time.Now().UTC()
	e := domain.Execution{ID: "e", ProjectID: "p", State: domain.ExecRunning, CreatedAt: now}
	st := domain.StepExecution{ID: "s", ExecutionID: "e", State: domain.StepRunning, AttemptID: "a", FenceToken: 1, CurrentLease: &domain.LeaseInfo{Owner: "w", ExpiresAt: now.Add(time.Hour)}}
	_, steps, err := s.Create(ctx, e, []domain.StepExecution{st})
	if err != nil {
		t.Fatal(err)
	}
	st = steps[0]
	if err := st.Finish(now, domain.StepSucceeded, nil); err != nil {
		t.Fatal(err)
	}
	capture := domain.ResultCapture{ExecutionID: "e", ProjectID: "p", StepID: "s", AttemptID: "a", FenceToken: 1, CapturedAt: now, Fingerprint: strings.Repeat("a", 64), Reports: []domain.UsageReport{{Provider: "fake"}}}
	change := ports.ExecutionChange{Steps: []domain.StepExecution{st}, Capture: &capture, Events: []domain.ExecutionEvent{{Kind: domain.EventStepSucceeded, StepID: "s", At: now}}}
	if _, err := db.Exec(`ALTER TABLE usage_publication_intents ADD CONSTRAINT refuse_intent CHECK (ordinal < 0)`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Commit(ctx, change); err == nil {
		t.Fatal("expected database failure")
	}
	_, stored, err := s.Get(ctx, "e")
	if err != nil || stored[0].State != domain.StepRunning || stored[0].Version != st.Version {
		t.Fatal("partial step write")
	}
	for _, table := range []string{"result_captures", "usage_publication_intents", "execution_events"} {
		var n int
		if err := db.QueryRow(`SELECT count(*) FROM ` + table).Scan(&n); err != nil || n != 0 {
			t.Fatalf("%s rows=%d err=%v", table, n, err)
		}
	}
	if _, err := db.Exec(`ALTER TABLE usage_publication_intents DROP CONSTRAINT refuse_intent`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Commit(ctx, change); err != nil {
		t.Fatal(err)
	}
	// Fresh adapter sees the exact durable receipt and a single pending intent.
	c, err := NewExecutionStore(db).ResultCapture(ctx, "a")
	if err != nil || c.UsageID(0) != capture.UsageID(0) {
		t.Fatal("receipt changed")
	}
	var id string
	if err := db.QueryRow(`SELECT usage_id FROM usage_publication_intents`).Scan(&id); err != nil || id != capture.UsageID(0) {
		t.Fatal("publication intent missing")
	}
	if _, err := s.Commit(ctx, change); err == nil {
		t.Fatal("replayed store write accepted")
	}
	for _, statement := range []string{`UPDATE result_captures SET receipt='{}'`, `DELETE FROM result_captures`} {
		if _, err := db.Exec(statement); err == nil {
			t.Fatal("receipt was mutable")
		}
	}
	var n int
	if err := db.QueryRow(`SELECT count(*) FROM usage_publication_intents`).Scan(&n); err != nil || n != 1 {
		t.Fatal("intent duplicated")
	}
}

// Every accepted/duplicate ack in the shared suite reads through this assertion.
type usageCheckedStore struct {
	*ExecutionStore
	db *sql.DB
}

func (s usageCheckedStore) ResultCapture(ctx context.Context, attempt string) (domain.ResultCapture, error) {
	c, err := s.ExecutionStore.ResultCapture(ctx, attempt)
	if err != nil {
		return c, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT usage_id, ordinal FROM usage_publication_intents WHERE attempt_id=$1 ORDER BY ordinal`, attempt)
	if err != nil {
		return c, err
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var id string
		var ordinal int
		if err := rows.Scan(&id, &ordinal); err != nil {
			return c, err
		}
		if ordinal != n || id != c.UsageID(n) {
			return c, fmt.Errorf("publication intent identity/order mismatch at ordinal %d", n)
		}
		n++
	}
	if err := rows.Err(); err != nil {
		return c, err
	}
	if n != len(c.Reports) {
		return c, fmt.Errorf("ack before publication intent: got %d want %d", n, len(c.Reports))
	}
	return c, nil
}
