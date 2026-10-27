package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"slices"
)

// MaxStepSkills mirrors packages/contracts/schemas/ResolvedAgent.json.
const MaxStepSkills = 64

// Resolution says how the Provider of a Step was chosen (FEAT-AGT-002 decides it; this records it).
type Resolution string

const (
	ResolutionPreference Resolution = "preference" // a providerPreference entry
	ResolutionFallback   Resolution = "fallback"   // preference set but unmet, first available used
	ResolutionDefault    Resolution = "default"    // no preference, first available used
)

func (r Resolution) Valid() bool {
	return r == ResolutionPreference || r == ResolutionFallback || r == ResolutionDefault
}

// ToolIsolation says whether a run saw only the tools the Platform provisioned.
type ToolIsolation string

const (
	// IsolationStrict: the Provider's own MCP servers, connectors and native skills were shut out.
	IsolationStrict ToolIsolation = "strict"
	// IsolationAmbient: it also saw whatever the user configured for that CLI.
	IsolationAmbient ToolIsolation = "ambient"
)

func (t ToolIsolation) Valid() bool { return t == "" || t == IsolationStrict || t == IsolationAmbient }

var sha256HexRe = regexp.MustCompile(`^[0-9a-f]{64}$`)

// SkillUse is the audit record of a Skill injected into a Step's context. Name, version and
// content hash identify what ran without storing Skill history. Trust, indexing and pin
// enforcement belong to FEAT-SKL-001.
type SkillUse struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Hash    string `json:"hash"` // HashBody of the Skill body
}

// HashBody is the content hash recorded for a Skill body.
func HashBody(body string) string {
	sum := sha256.Sum256([]byte(body))
	return hex.EncodeToString(sum[:])
}

func (u SkillUse) validate() error {
	switch {
	case !ValidAgentID(u.Name):
		return invalid("skill name %q must be a slug", u.Name)
	case u.Version == "":
		return invalid("skill %s: version is required", u.Name)
	case !sha256HexRe.MatchString(u.Hash):
		return invalid("skill %s: hash must be 64 lowercase hex digits", u.Name)
	}
	return nil
}

// ResolvedAgent is the snapshot of the Agent a Step ran with: a full copy, so the run stays
// reconstructable after the Agent changes. Revision and Hash name the exact AgentVersion it came
// from; (ID, Revision) is unambiguous because history is append-only.
type ResolvedAgent struct {
	ID                 string        `json:"id"`
	Name               string        `json:"name"`
	Role               string        `json:"role,omitempty"`
	Instructions       string        `json:"instructions"`
	Model              string        `json:"model,omitempty"`
	ProviderPreference []string      `json:"providerPreference"`
	ProviderID         string        `json:"providerId"`
	Resolution         Resolution    `json:"resolution"`
	Revision           int           `json:"revision"`
	Hash               string        `json:"hash"`
	Skills             []SkillUse    `json:"skills,omitempty"` // injection order
	ToolIsolation      ToolIsolation `json:"toolIsolation,omitempty"`
	WorktreeWrite      bool          `json:"worktreeWrite,omitempty"`
	WorktreeCommands   []string      `json:"worktreeCommands,omitempty"`
}

// Resolve snapshots the Agent version v as run by providerID with the given Skills. Revision and
// Hash come from the recorded version, never from the caller. A deleted version cannot be resolved.
func (v AgentVersion) Resolve(providerID string, res Resolution, isolation ToolIsolation, skills []SkillUse) (ResolvedAgent, error) {
	switch {
	case v.Deleted:
		return ResolvedAgent{}, fmt.Errorf("%w: agent %s revision %d is a tombstone", ErrDeleted, v.Definition.ID, v.Revision)
	case !providerIDRe.MatchString(providerID):
		return ResolvedAgent{}, invalid("provider id %q is not valid", providerID)
	case !res.Valid():
		return ResolvedAgent{}, invalid("resolution %q is not valid", res)
	case !isolation.Valid():
		return ResolvedAgent{}, invalid("tool isolation %q is not valid", isolation)
	case len(skills) > MaxStepSkills:
		return ResolvedAgent{}, invalid("at most %d skills per step", MaxStepSkills)
	}
	seen := map[string]bool{}
	for _, s := range skills {
		if err := s.validate(); err != nil {
			return ResolvedAgent{}, err
		}
		if seen[s.Name] {
			return ResolvedAgent{}, invalid("skill %s listed twice", s.Name)
		}
		seen[s.Name] = true
	}
	a := v.Definition.clone()
	return ResolvedAgent{
		ID: a.ID, Name: a.Name, Role: a.Role, Instructions: a.Instructions, Model: a.ModelFor(providerID),
		ProviderPreference: a.ProviderPreference, ProviderID: providerID, Resolution: res,
		Revision: v.Revision, Hash: v.Hash, Skills: slices.Clone(skills), ToolIsolation: isolation,
		WorktreeWrite: a.WorktreeWrite, WorktreeCommands: a.WorktreeCommands,
	}, nil
}
