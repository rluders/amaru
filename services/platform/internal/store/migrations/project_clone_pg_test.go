//go:build pg

package migrations

import (
	"context"
	"io/fs"
	"strings"
	"testing"
	"time"
)

func TestProjectCloneUpgrade(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	old := map[string]string{}
	entries, err := fs.ReadDir(Embedded(), ".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".sql") || entry.Name() >= "20261112233000" {
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
	if _, err := db.Exec(`INSERT INTO projects (key,name,created_at) VALUES ('existing','preserved',now()); INSERT INTO project_settings (project_key,import_claude_skills) VALUES ('existing',true)`); err != nil {
		t.Fatal(err)
	}
	dir, err := OpenDir(Embedded())
	if err != nil {
		t.Fatal(err)
	}
	runner := NewRunner(db, dir, 5*time.Second)
	for range 2 {
		if _, err := runner.Apply(ctx); err != nil {
			t.Fatal(err)
		}
	}
	var name string
	var enabled bool
	var count int
	if err := db.QueryRow(`SELECT p.name,s.import_claude_skills FROM projects p JOIN project_settings s ON p.key=s.project_key WHERE p.key='existing'`).Scan(&name, &enabled); err != nil || name != "preserved" || !enabled {
		t.Fatalf("upgrade: %s %v %v", name, enabled, err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM project_clones`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("invented root: %d %v", count, err)
	}
	// The runner is forward-only. Exercise the new file's Down SQL in a rolled-back transaction.
	migration, err := fs.ReadFile(Embedded(), "20261112233000_project_clones.sql")
	if err != nil {
		t.Fatal(err)
	}
	_, down, ok := strings.Cut(string(migration), "-- +goose Down")
	if !ok {
		t.Fatal("missing inverse")
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, down); err != nil {
		t.Fatal(err)
	}
	var exists bool
	if err := tx.QueryRowContext(ctx, `SELECT to_regclass('project_clones') IS NOT NULL`).Scan(&exists); err != nil || exists {
		t.Fatal("inverse failed", err)
	}
	if err := tx.QueryRowContext(ctx, `SELECT import_claude_skills FROM project_settings WHERE project_key='existing'`).Scan(&enabled); err != nil || !enabled {
		t.Fatal("inverse changed settings", err)
	}
}
