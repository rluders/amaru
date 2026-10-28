package domain

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// WorkflowStatus is set by people. Only an active Workflow can be run.
type WorkflowStatus string

const (
	WorkflowDraft    WorkflowStatus = "draft"
	WorkflowActive   WorkflowStatus = "active"
	WorkflowArchived WorkflowStatus = "archived"
)

func (s WorkflowStatus) Valid() bool {
	switch s {
	case WorkflowDraft, WorkflowActive, WorkflowArchived:
		return true
	}
	return false
}

// WorkflowStepType says what a Workflow step is. It is the definition's word; the running
// StepExecution has its own lifecycle (step.go).
type WorkflowStepType string

const (
	StepTypeAgent WorkflowStepType = "agent"
	StepTypeTool  WorkflowStepType = "tool"
	// StepTypeSystem is a Step the Platform itself runs through a Capability.
	StepTypeSystem WorkflowStepType = "system"
	// StepTypeReview runs a reviewer Agent from a Team role over another step's output.
	StepTypeReview WorkflowStepType = "review"
	// StepTypeApproval pauses the Execution until a person decides.
	StepTypeApproval WorkflowStepType = "approval"
)

func (t WorkflowStepType) Valid() bool {
	switch t {
	case StepTypeAgent, StepTypeTool, StepTypeSystem, StepTypeReview, StepTypeApproval:
		return true
	}
	return false
}

// Workflow limits (FEAT-WFL-001: steps, edges, prompt size). MaxVarValue is at least MaxIntent so
// a Work Item intent travels whole (F-15), with room for a CI log tail in feedback.
const (
	MaxWorkflowSteps = 50
	MaxWorkflowEdges = 200
	MaxWorkflowVars  = 50
	MaxStepPrompt    = 8000
	MaxVarValue      = 10000
	MaxStepInputs    = 20
)

