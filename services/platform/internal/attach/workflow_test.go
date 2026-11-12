package attach_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rluders/amaru/services/platform/internal/attach"
	"github.com/rluders/amaru/services/platform/internal/coordinator"
	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports/fakes"
)

func workflowFile(name string) string {
	return "schemaVersion: 1\nid: same-flow\nname: " + name + "\nstatus: active\nsteps:\n  - id: test\n    name: Same step\n    type: tool\n    capability: code.test\n"
}

func writeWorkflow(t *testing.T, dir, content string) {
	t.Helper()
	path := filepath.Join(dir, ".amaru", "workflows", "same-flow.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestProjectWorkflowSelection(t *testing.T) {
	needGit(t)
	ctx := context.Background()
	a, projects := newAttacher()
	locations := map[domain.ProjectKey]string{}
	type attached struct {
		key                 domain.ProjectKey
		dir, revision, name string
	}
	var both []attached
	for _, name := range []string{"Alpha", "Beta"} {
		dir := repo(t, "https://example.com/"+name+".git")
		if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
		git(t, dir, "add", ".")
		git(t, dir, "commit", "--amend", "-q", "--no-edit")
		writeWorkflow(t, dir, workflowFile(name))
		git(t, dir, "add", ".")
		git(t, dir, "commit", "-q", "-m", "definition")
		p, created, err := a.Attach(ctx, dir)
		if err != nil || !created {
			t.Fatalf("attach: %+v %v %v", p, created, err)
		}
		locations[p.Key] = dir
		both = append(both, attached{p.Key, dir, git(t, dir, "rev-parse", "HEAD"), name})
	}
	reader := attach.WorkflowReader{Git: a, Location: func(_ context.Context, p domain.ProjectKey) (string, error) {
		return locations[p], nil
	}}
	store := fakes.NewExecutionStore()
	trust := fakes.NewTrustStore(projects)
	fakes.LinkTrustAudit(store, trust)
	owner := domain.Principal{Kind: domain.PrincipalHuman, ID: "owner", Role: domain.RoleOwner}
	var ids atomic.Int64
	c := coordinator.New(store, func() time.Time { return t0 }, coordinator.WithTrust(trust), coordinator.WithExecutionIDs(func() string { return fmt.Sprintf("exec-%d", ids.Add(1)) }))
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
	run := func(p attached, key string) (coordinator.Submitted, bool, error) {
		return api.RunWorkflow(ctx, p.key, coordinator.RunWorkflowRequest{Key: key, WorkflowID: "same-flow", Revision: p.revision}, reader)
	}
	var submitted []coordinator.Submitted
	for _, p := range both {
		if _, err := c.TrustContent(ctx, "trust-"+p.name, owner, p.key, domain.TrustProject, "", ""); err != nil {
			t.Fatal(err)
		}
		before := git(t, p.dir, "status", "--porcelain")
		w, err := api.Workflow(ctx, p.key, "same-flow", p.revision, reader)
		if err != nil || w.Name != p.name {
			t.Fatalf("wrong definition: %+v %v", w, err)
		}
		out, replay, err := run(p, "run-"+p.name)
		if err != nil || replay || out.Execution.ProjectID != string(p.key) || out.Execution.Definition.Revision != p.revision || out.Execution.RequestedBy != owner {
			t.Fatalf("run: %+v %v %v", out, replay, err)
		}
		pinned, err := api.Definition(ctx, p.key, out.Execution.ID)
		if err != nil || pinned.Name != p.name {
			t.Fatalf("pin: %+v %v", pinned, err)
		}
		if got := git(t, p.dir, "status", "--porcelain"); got != before {
			t.Fatalf("Project tree changed: %s", got)
		}
		submitted = append(submitted, out)
	}
	alpha, beta := both[0], both[1]
	state := func() string {
		t.Helper()
		var all []any
		for _, out := range submitted {
			e, steps, err := store.Get(ctx, out.Execution.ID)
			if err != nil {
				t.Fatal(err)
			}
			events, err := store.Events(ctx, out.Execution.ID)
			if err != nil {
				t.Fatal(err)
			}
			all = append(all, e, steps, events)
		}
		b, err := json.Marshal(all)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	beforeState := state()
	assertRejected := func(p attached, id, revision string, want error) {
		t.Helper()
		before := ids.Load()
		out, replay, err := api.RunWorkflow(ctx, p.key, coordinator.RunWorkflowRequest{Key: "rejected", WorkflowID: id, Revision: revision}, reader)
		if !errors.Is(err, want) || replay || out.Execution.ID != "" || ids.Load() != before {
			t.Fatalf("rejection: %+v %v %v; allocated %d", out, replay, err, ids.Load()-before)
		}
	}
	for _, revision := range []string{"", "HEAD", "--help", strings.Repeat("A", 40)} {
		assertRejected(alpha, "same-flow", revision, domain.ErrInvalid)
	}
	for _, id := range []string{"../same-flow", "/tmp/private", "same-flow.yaml"} {
		assertRejected(alpha, id, alpha.revision, domain.ErrInvalid)
	}
	assertRejected(alpha, "missing-flow", alpha.revision, domain.ErrNotFound)
	assertRejected(alpha, "same-flow", beta.revision, domain.ErrNotFound)
	// Even if a foreign commit is present in this object database, it is not this Project's history.
	git(t, alpha.dir, "fetch", "-q", "--no-tags", beta.dir, "HEAD")
	assertRejected(alpha, "same-flow", beta.revision, domain.ErrNotFound)
	assertRejected(alpha, "same-flow", git(t, alpha.dir, "rev-parse", "HEAD^{tree}"), domain.ErrNotFound)
	locations[alpha.key] = beta.dir
	assertRejected(alpha, "same-flow", beta.revision, domain.ErrNotFound)
	locations[alpha.key] = alpha.dir
	if _, _, err := run(beta, "run-Alpha"); !errors.Is(err, domain.ErrKeyReused) {
		t.Fatalf("cross-Project key: %v", err)
	}
	// Working-tree edits and a newer commit cannot alter the explicitly requested old revision.
	writeWorkflow(t, alpha.dir, "invalid: uncommitted\n")
	if _, replay, err := run(alpha, "run-Alpha"); err != nil || !replay {
		t.Fatalf("working tree affected replay: %v %v", replay, err)
	}
	git(t, alpha.dir, "add", ".")
	git(t, alpha.dir, "commit", "-q", "-m", "invalid definition")
	invalidRevision := git(t, alpha.dir, "rev-parse", "HEAD")
	assertRejected(alpha, "same-flow", invalidRevision, domain.ErrInvalid)
	if _, replay, err := run(beta, "run-Beta"); err != nil || !replay {
		t.Fatalf("bad Project stopped healthy Project: %v %v", replay, err)
	}
	writeWorkflow(t, alpha.dir, workflowFile("Alpha updated"))
	git(t, alpha.dir, "add", ".")
	git(t, alpha.dir, "commit", "-q", "-m", "updated definition")
	updated := alpha
	updated.revision = git(t, alpha.dir, "rev-parse", "HEAD")
	if _, _, err := run(updated, "run-Alpha"); !errors.Is(err, domain.ErrKeyReused) {
		t.Fatalf("changed revision reused key: %v", err)
	}
	if out, replay, err := run(updated, "run-updated"); err != nil || replay || out.Execution.Definition.Hash == submitted[0].Execution.Definition.Hash {
		t.Fatalf("new definition: %+v %v %v", out, replay, err)
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			if out, replay, err := run(alpha, "run-Alpha"); err != nil || !replay || out.Execution.ID != submitted[0].Execution.ID {
				t.Errorf("concurrent old revision replay: %+v %v %v", out, replay, err)
			}
		})
	}
	wg.Wait()
	if state() != beforeState {
		t.Fatal("rejected operations/replays changed existing Executions or Events")
	}
	before := ids.Load()
	revoked.Store(true)
	if _, _, err := api.RunWorkflow(ctx, alpha.key, coordinator.RunWorkflowRequest{Key: "run-Alpha", WorkflowID: "same-flow", Revision: alpha.revision}, nil); err != domain.ErrNotPermitted {
		t.Fatalf("revoked identity reached source/replay: %v", err)
	}
	if ids.Load() != before {
		t.Fatal("revoked replay mutated state")
	}
}

