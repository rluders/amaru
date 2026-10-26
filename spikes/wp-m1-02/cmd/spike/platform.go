package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/rluders/amaru/spikes/wp-m1-02/internal/bus"
	"github.com/rluders/amaru/spikes/wp-m1-02/internal/outbox"
	"github.com/rluders/amaru/spikes/wp-m1-02/internal/pgsup"
	"github.com/rluders/amaru/spikes/wp-m1-02/internal/wf"
)

// flows are the representative Workflows: prep -> work -> deliver, where the
// middle Step is the kind under test.
var flows = map[string][]wf.StepDef{
	"idem":      {{Name: "prep", Kind: wf.KindIdempotent}, {Name: "work", Kind: wf.KindIdempotent}, {Name: "deliver", Kind: wf.KindIdempotent}},
	"nonidem":   {{Name: "prep", Kind: wf.KindIdempotent}, {Name: "work", Kind: wf.KindNonIdempotent}, {Name: "deliver", Kind: wf.KindIdempotent}},
	"resumable": {{Name: "prep", Kind: wf.KindIdempotent}, {Name: "work", Kind: wf.KindResumable}, {Name: "deliver", Kind: wf.KindIdempotent}},
	"approval":  {{Name: "prep", Kind: wf.KindIdempotent}, {Name: "gate", Kind: wf.KindApproval, ApprovalDeadlineS: 5 * 86400}, {Name: "deliver", Kind: wf.KindIdempotent}},
	"flaky":     {{Name: "prep", Kind: wf.KindIdempotent}, {Name: "work", Kind: wf.KindIdempotent, FailFirst: true}, {Name: "deliver", Kind: wf.KindIdempotent}},
	"timeout":   {{Name: "prep", Kind: wf.KindIdempotent}, {Name: "work", Kind: wf.KindNonIdempotent, TimeoutS: 60}, {Name: "deliver", Kind: wf.KindIdempotent}},
}

// cmdPlatform is one run of the Platform process: bundled PostgreSQL, embedded
// JetStream, coordinator, relay and embedded Worker. It exits when the work is
// quiescent, or earlier if a crash point kills it.
func cmdPlatform(args []string) error {
	fl := flag.NewFlagSet("platform", flag.ExitOnError)
	bundle := fl.String("bundle", "", "bundle dir")
	state := fl.String("state", "", "state dir (persistent across runs)")
	flow := fl.String("flow", "", "start this Workflow (idempotent if already started)")
	crash := fl.String("crash", "", "crash point spec, see wf.ParseCrash")
	advance := fl.Int64("advance", 0, "advance the simulated clock by N seconds at start")
	advAfter := fl.Int("advance-after-ms", 0, "advance the clock by -advance-late seconds this long after start")
	advLate := fl.Int64("advance-late", 0, "seconds for -advance-after-ms")
	approve := fl.String("approve", "", "approve this gate Step")
	reject := fl.String("reject", "", "reject this gate Step")
	resume := fl.String("resume", "", "human resume of this interrupted Step")
	leaseTTL := fl.Int64("lease-ttl", 30, "lease seconds (simulated clock)")
	wipeNATS := fl.Bool("wipe-nats", false, "delete the JetStream store before start (store loss)")
	maxMS := fl.Int("max-run-ms", 30000, "give up after this long")
	hold := fl.Int("hold-ms", 0, "keep running at least this long even if quiescent")
	fl.Parse(args)

	ctx := context.Background()
	inst, err := pgsup.Start(ctx, pgsup.Bundle{Dir: *bundle}, *state)
	if err != nil {
		return err
	}
	defer inst.Close()
	fmt.Fprintf(os.Stderr, "POSTGRES start_ms=%d waited_for_previous_postmaster_ms=%d\n", inst.StartedIn.Milliseconds(), inst.WaitedPrevious.Milliseconds())
	pool, err := pgxpool.New(ctx, inst.URL)
	if err != nil {
		return err
	}
	defer pool.Close()
	for _, s := range []string{outbox.Schema, wf.Schema} {
		if _, err := pool.Exec(ctx, s); err != nil {
			return err
		}
	}
	sim := wf.Sim{Pool: pool}
	if err := sim.Init(ctx, 1_800_000_000); err != nil {
		return err
	}
	if *advance > 0 {
		if err := sim.Advance(ctx, *advance); err != nil {
			return err
		}
	}
	store := filepath.Join(*state, "jetstream")
	if *wipeNATS {
		os.RemoveAll(store)
	}
	e, err := bus.Start(ctx, store, 2*time.Minute)
	if err != nil {
		return err
	}
	defer e.Stop()
	cons, err := e.Consumer(ctx, 500*time.Millisecond, 20)
	if err != nil {
		return err
	}
	cr := wf.ParseCrash(*crash)
	co := &wf.Coord{Pool: pool, Clock: sim, Crash: cr, WorkerID: fmt.Sprintf("w-%d", os.Getpid()), LeaseTTL: *leaseTTL}
	relay := &outbox.Relay{Pool: pool, JS: e.JS, Hooks: outbox.Hooks{
		BeforePublish: func(int64) error { cr.Hit("relay.before_publish"); return nil },
		AfterPublish:  func(int64, bool) error { cr.Hit("relay.after_publish"); return nil },
	}}

	if err := co.Reconcile(ctx); err != nil {
		return err
	}
	if *flow != "" {
		defs, ok := flows[*flow]
		if !ok {
			return fmt.Errorf("unknown flow %q", *flow)
		}
		if err := co.StartExecution(ctx, "e1", defs); err != nil {
			return err
		}
	}
	if *approve != "" {
		if _, err := co.Approve(ctx, *approve, true); err != nil {
			return err
		}
	}
	if *reject != "" {
		if _, err := co.Approve(ctx, *reject, false); err != nil {
			return err
		}
	}
	if *resume != "" {
		if _, err := co.Resume(ctx, *resume); err != nil {
			return err
		}
	}

	go workerLoop(ctx, co, cons, cr)
	if *advAfter > 0 {
		time.AfterFunc(time.Duration(*advAfter)*time.Millisecond, func() { sim.Advance(ctx, *advLate) })
	}

	start := time.Now()
	stable := 0
	for time.Since(start) < time.Duration(*maxMS)*time.Millisecond {
		if _, err := relay.RunOnce(ctx, 20); err != nil {
			fmt.Fprintln(os.Stderr, "relay:", err)
		}
		for _, f := range []func(context.Context) error{co.ExpireLeases, co.FireTimers} {
			if err := f(ctx); err != nil {
				return err
			}
		}
		snap, err := co.Snapshot(ctx)
		if err != nil {
			return err
		}
		if quiescent(snap) && time.Since(start) >= time.Duration(*hold)*time.Millisecond {
			stable++
		} else {
			stable = 0
		}
		if stable >= 4 {
			return printSnapshot(snap, false)
		}
		time.Sleep(50 * time.Millisecond)
	}
	snap, _ := co.Snapshot(ctx)
	printSnapshot(snap, true)
	return fmt.Errorf("not quiescent after %dms", *maxMS)
}

