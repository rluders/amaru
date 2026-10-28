//go:build pg

package postgres

// PostgreSQL-backed tests. They run only with `-tags pg` and AMARU_TEST_PG set to the DSN of a
// server whose role may create databases (see internal/store/migrations/runner_pg_test.go). Each
// test uses its own database, migrated with the embedded migrations.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/rluders/amaru/services/platform/internal/coordinator/coordtest"
	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
	"github.com/rluders/amaru/services/platform/internal/ports/porttest"
	"github.com/rluders/amaru/services/platform/internal/store/migrations"
	"github.com/rluders/amaru/services/platform/internal/worker/inproc"
)

var dbSeq atomic.Int64

// migratedDB returns a connection to a new database at the current schema; it is dropped on cleanup.
func migratedDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("AMARU_TEST_PG")
	if dsn == "" {
		t.Skip("AMARU_TEST_PG is not set")
	}
	admin, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { admin.Close() })
	name := fmt.Sprintf("amaru_pg_%d_%d", os.Getpid(), dbSeq.Add(1))
	if _, err := admin.Exec("CREATE DATABASE " + name); err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + name
	db, err := sql.Open("pgx", u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		db.Close()
		_, _ = admin.Exec("DROP DATABASE IF EXISTS " + name + " WITH (FORCE)")
	})
	dir, err := migrations.OpenDir(migrations.Embedded())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := migrations.NewRunner(db, dir, 10*time.Second).Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestProjectStoreConformance(t *testing.T) {
	porttest.ProjectStore(t, func(t *testing.T) ports.ProjectStore { return NewProjectStore(migratedDB(t)) })
}

func TestGrantStoreConformance(t *testing.T) {
	porttest.GrantStore(t, func(t *testing.T) ports.GrantStore { return NewGrantStore(migratedDB(t)) })
}

func TestAuditTrailConformance(t *testing.T) {
	porttest.AuditTrail(t, func(t *testing.T) ports.GrantAuditStore { return NewGrantStore(migratedDB(t)) })
}

func TestAuditLogIsAppendOnlyAndGaplessInTheDatabase(t *testing.T) {
	db := migratedDB(t)
	s := NewGrantStore(db)
	ctx := context.Background()
	mk := func(id string) domain.Grant {
		return domain.Grant{ID: id, Level: domain.GrantOrganization, Subject: domain.Subject{Kind: domain.SubjectAgent, ID: "a"},
			Capabilities: []domain.Capability{"mcp.call"}, GrantedBy: "u1", CreatedAt: time.Date(2026, 10, 28, 9, 0, 0, 0, time.UTC), Status: domain.GrantActive}
	}
	if _, err := s.Create(ctx, mk("g1")); err != nil {
		t.Fatal(err)
	}
	// A refused write rolls back its event and gives its number back: no gap.
	if _, err := s.Create(ctx, mk("g1")); !errors.Is(err, domain.ErrExists) {
		t.Fatalf("duplicate: %v", err)
	}
	if _, err := s.Create(ctx, mk("g2")); err != nil {
		t.Fatal(err)
	}
	evs, err := s.Audit(ctx, 0, 0)
	if err != nil || len(evs) != 2 || evs[0].Seq != 1 || evs[1].Seq != 2 {
		t.Fatalf("events %+v, %v: want seq 1 and 2", evs, err)
	}
	for name, q := range map[string]string{
		"edit":   `UPDATE audit_events SET actor = 'x' WHERE seq = 1`,
		"delete": `DELETE FROM audit_events WHERE seq = 1`,
	} {
		if _, err := db.Exec(q); err == nil {
			t.Errorf("%s of an audit event was allowed", name)
		}
	}
}

