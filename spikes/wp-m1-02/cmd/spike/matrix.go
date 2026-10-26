package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/rluders/amaru/spikes/wp-m1-02/internal/wf"
)

// run is one invocation of the Platform process.
type run struct {
	args      []string
	crash     bool          // the injected crash point must fire (exit 137)
	killAfter time.Duration // the driver kills the process (SIGKILL / TerminateProcess)
}

// exp is the expected final state of a case.
type exp struct {
	exec    string
	steps   map[string]string // step name -> state
	effects map[string]int    // step name -> external side effects
	events  []string          // substrings that must appear in the event log
	absent  []string          // substrings that must not appear
}

type tcase struct {
	id, what string
	runs     []run
	want     exp
}

type matrixResult struct {
	Case   string `json:"case"`
	What   string `json:"what"`
	Pass   bool   `json:"pass"`
	Detail string `json:"detail,omitempty"`
	Runs   int    `json:"runs"`
}

func cmdMatrix(args []string) error {
	fl := flag.NewFlagSet("matrix", flag.ExitOnError)
	bundle := fl.String("bundle", "", "bundle dir")
	work := fl.String("work", "", "work dir for state and logs (created)")
	out := fl.String("out", "", "JSON lines output")
	only := fl.String("only", "", "run only cases whose id contains this")
	fuzz := fl.Int("fuzz", 10, "random external kills per flow")
	fl.Parse(args)
	if *bundle == "" || *work == "" {
		return fmt.Errorf("-bundle and -work required")
	}
	if _, err := os.Stat(*work); err == nil {
		return fmt.Errorf("work dir exists")
	}
	if err := os.MkdirAll(filepath.Join(*work, "logs"), 0o755); err != nil {
		return err
	}
	exe, _ := os.Executable()
	var w io.Writer = os.Stdout
	if *out != "" {
		f, err := os.Create(*out)
		if err != nil {
			return err
		}
		defer f.Close()
		w = io.MultiWriter(os.Stdout, f)
	}
	failed, total := 0, 0
	emit := func(r matrixResult) {
		total++
		if !r.Pass {
			failed++
		}
		b, _ := json.Marshal(r)
		fmt.Fprintln(w, string(b))
	}
	for _, c := range cases() {
		if *only != "" && !strings.Contains(c.id, *only) {
			continue
		}
		snap, detail := execute(exe, *bundle, *work, c.id, c.runs)
		err := check(snap, c.want, detail)
		emit(matrixResult{Case: c.id, What: c.what, Pass: err == nil, Detail: errStr(err), Runs: len(c.runs)})
	}
	if *only == "" || strings.Contains("fuzz", *only) {
		for _, flow := range []string{"idem", "nonidem", "resumable"} {
			for i := 0; i < *fuzz; i++ {
				id := fmt.Sprintf("fuzz-%s-%02d", flow, i)
				seed := int64(1000 + i)
				d := time.Duration(80+rand.New(rand.NewSource(seed)).Intn(900)) * time.Millisecond
				runs := []run{{args: []string{"-flow", flow}, killAfter: d}, {args: []string{"-flow", flow}}}
				snap, detail := execute(exe, *bundle, *work, id, runs)
				err := fuzzCheck(snap, detail, flow)
				emit(matrixResult{Case: id, What: fmt.Sprintf("external kill after %v (seed %d), then restart", d, seed), Pass: err == nil, Detail: errStr(err), Runs: 2})
			}
		}
	}
	fmt.Fprintf(os.Stderr, "cases=%d failed=%d\n", total, failed)
	if failed > 0 {
		return fmt.Errorf("%d case(s) failed", failed)
	}
	return nil
}

