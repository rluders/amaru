package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
)

// GrantStore is the PostgreSQL ports.GrantStore.
type GrantStore struct{ db *sql.DB }

var _ ports.GrantStore = (*GrantStore)(nil)

// NewGrantStore returns a store over db, which must be connected through the pgx stdlib driver
// to a database at the current schema.
func NewGrantStore(db *sql.DB) *GrantStore { return &GrantStore{db: db} }

const grantColumns = `id, version, level, project_id, subject_kind, subject_id, capabilities, resource_type, resource_value,
	definition_hash, granted_by, created_at, expires_at, status, revoked_by, revoked_at, revoke_reason`

func (s *GrantStore) Create(ctx context.Context, g domain.Grant) (domain.Grant, error) {
	if err := g.Validate(); err != nil {
		return domain.Grant{}, err
	}
	caps, err := json.Marshal(g.Capabilities)
	if err != nil {
		return domain.Grant{}, fmt.Errorf("postgres: encode grant capabilities: %w", err)
	}
	g.Version = 1
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO grants (`+grantColumns+`) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)`,
		g.ID, g.Version, string(g.Level), g.ProjectID, string(g.Subject.Kind), g.Subject.ID, caps, g.Resource.Type, g.Resource.Value,
		g.DefinitionHash, g.GrantedBy, g.CreatedAt.UTC(), nullTime(g.ExpiresAt), string(g.Status), g.RevokedBy, nullTime(g.RevokedAt), g.RevokeReason)
	if err != nil {
		return domain.Grant{}, mapCreateErr(err, "grant", g.ID)
	}
	return s.Get(ctx, g.ID)
}

func (s *GrantStore) Get(ctx context.Context, id string) (domain.Grant, error) {
	g, err := scanGrant(s.db.QueryRowContext(ctx, `SELECT `+grantColumns+` FROM grants WHERE id = $1`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Grant{}, fmt.Errorf("%w: grant %s", domain.ErrNotFound, id)
	}
	return g, err
}

func (s *GrantStore) Update(ctx context.Context, g domain.Grant) (domain.Grant, error) {
	if err := g.Validate(); err != nil {
		return domain.Grant{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.Grant{}, fmt.Errorf("postgres: update grant: %w", err)
	}
	defer tx.Rollback()
	// Lock the row, so the comparison below and the write see the same Grant.
	old, err := scanGrant(tx.QueryRowContext(ctx, `SELECT `+grantColumns+` FROM grants WHERE id = $1 FOR UPDATE`, g.ID))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Grant{}, fmt.Errorf("%w: grant %s", domain.ErrNotFound, g.ID)
	}
	if err != nil {
		return domain.Grant{}, err
	}
	if old.Version != g.Version {
		return domain.Grant{}, fmt.Errorf("%w: grant %s is at version %d, not %d", domain.ErrConflict, g.ID, old.Version, g.Version)
	}
	probe := g
	probe.Status, probe.RevokedBy, probe.RevokedAt, probe.RevokeReason, probe.Version = old.Status, old.RevokedBy, old.RevokedAt, old.RevokeReason, old.Version
	probe.CreatedAt, probe.ExpiresAt, probe.RevokedAt = probe.CreatedAt.UTC(), utcPtr(probe.ExpiresAt), utcPtr(probe.RevokedAt)
	a, _ := json.Marshal(probe)
	b, _ := json.Marshal(old)
	if string(a) != string(b) {
		return domain.Grant{}, fmt.Errorf("%w: grant %s: only its status and revocation may change", domain.ErrInvalid, g.ID)
	}
	if old.Status != domain.GrantActive && g.Status != old.Status {
		return domain.Grant{}, fmt.Errorf("%w: grant %s is %s", domain.ErrInvalidTransition, g.ID, old.Status)
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE grants SET version = version + 1, status = $2, revoked_by = $3, revoked_at = $4, revoke_reason = $5 WHERE id = $1`,
		g.ID, string(g.Status), g.RevokedBy, nullTime(g.RevokedAt), g.RevokeReason); err != nil {
		return domain.Grant{}, fmt.Errorf("postgres: update grant %s: %w", g.ID, err)
	}
	if err := tx.Commit(); err != nil {
		return domain.Grant{}, fmt.Errorf("postgres: update grant: %w", err)
	}
	return s.Get(ctx, g.ID)
}

func (s *GrantStore) Active(ctx context.Context, projectID string) ([]domain.Grant, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+grantColumns+` FROM grants WHERE status = 'active' AND (level = 'organization' OR project_id = $1) ORDER BY id`, projectID)
	if err != nil {
		return nil, fmt.Errorf("postgres: list active grants: %w", err)
	}
	defer rows.Close()
	var out []domain.Grant
	for rows.Next() {
		g, err := scanGrant(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

type rowScanner interface{ Scan(dest ...any) error }

func scanGrant(r rowScanner) (domain.Grant, error) {
	var g domain.Grant
	var level, kind, status string
	var caps []byte
	var expires, revoked sql.NullTime
	err := r.Scan(&g.ID, &g.Version, &level, &g.ProjectID, &kind, &g.Subject.ID, &caps, &g.Resource.Type, &g.Resource.Value,
		&g.DefinitionHash, &g.GrantedBy, &g.CreatedAt, &expires, &status, &g.RevokedBy, &revoked, &g.RevokeReason)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Grant{}, err
		}
		return domain.Grant{}, fmt.Errorf("postgres: read grant: %w", err)
	}
	if err := json.Unmarshal(caps, &g.Capabilities); err != nil {
		return domain.Grant{}, fmt.Errorf("postgres: decode grant %s capabilities: %w", g.ID, err)
	}
	g.Level, g.Subject.Kind, g.Status = domain.GrantLevel(level), domain.SubjectKind(kind), domain.GrantStatus(status)
	g.CreatedAt = g.CreatedAt.UTC()
	g.ExpiresAt, g.RevokedAt = utcPtr(timePtr(expires)), utcPtr(timePtr(revoked))
	return g, nil
}

func utcPtr(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}
