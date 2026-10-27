package migrations

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"ariga.io/atlas/sql/migrate"
	"ariga.io/atlas/sql/postgres"
)

// lockName is hashed by Atlas into a PostgreSQL advisory lock id.
const lockName = "amaru_migrate"

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

func (r *Runner) hashes() (map[string]string, error) {
	sum, err := r.dir.Checksum()
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, f := range r.dir.files {
		h, err := sum.SumByName(f.Name())
		if err != nil {
			return nil, err
		}
		out[f.Version()] = h
	}
	return out, nil
}

func (r *Runner) status(ctx context.Context, q querier) (Status, error) {
	revs, err := revisions{q}.ReadRevisions(ctx)
	if err != nil {
		return Status{}, fmt.Errorf("migrations: read revisions: %w", err)
	}
	var done []Applied
	for _, rev := range revs {
		if rev.Applied != rev.Total {
			return Status{}, fmt.Errorf("migrations: version %s is partially applied (%d of %d statements); restore from backup", rev.Version, rev.Applied, rev.Total)
		}
		done = append(done, Applied{Version: rev.Version, Hash: rev.Hash})
	}
	h, err := r.hashes()
	if err != nil {
		return Status{}, err
	}
	return plan(h, done)
}

// Status reports where the database stands. It writes nothing; it returns ErrSchemaNewer when
// the database was migrated by a newer build.
func (r *Runner) Status(ctx context.Context) (Status, error) { return r.status(ctx, r.db) }

// Apply migrates the database to the newest version and returns the resulting Status.
// It takes the advisory lock, checks the database is not newer than this build, then applies
// each pending file and its revision row in one transaction, so a failing file changes nothing.
func (r *Runner) Apply(ctx context.Context) (Status, error) {
	drv, err := postgres.Open(r.db)
	if err != nil {
		return Status{}, err
	}
	unlock, err := drv.Lock(ctx, lockName, r.lockTimeout)
	if err != nil {
		return Status{}, fmt.Errorf("migrations: another migration is running: %w", err)
	}
	defer func() { _ = unlock() }()

	st, err := r.status(ctx, r.db)
	if err != nil {
		return st, err
	}
	for _, v := range st.Pending {
		if err := r.applyOne(ctx, v); err != nil {
			return st, err
		}
	}
	return r.status(ctx, r.db)
}

func (r *Runner) applyOne(ctx context.Context, version string) (err error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, tx.Rollback())
		}
	}()
	drv, err := postgres.Open(tx)
	if err != nil {
		return err
	}
	ex, err := migrate.NewExecutor(drv, r.dir, revisions{tx}, migrate.WithAllowDirty(true), migrate.WithOperatorVersion("amaru"))
	if err != nil {
		return err
	}
	f := r.dir.file(version)
	if err := ex.Execute(ctx, f); err != nil {
		return fmt.Errorf("migrations: apply %s: %w", f.Name(), err)
	}
	return tx.Commit()
}
