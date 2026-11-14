// Package evidence answers FEAT-ART-002: one query returns what the Platform holds about an
// Execution. It reads the stores and hands the result to domain.BuildExecutionEvidence; it writes
// nothing.
package evidence

import (
	"context"
	"errors"
	"fmt"

	"github.com/rluders/amaru/services/platform/internal/domain"
)

// Reader builds Evidence bundles. All three stores are required: a missing one would make a
// bundle look complete without a part of its content.
type Reader struct {
	Executions interface {
		Get(context.Context, string) (domain.Execution, []domain.StepExecution, error)
		Events(context.Context, string) ([]domain.ExecutionEvent, error)
		Approvals(context.Context, string) ([]domain.Approval, error)
		Audit(context.Context, int64, int) ([]domain.AuditEvent, error)
		UsageCaptures(context.Context, string) ([]domain.ResultCapture, error)
	}
	Contexts interface {
		Get(context.Context, string, string) (domain.StepContext, error)
	}
	Artifacts interface {
		ByExecution(context.Context, string) ([]domain.ArtifactVersion, error)
	}
}

// auditPage is how many audit entries one read asks for.
const auditPage = 1000

// Bundle returns the Evidence of the requested Execution, or domain.ErrNotFound. Store identity
// mismatches return domain.ErrInvalid; any read failure returns no bundle. The parts are read
// sequentially; a shared snapshot requires transaction-bound readers supplied by the adapter.
// Final reflects the Execution state read by Get; it does not
// certify snapshot consistency or prevent later Artifact versions from changing the Digest.
func (r Reader) Bundle(ctx context.Context, executionID string) (domain.ExecutionEvidence, error) {
	if r.Executions == nil || r.Contexts == nil || r.Artifacts == nil {
		return domain.ExecutionEvidence{}, fmt.Errorf("%w: evidence reader needs the execution, context and artifact stores", domain.ErrInvalid)
	}
	e, steps, err := r.Executions.Get(ctx, executionID)
	if err != nil {
		return domain.ExecutionEvidence{}, err
	}
	if e.ID != executionID {
		return domain.ExecutionEvidence{}, fmt.Errorf("%w: evidence store returned a different execution", domain.ErrInvalid)
	}
	in := domain.ExecutionEvidenceInput{Execution: e, Steps: steps}
	if in.Events, err = r.Executions.Events(ctx, executionID); err != nil {
		return domain.ExecutionEvidence{}, err
	}
	if in.Approvals, err = r.Executions.Approvals(ctx, executionID); err != nil {
		return domain.ExecutionEvidence{}, err
	}
	if in.Audit, err = r.approvalAudit(ctx, in.Approvals); err != nil {
		return domain.ExecutionEvidence{}, err
	}
	for _, st := range steps {
		c, err := r.Contexts.Get(ctx, executionID, st.ID)
		switch {
		case err == nil:
			if c.ExecutionID != executionID || c.StepID != st.ID {
				return domain.ExecutionEvidence{}, fmt.Errorf("%w: evidence store returned context for a different step", domain.ErrInvalid)
			}
			in.Contexts = append(in.Contexts, c)
		case errors.Is(err, domain.ErrNotFound): // no context was assembled for this Step
		default:
			return domain.ExecutionEvidence{}, err
		}
	}
	if in.Artifacts, err = r.Artifacts.ByExecution(ctx, executionID); err != nil {
		return domain.ExecutionEvidence{}, err
	}
	if in.Usage, err = r.Executions.UsageCaptures(ctx, executionID); err != nil {
		return domain.ExecutionEvidence{}, err
	}
	return domain.BuildExecutionEvidence(in)
}

// approvalAudit reads the whole audit log and keeps the entries about the given Approvals.
func (r Reader) approvalAudit(ctx context.Context, approvals []domain.Approval) ([]domain.AuditEvent, error) {
	if len(approvals) == 0 {
		return nil, nil
	}
	want := make(map[string]bool, len(approvals))
	for _, a := range approvals {
		want[a.ID] = true
	}
	var out []domain.AuditEvent
	for after := int64(0); ; {
		page, err := r.Executions.Audit(ctx, after, auditPage)
		if err != nil {
			return nil, err
		}
		if len(page) > auditPage {
			return nil, fmt.Errorf("%w: evidence audit page exceeds limit %d", domain.ErrInvalid, auditPage)
		}
		last := after
		for _, ev := range page {
			if ev.Seq <= last {
				return nil, fmt.Errorf("%w: evidence audit sequence %d does not advance past %d", domain.ErrInvalid, ev.Seq, last)
			}
			if ev.TargetType == "approval" && want[ev.TargetID] {
				out = append(out, ev)
			}
			last = ev.Seq
		}
		if len(page) < auditPage {
			return out, nil
		}
		after = last
	}
}
