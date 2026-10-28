package porttest

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
)

var t0 = time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)

func newExec(id string, steps ...string) (domain.Execution, []domain.StepExecution) {
	e := domain.Execution{ID: id, State: domain.ExecQueued, CreatedAt: t0}
	out := make([]domain.StepExecution, len(steps))
	for i, s := range steps {
		out[i] = domain.StepExecution{ID: s, ExecutionID: id, State: domain.StepPending}
	}
	return e, out
}

// ExecutionStore runs the ports.ExecutionStore contract against newStore, which must return an
// empty store each call. reopen, when not nil, returns a new store over the same data, as a
// restarted Platform sees it; the durability checks use it.
func ExecutionStore(t *testing.T, newStore func(t *testing.T) ports.ExecutionStore, reopen func(t *testing.T, s ports.ExecutionStore) ports.ExecutionStore) {
	ctx := context.Background()
	if reopen == nil {
		reopen = func(_ *testing.T, s ports.ExecutionStore) ports.ExecutionStore { return s }
	}

	t.Run("create then get keeps step order and sets version 1", func(t *testing.T) {
		s := newStore(t)
		e, steps := newExec("e1", "b", "a", "c")
		ce, cs, err := s.Create(ctx, e, steps)
		if err != nil {
			t.Fatal(err)
		}
		if ce.Version != 1 || len(cs) != 3 || cs[0].Version != 1 {
			t.Fatalf("create result: %+v %+v", ce, cs)
		}
		ge, gs, err := s.Get(ctx, "e1")
		if err != nil {
			t.Fatal(err)
		}
		if ge.State != domain.ExecQueued || !ge.CreatedAt.Equal(t0) || ge.Version != 1 {
			t.Fatalf("execution: %+v", ge)
		}
		if len(gs) != 3 || gs[0].ID != "b" || gs[1].ID != "a" || gs[2].ID != "c" {
			t.Fatalf("steps not in creation order: %+v", gs)
		}
	})

	t.Run("create validates", func(t *testing.T) {
		s := newStore(t)
		e, steps := newExec("e1", "s1")
		if _, _, err := s.Create(ctx, e, steps); err != nil {
			t.Fatal(err)
		}
		if _, _, err := s.Create(ctx, e, steps); !errors.Is(err, domain.ErrExists) {
			t.Fatalf("second create: got %v, want ErrExists", err)
		}
		e2, _ := newExec("e2")
		if _, _, err := s.Create(ctx, e2, []domain.StepExecution{{ID: "x", ExecutionID: "other"}}); !errors.Is(err, domain.ErrInvalid) {
			t.Fatalf("foreign step: got %v, want ErrInvalid", err)
		}
		if _, _, err := s.Create(ctx, e2, []domain.StepExecution{{ID: "x", ExecutionID: "e2"}, {ID: "x", ExecutionID: "e2"}}); !errors.Is(err, domain.ErrInvalid) {
			t.Fatalf("duplicate step: got %v, want ErrInvalid", err)
		}
		if _, _, err := s.Get(ctx, "e2"); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("failed create left a row: %v", err)
		}
	})

	t.Run("get unknown", func(t *testing.T) {
		s := newStore(t)
		if _, _, err := s.Get(ctx, "nope"); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("got %v, want ErrNotFound", err)
		}
		if _, err := s.Events(ctx, "nope"); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("events: got %v, want ErrNotFound", err)
		}
	})

	t.Run("commit persists the whole step and bumps versions", func(t *testing.T) {
		s := newStore(t)
		e, steps := newExec("e1", "s1")
		e, steps = mustCreate(t, s, e, steps)
		if err := steps[0].Plan(); err != nil {
			t.Fatal(err)
		}
		if err := steps[0].Lease("w1", "att-1", t0.Add(time.Minute)); err != nil {
			t.Fatal(err)
		}
		steps[0].Idempotent = true
		steps[0].Capability, steps[0].InputsJSON = "code.test", []byte(`{"a":1}`)
		steps[0].RemoteJob = &domain.RemoteJob{ID: "job-1", Resumable: true}
		if err := e.Start(t0); err != nil {
			t.Fatal(err)
		}
		out, err := s.Commit(ctx, ports.ExecutionChange{Execution: &e, Steps: steps})
		if err != nil {
			t.Fatal(err)
		}
		if out.Execution.Version != 2 || out.Steps[0].Version != 2 {
			t.Fatalf("versions: %+v %+v", out.Execution, out.Steps)
		}
		s = reopen(t, s)
		ge, gs, err := s.Get(ctx, "e1")
		if err != nil {
			t.Fatal(err)
		}
		g := gs[0]
		if ge.State != domain.ExecRunning || ge.StartedAt == nil || !ge.StartedAt.Equal(t0) || ge.Version != 2 {
			t.Fatalf("execution: %+v", ge)
		}
		if g.State != domain.StepLeased || g.Version != 2 || !g.Idempotent ||
			g.CurrentLease == nil || g.CurrentLease.Owner != "w1" || !g.CurrentLease.ExpiresAt.Equal(t0.Add(time.Minute)) ||
			g.RemoteJob == nil || g.RemoteJob.ID != "job-1" || !g.RemoteJob.Resumable ||
			g.Capability != "code.test" || string(g.InputsJSON) != `{"a":1}` || g.AttemptID != "att-1" || g.FenceToken != 1 {
			t.Fatalf("step: %+v", g)
		}
	})

	t.Run("commit keeps reason, times, error and agent snapshot", func(t *testing.T) {
		s := newStore(t)
		e, steps := newExec("e1", "s1", "s2")
		_, steps = mustCreate(t, s, e, steps)
		if err := steps[0].Skip(t0, "condition not met"); err != nil {
			t.Fatal(err)
		}
		if err := steps[1].BindAgent(domain.ResolvedAgent{ID: "a1", Name: "reviewer", Revision: 3}); err != nil {
			t.Fatal(err)
		}
		if err := steps[1].Plan(); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Commit(ctx, ports.ExecutionChange{Steps: steps}); err != nil {
			t.Fatal(err)
		}
		_, gs, _ := reopen(t, s).Get(ctx, "e1")
		if gs[0].State != domain.StepSkipped || gs[0].Reason != "condition not met" || gs[0].EndedAt == nil || !gs[0].EndedAt.Equal(t0) {
			t.Fatalf("skipped step: %+v", gs[0])
		}
		if gs[1].Agent == nil || gs[1].Agent.ID != "a1" || gs[1].Agent.Revision != 3 {
			t.Fatalf("agent snapshot lost: %+v", gs[1].Agent)
		}
		e2, steps2 := newExec("e2", "e2-s1")
		_, steps2 = mustCreate(t, s, e2, steps2)
		_ = steps2[0].RequireApproval()
		if err := steps2[0].Reject(t0, &domain.ExecError{Code: "approval_rejected", Message: "no"}); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Commit(ctx, ports.ExecutionChange{Steps: steps2}); err != nil {
			t.Fatal(err)
		}
		_, gs2, _ := reopen(t, s).Get(ctx, "e2")
		if gs2[0].Error == nil || gs2[0].Error.Code != "approval_rejected" || gs2[0].Error.Message != "no" {
			t.Fatalf("error lost: %+v", gs2[0])
		}
	})

	t.Run("stale version is refused and writes nothing", func(t *testing.T) {
		s := newStore(t)
		e, steps := newExec("e1", "s1", "s2")
		e, steps = mustCreate(t, s, e, steps)
		stale := append([]domain.StepExecution{}, steps...)
		if err := steps[0].Plan(); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Commit(ctx, ports.ExecutionChange{Steps: steps[:1]}); err != nil {
			t.Fatal(err)
		}
		// a second writer still holds the old version of s1, with a good change to s2 and the execution
		_ = stale[0].Plan()
		_ = stale[1].Plan()
		_ = e.Start(t0)
		_, err := s.Commit(ctx, ports.ExecutionChange{Execution: &e, Steps: stale, Events: []domain.ExecutionEvent{{Kind: "x", At: t0}}})
		if !errors.Is(err, domain.ErrConflict) {
			t.Fatalf("got %v, want ErrConflict", err)
		}
		ge, gs, _ := s.Get(ctx, "e1")
		if ge.State != domain.ExecQueued || ge.Version != 1 || gs[1].State != domain.StepPending || gs[1].Version != 1 {
			t.Fatalf("a refused change wrote rows: %+v %+v", ge, gs)
		}
		if evs, _ := s.Events(ctx, "e1"); len(evs) != 0 {
			t.Fatalf("a refused change appended events: %+v", evs)
		}
	})

	t.Run("concurrent writers on one step: exactly one wins", func(t *testing.T) {
		s := newStore(t)
		e, steps := newExec("e1", "s1")
		_, steps = mustCreate(t, s, e, steps)
		_ = steps[0].Plan()
		const n = 8
		var wg sync.WaitGroup
		var mu sync.Mutex
		wins, conflicts := 0, 0
		for i := 0; i < n; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := s.Commit(ctx, ports.ExecutionChange{Steps: append([]domain.StepExecution{}, steps...)})
				mu.Lock()
				defer mu.Unlock()
				switch {
				case err == nil:
					wins++
				case errors.Is(err, domain.ErrConflict):
					conflicts++
				default:
					t.Errorf("unexpected error: %v", err)
				}
			}()
		}
		wg.Wait()
		if wins != 1 || conflicts != n-1 {
			t.Fatalf("wins=%d conflicts=%d, want 1 and %d", wins, conflicts, n-1)
		}
	})

	t.Run("commit unknown rows", func(t *testing.T) {
		s := newStore(t)
		if _, err := s.Commit(ctx, ports.ExecutionChange{Steps: []domain.StepExecution{{ID: "x", ExecutionID: "e1", Version: 1}}}); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("got %v, want ErrNotFound", err)
		}
		e, steps := newExec("e1", "s1")
		_, steps = mustCreate(t, s, e, steps)
		steps[0].ID = "other"
		if _, err := s.Commit(ctx, ports.ExecutionChange{Steps: steps}); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("unknown step: got %v, want ErrNotFound", err)
		}
	})

	t.Run("commit must name one execution", func(t *testing.T) {
		s := newStore(t)
		if _, err := s.Commit(ctx, ports.ExecutionChange{}); !errors.Is(err, domain.ErrInvalid) {
			t.Fatalf("empty: got %v, want ErrInvalid", err)
		}
		_, err := s.Commit(ctx, ports.ExecutionChange{Steps: []domain.StepExecution{{ID: "a", ExecutionID: "e1"}, {ID: "b", ExecutionID: "e2"}}})
		if !errors.Is(err, domain.ErrInvalid) {
			t.Fatalf("spanning: got %v, want ErrInvalid", err)
		}
	})

	t.Run("events are ordered, gapless and committed with the change", func(t *testing.T) {
		s := newStore(t)
		e, steps := newExec("e1", "s1")
		e, steps = mustCreate(t, s, e, steps)
		_ = steps[0].Plan()
		out, err := s.Commit(ctx, ports.ExecutionChange{Steps: steps, Events: []domain.ExecutionEvent{
			{Kind: "a", StepID: "s1", At: t0}, {Kind: "b", StepID: "s1", Reason: "why", At: t0.Add(time.Second)},
		}})
		if err != nil {
			t.Fatal(err)
		}
		if out.Events[0].Seq != 1 || out.Events[1].Seq != 2 || out.Events[0].ExecutionID != "e1" {
			t.Fatalf("returned events: %+v", out.Events)
		}
		_ = e.Start(t0)
		if _, err := s.Commit(ctx, ports.ExecutionChange{Execution: &e, Events: []domain.ExecutionEvent{{Kind: "c", At: t0}}}); err != nil {
			t.Fatal(err)
		}
		evs, err := reopen(t, s).Events(ctx, "e1")
		if err != nil {
			t.Fatal(err)
		}
		if len(evs) != 3 || evs[0].Kind != "a" || evs[1].Reason != "why" || !evs[1].At.Equal(t0.Add(time.Second)) || evs[2].Seq != 3 || evs[2].StepID != "" {
			t.Fatalf("log: %+v", evs)
		}
	})

	t.Run("concurrent commits give a gapless log", func(t *testing.T) {
		s := newStore(t)
		e, steps := newExec("e1", "s1", "s2", "s3", "s4")
		_, steps = mustCreate(t, s, e, steps)
		var wg sync.WaitGroup
		for i := range steps {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if _, err := s.Commit(ctx, ports.ExecutionChange{Steps: steps[i : i+1], Events: []domain.ExecutionEvent{{Kind: "k", StepID: steps[i].ID, At: t0}}}); err != nil {
					t.Error(err)
				}
			}()
		}
		wg.Wait()
		evs, _ := s.Events(ctx, "e1")
		for i, ev := range evs {
			if ev.Seq != int64(i+1) {
				t.Fatalf("gap or reorder at %d: %+v", i, evs)
			}
		}
		if len(evs) != len(steps) {
			t.Fatalf("got %d events, want %d", len(evs), len(steps))
		}
	})

	t.Run("queued lists queued steps of open executions, oldest first, up to the limit", func(t *testing.T) {
		s := newStore(t)
		older, newer := t0, t0.Add(time.Minute)
		mk := func(id string, created time.Time, ids ...string) {
			e, steps := newExec(id, ids...)
			e.CreatedAt = created
			_, steps = mustCreate(t, s, e, steps)
			for i := range steps {
				if err := steps[i].Plan(); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := s.Commit(ctx, ports.ExecutionChange{Steps: steps}); err != nil {
				t.Fatal(err)
			}
		}
		mk("b-new", newer, "n1", "n2")
		mk("a-old", older, "o1", "o2")
		mk("c-done", older, "d1")
		d, _, _ := s.Get(ctx, "c-done")
		_ = d.Finish(t0, domain.ExecCancelled, nil)
		if _, err := s.Commit(ctx, ports.ExecutionChange{Execution: &d}); err != nil {
			t.Fatal(err)
		}
		_, ns, _ := s.Get(ctx, "b-new")
		if err := ns[0].Lease("w1", "att-n1", t0.Add(time.Minute)); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Commit(ctx, ports.ExecutionChange{Steps: ns[:1]}); err != nil {
			t.Fatal(err)
		}
		got, err := reopen(t, s).Queued(ctx, 10)
		if err != nil {
			t.Fatal(err)
		}
		want := []string{"o1", "o2", "n2"}
		if len(got) != len(want) {
			t.Fatalf("got %v, want %v", got, want)
		}
		for i := range want {
			if got[i].StepID != want[i] {
				t.Fatalf("got %v, want %v", got, want)
			}
		}
		if lim, _ := s.Queued(ctx, 2); len(lim) != 2 {
			t.Fatalf("limit ignored: %v", lim)
		}
	})

	t.Run("by attempt finds the step after its lease ended", func(t *testing.T) {
		s := newStore(t)
		e, steps := newExec("e1", "s1")
		_, steps = mustCreate(t, s, e, steps)
		_ = steps[0].Plan()
		_ = steps[0].Lease("w1", "att-1", t0.Add(time.Minute))
		out, err := s.Commit(ctx, ports.ExecutionChange{Steps: steps})
		if err != nil {
			t.Fatal(err)
		}
		st := out.Steps[0]
		_ = st.ReleaseLease(domain.ReasonAttemptRejected)
		if _, err := s.Commit(ctx, ports.ExecutionChange{Steps: []domain.StepExecution{st}}); err != nil {
			t.Fatal(err)
		}
		ref, err := reopen(t, s).ByAttempt(ctx, "att-1")
		if err != nil || ref.ExecutionID != "e1" || ref.StepID != "s1" {
			t.Fatalf("got %+v, %v", ref, err)
		}
		for _, bad := range []string{"", "nope"} {
			if _, err := s.ByAttempt(ctx, bad); !errors.Is(err, domain.ErrNotFound) {
				t.Fatalf("%q: got %v, want ErrNotFound", bad, err)
			}
		}
	})

	t.Run("an attempt id is unique", func(t *testing.T) {
		s := newStore(t)
		e, steps := newExec("e1", "s1", "s2")
		_, steps = mustCreate(t, s, e, steps)
		for i := range steps {
			_ = steps[i].Plan()
			_ = steps[i].Lease("w1", "same", t0.Add(time.Minute))
		}
		if _, err := s.Commit(ctx, ports.ExecutionChange{Steps: steps}); err == nil {
			t.Fatal("two steps committed with one attempt id")
		}
		_, got, _ := s.Get(ctx, "e1")
		if got[0].State != domain.StepPending {
			t.Fatalf("a refused commit wrote rows: %+v", got[0])
		}
	})

	t.Run("open lists executions that are not terminal", func(t *testing.T) {
		s := newStore(t)
		for _, id := range []string{"e3", "e1", "e2"} {
			e, steps := newExec(id, id+"-s")
			mustCreate(t, s, e, steps)
		}
		e, _, _ := s.Get(ctx, "e2")
		if err := e.Finish(t0, domain.ExecCancelled, nil); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Commit(ctx, ports.ExecutionChange{Execution: &e}); err != nil {
			t.Fatal(err)
		}
		got, err := reopen(t, s).Open(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 2 || got[0] != "e1" || got[1] != "e3" {
			t.Fatalf("got %v, want [e1 e3]", got)
		}
	})

	rec := func(key string) domain.CommandRecord {
		return domain.CommandRecord{Name: "execution.submit", Key: key, Fingerprint: "fp-" + key, Result: []byte(`{"id":"` + key + `"}`), At: t0}
	}

	t.Run("a command record is stored with the Execution and survives a restart", func(t *testing.T) {
		s := newStore(t)
		e, steps := newExec("e1", "s1")
		if _, _, err := s.CreateOnce(ctx, rec("k1"), e, steps); err != nil {
			t.Fatal(err)
		}
		got, err := reopen(t, s).Command(ctx, "execution.submit", "k1")
		if err != nil {
			t.Fatal(err)
		}
		want := rec("k1")
		if got.Fingerprint != want.Fingerprint || string(got.Result) != string(want.Result) || !got.At.Equal(want.At) {
			t.Fatalf("got %+v, want %+v", got, want)
		}
		if _, err := s.Command(ctx, "execution.submit", "other"); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("unknown key: got %v, want ErrNotFound", err)
		}
		if _, err := s.Command(ctx, "other.command", "k1"); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("a key is scoped to its command: got %v, want ErrNotFound", err)
		}
	})

	t.Run("a taken key writes nothing", func(t *testing.T) {
		s := newStore(t)
		e1, steps1 := newExec("e1", "s1")
		if _, _, err := s.CreateOnce(ctx, rec("k1"), e1, steps1); err != nil {
			t.Fatal(err)
		}
		e2, steps2 := newExec("e2", "e2-s1")
		if _, _, err := s.CreateOnce(ctx, rec("k1"), e2, steps2); !errors.Is(err, domain.ErrDuplicateCommand) {
			t.Fatalf("got %v, want ErrDuplicateCommand", err)
		}
		if _, _, err := reopen(t, s).Get(ctx, "e2"); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("the Execution of a refused command was stored: %v", err)
		}
	})

	t.Run("an invalid record is refused", func(t *testing.T) {
		s := newStore(t)
		e, steps := newExec("e1", "s1")
		for _, bad := range []domain.CommandRecord{{}, {Name: "n", Key: "k"}, {Name: "n", Fingerprint: "f"}, {Key: "k", Fingerprint: "f"}} {
			if _, _, err := s.CreateOnce(ctx, bad, e, steps); !errors.Is(err, domain.ErrInvalid) {
				t.Fatalf("%+v: got %v, want ErrInvalid", bad, err)
			}
		}
	})

	t.Run("a commit stores its record atomically", func(t *testing.T) {
		s := newStore(t)
		e, steps := newExec("e1", "s1")
		e, steps = mustCreate(t, s, e, steps)
		_ = steps[0].Plan()
		c := rec("cancel-1")
		c.Name = "execution.test"
		if _, err := s.Commit(ctx, ports.ExecutionChange{Steps: steps, Command: &c}); err != nil {
			t.Fatal(err)
		}
		if _, err := reopen(t, s).Command(ctx, "execution.test", "cancel-1"); err != nil {
			t.Fatal(err)
		}
		// The same key again: the second write is refused as a whole.
		_, cur, _ := s.Get(ctx, "e1")
		_ = cur[0].Lease("w1", "att-1", t0.Add(time.Minute))
		if _, err := s.Commit(ctx, ports.ExecutionChange{Steps: cur, Command: &c}); !errors.Is(err, domain.ErrDuplicateCommand) {
			t.Fatalf("got %v, want ErrDuplicateCommand", err)
		}
		_, after, _ := reopen(t, s).Get(ctx, "e1")
		if after[0].State != domain.StepQueued {
			t.Fatalf("a refused command changed the Step to %s", after[0].State)
		}
	})

	t.Run("a stale commit leaves no record", func(t *testing.T) {
		s := newStore(t)
		e, steps := newExec("e1", "s1")
		_, steps = mustCreate(t, s, e, steps)
		stale := steps[0]
		_ = steps[0].Plan()
		if _, err := s.Commit(ctx, ports.ExecutionChange{Steps: steps}); err != nil {
			t.Fatal(err)
		}
		_ = stale.Plan()
		c := rec("late")
		if _, err := s.Commit(ctx, ports.ExecutionChange{Steps: []domain.StepExecution{stale}, Command: &c}); !errors.Is(err, domain.ErrConflict) {
			t.Fatalf("got %v, want ErrConflict", err)
		}
		if _, err := reopen(t, s).Command(ctx, c.Name, c.Key); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("a refused write left its record: %v", err)
		}
	})

	t.Run("the requester and trigger are stored with the Execution", func(t *testing.T) {
		s := newStore(t)
		e, steps := newExec("e1", "s1")
		e.RequestedBy = domain.Principal{Kind: domain.PrincipalTrigger, ID: "hook-1", Sponsor: "u1"}
		e.Trigger = domain.TriggerWebhook
		mustCreate(t, s, e, steps)
		got, _, err := reopen(t, s).Get(ctx, "e1")
		if err != nil {
			t.Fatal(err)
		}
		if got.RequestedBy != e.RequestedBy || got.Trigger != domain.TriggerWebhook {
			t.Fatalf("got %+v %q", got.RequestedBy, got.Trigger)
		}
		// An Execution created without one reads back with the zero requester.
		e2, steps2 := newExec("e2", "s2")
		mustCreate(t, s, e2, steps2)
		if got, _, _ := reopen(t, s).Get(ctx, "e2"); got.RequestedBy != (domain.Principal{}) || got.Trigger != "" {
			t.Fatalf("zero requester read back as %+v %q", got.RequestedBy, got.Trigger)
		}
	})

	t.Run("scopes are stored with the Execution, listed with Queued and Active, and never change", func(t *testing.T) {
		s := newStore(t)
		e, steps := newExec("e1", "s1", "s2")
		e.ProjectID, e.EnvironmentID = "p1", "dev"
		e, steps = mustCreate(t, s, e, steps)
		for i := range steps {
			_ = steps[i].Plan()
		}
		out, err := s.Commit(ctx, ports.ExecutionChange{Steps: steps})
		if err != nil {
			t.Fatal(err)
		}
		got, _, _ := reopen(t, s).Get(ctx, "e1")
		if got.ProjectID != "p1" || got.EnvironmentID != "dev" {
			t.Fatalf("scopes: %q %q", got.ProjectID, got.EnvironmentID)
		}
		q, err := reopen(t, s).Queued(ctx, 10)
		if err != nil || len(q) != 2 || q[0].ProjectID != "p1" || q[0].EnvironmentID != "dev" {
			t.Fatalf("queued %+v, %v", q, err)
		}
		if a, err := s.Active(ctx); err != nil || len(a) != 0 {
			t.Fatalf("active before any Lease: %+v, %v", a, err)
		}
		st := out.Steps[0]
		_ = st.Lease("w1", "att-1", t0.Add(time.Minute))
		out, err = s.Commit(ctx, ports.ExecutionChange{Steps: []domain.StepExecution{st}})
		if err != nil {
			t.Fatal(err)
		}
		a, err := reopen(t, s).Active(ctx)
		if err != nil || len(a) != 1 || a[0].StepID != "s1" || a[0].ProjectID != "p1" {
			t.Fatalf("active %+v, %v", a, err)
		}
		// A write that names other scopes does not move the Execution.
		e.ProjectID, e.EnvironmentID = "other", "prod"
		if _, err := s.Commit(ctx, ports.ExecutionChange{Execution: &e}); err != nil {
			t.Fatal(err)
		}
		got, _, _ = reopen(t, s).Get(ctx, "e1")
		if got.ProjectID != "p1" || got.EnvironmentID != "dev" {
			t.Fatalf("scopes changed to %q %q", got.ProjectID, got.EnvironmentID)
		}
		// A finished Execution holds no slot.
		if err := got.Finish(t0, domain.ExecCancelled, nil); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Commit(ctx, ports.ExecutionChange{Execution: &got}); err != nil {
			t.Fatal(err)
		}
		if a, err := s.Active(ctx); err != nil || len(a) != 0 {
			t.Fatalf("active after the Execution finished: %+v, %v", a, err)
		}
	})
}

func mustCreate(t *testing.T, s ports.ExecutionStore, e domain.Execution, steps []domain.StepExecution) (domain.Execution, []domain.StepExecution) {
	t.Helper()
	ce, cs, err := s.Create(context.Background(), e, steps)
	if err != nil {
		t.Fatal(err)
	}
	return ce, cs
}
