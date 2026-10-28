//go:build pg

package postgres

// PostgreSQL-backed tests. They run only with `-tags pg` and AMARU_TEST_PG set to the DSN of a
// server whose role may create databases (see internal/store/migrations/runner_pg_test.go). Each
// test uses its own database, migrated with the embedded migrations.

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/rluders/amaru/services/platform/internal/coordinator/coordtest"
	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
	"github.com/rluders/amaru/services/platform/internal/ports/porttest"
	"github.com/rluders/amaru/services/platform/internal/store/migrations"
	"github.com/rluders/amaru/services/platform/internal/worker/inproc"
)

var dbSeq atomic.Int64

// migratedDB returns a connection to a new database at the current schema; it is dropped on cleanup.
func migratedDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("AMARU_TEST_PG")
	if dsn == "" {
		t.Skip("AMARU_TEST_PG is not set")
	}
	admin, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { admin.Close() })
	name := fmt.Sprintf("amaru_pg_%d_%d", os.Getpid(), dbSeq.Add(1))
	if _, err := admin.Exec("CREATE DATABASE " + name); err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + name
	db, err := sql.Open("pgx", u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		db.Close()
		_, _ = admin.Exec("DROP DATABASE IF EXISTS " + name + " WITH (FORCE)")
	})
	dir, err := migrations.OpenDir(migrations.Embedded())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := migrations.NewRunner(db, dir, 10*time.Second).Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestProjectStoreConformance(t *testing.T) {
	porttest.ProjectStore(t, func(t *testing.T) ports.ProjectStore { return NewProjectStore(migratedDB(t)) })
}

func TestExecutionStoreConformance(t *testing.T) {
	// reopen builds a second store over the same database: a restarted Platform sees only what was committed.
	var db *sql.DB
	porttest.ExecutionStore(t,
		func(t *testing.T) ports.ExecutionStore { db = migratedDB(t); return NewExecutionStore(db) },
		func(_ *testing.T, _ ports.ExecutionStore) ports.ExecutionStore { return NewExecutionStore(db) })
}

// The crash-injection matrix of the coordinator against the real store: a restart is a new store
// over the same database, so only committed rows survive.
func TestCrashInjectionWithPostgres(t *testing.T) {
	var db *sql.DB
	coordtest.CrashInjection(t,
		func(t *testing.T) ports.ExecutionStore { db = migratedDB(t); return NewExecutionStore(db) },
		func(_ *testing.T, _ ports.ExecutionStore) ports.ExecutionStore { return NewExecutionStore(db) })
}

// The coordinator's Worker-interface contract against the real store, over both kinds of link.
func TestAssignmentWithPostgres(t *testing.T) {
	newStore := func(t *testing.T) ports.ExecutionStore { return NewExecutionStore(migratedDB(t)) }
	coordtest.AssignmentSuite(t, newStore, func(*testing.T) (ports.PlatformLink, ports.WorkerLink) { return inproc.Pair() })
}

// A repeated command returns the original result, also after a restart and under a race, against
// the real store.
func TestCommandsWithPostgres(t *testing.T) {
	var db *sql.DB
	coordtest.CommandSuite(t,
		func(t *testing.T) ports.ExecutionStore { db = migratedDB(t); return NewExecutionStore(db) },
		func(_ *testing.T, _ ports.ExecutionStore) ports.ExecutionStore { return NewExecutionStore(db) })
}

// The pinned definition survives a restart and cannot be updated, also by a direct write.
func TestDefinitionWithPostgres(t *testing.T) {
	var db *sql.DB
	coordtest.DefinitionSuite(t,
		func(t *testing.T) ports.ExecutionStore { db = migratedDB(t); return NewExecutionStore(db) },
		func(_ *testing.T, _ ports.ExecutionStore) ports.ExecutionStore { return NewExecutionStore(db) })
	t.Run("the database refuses to change a pin", func(t *testing.T) {
		db = migratedDB(t)
		s := NewExecutionStore(db)
		pin, err := domain.Pin(domain.Workflow{ID: "ship-it", Name: "Ship it", Status: domain.WorkflowActive,
			Steps: []domain.WorkflowStep{{ID: "build", Name: "Build", Type: domain.StepTypeTool, Capability: "code.build"}}}, "")
		if err != nil {
			t.Fatal(err)
		}
		e := domain.Execution{ID: "e1", State: domain.ExecQueued, CreatedAt: time.Now(), Definition: &pin}
		if _, _, err := s.Create(context.Background(), e, nil); err != nil {
			t.Fatal(err)
		}
		_, err = db.Exec(`UPDATE executions SET definition_hash = 'sha256:x' WHERE id = 'e1'`)
		if err == nil || !strings.Contains(err.Error(), "pinned") {
			t.Fatalf("got %v, want the pin refused", err)
		}
	})
}

// The concurrency limits against the real store, which counts the Steps that hold a slot.
func TestLimitsWithPostgres(t *testing.T) {
	newStore := func(t *testing.T) ports.ExecutionStore { return NewExecutionStore(migratedDB(t)) }
	coordtest.LimitsSuite(t, newStore, func(*testing.T) (ports.PlatformLink, ports.WorkerLink) { return inproc.Pair() })
}

// Per-Step timeouts against the real store: the override is stored and a timed_out Step stays so.
func TestTimeoutsWithPostgres(t *testing.T) {
	var db *sql.DB
	coordtest.TimeoutSuite(t,
		func(t *testing.T) ports.ExecutionStore { db = migratedDB(t); return NewExecutionStore(db) },
		func(*testing.T) (ports.PlatformLink, ports.WorkerLink) { return inproc.Pair() },
		func(_ *testing.T, _ ports.ExecutionStore) ports.ExecutionStore { return NewExecutionStore(db) })
}
