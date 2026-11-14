package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/evidence"
)

// metadataQuery is deliberately read-only. Both sql.DB and sql.Tx implement it.
// These helpers never open nested transactions or fall back to another connection.
type metadataQuery interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

// EvidenceReader composes metadata from one PostgreSQL database. It is internal,
// unscoped and unsuitable for direct API exposure: it performs no authorization
// and does not read blobs. Use the ordinary stores for writes.
type EvidenceReader struct{ db *sql.DB }

func NewEvidenceReader(db *sql.DB) *EvidenceReader { return &EvidenceReader{db: db} }

// Bundle reuses the Evidence assembler within one read-only repeatable-read
// transaction, including every audit page. Its snapshot starts at the first
// metadata query. A later call can see later commits; Final does not seal it.
// The scope lasts at most 30 seconds (or the caller's earlier deadline).
func (r *EvidenceReader) Bundle(ctx context.Context, id string) (domain.ExecutionEvidence, error) {
	if r == nil || r.db == nil {
		return domain.ExecutionEvidence{}, fmt.Errorf("%w: evidence reader needs a database", domain.ErrInvalid)
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return domain.ExecutionEvidence{}, fmt.Errorf("postgres: evidence read scope: %w", err)
	}
	defer tx.Rollback()
	b, err := readEvidence(ctx, tx, id)
	if err != nil {
		return domain.ExecutionEvidence{}, err
	}
	if err := ctx.Err(); err != nil {
		return domain.ExecutionEvidence{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.ExecutionEvidence{}, fmt.Errorf("postgres: finish evidence read scope: %w", err)
	}
	return b, nil
}

func readEvidence(ctx context.Context, q metadataQuery, id string) (domain.ExecutionEvidence, error) {
	return (evidence.Reader{Executions: executionMetadata{q}, Contexts: contextMetadata{q}, Artifacts: artifactMetadata{q}}).Bundle(ctx, id)
}

type executionMetadata struct{ q metadataQuery }

func (r executionMetadata) Get(ctx context.Context, id string) (domain.Execution, []domain.StepExecution, error) {
	return getExecution(ctx, r.q, id)
}
func (r executionMetadata) Events(ctx context.Context, id string) ([]domain.ExecutionEvent, error) {
	return executionEvents(ctx, r.q, id)
}
func (r executionMetadata) Approvals(ctx context.Context, id string) ([]domain.Approval, error) {
	return executionApprovals(ctx, r.q, id)
}
func (r executionMetadata) Audit(ctx context.Context, after int64, limit int) ([]domain.AuditEvent, error) {
	return readAudit(ctx, r.q, after, limit)
}
func (r executionMetadata) UsageCaptures(ctx context.Context, id string) ([]domain.ResultCapture, error) {
	return usageCaptures(ctx, r.q, id)
}

type contextMetadata struct{ q metadataQuery }

func (r contextMetadata) Get(ctx context.Context, execution, step string) (domain.StepContext, error) {
	return getContext(ctx, r.q, execution, step)
}

type artifactMetadata struct{ q metadataQuery }

func (r artifactMetadata) ByExecution(ctx context.Context, id string) ([]domain.ArtifactVersion, error) {
	return artifactsByExecution(ctx, r.q, id)
}
