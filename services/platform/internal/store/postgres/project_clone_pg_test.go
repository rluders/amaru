//go:build pg

package postgres

import (
	"context"
	"database/sql"
	"errors"
	"github.com/rluders/amaru/services/platform/internal/attach"
	"github.com/rluders/amaru/services/platform/internal/worker/osproc"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
	"github.com/rluders/amaru/services/platform/internal/ports/porttest"
)

func TestProjectCloneStore(t *testing.T) {
	db := migratedDB(t)
	porttest.ProjectCloneStore(t, func(*testing.T) ports.ProjectCloneStore { return NewProjectStore(db) }, func(ports.ProjectCloneStore) ports.ProjectCloneStore { return NewProjectStore(db) })
}

func TestProjectCloneRollback(t *testing.T) {
	db := migratedDB(t)
	s := NewProjectStore(db)
	ctx := context.Background()
	a := domain.Project{Key: "id:01ARZ3NDEKTSV4RRFFQ69G5FAV", Name: "original", CreatedAt: time.Now().UTC()}
	b := a
	b.Key = "id:01ARZ3NDEKTSV4RRFFQ69G5FAW"
	root1, root2 := t.TempDir(), t.TempDir()
	if _, _, err := s.RegisterClone(ctx, a, domain.ProjectClone{ProjectKey: a.Key, Root: root1}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE FUNCTION refuse_clone() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'test refusal'; END $$;
 CREATE TRIGGER refuse_clone AFTER INSERT OR UPDATE ON project_clones FOR EACH ROW EXECUTE FUNCTION refuse_clone()`); err != nil {
		t.Fatal(err)
	}
	for _, p := range []domain.Project{a, b} {
		if _, _, err := s.RegisterClone(ctx, p, domain.ProjectClone{ProjectKey: p.Key, Root: root2}); err == nil {
			t.Fatal("write succeeded")
		}
	}
	if got, err := s.ActiveClone(ctx, a.Key); err != nil || got.Root != root1 {
		t.Fatalf("binding rollback: %+v %v", got, err)
	}
	if _, err := s.Get(ctx, b.Key); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Project rollback: %v", err)
	}
}

// Hold a selection uncommitted and observe the later writer's PostgreSQL lock wait.
func TestProjectCloneCommitOrderAndCancellation(t *testing.T) {
	for _, cancelled := range []bool{false, true} {
		t.Run(map[bool]string{false: "commit order", true: "cancel blocked writer"}[cancelled], func(t *testing.T) {
			db := migratedDB(t)
			s := NewProjectStore(db)
			ctx := context.Background()
			p := domain.Project{Key: "id:01ARZ3NDEKTSV4RRFFQ69G5FAV", Name: "app", CreatedAt: time.Now().UTC()}
			first, second := t.TempDir(), t.TempDir()
			if _, _, err := s.RegisterClone(ctx, p, domain.ProjectClone{ProjectKey: p.Key, Root: first}); err != nil {
				t.Fatal(err)
			}
			tx, err := db.BeginTx(ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback()
			if _, err := tx.ExecContext(ctx, `SELECT key FROM projects WHERE key=$1 FOR UPDATE`, string(p.Key)); err != nil {
				t.Fatal(err)
			}
			if _, err := tx.ExecContext(ctx, `UPDATE project_clones SET root=$1 WHERE project_key=$2`, first, string(p.Key)); err != nil {
				t.Fatal(err)
			}
			pending, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			done := make(chan error, 1)
			go func() {
				_, _, err := s.RegisterClone(pending, p, domain.ProjectClone{ProjectKey: p.Key, Root: second})
				done <- err
			}()
			ticker := time.NewTicker(5 * time.Millisecond)
			defer ticker.Stop()
			for {
				var waiting bool
				if err := db.QueryRowContext(pending, `SELECT EXISTS (SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE 'SELECT name,created_at FROM projects%')`).Scan(&waiting); err != nil {
					t.Fatal(err)
				}
				if waiting {
					break
				}
				select {
				case err := <-done:
					t.Fatalf("writer did not wait: %v", err)
				case <-pending.Done():
					t.Fatal("writer never blocked")
				case <-ticker.C:
				}
			}
			want := second
			if cancelled {
				cancel()
				if err := <-done; !errors.Is(err, context.Canceled) {
					t.Fatalf("cancel: %v", err)
				}
				want = first
			}
			if err := tx.Commit(); err != nil {
				t.Fatal(err)
			}
			if !cancelled {
				if err := <-done; err != nil {
					t.Fatal(err)
				}
			}
			if got, err := s.ActiveClone(ctx, p.Key); err != nil || got.Root != want {
				t.Fatalf("selection: %+v %v want %s", got, err, want)
			}
		})
	}
}

func TestProjectCloneGitAndReopen(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git unavailable")
	}
	db := migratedDB(t)
	ctx := context.Background()
	root := t.TempDir()
	git := func(dir string, args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1", "GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com", "GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git: %v %s", err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git(root, "init", "-q")
	git(root, "commit", "--allow-empty", "-qm", "initial")
	git(root, "remote", "add", "origin", "https://github.com/acme/app")
	clone := filepath.Join(t.TempDir(), "clone")
	git(root, "clone", "-q", root, clone)
	git(clone, "remote", "set-url", "origin", "https://github.com/acme/app")
	workflowDir := filepath.Join(clone, ".amaru", "workflows")
	if err := os.MkdirAll(workflowDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workflowDir, "flow.yaml"), []byte("schemaVersion: 1\nid: flow\nname: Selected clone\nstatus: active\nsteps:\n  - id: test\n    name: Test\n    type: tool\n    capability: code.test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(clone, "add", ".")
	git(clone, "commit", "-qm", "clone-only workflow")
	revision := git(clone, "rev-parse", "HEAD")
	s := NewProjectStore(db)
	a := attach.Attacher{Runner: osproc.Runner{}, Resolver: osproc.Resolver{}, Projects: s, Now: func() time.Time { return time.Now().UTC() }}
	first, created, err := a.Attach(ctx, root)
	if err != nil || !created {
		t.Fatal(created, err)
	}
	enabled := true
	if _, err := s.UpdateSettings(ctx, first.Key, domain.UpdateSettings{ImportClaudeSkills: &enabled}); err != nil {
		t.Fatal(err)
	}
	second, created, err := a.Attach(ctx, clone)
	if err != nil || created || first != second {
		t.Fatalf("clone attach: %+v %v %v", second, created, err)
	}
	var database string
	if err := db.QueryRow(`SELECT current_database()`).Scan(&database); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	dsn, err := url.Parse(os.Getenv("AMARU_TEST_PG"))
	if err != nil {
		t.Fatal(err)
	}
	dsn.Path = "/" + database
	reopened, err := sql.Open("pgx", dsn.String())
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	fresh := NewProjectStore(reopened)
	canonical, err := filepath.EvalSymlinks(clone)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := fresh.ActiveClone(ctx, first.Key); err != nil || got.Root != canonical {
		t.Fatalf("reopen: %+v %v", got, err)
	}
	if got, err := fresh.Settings(ctx, first.Key); err != nil || !got.ImportClaudeSkills {
		t.Fatalf("settings: %+v %v", got, err)
	}
	if rows, err := fresh.List(ctx); err != nil || len(rows) != 1 || rows[0].Key != first.Key || rows[0].Name != first.Name || !rows[0].CreatedAt.Equal(first.CreatedAt) {
		t.Fatalf("Projects: %+v %v", rows, err)
	}
	a.Projects = fresh
	reader := attach.WorkflowReader{Git: a}
	if got, err := reader.Workflow(ctx, first.Key, "flow", revision); err != nil || got.Name != "Selected clone" {
		t.Fatalf("persisted Workflow clone after pool reopen: %+v %v", got, err)
	}

}
