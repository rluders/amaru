package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
)

var _ ports.AuditLog = (*GrantStore)(nil)

const defaultAuditLimit = 1000

// appendAudit stores ev in tx with the next Seq. It locks the counter row, so Seq is assigned in
// commit order and has no gaps; a rolled-back transaction gives its number back.
func appendAudit(ctx context.Context, tx *sql.Tx, ev domain.AuditEvent) error {
	var seq int64
	if err := tx.QueryRowContext(ctx, `UPDATE audit_counter SET last_seq = last_seq + 1 RETURNING last_seq`).Scan(&seq); err != nil {
		return fmt.Errorf("postgres: next audit seq: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO audit_events (seq, at, kind, actor, actor_kind, target_type, target_id, project_id, reason) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		seq, ev.At.UTC(), ev.Kind, ev.Actor, string(ev.ActorKind), ev.TargetType, ev.TargetID, ev.ProjectID, ev.Reason); err != nil {
		return fmt.Errorf("postgres: append audit event: %w", err)
	}
	return nil
}

// Audit implements ports.AuditLog.
func (s *GrantStore) Audit(ctx context.Context, after int64, limit int) ([]domain.AuditEvent, error) {
	if limit <= 0 {
		limit = defaultAuditLimit
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT seq, at, kind, actor, actor_kind, target_type, target_id, project_id, reason FROM audit_events WHERE seq > $1 ORDER BY seq LIMIT $2`, after, limit)
	if err != nil {
		return nil, fmt.Errorf("postgres: read audit log: %w", err)
	}
	defer rows.Close()
	out := []domain.AuditEvent{}
	for rows.Next() {
		var ev domain.AuditEvent
		var kind string
		if err := rows.Scan(&ev.Seq, &ev.At, &ev.Kind, &ev.Actor, &kind, &ev.TargetType, &ev.TargetID, &ev.ProjectID, &ev.Reason); err != nil {
			return nil, fmt.Errorf("postgres: read audit log: %w", err)
		}
		ev.ActorKind, ev.At = domain.ActorKind(kind), ev.At.UTC()
		out = append(out, ev)
	}
	return out, rows.Err()
}