func TestWorkflowReaderRefusesUnsafeContent(t *testing.T) {
	needGit(t)
	ctx := context.Background()
	a, _ := newAttacher()
	dir := repo(t, "https://example.com/unsafe.git")
	p, _, err := a.Attach(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	reader := attach.WorkflowReader{Git: a, Location: func(context.Context, domain.ProjectKey) (string, error) { return dir, nil }}
	for _, tc := range []struct{ name, content string }{
		{"different id", strings.Replace(workflowFile("Good"), "id: same-flow", "id: other-flow", 1)},
		{"future version", strings.Replace(workflowFile("Good"), "schemaVersion: 1", "schemaVersion: 2", 1)},
		{"unknown field", workflowFile("Good") + "password: PUBLIC_TEST_SENTINEL\n"},
		{"too large", strings.Repeat("# padding\n", 120000) + workflowFile("Good")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			writeWorkflow(t, dir, tc.content)
			git(t, dir, "add", ".")
			git(t, dir, "commit", "-q", "-m", tc.name)
			w, err := reader.Workflow(ctx, p.Key, "same-flow", git(t, dir, "rev-parse", "HEAD"))
			if err != domain.ErrInvalid || w.ID != "" {
				t.Fatalf("unsafe content returned: %+v %v", w, err)
			}
		})
	}
	t.Run("symlink", func(t *testing.T) {
		path := filepath.Join(dir, ".amaru", "workflows", "same-flow.yaml")
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("../../a.txt", path); err != nil {
			t.Skipf("symlink unavailable: %v", err)
		}
		git(t, dir, "add", ".")
		git(t, dir, "commit", "-q", "-m", "symlink")
		if _, err := reader.Workflow(ctx, p.Key, "same-flow", git(t, dir, "rev-parse", "HEAD")); err != domain.ErrNotFound {
			t.Fatalf("symlink: %v", err)
		}
	})
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := reader.Workflow(cancelled, p.Key, "same-flow", strings.Repeat("a", 40)); err != context.Canceled {
		t.Fatalf("cancelled lookup: %v", err)
	}
}
