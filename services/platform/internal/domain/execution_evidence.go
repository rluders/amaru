package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"time"
)

// EvidenceGap names a kind of Evidence the bundle cannot hold yet because the Platform records
// none of it. A gap is listed, never left out, so a reader does not take its absence for "there
// was nothing".
type EvidenceGap struct {
	Kind   string `json:"kind"`
	Reason string `json:"reason"`
}

// Kinds of Evidence that FEAT-ART-002 names and the Platform does not record yet. Each is removed
// from ExecutionEvidence.Gaps when its recorder lands.
const (
	GapDiff      = "diff"
	GapLog       = "log"
	GapCheck     = "check"
	GapUsage     = "usage"
	GapWorkspace = "workspace_snapshot"
)

// evidenceGaps is the list of kinds with no recorder. It is a function so a caller cannot edit it.
func evidenceGaps() []EvidenceGap {
	const no = "the Platform records no %s yet"
	return []EvidenceGap{
		{GapCheck, fmt.Sprintf(no, "check results")},
		{GapDiff, fmt.Sprintf(no, "diffs")},
		{GapLog, fmt.Sprintf(no, "Step logs")},
		{GapUsage, fmt.Sprintf(no, "usage")},
		{GapWorkspace, fmt.Sprintf(no, "workspace snapshots")},
	}
}

// StepEvidence is what the Platform holds about one Step of an Execution. The Step's inputs are
// not copied, because they may hold values the Evidence reader should not see; InputsHash lets a
// reader check a copy of them.
type StepEvidence struct {
	ID         string     `json:"id"`
	State      StepState  `json:"state"`
	Reason     string     `json:"reason,omitempty"`
	Capability string     `json:"capability,omitempty"`
	AttemptID  string     `json:"attemptId,omitempty"`
	StartedAt  *time.Time `json:"startedAt,omitempty"`
	EndedAt    *time.Time `json:"endedAt,omitempty"`
	Error      *ExecError `json:"error,omitempty"`
	InputsHash string     `json:"inputsHash,omitempty"`
	// Skills are the Skills the Step pinned; Agent is the Agent definition it ran as, resolved
	// when the Step was planned.
	Skills []SkillRef     `json:"skills,omitempty"`
	Agent  *ResolvedAgent `json:"agent,omitempty"`
	// Context is what the Agent was told; nil when the Platform assembled none for the Step.
	Context *StepContext `json:"context,omitempty"`
	// Artifacts are the versions the Step produced, ordered by Artifact id.
	Artifacts []ArtifactVersion `json:"artifacts,omitempty"`
}

// ExecutionEvidence is the bundle of everything the Platform holds about one Execution, so one
// query answers an audit (FEAT-ART-002). It has no clock of its own: the same stored data always
// gives the same bundle and the same Digest.
type ExecutionEvidence struct {
	Execution Execution        `json:"execution"`
	Steps     []StepEvidence   `json:"steps"`
	Events    []ExecutionEvent `json:"events"`
	// Approvals are the Approvals that gate the Execution's Steps, each with the Evidence its
	// approver was shown and the decision; Audit is their entries in the audit log.
	Approvals []Approval   `json:"approvals"`
	Audit     []AuditEvent `json:"audit"`
	// Final is true once the Execution is in a terminal state. A bundle read earlier is a view of
	// a run still in progress.
	Final bool `json:"final"`
	// Gaps lists the Evidence kinds the Platform does not record yet.
	Gaps []EvidenceGap `json:"gaps"`
	// Digest is "sha256:" and the hash of the bundle's JSON with Digest empty.
	Digest string `json:"digest"`
}

// ExecutionEvidenceInput is what the Platform read from its stores. Build checks that it is about
// one Execution.
type ExecutionEvidenceInput struct {
	Execution Execution
	Steps     []StepExecution
	Events    []ExecutionEvent
	Approvals []Approval
	// Audit holds the audit entries of the Approvals only.
	Audit     []AuditEvent
	Contexts  []StepContext
	Artifacts []ArtifactVersion
}

