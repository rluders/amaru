package wf_test

import (
	"context"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rluders/amaru/spikes/wp-m1-02/internal/bus"
	"github.com/rluders/amaru/spikes/wp-m1-02/internal/outbox"
	"github.com/rluders/amaru/spikes/wp-m1-02/internal/pgsup"
	"github.com/rluders/amaru/spikes/wp-m1-02/internal/wf"
)

var pool *pgxpool.Pool

func TestMain(m *testing.M) {
	bundle := os.Getenv("SPIKE_BUNDLE")
	if bundle == "" {
		println("SPIKE_BUNDLE not set: build one with `spike bundle -out DIR`")
		os.Exit(2)
	}
	state, _ := os.MkdirTemp("", "spike-wf-")
	inst, err := pgsup.Start(context.Background(), pgsup.Bundle{Dir: bundle}, state)
	if err != nil {
		println("start postgres:", err.Error())
		os.Exit(2)
	}
	// A small fixed pool makes pool exhaustion (16 concurrent transactions) the
	// same on every machine; the default scales with CPU count and hid a
	// nested-acquire deadlock on a 24-thread host.
	pcfg, err := pgxpool.ParseConfig(inst.URL)
	if err == nil {
		pcfg.MaxConns = 4
		pool, err = pgxpool.NewWithConfig(context.Background(), pcfg)
	}
	if err == nil {
		for _, s := range []string{outbox.Schema, wf.Schema} {
			if _, err = pool.Exec(context.Background(), s); err != nil {
				break
			}
		}
	}
	if err != nil {
		println("schema:", err.Error())
		os.Exit(2)
	}
	code := m.Run()
	pool.Close()
	inst.Close()
	os.RemoveAll(state)
	os.Exit(code)
}

func reset(t *testing.T) wf.Sim {
	t.Helper()
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `truncate events, timers, world, steps, executions, outbox, consumer_dedup, effects, sim_clock restart identity cascade`); err != nil {
		t.Fatal(err)
	}
	s := wf.Sim{Pool: pool}
	if err := s.Init(ctx, 1_800_000_000); err != nil {
		t.Fatal(err)
	}
	return s
}

func coord(sim wf.Sim, worker string, cr wf.Crasher) *wf.Coord {
	return &wf.Coord{Pool: pool, Clock: sim, Crash: cr, WorkerID: worker, LeaseTTL: 30}
}

func stepState(t *testing.T, id string) string {
	t.Helper()
	var s string
	if err := pool.QueryRow(context.Background(), `select state from steps where id=$1`, id).Scan(&s); err != nil {
		t.Fatal(err)
	}
	return s
}

func n(t *testing.T, q string, a ...any) int {
	t.Helper()
	var x int
	if err := pool.QueryRow(context.Background(), q, a...).Scan(&x); err != nil {
		t.Fatal(err)
	}
	return x
}

func assignmentOf(t *testing.T, step string) wf.Assignment {
	t.Helper()
	var v int
	if err := pool.QueryRow(context.Background(), `select version from steps where id=$1`, step).Scan(&v); err != nil {
		t.Fatal(err)
	}
	return wf.Assignment{Step: step, Version: v}
}

var one = []wf.StepDef{{Name: "work", Kind: wf.KindNonIdempotent}}
var oneIdem = []wf.StepDef{{Name: "work", Kind: wf.KindIdempotent}}

// blocker pauses the Worker at a crash point until released.
type blocker struct {
	point   string
	reached chan struct{}
	release chan struct{}
	once    sync.Once
}

func newBlocker(point string) *blocker {
	return &blocker{point: point, reached: make(chan struct{}), release: make(chan struct{})}
}
func (b *blocker) Hit(p string) {
	if p == b.point {
		b.once.Do(func() { close(b.reached); <-b.release })
	}
}

