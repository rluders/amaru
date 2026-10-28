package domain

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
)

// DefinitionPin is the exact Workflow definition an Execution runs (FEAT-WFL-002). Versions of a
// definition are the Git history of .amaru/, but an Execution does not depend on Git: it carries
// an immutable Snapshot, so editing the file never changes a running or past Execution and a
// resume after a restart reads the snapshot, not the working tree.
type DefinitionPin struct {
	WorkflowID string `json:"workflowId"`
	// Revision is the commit SHA the definition was read at. It is empty when the file was not
	// committed; the Hash still identifies the content.
	Revision string `json:"revision,omitempty"`
	// Hash is "sha256:" and the hex SHA-256 of Snapshot.
	Hash string `json:"hash"`
	// Snapshot is the canonical JSON of the Workflow as it was run.
	Snapshot json.RawMessage `json:"snapshot"`
}

var revisionPattern = regexp.MustCompile(`^([0-9a-f]{40}|[0-9a-f]{64})$`)

// Pin validates w and returns the pin for it. revision is a Git commit id (SHA-1 or SHA-256, in
// lowercase hex) or empty. A draft or archived Workflow cannot be pinned because it cannot run.
func Pin(w Workflow, revision string) (DefinitionPin, error) {
	if err := w.Validate(); err != nil {
		return DefinitionPin{}, err
	}
	if w.Status != WorkflowActive {
		return DefinitionPin{}, invalid("workflow %s is %s; only an active workflow can run", w.ID, w.Status)
	}
	if revision != "" && !revisionPattern.MatchString(revision) {
		return DefinitionPin{}, invalid("revision must be a full Git commit id in lowercase hex")
	}
	snap, err := json.Marshal(w)
	if err != nil {
		return DefinitionPin{}, fmt.Errorf("encode workflow snapshot: %w", err)
	}
	return DefinitionPin{WorkflowID: w.ID, Revision: revision, Hash: hashOf(snap), Snapshot: snap}, nil
}

// Workflow returns the pinned definition. It refuses a pin whose snapshot no longer matches its
// hash or no longer describes the Workflow it names, so a damaged row fails loudly instead of
// running something else.
func (p DefinitionPin) Workflow() (Workflow, error) {
	if hashOf(p.Snapshot) != p.Hash {
		return Workflow{}, invalid("pinned definition %s does not match its hash", p.WorkflowID)
	}
	var w Workflow
	dec := json.NewDecoder(bytes.NewReader(p.Snapshot))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&w); err != nil {
		return Workflow{}, invalid("pinned definition %s: %v", p.WorkflowID, err)
	}
	if err := w.Validate(); err != nil {
		return Workflow{}, err
	}
	if w.ID != p.WorkflowID {
		return Workflow{}, invalid("pinned definition names workflow %s but holds %s", p.WorkflowID, w.ID)
	}
	return w, nil
}

func hashOf(b []byte) string {
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:])
}
