package domain

import (
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// AgentStatus is set by people. Only an active Agent can be selected by a run.
type AgentStatus string

const (
	AgentActive   AgentStatus = "active"
	AgentDisabled AgentStatus = "disabled"
)

func (s AgentStatus) Valid() bool { return s == AgentActive || s == AgentDisabled }

// Limits mirror packages/contracts/schemas/Agent.json.
const (
	MaxAgentRole          = 120
	MaxAgentInstructions  = 16000
	MaxAgentModel         = 80
	MaxAgentCapabilities  = 20
	MaxProviderPreference = 10
	MaxAgentModels        = 16
	MaxWorktreeCommands   = 32
)

var (
	agentIDPattern  = regexp.MustCompile(`^[a-z][a-z0-9-]{1,62}[a-z0-9]$`)
	providerIDRe    = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,62}$`)
	agentModelRe    = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/\[\]-]*$`)
	nonSlugRe       = regexp.MustCompile(`[^a-z0-9]+`)
	worktreeCmdRe   = regexp.MustCompile(`^[a-z][a-z0-9._+-]{0,31}( -{0,2}[A-Za-z0-9][A-Za-z0-9._+=/:-]{0,63}){0,3}$`)
	deniedWorktreeC = map[string]bool{
		"git": true, "gh": true, "curl": true, "wget": true, "ssh": true, "scp": true, "rsync": true, "nc": true,
		"sh": true, "bash": true, "zsh": true, "fish": true, "dash": true, "env": true, "sudo": true, "su": true,
		"eval": true, "exec": true, "xargs": true, "python": true, "python3": true, "node": true, "npx": true,
		"perl": true, "ruby": true, "docker": true, "podman": true, "kubectl": true, "rm": true, "chmod": true,
		"chown": true, "dd": true, "tee": true, "open": true, "xdg-open": true, "flatpak": true,
	}
)

// Agent is a reusable participant definition: persona, preferred providers and model. It is not a
// Capability, a Provider or a Team. Its fields are those of the Agent contract in
// packages/contracts; Skills, tool scope and Team membership arrive with their own features.
type Agent struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Role         string `json:"role,omitempty"`
	Instructions string `json:"instructions"`
	// RequestedCapabilities is a declaration only: nothing enforces or grants it.
	RequestedCapabilities []Capability `json:"requestedCapabilities"`
	// ProviderPreference is an ordered list of agent Provider ids; empty means any available.
	ProviderPreference []string `json:"providerPreference"`
	Model              string   `json:"model,omitempty"` // default; opaque, the Provider decides if it is valid
	// Models overrides Model per agent Provider id: one string cannot fit both "gpt-5" and
	// "anthropic/claude-sonnet-5-5".
	Models map[string]string `json:"models,omitempty"`
	// WorktreeWrite lets the Agent edit files when, and only when, its Step works in a
	// Platform-managed Worktree. It never widens what it may do elsewhere.
	WorktreeWrite bool `json:"worktreeWrite,omitempty"`
	// WorktreeCommands are shell command prefixes (for example "go test") the Agent may run in its
	// Worktree. Only with WorktreeWrite; only through a Provider that confines the shell.
	WorktreeCommands []string    `json:"worktreeCommands,omitempty"`
	Status           AgentStatus `json:"status"`
	CreatedAt        time.Time   `json:"createdAt"`
	UpdatedAt        time.Time   `json:"updatedAt"`
}

// ValidAgentID reports whether id is a well-formed Agent slug.
func ValidAgentID(id string) bool { return agentIDPattern.MatchString(id) }

// ValidModel reports whether m is safe to hand to a Provider (also before it reaches argv).
func ValidModel(m string) bool { return len(m) <= MaxAgentModel && agentModelRe.MatchString(m) }

// SlugFromName derives an Agent id from a display name; "" when nothing usable remains.
func SlugFromName(name string) string {
	s := strings.Trim(nonSlugRe.ReplaceAllString(strings.ToLower(name), "-"), "-")
	if len(s) > 64 {
		s = strings.Trim(s[:64], "-")
	}
	return s
}

// ModelFor is the model to hand to providerID: its entry in Models, else Model, else "" (the
// Provider's own default).
func (a Agent) ModelFor(providerID string) string {
	if m := a.Models[providerID]; m != "" {
		return m
	}
	return a.Model
}

// ValidWorktreeCommand checks one command prefix: up to four plain words (no shell syntax), and not
// a program that can reach the network, the user's credentials or an arbitrary interpreter, or
// rewrite history. Delivery (commit, push, pull request) is the Platform's job.
func ValidWorktreeCommand(c string) error {
	if !worktreeCmdRe.MatchString(c) {
		return invalid("worktree command %q must be up to four plain lowercase words (no shell syntax)", c)
	}
	if prog, _, _ := strings.Cut(c, " "); deniedWorktreeC[prog] {
		return invalid("worktree command %q is not allowed: %s is reserved to the Platform or can leave the sandbox", c, prog)
	}
	return nil
}

