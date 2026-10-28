package coordtest

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/rluders/amaru/services/platform/internal/coordinator"
	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
)

// CommandSuite checks that a repeated mutating command returns the original result and does not
// run twice (FEAT-EXE-005), against the store newStore returns. reopen builds a store over the
// same data, as a restarted Platform sees it; it may be nil for a store that cannot restart.
func CommandSuite(t *testing.T, newStore func(t *testing.T) ports.ExecutionStore, reopen Restart) {
	t.Helper()
	if reopen == nil {
		reopen = func(_ *testing.T, s ports.ExecutionStore) ports.ExecutionStore { return s }
	}
	ctx := context.Background()
	steps := []coordinator.StepSpec{
		{Name: "build", Capability: "code.build"},
		{Name: "test", Capability: "code.test", Inputs: []byte(`{"pkg":"./..."}`), Idempotent: true},
	}
	// ids counts Executions the coordinator creates, so a test can tell "second Execution" from
	// "second answer".
	build := func(t *testing.T, s ports.ExecutionStore) (*coordinator.Coordinator, *atomic.Int32) {
		var n atomic.Int32
		c := coordinator.New(s, clock, coordinator.WithExecutionIDs(func() string { return fmt.Sprintf("exec-%d", n.Add(1)) }))
		return c, &n
	}

	t.Run("a Submit creates a queued Execution with its Steps", func(t *testing.T) {
		s := newStore(t)
		c, _ := build(t, s)
		got, replayed, err := c.Submit(ctx, coordinator.SubmitRequest{Key: "k1", Steps: steps})
		if err != nil || replayed {
			t.Fatalf("got replayed=%v, %v", replayed, err)
		}
		e, ss, err := s.Get(ctx, got.Execution.ID)
		if err != nil {
			t.Fatal(err)
		}
		if e.State != domain.ExecQueued || len(ss) != 2 {
			t.Fatalf("got %s with %d steps", e.State, len(ss))
		}
		for i, st := range ss {
			if st.State != domain.StepQueued || st.Capability != steps[i].Capability || st.Idempotent != steps[i].Idempotent {
				t.Fatalf("step %d: %+v", i, st)
			}
		}
		if string(ss[1].InputsJSON) != string(steps[1].Inputs) {
			t.Fatalf("inputs %q", ss[1].InputsJSON)
		}
	})

	t.Run("the same key and request returns the original result", func(t *testing.T) {
		s := newStore(t)
		c, made := build(t, s)
		first, _, err := c.Submit(ctx, coordinator.SubmitRequest{Key: "k1", Steps: steps})
		if err != nil {
			t.Fatal(err)
		}
		second, replayed, err := c.Submit(ctx, coordinator.SubmitRequest{Key: "k1", Steps: steps})
		if err != nil || !replayed {
			t.Fatalf("got replayed=%v, %v", replayed, err)
		}
		if second.Execution.ID != first.Execution.ID || len(second.Steps) != len(first.Steps) || !second.Execution.CreatedAt.Equal(first.Execution.CreatedAt) {
			t.Fatalf("got %+v, want %+v", second, first)
		}
		open, _ := s.Open(ctx)
		if len(open) != 1 {
			t.Fatalf("a repeated command created %d Executions", len(open))
		}
		// A replay is answered from the record, so it does not even draw a new id.
		if made.Load() != 1 {
			t.Fatalf("ids drawn: %d", made.Load())
		}
	})

	t.Run("a replay returns the original outcome, not the state since", func(t *testing.T) {
		s := newStore(t)
		c, _ := build(t, s)
		first, _, _ := c.Submit(ctx, coordinator.SubmitRequest{Key: "k1", Steps: steps})
		e, _, _ := s.Get(ctx, first.Execution.ID)
		if err := e.Finish(clock(), domain.ExecCancelled, nil); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Commit(ctx, ports.ExecutionChange{Execution: &e}); err != nil {
			t.Fatal(err)
		}
		again, replayed, err := c.Submit(ctx, coordinator.SubmitRequest{Key: "k1", Steps: steps})
		if err != nil || !replayed || again.Execution.State != domain.ExecQueued {
			t.Fatalf("got %+v replayed=%v, %v", again.Execution, replayed, err)
		}
	})

	t.Run("a different key is a different command", func(t *testing.T) {
		s := newStore(t)
		c, _ := build(t, s)
		a, _, _ := c.Submit(ctx, coordinator.SubmitRequest{Key: "k1", Steps: steps})
		b, replayed, err := c.Submit(ctx, coordinator.SubmitRequest{Key: "k2", Steps: steps})
		if err != nil || replayed || a.Execution.ID == b.Execution.ID {
			t.Fatalf("got %+v replayed=%v, %v", b.Execution, replayed, err)
		}
	})

	t.Run("a key reused for another request is refused and changes nothing", func(t *testing.T) {
		s := newStore(t)
		c, _ := build(t, s)
		if _, _, err := c.Submit(ctx, coordinator.SubmitRequest{Key: "k1", Steps: steps}); err != nil {
			t.Fatal(err)
		}
		other := []coordinator.StepSpec{{Name: "build", Capability: "code.lint"}}
		if _, _, err := c.Submit(ctx, coordinator.SubmitRequest{Key: "k1", Steps: other}); !errors.Is(err, domain.ErrKeyReused) {
			t.Fatalf("got %v, want ErrKeyReused", err)
		}
		if open, _ := s.Open(ctx); len(open) != 1 {
			t.Fatalf("%d Executions", len(open))
		}
	})

	t.Run("a bad request is refused before anything is written", func(t *testing.T) {
		s := newStore(t)
		c, _ := build(t, s)
		for name, req := range map[string]coordinator.SubmitRequest{
			"no key":          {Steps: steps},
			"no steps":        {Key: "k"},
			"no capability":   {Key: "k", Steps: []coordinator.StepSpec{{Name: "x"}}},
			"no name":         {Key: "k", Steps: []coordinator.StepSpec{{Capability: "c"}}},
			"duplicate names": {Key: "k", Steps: []coordinator.StepSpec{{Name: "x", Capability: "c"}, {Name: "x", Capability: "c"}}},
		} {
			if _, _, err := c.Submit(ctx, req); !errors.Is(err, domain.ErrInvalid) {
				t.Fatalf("%s: got %v, want ErrInvalid", name, err)
			}
		}
		if open, _ := s.Open(ctx); len(open) != 0 {
			t.Fatalf("%d Executions", len(open))
		}
	})

	t.Run("the answer survives a restart", func(t *testing.T) {
		s := newStore(t)
		c, _ := build(t, s)
		first, _, err := c.Submit(ctx, coordinator.SubmitRequest{Key: "k1", Steps: steps})
		if err != nil {
			t.Fatal(err)
		}
		// The caller never saw the answer, the Platform restarted, the caller retries.
		restarted, _ := build(t, reopen(t, s))
		again, replayed, err := restarted.Submit(ctx, coordinator.SubmitRequest{Key: "k1", Steps: steps})
		if err != nil || !replayed || again.Execution.ID != first.Execution.ID {
			t.Fatalf("got %+v replayed=%v, %v", again.Execution, replayed, err)
		}
		if open, _ := reopen(t, s).Open(ctx); len(open) != 1 {
			t.Fatalf("%d Executions", len(open))
		}
	})

	t.Run("concurrent requests with one key create one Execution", func(t *testing.T) {
		s := newStore(t)
		c, _ := build(t, s)
		const n = 16
		var wg sync.WaitGroup
		ids := make([]string, n)
		fresh := make([]bool, n)
		errs := make([]error, n)
		for i := range n {
			wg.Add(1)
			go func() {
				defer wg.Done()
				got, replayed, err := c.Submit(ctx, coordinator.SubmitRequest{Key: "race", Steps: steps})
				ids[i], fresh[i], errs[i] = got.Execution.ID, !replayed, err
			}()
		}
		wg.Wait()
		created := 0
		for i := range n {
			if errs[i] != nil {
				t.Fatalf("request %d: %v", i, errs[i])
			}
			if ids[i] != ids[0] {
				t.Fatalf("request %d got %s, request 0 got %s", i, ids[i], ids[0])
			}
			if fresh[i] {
				created++
			}
		}
		if created != 1 {
			t.Fatalf("%d requests created the Execution, want 1", created)
		}
		if open, _ := s.Open(ctx); len(open) != 1 {
			t.Fatalf("%d Executions", len(open))
		}
	})
}
