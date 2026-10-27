package migrations

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"ariga.io/atlas/sql/migrate"
)

// revisionsTable holds one row per migration file. It is created on first write and is not
// part of any migration file: the runner must be able to record the first one.
const revisionsTable = "amaru_schema_revisions"

const createRevisions = `CREATE TABLE IF NOT EXISTS ` + revisionsTable + ` (
	version          text        PRIMARY KEY,
	description      text        NOT NULL,
	type             bigint      NOT NULL,
	applied          integer     NOT NULL,
	total            integer     NOT NULL,
	executed_at      timestamptz NOT NULL,
	execution_time   bigint      NOT NULL,
	error            text        NOT NULL DEFAULT '',
	error_stmt       text        NOT NULL DEFAULT '',
	hash             text        NOT NULL,
	partial_hashes   jsonb       NOT NULL DEFAULT '[]',
	operator_version text        NOT NULL DEFAULT ''
)`

// querier is the part of *sql.DB and *sql.Tx the revisions store needs.
type querier interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

// revisions implements migrate.RevisionReadWriter on PostgreSQL.
type revisions struct{ q querier }

func (revisions) Ident() *migrate.TableIdent { return &migrate.TableIdent{Name: revisionsTable} }

func (r revisions) exists(ctx context.Context) (bool, error) {
	var ok bool
	err := r.q.QueryRowContext(ctx, `SELECT to_regclass($1) IS NOT NULL`, revisionsTable).Scan(&ok)
	return ok, err
}

const selectRevisions = `SELECT version, description, type, applied, total, executed_at, execution_time,
	error, error_stmt, hash, partial_hashes, operator_version FROM ` + revisionsTable

func scanRevision(s interface{ Scan(...any) error }) (*migrate.Revision, error) {
	var (
		rev     migrate.Revision
		typ, ex int64
		partial []byte
	)
	if err := s.Scan(&rev.Version, &rev.Description, &typ, &rev.Applied, &rev.Total, &rev.ExecutedAt, &ex,
		&rev.Error, &rev.ErrorStmt, &rev.Hash, &partial, &rev.OperatorVersion); err != nil {
		return nil, err
	}
	rev.Type = migrate.RevisionType(typ)
	rev.ExecutionTime = time.Duration(ex)
	if err := json.Unmarshal(partial, &rev.PartialHashes); err != nil {
		return nil, err
	}
	return &rev, nil
}

func (r revisions) ReadRevisions(ctx context.Context) ([]*migrate.Revision, error) {
	if ok, err := r.exists(ctx); err != nil || !ok {
		return nil, err
	}
	rows, err := r.q.QueryContext(ctx, selectRevisions+` ORDER BY version`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*migrate.Revision
	for rows.Next() {
		rev, err := scanRevision(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rev)
	}
	return out, rows.Err()
}

func (r revisions) ReadRevision(ctx context.Context, version string) (*migrate.Revision, error) {
	ok, err := r.exists(ctx)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, migrate.ErrRevisionNotExist
	}
	rev, err := scanRevision(r.q.QueryRowContext(ctx, selectRevisions+` WHERE version = $1`, version))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, migrate.ErrRevisionNotExist
	}
	return rev, err
}

func (r revisions) WriteRevision(ctx context.Context, rev *migrate.Revision) error {
	if _, err := r.q.ExecContext(ctx, createRevisions); err != nil {
		return err
	}
	partial, err := json.Marshal(append([]string{}, rev.PartialHashes...))
	if err != nil {
		return err
	}
	_, err = r.q.ExecContext(ctx, `INSERT INTO `+revisionsTable+`
		(version, description, type, applied, total, executed_at, execution_time, error, error_stmt, hash, partial_hashes, operator_version)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		ON CONFLICT (version) DO UPDATE SET description = EXCLUDED.description, type = EXCLUDED.type,
			applied = EXCLUDED.applied, total = EXCLUDED.total, executed_at = EXCLUDED.executed_at,
			execution_time = EXCLUDED.execution_time, error = EXCLUDED.error, error_stmt = EXCLUDED.error_stmt,
			hash = EXCLUDED.hash, partial_hashes = EXCLUDED.partial_hashes, operator_version = EXCLUDED.operator_version`,
		rev.Version, rev.Description, int64(rev.Type), rev.Applied, rev.Total, rev.ExecutedAt, int64(rev.ExecutionTime),
		rev.Error, rev.ErrorStmt, rev.Hash, string(partial), rev.OperatorVersion)
	return err
}

func (r revisions) DeleteRevision(ctx context.Context, version string) error {
	_, err := r.q.ExecContext(ctx, `DELETE FROM `+revisionsTable+` WHERE version = $1`, version)
	return err
}