// Concurrency: many coordinators dispatching at once lease each Step once.
func TestConcurrentDispatchAssignsEachStepOnce(t *testing.T) {
	sim := reset(t)
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		id := "x" + string(rune('a'+i))
		pool.Exec(ctx, `insert into executions(id,state) values ($1,'running')`, id)
		pool.Exec(ctx, `insert into steps(id,exec_id,ord,name,kind,state) values ($1,$2,0,'work','idempotent','pending')`, id+"/work", id)
	}
	var wg sync.WaitGroup
	for g := 0; g < 16; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			c := coord(sim, "c"+string(rune('a'+g)), nil)
			for k := 0; k < 5; k++ {
				if err := c.Dispatch(ctx); err != nil {
					t.Error(err)
				}
			}
		}(g)
	}
	wg.Wait()
	if got := n(t, `select count(*) from steps where state='assigned' and version=1`); got != 5 {
		t.Fatalf("assigned once: %d of 5", got)
	}
	if got := n(t, `select count(*) from outbox`); got != 5 {
		t.Fatalf("outbox rows=%d, want 5 (one per Step)", got)
	}
}

// Duplicate and out-of-order reports are rejected by compare-and-set.
func TestDuplicateAndOutOfOrderReports(t *testing.T) {
	sim := reset(t)
	ctx := context.Background()
	c := coord(sim, "w1", nil)
	if err := c.StartExecution(ctx, "e1", one); err != nil {
		t.Fatal(err)
	}
	a := assignmentOf(t, "e1/work")

	if ok, _ := c.Complete(ctx, a.Step, a.Version+1, true, ""); ok {
		t.Fatal("a result for a Step that never started must be rejected (out of order)")
	}
	if ok, err := c.Execute(ctx, a); !ok || err != nil {
		t.Fatalf("first delivery must run: ok=%v err=%v", ok, err)
	}
	if ok, _ := c.Execute(ctx, a); ok {
		t.Fatal("duplicate delivery of the same assignment must not run again")
	}
	if ok, _ := c.Complete(ctx, a.Step, a.Version+1, true, ""); ok {
		t.Fatal("duplicate result must be rejected")
	}
	if got := n(t, `select count(*) from world`); got != 1 {
		t.Fatalf("effects=%d, want 1", got)
	}
	if got := n(t, `select count(*) from events where kind='step.succeeded'`); got != 1 {
		t.Fatalf("succeeded events=%d, want 1", got)
	}
}

// A Worker that loses its lease and wakes up later is fenced.
func TestZombieWorkerResultIsFenced(t *testing.T) {
	for _, tc := range []struct {
		name      string
		defs      []wf.StepDef
		wantState string
		wantFx    int
	}{
		{"idempotent re-leased and completed by B", oneIdem, wf.Succeeded, 1},
		{"non-idempotent interrupted, late result discarded", one, wf.Interrupted, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sim := reset(t)
			ctx := context.Background()
			bl := newBlocker("worker.after_effect")
			a := coord(sim, "A", bl)
			b := coord(sim, "B", nil)
			a.StartExecution(ctx, "e1", tc.defs)
			as := assignmentOf(t, "e1/work")
			done := make(chan bool, 1)
			go func() { ok, _ := a.Execute(ctx, as); done <- ok }()
			<-bl.reached // A ran the effect and stalls before reporting
			sim.Advance(ctx, 29)
			b.ExpireLeases(ctx)
			if s := stepState(t, "e1/work"); s != wf.Running {
				t.Fatalf("lease not yet expired at TTL-1 but state=%s", s)
			}
			sim.Advance(ctx, 1) // exactly at lease_expires_at
			if err := b.ExpireLeases(ctx); err != nil {
				t.Fatal(err)
			}
			if tc.defs[0].Kind == wf.KindIdempotent {
				if ok, err := b.Execute(ctx, assignmentOf(t, "e1/work")); !ok || err != nil {
					t.Fatalf("B must run the re-leased Step: ok=%v err=%v", ok, err)
				}
			}
			close(bl.release)
			if ok := <-done; ok {
				t.Fatal("zombie result must be fenced (rejected)")
			}
			if s := stepState(t, "e1/work"); s != tc.wantState {
				t.Fatalf("state=%s want %s", s, tc.wantState)
			}
			if got := n(t, `select count(*) from world`); got != tc.wantFx {
				t.Fatalf("effects=%d want %d", got, tc.wantFx)
			}
			if got := n(t, `select count(*) from events where kind='step.succeeded'`); tc.wantState == wf.Succeeded && got != 1 {
				t.Fatalf("succeeded events=%d want 1", got)
			}
		})
	}
}

