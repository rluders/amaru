package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// MaxSkillBody bounds a Skill body in bytes, so an untrusted client cannot store unbounded text.
const MaxSkillBody = 256 << 10

// SkillScope is where a Skill lives (ADR-0039): the Organization library or one Project.
type SkillScope string

const (
	SkillOrganization SkillScope = "organization"
	SkillProject      SkillScope = "project"
)

// Skill is procedural knowledge an Agent is given, immutable per name and version. Its Hash is
// computed by the Platform from the Body, never taken from a caller (FEAT-SKL-001). An
// Organization Skill is put there by a human with authority and is trusted by that; a Project
// Skill comes from the Project, which is untrusted content, so it needs a Trust of kind skill
// at its exact hash before any run may use it (FEAT-PRJ-003).
type Skill struct {
	Scope SkillScope `json:"scope"`
	// ProjectKey is set for a Project Skill and empty for an Organization Skill.
	ProjectKey ProjectKey `json:"projectKey,omitempty"`
	Name       string     `json:"name"`
	Version    string     `json:"version"`
	Body       string     `json:"body"`
	// Hash is HashBody(Body): 64 lowercase hex digits.
	Hash    string    `json:"hash"`
	AddedBy string    `json:"addedBy"`
	AddedAt time.Time `json:"addedAt"`
}

// NewSkill validates a Skill added by a human Maintainer, Admin or Owner and computes its hash.
func NewSkill(now time.Time, by Principal, scope SkillScope, project ProjectKey, name, version, body string) (Skill, error) {
	if by.Kind != PrincipalHuman || by.ID == "" {
		return Skill{}, fmt.Errorf("%w: only a human may add a skill, not %q", ErrNotPermitted, by.Kind)
	}
	if !by.Role.MayTrustProjects() {
		return Skill{}, fmt.Errorf("%w: role %q may not add a skill; maintainer, admin or owner only", ErrNotPermitted, by.Role)
	}
	s := Skill{Scope: scope, ProjectKey: project, Name: name, Version: version, Body: body, Hash: HashBody(body), AddedBy: by.ID, AddedAt: now}
	if err := s.Validate(); err != nil {
		return Skill{}, err
	}
	return s, nil
}

// Validate checks the shape of a Skill, including one read back from the store.
func (s Skill) Validate() error {
	switch {
	case !ValidAgentID(s.Name):
		return invalid("skill name %q must be a slug", s.Name)
	case s.Version == "" || len(s.Version) > 64 || strings.TrimSpace(s.Version) != s.Version:
		return invalid("skill %s: version must be 1 to 64 characters without surrounding space", s.Name)
	case s.Scope != SkillOrganization && s.Scope != SkillProject:
		return invalid("skill %s: scope %q is not organization or project", s.Name, s.Scope)
	case s.Scope == SkillProject && !s.ProjectKey.Valid():
		return invalid("skill %s: a project skill names its project by a valid key", s.Name)
	case s.Scope == SkillOrganization && s.ProjectKey != "":
		return invalid("skill %s: an organization skill has no project", s.Name)
	case s.Body == "" || len(s.Body) > MaxSkillBody || !utf8.ValidString(s.Body):
		return invalid("skill %s: body must be 1 to %d bytes of valid UTF-8", s.Name, MaxSkillBody)
	case s.Hash != HashBody(s.Body):
		return invalid("skill %s: hash is not the hash of the body", s.Name)
	case s.AddedBy == "" || s.AddedAt.IsZero():
		return invalid("skill %s: addedBy and addedAt are required", s.Name)
	}
	return nil
}

// SkillRef is how a run names a Skill: the name, the version and the hash it expects. The hash
// pins the content, so a Skill whose body changed under the same version is refused, not run.
type SkillRef struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Hash    string `json:"hash"`
}

// Validate checks the shape of a reference.
func (r SkillRef) Validate() error {
	switch {
	case !ValidAgentID(r.Name):
		return invalid("skill name %q must be a slug", r.Name)
	case r.Version == "":
		return invalid("skill %s: version is required", r.Name)
	case !sha256HexRe.MatchString(r.Hash):
		return invalid("skill %s: hash must be 64 lowercase hex digits", r.Name)
	}
	return nil
}

// Codes of a Skill refusal, in the Step's Reason.
const (
	ReasonSkillInvalid      = "skill.invalid"
	ReasonSkillUnknown      = "skill.unknown"
	ReasonSkillHashMismatch = "skill.hash_mismatch"
	ReasonSkillUntrusted    = "skill.untrusted"
)

// SkillError says why a Skill reference may not be used. Code is one of the ReasonSkill* values.
type SkillError struct {
	Code    string
	Message string
}

func (e *SkillError) Error() string { return e.Code + ": " + e.Message }

// IsSkillError reports whether err is a SkillError and returns it.
func IsSkillError(err error) (*SkillError, bool) {
	var se *SkillError
	return se, errors.As(err, &se)
}

// SkillTrustHash is the content hash a Trust of kind skill holds for the Skill: the form every
// Trust record uses, "sha256:" and the hex hash of the body.
func SkillTrustHash(s Skill) string { return "sha256:" + s.Hash }

// ResolveSkill decides whether ref may be used and returns the audit record of the use. found is
// the Skill the reference names, or nil; trust is the Trust of kind skill for that name in the
// Project, or nil. It is pure: the caller loads both. The order is fixed and every refusal is a
// *SkillError: a reference of a wrong shape, an unknown Skill, a hash that is not the Skill's, and
// last a Project Skill with no live Trust at its hash.
func ResolveSkill(ref SkillRef, found *Skill, trust *Trust) (SkillUse, error) {
	if err := ref.Validate(); err != nil {
		return SkillUse{}, &SkillError{ReasonSkillInvalid, err.Error()}
	}
	if found == nil {
		return SkillUse{}, &SkillError{ReasonSkillUnknown, fmt.Sprintf("skill %s %s does not exist", ref.Name, ref.Version)}
	}
	if found.Hash != ref.Hash {
		return SkillUse{}, &SkillError{ReasonSkillHashMismatch, fmt.Sprintf("skill %s %s has hash %s, not the pinned %s", ref.Name, ref.Version, found.Hash, ref.Hash)}
	}
	if found.Scope == SkillProject && (trust == nil || trust.Kind != TrustSkill || trust.RevokedAt != nil || trust.Hash != SkillTrustHash(*found)) {
		return SkillUse{}, &SkillError{ReasonSkillUntrusted, fmt.Sprintf("project skill %s %s is not trusted at this content; a maintainer, admin or owner can trust it", ref.Name, ref.Version)}
	}
	return SkillUse{Name: found.Name, Version: found.Version, Hash: found.Hash}, nil
}
