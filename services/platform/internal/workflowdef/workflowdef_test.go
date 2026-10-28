package workflowdef

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/rluders/amaru/services/platform/internal/domain"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestParseValidFixture(t *testing.T) {
	w, err := Parse(fixture(t, "valid.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if w.ID != "release-check" || w.Status != domain.WorkflowActive || len(w.Steps) != 4 || len(w.Edges) != 3 || len(w.Variables) != 1 {
		t.Fatalf("unexpected workflow: %+v", w)
	}
	if got := w.Steps[1]; got.Type != domain.StepTypeTool || got.Capability != "code.test" || got.Input["path"] != "./..." {
		t.Fatalf("tool step not read: %+v", got)
	}
	order, err := w.Order()
	if err != nil || fmt.Sprint(order) != "[0 1 2 3]" {
		t.Fatalf("order = %v, %v", order, err)
	}
}

// header is a valid prefix; each case appends the part under test.
const header = "schemaVersion: 1\nid: demo-flow\nname: Demo\nstatus: draft\n"

const oneStep = "steps:\n  - {id: a, name: A, type: agent, prompt: do it}\n"

func TestParseRefusesWhatIsNotTheCurrentFormat(t *testing.T) {
	cases := map[string]string{
		"empty":                  "",
		"not yaml":               "steps: [",
		"no schemaVersion":       "id: demo-flow\nname: Demo\nstatus: draft\n" + oneStep,
		"future schemaVersion":   "schemaVersion: 2\nid: demo-flow\nname: Demo\nstatus: draft\n" + oneStep,
		"unknown field":          header + oneStep + "owner: me\n",
		"unknown step field":     header + "steps:\n  - {id: a, name: A, type: agent, prompt: p, tool: claude}\n",
		"duplicate key":          header + "name: Again\n" + oneStep,
		"two documents":          header + oneStep + "---\n" + header + oneStep,
		"prototype shape":        "id: WF-001\nname: Demo\nstatus: active\ntype: pipeline\nsteps:\n  - {id: step-1, name: A, type: agent, prompt: p}\n",
		"prototype json":         `{"id":"WF-001","name":"Demo","status":"active","type":"pipeline","steps":[{"id":"step-1","name":"A","type":"agent","prompt":"p"}]}`,
		"anchor":                 header + "steps: &s\n  - {id: a, name: A, type: agent, prompt: p}\n",
		"alias":                  header + "variables: &v []\nsteps:\n  - {id: a, name: A, type: agent, prompt: p}\nedges: *v\n",
		"wrong type for a field": header + "steps: not-a-list\n",
	}
	for name, doc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := Parse([]byte(doc))
			if !errors.Is(err, domain.ErrInvalid) {
				t.Fatalf("want ErrInvalid, got %v", err)
			}
		})
	}
}

func TestParseNamesTheMigrationForAnEarlierFormat(t *testing.T) {
	_, err := Parse([]byte("id: WF-001\nname: Demo\nstatus: active\n" + oneStep))
	if err == nil || !strings.Contains(err.Error(), "explicit migration") {
		t.Fatalf("error should point at the migration: %v", err)
	}
}

func TestParseRefusesInvalidWorkflows(t *testing.T) {
	cases := map[string]string{
		"bad id":           "schemaVersion: 1\nid: Demo_Flow\nname: Demo\nstatus: draft\n" + oneStep,
		"bad status":       "schemaVersion: 1\nid: demo-flow\nname: Demo\nstatus: running\n" + oneStep,
		"no steps":         header,
		"duplicate step":   header + "steps:\n  - {id: a, name: A, type: agent, prompt: p}\n  - {id: a, name: B, type: agent, prompt: p}\n",
		"agent no prompt":  header + "steps:\n  - {id: a, name: A, type: agent}\n",
		"edge unknown":     header + oneStep + "edges:\n  - {from: a, to: zzz}\n",
		"self edge":        header + oneStep + "edges:\n  - {from: a, to: a}\n",
		"cycle":            header + "steps:\n  - {id: a, name: A, type: agent, prompt: p}\n  - {id: b, name: B, type: agent, prompt: p}\nedges:\n  - {from: a, to: b}\n  - {from: b, to: a}\n",
		"negative timeout": header + "steps:\n  - {id: a, name: A, type: agent, prompt: p, timeoutSeconds: -5}\n",
		"review no target": header + "steps:\n  - {id: a, name: A, type: review, teamId: team-one, role: reviewer}\n",
	}
	for name, doc := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse([]byte(doc)); !errors.Is(err, domain.ErrInvalid) {
				t.Fatalf("want ErrInvalid, got %v", err)
			}
		})
	}
}

func TestParseEnforcesLimits(t *testing.T) {
	steps := func(n int) string {
		var b strings.Builder
		b.WriteString("steps:\n")
		for i := range n {
			fmt.Fprintf(&b, "  - {id: s%d, name: S, type: agent, prompt: p}\n", i)
		}
		return b.String()
	}
	edges := func(n int) string {
		var b strings.Builder
		b.WriteString("edges:\n")
		for i := range n {
			// forward edges only, so the graph stays acyclic and only the count is wrong
			fmt.Fprintf(&b, "  - {from: s%d, to: s%d}\n", i%25, 25+i%25)
		}
		return b.String()
	}
	vars := func(n int) string {
		var b strings.Builder
		b.WriteString("variables:\n")
		for i := range n {
			fmt.Fprintf(&b, "  - {name: v%d, value: x}\n", i)
		}
		return b.String()
	}
	prompt := func(n int) string {
		return "steps:\n  - {id: a, name: A, type: agent, prompt: " + strings.Repeat("x", n) + "}\n"
	}
	type tc struct {
		doc string
		ok  bool
	}
	cases := map[string]tc{
		"steps at the limit":     {header + steps(domain.MaxWorkflowSteps), true},
		"steps over the limit":   {header + steps(domain.MaxWorkflowSteps+1), false},
		"prompt at the limit":    {header + prompt(domain.MaxStepPrompt), true},
		"prompt over the limit":  {header + prompt(domain.MaxStepPrompt+1), false},
		"variables at the limit": {header + oneStep + vars(domain.MaxWorkflowVars), true},
		"variables over limit":   {header + oneStep + vars(domain.MaxWorkflowVars+1), false},
		"edges over the limit":   {header + steps(50) + edges(domain.MaxWorkflowEdges+1), false},
		"file over the byte cap": {header + oneStep + "description: " + strings.Repeat("x", MaxBytes), false},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := Parse([]byte(c.doc))
			if c.ok && err != nil {
				t.Fatalf("want ok, got %v", err)
			}
			if !c.ok && !errors.Is(err, domain.ErrInvalid) {
				t.Fatalf("want ErrInvalid, got %v", err)
			}
		})
	}
}

func TestParseReadsTheStepTimeout(t *testing.T) {
	w, err := Parse([]byte(header + "steps:\n  - {id: a, name: A, type: agent, prompt: p, timeoutSeconds: 5400}\n  - {id: b, name: B, type: tool, capability: code.test}\n"))
	if err != nil {
		t.Fatal(err)
	}
	if w.Steps[0].TimeoutSeconds != 5400 || w.Steps[1].TimeoutSeconds != 0 {
		t.Fatalf("timeouts %d, %d", w.Steps[0].TimeoutSeconds, w.Steps[1].TimeoutSeconds)
	}
}
