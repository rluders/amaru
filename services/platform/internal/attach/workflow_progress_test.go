package attach_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rluders/amaru/services/platform/internal/attach"
	"github.com/rluders/amaru/services/platform/internal/coordinator"
	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
	"github.com/rluders/amaru/services/platform/internal/ports/fakes"
	"github.com/rluders/amaru/services/platform/internal/worker/embedded"
	"github.com/rluders/amaru/services/platform/internal/worker/inproc"
)

// Connect committed definition selection to dispatch, preserving Project isolation.
func TestCommittedWorkflowsProgressIndependently(t *testing.T) {
	needGit(t)
	ctx := context.Background()
	attacher, projects := newAttacher()
	store := fakes.NewExecutionStore()
	trust := fakes.NewTrustStore(projects)
	fakes.LinkTrustAudit(store, trust)
	owner := domain.Principal{Kind: domain.PrincipalHuman, ID: "owner", Role: domain.RoleOwner}
	type project struct {
		key            domain.ProjectKey
		root, revision string
	}
	commit := func(root, content string) string {
		t.Helper()
		writeWorkflow(t, root, content)
		git(t, root, "add", ".")
		git(t, root, "commit", "-qm", "definition")
		return git(t, root, "rev-parse", "HEAD")
	}
	var both []project
	for _, name := range []string{"affected", "healthy"} {
		root := repo(t, "https://example.com/"+name+".git")
		content := workflowFile(name)
		if name == "affected" {
			content = strings.Replace(content, "code.test", "git.push", 1)
		}
		revision := commit(root, content)
		p, _, err := attacher.Attach(ctx, root)
		if err != nil {
			t.Fatal(err)
		}
		both = append(both, project{p.Key, root, revision})
	}
	affected, healthy := both[0], both[1]
	reader := attach.WorkflowReader{Git: attacher}
	var ids, goodRuns, badRuns atomic.Int64
	occupied := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(unblock)
	runner := embedded.RunnerFunc(func(ctx context.Context, a ports.Assignment) embedded.Outcome {
		switch a.Capability {
		case "code.test":
			goodRuns.Add(1)
			return embedded.Outcome{Result: ports.OutcomeSucceeded}
		case "check.run":
			if badRuns.Add(1) == 1 {
				close(occupied)
				select {
				case <-release:
				case <-ctx.Done():
					return embedded.Outcome{Result: ports.OutcomeCancelled}
				}
			}
			return embedded.Outcome{Result: ports.OutcomeFailed, ErrorCode: "provider_unavailable", ErrorDetail: "fake unavailable Provider"}
		default:
			t.Errorf("unexpected assignment capability %s", a.Capability)
			return embedded.Outcome{Result: ports.OutcomeFailed}
		}
	})
	start := func() (*coordinator.Coordinator, *coordinator.ProjectAccess, func()) {
		t.Helper()
		c := coordinator.New(store, time.Now, coordinator.WithTrust(trust), coordinator.WithLimits(coordinator.Limits{PerProject: 1}), coordinator.WithExecutionIDs(func() string { return fmt.Sprintf("workflow-progress-%d", ids.Add(1)) }))
		if _, err := c.Reconcile(ctx); err != nil {
			t.Fatal(err)
		}
		access, err := coordinator.NewProjectAccess(c, projects, func(context.Context) (domain.Principal, error) { return owner, nil })
		if err != nil {
			t.Fatal(err)
		}
		pl, wl := inproc.Pair()
		runCtx, cancel := context.WithCancel(ctx)
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			if err := c.Serve(runCtx, pl, "embedded"); err != nil {
				t.Errorf("Serve: %v", err)
			}
		}()
		go func() {
			defer wg.Done()
			if err := (&embedded.Worker{Slots: 2, HeartbeatPeriod: time.Hour, Runner: runner}).Run(runCtx, wl); err != nil {
				t.Errorf("Worker: %v", err)
			}
		}()
		var once sync.Once
		stop := func() { once.Do(func() { cancel(); pl.Close(); wg.Wait() }) }
		t.Cleanup(stop)
		return c, access, stop
	}
	c, access, stop := start()
	for _, p := range both {
		if _, err := c.TrustContent(ctx, "trust-"+string(p.key), owner, p.key, domain.TrustProject, "", ""); err != nil {
			t.Fatal(err)
		}
	}
	run := func(api *coordinator.ProjectAccess, p project, key string) coordinator.Submitted {
		t.Helper()
		definition, err := api.Workflow(ctx, p.key, "same-flow", p.revision, reader)
		if err != nil {
			t.Fatal(err)
		}
		pin, err := domain.Pin(definition, p.revision)
		if err != nil {
			t.Fatal(err)
		}
		// Explicit test-owner trust is a precondition; default Policy remains active.
		if _, err := c.TrustContent(ctx, "trust-workflow-"+key, owner, p.key, domain.TrustWorkflow, definition.ID, pin.Hash); err != nil {
			t.Fatal(err)
		}
		out, replay, err := api.RunWorkflow(ctx, p.key, coordinator.RunWorkflowRequest{Key: key, WorkflowID: "same-flow", Revision: p.revision}, reader)
		if err != nil || replay || out.Execution.ProjectID != string(p.key) || out.Execution.Definition.CloneRoot != p.root {
			t.Fatalf("run %s: %+v %v %v", key, out, replay, err)
		}
		return out
	}
	wait := func(id string, want domain.ExecState) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for {
			e, _, err := store.Get(ctx, id)
			if err != nil {
				t.Fatal(err)
			}
			if e.State == want {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("%s state %s, want %s", id, e.State, want)
			}
			time.Sleep(time.Millisecond)
		}
	}
	snapshot := func(id string) string {
		t.Helper()
		e, steps, err := store.Get(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		events, err := store.Events(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		approvals, err := store.Approvals(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		data, err := json.Marshal([]any{e, steps, events, approvals})
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	waiting := run(access, affected, "waiting")
	if len(waiting.Approvals) != 1 || waiting.Steps[0].State != domain.StepAwaitingApproval {
		t.Fatalf("git.push did not wait: %+v", waiting)
	}
	waitBefore := snapshot(waiting.Execution.ID)
	invalid := commit(affected.root, "invalid: definition\n")
	before := ids.Load()
	out, replay, err := access.RunWorkflow(ctx, affected.key, coordinator.RunWorkflowRequest{Key: "invalid", WorkflowID: "same-flow", Revision: invalid}, reader)
	if err != domain.ErrInvalid || replay || out.Execution.ID != "" || ids.Load() != before {
		t.Fatalf("invalid definition created work: %+v %v %v", out, replay, err)
	}
	affected.revision = commit(affected.root, strings.Replace(workflowFile("affected failure"), "code.test", "check.run", 1))
	failed := run(access, affected, "failure")
	select {
	case <-occupied:
	case <-time.After(5 * time.Second):
		t.Fatal("affected Project did not occupy its slot")
	}
	queued := run(access, affected, "queued-behind-limit")
	good := run(access, healthy, "healthy")
	wait(good.Execution.ID, domain.ExecSucceeded)
	// Healthy completion proves spare Worker capacity and a scan past the limited
	// Project while its first Runner is held. No absence-by-sleep assertion is used.
	_, steps, err := store.Get(ctx, queued.Execution.ID)
	if err != nil || len(steps) != 1 || steps[0].State != domain.StepQueued || steps[0].Reason != domain.ReasonLimitProject || badRuns.Load() != 1 {
		t.Fatalf("Project limit not retained: %+v runs=%d err=%v", steps, badRuns.Load(), err)
	}
	unblock()
	wait(failed.Execution.ID, domain.ExecFailed)
	wait(queued.Execution.ID, domain.ExecFailed)
	stop()
	snapshots := map[string]string{waiting.Execution.ID: waitBefore, failed.Execution.ID: snapshot(failed.Execution.ID), good.Execution.ID: snapshot(good.Execution.ID), queued.Execution.ID: snapshot(queued.Execution.ID)}
	if snapshot(waiting.Execution.ID) != waitBefore {
		t.Fatal("unrelated work changed waiting Execution")
	}
	c, access, stop = start()
	next := run(access, healthy, "healthy-after-recreation")
	wait(next.Execution.ID, domain.ExecSucceeded)
	stop()
	for id, before := range snapshots {
		if snapshot(id) != before {
			t.Fatalf("recreation or healthy work changed %s", id)
		}
	}
	for _, check := range []struct {
		p    project
		run  coordinator.Submitted
		name string
	}{{both[0], waiting, "affected"}, {healthy, good, "healthy"}, {healthy, next, "healthy"}} {
		pinned, err := access.Definition(ctx, check.p.key, check.run.Execution.ID)
		if err != nil || pinned.Name != check.name {
			t.Fatalf("wrong pinned definition: %+v %v", pinned, err)
		}
		foreign := affected.key
		if foreign == check.p.key {
			foreign = healthy.key
		}
		if got, _, err := access.Execution(ctx, foreign, check.run.Execution.ID); err != domain.ErrNotFound || got.ID != "" {
			t.Fatalf("foreign Execution leaked: %+v %v", got, err)
		}
	}
	if goodRuns.Load() != 2 || badRuns.Load() != 2 {
		t.Fatalf("Runner calls: healthy=%d failed=%d", goodRuns.Load(), badRuns.Load())
	}
}
