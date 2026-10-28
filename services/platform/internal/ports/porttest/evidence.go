package porttest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/evidence"
	"github.com/rluders/amaru/services/platform/internal/ports"
)

// EvidenceStores are the stores one Evidence bundle reads. The three must share data, as the
// Platform's stores do: a Step in Executions is the Step a context or an Artifact refers to.
type EvidenceStores struct {
	Executions ports.ExecutionStore
	Contexts   ports.ContextStore
	Artifacts  ports.ArtifactIndex
}

// ExecutionEvidence runs the Evidence bundle contract (FEAT-ART-002) against newStores, which
// returns empty stores each call: one query holds the Execution, its Steps, log, Approvals with
// their audit entries, contexts and Artifact versions, and nothing of another Execution.
func ExecutionEvidence(t *testing.T, newStores func(t *testing.T) EvidenceStores) {
	ctx := context.Background()
	at := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)

	// run creates an Execution with a plain Step and a Step waiting for approval, a context for the
	// first and an Artifact version for each.
	run := func(t *testing.T, s EvidenceStores, id string) {
		t.Helper()
		e, steps := newExec(id, id+".a", id+".b")
		if err := steps[1].RequireApproval(); err != nil {
			t.Fatal(err)
		}
		rec := domain.CommandRecord{Name: "execution.submit", Key: "k-" + id, Fingerprint: "fp", Result: []byte(`{}`), At: t0}
		if _, _, err := s.Executions.CreateOnce(ctx, rec, e, steps, newApproval(t, id+".ap", id, id+".b")); err != nil {
			t.Fatal(err)
		}
		a, err := domain.AssembleContext(domain.ContextInput{Intent: "work on " + id})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.Contexts.Record(ctx, domain.StepContext{ExecutionID: id, StepID: id + ".a", Format: domain.ContextFormat, Hash: a.Hash, Document: a.Document, Items: a.Items, CreatedAt: at}); err != nil {
			t.Fatal(err)
		}
		for _, st := range []string{id + ".a", id + ".b"} {
			if _, err := s.Artifacts.Record(ctx, domain.ArtifactVersion{ArtifactID: "out-" + st, Hash: domain.ContentHash([]byte(st)), Size: 1, Path: "out.txt",
				ExecutionID: id, StepID: st, AttemptID: "at-" + st, CreatedAt: at}); err != nil {
				t.Fatal(err)
			}
		}
	}

	t.Run("one query returns the Execution with everything recorded for it", func(t *testing.T) {
		s := newStores(t)
		run(t, s, "e1")
		b, err := evidence.Reader{Executions: s.Executions, Contexts: s.Contexts, Artifacts: s.Artifacts}.Bundle(ctx, "e1")
		if err != nil {
			t.Fatal(err)
		}
		if b.Execution.ID != "e1" || len(b.Steps) != 2 || b.Steps[0].ID != "e1.a" || b.Steps[1].ID != "e1.b" {
			t.Fatalf("execution and steps: %+v", b)
		}
		if b.Steps[0].Context == nil || b.Steps[0].Context.Document == "" || b.Steps[1].Context != nil {
			t.Errorf("context belongs to the first Step only: %+v / %+v", b.Steps[0].Context, b.Steps[1].Context)
		}
		for _, st := range b.Steps {
			if len(st.Artifacts) != 1 || st.Artifacts[0].ArtifactID != "out-"+st.ID || st.Artifacts[0].StepID != st.ID {
				t.Errorf("artifacts of %s: %+v", st.ID, st.Artifacts)
			}
		}
		if len(b.Approvals) != 1 || b.Approvals[0].ID != "e1.ap" || b.Approvals[0].Evidence.Digest == "" {
			t.Errorf("approvals: %+v", b.Approvals)
		}
		if len(b.Audit) != 1 || b.Audit[0].Kind != domain.AuditApprovalRequested || b.Audit[0].TargetID != "e1.ap" {
			t.Errorf("audit: %+v", b.Audit)
		}
		if b.Final || b.Digest == "" || len(b.Gaps) == 0 {
			t.Errorf("final %v digest %q gaps %d", b.Final, b.Digest, len(b.Gaps))
		}
	})

	t.Run("nothing of another Execution is in the bundle", func(t *testing.T) {
		s := newStores(t)
		run(t, s, "e1")
		run(t, s, "e2")
		b, err := evidence.Reader{Executions: s.Executions, Contexts: s.Contexts, Artifacts: s.Artifacts}.Bundle(ctx, "e2")
		if err != nil {
			t.Fatal(err)
		}
		if len(b.Steps) != 2 || len(b.Approvals) != 1 || b.Approvals[0].ID != "e2.ap" || len(b.Audit) != 1 || b.Audit[0].TargetID != "e2.ap" {
			t.Fatalf("approvals and audit: %+v", b)
		}
		for _, st := range b.Steps {
			if len(st.Artifacts) != 1 || st.Artifacts[0].ExecutionID != "e2" {
				t.Errorf("artifacts of %s: %+v", st.ID, st.Artifacts)
			}
		}
		if b.Steps[0].Context == nil || b.Steps[0].Context.ExecutionID != "e2" {
			t.Errorf("context: %+v", b.Steps[0].Context)
		}
	})

	t.Run("the same stored data gives the same digest, and a new version changes it", func(t *testing.T) {
		s := newStores(t)
		run(t, s, "e1")
		r := evidence.Reader{Executions: s.Executions, Contexts: s.Contexts, Artifacts: s.Artifacts}
		a, _ := r.Bundle(ctx, "e1")
		b, _ := r.Bundle(ctx, "e1")
		if a.Digest == "" || a.Digest != b.Digest {
			t.Fatalf("digest %q vs %q", a.Digest, b.Digest)
		}
		if _, err := s.Artifacts.Record(ctx, domain.ArtifactVersion{ArtifactID: "out-e1.a", Hash: domain.ContentHash([]byte("v2")), Size: 2, Path: "out.txt",
			ExecutionID: "e1", StepID: "e1.a", AttemptID: "at-2", CreatedAt: at}); err != nil {
			t.Fatal(err)
		}
		c, _ := r.Bundle(ctx, "e1")
		if c.Digest == a.Digest || len(c.Steps[0].Artifacts) != 2 {
			t.Errorf("a new version left the digest at %q", c.Digest)
		}
	})

	t.Run("an Execution with no Approval and no context still has a bundle", func(t *testing.T) {
		s := newStores(t)
		e, steps := newExec("e3", "e3.a")
		if _, _, err := s.Executions.Create(ctx, e, steps); err != nil {
			t.Fatal(err)
		}
		b, err := evidence.Reader{Executions: s.Executions, Contexts: s.Contexts, Artifacts: s.Artifacts}.Bundle(ctx, "e3")
		if err != nil || len(b.Steps) != 1 || b.Steps[0].Context != nil || len(b.Approvals) != 0 || len(b.Audit) != 0 {
			t.Fatalf("%+v %v", b, err)
		}
	})

	t.Run("an unknown Execution is not found, and a reader without a store is refused", func(t *testing.T) {
		s := newStores(t)
		if _, err := (evidence.Reader{Executions: s.Executions, Contexts: s.Contexts, Artifacts: s.Artifacts}).Bundle(ctx, "nope"); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("unknown: %v", err)
		}
		if _, err := (evidence.Reader{Executions: s.Executions, Contexts: s.Contexts}).Bundle(ctx, "nope"); !errors.Is(err, domain.ErrInvalid) {
			t.Errorf("no artifact index: %v", err)
		}
	})
}