// A timer for a Step that already moved on does nothing.
func TestStaleTimerIsIgnored(t *testing.T) {
	sim := reset(t)
	ctx := context.Background()
	c := coord(sim, "w1", nil)
	c.StartExecution(ctx, "e1", []wf.StepDef{{Name: "work", Kind: wf.KindIdempotent, TimeoutS: 60}})
	c.Execute(ctx, assignmentOf(t, "e1/work"))
	sim.Advance(ctx, 100)
	if err := c.FireTimers(ctx); err != nil {
		t.Fatal(err)
	}
	if s := stepState(t, "e1/work"); s != wf.Succeeded {
		t.Fatalf("state=%s: a finished Step must not time out", s)
	}
	if n(t, `select count(*) from events where kind='timer.step_timeout'`) != 0 {
		t.Fatal("stale timeout must leave no event")
	}
	if n(t, `select count(*) from timers where not fired`) != 0 {
		t.Fatal("the stale timer must be consumed")
	}
}

// Deadline boundary under controlled time: due == now fires, due-1 does not.
func TestApprovalDeadlineBoundary(t *testing.T) {
	sim := reset(t)
	ctx := context.Background()
	c := coord(sim, "w1", nil)
	c.StartExecution(ctx, "e1", []wf.StepDef{{Name: "gate", Kind: wf.KindApproval, ApprovalDeadlineS: 1000}})
	sim.Advance(ctx, 999)
	c.FireTimers(ctx)
	if s := stepState(t, "e1/gate"); s != wf.WaitingApproval {
		t.Fatalf("at deadline-1: %s", s)
	}
	sim.Advance(ctx, 1)
	c.FireTimers(ctx)
	if s := stepState(t, "e1/gate"); s != wf.Failed {
		t.Fatalf("at deadline: %s", s)
	}
	if ok, _ := c.Approve(ctx, "e1/gate", true); ok {
		t.Fatal("approval after expiry must be ignored")
	}
}

// Two relays over the same outbox publish each row once.
func TestTwoRelaysPublishEachRowOnce(t *testing.T) {
	reset(t)
	ctx := context.Background()
	e, err := bus.Start(ctx, t.TempDir(), 2*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	defer e.Stop()
	for i := 0; i < 60; i++ {
		pool.Exec(ctx, `insert into outbox(subject,payload) values ($1,'x')`, bus.SubjectBase+".pool")
	}
	var wg sync.WaitGroup
	relays := []*outbox.Relay{{Pool: pool, JS: e.JS}, {Pool: pool, JS: e.JS}, {Pool: pool, JS: e.JS}}
	for _, r := range relays {
		wg.Add(1)
		go func(r *outbox.Relay) {
			defer wg.Done()
			for k := 0; k < 20; k++ {
				r.RunOnce(ctx, 7)
			}
		}(r)
	}
	wg.Wait()
	if got := n(t, `select count(*) from outbox where sent_at is not null`); got != 60 {
		t.Fatalf("sent=%d", got)
	}
	st, _ := e.JS.Stream(ctx, bus.StreamName)
	info, _ := st.Info(ctx)
	dups := 0
	for _, r := range relays {
		dups += r.Stats.Duplicates
	}
	if info.State.Msgs != 60 || dups != 0 {
		t.Fatalf("stream msgs=%d duplicates reported=%d; SKIP LOCKED should prevent double publish", info.State.Msgs, dups)
	}
}

// Reconcile after losing the bus store: assigned Step gets its assignment back.
func TestReconcileReenqueuesAssignedStep(t *testing.T) {
	sim := reset(t)
	ctx := context.Background()
	old := coord(sim, "old", nil)
	old.StartExecution(ctx, "e1", oneIdem)
	pool.Exec(ctx, `update outbox set sent_at=now()`) // the message was published, then the store was lost
	fresh := coord(sim, "new", nil)
	if err := fresh.Reconcile(ctx); err != nil {
		t.Fatal(err)
	}
	if got := n(t, `select count(*) from outbox where sent_at is null`); got != 1 {
		t.Fatalf("unsent=%d, want 1 re-enqueued assignment", got)
	}
	fresh.Reconcile(ctx)
	if got := n(t, `select count(*) from outbox where sent_at is null`); got != 1 {
		t.Fatalf("a second reconcile must not pile up copies, unsent=%d", got)
	}
	if !strings.Contains(stepState(t, "e1/work"), wf.Assigned) {
		t.Fatal("state must stay assigned")
	}
}