// BuildExecutionEvidence assembles the bundle. It returns ErrInvalid when a part belongs to
// another Execution or Step, so a bundle never mixes runs. Steps keep their creation order,
// Approvals their request order and the logs their Seq order; Artifacts are grouped under their
// Step.
func BuildExecutionEvidence(in ExecutionEvidenceInput) (ExecutionEvidence, error) {
	id := in.Execution.ID
	if id == "" {
		return ExecutionEvidence{}, invalid("evidence: the execution has no id")
	}
	steps := make(map[string]int, len(in.Steps))
	out := ExecutionEvidence{
		Execution: in.Execution,
		Steps:     make([]StepEvidence, len(in.Steps)),
		Events:    slices.Clone(in.Events),
		Approvals: slices.Clone(in.Approvals),
		Audit:     slices.Clone(in.Audit),
		Final:     in.Execution.State.Terminal(),
		Gaps:      evidenceGaps(),
	}
	for i, st := range in.Steps {
		if st.ExecutionID != id {
			return ExecutionEvidence{}, invalid("evidence: step %s belongs to execution %s, not %s", st.ID, st.ExecutionID, id)
		}
		steps[st.ID] = i
		se := StepEvidence{
			ID: st.ID, State: st.State, Reason: st.Reason, Capability: st.Capability, AttemptID: st.AttemptID,
			StartedAt: st.StartedAt, EndedAt: st.EndedAt, Error: st.Error,
			Skills: slices.Clone(st.Skills), Agent: st.Agent,
		}
		if len(st.InputsJSON) > 0 {
			sum := sha256.Sum256(st.InputsJSON)
			se.InputsHash = "sha256:" + hex.EncodeToString(sum[:])
		}
		out.Steps[i] = se
	}
	for _, c := range in.Contexts {
		i, ok := steps[c.StepID]
		if !ok || c.ExecutionID != id {
			return ExecutionEvidence{}, invalid("evidence: context of step %s/%s is not a step of execution %s", c.ExecutionID, c.StepID, id)
		}
		c := c
		out.Steps[i].Context = &c
	}
	for _, v := range in.Artifacts {
		i, ok := steps[v.StepID]
		if !ok || v.ExecutionID != id {
			return ExecutionEvidence{}, invalid("evidence: artifact %s v%d of step %s/%s is not a step of execution %s", v.ArtifactID, v.Version, v.ExecutionID, v.StepID, id)
		}
		out.Steps[i].Artifacts = append(out.Steps[i].Artifacts, v)
	}
	for i := range out.Steps {
		slices.SortStableFunc(out.Steps[i].Artifacts, func(a, b ArtifactVersion) int {
			if a.ArtifactID != b.ArtifactID {
				if a.ArtifactID < b.ArtifactID {
					return -1
				}
				return 1
			}
			return int(a.Version - b.Version)
		})
	}
	approvals := make(map[string]bool, len(in.Approvals))
	for _, a := range in.Approvals {
		if a.Gate.ExecutionID != id {
			return ExecutionEvidence{}, invalid("evidence: approval %s gates execution %s, not %s", a.ID, a.Gate.ExecutionID, id)
		}
		approvals[a.ID] = true
	}
	for _, ev := range in.Audit {
		if ev.TargetType != "approval" || !approvals[ev.TargetID] {
			return ExecutionEvidence{}, invalid("evidence: audit entry %d (%s %s) is not about an approval of execution %s", ev.Seq, ev.TargetType, ev.TargetID, id)
		}
	}
	for _, ev := range in.Events {
		if ev.ExecutionID != id {
			return ExecutionEvidence{}, invalid("evidence: event %d belongs to execution %s, not %s", ev.Seq, ev.ExecutionID, id)
		}
	}
	if out.Events == nil {
		out.Events = []ExecutionEvent{}
	}
	if out.Approvals == nil {
		out.Approvals = []Approval{}
	}
	if out.Audit == nil {
		out.Audit = []AuditEvent{}
	}
	raw, err := json.Marshal(out)
	if err != nil {
		return ExecutionEvidence{}, fmt.Errorf("evidence: encode: %w", err)
	}
	sum := sha256.Sum256(raw)
	out.Digest = "sha256:" + hex.EncodeToString(sum[:])
	return out, nil
}
