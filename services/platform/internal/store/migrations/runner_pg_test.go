//go:build pg

package migrations

// PostgreSQL-backed tests. They run only with `-tags pg` and AMARU_TEST_PG set to the DSN of
// a server whose role may create databases, e.g.
//
//	AMARU_TEST_PG='postgres://user:pass@127.0.0.1:5432/postgres?sslmode=disable' go test -tags pg ./internal/store/migrations
//
// Each test migrates its own freshly created database.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

var dbSeq atomic.Int64

// testDB returns a connection to a new, empty database and drops it on cleanup.
func testDB(t *testing.T) *sql.DB {
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
	name := fmt.Sprintf("amaru_mig_%d_%d", os.Getpid(), dbSeq.Add(1))
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
	return db
}

func runnerFor(t *testing.T, db *sql.DB, files map[string]string) *Runner {
	t.Helper()
	d, err := OpenDir(fixture(t, files))
	if err != nil {
		t.Fatal(err)
	}
	return NewRunner(db, d, 5*time.Second)
}

func hasTable(t *testing.T, db *sql.DB, name string) bool {
	t.Helper()
	var ok bool
	if err := db.QueryRow(`SELECT to_regclass($1) IS NOT NULL`, name).Scan(&ok); err != nil {
		t.Fatal(err)
	}
	return ok
}

