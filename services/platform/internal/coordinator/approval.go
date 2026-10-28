package coordinator

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
)

// CmdDecideApproval is the command name under which a decision's outcome is recorded.
const CmdDecideApproval = "approval.decide"

// ErrCodeApprovalRejected is the error code of a Step that ended because its Approval was
// rejected.
const ErrCodeApprovalRejected = "approval_rejected"

// Approvals (FEAT-REV-001, ADR-0036). When Policy answers require_approval for a Step, Submit
// creates the Step awaiting_approval and one requested Approval in the same write, so a restart
// finds both. Nothing runs while a Step waits. A human with the role the gate asks for decides
// with DecideApproval: approving queues the Step (the lease path then lets it start, because an
// approved Approval covers its require_approval); rejecting ends it failed with the rationale.
// Cancelling the Execution withdraws a waiting Step and voids its Approval.
//
// Not implemented: the explicit Approval Step of a Workflow and Work Item acceptance gates, a
// Workflow's choice to reroute after a rejection (a rejection ends the Step), expiry (no timer
// exists), separation of duties and Team rules (FEAT-REV-007), re-requesting when the Evidence
// changes (a Step whose Evidence changed after the request cannot be approved and is not
// re-requested: reject or cancel it), the commit, diff and check-result items of FEAT-REV-003
// (the Platform records none: the Evidence is the Execution, the Step, its inputs, the pinned
// definition, Skills and Agent, and the assembled context), the inbox (FEAT-REV-002) and an Event
// for a refused decision attempt.

// approvalEvidence is what an approver is shown for a gated Step: which run and Step, exactly what
// the Step is asked to do, the definition it came from, the Skills and Agent definition the Step
// pinned, and the context its Agent will be told once one is assembled. An approval is only good
// for these exact items: if any changes, the digest changes and the approval no longer covers the
// Step (FEAT-REV-003).
func approvalEvidence(e domain.Execution, st domain.StepExecution, sc *domain.StepContext) (domain.Evidence, error) {
	sum := sha256.Sum256(append([]byte(st.Capability+"\x00"), st.InputsJSON...))
	items := []domain.EvidenceItem{
		{Kind: "execution", ID: e.ID},
		{Kind: "step_execution", ID: st.ID},
		{Kind: "step_inputs", Hash: "sha256:" + hex.EncodeToString(sum[:])},
	}
	if e.Definition != nil {
		items = append(items, domain.EvidenceItem{Kind: "definition", ID: e.Definition.WorkflowID, Hash: e.Definition.Hash})
	}
	for _, sk := range st.Skills {
		items = append(items, domain.EvidenceItem{Kind: "skill", ID: sk.Name + "@" + sk.Version, Hash: "sha256:" + sk.Hash})
	}
	if st.Agent != nil {
		items = append(items, domain.EvidenceItem{Kind: "agent", ID: st.Agent.ID, Hash: st.Agent.Hash})
	}
	if sc != nil {
		items = append(items, domain.EvidenceItem{Kind: "context", ID: st.ID, Hash: sc.Hash})
	}
	return domain.NewEvidence(items...)
}

// currentEvidence is the Evidence the Step has now, read from the stores: what the Platform would
// show an approver if it asked today. It is compared with the Evidence an Approval recorded.
func (c *Coordinator) currentEvidence(ctx context.Context, e domain.Execution, st domain.StepExecution) (domain.Evidence, error) {
	var sc *domain.StepContext
	if c.contexts != nil {
		got, err := c.contexts.Get(ctx, e.ID, st.ID)
		switch {
		case err == nil:
			sc = &got
		case errors.Is(err, domain.ErrNotFound): // no context was assembled yet
		default:
			return domain.Evidence{}, err
		}
	}
	return approvalEvidence(e, st, sc)
}

// DecideRequest is a human's decision on an Approval.
type DecideRequest struct {
	// Key is the idempotency key. A repeat with the same request returns the recorded outcome.
	Key        string
	ApprovalID string
	By         domain.Principal
	Outcome    domain.ApprovalOutcome
	Rationale  string
	// EvidenceDigest is the digest of the Evidence the person was shown. A decision on any other
	// is refused with domain.ErrEvidenceChanged.
	EvidenceDigest string
}

// Decided is the outcome of DecideApproval.
type Decided struct {
	Approval domain.Approval      `json:"approval"`
	Step     domain.StepExecution `json:"step"`
}

