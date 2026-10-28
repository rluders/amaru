package domain

import (
	"errors"
	"fmt"
	"testing"
)

func okWorkflow() Workflow {
	return Workflow{ID: "demo-flow", Name: "Demo", Status: WorkflowActive,
		Steps: []WorkflowStep{{ID: "a", Name: "A", Type: StepTypeAgent, Prompt: "do"}}}
}

func TestWorkflowValidate(t *testing.T) {
	if err := okWorkflow().Validate(); err != nil {
		t.Fatal(err)
	}
	two := func(w *Workflow) {
		w.Steps = append(w.Steps, WorkflowStep{ID: "b", Name: "B", Type: StepTypeTool, Capability: "code.test"})
	}
	cases := map[string]func(*Workflow){
		"bad id":              func(w *Workflow) { w.ID = "Demo" },
		"empty name":          func(w *Workflow) { w.Name = " " },
		"bad status":          func(w *Workflow) { w.Status = "x" },
		"no steps":            func(w *Workflow) { w.Steps = nil },
		"bad step type":       func(w *Workflow) { w.Steps[0].Type = "x" },
		"bad step id":         func(w *Workflow) { w.Steps[0].ID = "a.b" },
		"agent no prompt":     func(w *Workflow) { w.Steps[0].Prompt = "" },
		"approval no prompt":  func(w *Workflow) { w.Steps[0].Type = StepTypeApproval; w.Steps[0].Prompt = "" },
		"tool with prompt":    func(w *Workflow) { two(w); w.Steps[1].Prompt = "x" },
		"dup step id":         func(w *Workflow) { w.Steps = append(w.Steps, w.Steps[0]) },
		"bad capability":      func(w *Workflow) { two(w); w.Steps[1].Capability = "Test" },
		"capability on agent": func(w *Workflow) { w.Steps[0].Capability = "code.test" },
		"input without cap":   func(w *Workflow) { w.Steps[0].Input = map[string]string{"k": "v"} },
		"bad input name":      func(w *Workflow) { two(w); w.Steps[1].Input = map[string]string{"Bad": "v"} },
		"agentId on a tool":   func(w *Workflow) { two(w); w.Steps[1].AgentID = "coder-agent" },
		"negative timeout":    func(w *Workflow) { w.Steps[0].TimeoutSeconds = -1 },
		"approval timeout":    func(w *Workflow) { w.Steps[0].Type = StepTypeApproval; w.Steps[0].TimeoutSeconds = 60 },
		"agentId and team": func(w *Workflow) {
			w.Steps[0].AgentID = "coder-agent"
			w.Steps[0].TeamID = "team-one"
			w.Steps[0].Role = "author"
		},
		"team without role": func(w *Workflow) { w.Steps[0].TeamID = "team-one" },
		"review of itself": func(w *Workflow) {
			w.Steps[0] = WorkflowStep{ID: "a", Name: "A", Type: StepTypeReview, TeamID: "team-one", Role: "reviewer", Reviews: "a"}
		},
		"review of unknown": func(w *Workflow) {
			w.Steps[0] = WorkflowStep{ID: "a", Name: "A", Type: StepTypeReview, TeamID: "team-one", Role: "reviewer", Reviews: "zz"}
		},
		"review without target": func(w *Workflow) {
			w.Steps[0] = WorkflowStep{ID: "a", Name: "A", Type: StepTypeReview, TeamID: "team-one", Role: "reviewer"}
		},
		"reviews on an agent": func(w *Workflow) { two(w); w.Steps[0].Reviews = "b" },
		"bad var name":        func(w *Workflow) { w.Variables = []WorkflowVariable{{Name: "a b"}} },
		"dup var":             func(w *Workflow) { w.Variables = []WorkflowVariable{{Name: "a"}, {Name: "a"}} },
		"long prompt":         func(w *Workflow) { w.Steps[0].Prompt = string(make([]rune, MaxStepPrompt+1)) },
	}
	for name, mut := range cases {
		t.Run(name, func(t *testing.T) {
			w := okWorkflow()
			w.Steps = append([]WorkflowStep{}, w.Steps...)
			mut(&w)
			if err := w.Validate(); !errors.Is(err, ErrInvalid) {
				t.Fatalf("want ErrInvalid, got %v", err)
			}
		})
	}
}