func TestAuditSeqHasNoGapsUnderConcurrentWriters(t *testing.T) {
	s := NewGrantStore(migratedDB(t))
	ctx := context.Background()
	const n = 12
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.Create(ctx, domain.Grant{ID: fmt.Sprintf("g%02d", i), Level: domain.GrantOrganization,
				Subject: domain.Subject{Kind: domain.SubjectAgent, ID: "a"}, Capabilities: []domain.Capability{"mcp.call"},
				GrantedBy: "u1", CreatedAt: time.Date(2026, 10, 28, 9, 0, 0, 0, time.UTC), Status: domain.GrantActive})
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	evs, err := s.Audit(ctx, 0, 0)
	if err != nil || len(evs) != n {
		t.Fatalf("%d events, %v; want %d", len(evs), err, n)
	}
	for i, ev := range evs {
		if ev.Seq != int64(i+1) {
			t.Fatalf("event %d has seq %d: not gapless", i, ev.Seq)
		}
	}
}

func TestGrantsAreAppendOnlyInTheDatabase(t *testing.T) {
	db := migratedDB(t)
	s := NewGrantStore(db)
	g := domain.Grant{ID: "g1", Level: domain.GrantOrganization, Subject: domain.Subject{Kind: domain.SubjectAgent, ID: "a"},
		Capabilities: []domain.Capability{"mcp.call"}, GrantedBy: "u1", CreatedAt: time.Date(2026, 10, 28, 9, 0, 0, 0, time.UTC), Status: domain.GrantActive}
	if _, err := s.Create(context.Background(), g); err != nil {
		t.Fatal(err)
	}
	// A direct write that edits what was granted, or deletes the row, is refused by the trigger.
	for name, q := range map[string]string{
		"edit capabilities": `UPDATE grants SET capabilities = '["git.push"]' WHERE id = 'g1'`,
		"edit granted_by":   `UPDATE grants SET granted_by = 'x' WHERE id = 'g1'`,
		"delete":            `DELETE FROM grants WHERE id = 'g1'`,
	} {
		if _, err := db.Exec(q); err == nil {
			t.Errorf("%s was allowed", name)
		}
	}
	if _, err := db.Exec(`UPDATE grants SET status = 'revoked', revoked_by = 'u2', revoked_at = now(), revoke_reason = 'r' WHERE id = 'g1'`); err != nil {
		t.Fatalf("revoking is allowed: %v", err)
	}
	if _, err := db.Exec(`UPDATE grants SET status = 'active', revoked_by = '', revoked_at = NULL WHERE id = 'g1'`); err == nil {
		t.Error("a revoked grant was brought back")
	}
}