// DecideApproval records a decision and, in the same write, queues the Step (approved) or ends it
// failed (rejected). It returns replayed true, with the recorded outcome, when Key was used before
// with the same request. A person without the role, an Agent, a stale digest, a rejection without
// a rationale and an Approval already decided or voided write nothing.
func (c *Coordinator) DecideApproval(ctx context.Context, req DecideRequest) (Decided, bool, error) {
	if req.Key == "" || req.ApprovalID == "" {
		return Decided{}, false, fmt.Errorf("%w: a decision needs an idempotency key and an approval id", domain.ErrInvalid)
	}
	fp, err := fingerprint(struct {
		Approval, Rationale, Digest string
		By                          domain.Principal
		Outcome                     domain.ApprovalOutcome
	}{req.ApprovalID, req.Rationale, req.EvidenceDigest, req.By, req.Outcome})
	if err != nil {
		return Decided{}, false, err
	}
	for range maxEditAttempts {
		if out, found, err := replay[Decided](ctx, c, CmdDecideApproval, req.Key, fp); err != nil || found {
			return out, found, err
		}
		a, err := c.store.Approval(ctx, req.ApprovalID)
		if err != nil {
			return Decided{}, false, err
		}
		e, steps, err := c.store.Get(ctx, a.Gate.ExecutionID)
		if err != nil {
			return Decided{}, false, err
		}
		i := indexOfStep(steps, a.Gate.StepID)
		if i < 0 {
			return Decided{}, false, fmt.Errorf("%w: step %s", domain.ErrNotFound, a.Gate.StepID)
		}
		now := c.now()
		if err := a.Decide(now, req.By, req.Outcome, req.Rationale, req.EvidenceDigest); err != nil {
			return Decided{}, false, err
		}
		st := steps[i]
		if req.Outcome == domain.OutcomeApproved {
			// An approval is for the Evidence as it is now. A Step whose Evidence changed since
			// the Approval was requested cannot be approved; a rejection is always allowed.
			cur, err := c.currentEvidence(ctx, e, st)
			if err != nil {
				return Decided{}, false, err
			}
			if cur.Digest != a.Evidence.Digest {
				return Decided{}, false, fmt.Errorf("%w: approval %s was requested for other Evidence than step %s has now", domain.ErrEvidenceChanged, a.ID, st.ID)
			}
		}
		ev := domain.ExecutionEvent{StepID: st.ID, Reason: req.Rationale, At: now}
		if req.Outcome == domain.OutcomeApproved {
			err, ev.Kind = st.Approve(), domain.EventApprovalApproved
		} else {
			err, ev.Kind = st.Reject(now, &domain.ExecError{Code: ErrCodeApprovalRejected, Message: req.Rationale}), domain.EventApprovalRejected
		}
		if err != nil {
			return Decided{}, false, err
		}
		// The store raises each version by one, so the outcome can be recorded before the write and
		// a replay returns exactly what this call returns.
		a.Version++
		st.Version++
		out := Decided{Approval: a, Step: st}
		a.Version--
		st.Version--
		result, err := json.Marshal(out)
		if err != nil {
			return Decided{}, false, fmt.Errorf("encode outcome: %w", err)
		}
		_, err = c.store.Commit(ctx, ports.ExecutionChange{Steps: []domain.StepExecution{st}, Approvals: []domain.Approval{a},
			Events:  []domain.ExecutionEvent{ev},
			Command: &domain.CommandRecord{Name: CmdDecideApproval, Key: req.Key, Fingerprint: fp, Result: result, At: now}})
		if errors.Is(err, domain.ErrConflict) || errors.Is(err, domain.ErrDuplicateCommand) {
			continue // the Approval, the Step or the key moved under us; read again
		}
		if err != nil {
			return Decided{}, false, err
		}
		c.Wake()
		return out, false, nil
	}
	return Decided{}, false, fmt.Errorf("%w: approval %s kept changing", domain.ErrConflict, req.ApprovalID)
}

func indexOfStep(steps []domain.StepExecution, id string) int {
	for i := range steps {
		if steps[i].ID == id {
			return i
		}
	}
	return -1
}

// voidWaiting voids the requested Approvals that gate the Steps in ended, because the Execution
// was cancelled. A decided Approval is left as it is.
func (c *Coordinator) voidWaiting(ctx context.Context, executionID string, ended []string, now time.Time) ([]domain.Approval, error) {
	all, err := c.store.Approvals(ctx, executionID)
	if err != nil {
		return nil, err
	}
	var out []domain.Approval
	for _, a := range all {
		if a.State != domain.ApprovalRequested || !slices.Contains(ended, a.Gate.StepID) {
			continue
		}
		if err := a.Void(now, "execution_cancelled"); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, nil
}

// approvedFor returns the approved Approval that covers the Step, if there is one.
func approvedFor(approvals []domain.Approval, stepID string) (domain.Approval, bool) {
	i := slices.IndexFunc(approvals, func(a domain.Approval) bool {
		return a.Gate.StepID == stepID && a.State == domain.ApprovalApproved
	})
	if i < 0 {
		return domain.Approval{}, false
	}
	return approvals[i], true
}

// request puts a pending Step into awaiting_approval and returns the requested Approval for it.
func (c *Coordinator) request(now time.Time, e domain.Execution, st *domain.StepExecution, by domain.Principal) (domain.Approval, error) {
	ev, err := approvalEvidence(e, *st, nil)
	if err != nil {
		return domain.Approval{}, err
	}
	if err := st.RequireApproval(); err != nil {
		return domain.Approval{}, err
	}
	gate := domain.ApprovalGate{Kind: domain.GatePolicyRequired, ExecutionID: e.ID, StepID: st.ID, Capability: domain.Capability(st.Capability)}
	a, err := domain.NewApproval(now, c.newApprovalID(), e.ProjectID, gate, by, ev)
	a.Version = 1 // the store creates every row at version 1, so the outcome can be recorded before the write
	return a, err
}
