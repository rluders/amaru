//go:build pg

package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/rluders/amaru/services/platform/internal/adapter/localauth"
	"github.com/rluders/amaru/services/platform/internal/attach"
	"github.com/rluders/amaru/services/platform/internal/coordinator"
	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/store/migrations"
	"github.com/rluders/amaru/services/platform/internal/store/postgres"
	"github.com/rluders/amaru/services/platform/internal/worker/osproc"
)

var projectDatabaseSequence atomic.Int64

// Each run owns a fresh database; credentials and backend errors are never printed.
func projectDatabase(t *testing.T) func() *sql.DB {
	t.Helper()
	dsn := os.Getenv("AMARU_TEST_PG")
	if dsn == "" {
		t.Skip("AMARU_TEST_PG is not set")
	}
	config, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatal("invalid PostgreSQL test configuration")
	}
	admin := sql.OpenDB(stdlib.GetConnector(*config))
	t.Cleanup(func() { admin.Close() })
	name := fmt.Sprintf("amaru_http_%d_%d", os.Getpid(), projectDatabaseSequence.Add(1))
	identifier := pgx.Identifier{name}.Sanitize()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := admin.ExecContext(ctx, "CREATE DATABASE "+identifier); err != nil {
		t.Fatal("cannot create disposable database")
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := admin.ExecContext(ctx, "DROP DATABASE "+identifier+" WITH (FORCE)"); err != nil {
			t.Error("cannot drop disposable database")
		}
	})
	config.Database = name
	return func() *sql.DB {
		db := sql.OpenDB(stdlib.GetConnector(*config))
		t.Cleanup(func() { db.Close() })
		return db
	}
}

func TestProjectsPostgresTwoClientsAndReopen(t *testing.T) {
	open := projectDatabase(t)
	db := open()
	dir, err := migrations.OpenDir(migrations.Embedded())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, err := migrations.NewRunner(db, dir, 10*time.Second).Apply(ctx); err != nil {
		t.Fatal("cannot migrate disposable database")
	}
	store := postgres.NewProjectStore(db)
	testProjectsTwoClients(t, store, coordinator.New(postgres.NewExecutionStore(db), time.Now))

	// Record authoritative state after the shared test's failed/foreign/revoked writes.
	// Close the entire pool and recreate every application/HTTP owner over the same DB.
	before, err := store.List(ctx)
	if err != nil || len(before) != 2 {
		t.Fatal("missing attached Projects")
	}
	bindings := make(map[domain.ProjectKey]domain.ProjectClone)
	preferences := make(map[domain.ProjectKey]domain.ProjectSettings)
	for _, p := range before {
		bindings[p.Key], err = store.ActiveClone(ctx, p.Key)
		if err != nil {
			t.Fatal("missing clone binding")
		}
		preferences[p.Key], err = store.Settings(ctx, p.Key)
		if err != nil {
			t.Fatal("missing settings")
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal("cannot close first pool")
	}
	reopened := open()
	fresh := postgres.NewProjectStore(reopened)
	after, err := fresh.List(ctx)
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("Project metadata changed across reopen")
	}
	c := coordinator.New(postgres.NewExecutionStore(reopened), time.Now)
	inspector := attach.Attacher{Runner: osproc.Runner{}, Resolver: osproc.Resolver{}}
	auth, err := localauth.New(&net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 7777})
	if err != nil {
		t.Fatal(err)
	}
	defer auth.Close()
	projects, err := ProjectsHandler(auth, c, fresh, inspector)
	if err != nil {
		t.Fatal(err)
	}
	access, err := coordinator.NewProjectAccess(c, fresh, auth.Principal)
	if err != nil {
		t.Fatal(err)
	}
	settings, err := SettingsHandler(auth, access, fresh)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range after {
		path := "/v1/projects/" + url.PathEscape(string(p.Key))
		var got projectJSON
		w := call(projects, auth, http.MethodGet, path, "", nil)
		check(t, w, 200, "")
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil || got.ID != p.Key || got.Name != p.Name || got.Root != bindings[p.Key].Root {
			t.Fatal("reopened HTTP detail lost selected Project")
		}
		var preference domain.ProjectSettings
		w = call(settings, auth, http.MethodGet, path+"/settings", "", nil)
		check(t, w, 200, "")
		if err := json.Unmarshal(w.Body.Bytes(), &preference); err != nil || preference != preferences[p.Key] {
			t.Fatal("reopened HTTP settings changed")
		}
	}

	// A real database failure after clone insertion must roll back both registration
	// and selection. HTTP exposes only the fixed error, never database diagnostics.
	if _, err := reopened.ExecContext(ctx, `CREATE FUNCTION refuse_http_clone() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'private database diagnostic'; END $$;
 CREATE TRIGGER refuse_http_clone AFTER INSERT OR UPDATE ON project_clones FOR EACH ROW EXECUTE FUNCTION refuse_http_clone()`); err != nil {
		t.Fatal("cannot install rollback fixture")
	}
	for _, p := range after {
		clone := filepath.Join(t.TempDir(), "replacement")
		projectGit(t, "", "clone", "-q", bindings[p.Key].Root, clone)
		remote := projectGit(t, bindings[p.Key].Root, "remote", "get-url", "origin")
		projectGit(t, clone, "remote", "set-url", "origin", remote)
		body, _ := json.Marshal(map[string]string{"root": clone})
		w := call(projects, auth, http.MethodPost, "/v1/projects", string(body), nil)
		check(t, w, 500, "internal")
		if w.Body.String() != "{\"error\":{\"code\":\"internal\",\"message\":\"Request rejected.\"}}\n" {
			t.Fatal("database diagnostic or unexpected error envelope")
		}
		binding, err := fresh.ActiveClone(ctx, p.Key)
		if err != nil || binding != bindings[p.Key] {
			t.Fatal("failed reattach changed binding")
		}
		preference, err := fresh.Settings(ctx, p.Key)
		if err != nil || preference != preferences[p.Key] {
			t.Fatal("failed reattach changed settings")
		}
	}
	// The same failure on first attachment must not leave an orphan Project row.
	root := t.TempDir()
	projectGit(t, root, "init", "-q", "-b", "main")
	projectGit(t, root, "commit", "-q", "--allow-empty", "-m", "new Project")
	projectGit(t, root, "remote", "add", "origin", "https://github.com/acme/new-project.git")
	body, _ := json.Marshal(map[string]string{"root": root})
	w := call(projects, auth, http.MethodPost, "/v1/projects", string(body), nil)
	check(t, w, 500, "internal")
	if w.Body.String() != "{\"error\":{\"code\":\"internal\",\"message\":\"Request rejected.\"}}\n" {
		t.Fatal("first attach leaked database diagnostic")
	}
	final, err := fresh.List(ctx)
	if err != nil || !reflect.DeepEqual(before, final) {
		t.Fatal("failed reattach changed Project metadata")
	}
}