var (
	workflowIDPattern     = regexp.MustCompile(`^[a-z][a-z0-9-]{1,62}[a-z0-9]$`)
	workflowStepIDPattern = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,63}$`)
	stepInputKey          = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	teamRolePattern       = regexp.MustCompile(`^[a-z][a-z0-9-]{0,30}[a-z0-9]$`)
)

// ValidWorkflowID reports whether id is a lowercase slug. The id names the Workflow file under
// .amaru/, so it carries no path separator or dot.
func ValidWorkflowID(id string) bool { return workflowIDPattern.MatchString(id) }

// WorkflowStep expresses WHAT happens. Providers decide HOW and are never referenced.
type WorkflowStep struct {
	ID     string           `json:"id"`
	Name   string           `json:"name"`
	Type   WorkflowStepType `json:"type"`
	Prompt string           `json:"prompt,omitempty"`
	// Capability is the semantic operation a tool or system step asks for (e.g. "code.test"). It
	// names WHAT, never a provider or an MCP server.
	Capability Capability `json:"capability,omitempty"`
	// Input is explicit, named input for the Capability. Values may use {{variable}} substitution
	// (FEAT-WFL-003); this type only bounds them.
	Input map[string]string `json:"input,omitempty"`
	// AgentID selects the Agent that runs an agent step. Existence and status are checked when an
	// Execution is planned, not when the Workflow is saved.
	AgentID string `json:"agentId,omitempty"`
	// TimeoutSeconds overrides the default wall-clock timeout of the Step (FEAT-WFL-004); zero
	// means the default for its kind. The Organization maximum is checked when an Execution is
	// submitted, because it is the Platform's setting, not the definition's.
	TimeoutSeconds int `json:"timeoutSeconds,omitempty"`
	// TeamID and Role select the Agent through a Team instead of AgentID. On a review step they
	// name the reviewer.
	TeamID string `json:"teamId,omitempty"`
	Role   string `json:"role,omitempty"`
	// Reviews is the step whose output a review or approval step looks at.
	Reviews string `json:"reviews,omitempty"`
}

type WorkflowVariable struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// WorkflowEdge orders two steps (From runs before To). With FromPort and ToPort set it is also a
// data edge: when To starts, its input ToPort is the output of From with role FromPort.
type WorkflowEdge struct {
	From     string `json:"from"`
	FromPort string `json:"fromPort,omitempty"`
	To       string `json:"to"`
	ToPort   string `json:"toPort,omitempty"`
}

// IsData reports whether the edge carries a value from one step to another.
func (e WorkflowEdge) IsData() bool { return e.FromPort != "" }

// Workflow is a reviewable graph of steps. Without edges the steps run in list order.
type Workflow struct {
	ID          string             `json:"id"`
	Name        string             `json:"name"`
	Description string             `json:"description,omitempty"`
	Status      WorkflowStatus     `json:"status"`
	Steps       []WorkflowStep     `json:"steps"`
	Edges       []WorkflowEdge     `json:"edges,omitempty"`
	Variables   []WorkflowVariable `json:"variables,omitempty"`
}

// Validate checks the invariants a stored or incoming Workflow must hold, including the limits.
func (w Workflow) Validate() error {
	if !ValidWorkflowID(w.ID) {
		return invalid("workflow id must be a lowercase slug of 3 to 64 characters")
	}
	if strings.TrimSpace(w.Name) == "" || utf8.RuneCountInString(w.Name) > MaxTitle {
		return invalid("workflow name is required and at most %d characters", MaxTitle)
	}
	if utf8.RuneCountInString(w.Description) > MaxSummary {
		return invalid("workflow description is at most %d characters", MaxSummary)
	}
	if !w.Status.Valid() {
		return invalid("invalid workflow status %q", w.Status)
	}
	if len(w.Steps) == 0 {
		return invalid("a workflow needs at least one step")
	}
	if len(w.Steps) > MaxWorkflowSteps {
		return invalid("too many steps (at most %d)", MaxWorkflowSteps)
	}
	ids := make(map[string]bool, len(w.Steps))
	for _, s := range w.Steps {
		if !workflowStepIDPattern.MatchString(s.ID) {
			return invalid("step id %q must be a lowercase slug", s.ID)
		}
		if ids[s.ID] {
			return invalid("step id %q is not unique", s.ID)
		}
		ids[s.ID] = true
	}
	for _, s := range w.Steps {
		if err := s.validate(ids); err != nil {
			return fmt.Errorf("step %s: %w", s.ID, err)
		}
	}
	if err := w.validateEdges(ids); err != nil {
		return err
	}
	return w.validateVariables()
}

func (s WorkflowStep) validate(ids map[string]bool) error {
	if strings.TrimSpace(s.Name) == "" || utf8.RuneCountInString(s.Name) > MaxTitle {
		return invalid("name is required and at most %d characters", MaxTitle)
	}
	if !s.Type.Valid() {
		return invalid("invalid type %q", s.Type)
	}
	if s.TimeoutSeconds < 0 || (s.TimeoutSeconds > 0 && s.Type == StepTypeApproval) {
		return invalid("timeoutSeconds must not be negative and an approval step waits without one")
	}
	if utf8.RuneCountInString(s.Prompt) > MaxStepPrompt {
		return invalid("prompt is at most %d characters", MaxStepPrompt)
	}
	switch s.Type {
	case StepTypeAgent, StepTypeApproval:
		if strings.TrimSpace(s.Prompt) == "" {
			return invalid("%s step requires a prompt", s.Type)
		}
	case StepTypeReview: // the prompt is optional extra review criteria
	default:
		if s.Prompt != "" {
			return invalid("prompt is only allowed on agent, review and approval steps")
		}
	}
	if s.AgentID != "" {
		if s.Type != StepTypeAgent {
			return invalid("agentId is only allowed on agent steps")
		}
		if !ValidAgentID(s.AgentID) {
			return invalid("invalid agentId")
		}
	}
	if s.TeamID != "" || s.Role != "" {
		if s.Type != StepTypeAgent && s.Type != StepTypeReview {
			return invalid("teamId and role are only allowed on agent and review steps")
		}
		if !ValidAgentID(s.TeamID) || !teamRolePattern.MatchString(s.Role) {
			return invalid("teamId and role must both be set and well-formed")
		}
		if s.AgentID != "" {
			return invalid("select the Agent by agentId or by teamId and role, not both")
		}
	}
	if s.Type == StepTypeReview && s.TeamID == "" {
		return invalid("review step requires teamId and role (the reviewer)")
	}
	switch {
	case s.Reviews != "":
		if s.Type != StepTypeReview && s.Type != StepTypeApproval {
			return invalid("reviews is only allowed on review and approval steps")
		}
		if s.Reviews == s.ID {
			return invalid("a step cannot review itself")
		}
		if !ids[s.Reviews] {
			return invalid("reviews must name a step of this workflow")
		}
	case s.Type == StepTypeReview:
		return invalid("review step requires reviews (the step to review)")
	}
	if s.Capability != "" {
		if s.Type != StepTypeTool && s.Type != StepTypeSystem {
			return invalid("capability is only allowed on tool and system steps")
		}
		if !s.Capability.Valid() {
			return invalid("invalid capability")
		}
	}
	if len(s.Input) > 0 {
		if s.Capability == "" {
			return invalid("input is only allowed on steps with a capability")
		}
		if len(s.Input) > MaxStepInputs {
			return invalid("too many inputs (at most %d)", MaxStepInputs)
		}
		for k, v := range s.Input {
			if !stepInputKey.MatchString(k) || len(k) > MaxLabel {
				return invalid("invalid input name %q", k)
			}
			if utf8.RuneCountInString(v) > MaxVarValue {
				return invalid("input %s is at most %d characters", k, MaxVarValue)
			}
		}
	}
	return nil
}

func (w Workflow) validateEdges(ids map[string]bool) error {
	if len(w.Edges) > MaxWorkflowEdges {
		return invalid("too many edges (at most %d)", MaxWorkflowEdges)
	}
	byID := make(map[string]WorkflowStep, len(w.Steps))
	for _, s := range w.Steps {
		byID[s.ID] = s
	}
	seen := map[WorkflowEdge]bool{}
	bound := map[[2]string]bool{}
	for _, e := range w.Edges {
		if !ids[e.From] || !ids[e.To] {
			return invalid("edge %s -> %s references an unknown step", e.From, e.To)
		}
		if e.From == e.To {
			return invalid("edge cannot connect step %s to itself", e.From)
		}
		if seen[e] {
			return invalid("duplicate edge %s -> %s", e.From, e.To)
		}
		seen[e] = true
		if (e.FromPort == "") != (e.ToPort == "") {
			return invalid("a data edge needs both fromPort and toPort")
		}
		if !e.IsData() {
			continue
		}
		if !stepInputKey.MatchString(e.FromPort) || !stepInputKey.MatchString(e.ToPort) || len(e.FromPort) > MaxLabel || len(e.ToPort) > MaxLabel {
			return invalid("invalid edge port name")
		}
		to := byID[e.To]
		if to.Capability == "" {
			return invalid("a data edge must end at a step with a capability")
		}
		k := [2]string{e.To, e.ToPort}
		if bound[k] {
			return invalid("input %s of %s is connected twice", e.ToPort, e.To)
		}
		bound[k] = true
		if _, set := to.Input[e.ToPort]; set {
			return invalid("input %s of %s is both connected and set", e.ToPort, e.To)
		}
	}
	_, err := w.Order()
	return err
}

func (w Workflow) validateVariables() error {
	if len(w.Variables) > MaxWorkflowVars {
		return invalid("too many variables (at most %d)", MaxWorkflowVars)
	}
	names := map[string]bool{}
	for _, v := range w.Variables {
		if strings.TrimSpace(v.Name) == "" || strings.ContainsAny(v.Name, "{} \t\n") {
			return invalid("variable name must be non-empty without spaces or braces")
		}
		if names[v.Name] {
			return invalid("variable %q is not unique", v.Name)
		}
		names[v.Name] = true
		if utf8.RuneCountInString(v.Value) > MaxVarValue {
			return invalid("variable %s is at most %d characters", v.Name, MaxVarValue)
		}
	}
	return nil
}

// Order returns step indexes in execution order: topological over Edges, ties broken by list
// order, so a Workflow without edges runs exactly in list order. A cycle is an error.
func (w Workflow) Order() ([]int, error) {
	idx := make(map[string]int, len(w.Steps))
	for i, s := range w.Steps {
		idx[s.ID] = i
	}
	indeg := make([]int, len(w.Steps))
	next := make([][]int, len(w.Steps))
	for _, e := range w.Edges {
		f, okF := idx[e.From]
		t, okT := idx[e.To]
		if !okF || !okT {
			continue
		}
		next[f] = append(next[f], t)
		indeg[t]++
	}
	order := make([]int, 0, len(w.Steps))
	done := make([]bool, len(w.Steps))
	for len(order) < len(w.Steps) {
		pick := -1
		for i := range w.Steps { // lowest list index with no pending predecessor: stable and small
			if !done[i] && indeg[i] == 0 {
				pick = i
				break
			}
		}
		if pick < 0 {
			return nil, invalid("workflow edges form a cycle")
		}
		done[pick] = true
		order = append(order, pick)
		for _, t := range next[pick] {
			indeg[t]--
		}
	}
	return order, nil
}

// DataEdgesTo returns the data edges that feed step id.
func (w Workflow) DataEdgesTo(id string) []WorkflowEdge {
	var out []WorkflowEdge
	for _, e := range w.Edges {
		if e.To == id && e.IsData() {
			out = append(out, e)
		}
	}
	return out
}
