package domain

import (
	"fmt"
	"strings"
	"time"
)

// Role is a human's authority in the Organization (ADR-0038). The set is fixed.
type Role string

const (
	RoleOwner       Role = "owner"
	RoleAdmin       Role = "admin"
	RoleMaintainer  Role = "maintainer"
	RoleContributor Role = "contributor"
	RoleViewer      Role = "viewer"
)

// Valid reports whether r is one of the fixed roles.
func (r Role) Valid() bool {
	switch r {
	case RoleOwner, RoleAdmin, RoleMaintainer, RoleContributor, RoleViewer:
		return true
	}
	return false
}

// MayManageGrants reports whether the role may create or revoke a Grant: Owner and Admin only
// (ADR-0038, security baseline rule 12).
func (r Role) MayManageGrants() bool { return r == RoleOwner || r == RoleAdmin }

// GrantLevel is where a Grant applies.
type GrantLevel string

const (
	// GrantOrganization applies to every Project. The Organization is an implicit scope, so it
	// has no id.
	GrantOrganization GrantLevel = "organization"
	// GrantProject applies to one Project and never matches another.
	GrantProject GrantLevel = "project"
)

// SubjectKind is what a Grant authorizes.
type SubjectKind string

const (
	SubjectAgent           SubjectKind = "agent"
	SubjectWorkflow        SubjectKind = "workflow"
	SubjectPluginComponent SubjectKind = "plugin_component"
)

// Valid reports whether k is a known subject kind.
func (k SubjectKind) Valid() bool {
	switch k {
	case SubjectAgent, SubjectWorkflow, SubjectPluginComponent:
		return true
	}
	return false
}

// Subject names the definition on whose behalf a Capability is requested, and whom a Grant
// authorizes. A request is never authority (ADR-0037): the Subject only says what asked.
type Subject struct {
	Kind SubjectKind `json:"kind"`
	ID   string      `json:"id"`
}

// Resource is what an action touches. Type selects the matching rule (git_ref, host, path,
// secret, plugin, ...); Value is matched against a rule's or a Grant's pattern. The zero value
// means the request names no resource.
type Resource struct {
	Type  string `json:"type,omitempty"`
	Value string `json:"value,omitempty"`
}

// GrantStatus is the lifecycle of a Grant. A Grant is never deleted: revoking, lapsing and
// expiring keep the record for the audit trail.
type GrantStatus string

const (
	GrantActive  GrantStatus = "active"
	GrantLapsed  GrantStatus = "lapsed"  // its definition changed; it is not silently renewed
	GrantRevoked GrantStatus = "revoked" // an Owner or Admin withdrew it
	GrantExpired GrantStatus = "expired" // its expiry passed
)

// Limits on a Grant, so an untrusted client cannot store unbounded records.
const (
	MaxGrantCapabilities = 50
	MaxGrantPattern      = 512
	MaxGrantReason       = 1000
)

// Grant is a standing human decision that authorizes a Subject to use Capabilities on matching
// resources, many times (an Approval authorizes one occurrence). It is scoped to the Organization
// or to one Project. It is recorded, attributed to the human who made it, and revocable
// (FEAT-POL-001); a Project cannot ship its own (ADR-0055).
type Grant struct {
	ID string `json:"id"`
	// Version is the optimistic-concurrency counter of the stored row, set by the store.
	Version int64      `json:"version"`
	Level   GrantLevel `json:"level"`
	// ProjectID is set for a Project Grant and empty for an Organization Grant.
	ProjectID    string       `json:"projectId,omitempty"`
	Subject      Subject      `json:"subject"`
	Capabilities []Capability `json:"capabilities"`
	// Resource limits the Grant. An empty Type matches any resource; otherwise the request's
	// resource Type must equal it and its Value must match Pattern ("*" at the end is a prefix).
	Resource Resource `json:"resource"`
	// DefinitionHash, when set, binds the Grant to one definition: it lapses when the definition
	// the request carries has a different hash.
	DefinitionHash string `json:"definitionHash,omitempty"`
	// GrantedBy is the human Owner or Admin who made the Grant.
	GrantedBy string      `json:"grantedBy"`
	CreatedAt time.Time   `json:"createdAt"`
	ExpiresAt *time.Time  `json:"expiresAt,omitempty"`
	Status    GrantStatus `json:"status"`
	// RevokedBy, RevokedAt and RevokeReason are set by Revoke.
	RevokedBy    string     `json:"revokedBy,omitempty"`
	RevokedAt    *time.Time `json:"revokedAt,omitempty"`
	RevokeReason string     `json:"revokeReason,omitempty"`
}

// NewGrant validates a new Grant made by a human with a role that may make it, and returns it
// active. The caller supplies the id, the clock and the Grant's content; By must be a human
// principal, because Agents, triggers and plugins never grant (ADR-0036, ADR-0038).
func NewGrant(now time.Time, by Principal, g Grant) (Grant, error) {
	if by.Kind != PrincipalHuman || by.ID == "" {
		return Grant{}, fmt.Errorf("%w: only a human may make a grant, not %q", ErrNotPermitted, by.Kind)
	}
	if !by.Role.MayManageGrants() {
		return Grant{}, fmt.Errorf("%w: role %q may not make a grant; owner or admin only", ErrNotPermitted, by.Role)
	}
	g.GrantedBy, g.CreatedAt, g.Status, g.Version = by.ID, now, GrantActive, 0
	g.RevokedBy, g.RevokedAt, g.RevokeReason = "", nil, ""
	if err := g.Validate(); err != nil {
		return Grant{}, err
	}
	return g, nil
}