// Opt-in SDK composition: pg tag plus AMARU_TEST_PG, Node and installed SDK dependencies.
func TestProjectsPostgresSDKRecreation(t *testing.T) {
	open := projectDatabase(t)
	roots := []string{t.TempDir(), t.TempDir()}
	for i, root := range roots {
		projectGit(t, root, "init", "-q", "-b", "main")
		projectGit(t, root, "commit", "-q", "--allow-empty", "-m", "initial")
		projectGit(t, root, "remote", "add", "origin", fmt.Sprintf("https://github.com/acme/sdk-%d.git", i))
	}
	snapshot := filepath.Join(t.TempDir(), "projects.json")
	for _, phase := range []string{"write", "read"} {
		t.Run(phase, func(t *testing.T) {
			db := open()
			defer db.Close()
			dir, err := migrations.OpenDir(migrations.Embedded())
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if _, err := migrations.NewRunner(db, dir, 10*time.Second).Apply(ctx); err != nil {
				t.Fatal("cannot migrate SDK test database")
			}
			srv := httptest.NewUnstartedServer(nil)
			auth, err := localauth.New(srv.Listener.Addr())
			if err != nil {
				t.Fatal(err)
			}
			defer auth.Close()
			store := postgres.NewProjectStore(db)
			c := coordinator.New(postgres.NewExecutionStore(db), time.Now)
			projects, err := ProjectsHandler(auth, c, store, attach.Attacher{Runner: osproc.Runner{}, Resolver: osproc.Resolver{}})
			if err != nil {
				t.Fatal(err)
			}
			access, err := coordinator.NewProjectAccess(c, store, auth.Principal)
			if err != nil {
				t.Fatal(err)
			}
			settings, err := SettingsHandler(auth, access, store)
			if err != nil {
				t.Fatal(err)
			}
			mux := http.NewServeMux()
			mux.Handle("/v1/projects/{projectId}/settings", settings)
			mux.Handle("/", projects)
			// Compatibility metadata only; this fixture is not product startup/readiness.
			mux.HandleFunc("/v1/health", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]any{"status": "ok", "platformVersion": "sdk-pg-fixture", "protocolVersion": 1, "startedAt": time.Now().UTC(), "uptimeSec": 0})
			})
			srv.Config.Handler = mux
			srv.Start()
			defer srv.Close()
			cmd := exec.CommandContext(ctx, "node", "test/postgres.fixture.ts")
			cmd.Dir = "../../../../../packages/sdk"
			cmd.Env = append(os.Environ(), "AMARU_SDK_ENDPOINT="+srv.URL+"/v1", "AMARU_SDK_TOKEN="+auth.AdministratorSecret(), "AMARU_SDK_PHASE="+phase, "AMARU_SDK_SNAPSHOT="+snapshot, "AMARU_SDK_ROOT_A="+roots[0], "AMARU_SDK_ROOT_B="+roots[1])
			// Assertion diagnostics could contain requests/credentials; never echo them.
			if err := cmd.Run(); err != nil {
				t.Fatal("SDK PostgreSQL acceptance failed in phase " + phase)
			}
		})
		if t.Failed() {
			return
		}
	}
}
