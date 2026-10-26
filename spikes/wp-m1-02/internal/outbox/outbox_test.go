package outbox_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/rluders/amaru/spikes/wp-m1-02/internal/bus"
	"github.com/rluders/amaru/spikes/wp-m1-02/internal/outbox"
	"github.com/rluders/amaru/spikes/wp-m1-02/internal/pgsup"
)

var pool *pgxpool.Pool

// TestMain starts the bundled, supervised PostgreSQL once. Without
// SPIKE_BUNDLE the tests cannot run and the package fails loudly: a test that
// did not run is not evidence.
func TestMain(m *testing.M) {
	bundle := os.Getenv("SPIKE_BUNDLE")
	if bundle == "" {
		println("SPIKE_BUNDLE not set: build one with `spike bundle -out DIR`")
		os.Exit(2)
	}
	state, _ := os.MkdirTemp("", "spike-outbox-")
	inst, err := pgsup.Start(context.Background(), pgsup.Bundle{Dir: bundle}, state)
	if err != nil {
		println("start postgres:", err.Error())
		os.Exit(2)
	}
	pool, err = pgxpool.New(context.Background(), inst.URL)
	if err == nil {
		_, err = pool.Exec(context.Background(), outbox.Schema)
	}
	if err != nil {
		println("pool/schema:", err.Error())
		os.Exit(2)
	}
	code := m.Run()
	pool.Close()
	inst.Close()
	os.RemoveAll(state)
	os.Exit(code)
}

func reset(t *testing.T) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `truncate outbox, consumer_dedup, effects restart identity`); err != nil {
		t.Fatal(err)
	}
}

func count(t *testing.T, q string, args ...any) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), q, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func enqueue(t *testing.T, subject string) int64 {
	t.Helper()
	ctx := context.Background()
	tx, _ := pool.Begin(ctx)
	id, err := outbox.Enqueue(ctx, tx, subject, []byte("assign"))
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	return id
}

