package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
)

var _ ports.AuditLog = (*ExecutionStore)(nil)

const approvalColumns = `id, version, execution_id, step_id, project_id, state, gate_kind, capability, requester, min_role,
	evidence, evidence_digest, decision, requested_at, voided_at, void_reason`

// insertApproval stores a requested Approval at version 1 and its audit event, in tx.
func insertApproval(ctx context.Context, tx *sql.Tx, a domain.Approval) error {
	ev, err := domain.ApprovalAuditEvent(nil, a)
	if err != nil {
		return err
	}
	if err := a.Validate(); err != nil {
		return err
	}
	requester, err := json.Marshal(a.Requester)
	if err != nil {
		return fmt.Errorf("postgres: encode approval requester: %w", err)
	}
	items, err := json.Marshal(a.Evidence.Items)
	if err != nil {
		return fmt.Errorf("postgres: encode approval evidence: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO approvals (`+approvalColumns+`) VALUES ($1, 1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, NULL, $12, NULL, '')`,
		a.ID, a.Gate.ExecutionID, a.Gate.StepID, a.ProjectID, string(a.State), string(a.Gate.Kind), string(a.Gate.Capability),
		requester, string(a.MinRole), items, a.Evidence.Digest, a.RequestedAt.UTC()); err != nil {
		return mapCreateErr(err, "approval", a.ID)
	}
	return appendAudit(ctx, tx, ev)
}

// updateApprovals writes the changed Approvals of one Execution, each at the version it was read
// at, with its audit event, in tx. Approvals are written in id order, a stable lock order.
func (s *ExecutionStore) updateApprovals(ctx context.Context, tx *sql.Tx, execID string, in []domain.Approval) ([]domain.Approval, error) {
	out := make([]domain.Approval, len(in))
	for i, a := range in {
		var cur domain.Approval
		cur, err := scanApproval(tx.QueryRowContext(ctx, `SELECT `+approvalColumns+` FROM approvals WHERE id = $1 FOR UPDATE`, a.ID))
		if errors.Is(err, sql.ErrNoRows) || (err == nil && cur.Gate.ExecutionID != execID) {
			return nil, fmt.Errorf("%w: approval %s", domain.ErrNotFound, a.ID)
		}
		if err != nil {
			return nil, err
		}
		if cur.Version != a.Version {
			return nil, fmt.Errorf("%w: approval %s is at version %d, change read %d", domain.ErrConflict, a.ID, cur.Version, a.Version)
		}
		ev, err := domain.ApprovalAuditEvent(&cur, a)
		if err == nil {
			err = a.Validate()
		}
		if err == nil && (a.Gate != cur.Gate || a.Requester != cur.Requester || a.Evidence.Digest != cur.Evidence.Digest || a.MinRole != cur.MinRole || a.ProjectID != cur.ProjectID) {
			err = fmt.Errorf("%w: approval %s: only its state, decision and void record may change", domain.ErrInvalid, a.ID)
		}
		if err != nil {
			return nil, err
		}
		var decision []byte
		if a.Decision != nil {
			if decision, err = json.Marshal(a.Decision); err != nil {
				return nil, fmt.Errorf("postgres: encode approval decision: %w", err)
			}
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE approvals SET version = version + 1, state = $2, decision = $3, voided_at = $4, void_reason = $5 WHERE id = $1`,
			a.ID, string(a.State), decision, nullTime(a.VoidedAt), a.VoidReason); err != nil {
			return nil, fmt.Errorf("postgres: update approval %s: %w", a.ID, err)
		}
		if err := appendAudit(ctx, tx, ev); err != nil {
			return nil, err
		}
		a.Version = cur.Version + 1
		out[i] = a
	}
	return out, nil
}

func (s *ExecutionStore) Approval(ctx context.Context, id string) (domain.Approval, error) {
	a, err := scanApproval(s.db.QueryRowContext(ctx, `SELECT `+approvalColumns+` FROM approvals WHERE id = $1`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Approval{}, fmt.Errorf("%w: approval %s", domain.ErrNotFound, id)
	}
	return a, err
}

func (s *ExecutionStore) Approvals(ctx context.Context, executionID string) ([]domain.Approval, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, fmt.Errorf("postgres: approvals: %w", err)
	}
	defer tx.Rollback()
	var one int
	if err := tx.QueryRowContext(ctx, `SELECT 1 FROM executions WHERE id = $1`, executionID).Scan(&one); errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%w: execution %s", domain.ErrNotFound, executionID)
	} else if err != nil {
		return nil, fmt.Errorf("postgres: approvals: %w", err)
	}
	rows, err := tx.QueryContext(ctx, `SELECT `+approvalColumns+` FROM approvals WHERE execution_id = $1 ORDER BY requested_at, id`, executionID)
	if err != nil {
		return nil, fmt.Errorf("postgres: approvals: %w", err)
	}
	defer rows.Close()
	out := []domain.Approval{}
	for rows.Next() {
		a, err := scanApproval(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func scanApproval(r rowScanner) (domain.Approval, error) {
	var a domain.Approval
	var state, kind, capability, minRole string
	var requester, items, decision []byte
	var voided sql.NullTime
	err := r.Scan(&a.ID, &a.Version, &a.Gate.ExecutionID, &a.Gate.StepID, &a.ProjectID, &state, &kind, &capability, &requester, &minRole,
		&items, &a.Evidence.Digest, &decision, &a.RequestedAt, &voided, &a.VoidReason)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Approval{}, err
		}
		return domain.Approval{}, fmt.Errorf("postgres: read approval: %w", err)
	}
	a.State, a.Gate.Kind, a.Gate.Capability, a.MinRole = domain.ApprovalState(state), domain.GateKind(kind), domain.Capability(capability), domain.Role(minRole)
	if err := json.Unmarshal(requester, &a.Requester); err != nil {
		return domain.Approval{}, fmt.Errorf("postgres: decode approval %s requester: %w", a.ID, err)
	}
	if err := json.Unmarshal(items, &a.Evidence.Items); err != nil {
		return domain.Approval{}, fmt.Errorf("postgres: decode approval %s evidence: %w", a.ID, err)
	}
	if decision != nil {
		a.Decision = &domain.ApprovalDecision{}
		if err := json.Unmarshal(decision, a.Decision); err != nil {
			return domain.Approval{}, fmt.Errorf("postgres: decode approval %s decision: %w", a.ID, err)
		}
		a.Decision.DecidedAt = a.Decision.DecidedAt.UTC()
	}
	a.RequestedAt, a.VoidedAt = a.RequestedAt.UTC(), utcPtr(timePtr(voided))
	return a, nil
}

// Audit implements ports.AuditLog; the log is shared with the Grant store.
func (s *ExecutionStore) Audit(ctx context.Context, after int64, limit int) ([]domain.AuditEvent, error) {
	return readAudit(ctx, s.db, after, limit)
}