// execute runs the sequence on a fresh state dir and returns the last snapshot.
func execute(exe, bundle, work, id string, runs []run) (*wf.Snapshot, string) {
	safe := strings.NewReplacer("/", "_", "#", "@", ":", "_").Replace(id)
	state := filepath.Join(work, safe)
	var snap *wf.Snapshot
	var notes []string
	for i, r := range runs {
		args := append([]string{"platform", "-bundle", bundle, "-state", state}, r.args...)
		cmd := exec.Command(exe, args...)
		var buf bytes.Buffer
		cmd.Stdout, cmd.Stderr = &buf, &buf
		if err := cmd.Start(); err != nil {
			return nil, err.Error()
		}
		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()
		var err error
		if r.killAfter > 0 {
			select {
			case err = <-done:
			case <-time.After(r.killAfter):
				cmd.Process.Kill()
				err = <-done
				notes = append(notes, fmt.Sprintf("run%d killed externally after %v", i+1, r.killAfter))
			}
		} else {
			err = <-done
		}
		os.WriteFile(filepath.Join(work, "logs", fmt.Sprintf("%s.run%d.log", safe, i+1)), buf.Bytes(), 0o644)
		code := 0
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		}
		if r.crash && code != 137 {
			notes = append(notes, fmt.Sprintf("run%d: crash point did not fire (exit %d)", i+1, code))
		}
		if !r.crash && r.killAfter == 0 && code != 0 {
			notes = append(notes, fmt.Sprintf("run%d: unexpected exit %d", i+1, code))
		}
		for _, l := range strings.Split(buf.String(), "\n") {
			if rest, ok := strings.CutPrefix(l, "SNAPSHOT "); ok {
				var s wf.Snapshot
				if json.Unmarshal([]byte(rest), &s) == nil {
					snap = &s
				}
			}
			if strings.HasPrefix(l, "SNAPSHOT-TIMEOUT") {
				notes = append(notes, fmt.Sprintf("run%d: not quiescent", i+1))
			}
		}
	}
	return snap, strings.Join(notes, "; ")
}

func check(s *wf.Snapshot, w exp, notes string) error {
	if notes != "" && !onlyKillNotes(notes) {
		return fmt.Errorf("%s", notes)
	}
	if s == nil {
		return fmt.Errorf("no snapshot")
	}
	var bad []string
	if got := s.Execs["e1"]; got != w.exec {
		bad = append(bad, fmt.Sprintf("execution=%s want %s", got, w.exec))
	}
	for name, st := range w.steps {
		if got := s.Steps["e1/"+name].State; got != st {
			bad = append(bad, fmt.Sprintf("%s=%s want %s", name, got, st))
		}
	}
	for name, n := range w.effects {
		if got := s.Effects["e1/"+name]; got != n {
			bad = append(bad, fmt.Sprintf("effects(%s)=%d want %d", name, got, n))
		}
	}
	log := strings.Join(s.Events, "\n")
	for _, e := range w.events {
		if !strings.Contains(log, e) {
			bad = append(bad, "missing event "+e)
		}
	}
	for _, e := range w.absent {
		if strings.Contains(log, e) {
			bad = append(bad, "unexpected event "+e)
		}
	}
	if !s.SeqOK {
		bad = append(bad, "event seq not contiguous")
	}
	if len(bad) > 0 {
		return fmt.Errorf("%s", strings.Join(bad, "; "))
	}
	return nil
}

// fuzzCheck asserts the ADR-0042 invariants after an arbitrary kill: nothing
// stuck, nothing duplicated, a non-idempotent Step either finished or waits
// for a human.
func fuzzCheck(s *wf.Snapshot, notes, flow string) error {
	if notes != "" && !onlyKillNotes(notes) || s == nil {
		return fmt.Errorf("%s (snapshot=%v)", notes, s != nil)
	}
	var bad []string
	for id, n := range s.Effects {
		max := 1
		if n > max {
			bad = append(bad, fmt.Sprintf("%s executed %d times", id, n))
		}
	}
	for id, st := range s.Steps {
		switch st.State {
		case wf.Pending, wf.Assigned, wf.Running:
			bad = append(bad, fmt.Sprintf("%s stuck in %s", id, st.State))
		}
	}
	e := s.Execs["e1"]
	if flow == "nonidem" {
		if e != "succeeded" && e != "needs_attention" {
			bad = append(bad, "execution="+e)
		}
	} else if e != "succeeded" {
		bad = append(bad, fmt.Sprintf("execution=%s want succeeded (idempotent/resumable work resumes by itself)", e))
	}
	if !s.SeqOK {
		bad = append(bad, "event seq not contiguous")
	}
	if len(bad) > 0 {
		sort.Strings(bad)
		return fmt.Errorf("%s", strings.Join(bad, "; "))
	}
	return nil
}

