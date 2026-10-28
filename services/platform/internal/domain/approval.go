package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

// ErrEvidenceChanged is returned (wrapped) when a decision names Evidence other than the Evidence
// the Approval was requested with: the person judged something that is no longer what the gate
// holds. Test with errors.Is.
var ErrEvidenceChanged = errors.New("evidence changed since it was shown")

// ApprovalState is the lifecycle of an Approval (DOMAIN_MODEL §5.5). approved and rejected are
// terminal and never edited. Nothing produces expired yet: no timer exists.
type ApprovalState string

const (
	ApprovalRequested ApprovalState = "requested"
	ApprovalApproved  ApprovalState = "approved"
	ApprovalRejected  ApprovalState = "rejected"
	ApprovalExpired   ApprovalState = "expired"
	ApprovalVoided    ApprovalState = "voided"
)

// Terminal reports whether no decision or change can follow.
func (s ApprovalState) Terminal() bool { return s != ApprovalRequested }

// GateKind is what an Approval gates (POLICY_AND_APPROVAL_SHAPES §3.1).
type GateKind string

// GatePolicyRequired gates a Step whose Capability Policy answered require_approval. The
// explicit Approval Step and Work Item acceptance gates are not implemented.
const GatePolicyRequired GateKind = "policy_required"

// ApprovalGate names the Step the Approval holds and the Capability whose request produced it.
type ApprovalGate struct {
	Kind        GateKind   `json:"kind"`
	ExecutionID string     `json:"executionId"`
	StepID      string     `json:"stepId"`
	Capability  Capability `json:"capability"`
}

// EvidenceItem references one thing the approver is shown: a kind ("execution",
// "step_execution", "definition", ...), its id and, for content, its hash. An Approval holds
// references and hashes, never the content (POLICY_AND_APPROVAL_SHAPES §3.2).
type EvidenceItem struct {
	Kind string `json:"kind"`
	ID   string `json:"id,omitempty"`
	Hash string `json:"hash,omitempty"`
}

// Evidence is the list of items an approver is shown and the digest of its canonical form.
type Evidence struct {
	Digest string         `json:"digest"`
	Items  []EvidenceItem `json:"items"`
}

// MaxEvidenceItems bounds an Approval's Evidence.
const MaxEvidenceItems = 100

// NewEvidence sorts the items (by kind, id, then hash) and computes the digest, so the same set
// of items always has the same digest whatever order it was gathered in. It has no timestamps.
func NewEvidence(items ...EvidenceItem) (Evidence, error) {
	if len(items) == 0 || len(items) > MaxEvidenceItems {
		return Evidence{}, invalid("evidence needs between 1 and %d items", MaxEvidenceItems)
	}
	items = slices.Clone(items)
	for _, it := range items {
		if it.Kind == "" || (it.ID == "" && it.Hash == "") {
			return Evidence{}, invalid("an evidence item needs a kind and an id or a hash: %+v", it)
		}
	}
	slices.SortFunc(items, func(a, b EvidenceItem) int {
		return strings.Compare(a.Kind+"\x00"+a.ID+"\x00"+a.Hash, b.Kind+"\x00"+b.ID+"\x00"+b.Hash)
	})
	h := sha256.New()
	for _, it := range items {
		fmt.Fprintf(h, "%s\x00%s\x00%s\n", it.Kind, it.ID, it.Hash)
	}
	return Evidence{Digest: "sha256:" + hex.EncodeToString(h.Sum(nil)), Items: items}, nil
}

// Outcome of a decision.
type ApprovalOutcome string

const (
	OutcomeApproved ApprovalOutcome = "approved"
	OutcomeRejected ApprovalOutcome = "rejected"
)

// MaxRationale bounds a decision's rationale.
const MaxRationale = 1000

// ApprovalDecision records who decided, with which role, when, why and on which Evidence.
type ApprovalDecision struct {
	Outcome        ApprovalOutcome `json:"outcome"`
	Approver       string          `json:"approver"`
	ApproverRole   Role            `json:"approverRole"`
	Rationale      string          `json:"rationale,omitempty"`
	EvidenceDigest string          `json:"evidenceDigest"`
	DecidedAt      time.Time       `json:"decidedAt"`
}

// Approval is a human decision on one gated action, bound to the Evidence the person was shown
// (ADR-0036). It authorizes one occurrence. It is created `requested`, decided once, and kept.
type Approval struct {
	ID string `json:"id"`
	// Version is the optimistic-concurrency counter of the stored row, set by the store.
	Version   int64         `json:"version"`
	ProjectID string        `json:"projectId"`
	State     ApprovalState `json:"state"`
	Gate      ApprovalGate  `json:"gate"`
	// Requester is who started the Execution; MinRole is the lowest role that may decide, copied
	// at request time so a later change does not rewrite what the approver was asked to satisfy.
	Requester Principal `json:"requester"`
	MinRole   Role      `json:"minRole"`
	Evidence  Evidence  `json:"evidence"`
	// Decision is set once the Approval is approved or rejected.
	Decision    *ApprovalDecision `json:"decision,omitempty"`
	RequestedAt time.Time         `json:"requestedAt"`
	// VoidedAt and VoidReason are set when the Approval is voided.
	VoidedAt   *time.Time `json:"voidedAt,omitempty"`
	VoidReason string     `json:"voidReason,omitempty"`
}

