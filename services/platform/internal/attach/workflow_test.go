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
		both = append(both, attached{p.Key, dir, git(t, dir, "rev-parse", "HEAD"), name})
	}
	reader := attach.WorkflowReader{Git: a}
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
	git(t, alpha.dir, "remote", "set-url", "origin", "https://example.com/Beta.git")
	assertRejected(alpha, "same-flow", alpha.revision, domain.ErrNotFound)
	git(t, alpha.dir, "remote", "set-url", "origin", "https://example.com/Alpha.git")
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
	reader := attach.WorkflowReader{Git: a}
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

// The hook runs after the binding is captured, before any Git process starts.
type workflowCloneStore struct {
	*fakes.ProjectStore
	lookup func(context.Context, domain.ProjectKey) (domain.ProjectClone, error)
}

func (s workflowCloneStore) ActiveClone(ctx context.Context, key domain.ProjectKey) (domain.ProjectClone, error) {
	return s.lookup(ctx, key)
}

func TestWorkflowUsesOnePersistedClone(t *testing.T) {
	needGit(t)
	ctx := context.Background()
	a, projects := newAttacher()
	first := repo(t, "https://example.com/app.git")
	writeWorkflow(t, first, workflowFile("First clone"))
	git(t, first, "add", ".")
	git(t, first, "commit", "-qm", "workflow")
	revision := git(t, first, "rev-parse", "HEAD")
	p, _, err := a.Attach(ctx, first)
	if err != nil {
		t.Fatal(err)
	}
	second := filepath.Join(t.TempDir(), "second")
	git(t, filepath.Dir(second), "clone", "-q", first, second)
	git(t, second, "remote", "set-url", "origin", "https://example.com/app.git")
	// Only the first clone contains this requested commit after the switch.
	writeWorkflow(t, first, workflowFile("Captured first clone"))
	git(t, first, "add", ".")
	git(t, first, "commit", "-qm", "first only")
	firstRevision := git(t, first, "rev-parse", "HEAD")
	var calls int
	var duringLookup func()
	a.Projects = workflowCloneStore{projects, func(ctx context.Context, key domain.ProjectKey) (domain.ProjectClone, error) {
		calls++
		clone, err := projects.ActiveClone(ctx, key)
		if duringLookup != nil {
			f := duringLookup
			duringLookup = nil
			f()
		}
		return clone, err
	}}
	reader := attach.WorkflowReader{Git: a}
	var revoked bool
	access, err := coordinator.NewProjectAccess(coordinator.New(fakes.NewExecutionStore(), time.Now), projects, func(context.Context) (domain.Principal, error) {
		if revoked {
			return domain.Principal{}, domain.ErrNotPermitted
		}
		return domain.Principal{Kind: domain.PrincipalHuman, ID: "owner", Role: domain.RoleOwner}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	load := func(rev string) (domain.Workflow, error) {
		return access.Workflow(ctx, p.Key, "same-flow", rev, reader)
	}
	duringLookup = func() {
		if _, _, err := a.Attach(ctx, second); err != nil {
			t.Fatal(err)
		}
	}
	if w, err := load(firstRevision); err != nil || w.Name != "Captured first clone" || calls != 1 {
		t.Fatalf("captured root: %+v %v calls=%d", w, err, calls)
	}
	if w, err := load(firstRevision); err != domain.ErrNotFound || w.ID != "" || calls != 2 {
		t.Fatalf("fell back to old root: %+v %v calls=%d", w, err, calls)
	}
	if w, err := load(revision); err != nil || w.Name != "First clone" {
		t.Fatalf("second clone: %+v %v", w, err)
	}
	// Moving the selected clone must fail even while the old clone is still usable.
	moved := second + "-moved"
	if err := os.Rename(second, moved); err != nil {
		t.Fatal(err)
	}
	if w, err := load(revision); err != domain.ErrNotFound || w.ID != "" {
		t.Fatalf("moved root fallback: %+v %v", w, err)
	}
	if err := os.Rename(moved, second); err != nil {
		t.Fatal(err)
	}
	git(t, second, "remote", "set-url", "origin", "https://example.com/foreign.git")
	if w, err := load(revision); err != domain.ErrNotFound || w.ID != "" {
		t.Fatalf("replaced identity: %+v %v", w, err)
	}
	git(t, second, "remote", "set-url", "origin", "https://example.com/app.git")
	duringLookup = func() { revoked = true }
	if w, err := load(revision); err != domain.ErrNotPermitted || w.ID != "" {
		t.Fatalf("revoked read leaked content: %+v %v", w, err)
	}
	before := calls
	if _, err := load(revision); err != domain.ErrNotPermitted || calls != before {
		t.Fatalf("revoked lookup: %v calls=%d", err, calls)
	}
	revoked = false
	missing := domain.ProjectKey("ssh://example.com/missing#" + strings.Repeat("a", 40))
	if _, err := access.Workflow(ctx, missing, "same-flow", revision, reader); err != domain.ErrNotFound || calls != before {
		t.Fatalf("unattached lookup: %v calls=%d", err, calls)
	}
	// A missing binding is a runtime error, with no path inference or Git IO.
	a.Projects = workflowCloneStore{projects, func(context.Context, domain.ProjectKey) (domain.ProjectClone, error) {
		return domain.ProjectClone{}, domain.ErrNotFound
	}}
	if w, err := access.Workflow(ctx, p.Key, "same-flow", revision, attach.WorkflowReader{Git: a}); err != domain.ErrNotFound || w.ID != "" {
		t.Fatalf("missing binding: %+v %v", w, err)
	}
}
