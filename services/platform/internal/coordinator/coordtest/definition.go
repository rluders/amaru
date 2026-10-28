package coordtest

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/rluders/amaru/services/platform/internal/coordinator"
	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
)

// DefinitionSuite checks that an Execution records the exact Workflow definition it was submitted
// with and keeps running that definition however the Workflow is edited afterwards (FEAT-WFL-002),
// against the store newStore returns. reopen builds a store over the same data, as a restarted
// Platform sees it; it may be nil for a store that cannot restart.
func DefinitionSuite(t *testing.T, newStore func(t *testing.T) ports.ExecutionStore, reopen Restart) {
	t.Helper()
	if reopen == nil {
		reopen = func(_ *testing.T, s ports.ExecutionStore) ports.ExecutionStore { return s }
	}
	ctx := context.Background()
	sha := strings.Repeat("c", 40)
	// The steps are listed against their order: test depends on build.
	flow := func() domain.Workflow {
		return domain.Workflow{
			ID: "ship-it", Name: "Ship it", Status: domain.WorkflowActive,
			Steps: []domain.WorkflowStep{
				{ID: "test", Name: "Test", Type: domain.StepTypeTool, Capability: "code.test", Input: map[string]string{"pkg": "./..."}},
				{ID: "build", Name: "Build", Type: domain.StepTypeTool, Capability: "code.build"},
			},
			Edges: []domain.WorkflowEdge{{From: "build", To: "test"}},
		}
	}
	build := func(s ports.ExecutionStore) *coordinator.Coordinator {
		var n atomic.Int32
		return coordinator.New(s, clock, coordinator.WithExecutionIDs(func() string { return fmt.Sprintf("exec-%d", n.Add(1)) }))
	}
	submit := func(t *testing.T, c *coordinator.Coordinator, key string, w domain.Workflow) coordinator.Submitted {
		t.Helper()
		got, _, err := c.Submit(ctx, coordinator.SubmitRequest{ProjectID: "p1", Key: key, Workflow: &w, Revision: sha})
		if err != nil {
			t.Fatal(err)
		}
		return got
	}

	t.Run("the Execution records the definition, its hash and the revision", func(t *testing.T) {
		s := newStore(t)
		w := flow()
		got := submit(t, build(s), "k1", w)
		want, _ := domain.Pin(w, sha)
		e, _, err := reopen(t, s).Get(ctx, got.Execution.ID)
		if err != nil {
			t.Fatal(err)
		}
		d := e.Definition
		if d == nil || d.WorkflowID != "ship-it" || d.Revision != sha || d.Hash != want.Hash || string(d.Snapshot) != string(want.Snapshot) {
			t.Fatalf("recorded %+v, want %+v", d, want)
		}
	})

	t.Run("Steps are created in Workflow order with their inputs", func(t *testing.T) {
		s := newStore(t)
		got := submit(t, build(s), "k1", flow())
		if len(got.Steps) != 2 || got.Steps[0].Capability != "code.build" || got.Steps[1].Capability != "code.test" {
			t.Fatalf("steps: %+v", got.Steps)
		}
		if string(got.Steps[1].InputsJSON) != `{"pkg":"./..."}` {
			t.Fatalf("inputs %q", got.Steps[1].InputsJSON)
		}
	})

	t.Run("editing the Workflow never changes a running or past Execution", func(t *testing.T) {
		s := newStore(t)
		c := build(s)
		first := submit(t, c, "k1", flow())
		edited := flow()
		edited.Steps[0].Capability = "code.lint"
		edited.Name = "Ship it, edited"
		second := submit(t, c, "k2", edited)

		for _, tc := range []struct {
			id, capability string
		}{{first.Execution.ID, "code.test"}, {second.Execution.ID, "code.lint"}} {
			got, err := build(reopen(t, s)).Definition(ctx, tc.id)
			if err != nil {
				t.Fatal(err)
			}
			if got.Steps[0].Capability != domain.Capability(tc.capability) {
				t.Fatalf("%s runs %s, want %s", tc.id, got.Steps[0].Capability, tc.capability)
			}
		}
		e1, _, _ := s.Get(ctx, first.Execution.ID)
		e2, _, _ := s.Get(ctx, second.Execution.ID)
		if e1.Definition.Hash == e2.Definition.Hash {
			t.Fatal("two different definitions share a hash")
		}
	})

	t.Run("a write cannot move the pin", func(t *testing.T) {
		s := newStore(t)
		got := submit(t, build(s), "k1", flow())
		e, _, _ := s.Get(ctx, got.Execution.ID)
		before := *e.Definition
		other := flow()
		other.Name = "Swapped"
		pin, _ := domain.Pin(other, "")
		e.Definition = &pin
		if err := e.Start(clock()); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Commit(ctx, ports.ExecutionChange{Execution: &e}); err != nil {
			t.Fatal(err)
		}
		after, _, _ := reopen(t, s).Get(ctx, got.Execution.ID)
		if after.State != domain.ExecRunning || after.Definition == nil || after.Definition.Hash != before.Hash || string(after.Definition.Snapshot) != string(before.Snapshot) {
			t.Fatalf("pin moved: %+v", after.Definition)
		}
	})

	t.Run("the same key and Workflow replays, another Workflow under the key is refused", func(t *testing.T) {
		s := newStore(t)
		c := build(s)
		w := flow()
		first := submit(t, c, "k1", w)
		again, replayed, err := c.Submit(ctx, coordinator.SubmitRequest{ProjectID: "p1", Key: "k1", Workflow: &w, Revision: sha})
		if err != nil || !replayed || again.Execution.ID != first.Execution.ID || again.Execution.Definition == nil || again.Execution.Definition.Hash != first.Execution.Definition.Hash {
			t.Fatalf("replay: %+v replayed=%v, %v", again, replayed, err)
		}
		w.Name = "Another"
		if _, _, err := c.Submit(ctx, coordinator.SubmitRequest{ProjectID: "p1", Key: "k1", Workflow: &w, Revision: sha}); !errors.Is(err, domain.ErrKeyReused) {
			t.Fatalf("got %v, want ErrKeyReused", err)
		}
		if _, _, err := c.Submit(ctx, coordinator.SubmitRequest{ProjectID: "p1", Key: "k1", Workflow: &w}); !errors.Is(err, domain.ErrKeyReused) {
			t.Fatalf("a different revision under the key: got %v, want ErrKeyReused", err)
		}
	})

	t.Run("an Execution without a Workflow has no pinned definition", func(t *testing.T) {
		s := newStore(t)
		c := build(s)
		got, _, err := c.Submit(ctx, coordinator.SubmitRequest{ProjectID: "p1", Key: "k1", Steps: []coordinator.StepSpec{{Name: "a", Capability: "code.test"}}})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := c.Definition(ctx, got.Execution.ID); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("got %v, want ErrNotFound", err)
		}
		if _, err := c.Definition(ctx, "missing"); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("got %v, want ErrNotFound", err)
		}
	})

	t.Run("a request that cannot run is refused and creates nothing", func(t *testing.T) {
		s := newStore(t)
		c := build(s)
		draft := flow()
		draft.Status = domain.WorkflowDraft
		agent := flow()
		agent.Steps = append(agent.Steps, domain.WorkflowStep{ID: "write", Name: "Write", Type: domain.StepTypeAgent, AgentID: "coder", Prompt: "do it"})
		w := flow()
		for name, req := range map[string]coordinator.SubmitRequest{
			"draft":                     {ProjectID: "p1", Key: "a", Workflow: &draft},
			"step without capability":   {ProjectID: "p1", Key: "b", Workflow: &agent},
			"workflow and steps":        {ProjectID: "p1", Key: "c", Workflow: &w, Steps: []coordinator.StepSpec{{Name: "a", Capability: "x.y"}}},
			"revision without workflow": {ProjectID: "p1", Key: "d", Revision: sha, Steps: []coordinator.StepSpec{{Name: "a", Capability: "x.y"}}},
			"bad revision":              {ProjectID: "p1", Key: "e", Workflow: &w, Revision: "main"},
		} {
			if _, _, err := c.Submit(ctx, req); !errors.Is(err, domain.ErrInvalid) {
				t.Fatalf("%s: got %v, want ErrInvalid", name, err)
			}
		}
		if open, _ := s.Open(ctx); len(open) != 0 {
			t.Fatalf("refused requests created %d Executions", len(open))
		}
	})
}
