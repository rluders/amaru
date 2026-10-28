package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
)

// TrustStore is the PostgreSQL ports.TrustStore.
type TrustStore struct{ db *sql.DB }

var _ ports.TrustStore = (*TrustStore)(nil)

// NewTrustStore returns a store over db, which must be connected through the pgx stdlib driver
// to a database at the current schema.
func NewTrustStore(db *sql.DB) *TrustStore { return &TrustStore{db: db} }

const trustColumns = `project_key, kind, ref, hash, trusted_by, trusted_at, revoked_by, revoked_at, revoke_reason`

const foreignKeyViolation = "23503"

func (s *TrustStore) Save(ctx context.Context, t domain.Trust) error {
	if err := t.Validate(); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("postgres: save trust: %w", err)
	}
	defer tx.Rollback()
	// Lock the row, so the audit event is derived from the record this write replaces.
	prev, err := scanTrust(tx.QueryRowContext(ctx,
		`SELECT `+trustColumns+` FROM project_trust WHERE project_key = $1 AND kind = $2 AND ref = $3 FOR UPDATE`,
		string(t.ProjectKey), string(t.Kind), t.Ref))
	var prevp *domain.Trust
	switch {
	case errors.Is(err, sql.ErrNoRows):
	case err != nil:
		return err
	default:
		prevp = &prev
	}
	ev, err := domain.TrustAuditEvent(prevp, t)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx,
		`INSERT INTO project_trust (`+trustColumns+`) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 ON CONFLICT (project_key, kind, ref) DO UPDATE SET hash = EXCLUDED.hash, trusted_by = EXCLUDED.trusted_by,
		   trusted_at = EXCLUDED.trusted_at, revoked_by = EXCLUDED.revoked_by, revoked_at = EXCLUDED.revoked_at,
		   revoke_reason = EXCLUDED.revoke_reason`,
		string(t.ProjectKey), string(t.Kind), t.Ref, t.Hash, t.TrustedBy, t.TrustedAt.UTC(), t.RevokedBy, nullTime(t.RevokedAt), t.RevokeReason)
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == foreignKeyViolation {
		return fmt.Errorf("%w: project %s", domain.ErrNotFound, t.ProjectKey)
	}
	if err != nil {
		return fmt.Errorf("postgres: save trust: %w", err)
	}
	if err := appendAudit(ctx, tx, ev); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("postgres: save trust: %w", err)
	}
	return nil
}

func (s *TrustStore) Get(ctx context.Context, key domain.ProjectKey, kind domain.TrustKind, ref string) (domain.Trust, error) {
	t, err := scanTrust(s.db.QueryRowContext(ctx,
		`SELECT `+trustColumns+` FROM project_trust WHERE project_key = $1 AND kind = $2 AND ref = $3`, string(key), string(kind), ref))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Trust{}, fmt.Errorf("%w: trust %s %s %s", domain.ErrNotFound, key, kind, ref)
	}
	return t, err
}

func (s *TrustStore) List(ctx context.Context, key domain.ProjectKey) ([]domain.Trust, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+trustColumns+` FROM project_trust WHERE project_key = $1 ORDER BY kind COLLATE "C", ref COLLATE "C"`, string(key))
	if err != nil {
		return nil, fmt.Errorf("postgres: list trust: %w", err)
	}
	defer rows.Close()
	out := []domain.Trust{}
	for rows.Next() {
		t, err := scanTrust(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// Audit implements ports.AuditLog.
func (s *TrustStore) Audit(ctx context.Context, after int64, limit int) ([]domain.AuditEvent, error) {
	return readAudit(ctx, s.db, after, limit)
}

func scanTrust(r rowScanner) (domain.Trust, error) {
	var t domain.Trust
	var key, kind string
	var revoked sql.NullTime
	if err := r.Scan(&key, &kind, &t.Ref, &t.Hash, &t.TrustedBy, &t.TrustedAt, &t.RevokedBy, &revoked, &t.RevokeReason); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Trust{}, err
		}
		return domain.Trust{}, fmt.Errorf("postgres: read trust: %w", err)
	}
	t.ProjectKey, t.Kind, t.TrustedAt = domain.ProjectKey(key), domain.TrustKind(kind), t.TrustedAt.UTC()
	t.RevokedAt = utcPtr(timePtr(revoked))
	return t, nil
}