// roleRank orders the fixed roles by authority.
func roleRank(r Role) int {
	switch r {
	case RoleOwner:
		return 5
	case RoleAdmin:
		return 4
	case RoleMaintainer:
		return 3
	case RoleContributor:
		return 2
	case RoleViewer:
		return 1
	}
	return 0
}

// MayDecideApprovals reports whether the role may decide an Approval at all: Maintainer and above
// (POLICY_AND_APPROVAL_SHAPES §2.1). A gate may ask for more through its MinRole.
func (r Role) MayDecideApprovals() bool { return roleRank(r) >= roleRank(RoleMaintainer) }

// NewApproval returns a requested Approval for a gated Step.
func NewApproval(now time.Time, id, projectID string, gate ApprovalGate, requester Principal, ev Evidence) (Approval, error) {
	a := Approval{ID: id, ProjectID: projectID, State: ApprovalRequested, Gate: gate, Requester: requester,
		MinRole: RoleMaintainer, Evidence: ev, RequestedAt: now}
	return a, a.Validate()
}

// Validate checks the shape of an Approval, including one read back from the store.
func (a Approval) Validate() error {
	switch {
	case a.ID == "":
		return invalid("approval id is required")
	case a.ProjectID == "":
		return invalid("approval %s: a project is required", a.ID)
	case a.Gate.Kind != GatePolicyRequired:
		return invalid("approval %s: gate kind %q is not supported", a.ID, a.Gate.Kind)
	case a.Gate.ExecutionID == "" || a.Gate.StepID == "" || !a.Gate.Capability.Valid():
		return invalid("approval %s: the gate names an execution, a step and a capability", a.ID)
	case a.Requester.ID == "" || !a.MinRole.Valid() || !a.MinRole.MayDecideApprovals():
		return invalid("approval %s: a requester and a minimum role of maintainer or above are required", a.ID)
	case a.Evidence.Digest == "" || len(a.Evidence.Items) == 0:
		return invalid("approval %s: evidence is required", a.ID)
	case a.RequestedAt.IsZero():
		return invalid("approval %s: requestedAt is required", a.ID)
	}
	switch a.State {
	case ApprovalRequested, ApprovalExpired:
		if a.Decision != nil || a.VoidedAt != nil {
			return invalid("approval %s: a %s approval has no decision", a.ID, a.State)
		}
	case ApprovalApproved, ApprovalRejected:
		d := a.Decision
		if d == nil || d.Outcome != ApprovalOutcome(a.State) || d.Approver == "" || d.DecidedAt.IsZero() || d.EvidenceDigest == "" {
			return invalid("approval %s: a %s approval records who decided, when and on which evidence", a.ID, a.State)
		}
	case ApprovalVoided:
		if a.VoidedAt == nil || a.VoidReason == "" {
			return invalid("approval %s: a voided approval records when and why", a.ID)
		}
	default:
		return invalid("approval %s: state %q is unknown", a.ID, a.State)
	}
	return nil
}

// Decide records a human's decision. By must be a human whose role is at least MinRole;
// seenDigest is the Evidence digest the person was shown and must equal the Approval's, or the
// decision is ErrEvidenceChanged and nothing is recorded. A rejection needs a rationale. An
// Approval that is not requested is ErrInvalidTransition. Agents, triggers and plugins never
// decide (ADR-0036), whatever role they carry.
func (a *Approval) Decide(now time.Time, by Principal, outcome ApprovalOutcome, rationale, seenDigest string) error {
	if by.Kind != PrincipalHuman || by.ID == "" {
		return fmt.Errorf("%w: only a human may decide an approval, not %q", ErrNotPermitted, by.Kind)
	}
	if !by.Role.Valid() || !by.Role.MayDecideApprovals() || roleRank(by.Role) < roleRank(a.MinRole) {
		return fmt.Errorf("%w: role %q may not decide this approval; %s or above only", ErrNotPermitted, by.Role, a.MinRole)
	}
	if outcome != OutcomeApproved && outcome != OutcomeRejected {
		return invalid("approval %s: outcome %q is not approved or rejected", a.ID, outcome)
	}
	if len(rationale) > MaxRationale {
		return invalid("approval %s: rationale is longer than %d bytes", a.ID, MaxRationale)
	}
	if outcome == OutcomeRejected && strings.TrimSpace(rationale) == "" {
		return ErrReasonRequired
	}
	if a.State != ApprovalRequested {
		return fmt.Errorf("%w: approval %s is %s", ErrInvalidTransition, a.ID, a.State)
	}
	if seenDigest != a.Evidence.Digest {
		return fmt.Errorf("%w: approval %s", ErrEvidenceChanged, a.ID)
	}
	a.State = ApprovalState(outcome)
	a.Decision = &ApprovalDecision{Outcome: outcome, Approver: by.ID, ApproverRole: by.Role, Rationale: rationale,
		EvidenceDigest: seenDigest, DecidedAt: now}
	return nil
}

// Void withdraws a requested Approval because the thing it gates ended, for example the
// Execution was cancelled. A decided Approval is never voided.
func (a *Approval) Void(now time.Time, reason string) error {
	if reason == "" {
		return ErrReasonRequired
	}
	if a.State != ApprovalRequested {
		return fmt.Errorf("%w: approval %s is %s", ErrInvalidTransition, a.ID, a.State)
	}
	a.State, a.VoidedAt, a.VoidReason = ApprovalVoided, &now, reason
	return nil
}