// quiescent: nothing is assigned or running, the outbox is drained, and every
// pending Step is blocked behind a Step that is not succeeded (a gate, an
// interrupted Step, a timer wait). Waiting for a human or for time is stable.
func quiescent(s wf.Snapshot) bool {
	if s.Unsent > 0 {
		return false
	}
	for _, st := range s.Steps {
		switch st.State {
		case wf.Assigned, wf.Running:
			return false
		case wf.Pending:
			blocked := false
			for _, o := range s.Steps {
				if o.Ord < st.Ord && o.State != wf.Succeeded {
					blocked = true
				}
			}
			if !blocked {
				return false
			}
		}
	}
	return true
}

func printSnapshot(s wf.Snapshot, timedOut bool) error {
	b, _ := json.Marshal(s)
	prefix := "SNAPSHOT "
	if timedOut {
		prefix = "SNAPSHOT-TIMEOUT "
	}
	fmt.Println(prefix + string(b))
	return nil
}

// workerLoop is the embedded Worker: pull an assignment, run it, ack after the
// result committed.
func workerLoop(ctx context.Context, co *wf.Coord, cons jetstream.Consumer, cr wf.Crasher) {
	for {
		b, err := cons.Fetch(1, jetstream.FetchMaxWait(200*time.Millisecond))
		if err != nil {
			time.Sleep(50 * time.Millisecond)
			continue
		}
		for m := range b.Messages() {
			// one goroutine per assignment: a hung Worker must not block the others
			go func(m jetstream.Msg) {
				cr.Hit("worker.after_fetch")
				var a wf.Assignment
				if err := json.Unmarshal(m.Data(), &a); err != nil {
					m.Term()
					return
				}
				if _, err := co.Execute(ctx, a); err != nil {
					fmt.Fprintln(os.Stderr, "execute:", err)
					return // no ack: redelivery
				}
				cr.Hit("worker.before_ack")
				m.Ack()
			}(m)
		}
	}
}
