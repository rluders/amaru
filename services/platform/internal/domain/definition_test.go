package domain

import (
	"errors"
	"strings"
	"testing"
)

func pinnable() Workflow {
	return Workflow{
		ID: "ship-it", Name: "Ship it", Status: WorkflowActive,
		Steps: []WorkflowStep{
			{ID: "build", Name: "Build", Type: StepTypeTool, Capability: "code.build"},
			{ID: "test", Name: "Test", Type: StepTypeTool, Capability: "code.test"},
		},
	}
}

func TestPin(t *testing.T) {
	sha := strings.Repeat("a", 40)
	p, err := Pin(pinnable(), sha)
	if err != nil {
		t.Fatal(err)
	}
	if p.WorkflowID != "ship-it" || p.Revision != sha || !strings.HasPrefix(p.Hash, "sha256:") {
		t.Fatalf("pin: %+v", p)
	}
	t.Run("the same content has the same hash", func(t *testing.T) {
		q, _ := Pin(pinnable(), "")
		if q.Hash != p.Hash {
			t.Fatalf("hash depends on the revision: %s vs %s", q.Hash, p.Hash)
		}
	})
	t.Run("an edit changes the hash and leaves the pin alone", func(t *testing.T) {
		w := pinnable()
		w.Steps[1].Capability = "code.lint"
		q, _ := Pin(w, "")
		if q.Hash == p.Hash {
			t.Fatal("an edited definition has the same hash")
		}
		got, err := p.Workflow()
		if err != nil || got.Steps[1].Capability != "code.test" {
			t.Fatalf("pinned definition changed: %+v, %v", got, err)
		}
	})
	t.Run("the snapshot does not alias the caller's Workflow", func(t *testing.T) {
		w := pinnable()
		q, _ := Pin(w, "")
		w.Steps[0].Capability = "x.y"
		got, _ := q.Workflow()
		if got.Steps[0].Capability != "code.build" {
			t.Fatalf("snapshot followed the caller: %+v", got)
		}
	})
}

func TestPinRefuses(t *testing.T) {
	bad := pinnable()
	bad.Steps = nil
	draft := pinnable()
	draft.Status = WorkflowDraft
	for name, tc := range map[string]struct {
		w   Workflow
		rev string
	}{
		"invalid workflow": {bad, ""},
		"draft":            {draft, ""},
		"short revision":   {pinnable(), "abc123"},
		"upper revision":   {pinnable(), strings.Repeat("A", 40)},
		"branch name":      {pinnable(), "main"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Pin(tc.w, tc.rev); !errors.Is(err, ErrInvalid) {
				t.Fatalf("got %v, want ErrInvalid", err)
			}
		})
	}
}

func TestPinWorkflowRefusesDamage(t *testing.T) {
	p, _ := Pin(pinnable(), "")
	t.Run("tampered snapshot", func(t *testing.T) {
		q := p
		q.Snapshot = []byte(strings.Replace(string(p.Snapshot), "code.test", "code.lint", 1))
		if _, err := q.Workflow(); !errors.Is(err, ErrInvalid) {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("hash matches but snapshot is another workflow", func(t *testing.T) {
		q := p
		q.WorkflowID = "other-flow"
		if _, err := q.Workflow(); !errors.Is(err, ErrInvalid) {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("hash matches but snapshot is not a valid workflow", func(t *testing.T) {
		q := DefinitionPin{WorkflowID: "ship-it", Snapshot: []byte(`{"id":"ship-it","extra":1}`)}
		q.Hash = hashOf(q.Snapshot)
		if _, err := q.Workflow(); !errors.Is(err, ErrInvalid) {
			t.Fatalf("got %v", err)
		}
	})
}
