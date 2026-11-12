//go:build pg

package migrations

import (
	"context"
	"io/fs"
	"strings"
	"testing"
	"time"
)

func TestProjectEventStreamUpgrade(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	old := map[string]string{}
	entries, err := fs.ReadDir(Embedded(), ".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".sql") || entry.Name() >= "20261112160000" {
			continue
		}
		b, err := fs.ReadFile(Embedded(), entry.Name())
		if err != nil {
			t.Fatal(err)
		}
		old[entry.Name()] = string(b)
	}
	if _, err := runnerFor(t, db, old).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO executions (id, version, state, created_at, next_seq, project_id) VALUES ('old', 1, 'queued', now(), 1, 'p');
		INSERT INTO execution_events (execution_id, seq, kind, at) VALUES ('old', 1, 'step.started', now())`); err != nil {
		t.Fatal(err)
	}
	dir, err := OpenDir(Embedded())
	if err != nil {
		t.Fatal(err)
	}
	r := NewRunner(db, dir, 5*time.Second)
	if _, err := r.Apply(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Apply(ctx); err != nil {
		t.Fatal(err)
	}
	var last, unavailable, count int64
	if err := db.QueryRow(`SELECT last_seq, unavailable_through FROM project_event_counters WHERE project_id = 'p'`).Scan(&last, &unavailable); err != nil || last != 1 || unavailable != 1 {
		t.Fatalf("baseline: %d %d %v", last, unavailable, err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM project_events`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("invented history: %d %v", count, err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM execution_events`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("lost original history: %d %v", count, err)
	}
}
