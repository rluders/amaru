//go:build pg

package postgres

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rluders/amaru/services/platform/internal/domain"
)

func seedEvidenceScope(t *testing.T, db *sql.DB) {
	t.Helper()
	at := time.Now().UTC()
	e := domain.Execution{ID: "e", ProjectID: "p", State: domain.ExecRunning, CreatedAt: at}
	st := domain.StepExecution{ID: "s", ExecutionID: "e", State: domain.StepRunning, AttemptID: "attempt", FenceToken: 1}
	if _, _, err := NewExecutionStore(db).Create(context.Background(), e, []domain.StepExecution{st}); err != nil {
		t.Fatal(err)
	}
}

// Wait for the reader's event query to reach our table lock. No sleeps establish
// ordering: PostgreSQL reports the actual lock wait after Get has read its snapshot.
func waitEvidenceLock(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		var waiting bool
		if err := db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE '%FROM execution_events WHERE%')`).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatal("reader never reached event lock", ctx.Err())
		case <-ticker.C:
		}
	}
}

func TestEvidenceReadScopeConcurrentCommit(t *testing.T) {
	db := migratedDB(t)
	seedEvidenceScope(t, db)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	before, err := NewEvidenceReader(db).Bundle(ctx, "e")
	if err != nil {
		t.Fatal(err)
	}
	writer, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Rollback()
	if _, err := writer.ExecContext(ctx, `LOCK TABLE execution_events IN ACCESS EXCLUSIVE MODE`); err != nil {
		t.Fatal(err)
	}
	type result struct {
		b   domain.ExecutionEvidence
		err error
	}
	done := make(chan result, 1)
	go func() { b, err := NewEvidenceReader(db).Bundle(ctx, "e"); done <- result{b, err} }()
	waitEvidenceLock(t, ctx, db)
	// One concurrent commit changes every metadata family after the root read.
	for _, q := range []string{
		`UPDATE executions SET state='succeeded',version=version+1 WHERE id='e'`,
		`UPDATE step_executions SET state='succeeded',version=version+1 WHERE id='s'`,
		`INSERT INTO execution_events(execution_id,seq,kind,step_id,at) VALUES('e',1,'step.succeeded','s',now())`,
		`INSERT INTO step_contexts(execution_id,step_id,format,hash,document,items,created_at) VALUES('e','s','amaru-context/1','sha256:'||encode(sha256(convert_to('doc','UTF8')),'hex'),'doc','[]',now())`,
		`INSERT INTO artifact_versions(artifact_id,version,hash,size,path,execution_id,step_id,attempt_id,created_at) VALUES('out',1,'sha256:'||repeat('a',64),1,'out.txt','e','s','attempt',now())`,
	} {
		if _, err := writer.ExecContext(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	ev, err := domain.NewEvidence(domain.EvidenceItem{Kind: "execution", ID: "e"})
	if err != nil {
		t.Fatal(err)
	}
	ap, err := domain.NewApproval(time.Now().UTC(), "approval", "p", domain.ApprovalGate{Kind: domain.GatePolicyRequired, ExecutionID: "e", StepID: "s", Capability: "git.push"}, domain.Principal{Kind: domain.PrincipalHuman, ID: "human", Role: domain.RoleContributor}, ev)
	if err != nil {
		t.Fatal(err)
	}
	if err := insertApproval(ctx, writer, ap); err != nil {
		t.Fatal(err)
	}
	capture := domain.ResultCapture{ExecutionID: "e", ProjectID: "p", StepID: "s", AttemptID: "attempt", FenceToken: 1, CapturedAt: time.Now().UTC(), Fingerprint: strings.Repeat("a", 64), Reports: []domain.UsageReport{{Provider: "fake"}}}
	if err := insertCapture(ctx, writer, capture); err != nil {
		t.Fatal(err)
	}
	if err := writer.Commit(); err != nil {
		t.Fatal(err)
	}
	got := <-done
	if got.err != nil || !reflect.DeepEqual(got.b, before) {
		t.Fatalf("mixed read: %+v err=%v; before=%+v", got.b, got.err, before)
	}
	after, err := NewEvidenceReader(db).Bundle(ctx, "e")
	if err != nil {
		t.Fatal(err)
	}
	if !after.Final || after.Digest == before.Digest || len(after.Events) != 1 || len(after.Approvals) != 1 || len(after.Audit) != 1 || after.Steps[0].Context == nil || len(after.Steps[0].Artifacts) != 1 || len(after.Steps[0].Usage) != 1 {
		t.Fatalf("next read missed commit: %+v", after)
	}
	if db.Stats().InUse != 0 {
		t.Fatal("read scope leaked connection")
	}
}

func TestEvidenceReadScopeCancellationAndRollback(t *testing.T) {
	db := migratedDB(t)
	seedEvidenceScope(t, db)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	writer, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Rollback()
	if _, err := writer.ExecContext(ctx, `LOCK TABLE execution_events IN ACCESS EXCLUSIVE MODE`); err != nil {
		t.Fatal(err)
	}
	readCtx, stop := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() {
		b, err := NewEvidenceReader(db).Bundle(readCtx, "e")
		if !reflect.DeepEqual(b, domain.ExecutionEvidence{}) {
			done <- errors.New("partial bundle on cancellation")
			return
		}
		done <- err
	}()
	waitEvidenceLock(t, ctx, db)
	stop()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancellation err=%v", err)
		}
	case <-ctx.Done():
		t.Fatal("cancel did not release query")
	}
	if err := writer.Rollback(); err != nil {
		t.Fatal(err)
	}
	// A failed read releases its transaction too, and the reader remains usable.
	db.SetMaxOpenConns(1)
	b, err := NewEvidenceReader(db).Bundle(ctx, "missing")
	if !errors.Is(err, domain.ErrNotFound) || !reflect.DeepEqual(b, domain.ExecutionEvidence{}) {
		t.Fatalf("missing: %+v %v", b, err)
	}
	if _, err := NewEvidenceReader(db).Bundle(ctx, "e"); err != nil {
		t.Fatal(err)
	}
	if db.Stats().InUse != 0 {
		t.Fatal("read scope leaked connection")
	}
}

// A query decorator inserts a committed audit entry precisely between pages.
// The actual SQL and transaction still run against PostgreSQL.
type betweenAuditPages struct {
	metadataQuery
	next  func()
	fired bool
}

func (q *betweenAuditPages) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	if strings.Contains(query, "FROM audit_events") && args[0].(int64) > 0 && !q.fired {
		q.fired = true
		q.next()
	}
	return q.metadataQuery.QueryContext(ctx, query, args...)
}

func TestEvidenceReadScopeAuditPages(t *testing.T) {
	db := migratedDB(t)
	seedEvidenceScope(t, db)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ev, err := domain.NewEvidence(domain.EvidenceItem{Kind: "execution", ID: "e"})
	if err != nil {
		t.Fatal(err)
	}
	ap, err := domain.NewApproval(time.Now().UTC(), "approval", "p", domain.ApprovalGate{Kind: domain.GatePolicyRequired, ExecutionID: "e", StepID: "s", Capability: "git.push"}, domain.Principal{Kind: domain.PrincipalHuman, ID: "human", Role: domain.RoleContributor}, ev)
	if err != nil {
		t.Fatal(err)
	}
	writer, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Rollback()
	if err := insertApproval(ctx, writer, ap); err != nil {
		t.Fatal(err)
	}
	audit, err := domain.ApprovalAuditEvent(nil, ap)
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i < 1000; i++ {
		if err := appendAudit(ctx, writer, audit); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Commit(); err != nil {
		t.Fatal(err)
	}
	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	q := &betweenAuditPages{metadataQuery: tx, next: func() {
		w, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer w.Rollback()
		if err := appendAudit(ctx, w, audit); err != nil {
			t.Fatal(err)
		}
		if err := w.Commit(); err != nil {
			t.Fatal(err)
		}
	}}
	b, err := readEvidence(ctx, q, "e")
	if err != nil {
		t.Fatal(err)
	}
	if !q.fired || len(b.Audit) != 1000 {
		t.Fatalf("mixed audit pages: fired=%v entries=%d", q.fired, len(b.Audit))
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	next, err := NewEvidenceReader(db).Bundle(ctx, "e")
	if err != nil || len(next.Audit) != 1001 {
		t.Fatalf("later audit=%d err=%v", len(next.Audit), err)
	}
}

func TestEvidenceReadScopeStoreFailure(t *testing.T) {
	db := migratedDB(t)
	seedEvidenceScope(t, db)
	db.SetMaxOpenConns(1)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	// Fail after Execution, Step, Event, Approval and context reads have succeeded.
	if _, err := db.ExecContext(ctx, `ALTER TABLE artifact_versions RENAME TO unavailable_artifacts`); err != nil {
		t.Fatal(err)
	}
	b, err := NewEvidenceReader(db).Bundle(ctx, "e")
	if err == nil || !reflect.DeepEqual(b, domain.ExecutionEvidence{}) {
		t.Fatalf("partial success: %+v %v", b, err)
	}
	if _, err := db.ExecContext(ctx, `ALTER TABLE unavailable_artifacts RENAME TO artifact_versions`); err != nil {
		t.Fatal(err)
	}
	if _, err := NewEvidenceReader(db).Bundle(ctx, "e"); err != nil {
		t.Fatal(err)
	}
	if db.Stats().InUse != 0 {
		t.Fatal("failed read leaked connection")
	}
}
