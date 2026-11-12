package coordtest

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"testing"

	"github.com/rluders/amaru/services/platform/internal/coordinator"
	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
)

// ProcessCrashInjection runs the state matrix in real child processes. run opens the same
// database in a fresh process, kills it at point, or lets it exit for point "reconcile".
func ProcessCrashInjection(t *testing.T, newStore func(*testing.T) ports.ExecutionStore, run func(t *testing.T, scenario, point string)) {
	ctx := context.Background()
	for _, sc := range scenarios() {
		t.Run("state/"+sc.name, func(t *testing.T) {
			store := newStore(t)
			run(t, sc.name, "state")
			run(t, "", "reconcile")
			e, steps, err := store.Get(ctx, "e1")
			if err != nil {
				t.Fatal(err)
			}
			st, evs := steps[0], readEvents(t, store)
			if st.State != sc.step || e.State != sc.exec || (sc.action != "" && st.Reason != sc.reason) {
				t.Fatalf("after kill: Step=%+v Execution=%+v", st, e)
			}
			if sc.action == domain.RestartResumed && (st.RemoteJob == nil || st.RemoteJob.ID != "job-1") {
				t.Fatal("lost external job id")
			}
			run(t, "", "reconcile")
			e2, steps2, err := store.Get(ctx, "e1")
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(e, e2) || !reflect.DeepEqual(steps, steps2) || !reflect.DeepEqual(evs, readEvents(t, store)) {
				t.Fatal("second process restart changed settled state")
			}
		})
	}
	for _, point := range []string{"before-1", "after-1", "before-2", "after-2", "before-3", "after-3", "before-4", "after-4", "before-5", "after-5", "before-6", "after-6", "assignment", "ack"} {
		t.Run("protocol/"+point, func(t *testing.T) {
			store := newStore(t)
			run(t, "protocol", point)
			// The child died at a known write/message boundary, so no timing assumptions are needed.
			durable := 0
			switch point {
			case "assignment":
				durable = 1
			case "ack":
				durable = 6
			default:
				var n int
				if _, err := fmt.Sscanf(point, "before-%d", &n); err == nil {
					durable = n - 1
				} else if _, err := fmt.Sscanf(point, "after-%d", &n); err == nil {
					durable = n
				} else {
					t.Fatal(point)
				}
			}
			verifyProtocolCommit(t, store, durable)
			run(t, "", "reconcile")
			run(t, "", "reconcile")
			verifyRecovery(t, store, func(_ *testing.T, s ports.ExecutionStore) ports.ExecutionStore { return s })
		})
	}
	t.Run("persisted Approval gate", func(t *testing.T) {
		store := newStore(t)
		run(t, "", "approval")
		before, err := store.Approval(ctx, "approval-1")
		if err != nil {
			t.Fatal(err)
		}
		initialExecution, initialSteps, err := store.Get(ctx, "e1")
		if err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 2; i++ {
			run(t, "", "reconcile")
			after, err := store.Approval(ctx, before.ID)
			if err != nil {
				t.Fatal(err)
			}
			e, steps, err := store.Get(ctx, "e1")
			if err != nil {
				t.Fatal(err)
			}
			// Submit leaves an Execution queued until its first Step starts; recovery preserves it.
			if !reflect.DeepEqual(before, after) || !reflect.DeepEqual(initialExecution, e) || !reflect.DeepEqual(initialSteps, steps) || steps[0].State != domain.StepAwaitingApproval {
				t.Fatal("restart changed Approval gate or released its Step")
			}
		}
	})
}

// ProcessCrashChild is called only by the PostgreSQL helper test in a child test binary.
// Its dedicated pipe signals readiness; stdin stays open until the parent kills the process.
func ProcessCrashChild(t *testing.T, store ports.ExecutionStore, scenario, point string) {
	t.Helper()
	barrier := func() {
		f := os.NewFile(3, "crash-ready")
		if f == nil {
			t.Fatal("missing readiness pipe")
		}
		if _, err := f.Write([]byte{1}); err != nil {
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
		var b [1]byte
		_, err := os.Stdin.Read(b[:])
		t.Fatalf("crash barrier released instead of process kill: %v", err)
	}
	if point == "reconcile" {
		if _, err := coordinator.New(store, clock).Reconcile(context.Background()); err != nil {
			t.Fatal(err)
		}
		return
	}
	if point == "state" {
		seed(t, store, named(t, scenario))
		barrier()
		return
	}
	if point == "approval" {
		c := coordinator.New(store, clock, NewOpenTrust().Option(),
			coordinator.WithExecutionIDs(func() string { return "e1" }),
			coordinator.WithApprovalIDs(func() string { return "approval-1" }))
		got, _, err := c.Submit(context.Background(), coordinator.SubmitRequest{By: Requester, Key: "approval", ProjectID: "p1",
			Steps: []coordinator.StepSpec{{Name: "push", Capability: "git.push"}}})
		if err != nil {
			t.Fatal(err)
		}
		if len(got.Approvals) != 1 {
			t.Fatal("expected persisted Approval")
		}
		barrier()
		return
	}
	queued(t, store, "e1", t0, "s1")
	fault := &boundaryStore{ExecutionStore: store,
		before: func(n int) error {
			if point == fmt.Sprintf("before-%d", n) {
				barrier()
			}
			return nil
		},
		after: func(n int) error {
			if point == fmt.Sprintf("after-%d", n) {
				barrier()
			}
			return nil
		},
	}
	link := &protocolLink{messages: protocolMessages(), send: func(m ports.PlatformMessage) error {
		switch m.(type) {
		case ports.Assignment:
			if point == "assignment" {
				barrier()
			}
		case ports.ResultAck:
			if point == "ack" {
				barrier()
			}
			return errCrash
		}
		return nil
	}}
	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()
	if err := newCoordinator(fault).Serve(ctx, link, "w1"); err != nil {
		t.Fatal(err)
	}
	t.Fatal("did not reach crash boundary")
}
