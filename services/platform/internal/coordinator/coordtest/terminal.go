package coordtest

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/rluders/amaru/services/platform/internal/coordinator"
	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
)

// terminalStore injects one competing write immediately before a selected commit.
type terminalStore struct {
	ports.ExecutionStore
	before func(ports.ExecutionChange)
}

func (s *terminalStore) Commit(ctx context.Context, ch ports.ExecutionChange) (ports.ExecutionChange, error) {
	if s.before != nil {
		s.before(ch)
	}
	return s.ExecutionStore.Commit(ctx, ch)
}

// TerminalSuite exercises ADR-0063 with the same application, store and Worker-link contracts
// for fakes and PostgreSQL. Seeded final Steps model a crash before parent aggregation.
func TerminalSuite(t *testing.T, newStore func(*testing.T) ports.ExecutionStore, nl NewLink) {
	ctx := context.Background()
	seed := func(t *testing.T, store ports.ExecutionStore, state domain.ExecState, states ...domain.StepState) {
		t.Helper()
		e := domain.Execution{ID: "e1", ProjectID: "p1", State: state, CreatedAt: t0, RequestedBy: Requester, Trigger: domain.TriggerManual}
		if state != domain.ExecQueued {
			e.StartedAt = &t0
		}
		var steps []domain.StepExecution
		for i, state := range states {
			st := domain.StepExecution{ID: fmt.Sprintf("s%d", i), ExecutionID: e.ID, State: state, Capability: "code.test", Idempotent: true}
			if state.Terminal() || state == domain.StepTimedOut {
				st.EndedAt = &t0
			}
			if state == domain.StepFailed {
				st.Error = &domain.ExecError{Code: "tests_failed", Message: "safe failure"}
			}
			steps = append(steps, st)
		}
		if _, _, err := store.Create(ctx, e, steps); err != nil {
			t.Fatal(err)
		}
	}
	assertParent := func(t *testing.T, store ports.ExecutionStore, state domain.ExecState) domain.Execution {
		t.Helper()
		e, _, err := store.Get(ctx, "e1")
		if err != nil || e.State != state || (state.Terminal() != (e.EndedAt != nil)) {
			t.Fatalf("parent %+v, want %s: %v", e, state, err)
		}
		evs, err := store.Events(ctx, "e1")
		if err != nil {
			t.Fatal(err)
		}
		n := 0
		for _, ev := range evs {
			if ev.Kind == domain.EventExecCompleted || ev.Kind == domain.EventExecFailed || ev.Kind == domain.EventExecCancelled {
				n++
				if ev.Reason != "" {
					t.Fatal("parent Event includes failure text")
				}
			}
		}
		if (state.Terminal() && n != 1) || (!state.Terminal() && n != 0) {
			t.Fatalf("%d terminal Events for %s", n, state)
		}
		return e
	}
	for _, tc := range []struct {
		name       string
		from, want domain.ExecState
		steps      []domain.StepState
	}{
		{"success crash window", domain.ExecRunning, domain.ExecSucceeded, []domain.StepState{domain.StepSucceeded}},
		{"failure crash window", domain.ExecRunning, domain.ExecFailed, []domain.StepState{domain.StepFailed}},
		{"all skipped", domain.ExecQueued, domain.ExecSucceeded, []domain.StepState{domain.StepSkipped, domain.StepSkipped}},
		{"prestart refusal", domain.ExecQueued, domain.ExecFailed, []domain.StepState{domain.StepRefused}},
		{"waiting success", domain.ExecWaiting, domain.ExecSucceeded, []domain.StepState{domain.StepSucceeded, domain.StepSkipped}},
		{"mixed cancelled", domain.ExecRunning, domain.ExecCancelled, []domain.StepState{domain.StepSucceeded, domain.StepCancelled}},
		{"failure beats cancelled", domain.ExecRunning, domain.ExecFailed, []domain.StepState{domain.StepCancelled, domain.StepFailed}},
		{"empty stays open", domain.ExecQueued, domain.ExecQueued, nil},
		{"drain pending and queued", domain.ExecRunning, domain.ExecFailed, []domain.StepState{domain.StepFailed, domain.StepPending, domain.StepQueued}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := newStore(t)
			seed(t, store, tc.from, tc.steps...)
			c := newCoordinator(store)
			if _, err := c.Reconcile(ctx); err != nil {
				t.Fatal(err)
			}
			e := assertParent(t, store, tc.want)
			if tc.from == domain.ExecQueued && e.StartedAt != nil {
				t.Fatal("fabricated parent start")
			}
			evs, _ := store.Events(ctx, "e1")
			if _, err := newCoordinator(store).Reconcile(ctx); err != nil {
				t.Fatal(err)
			}
			after := assertParent(t, store, tc.want)
			afterEvents, _ := store.Events(ctx, "e1")
			if after.Version != e.Version || len(afterEvents) != len(evs) {
				t.Fatal("repeat reconciliation changed settled state")
			}
		})
	}

	for _, waitState := range []domain.StepState{domain.StepTimedOut, domain.StepInterrupted} {
		t.Run("failure preserves "+string(waitState), func(t *testing.T) {
			store := newStore(t)
			seed(t, store, domain.ExecRunning, domain.StepFailed, waitState)
			c := newCoordinator(store)
			if _, err := c.Reconcile(ctx); err != nil {
				t.Fatal(err)
			}
			assertParent(t, store, domain.ExecWaiting)
			req := coordinator.HumanStepRequest{Key: "resume", ExecutionID: "e1", StepID: "s1", By: domain.Actor{Kind: domain.ActorHuman}, ActorID: "owner"}
			_, before, _ := store.Get(ctx, "e1")
			if _, _, err := c.ResumeStep(ctx, req); !errors.Is(err, domain.ErrInvalidTransition) {
				t.Fatalf("resume during failure: %v", err)
			}
			_, after, _ := store.Get(ctx, "e1")
			if after[1].Version != before[1].Version || after[1].State != waitState {
				t.Fatal("refused resume changed human wait")
			}
			req.Key = "abandon"
			if _, replayed, err := c.AbandonStep(ctx, req); err != nil || replayed {
				t.Fatalf("abandon: %v replay %v", err, replayed)
			}
			assertParent(t, store, domain.ExecFailed)
			if _, replayed, err := c.AbandonStep(ctx, req); err != nil || !replayed {
				t.Fatalf("replay: %v %v", err, replayed)
			}
			assertParent(t, store, domain.ExecFailed)
		})
	}

	t.Run("healthy human resume remains queued and explicit cancel may abandon", func(t *testing.T) {
		store := newStore(t)
		seed(t, store, domain.ExecWaiting, domain.StepInterrupted, domain.StepTimedOut)
		c := newCoordinator(store)
		req := coordinator.HumanStepRequest{Key: "resume", ExecutionID: "e1", StepID: "s0", By: domain.Actor{Kind: domain.ActorHuman}, ActorID: "owner"}
		if out, _, err := c.ResumeStep(ctx, req); err != nil || out.State != domain.StepQueued {
			t.Fatalf("resume: %+v %v", out, err)
		}
		if _, _, err := c.Cancel(ctx, coordinator.CancelRequest{Key: "cancel", ExecutionID: "e1", By: req.By, ActorID: req.ActorID}); err != nil {
			t.Fatal(err)
		}
		assertParent(t, store, domain.ExecCancelled)
	})

	t.Run("failure choice is sorted and re-redacted", func(t *testing.T) {
		store := newStore(t)
		secret := "ghp_" + strings.Repeat("k3", 18)
		e := domain.Execution{ID: "e1", State: domain.ExecRunning, CreatedAt: t0, StartedAt: &t0}
		ss := []domain.StepExecution{
			{ID: "z", ExecutionID: "e1", State: domain.StepFailed, EndedAt: &t0, Error: &domain.ExecError{Code: "other", Message: "wrong failure"}},
			{ID: "a", ExecutionID: "e1", State: domain.StepFailed, EndedAt: &t0, Error: &domain.ExecError{Code: secret, Message: "error " + secret}},
		}
		if _, _, err := store.Create(ctx, e, ss); err != nil {
			t.Fatal(err)
		}

		if _, err := newCoordinator(store).Reconcile(ctx); err != nil {
			t.Fatal(err)
		}
		e = assertParent(t, store, domain.ExecFailed)
		if e.Error == nil || strings.Contains(e.Error.Message, secret) || strings.Contains(e.Error.Code, secret) || e.Error.Message != "error "+domain.RedactionMarker {
			t.Fatalf("parent error: %+v", e.Error)
		}
	})

	t.Run("a failure drains held work before finalizing", func(t *testing.T) {
		store := newStore(t)
		c := limited(store)
		got := submit(t, c, "run", "p1", "", 3)
		s := startSession(t, store, c, nl, "w1")
		s.send(ports.Offer{FreeSlots: 2})
		as := []ports.Assignment{s.assignment(), s.assignment()}
		for _, a := range as {
			s.send(ports.AcceptAttempt{AttemptID: a.AttemptID, FencingToken: a.Lease.FencingToken})
		}
		s.send(ports.Result{AttemptID: as[0].AttemptID, FencingToken: as[0].Lease.FencingToken, Outcome: ports.OutcomeFailed})
		for range 2 {
			switch m := s.recv().(type) {
			case ports.ResultAck:
				if m.Disposition != ports.ResultAccepted {
					t.Fatal(m)
				}
			case ports.Cancel:
				if m.AttemptID != as[1].AttemptID {
					t.Fatal(m)
				}
			default:
				t.Fatalf("unexpected message %T", m)
			}
		}
		e, ss, _ := store.Get(ctx, got.Execution.ID)
		if e.State.Terminal() || ss[2].State != domain.StepCancelled {
			t.Fatalf("drain finalized early: %s %+v", e.State, ss)
		}
		s.send(ports.Result{AttemptID: as[1].AttemptID, FencingToken: as[1].Lease.FencingToken, Outcome: ports.OutcomeSucceeded})
		if ack, ok := s.recv().(ports.ResultAck); !ok || ack.Disposition != ports.ResultAccepted {
			t.Fatalf("ack %+v", ack)
		}
		e, ss, _ = store.Get(ctx, got.Execution.ID)
		if e.State != domain.ExecFailed || ss[1].State != domain.StepSucceeded {
			t.Fatalf("winner lost: %s %+v", e.State, ss[1])
		}
		s.send(ports.Offer{FreeSlots: 1})
		s.expectNothing()
	})

	t.Run("concurrent final results append exactly one parent Event", func(t *testing.T) {
		store := newStore(t)
		c := newCoordinator(store)
		queued(t, store, "e1", t0, "s0", "s1")
		sessions := []*session{startSession(t, store, c, nl, "w0"), startSession(t, store, c, nl, "w1")}
		as := make([]ports.Assignment, 2)
		for i, s := range sessions {
			s.send(ports.Offer{FreeSlots: 1})
			as[i] = s.assignment()
			s.send(ports.AcceptAttempt{AttemptID: as[i].AttemptID, FencingToken: as[i].Lease.FencingToken})
		}
		var wg sync.WaitGroup
		for i, s := range sessions {
			wg.Add(1)
			go func(i int, s *session) {
				defer wg.Done()
				s.send(ports.Result{AttemptID: as[i].AttemptID, FencingToken: as[i].Lease.FencingToken, Outcome: ports.OutcomeSucceeded})
			}(i, s)
		}
		wg.Wait()
		for _, s := range sessions {
			if ack, ok := s.recv().(ports.ResultAck); !ok || ack.Disposition != ports.ResultAccepted {
				t.Fatalf("ack %+v", ack)
			}
		}
		assertParent(t, store, domain.ExecSucceeded)
	})

	t.Run("a cancel committed before finalization wins", func(t *testing.T) {
		base := newStore(t)
		seed(t, base, domain.ExecRunning, domain.StepSucceeded)
		store := &terminalStore{ExecutionStore: base}
		store.before = func(ch ports.ExecutionChange) {
			if ch.Execution == nil || ch.Execution.State != domain.ExecSucceeded {
				return
			}
			store.before = nil
			if _, _, err := newCoordinator(base).Cancel(ctx, coordinator.CancelRequest{Key: "cancel", ExecutionID: "e1", By: domain.Actor{Kind: domain.ActorHuman}, ActorID: "owner"}); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := newCoordinator(store).Reconcile(ctx); err != nil {
			t.Fatal(err)
		}
		assertParent(t, base, domain.ExecCancelled)
	})

	t.Run("failure voids requested Approvals without a decision", func(t *testing.T) {
		store := newStore(t)
		c := coordinator.New(store, clock, NewOpenTrust().Option(), coordinator.WithRules(domain.PolicyRule{
			ID: "allow-check", Source: domain.SourceOrganization, Effect: domain.EffectAllow, Capabilities: []string{"code.test"},
		}))
		got, _, err := c.Submit(ctx, coordinator.SubmitRequest{Key: "run", ProjectID: "p1", By: Requester,
			Steps: []coordinator.StepSpec{{Name: "check", Capability: "code.test"}, {Name: "push", Capability: "git.push"}}})
		if err != nil || len(got.Approvals) != 1 {
			t.Fatalf("submit: %+v %v", got, err)
		}
		sess := startSession(t, store, c, nl, "w1")
		sess.send(ports.Offer{FreeSlots: 1})
		a := sess.assignment()
		sess.send(ports.AcceptAttempt{AttemptID: a.AttemptID, FencingToken: a.Lease.FencingToken})
		sess.send(ports.Result{AttemptID: a.AttemptID, FencingToken: a.Lease.FencingToken, Outcome: ports.OutcomeFailed})
		sess.recv()
		appr, err := store.Approval(ctx, got.Approvals[0].ID)
		if err != nil || appr.State != domain.ApprovalVoided || appr.Decision != nil {
			t.Fatalf("Approval %+v %v", appr, err)
		}
		e, ss, _ := store.Get(ctx, got.Execution.ID)
		if e.State != domain.ExecFailed || ss[1].State != domain.StepCancelled {
			t.Fatalf("drain %s %+v", e.State, ss)
		}
		audit, _ := store.Audit(ctx, 0, 0)
		if _, err := c.Reconcile(ctx); err != nil {
			t.Fatal(err)
		}
		after, _ := store.Audit(ctx, 0, 0)
		if len(audit) != 2 || len(after) != len(audit) {
			t.Fatal("Approval withdrawal was not exactly once")
		}
	})

	t.Run("prestart rejection drains sibling Approvals and retains safe failure", func(t *testing.T) {
		store := newStore(t)
		c := coordinator.New(store, clock, NewOpenTrust().Option())
		got, _, err := c.Submit(ctx, coordinator.SubmitRequest{Key: "run", ProjectID: "p1", By: Requester,
			Steps: []coordinator.StepSpec{{Name: "push", Capability: "git.push"}, {Name: "push2", Capability: "git.push"}}})
		if err != nil || len(got.Approvals) != 2 {
			t.Fatalf("submit %+v %v", got, err)
		}
		a := got.Approvals[0]
		req := coordinator.DecideRequest{Key: "reject", ApprovalID: a.ID, By: domain.Principal{Kind: domain.PrincipalHuman, ID: "owner", Role: domain.RoleOwner}, Outcome: domain.OutcomeRejected, Rationale: "private rationale", EvidenceDigest: a.Evidence.Digest}
		if _, _, err := c.DecideApproval(ctx, req); err != nil {
			t.Fatal(err)
		}
		e, ss, _ := store.Get(ctx, got.Execution.ID)
		other, _ := store.Approval(ctx, got.Approvals[1].ID)
		if e.State != domain.ExecFailed || e.StartedAt != nil || e.EndedAt == nil || e.Error == nil || e.Error.Message == req.Rationale || ss[1].State != domain.StepCancelled || other.State != domain.ApprovalVoided {
			t.Fatalf("parent %+v sibling %+v Approval %+v", e, ss[1], other)
		}
		events, _ := store.Events(ctx, e.ID)
		if _, replayed, err := c.DecideApproval(ctx, req); err != nil || !replayed {
			t.Fatalf("replay %v %v", replayed, err)
		}
		after, _ := store.Events(ctx, e.ID)
		if len(after) != len(events) {
			t.Fatal("replay duplicated terminal Events")
		}
	})

	t.Run("resume loses to a sibling failure without changing the wait", func(t *testing.T) {
		base := newStore(t)
		seed(t, base, domain.ExecWaiting, domain.StepInterrupted, domain.StepQueued)
		store := &terminalStore{ExecutionStore: base}
		store.before = func(ch ports.ExecutionChange) {
			if ch.Command == nil || ch.Command.Name != coordinator.CmdResumeStep {
				return
			}
			store.before = nil
			_, ss, err := base.Get(ctx, "e1")
			if err != nil {
				t.Fatal(err)
			}
			ss[1].State, ss[1].EndedAt, ss[1].Error = domain.StepFailed, &now, &domain.ExecError{Code: "failed", Message: "failed"}
			if _, err := base.Commit(ctx, ports.ExecutionChange{Steps: ss[1:]}); err != nil {
				t.Fatal(err)
			}
		}
		_, _, err := newCoordinator(store).ResumeStep(ctx, coordinator.HumanStepRequest{Key: "resume", ExecutionID: "e1", StepID: "s0", By: domain.Actor{Kind: domain.ActorHuman}, ActorID: "owner"})
		if !errors.Is(err, domain.ErrInvalidTransition) {
			t.Fatalf("resume: %v", err)
		}
		_, ss, _ := base.Get(ctx, "e1")
		if ss[0].State != domain.StepInterrupted || ss[0].Version != 1 {
			t.Fatal("losing resume changed the human wait")
		}
	})

	t.Run("a crash before the parent write is repaired before duplicate acknowledgement", func(t *testing.T) {
		base := newStore(t)
		queued(t, base, "e1", t0, "s0")
		store := &boundaryStore{ExecutionStore: base, before: func(n int) error {
			if n == 4 {
				return errCrash
			}
			return nil
		}}
		pl, wl := nl(t)
		runCtx, stop := context.WithTimeout(ctx, wait)
		defer stop()
		defer wl.Close()
		defer pl.Close()
		done := make(chan error, 1)
		go func() { done <- newCoordinator(store).Serve(runCtx, pl, "w1") }()
		if err := wl.Send(runCtx, ports.Offer{FreeSlots: 1}); err != nil {
			t.Fatal(err)
		}
		message, err := wl.Receive(runCtx)
		if err != nil {
			t.Fatal(err)
		}
		a := message.(ports.Assignment)
		if err := wl.Send(runCtx, ports.AcceptAttempt{AttemptID: a.AttemptID, FencingToken: a.Lease.FencingToken}); err != nil {
			t.Fatal(err)
		}
		result := ports.Result{AttemptID: a.AttemptID, FencingToken: a.Lease.FencingToken, Outcome: ports.OutcomeSucceeded}
		if err := wl.Send(runCtx, result); err != nil {
			t.Fatal(err)
		}
		select {
		case err := <-done:
			if !errors.Is(err, errCrash) {
				t.Fatalf("crash: %v", err)
			}
		case <-runCtx.Done():
			t.Fatal("coordinator did not reach crash boundary")
		}
		e, ss, _ := base.Get(ctx, "e1")
		if e.State != domain.ExecRunning || ss[0].State != domain.StepSucceeded {
			t.Fatalf("wrong crash boundary: %s %s", e.State, ss[0].State)
		}
		quietCtx, cancel := context.WithTimeout(ctx, quiet)
		defer cancel()
		if m, err := wl.Receive(quietCtx); err == nil {
			t.Fatalf("acknowledged before parent commit: %#v", m)
		}
		c := newCoordinator(base)
		if _, err := c.Reconcile(ctx); err != nil {
			t.Fatal(err)
		}
		before := assertParent(t, base, domain.ExecSucceeded)
		sess := startSession(t, base, c, nl, "w2")
		sess.send(result)
		if ack, ok := sess.recv().(ports.ResultAck); !ok || ack.Disposition != ports.ResultDuplicate {
			t.Fatalf("retry %+v", ack)
		}
		after := assertParent(t, base, domain.ExecSucceeded)
		if before.Version != after.Version {
			t.Fatal("duplicate changed parent after crash repair")
		}
	})

	t.Run("explicit cancellation overrides failure and may abandon human waits", func(t *testing.T) {
		store := newStore(t)
		seed(t, store, domain.ExecWaiting, domain.StepFailed, domain.StepTimedOut, domain.StepInterrupted, domain.StepPending)
		if _, _, err := newCoordinator(store).Cancel(ctx, coordinator.CancelRequest{Key: "cancel", ExecutionID: "e1", By: domain.Actor{Kind: domain.ActorHuman}, ActorID: "owner"}); err != nil {
			t.Fatal(err)
		}
		assertParent(t, store, domain.ExecCancelled)
	})

	t.Run("a competing healthy resume prevents abandonment from finalizing", func(t *testing.T) {
		base := newStore(t)
		seed(t, base, domain.ExecWaiting, domain.StepInterrupted)
		store := &terminalStore{ExecutionStore: base}
		req := coordinator.HumanStepRequest{Key: "abandon", ExecutionID: "e1", StepID: "s0", By: domain.Actor{Kind: domain.ActorHuman}, ActorID: "owner"}
		store.before = func(ch ports.ExecutionChange) {
			if ch.Command == nil || ch.Command.Name != coordinator.CmdAbandonStep {
				return
			}
			store.before = nil
			resume := req
			resume.Key = "resume"
			if _, _, err := newCoordinator(base).ResumeStep(ctx, resume); err != nil {
				t.Fatal(err)
			}
		}
		if _, _, err := newCoordinator(store).AbandonStep(ctx, req); !errors.Is(err, domain.ErrInvalidTransition) {
			t.Fatalf("abandon after resume: %v", err)
		}
		assertParent(t, base, domain.ExecWaiting)
		_, ss, _ := base.Get(ctx, "e1")
		if ss[0].State != domain.StepQueued {
			t.Fatal("lost winning resume")
		}
	})

	t.Run("an Approval decision cannot cross a sibling failure", func(t *testing.T) {
		base := newStore(t)
		store := &terminalStore{ExecutionStore: base}
		c := coordinator.New(store, clock, NewOpenTrust().Option())
		got, _, err := c.Submit(ctx, coordinator.SubmitRequest{Key: "run", ProjectID: "p1", By: Requester,
			Steps: []coordinator.StepSpec{{Name: "push", Capability: "git.push"}, {Name: "push2", Capability: "git.push"}}})
		if err != nil || len(got.Approvals) != 2 {
			t.Fatalf("submit %+v %v", got, err)
		}
		a := got.Approvals[0]
		store.before = func(ch ports.ExecutionChange) {
			if ch.Command == nil || ch.Command.Name != coordinator.CmdDecideApproval {
				return
			}
			store.before = nil
			_, ss, err := base.Get(ctx, got.Execution.ID)
			if err != nil {
				t.Fatal(err)
			}
			if err := ss[1].Reject(now, &domain.ExecError{Code: "failed", Message: "failed"}); err != nil {
				t.Fatal(err)
			}
			if _, err := base.Commit(ctx, ports.ExecutionChange{Steps: ss[1:]}); err != nil {
				t.Fatal(err)
			}
		}
		_, _, err = c.DecideApproval(ctx, coordinator.DecideRequest{Key: "approve", ApprovalID: a.ID, By: domain.Principal{Kind: domain.PrincipalHuman, ID: "owner", Role: domain.RoleOwner}, Outcome: domain.OutcomeApproved, EvidenceDigest: a.Evidence.Digest})
		if !errors.Is(err, domain.ErrInvalidTransition) {
			t.Fatalf("approval: %v", err)
		}
		after, _ := base.Approval(ctx, a.ID)
		if after.State != domain.ApprovalVoided || after.Decision != nil {
			t.Fatalf("losing approval changed gate: %+v", after)
		}
	})

	t.Run("missing failure details use safe defaults", func(t *testing.T) {
		store := newStore(t)
		seed(t, store, domain.ExecRunning, domain.StepFailed)
		_, ss, _ := store.Get(ctx, "e1")
		ss[0].Error = nil
		if _, err := store.Commit(ctx, ports.ExecutionChange{Steps: ss}); err != nil {
			t.Fatal(err)
		}
		if _, err := newCoordinator(store).Reconcile(ctx); err != nil {
			t.Fatal(err)
		}
		e := assertParent(t, store, domain.ExecFailed)
		if e.Error == nil || e.Error.Code != "step_failed" || e.Error.Message != "A Step failed." {
			t.Fatalf("fallback %+v", e.Error)
		}
	})

	t.Run("a lease cannot cross a durable sibling failure", func(t *testing.T) {
		base := newStore(t)
		queued(t, base, "e1", t0, "s0", "s1")
		store := &terminalStore{ExecutionStore: base}
		store.before = func(ch ports.ExecutionChange) {
			if len(ch.Steps) != 1 || ch.Steps[0].State != domain.StepLeased {
				return
			}
			store.before = nil
			_, ss, err := base.Get(ctx, "e1")
			if err != nil {
				t.Fatal(err)
			}
			ss[1].State, ss[1].EndedAt, ss[1].Error = domain.StepFailed, &now, &domain.ExecError{Code: "failed", Message: "failed"}
			// A Step-only writer demonstrates why checking just the parent is insufficient.
			if _, err := base.Commit(ctx, ports.ExecutionChange{Steps: ss[1:]}); err != nil {
				t.Fatal(err)
			}
		}
		s := startSession(t, store, newCoordinator(store), nl, "w1")
		s.send(ports.Offer{FreeSlots: 1})
		s.expectNothing()
		if _, err := newCoordinator(base).Reconcile(ctx); err != nil {
			t.Fatal(err)
		}
		assertParent(t, base, domain.ExecFailed)
	})
}
