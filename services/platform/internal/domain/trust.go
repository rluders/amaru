package domain

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// TrustKind is what a Trust record covers (FEAT-PRJ-003, ADR-0055 item 3).
type TrustKind string

const (
	// TrustProject trusts the Project itself: its identity, which ProjectKey ties to one
	// repository by its root commit. Nothing project-supplied runs without it.
	TrustProject TrustKind = "project"
	// TrustWorkflow trusts one Workflow definition by the hash of its content.
	TrustWorkflow TrustKind = "workflow"
	// TrustSkill trusts one Skill by the hash of its content.
	TrustSkill TrustKind = "skill"
)

// MaxTrustRef bounds the Ref of a Trust record.
const MaxTrustRef = 200

// trustHashPattern is the content hash form DefinitionPin uses.
var trustHashPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

// MayTrustProjects reports whether the role may trust or untrust Project content: Maintainer,
// Admin and Owner (ADR-0038). An Agent, trigger or plugin never does, whatever role it names.
func (r Role) MayTrustProjects() bool {
	return r == RoleOwner || r == RoleAdmin || r == RoleMaintainer
}

// Trust is a human decision that one Project, or one exact content of it, may be used. It is
// bound to content: a Workflow or Skill record holds the hash it was trusted at, and a different
// hash is not trusted until a human trusts it again. Project-supplied content cannot trust
// itself: only the Platform API writes a Trust, for a human (ADR-0055, rule 12).
type Trust struct {
	ProjectKey ProjectKey `json:"projectKey"`
	Kind       TrustKind  `json:"kind"`
	// Ref names the content: empty for the Project, the Workflow id or the Skill id otherwise.
	Ref string `json:"ref,omitempty"`
	// Hash is "sha256:<hex>" of the trusted content; empty for the Project.
	Hash      string    `json:"hash,omitempty"`
	TrustedBy string    `json:"trustedBy"`
	TrustedAt time.Time `json:"trustedAt"`
	// RevokedBy, RevokedAt and RevokeReason are set by Revoke. A revoked record stays, so the
	// store keeps what was trusted before.
	RevokedBy    string     `json:"revokedBy,omitempty"`
	RevokedAt    *time.Time `json:"revokedAt,omitempty"`
	RevokeReason string     `json:"revokeReason,omitempty"`
}

// NewTrust validates a new Trust made by a human who may make it. Trusting content again with a
// new hash is a new Trust for the same kind and ref: it replaces the old one.
func NewTrust(now time.Time, by Principal, key ProjectKey, kind TrustKind, ref, hash string) (Trust, error) {
	if by.Kind != PrincipalHuman || by.ID == "" {
		return Trust{}, fmt.Errorf("%w: only a human may trust project content, not %q", ErrNotPermitted, by.Kind)
	}
	if !by.Role.MayTrustProjects() {
		return Trust{}, fmt.Errorf("%w: role %q may not trust project content; maintainer, admin or owner only", ErrNotPermitted, by.Role)
	}
	t := Trust{ProjectKey: key, Kind: kind, Ref: ref, Hash: hash, TrustedBy: by.ID, TrustedAt: now}
	if err := t.Validate(); err != nil {
		return Trust{}, err
	}
	return t, nil
}

// Validate checks the shape of a Trust, including one read back from the store.
func (t Trust) Validate() error {
	if !t.ProjectKey.Valid() {
		return invalid("trust: project key %q is not valid", t.ProjectKey)
	}
	switch t.Kind {
	case TrustProject:
		if t.Ref != "" || t.Hash != "" {
			return invalid("trust %s: a project trust has no ref and no hash", t.ProjectKey)
		}
	case TrustWorkflow, TrustSkill:
		if t.Ref == "" || len(t.Ref) > MaxTrustRef || strings.TrimSpace(t.Ref) != t.Ref {
			return invalid("trust %s: a %s trust names its content in 1 to %d characters", t.ProjectKey, t.Kind, MaxTrustRef)
		}
		if !trustHashPattern.MatchString(t.Hash) {
			return invalid("trust %s: a %s trust binds a content hash (sha256:<hex>)", t.ProjectKey, t.Kind)
		}
	default:
		return invalid("trust %s: kind %q is not project, workflow or skill", t.ProjectKey, t.Kind)
	}
	switch {
	case t.TrustedBy == "":
		return invalid("trust %s: trustedBy is required", t.ProjectKey)
	case t.TrustedAt.IsZero():
		return invalid("trust %s: trustedAt is required", t.ProjectKey)
	case (t.RevokedBy != "") != (t.RevokedAt != nil):
		return invalid("trust %s: a revoked trust records who revoked it and when, and only then", t.ProjectKey)
	}
	return nil
}

// Revoke withdraws the Trust. By must be a human who may trust, and reason is required so the
// audit trail says why. A trust already revoked is not revoked again.
func (t *Trust) Revoke(now time.Time, by Principal, reason string) error {
	if by.Kind != PrincipalHuman || by.ID == "" {
		return fmt.Errorf("%w: only a human may revoke trust, not %q", ErrNotPermitted, by.Kind)
	}
	if !by.Role.MayTrustProjects() {
		return fmt.Errorf("%w: role %q may not revoke trust; maintainer, admin or owner only", ErrNotPermitted, by.Role)
	}
	if strings.TrimSpace(reason) == "" {
		return ErrReasonRequired
	}
	if len(reason) > MaxGrantReason {
		return invalid("trust %s: revoke reason is longer than %d bytes", t.ProjectKey, MaxGrantReason)
	}
	if t.RevokedAt != nil {
		return fmt.Errorf("%w: trust %s %s is revoked", ErrInvalidTransition, t.ProjectKey, t.Kind)
	}
	t.RevokedBy, t.RevokedAt, t.RevokeReason = by.ID, &now, reason
	return nil
}

// TrustStanding is what Policy knows about trust for a request. The zero value is untrusted, so
// a caller that forgets to look is refused, not allowed.
type TrustStanding string

const (
	// TrustTrusted: the Project is trusted and so is the exact content the request runs.
	TrustTrusted TrustStanding = "trusted"
	// TrustUntrusted: the Project is not trusted, or its trust was revoked.
	TrustUntrusted TrustStanding = "untrusted"
	// TrustChanged: the Project is trusted but the content the request runs is not the content
	// that was trusted. It needs a new trust.
	TrustChanged TrustStanding = "changed"
)

// Standing returns the standing of the content a request runs. project is the Project's own
// Trust (zero when none), content the Trust of the Workflow or Skill the request runs and hash
// that content's current hash. A request that runs no named content (hash empty) depends on the
// Project trust alone.
func Standing(project Trust, content *Trust, hash string) TrustStanding {
	if project.Kind != TrustProject || project.RevokedAt != nil {
		return TrustUntrusted
	}
	if hash == "" {
		return TrustTrusted
	}
	if content == nil || content.RevokedAt != nil || content.Hash != hash {
		return TrustChanged
	}
	return TrustTrusted
}
