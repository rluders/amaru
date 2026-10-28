package domain

import (
	"fmt"
	"time"
)

// Kinds of AuditEvent. Each is recorded in the same transaction as the change it describes
// (FEAT-POL-003). Approval kinds are added with the Approval Step; nothing records one yet.
const (
	AuditGrantCreated = "grant.created"
	AuditGrantRevoked = "grant.revoked"
	AuditGrantLapsed  = "grant.lapsed"
	AuditGrantExpired = "grant.expired"

	AuditApprovalRequested = "approval.requested"
	AuditApprovalApproved  = "approval.approved"
	AuditApprovalRejected  = "approval.rejected"
	AuditApprovalVoided    = "approval.voided"

	AuditTrustGranted = "trust.granted"
	AuditTrustRevoked = "trust.revoked"
)

// Who made a change. A person's change records the person's id; a change nobody asked for, such
// as a Grant lapsing or expiring, is the Platform's.
const (
	// ActorPlatform is the actor of a change nobody asked for, and its id.
	ActorPlatform ActorKind = "platform"
)

// actorPolicyID is the id of the actor (ActorPolicy, work.go) of an Approval Policy asked for.
const actorPolicyID = "policy"

// AuditEvent is one entry of the Platform's append-only audit log of approvals, grants and
// policy changes. Seq starts at 1 and has no gaps across the whole log; the store assigns it, in
// the same transaction as the change the event records, so a change without an event, or an event
// without a change, cannot be stored.
type AuditEvent struct {
	Seq  int64     `json:"seq"`
	At   time.Time `json:"at"`
	Kind string    `json:"kind"`
	// Actor is the id of the human who made the change, or ActorPlatform. ActorKind says which
	// (ActorHuman or ActorPlatform).
	Actor     string    `json:"actor"`
	ActorKind ActorKind `json:"actorKind"`
	// TargetType and TargetID name what changed (a "grant" and its id).
	TargetType string `json:"targetType"`
	TargetID   string `json:"targetId"`
	// ProjectID is the Project the change applies to; empty for the Organization.
	ProjectID string `json:"projectId,omitempty"`
	// Reason is why: the human's revoke reason, or the code the Platform gives for a lapse.
	Reason string `json:"reason,omitempty"`
}

// Audit reasons the Platform gives for a change nobody asked for.
const (
	AuditReasonDefinitionChanged = "definition_changed"
	AuditReasonExpired           = "expired"
)

// GrantAuditEvent returns the event that records the change from prev to next. A nil prev means
// next was just created. The store calls it inside the write, so every path that changes a Grant
// is audited and none can skip it. It returns ErrInvalid for a change that is not a Grant
// lifecycle step, which the store refuses anyway. Seq is left for the store to assign. The time
// is the Grant's own (created, revoked); a lapse or expiry has none, so it takes now.
func GrantAuditEvent(prev *Grant, next Grant, now time.Time) (AuditEvent, error) {
	ev := AuditEvent{At: now, TargetType: "grant", TargetID: next.ID, ProjectID: next.ProjectID}
	switch {
	case prev == nil:
		if next.Status != GrantActive {
			return AuditEvent{}, invalid("grant %s: a new grant is active, not %s", next.ID, next.Status)
		}
		ev.Kind, ev.Actor, ev.ActorKind, ev.At = AuditGrantCreated, next.GrantedBy, ActorHuman, next.CreatedAt
	case prev.Status != GrantActive || next.Status == GrantActive:
		return AuditEvent{}, invalid("grant %s: %s to %s is not a lifecycle change", next.ID, prev.Status, next.Status)
	case next.Status == GrantRevoked:
		ev.Kind, ev.Actor, ev.ActorKind, ev.Reason = AuditGrantRevoked, next.RevokedBy, ActorHuman, next.RevokeReason
		if next.RevokedAt != nil {
			ev.At = *next.RevokedAt
		}
	case next.Status == GrantLapsed:
		ev.Kind, ev.Actor, ev.ActorKind, ev.Reason = AuditGrantLapsed, string(ActorPlatform), ActorPlatform, AuditReasonDefinitionChanged
	case next.Status == GrantExpired:
		ev.Kind, ev.Actor, ev.ActorKind, ev.Reason = AuditGrantExpired, string(ActorPlatform), ActorPlatform, AuditReasonExpired
	default:
		return AuditEvent{}, fmt.Errorf("%w: grant %s: unknown status %q", ErrInvalid, next.ID, next.Status)
	}
	return ev, nil
}