// Validate checks the shape of a Grant, including one read back from the store.
func (g Grant) Validate() error {
	switch {
	case g.ID == "":
		return invalid("grant id is required")
	case g.Level != GrantOrganization && g.Level != GrantProject:
		return invalid("grant %s: level %q is not organization or project", g.ID, g.Level)
	case g.Level == GrantProject && g.ProjectID == "":
		return invalid("grant %s: a project grant names its project", g.ID)
	case g.Level == GrantOrganization && g.ProjectID != "":
		return invalid("grant %s: an organization grant has no project", g.ID)
	case !g.Subject.Kind.Valid() || g.Subject.ID == "":
		return invalid("grant %s: subject needs a kind (agent, workflow, plugin_component) and an id", g.ID)
	case len(g.Capabilities) == 0 || len(g.Capabilities) > MaxGrantCapabilities:
		return invalid("grant %s: between 1 and %d capabilities", g.ID, MaxGrantCapabilities)
	case g.GrantedBy == "":
		return invalid("grant %s: grantedBy is required", g.ID)
	case g.CreatedAt.IsZero():
		return invalid("grant %s: createdAt is required", g.ID)
	case len(g.Resource.Value) > MaxGrantPattern || len(g.Resource.Type) > MaxGrantPattern:
		return invalid("grant %s: resource is longer than %d bytes", g.ID, MaxGrantPattern)
	case g.Resource.Type == "" && g.Resource.Value != "":
		return invalid("grant %s: a resource pattern needs a resource type", g.ID)
	case g.ExpiresAt != nil && !g.ExpiresAt.After(g.CreatedAt):
		return invalid("grant %s: expiry is not after creation", g.ID)
	}
	for _, c := range g.Capabilities {
		if !c.Valid() {
			return invalid("grant %s: capability %q is not a dotted lowercase identifier", g.ID, c)
		}
		if strings.Contains(string(c), "*") {
			return invalid("grant %s: capability %q: a grant names exact capabilities, no wildcards", g.ID, c)
		}
	}
	switch g.Status {
	case GrantActive, GrantLapsed, GrantRevoked, GrantExpired:
	default:
		return invalid("grant %s: status %q is unknown", g.ID, g.Status)
	}
	if (g.Status == GrantRevoked) != (g.RevokedBy != "" && g.RevokedAt != nil) {
		return invalid("grant %s: a revoked grant records who revoked it and when, and only then", g.ID)
	}
	return nil
}

// Revoke withdraws an active Grant. Any other state is refused: a Grant already out of use is
// not changed. By must be a human Owner or Admin and reason is required, so the audit trail says
// why.
func (g *Grant) Revoke(now time.Time, by Principal, reason string) error {
	if by.Kind != PrincipalHuman || by.ID == "" {
		return fmt.Errorf("%w: only a human may revoke a grant, not %q", ErrNotPermitted, by.Kind)
	}
	if !by.Role.MayManageGrants() {
		return fmt.Errorf("%w: role %q may not revoke a grant; owner or admin only", ErrNotPermitted, by.Role)
	}
	if strings.TrimSpace(reason) == "" {
		return ErrReasonRequired
	}
	if len(reason) > MaxGrantReason {
		return invalid("grant %s: revoke reason is longer than %d bytes", g.ID, MaxGrantReason)
	}
	if g.Status != GrantActive {
		return fmt.Errorf("%w: grant %s is %s", ErrInvalidTransition, g.ID, g.Status)
	}
	g.Status, g.RevokedBy, g.RevokedAt, g.RevokeReason = GrantRevoked, by.ID, &now, reason
	return nil
}

// Lapse marks an active Grant whose definition changed. It is not renewed: a human makes a new
// Grant for the new definition.
func (g *Grant) Lapse() error {
	if g.Status != GrantActive {
		return fmt.Errorf("%w: grant %s is %s", ErrInvalidTransition, g.ID, g.Status)
	}
	g.Status = GrantLapsed
	return nil
}

// Expire marks an active Grant whose expiry passed.
func (g *Grant) Expire() error {
	if g.Status != GrantActive {
		return fmt.Errorf("%w: grant %s is %s", ErrInvalidTransition, g.ID, g.Status)
	}
	g.Status = GrantExpired
	return nil
}

// Standing is the status a Grant has for a request: what is stored, except that an active Grant
// is expired once its expiry passed and lapsed once the request's definition hash differs. It
// does not change the Grant; the caller records the change with Lapse or Expire.
func (g Grant) Standing(now time.Time, definitionHash string) GrantStatus {
	switch {
	case g.Status != GrantActive:
		return g.Status
	case g.ExpiresAt != nil && !now.Before(*g.ExpiresAt):
		return GrantExpired
	case g.DefinitionHash != "" && g.DefinitionHash != definitionHash:
		return GrantLapsed
	}
	return GrantActive
}

// covers reports whether the Grant authorizes the request, ignoring its standing.
func (g Grant) covers(r PolicyRequest) bool {
	if g.Level == GrantProject && g.ProjectID != r.Scope.ProjectID {
		return false
	}
	if g.Subject != r.Subject {
		return false
	}
	found := false
	for _, c := range g.Capabilities {
		if c == r.Capability {
			found = true
			break
		}
	}
	if !found {
		return false
	}
	if g.Resource.Type == "" {
		return true
	}
	return g.Resource.Type == r.Resource.Type && matchPattern(g.Resource.Value, r.Resource.Value)
}

// matchPattern matches value against pattern: "*" matches anything, a trailing "*" is a prefix,
// and anything else must be equal. An empty pattern matches only an empty value.
func matchPattern(pattern, value string) bool {
	if pattern == "*" {
		return true
	}
	if prefix, ok := strings.CutSuffix(pattern, "*"); ok {
		return strings.HasPrefix(value, prefix)
	}
	return pattern == value
}