// Validate checks the invariants a stored or incoming Agent must hold.
func (a Agent) Validate() error {
	if !ValidAgentID(a.ID) {
		return invalid("invalid agent id")
	}
	if strings.TrimSpace(a.Name) == "" || utf8.RuneCountInString(a.Name) > MaxTitle {
		return invalid("name is required and at most %d characters", MaxTitle)
	}
	if utf8.RuneCountInString(a.Role) > MaxAgentRole {
		return invalid("role is longer than %d characters", MaxAgentRole)
	}
	if strings.TrimSpace(a.Instructions) == "" || utf8.RuneCountInString(a.Instructions) > MaxAgentInstructions {
		return invalid("instructions are required and at most %d characters", MaxAgentInstructions)
	}
	if !a.Status.Valid() {
		return invalid("invalid status")
	}
	if a.Model != "" && !ValidModel(a.Model) {
		return invalid("invalid model")
	}
	if len(a.Models) > MaxAgentModels {
		return invalid("more than %d per-provider models", MaxAgentModels)
	}
	for id, m := range a.Models {
		if !providerIDRe.MatchString(id) || m == "" || !ValidModel(m) {
			return invalid("models must map an agent provider id to a valid model")
		}
	}
	if len(a.WorktreeCommands) > 0 && !a.WorktreeWrite {
		return invalid("worktreeCommands need worktreeWrite")
	}
	if len(a.WorktreeCommands) > MaxWorktreeCommands {
		return invalid("more than %d worktree commands", MaxWorktreeCommands)
	}
	for _, c := range a.WorktreeCommands {
		if err := ValidWorktreeCommand(c); err != nil {
			return err
		}
	}
	if len(a.RequestedCapabilities) > MaxAgentCapabilities {
		return invalid("more than %d requested capabilities", MaxAgentCapabilities)
	}
	caps := map[Capability]bool{}
	for _, c := range a.RequestedCapabilities {
		if !c.Valid() || caps[c] {
			return invalid("requested capabilities must be unique, valid capabilities")
		}
		caps[c] = true
	}
	if len(a.ProviderPreference) > MaxProviderPreference {
		return invalid("more than %d provider preferences", MaxProviderPreference)
	}
	prefs := map[string]bool{}
	for _, p := range a.ProviderPreference {
		if !providerIDRe.MatchString(p) || prefs[p] {
			return invalid("provider preference must be unique, well-formed provider ids")
		}
		prefs[p] = true
	}
	return nil
}

// clone returns a deep copy with nil lists normalised to empty, so a stored Agent never aliases a
// caller's slices or map.
func (a Agent) clone() Agent {
	a.RequestedCapabilities = append([]Capability{}, a.RequestedCapabilities...)
	a.ProviderPreference = append([]string{}, a.ProviderPreference...)
	if a.Models != nil {
		m := make(map[string]string, len(a.Models))
		for k, v := range a.Models {
			m[k] = v
		}
		a.Models = m
	}
	if a.WorktreeCommands != nil {
		a.WorktreeCommands = append([]string{}, a.WorktreeCommands...)
	}
	return a
}

// AgentPatch is an edit to an Agent, shaped like the UpdateAgent contract: a field that is set
// replaces the stored value, and a list replaces the whole list. The id is not editable. Model ""
// clears the model.
type AgentPatch struct {
	Name                  *string            `json:"name,omitempty"`
	Role                  *string            `json:"role,omitempty"`
	Instructions          *string            `json:"instructions,omitempty"`
	RequestedCapabilities *[]Capability      `json:"requestedCapabilities,omitempty"`
	ProviderPreference    *[]string          `json:"providerPreference,omitempty"`
	Model                 *string            `json:"model,omitempty"`
	Models                *map[string]string `json:"models,omitempty"`
	WorktreeWrite         *bool              `json:"worktreeWrite,omitempty"`
	WorktreeCommands      *[]string          `json:"worktreeCommands,omitempty"`
	Status                *AgentStatus       `json:"status,omitempty"`
}

// Empty reports whether the patch sets nothing.
func (p AgentPatch) Empty() bool {
	return p.Name == nil && p.Role == nil && p.Instructions == nil && p.RequestedCapabilities == nil &&
		p.ProviderPreference == nil && p.Model == nil && p.Models == nil && p.WorktreeWrite == nil &&
		p.WorktreeCommands == nil && p.Status == nil
}

// Apply returns a with the patch applied. It does not validate or touch timestamps.
func (p AgentPatch) Apply(a Agent) Agent {
	a = a.clone()
	if p.Name != nil {
		a.Name = *p.Name
	}
	if p.Role != nil {
		a.Role = *p.Role
	}
	if p.Instructions != nil {
		a.Instructions = *p.Instructions
	}
	if p.RequestedCapabilities != nil {
		a.RequestedCapabilities = append([]Capability{}, *p.RequestedCapabilities...)
	}
	if p.ProviderPreference != nil {
		a.ProviderPreference = append([]string{}, *p.ProviderPreference...)
	}
	if p.Model != nil {
		a.Model = *p.Model
	}
	if p.Models != nil {
		a.Models = (Agent{Models: *p.Models}).clone().Models
	}
	if p.WorktreeWrite != nil {
		a.WorktreeWrite = *p.WorktreeWrite
	}
	if p.WorktreeCommands != nil {
		a.WorktreeCommands = append([]string{}, *p.WorktreeCommands...)
	}
	if p.Status != nil {
		a.Status = *p.Status
	}
	return a
}