// ApprovalAuditEvent returns the event that records the change from prev to next. A nil prev means
// next was just requested. Like GrantAuditEvent, the store calls it inside the write. Seq is left
// for the store. The time is the Approval's own.
func ApprovalAuditEvent(prev *Approval, next Approval) (AuditEvent, error) {
	ev := AuditEvent{TargetType: "approval", TargetID: next.ID, ProjectID: next.ProjectID}
	switch {
	case prev == nil:
		if next.State != ApprovalRequested {
			return AuditEvent{}, invalid("approval %s: a new approval is requested, not %s", next.ID, next.State)
		}
		ev.Kind, ev.Actor, ev.ActorKind, ev.At = AuditApprovalRequested, actorPolicyID, ActorPolicy, next.RequestedAt
		ev.Reason = fmt.Sprintf("%s needs approval; requested by %s %s", next.Gate.Capability, next.Requester.Kind, next.Requester.ID)
	case prev.State != ApprovalRequested || next.State == ApprovalRequested:
		return AuditEvent{}, invalid("approval %s: %s to %s is not a lifecycle change", next.ID, prev.State, next.State)
	case (next.State == ApprovalApproved || next.State == ApprovalRejected) && next.Decision != nil:
		d := next.Decision
		ev.Kind, ev.Actor, ev.ActorKind, ev.At, ev.Reason = AuditApprovalApproved, d.Approver, ActorHuman, d.DecidedAt, d.Rationale
		if next.State == ApprovalRejected {
			ev.Kind = AuditApprovalRejected
		}
	case next.State == ApprovalVoided && next.VoidedAt != nil:
		ev.Kind, ev.Actor, ev.ActorKind, ev.At, ev.Reason = AuditApprovalVoided, string(ActorPlatform), ActorPlatform, *next.VoidedAt, next.VoidReason
	default:
		return AuditEvent{}, invalid("approval %s: %s to %s is not recorded", next.ID, prev.State, next.State)
	}
	return ev, nil
}

// TrustAuditEvent returns the event that records the change from prev to next. A nil prev means
// there was no record before; a prev that is not revoked and a next that is not revoked is a
// renewal at a new hash. The store calls it inside the write. Seq is left for the store. The
// target is "project_trust" and its id is "<kind>" or "<kind>:<ref>".
func TrustAuditEvent(prev *Trust, next Trust) (AuditEvent, error) {
	id := string(next.Kind)
	if next.Ref != "" {
		id += ":" + next.Ref
	}
	ev := AuditEvent{TargetType: "project_trust", TargetID: id, ProjectID: string(next.ProjectKey), ActorKind: ActorHuman}
	switch {
	case next.RevokedAt != nil:
		if prev == nil || prev.RevokedAt != nil {
			return AuditEvent{}, invalid("trust %s %s: only a trust in force can be revoked", next.ProjectKey, id)
		}
		ev.Kind, ev.Actor, ev.At, ev.Reason = AuditTrustRevoked, next.RevokedBy, *next.RevokedAt, next.RevokeReason
	default:
		ev.Kind, ev.Actor, ev.At = AuditTrustGranted, next.TrustedBy, next.TrustedAt
		if next.Hash != "" {
			ev.Reason = "trusted " + next.Hash
		}
	}
	return ev, nil
}