func revisionCount(t *testing.T, db *sql.DB) int {
	t.Helper()
	var n int
	if err := db.QueryRow(`SELECT count(*) FROM ` + versionsTable + ` WHERE version_id > 0`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

const (
	fileA = "-- +goose Up\nCREATE TABLE a (id integer PRIMARY KEY);\nINSERT INTO a VALUES (1);\n\n-- +goose Down\nDROP TABLE a;\n"
	fileB = "-- +goose Up\nCREATE TABLE b (id integer PRIMARY KEY);\n"
)

func TestApplyFreshDatabase(t *testing.T) {
	db := testDB(t)
	r := runnerFor(t, db, map[string]string{v1 + "_a.sql": fileA, v2 + "_b.sql": fileB})
	ctx := context.Background()

	st, err := r.Status(ctx)
	if err != nil || len(st.Pending) != 2 || st.Current != "" {
		t.Fatalf("fresh status = %+v, %v", st, err)
	}
	if hasTable(t, db, versionsTable) {
		t.Fatal("Status created the version table; it must not write")
	}

	st, err = r.Apply(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !st.UpToDate() || st.Current != v2 || st.Latest != v2 {
		t.Fatalf("after apply = %+v", st)
	}
	if !hasTable(t, db, "a") || !hasTable(t, db, "b") || revisionCount(t, db) != 2 {
		t.Fatal("tables or revision rows missing")
	}
}

func TestApplyIsIdempotent(t *testing.T) {
	db := testDB(t)
	r := runnerFor(t, db, map[string]string{v1 + "_a.sql": fileA})
	for i := 0; i < 3; i++ {
		if _, err := r.Apply(context.Background()); err != nil {
			t.Fatalf("run %d: %v", i, err)
		}
	}
	if revisionCount(t, db) != 1 {
		t.Fatal("a migration was recorded twice")
	}
}

// Forward migration on a fixture: data written under the old schema survives the upgrade.
func TestUpgradeKeepsData(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	if _, err := runnerFor(t, db, map[string]string{v1 + "_a.sql": fileA}).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO a VALUES (2)`); err != nil {
		t.Fatal(err)
	}
	upgrade := "-- +goose Up\nALTER TABLE a ADD COLUMN note text NOT NULL DEFAULT 'x';\nUPDATE a SET note = 'old';\n"
	st, err := runnerFor(t, db, map[string]string{v1 + "_a.sql": fileA, v2 + "_b.sql": upgrade}).Apply(ctx)
	if err != nil || st.Current != v2 {
		t.Fatalf("upgrade = %+v, %v", st, err)
	}
	var n int
	if err := db.QueryRow(`SELECT count(*) FROM a WHERE note = 'old'`).Scan(&n); err != nil || n != 2 {
		t.Fatalf("rows with the new column = %d, %v; want 2", n, err)
	}
}

func TestDowngradeIsRefusedAndChangesNothing(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	if _, err := runnerFor(t, db, map[string]string{v1 + "_a.sql": fileA, v2 + "_b.sql": fileB}).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	older := runnerFor(t, db, map[string]string{v1 + "_a.sql": fileA})
	for name, call := range map[string]func() error{
		"Status": func() error { _, err := older.Status(ctx); return err },
		"Apply":  func() error { _, err := older.Apply(ctx); return err },
	} {
		err := call()
		if !errors.Is(err, ErrSchemaNewer) || !strings.Contains(err.Error(), "upgrade Amaru") {
			t.Fatalf("%s: err = %v, want ErrSchemaNewer with a message", name, err)
		}
	}
	if !hasTable(t, db, "b") || revisionCount(t, db) != 2 {
		t.Fatal("the refused run changed the database")
	}
}

func TestFailingMigrationRollsBackAndCanBeRetried(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	bad := "-- +goose Up\nCREATE TABLE c (id integer);\nCREATE TABLE c (id integer);\n"
	_, err := runnerFor(t, db, map[string]string{v1 + "_a.sql": fileA, v2 + "_c.sql": bad}).Apply(ctx)
	if err == nil || !strings.Contains(err.Error(), v2+"_c.sql") {
		t.Fatalf("err = %v, want the failing file named", err)
	}
	if hasTable(t, db, "c") {
		t.Fatal("the first statement of the failed file stayed")
	}
	var n int
	if e := db.QueryRow(`SELECT count(*) FROM `+versionsTable+` WHERE version_id=$1`, v2).Scan(&n); e != nil || n != 0 {
		t.Fatalf("revision of the failed file = %d, %v; want none", n, e)
	}
	if revisionCount(t, db) != 1 {
		t.Fatal("the earlier migration should stay applied")
	}
	fixed := "-- +goose Up\nCREATE TABLE c (id integer);\n"
	if st, err := runnerFor(t, db, map[string]string{v1 + "_a.sql": fileA, v2 + "_c.sql": fixed}).Apply(ctx); err != nil || st.Current != v2 {
		t.Fatalf("retry = %+v, %v", st, err)
	}
}

func TestOlderPendingFileIsRefused(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	if _, err := runnerFor(t, db, map[string]string{v2 + "_b.sql": fileB}).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	late := runnerFor(t, db, map[string]string{v1 + "_a.sql": fileA, v2 + "_b.sql": fileB})
	if _, err := late.Apply(ctx); !errors.Is(err, ErrOutOfOrder) {
		t.Fatalf("err = %v, want ErrOutOfOrder", err)
	}
	if hasTable(t, db, "a") {
		t.Fatal("the out-of-order file ran")
	}
}

func TestConcurrentPlatformsMigrateOnce(t *testing.T) {
	db := testDB(t)
	slow := "-- +goose Up\nSELECT pg_sleep(0.3);\nCREATE TABLE a (id integer);\n"
	files := map[string]string{v1 + "_a.sql": slow}
	var wg sync.WaitGroup
	errs := make([]error, 4)
	for i := range errs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = runnerFor(t, db, files).Apply(context.Background())
		}()
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("runner %d: %v", i, err)
		}
	}
	if revisionCount(t, db) != 1 {
		t.Fatalf("revisions = %d, want 1", revisionCount(t, db))
	}
}

func TestLockTimeout(t *testing.T) {
	db := testDB(t)
	holder, err := db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Close()
	if _, err := holder.ExecContext(context.Background(), `SELECT pg_advisory_lock($1)`, lockID); err != nil {
		t.Fatal(err)
	}

	d, err := OpenDir(fixture(t, map[string]string{v1 + "_a.sql": fileA}))
	if err != nil {
		t.Fatal(err)
	}
	_, err = NewRunner(db, d, 300*time.Millisecond).Apply(context.Background())
	if err == nil || !strings.Contains(err.Error(), "another migration is running") {
		t.Fatalf("err = %v, want a lock error", err)
	}
	if hasTable(t, db, "a") {
		t.Fatal("migrated without holding the lock")
	}
}