func chain(n int) Workflow {
	w := Workflow{ID: "chain-flow", Name: "C", Status: WorkflowDraft}
	for i := range n {
		w.Steps = append(w.Steps, WorkflowStep{ID: fmt.Sprintf("s%d", i), Name: "S", Type: StepTypeAgent, Prompt: "p"})
	}
	return w
}

func TestWorkflowLimits(t *testing.T) {
	if err := chain(MaxWorkflowSteps).Validate(); err != nil {
		t.Fatalf("at the step limit: %v", err)
	}
	if err := chain(MaxWorkflowSteps + 1).Validate(); !errors.Is(err, ErrInvalid) {
		t.Fatalf("over the step limit: %v", err)
	}
	w := chain(MaxWorkflowSteps)
	for i := range MaxWorkflowEdges + 1 {
		w.Edges = append(w.Edges, WorkflowEdge{From: fmt.Sprintf("s%d", i%25), To: fmt.Sprintf("s%d", 25+i%25), FromPort: fmt.Sprintf("o%d", i), ToPort: fmt.Sprintf("i%d", i)})
	}
	if err := w.Validate(); !errors.Is(err, ErrInvalid) {
		t.Fatalf("over the edge limit: %v", err)
	}
}

func TestWorkflowOrder(t *testing.T) {
	w := chain(4)
	if got, _ := w.Order(); fmt.Sprint(got) != "[0 1 2 3]" {
		t.Fatalf("no edges keeps list order, got %v", got)
	}
	w.Edges = []WorkflowEdge{{From: "s3", To: "s0"}, {From: "s1", To: "s2"}}
	got, err := w.Order()
	if err != nil || fmt.Sprint(got) != "[1 2 3 0]" {
		t.Fatalf("edges order, ties by list index: %v, %v", got, err)
	}
	w.Edges = append(w.Edges, WorkflowEdge{From: "s0", To: "s1"}, WorkflowEdge{From: "s2", To: "s3"})
	if _, err := w.Order(); !errors.Is(err, ErrInvalid) {
		t.Fatalf("cycle must be refused, got %v", err)
	}
	if err := w.Validate(); !errors.Is(err, ErrInvalid) {
		t.Fatalf("Validate must refuse a cycle, got %v", err)
	}
}

func TestWorkflowDataEdges(t *testing.T) {
	w := Workflow{ID: "data-flow", Name: "D", Status: WorkflowDraft, Steps: []WorkflowStep{
		{ID: "a", Name: "A", Type: StepTypeTool, Capability: "code.build"},
		{ID: "b", Name: "B", Type: StepTypeTool, Capability: "code.test", Input: map[string]string{"set": "x"}},
	}, Edges: []WorkflowEdge{{From: "a", To: "b", FromPort: "bin", ToPort: "binary"}}}
	if err := w.Validate(); err != nil {
		t.Fatal(err)
	}
	if got := w.DataEdgesTo("b"); len(got) != 1 || !got[0].IsData() {
		t.Fatalf("DataEdgesTo = %+v", got)
	}
	bad := map[string]func(*Workflow){
		"half edge":          func(w *Workflow) { w.Edges[0].ToPort = "" },
		"both bound and set": func(w *Workflow) { w.Edges[0].ToPort = "set" },
		"into an agent":      func(w *Workflow) { w.Steps[1] = WorkflowStep{ID: "b", Name: "B", Type: StepTypeAgent, Prompt: "p"} },
		"port bound twice": func(w *Workflow) {
			w.Steps = append(w.Steps, WorkflowStep{ID: "c", Name: "C", Type: StepTypeTool, Capability: "code.build"})
			w.Edges = append(w.Edges, WorkflowEdge{From: "c", To: "b", FromPort: "bin", ToPort: "binary"})
		},
	}
	for name, mut := range bad {
		t.Run(name, func(t *testing.T) {
			c := w
			c.Steps = append([]WorkflowStep{}, w.Steps...)
			c.Edges = append([]WorkflowEdge{}, w.Edges...)
			mut(&c)
			if err := c.Validate(); !errors.Is(err, ErrInvalid) {
				t.Fatalf("want ErrInvalid, got %v", err)
			}
		})
	}
}
