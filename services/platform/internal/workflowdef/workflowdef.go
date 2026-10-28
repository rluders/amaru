// Package workflowdef reads a Workflow definition from its text form: the reviewable YAML file
// a Project commits under .amaru/workflows/ (ADR-0027). It decodes strictly, hands the result to
// domain.Workflow.Validate and returns nothing that did not pass.
//
// It reads bytes, not files: where the bytes come from (Git, a snapshot, an upload) is the
// caller's concern, and this package stays free of OS access. Only the current format is read.
// A file in any earlier format is refused with an error that says so; converting one is an
// explicit migration, never a side effect of loading.
package workflowdef

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"go.yaml.in/yaml/v3"

	"github.com/rluders/amaru/services/platform/internal/domain"
)

// SchemaVersion is the only format version this package reads.
const SchemaVersion = 1

// MaxBytes bounds a definition before it is parsed. The field limits in domain bound a valid
// Workflow well below it; this stops a hostile file from costing parser time and memory.
const MaxBytes = 512 << 10

// ErrFormat marks a definition that is not a Workflow in the current format. It wraps
// domain.ErrInvalid, so callers that only test for invalid input need nothing more.
var ErrFormat = fmt.Errorf("%w: workflow format", domain.ErrInvalid)

type file struct {
	SchemaVersion int        `yaml:"schemaVersion"`
	ID            string     `yaml:"id"`
	Name          string     `yaml:"name"`
	Description   string     `yaml:"description"`
	Status        string     `yaml:"status"`
	Steps         []step     `yaml:"steps"`
	Edges         []edge     `yaml:"edges"`
	Variables     []variable `yaml:"variables"`
}

type step struct {
	ID             string            `yaml:"id"`
	Name           string            `yaml:"name"`
	Type           string            `yaml:"type"`
	Prompt         string            `yaml:"prompt"`
	Capability     string            `yaml:"capability"`
	Input          map[string]string `yaml:"input"`
	AgentID        string            `yaml:"agentId"`
	TimeoutSeconds int               `yaml:"timeoutSeconds"`
	TeamID         string            `yaml:"teamId"`
	Role           string            `yaml:"role"`
	Reviews        string            `yaml:"reviews"`
}

type edge struct {
	From     string `yaml:"from"`
	FromPort string `yaml:"fromPort"`
	To       string `yaml:"to"`
	ToPort   string `yaml:"toPort"`
}

type variable struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
}

// Parse reads one Workflow definition. Unknown fields, duplicate keys, a second YAML document,
// anchors and aliases, a missing or different schemaVersion and anything Validate refuses are errors.
func Parse(data []byte) (domain.Workflow, error) {
	if len(data) > MaxBytes {
		return domain.Workflow{}, fmt.Errorf("%w: definition is larger than %d bytes", ErrFormat, MaxBytes)
	}
	if err := refuseAliases(data); err != nil {
		return domain.Workflow{}, err
	}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	var f file
	if err := dec.Decode(&f); err != nil {
		if errors.Is(err, io.EOF) {
			return domain.Workflow{}, fmt.Errorf("%w: empty definition", ErrFormat)
		}
		return domain.Workflow{}, fmt.Errorf("%w: %v", ErrFormat, err)
	}
	var extra yaml.Node
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return domain.Workflow{}, fmt.Errorf("%w: one definition per file", ErrFormat)
	}
	if f.SchemaVersion != SchemaVersion {
		return domain.Workflow{}, fmt.Errorf("%w: schemaVersion must be %d, got %d; an earlier format needs an explicit migration", ErrFormat, SchemaVersion, f.SchemaVersion)
	}
	w := f.workflow()
	if err := w.Validate(); err != nil {
		return domain.Workflow{}, err
	}
	return w, nil
}

// refuseAliases rejects anchors and aliases. A reviewer reads a Workflow top to bottom; a value
// that hides behind a reference is harder to review, and expanding references is how a small file
// becomes a large one.
func refuseAliases(data []byte) error {
	var root yaml.Node
	if err := yaml.NewDecoder(bytes.NewReader(data)).Decode(&root); err != nil {
		return nil // empty or malformed input is reported by the strict decode that follows
	}
	var walk func(n *yaml.Node) error
	walk = func(n *yaml.Node) error {
		if n.Kind == yaml.AliasNode || n.Anchor != "" {
			return fmt.Errorf("%w: line %d: anchors and aliases are not allowed", ErrFormat, n.Line)
		}
		for _, c := range n.Content {
			if err := walk(c); err != nil {
				return err
			}
		}
		return nil
	}
	return walk(&root)
}

func (f file) workflow() domain.Workflow {
	w := domain.Workflow{ID: f.ID, Name: f.Name, Description: f.Description, Status: domain.WorkflowStatus(f.Status)}
	for _, s := range f.Steps {
		w.Steps = append(w.Steps, domain.WorkflowStep{
			ID: s.ID, Name: s.Name, Type: domain.WorkflowStepType(s.Type), Prompt: s.Prompt,
			Capability: domain.Capability(s.Capability), Input: s.Input,
			AgentID: s.AgentID, TimeoutSeconds: s.TimeoutSeconds, TeamID: s.TeamID, Role: s.Role, Reviews: s.Reviews,
		})
	}
	for _, e := range f.Edges {
		w.Edges = append(w.Edges, domain.WorkflowEdge(e))
	}
	for _, v := range f.Variables {
		w.Variables = append(w.Variables, domain.WorkflowVariable(v))
	}
	return w
}