func TestExecutionRequesterIsFixedInTheDatabase(t *testing.T) {
	db := migratedDB(t)
	s := NewExecutionStore(db)
	e := domain.Execution{ID: "e1", Version: 1, State: domain.ExecQueued, CreatedAt: time.Now(), RequestedBy: domain.Principal{Kind: domain.PrincipalHuman, ID: "u1", Role: domain.RoleContributor}, Trigger: domain.TriggerManual}
	st := domain.StepExecution{ID: "e1.s", ExecutionID: "e1", Version: 1, State: domain.StepPending, Capability: "code.test"}
	if _, _, err := s.Create(context.Background(), e, []domain.StepExecution{st}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE executions SET requested_by = '{"kind":"human","id":"other"}' WHERE id = 'e1'`); err == nil {
		t.Error("the requester was changed")
	}
	if _, err := db.Exec(`UPDATE executions SET trigger_kind = 'webhook' WHERE id = 'e1'`); err == nil {
		t.Error("the trigger was changed")
	}
}

func TestExecutionStoreConformance(t *testing.T) {
	// reopen builds a second store over the same database: a restarted Platform sees only what was committed.
	var db *sql.DB
	porttest.ExecutionStore(t,
		func(t *testing.T) ports.ExecutionStore { db = migratedDB(t); return NewExecutionStore(db) },
		func(_ *testing.T, _ ports.ExecutionStore) ports.ExecutionStore { return NewExecutionStore(db) })
}

// The crash-injection matrix of the coordinator against the real store: a restart is a new store
// over the same database, so only committed rows survive.
func TestCrashInjectionWithPostgres(t *testing.T) {
	var db *sql.DB
	coordtest.CrashInjection(t,
		func(t *testing.T) ports.ExecutionStore { db = migratedDB(t); return NewExecutionStore(db) },
		func(_ *testing.T, _ ports.ExecutionStore) ports.ExecutionStore { return NewExecutionStore(db) })
}

// The coordinator's Worker-interface contract against the real store, over both kinds of link.
func TestAssignmentWithPostgres(t *testing.T) {
	newStore := func(t *testing.T) ports.ExecutionStore { return NewExecutionStore(migratedDB(t)) }
	coordtest.AssignmentSuite(t, newStore, func(*testing.T) (ports.PlatformLink, ports.WorkerLink) { return inproc.Pair() })
}

// A repeated command returns the original result, also after a restart and under a race, against
// the real store.
func TestCommandsWithPostgres(t *testing.T) {
	var db *sql.DB
	coordtest.CommandSuite(t,
		func(t *testing.T) ports.ExecutionStore { db = migratedDB(t); return NewExecutionStore(db) },
		func(_ *testing.T, _ ports.ExecutionStore) ports.ExecutionStore { return NewExecutionStore(db) })
}

// The pinned definition survives a restart and cannot be updated, also by a direct write.
func TestDefinitionWithPostgres(t *testing.T) {
	var db *sql.DB
	coordtest.DefinitionSuite(t,
		func(t *testing.T) ports.ExecutionStore { db = migratedDB(t); return NewExecutionStore(db) },
		func(_ *testing.T, _ ports.ExecutionStore) ports.ExecutionStore { return NewExecutionStore(db) })
	t.Run("the database refuses to change a pin", func(t *testing.T) {
		db = migratedDB(t)
		s := NewExecutionStore(db)
		pin, err := domain.Pin(domain.Workflow{ID: "ship-it", Name: "Ship it", Status: domain.WorkflowActive,
			Steps: []domain.WorkflowStep{{ID: "build", Name: "Build", Type: domain.StepTypeTool, Capability: "code.build"}}}, "")
		if err != nil {
			t.Fatal(err)
		}
		e := domain.Execution{ID: "e1", State: domain.ExecQueued, CreatedAt: time.Now(), Definition: &pin}
		if _, _, err := s.Create(context.Background(), e, nil); err != nil {
			t.Fatal(err)
		}
		_, err = db.Exec(`UPDATE executions SET definition_hash = 'sha256:x' WHERE id = 'e1'`)
		if err == nil || !strings.Contains(err.Error(), "pinned") {
			t.Fatalf("got %v, want the pin refused", err)
		}
	})
}

// The concurrency limits against the real store, which counts the Steps that hold a slot.
func TestLimitsWithPostgres(t *testing.T) {
	newStore := func(t *testing.T) ports.ExecutionStore { return NewExecutionStore(migratedDB(t)) }
	coordtest.LimitsSuite(t, newStore, func(*testing.T) (ports.PlatformLink, ports.WorkerLink) { return inproc.Pair() })
}

// Per-Step timeouts against the real store: the override is stored and a timed_out Step stays so.
func TestTimeoutsWithPostgres(t *testing.T) {
	var db *sql.DB
	coordtest.TimeoutSuite(t,
		func(t *testing.T) ports.ExecutionStore { db = migratedDB(t); return NewExecutionStore(db) },
		func(*testing.T) (ports.PlatformLink, ports.WorkerLink) { return inproc.Pair() },
		func(_ *testing.T, _ ports.ExecutionStore) ports.ExecutionStore { return NewExecutionStore(db) })
}

// Cancellation against the real store.
func TestCancelWithPostgres(t *testing.T) {
	var db *sql.DB
	coordtest.CancelSuite(t,
		func(t *testing.T) ports.ExecutionStore { db = migratedDB(t); return NewExecutionStore(db) },
		func(*testing.T) (ports.PlatformLink, ports.WorkerLink) { return inproc.Pair() },
		func(_ *testing.T, _ ports.ExecutionStore) ports.ExecutionStore { return NewExecutionStore(db) })
}

// Remote-job recording and re-attachment against the real store.
func TestRemoteJobWithPostgres(t *testing.T) {
	var db *sql.DB
	coordtest.RemoteJobSuite(t,
		func(t *testing.T) ports.ExecutionStore { db = migratedDB(t); return NewExecutionStore(db) },
		func(*testing.T) (ports.PlatformLink, ports.WorkerLink) { return inproc.Pair() },
		func(_ *testing.T, _ ports.ExecutionStore) ports.ExecutionStore { return NewExecutionStore(db) })
}

// Policy against the real stores: Grants and the requester are stored, and a revoked Grant
// stops a queued Step from starting.
func TestPolicyWithPostgres(t *testing.T) {
	coordtest.PolicySuite(t,
		func(t *testing.T) (ports.ExecutionStore, ports.GrantStore) {
			db := migratedDB(t)
			return NewExecutionStore(db), NewGrantStore(db)
		},
		func(*testing.T) (ports.PlatformLink, ports.WorkerLink) { return inproc.Pair() })
}

func TestApprovalStoreConformance(t *testing.T) {
	var db *sql.DB
	porttest.ApprovalStore(t,
		func(t *testing.T) ports.ExecutionStore { db = migratedDB(t); return NewExecutionStore(db) },
		func(_ *testing.T, _ ports.ExecutionStore) ports.ExecutionStore { return NewExecutionStore(db) })
}

// The approval workflow against the real store: a waiting Step and its Approval survive a restart.
func TestApprovalWithPostgres(t *testing.T) {
	var db *sql.DB
	coordtest.ApprovalSuite(t,
		func(t *testing.T) ports.ExecutionStore { db = migratedDB(t); return NewExecutionStore(db) },
		func(*testing.T) (ports.PlatformLink, ports.WorkerLink) { return inproc.Pair() },
		func(_ *testing.T, _ ports.ExecutionStore) ports.ExecutionStore { return NewExecutionStore(db) })
}

func TestApprovalsAreFinalInTheDatabase(t *testing.T) {
	db := migratedDB(t)
	s := NewExecutionStore(db)
	e := domain.Execution{ID: "e1", State: domain.ExecQueued, CreatedAt: time.Date(2026, 10, 28, 9, 0, 0, 0, time.UTC), ProjectID: "p1"}
	st := domain.StepExecution{ID: "e1.s", ExecutionID: "e1", State: domain.StepAwaitingApproval}
	ev, _ := domain.NewEvidence(domain.EvidenceItem{Kind: "execution", ID: "e1"})
	a, err := domain.NewApproval(e.CreatedAt, "a1", "p1", domain.ApprovalGate{Kind: domain.GatePolicyRequired, ExecutionID: "e1", StepID: "e1.s", Capability: "git.push"},
		domain.Principal{Kind: domain.PrincipalHuman, ID: "u", Role: domain.RoleContributor}, ev)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.CreateOnce(context.Background(), domain.CommandRecord{Name: "execution.submit", Key: "k", Fingerprint: "f", Result: []byte(`{}`), At: e.CreatedAt}, e, []domain.StepExecution{st}, a); err != nil {
		t.Fatal(err)
	}
	for name, q := range map[string]string{
		"edit the capability":        `UPDATE approvals SET capability = 'deploy.run' WHERE id = 'a1'`,
		"edit the evidence":          `UPDATE approvals SET evidence_digest = 'sha256:x' WHERE id = 'a1'`,
		"edit the requester":         `UPDATE approvals SET requester = '{}' WHERE id = 'a1'`,
		"delete":                     `DELETE FROM approvals WHERE id = 'a1'`,
		"approve without a decision": `UPDATE approvals SET state = 'approved' WHERE id = 'a1'`,
	} {
		if _, err := db.Exec(q); err == nil {
			t.Errorf("%s was allowed", name)
		}
	}
	if _, err := db.Exec(`UPDATE approvals SET state = 'approved', decision = '{"outcome":"approved"}' WHERE id = 'a1'`); err != nil {
		t.Fatalf("deciding is allowed: %v", err)
	}
	for name, q := range map[string]string{
		"change a decided approval": `UPDATE approvals SET state = 'rejected', decision = '{"outcome":"rejected"}' WHERE id = 'a1'`,
		"void a decided approval":   `UPDATE approvals SET state = 'voided', decision = NULL, voided_at = now(), void_reason = 'x' WHERE id = 'a1'`,
	} {
		if _, err := db.Exec(q); err == nil {
			t.Errorf("%s was allowed", name)
		}
	}
}