func startBus(t *testing.T, dir string, dup time.Duration) *bus.Embedded {
	t.Helper()
	e, err := bus.Start(context.Background(), dir, dup)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func fetch(t *testing.T, c jetstream.Consumer, n int, wait time.Duration) []jetstream.Msg {
	t.Helper()
	b, err := c.Fetch(n, jetstream.FetchMaxWait(wait))
	if err != nil {
		t.Fatal(err)
	}
	var out []jetstream.Msg
	for m := range b.Messages() {
		out = append(out, m)
	}
	return out
}

func streamMsgs(t *testing.T, e *bus.Embedded) uint64 {
	t.Helper()
	s, err := e.JS.Stream(context.Background(), bus.StreamName)
	if err != nil {
		t.Fatal(err)
	}
	info, err := s.Info(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return info.State.Msgs
}

// Transactional boundary: state and outbox row commit or roll back together.
func TestOutboxIsAtomicWithState(t *testing.T) {
	reset(t)
	ctx := context.Background()
	pool.Exec(ctx, `create table if not exists steps(id int primary key, state text)`)
	pool.Exec(ctx, `truncate steps`)

	tx, _ := pool.Begin(ctx)
	tx.Exec(ctx, `insert into steps values (1,'assigned')`)
	if _, err := outbox.Enqueue(ctx, tx, bus.SubjectBase+".p", []byte("x")); err != nil {
		t.Fatal(err)
	}
	tx.Rollback(ctx)
	if n := count(t, `select count(*) from steps`) + count(t, `select count(*) from outbox`); n != 0 {
		t.Fatalf("rollback left %d rows", n)
	}

	tx, _ = pool.Begin(ctx)
	tx.Exec(ctx, `insert into steps values (1,'assigned')`)
	outbox.Enqueue(ctx, tx, bus.SubjectBase+".p", []byte("x"))
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if count(t, `select count(*) from steps`) != 1 || count(t, `select count(*) from outbox where sent_at is null`) != 1 {
		t.Fatal("commit must leave one step and one unsent outbox row")
	}
	// Nothing was published: the commit does not touch the bus. A crash here is
	// the "between commit and publish" gap; the unsent row is what survives.
}

func TestRelayPublishesWithMsgIDAndMarksSentAfterAck(t *testing.T) {
	reset(t)
	e := startBus(t, t.TempDir(), 2*time.Minute)
	defer e.Stop()
	id := enqueue(t, bus.SubjectBase+".p")
	r := &outbox.Relay{Pool: pool, JS: e.JS}
	if n, err := r.RunOnce(context.Background(), 10); n != 1 || err != nil {
		t.Fatalf("n=%d err=%v", n, err)
	}
	cons, _ := e.Consumer(context.Background(), time.Minute, 5)
	ms := fetch(t, cons, 1, 2*time.Second)
	if len(ms) != 1 || ms[0].Headers().Get("Nats-Msg-Id") != outbox.MsgID(id) {
		t.Fatalf("got %d messages, header %q", len(ms), func() string {
			if len(ms) > 0 {
				return ms[0].Headers().Get("Nats-Msg-Id")
			}
			return ""
		}())
	}
	ms[0].Ack()
	if count(t, `select count(*) from outbox where sent_at is not null`) != 1 {
		t.Fatal("row not marked sent")
	}
}

// Publication failure: the bus is down. The row stays unsent with the error
// recorded; after the bus is back the same row is delivered exactly once.
func TestPublishFailureKeepsRowAndRetries(t *testing.T) {
	reset(t)
	dir := t.TempDir()
	e := startBus(t, dir, 2*time.Minute)
	id := enqueue(t, bus.SubjectBase+".p")
	r := &outbox.Relay{Pool: pool, JS: e.JS}
	e.Stop()
	_, err := r.RunOnce(context.Background(), 10)
	if err == nil {
		t.Fatal("publish to a stopped server must fail")
	}
	if count(t, `select count(*) from outbox where id=$1 and sent_at is null and attempts=1 and last_error is not null`, id) != 1 {
		t.Fatal("failed row must stay unsent with attempts=1 and last_error set")
	}
	t.Logf("publish failure surfaced as: %v", err)

	e = startBus(t, dir, 2*time.Minute)
	defer e.Stop()
	r.JS = e.JS
	if n, err := r.RunOnce(context.Background(), 10); n != 1 || err != nil {
		t.Fatalf("retry: n=%d err=%v", n, err)
	}
	if streamMsgs(t, e) != 1 {
		t.Fatal("exactly one message expected after retry")
	}
}

// Acknowledgement failure: JetStream stored the message but the relay never
// marks the row sent (crash or lost response). The next run republishes the
// same Nats-Msg-Id inside the window and JetStream reports a duplicate.
func TestAckLostIsDeduplicatedInsideWindow(t *testing.T) {
	reset(t)
	e := startBus(t, t.TempDir(), 2*time.Minute)
	defer e.Stop()
	enqueue(t, bus.SubjectBase+".p")
	lost := errors.New("simulated: crash between JetStream ack and mark-sent")
	r := &outbox.Relay{Pool: pool, JS: e.JS, Hooks: outbox.Hooks{AfterPublish: func(int64, bool) error { return lost }}}
	if _, err := r.RunOnce(context.Background(), 10); !errors.Is(err, lost) {
		t.Fatalf("want simulated failure, got %v", err)
	}
	if count(t, `select count(*) from outbox where sent_at is null`) != 1 {
		t.Fatal("row must still be unsent")
	}
	r.Hooks = outbox.Hooks{}
	if n, err := r.RunOnce(context.Background(), 10); n != 1 || err != nil {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if r.Stats.Duplicates != 1 {
		t.Fatalf("second publish must be reported Duplicate, stats=%+v", r.Stats)
	}
	if streamMsgs(t, e) != 1 {
		t.Fatal("stream must hold one message")
	}
}

// Consumer crash between effect commit and ack, then server restart: the
// message is redelivered; the PostgreSQL key makes the effect happen once.
func TestRedeliveryAfterRestartIsIdempotent(t *testing.T) {
	reset(t)
	dir := t.TempDir()
	e := startBus(t, dir, 2*time.Minute)
	enqueue(t, bus.SubjectBase+".p")
	r := &outbox.Relay{Pool: pool, JS: e.JS}
	r.RunOnce(context.Background(), 10)
	cons, _ := e.Consumer(context.Background(), 500*time.Millisecond, 5)
	crash := errors.New("simulated: crash before ack")
	a := &outbox.Applier{Pool: pool, BeforeAck: func(string) error { return crash }}
	ms := fetch(t, cons, 1, 2*time.Second)
	if err := a.Handle(context.Background(), ms[0]); !errors.Is(err, crash) {
		t.Fatalf("got %v", err)
	}
	e.Stop() // the whole embedded server restarts from its file store
	e = startBus(t, dir, 2*time.Minute)
	defer e.Stop()
	cons, _ = e.Consumer(context.Background(), 500*time.Millisecond, 5)
	a.BeforeAck = nil
	ms = fetch(t, cons, 1, 3*time.Second)
	if len(ms) != 1 {
		t.Fatalf("expected redelivery after restart, got %d", len(ms))
	}
	md, _ := ms[0].Metadata()
	if md.NumDelivered < 2 {
		t.Fatalf("NumDelivered=%d, want >=2", md.NumDelivered)
	}
	if err := a.Handle(context.Background(), ms[0]); err != nil {
		t.Fatal(err)
	}
	if got := count(t, `select count(*) from effects`); got != 1 || a.Skipped != 1 {
		t.Fatalf("effects=%d skipped=%d, want 1 and 1", got, a.Skipped)
	}
	t.Logf("redelivered with NumDelivered=%d; effect applied once, duplicate skipped by key", md.NumDelivered)
}

// The gap: after the duplicate window the bus accepts the same Nats-Msg-Id
// again. Only the PostgreSQL key keeps the effect single.
func TestDuplicateWindowGap(t *testing.T) {
	reset(t)
	e := startBus(t, t.TempDir(), 2*time.Second)
	defer e.Stop()
	enqueue(t, bus.SubjectBase+".p")
	lost := errors.New("simulated: relay died before mark-sent")
	r := &outbox.Relay{Pool: pool, JS: e.JS, Hooks: outbox.Hooks{AfterPublish: func(int64, bool) error { return lost }}}
	r.RunOnce(context.Background(), 10)
	time.Sleep(3 * time.Second) // relay was down longer than the window
	r.Hooks = outbox.Hooks{}
	r.RunOnce(context.Background(), 10)
	if r.Stats.Duplicates != 0 {
		t.Fatalf("outside the window JetStream must NOT flag a duplicate; stats=%+v", r.Stats)
	}
	if got := streamMsgs(t, e); got != 2 {
		t.Fatalf("stream holds %d messages, want 2 (same Nats-Msg-Id twice)", got)
	}
	cons, _ := e.Consumer(context.Background(), time.Minute, 5)
	ms := fetch(t, cons, 5, 2*time.Second)
	if len(ms) != 2 {
		t.Fatalf("delivered %d", len(ms))
	}
	naive := &outbox.Applier{Pool: pool, Naive: true}
	safe := &outbox.Applier{Pool: pool}
	naive.Handle(context.Background(), ms[0])
	naive.Handle(context.Background(), ms[1])
	naiveEffects := count(t, `select count(*) from effects`)
	reset2 := func() { pool.Exec(context.Background(), `truncate effects, consumer_dedup`) }
	reset2()
	safe.Handle(context.Background(), ms[0])
	safe.Handle(context.Background(), ms[1])
	safeEffects := count(t, `select count(*) from effects`)
	if naiveEffects != 2 || safeEffects != 1 {
		t.Fatalf("naive effects=%d (want 2), keyed effects=%d (want 1)", naiveEffects, safeEffects)
	}
	t.Logf("window gap confirmed: bus-only consumer applied the effect %d times; PostgreSQL-keyed consumer %d time", naiveEffects, safeEffects)
}

// MaxDeliver exhaustion: records what JetStream does with a message nobody
// acks. The authoritative recovery path is PostgreSQL (Lease/reconcile).
func TestMaxDeliverExhaustionLeavesOrphan(t *testing.T) {
	reset(t)
	e := startBus(t, t.TempDir(), 2*time.Minute)
	defer e.Stop()
	enqueue(t, bus.SubjectBase+".p")
	(&outbox.Relay{Pool: pool, JS: e.JS}).RunOnce(context.Background(), 10)
	cons, _ := e.Consumer(context.Background(), 200*time.Millisecond, 2)
	delivered := 0
	for i := 0; i < 4; i++ {
		delivered += len(fetch(t, cons, 1, 600*time.Millisecond))
	}
	info, _ := cons.Info(context.Background())
	t.Logf("deliveries=%d, stream msgs=%d, consumer ack_pending=%d redelivered=%d", delivered, streamMsgs(t, e), info.NumAckPending, info.NumRedelivered)
	if delivered != 2 {
		t.Fatalf("MaxDeliver=2 should allow exactly 2 deliveries, got %d", delivered)
	}
	if streamMsgs(t, e) != 1 {
		t.Fatalf("expected the undeliverable message to remain in the work-queue stream (orphan), msgs=%d", streamMsgs(t, e))
	}
}
