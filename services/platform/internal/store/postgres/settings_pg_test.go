//go:build pg

package postgres

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
	"github.com/rluders/amaru/services/platform/internal/ports/porttest"
)

func TestSettingsStore(t *testing.T) {
	var db *sql.DB
	porttest.SettingsStore(t, func(t *testing.T) (ports.SettingsStore, ports.ProjectStore) {
		db = migratedDB(t)
		s := NewProjectStore(db)
		return s, s
	}, func(ports.SettingsStore) ports.SettingsStore { return NewProjectStore(db) })
}
func TestSettingsRollback(t *testing.T) {
	db := migratedDB(t)
	s := NewProjectStore(db)
	ctx := context.Background()
	key := domain.ProjectKey("ssh://github.com/acme/a#0123456789abcdef0123456789abcdef01234567")
	if err := s.Create(ctx, domain.Project{Key: key, Name: "a", CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	value := true
	if _, err := s.UpdateSettings(ctx, key, domain.UpdateSettings{ImportClaudeSkills: &value}); err != nil {
		t.Fatal(err)
	}
	// Force a failure after the row is modified to prove statement rollback.
	if _, err := db.Exec(`CREATE FUNCTION refuse_settings() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'test refusal'; END $$;
 CREATE TRIGGER refuse_settings AFTER UPDATE ON project_settings FOR EACH ROW EXECUTE FUNCTION refuse_settings()`); err != nil {
		t.Fatal(err)
	}
	value = false
	if _, err := s.UpdateSettings(ctx, key, domain.UpdateSettings{ImportClaudeSkills: &value}); err == nil {
		t.Fatal("write succeeded")
	}
	got, err := NewProjectStore(db).Settings(ctx, key)
	if err != nil || !got.ImportClaudeSkills {
		t.Fatalf("rollback: %+v %v", got, err)
	}
}
