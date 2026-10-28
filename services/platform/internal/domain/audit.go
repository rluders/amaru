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
)

// Who made a change. A person's change records the person's id; a change nobody asked for, such
// as a Grant lapsing or expiring, is the Platform's.
const (
	// ActorPlatform is the actor of a change nobody asked for, and its id.
	ActorPlatform ActorKind = "platform"
)

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