func cases() []tcase {
	var cs []tcase
	ok := exp{exec: "succeeded", steps: map[string]string{"prep": "succeeded", "work": "succeeded", "deliver": "succeeded"},
		effects: map[string]int{"prep": 1, "work": 1, "deliver": 1}}
	with := func(base exp, f func(*exp)) exp {
		c := base
		c.steps = map[string]string{}
		for k, v := range base.steps {
			c.steps[k] = v
		}
		c.effects = map[string]int{}
		for k, v := range base.effects {
			c.effects[k] = v
		}
		f(&c)
		return c
	}
	interrupted := func(workEffects int) exp {
		return with(ok, func(e *exp) {
			e.exec = "needs_attention"
			e.steps["work"], e.steps["deliver"] = "interrupted", "pending"
			e.effects["work"], e.effects["deliver"] = workEffects, 0
			e.events = []string{"step.interrupted e1/work (platform_restarted)"}
		})
	}
	type point struct{ spec, what string }
	// Points that do not depend on the Step kind: the whole flow must finish
	// with every side effect exactly once.
	for _, p := range []point{
		{"start.before_commit#1", "Execution creation uncommitted (nothing durable)"},
		{"start.after_commit#1", "Execution + first assignment committed, outbox unsent"},
		{"relay.before_publish#2", "work assignment committed, not yet published"},
		{"relay.after_publish#2", "work assignment published, row not marked sent (lost ack)"},
		{"worker.after_fetch#2", "work assignment fetched, nothing recorded"},
		{"running.before_commit#2", "running transition uncommitted"},
		{"complete.after_commit#2", "work result committed, assignment un-acked, next assignment unsent"},
		{"worker.before_ack#2", "work result committed, ack not sent"},
	} {
		for _, flow := range []string{"idem", "nonidem", "resumable"} {
			cs = append(cs, tcase{id: fmt.Sprintf("%s/%s", flow, p.spec), what: p.what,
				runs: []run{{args: []string{"-flow", flow, "-crash", p.spec}, crash: true}, {args: []string{"-flow", flow}}}, want: ok})
		}
	}
	// Points inside the work Step: the outcome depends on the Step kind (ADR-0042).
	type kindcase struct {
		spec, what string
		by         map[string]exp // flow -> expectation after the restart
		resumeFx   map[string]int // flow -> work effects after a human resumes the interrupted Step
	}
	for _, k := range []kindcase{
		{"running.after_commit#2", "work running, side effect not yet performed", map[string]exp{
			"idem": with(ok, func(e *exp) { e.events = []string{"step.retried e1/work (platform_restarted)"} }), "nonidem": interrupted(0), "resumable": interrupted(0)},
			map[string]int{"nonidem": 1, "resumable": 1}},
		{"worker.after_effect#2", "side effect performed, result not recorded", map[string]exp{
			"idem": with(ok, func(e *exp) { e.events = []string{"step.retried e1/work (platform_restarted)"} }), "nonidem": interrupted(1),
			"resumable": with(ok, func(e *exp) { e.events = []string{"step.resumed e1/work (platform_restarted)"} })},
			map[string]int{"nonidem": 2}},
		{"complete.before_commit#2", "result written, transaction uncommitted", map[string]exp{
			"idem": with(ok, func(e *exp) { e.events = []string{"step.retried e1/work (platform_restarted)"} }), "nonidem": interrupted(1),
			"resumable": with(ok, func(e *exp) { e.events = []string{"step.resumed e1/work (platform_restarted)"} })},
			map[string]int{"nonidem": 2}},
	} {
		for _, flow := range []string{"idem", "nonidem", "resumable"} {
			runs := []run{{args: []string{"-flow", flow, "-crash", k.spec}, crash: true}, {args: []string{"-flow", flow}}}
			cs = append(cs, tcase{id: fmt.Sprintf("%s/%s", flow, k.spec), what: k.what, runs: runs, want: k.by[flow]})
			if n, humanDup := k.resumeFx[flow]; humanDup {
				runs2 := append(append([]run{}, runs...), run{args: []string{"-resume", "e1/work"}})
				w := with(ok, func(e *exp) {
					e.effects["work"] = n
					e.events = []string{"step.interrupted e1/work", "step.resumed_by_human e1/work"}
				})
				cs = append(cs, tcase{id: fmt.Sprintf("%s/%s+human-resume", flow, k.spec), what: k.what + "; human resumes the interrupted Step", runs: runs2, want: w})
			}
		}
	}
	cs = append(cs, tcase{id: "resumable/worker.after_job_recorded#1", what: "remote job id recorded, side effect not yet performed",
		runs: []run{{args: []string{"-flow", "resumable", "-crash", "worker.after_job_recorded#1"}, crash: true}, {args: []string{"-flow", "resumable"}}},
		want: with(ok, func(e *exp) { e.events = []string{"step.resumed e1/work (platform_restarted)"} })})

	// Crash storm: three crashes in a row inside the same Step, then a clean run.
	cs = append(cs, tcase{id: "idem/crash-storm", what: "crash after the effect in three consecutive runs",
		runs: []run{{args: []string{"-flow", "idem", "-crash", "worker.after_effect#2"}, crash: true},
			{args: []string{"-flow", "idem", "-crash", "worker.after_effect#1"}, crash: true},
			{args: []string{"-flow", "idem", "-crash", "worker.after_effect#1"}, crash: true},
			{args: []string{"-flow", "idem"}}}, want: ok})

	// NATS store loss: ADR-0016 says no state is lost, the assignment is re-enqueued.
	for _, flow := range []string{"idem", "nonidem"} {
		cs = append(cs, tcase{id: flow + "/nats-store-lost", what: "assignment fetched, then the whole JetStream store is deleted before restart",
			runs: []run{{args: []string{"-flow", flow, "-crash", "worker.after_fetch#2"}, crash: true}, {args: []string{"-flow", flow, "-wipe-nats"}}}, want: ok})
	}

	// Worker loss without a process crash: the Worker hangs, the lease expires.
	hang := func(flow, spec string) []run {
		return []run{{args: []string{"-flow", flow, "-crash", spec + ":hang", "-advance-after-ms", "800", "-advance-late", "100"}}}
	}
	cs = append(cs,
		tcase{id: "idem/worker-hangs-lease-expires", what: "Worker hangs while running; lease expires under controlled time", runs: hang("idem", "running.after_commit#2"),
			want: with(ok, func(e *exp) { e.events = []string{"step.retried e1/work (lease_expired)"} })},
		tcase{id: "nonidem/worker-hangs-lease-expires", what: "same, non-idempotent Step: must not be re-run", runs: hang("nonidem", "running.after_commit#2"),
			want: with(interrupted(0), func(e *exp) { e.events = []string{"step.interrupted e1/work (lease_expired)"} })},
		tcase{id: "nonidem/worker-hangs-after-effect-lease-expires", what: "non-idempotent effect done, Worker hangs before the result: interrupted, effect not duplicated", runs: hang("nonidem", "worker.after_effect#2"),
			want: with(interrupted(1), func(e *exp) { e.events = []string{"step.interrupted e1/work (lease_expired)"} })},
		tcase{id: "resumable/worker-hangs-lease-expires", what: "resumable Step with job id: resumed under a new lease", runs: hang("resumable", "worker.after_effect#2"),
			want: with(ok, func(e *exp) { e.events = []string{"step.resumed e1/work (lease_expired)"} })},
	)

	// Step timeout under controlled time.
	cs = append(cs, tcase{id: "timeout/step-timeout", what: "running Step exceeds its 60 s timeout; lease is longer",
		runs: []run{{args: []string{"-flow", "timeout", "-lease-ttl", "3600", "-crash", "running.after_commit#2:hang", "-advance-after-ms", "800", "-advance-late", "100"}}},
		want: with(ok, func(e *exp) {
			e.exec = "needs_attention"
			e.steps["work"], e.steps["deliver"] = "timed_out", "pending"
			e.effects["work"], e.effects["deliver"] = 0, 0
			e.events = []string{"timer.step_timeout e1/work"}
		})})

	// Flaky Step: failure, back-off timer, retry under controlled time.
	cs = append(cs, tcase{id: "flaky/retry-backoff", what: "first attempt fails; back-off timer fires after the clock advances",
		runs: []run{{args: []string{"-flow", "flaky"}}, {args: []string{"-flow", "flaky", "-advance", "11"}}},
		want: with(ok, func(e *exp) { e.events = []string{"step.retry_scheduled e1/work", "timer.retry e1/work"} })})
	cs = append(cs, tcase{id: "flaky/retry-timer-survives-crash", what: "crash while the retry timer is pending; timer still fires after restart",
		runs: []run{{args: []string{"-flow", "flaky"}, killAfter: 1500 * time.Millisecond}, {args: []string{"-flow", "flaky", "-advance", "11"}}},
		want: with(ok, func(e *exp) { e.events = []string{"timer.retry e1/work"} })})

	// Approval wait over several simulated days.
	waiting := exp{exec: "running", steps: map[string]string{"prep": "succeeded", "gate": "waiting_approval", "deliver": "pending"},
		effects: map[string]int{"prep": 1, "deliver": 0}, absent: []string{"interrupted"}}
	day := "86400"
	cs = append(cs, tcase{id: "approval/multi-day-wait-then-approve", what: "waits 3 simulated days with a restart each day, then approved",
		runs: []run{{args: []string{"-flow", "approval"}}, {args: []string{"-advance", day}}, {args: []string{"-advance", day}}, {args: []string{"-advance", day}}, {args: []string{"-approve", "e1/gate"}}},
		want: exp{exec: "succeeded", steps: map[string]string{"prep": "succeeded", "gate": "succeeded", "deliver": "succeeded"},
			effects: map[string]int{"prep": 1, "deliver": 1}, absent: []string{"interrupted", "approval_deadline"}}})
	cs = append(cs, tcase{id: "approval/still-waiting-after-days", what: "three days pass across restarts; gate keeps waiting and nothing else runs",
		runs: []run{{args: []string{"-flow", "approval"}}, {args: []string{"-advance", day}}, {args: []string{"-advance", day}}, {args: []string{"-advance", day}}}, want: waiting})
	cs = append(cs, tcase{id: "approval/killed-while-waiting", what: "process killed externally while the gate waits; restart keeps waiting",
		runs: []run{{args: []string{"-flow", "approval", "-hold-ms", "5000"}, killAfter: 2500 * time.Millisecond}, {args: []string{}}}, want: waiting})
	cs = append(cs, tcase{id: "approval/deadline-expires", what: "5-day approval deadline passes: gate fails, delivery never runs",
		runs: []run{{args: []string{"-flow", "approval"}}, {args: []string{"-advance", "432001"}}},
		want: exp{exec: "failed", steps: map[string]string{"prep": "succeeded", "gate": "failed", "deliver": "pending"},
			effects: map[string]int{"prep": 1, "deliver": 0}, events: []string{"timer.approval_deadline e1/gate"}}})
	cs = append(cs, tcase{id: "approval/approve-crash-before-commit", what: "crash while recording the approval: still waiting, approving again succeeds",
		runs: []run{{args: []string{"-flow", "approval"}}, {args: []string{"-approve", "e1/gate", "-crash", "approve.before_commit#1"}, crash: true}, {args: []string{"-approve", "e1/gate"}}},
		want: exp{exec: "succeeded", steps: map[string]string{"gate": "succeeded", "deliver": "succeeded"}, effects: map[string]int{"deliver": 1}}})
	cs = append(cs, tcase{id: "approval/approve-crash-after-commit", what: "crash right after the approval committed: delivery completes after restart, once",
		runs: []run{{args: []string{"-flow", "approval"}}, {args: []string{"-approve", "e1/gate", "-crash", "approve.after_commit#1"}, crash: true}, {args: []string{}}},
		want: exp{exec: "succeeded", steps: map[string]string{"gate": "succeeded", "deliver": "succeeded"}, effects: map[string]int{"deliver": 1}}})
	cs = append(cs, tcase{id: "approval/duplicate-approve", what: "the same approval is submitted twice",
		runs: []run{{args: []string{"-flow", "approval"}}, {args: []string{"-approve", "e1/gate"}}, {args: []string{"-approve", "e1/gate"}}},
		want: exp{exec: "succeeded", steps: map[string]string{"gate": "succeeded", "deliver": "succeeded"}, effects: map[string]int{"deliver": 1}}})
	cs = append(cs, tcase{id: "approval/approve-after-reject", what: "approval arrives after the gate was rejected: ignored",
		runs: []run{{args: []string{"-flow", "approval"}}, {args: []string{"-reject", "e1/gate"}}, {args: []string{"-approve", "e1/gate"}}},
		want: exp{exec: "failed", steps: map[string]string{"gate": "failed", "deliver": "pending"}, effects: map[string]int{"deliver": 0}}})
	_ = context.Background
	return cs
}

// onlyKillNotes is true when every note is the driver's own record of an
// intentional external kill.
func onlyKillNotes(notes string) bool {
	for _, n := range strings.Split(notes, "; ") {
		if !strings.Contains(n, "killed externally") {
			return false
		}
	}
	return true
}
