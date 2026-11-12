//go:build pg

package migrations

import (
	"context"
	"io/fs"
	"strings"
	"testing"
	"time"
)

func TestProjectSettingsUpgrade(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	old := map[string]string{}
	entries, err := fs.ReadDir(Embedded(), ".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".sql") || entry.Name() >= "20261112230000" {
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
	if _, err := db.Exec(`INSERT INTO projects (key,name,created_at) VALUES ('existing','preserved',now())`); err != nil {
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
	if err := db.QueryRow(`SELECT p.name, COALESCE(s.import_claude_skills,false) FROM projects p LEFT JOIN project_settings s ON p.key=s.project_key WHERE p.key='existing'`).Scan(&name, &enabled); err != nil || name != "preserved" || enabled {
		t.Fatalf("upgrade: %s %v %v", name, enabled, err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM project_settings`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("invented preference: %d %v", count, err)
	}
}
