package evidence

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
	"github.com/rluders/amaru/services/platform/internal/ports/fakes"
)

// Embed unused methods so an unexpected store call fails the test instead of returning data.
type executionRead struct {
	ports.ExecutionStore
	id         string
	fail       string
	err        error
	auditCalls []int64
	audit      func(int64, int) ([]domain.AuditEvent, error)
}

func (s *executionRead) Get(context.Context, string) (domain.Execution, []domain.StepExecution, error) {
	if s.fail == "execution" {
		return domain.Execution{}, nil, s.err
	}
	return domain.Execution{ID: s.id, State: domain.ExecRunning}, []domain.StepExecution{{ID: "s1", ExecutionID: s.id}, {ID: "s2", ExecutionID: s.id}}, nil
}
func (s *executionRead) Events(context.Context, string) ([]domain.ExecutionEvent, error) {
	if s.fail == "events" {
		return nil, s.err
	}
	return nil, nil
}
func (s *executionRead) Approvals(context.Context, string) ([]domain.Approval, error) {
	if s.fail == "approvals" {
		return nil, s.err
	}
	return []domain.Approval{{ID: "a1", Gate: domain.ApprovalGate{ExecutionID: s.id, StepID: "s1"}}}, nil
}
func (s *executionRead) Audit(_ context.Context, after int64, limit int) ([]domain.AuditEvent, error) {
	s.auditCalls = append(s.auditCalls, after)
	if s.fail == "audit" {
		return nil, s.err
	}
	if s.audit != nil {
		return s.audit(after, limit)
	}
	return nil, nil
}

type contextRead struct {
	ports.ContextStore
	get func(string, string) (domain.StepContext, error)
}

func (s contextRead) Get(_ context.Context, e, st string) (domain.StepContext, error) {
	return s.get(e, st)
}

type artifactRead struct {
	ports.ArtifactIndex
	err error
}

func (s artifactRead) ByExecution(context.Context, string) ([]domain.ArtifactVersion, error) {
	return nil, s.err
}

func readerFixture() (Reader, *executionRead) {
	s := &executionRead{id: "e1"}
	return Reader{Executions: s, Contexts: fakes.NewContextStore(), Artifacts: fakes.NewArtifactIndex()}, s
}

func assertNoBundle(t *testing.T, b domain.ExecutionEvidence, err, want error) {
	t.Helper()
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
	if !reflect.DeepEqual(b, domain.ExecutionEvidence{}) {
		t.Fatal("failure returned a partial bundle")
	}
}

func TestBundleRejectsMismatchedStoreIdentity(t *testing.T) {
	t.Run("execution", func(t *testing.T) {
		r, s := readerFixture()
		s.id = "other-execution"
		b, err := r.Bundle(context.Background(), "e1")
		assertNoBundle(t, b, err, domain.ErrInvalid)
	})
	for _, tt := range []struct{ name, e, st string }{
		{"context execution", "other-execution", "s1"},
		{"context step", "e1", "s2"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r, _ := readerFixture()
			r.Contexts = contextRead{get: func(e, st string) (domain.StepContext, error) {
				if st == "s1" {
					return domain.StepContext{ExecutionID: tt.e, StepID: tt.st}, nil
				}
				return domain.StepContext{}, domain.ErrNotFound
			}}
			b, err := r.Bundle(context.Background(), "e1")
			assertNoBundle(t, b, err, domain.ErrInvalid)
		})
	}
}

func TestBundleStoreFailureReturnsNoPartialEvidence(t *testing.T) {
	for _, stage := range []string{"execution", "events", "approvals", "audit", "context", "artifacts"} {
		for _, cause := range []error{errors.New("store unavailable"), context.Canceled} {
			t.Run(stage+"/"+cause.Error(), func(t *testing.T) {
				r, s := readerFixture()
				s.fail = stage
				s.err = fmt.Errorf("read failed: %w", cause)
				if stage == "context" {
					r.Contexts = contextRead{get: func(string, string) (domain.StepContext, error) { return domain.StepContext{}, s.err }}
				}
				if stage == "artifacts" {
					r.Artifacts = artifactRead{err: s.err}
				}
				b, err := r.Bundle(context.Background(), "e1")
				assertNoBundle(t, b, err, cause)
			})
		}
	}
}

func TestBundleAuditPaginationFiltersEveryPage(t *testing.T) {
	r, s := readerFixture()
	s.audit = func(after int64, limit int) ([]domain.AuditEvent, error) {
		if limit != auditPage {
			t.Fatalf("limit = %d", limit)
		}
		switch after {
		case 0:
			page := make([]domain.AuditEvent, auditPage)
			for i := range page {
				page[i] = domain.AuditEvent{Seq: int64(i + 1), TargetType: "approval", TargetID: "unrelated"}
			}
			// A matching ID on a different resource type must also be excluded.
			page[0].TargetType = "grant"
			page[0].TargetID = "a1"
			page[auditPage-1].TargetID = "a1"
			return page, nil
		case auditPage:
			return []domain.AuditEvent{{Seq: auditPage + 1, TargetType: "approval", TargetID: "a1"}}, nil
		default:
			t.Fatalf("unexpected cursor %d", after)
			return nil, nil
		}
	}
	b, err := r.Bundle(context.Background(), "e1")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(s.auditCalls, []int64{0, auditPage}) {
		t.Fatalf("cursors = %v", s.auditCalls)
	}
	if len(b.Audit) != 2 || b.Audit[0].Seq != auditPage || b.Audit[1].Seq != auditPage+1 {
		t.Fatalf("audit = %+v", b.Audit)
	}
	// Failure after a complete first page must not publish a truncated history.
	cause := errors.New("second page unavailable")
	first := s.audit
	s.audit = func(after int64, limit int) ([]domain.AuditEvent, error) {
		if after != 0 {
			return nil, cause
		}
		return first(after, limit)
	}
	b, err = r.Bundle(context.Background(), "e1")
	assertNoBundle(t, b, err, cause)
}
