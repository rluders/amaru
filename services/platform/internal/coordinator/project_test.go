package coordinator_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rluders/amaru/services/platform/internal/coordinator"
	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/evidence"
	"github.com/rluders/amaru/services/platform/internal/ports/fakes"
)

func TestProjectAccess(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
	owner := domain.Principal{Kind: domain.PrincipalHuman, ID: "owner", Role: domain.RoleOwner}
	a := domain.ProjectKey("ssh://github.com/acme/a#0123456789abcdef0123456789abcdef01234567")
	b := domain.ProjectKey("ssh://github.com/acme/b#0123456789abcdef0123456789abcdef01234567")
	projects, store := fakes.NewProjectStore(), fakes.NewExecutionStore()
	trust := fakes.NewTrustStore(projects)
	fakes.LinkTrustAudit(store, trust)
	var ids atomic.Int64
	c := coordinator.New(store, func() time.Time { return now }, coordinator.WithTrust(trust), coordinator.WithExecutionIDs(func() string { return fmt.Sprintf("exec-%d", ids.Add(1)) }), coordinator.WithApprovalIDs(func() string { return fmt.Sprintf("approval-%d", ids.Add(1)) }))
	for _, p := range []domain.ProjectKey{a, b} {
		if err := projects.Create(ctx, domain.Project{Key: p, Name: "same-name", CreatedAt: now}); err != nil {
			t.Fatal(err)
		}
		if _, err := c.TrustContent(ctx, "trust-"+string(p), owner, p, domain.TrustProject, "", ""); err != nil {
			t.Fatal(err)
		}
	}
	var revoked atomic.Bool
	api, err := coordinator.NewProjectAccess(c, projects, func(context.Context) (domain.Principal, error) {
		if revoked.Load() {
			return domain.Principal{}, domain.ErrNotPermitted
		}
		return owner, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	submit := func(project domain.ProjectKey, key string) coordinator.Submitted {
		t.Helper()
		out, replay, err := api.Submit(ctx, project, coordinator.SubmitRequest{Key: key, ProjectID: "spoof", By: domain.Principal{ID: "spoof"}, Trigger: domain.TriggerWebhook, Steps: []coordinator.StepSpec{{Name: "same-name", Capability: "git.push"}}})
		if err != nil || replay || out.Execution.ProjectID != string(project) || len(out.Approvals) != 1 {
			t.Fatalf("submit: %+v replay=%v err=%v", out, replay, err)
		}
		if out.Approvals[0].Requester != owner {
			t.Fatalf("spoofed requester: %+v", out.Approvals[0].Requester)
		}
		return out
	}
	first, second := submit(a, "submit-a"), submit(b, "submit-b")
	if first.Execution.ID == second.Execution.ID {
		t.Fatal("shared Execution")
	}
	contexts, artifacts := fakes.NewContextStore(), fakes.NewArtifactIndex()
	snapshot := func() string {
		t.Helper()
		var all []any
		for _, id := range []string{first.Execution.ID, second.Execution.ID} {
			e, st, err := store.Get(ctx, id)
			if err != nil {
				t.Fatal(err)
			}
			ev, err := store.Events(ctx, id)
			if err != nil {
				t.Fatal(err)
			}
			ap, err := store.Approvals(ctx, id)
			if err != nil {
				t.Fatal(err)
			}
			all = append(all, e, st, ev, ap)
		}
		audit, err := store.Audit(ctx, 0, 0)
		if err != nil {
			t.Fatal(err)
		}
		all = append(all, audit)
		raw, err := json.Marshal(all)
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}
	decision := coordinator.DecideRequest{Key: "decision-b", ApprovalID: second.Approvals[0].ID, By: owner, Outcome: domain.OutcomeApproved, EvidenceDigest: second.Approvals[0].Evidence.Digest}
	if _, replayed, err := api.DecideApproval(ctx, b, decision); err != nil || replayed {
		t.Fatalf("own decision: %v %v", replayed, err)
	}
	cancel := coordinator.CancelRequest{Key: "cancel-b", ExecutionID: second.Execution.ID, Reason: "stop", By: domain.Actor{Kind: domain.ActorKind("agent")}, ActorID: "spoof"}
	if _, replayed, err := api.Cancel(ctx, b, cancel); err != nil || replayed {
		t.Fatalf("own cancel: %v %v", replayed, err)
	}
	ownDecision := decision
	ownDecision.Key, ownDecision.ApprovalID, ownDecision.EvidenceDigest = "decision-a", first.Approvals[0].ID, first.Approvals[0].Evidence.Digest
	if _, _, err := api.DecideApproval(ctx, a, ownDecision); err != nil {
		t.Fatal(err)
	}
	before := snapshot()
	for _, id := range []string{second.Execution.ID, "missing"} {
		t.Run("foreign or missing "+id, func(t *testing.T) {
			e, st, err := api.Execution(ctx, a, id)
			if err != domain.ErrNotFound || !reflect.DeepEqual(e, domain.Execution{}) || st != nil {
				t.Fatalf("Execution leaked: %+v %+v %v", e, st, err)
			}
			w, err := api.Definition(ctx, a, id)
			if err != domain.ErrNotFound || !reflect.DeepEqual(w, domain.Workflow{}) {
				t.Fatalf("definition leaked: %+v %v", w, err)
			}
			// Nil child stores prove ownership is checked before Evidence assembles any child data.
			ev, err := api.Evidence(ctx, a, id, nil)
			if err != domain.ErrNotFound || !reflect.DeepEqual(ev, domain.ExecutionEvidence{}) {
				t.Fatalf("Evidence leaked: %+v %v", ev, err)
			}
			page, err := api.Events(ctx, domain.EventQuery{ProjectID: string(a), ExecutionID: id, After: 999, Limit: 20}, nil, time.Hour)
			if err != domain.ErrNotFound || !reflect.DeepEqual(page, domain.EventPage{}) {
				t.Fatalf("cursor leaked: %+v %v", page, err)
			}
			req := cancel
			req.ExecutionID = id
			out, replay, err := api.Cancel(ctx, a, req)
			if err != domain.ErrNotFound || replay || !reflect.DeepEqual(out, coordinator.Cancelled{}) {
				t.Fatalf("cancel leaked replay: %+v %v %v", out, replay, err)
			}
		})
	}
	for _, id := range []string{second.Approvals[0].ID, "missing"} {
		approval, err := api.Approval(ctx, a, id)
		if err != domain.ErrNotFound || !reflect.DeepEqual(approval, domain.Approval{}) {
			t.Fatalf("Approval leaked: %+v %v", approval, err)
		}
		req := decision
		req.ApprovalID = id
		out, replay, err := api.DecideApproval(ctx, a, req)
		if err != domain.ErrNotFound || replay || !reflect.DeepEqual(out, coordinator.Decided{}) {
			t.Fatalf("decision leaked replay: %+v %v %v", out, replay, err)
		}
	}
	if snapshot() != before {
		t.Fatal("rejected foreign/missing access mutated rows, Events or audit")
	}
	// Namespace remains (command name,key): Project B cannot receive A's outcome or create a new one.
	_, _, err = api.Submit(ctx, b, coordinator.SubmitRequest{Key: "submit-a", Steps: []coordinator.StepSpec{{Name: "same-name", Capability: "git.push"}}})
	if !errors.Is(err, domain.ErrKeyReused) {
		t.Fatalf("cross-Project command key: %v", err)
	}
	replayedSubmit, replayed, err := api.Submit(ctx, a, coordinator.SubmitRequest{Key: "submit-a", Steps: []coordinator.StepSpec{{Name: "same-name", Capability: "git.push"}}})
	if err != nil || !replayed || !reflect.DeepEqual(replayedSubmit, first) {
		t.Fatalf("own Submit replay: %+v %v %v", replayedSubmit, replayed, err)
	}
	if ids.Load() != 4 {
		t.Fatalf("conflicting key allocated an Execution/Approval: %d", ids.Load())
	}
	if snapshot() != before {
		t.Fatal("conflicting key mutated state")
	}
	if _, _, err := api.Execution(ctx, a, first.Execution.ID); err != nil {
		t.Fatal(err)
	}
	bundle, err := api.Evidence(ctx, a, first.Execution.ID, evidence.Reader{Executions: store, Contexts: contexts, Artifacts: artifacts})
	if err != nil || bundle.Execution.ID != first.Execution.ID {
		t.Fatalf("own Evidence: %+v %v", bundle, err)
	}
	page, err := api.Events(ctx, domain.EventQuery{ProjectID: string(a), Limit: 100}, store, time.Hour)
	if err != nil || len(page.Events) == 0 {
		t.Fatalf("own Events: %+v %v", page, err)
	}
	for _, ev := range page.Events {
		if ev.ProjectID != string(a) || ev.Event.ExecutionID != first.Execution.ID {
			t.Fatal("foreign Event")
		}
	}
	// Concurrent duplicate replay is read-only, and attribution is taken from authentication.
	var wg sync.WaitGroup
	for range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			out, replayed, err := api.Cancel(ctx, b, cancel)
			if err != nil || !replayed || out.ExecutionID != second.Execution.ID {
				t.Errorf("own concurrent replay: %+v %v %v", out, replayed, err)
			}
		}()
	}
	wg.Wait()
	if snapshot() != before {
		t.Fatal("duplicate replay mutated state")
	}
	workflow := domain.Workflow{ID: "same-workflow", Name: "Same workflow", Status: domain.WorkflowActive, Steps: []domain.WorkflowStep{{ID: "test", Name: "Test", Type: domain.StepTypeTool, Capability: "code.test"}}}
	pinned, _, err := api.Submit(ctx, a, coordinator.SubmitRequest{Key: "pinned", Workflow: &workflow})
	if err != nil {
		t.Fatal(err)
	}
	gotWorkflow, err := api.Definition(ctx, a, pinned.Execution.ID)
	if err != nil || gotWorkflow.ID != workflow.ID {
		t.Fatalf("own pinned definition: %+v %v", gotWorkflow, err)
	}
	if _, err := api.Definition(ctx, b, pinned.Execution.ID); err != domain.ErrNotFound {
		t.Fatalf("foreign pinned definition: %v", err)
	}
	revoked.Store(true)
	if _, _, err := api.Cancel(ctx, b, cancel); err != domain.ErrNotPermitted {
		t.Fatalf("revoked replay: %v", err)
	}
	if _, _, err := api.Submit(ctx, a, coordinator.SubmitRequest{Key: "submit-a"}); err != domain.ErrNotPermitted {
		t.Fatalf("revoked Submit replay: %v", err)
	}
	if _, _, err := api.DecideApproval(ctx, b, decision); err != domain.ErrNotPermitted {
		t.Fatalf("revoked decision replay: %v", err)
	}
	if _, _, err := api.Execution(ctx, a, first.Execution.ID); err != domain.ErrNotPermitted {
		t.Fatalf("revoked query: %v", err)
	}
	if snapshot() != before {
		t.Fatal("revoked access mutated state")
	}
	revoked.Store(false)
	for _, project := range []domain.ProjectKey{"", "invalid", "ssh://github.com/acme/missing#0123456789abcdef0123456789abcdef01234567"} {
		if _, _, err := api.Cancel(ctx, project, cancel); err != domain.ErrNotFound {
			t.Fatalf("unattached Project: %v", err)
		}
	}
	for _, by := range []domain.Principal{{}, {Kind: domain.PrincipalAgent, ID: "agent", Role: domain.RoleOwner}, {Kind: domain.PrincipalHuman, ID: "reader", Role: domain.RoleViewer}, {Kind: domain.PrincipalHuman, ID: "owner", Role: domain.RoleOwner, Sponsor: "spoof"}} {
		denied, err := coordinator.NewProjectAccess(c, projects, func(context.Context) (domain.Principal, error) { return by, nil })
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err := denied.Cancel(ctx, b, cancel); err != domain.ErrNotPermitted {
			t.Fatalf("invalid identity %+v: %v", by, err)
		}
	}
}
