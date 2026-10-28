package porttest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
)

// ContextStore runs the context-Evidence contract (FEAT-SKL-002) against newStore, which returns an
// empty store each call, and a function that creates a Step so a record can refer to it (PostgreSQL
// keeps a foreign key; the fake needs nothing).
func ContextStore(t *testing.T, newStore func(t *testing.T) (ports.ContextStore, func(execID, stepID string))) {
	ctx := context.Background()
	at := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	mk := func(t *testing.T, exec, step, intent string) domain.StepContext {
		t.Helper()
		a, err := domain.AssembleContext(domain.ContextInput{Intent: intent})
		if err != nil {
			t.Fatal(err)
		}
		return domain.StepContext{ExecutionID: exec, StepID: step, Format: domain.ContextFormat, Hash: a.Hash, Document: a.Document, Items: a.Items, CreatedAt: at}
	}

	t.Run("a context is stored and read back whole", func(t *testing.T) {
		s, step := newStore(t)
		step("e1", "e1.s1")
		want := mk(t, "e1", "e1.s1", "fix the test")
		got, err := s.Record(ctx, want)
		if err != nil || got.Hash != want.Hash {
			t.Fatalf("%+v %v", got, err)
		}
		read, err := s.Get(ctx, "e1", "e1.s1")
		if err != nil || read.Document != want.Document || read.Hash != want.Hash || len(read.Items) != 1 || read.Items[0].Kind != domain.ContextIntent || !read.CreatedAt.Equal(at) {
			t.Fatalf("%+v %v", read, err)
		}
		if _, err := s.Get(ctx, "e1", "nope"); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("missing: %v", err)
		}
	})

	t.Run("the context of a Step never changes", func(t *testing.T) {
		s, step := newStore(t)
		step("e1", "e1.s1")
		first, _ := s.Record(ctx, mk(t, "e1", "e1.s1", "one"))
		again := mk(t, "e1", "e1.s1", "one")
		again.CreatedAt = at.Add(time.Hour)
		got, err := s.Record(ctx, again)
		if err != nil || !got.CreatedAt.Equal(first.CreatedAt) {
			t.Fatalf("the same document again: %+v %v", got, err)
		}
		if _, err := s.Record(ctx, mk(t, "e1", "e1.s1", "two")); !errors.Is(err, domain.ErrConflict) {
			t.Fatalf("another document: %v", err)
		}
		if read, _ := s.Get(ctx, "e1", "e1.s1"); read.Hash != first.Hash {
			t.Fatalf("the stored context changed: %+v", read)
		}
	})

	t.Run("a record whose hash is not its document's is refused", func(t *testing.T) {
		s, step := newStore(t)
		step("e1", "e1.s1")
		bad := mk(t, "e1", "e1.s1", "one")
		bad.Document += "tampered"
		if _, err := s.Record(ctx, bad); !errors.Is(err, domain.ErrInvalid) {
			t.Fatalf("got %v", err)
		}
		if _, err := s.Get(ctx, "e1", "e1.s1"); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("a refused record was stored: %v", err)
		}
	})
}
