package migrations

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path"
	"time"

	"github.com/pressly/goose/v3"
)

const (
	// versionsTable is goose's version table, under an Amaru name.
	versionsTable = "amaru_schema_migrations"
	// lockID is the PostgreSQL advisory lock that serializes migrating Platforms.
	lockID int64 = 0x616d6172755f6d67 // "amaru_mg"
	// lockPoll is how often a waiting runner retries the lock.
	lockPoll = 50 * time.Millisecond
)

// Runner applies a Dir to one PostgreSQL database. Open db with the pgx stdlib driver.
type Runner struct {
	db          *sql.DB
	dir         *Dir
	lockTimeout time.Duration
}

// NewRunner returns a Runner; lockTimeout bounds the wait for another migrating Platform.
func NewRunner(db *sql.DB, dir *Dir, lockTimeout time.Duration) *Runner {
	return &Runner{db: db, dir: dir, lockTimeout: lockTimeout}
}

// applied lists the applied versions without writing: a database that was never migrated has
// no version table and no applied version.
func (r *Runner) applied(ctx context.Context, q interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}) ([]string, error) {
	var exists bool
	if err := q.QueryRowContext(ctx, `SELECT to_regclass($1) IS NOT NULL`, versionsTable).Scan(&exists); err != nil || !exists {
		return nil, err
	}
	// goose records one row per apply or rollback; the latest row per version tells its state.
	rows, err := q.QueryContext(ctx, `SELECT version_id FROM (
		SELECT DISTINCT ON (version_id) version_id, is_applied FROM `+versionsTable+`
		WHERE version_id > 0 ORDER BY version_id, id DESC) t WHERE is_applied ORDER BY version_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var v int64
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out = append(out, fmt.Sprintf("%014d", v))
	}
	return out, rows.Err()
}

// Status reports where the database stands. It writes nothing; it returns ErrSchemaNewer when
// the database was migrated by a newer build.
func (r *Runner) Status(ctx context.Context) (Status, error) {
	done, err := r.applied(ctx, r.db)
	if err != nil {
		return Status{}, fmt.Errorf("migrations: read applied versions: %w", err)
	}
	return plan(r.dir.versions, done)
}

// Apply migrates the database to the newest version and returns the resulting Status. It takes
// the advisory lock, checks the database is not newer than this build, then applies each pending
// file in its own transaction with its version row, so a failing file changes nothing.
func (r *Runner) Apply(ctx context.Context) (Status, error) {
	conn, err := r.db.Conn(ctx)
	if err != nil {
		return Status{}, err
	}
	defer conn.Close()
	if err := r.lock(ctx, conn); err != nil {
		return Status{}, err
	}
	defer func() { _, _ = conn.ExecContext(context.WithoutCancel(ctx), `SELECT pg_advisory_unlock($1)`, lockID) }()

	st, err := r.Status(ctx)
	if err != nil || st.UpToDate() {
		return st, err
	}
	p, err := goose.NewProvider(goose.DialectPostgres, r.db, r.dir.fsys, goose.WithTableName(versionsTable))
	if err != nil {
		return st, fmt.Errorf("migrations: %w", err)
	}
	if _, err := p.Up(ctx); err != nil {
		var pe *goose.PartialError
		if errors.As(err, &pe) && pe.Failed != nil && pe.Failed.Source != nil {
			return st, fmt.Errorf("migrations: apply %s: %w", path.Base(pe.Failed.Source.Path), pe.Err)
		}
		return st, fmt.Errorf("migrations: apply: %w", err)
	}
	return r.Status(ctx)
}

func (r *Runner) lock(ctx context.Context, conn *sql.Conn) error {
	deadline := time.Now().Add(r.lockTimeout)
	for {
		var got bool
		if err := conn.QueryRowContext(ctx, `SELECT pg_try_advisory_lock($1)`, lockID).Scan(&got); err != nil {
			return fmt.Errorf("migrations: take lock: %w", err)
		}
		if got {
			return nil
		}
		if !time.Now().Before(deadline) {
			return fmt.Errorf("migrations: another migration is running (waited %s for the lock)", r.lockTimeout)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(lockPoll):
		}
	}
}
