package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rluders/amaru/services/platform/internal/adapter/localauth"
	"github.com/rluders/amaru/services/platform/internal/attach"
	"github.com/rluders/amaru/services/platform/internal/coordinator"
	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
	"github.com/rluders/amaru/services/platform/internal/ports/fakes"
	"github.com/rluders/amaru/services/platform/internal/worker/osproc"
	"go.yaml.in/yaml/v3"
)

type projectInspector func(context.Context, string) (ports.ProjectInspection, error)

func (f projectInspector) Inspect(ctx context.Context, root string) (ports.ProjectInspection, error) {
	return f(ctx, root)
}

type projectStore struct {
	*fakes.ProjectStore
	reads, writes int
	failure       error
	afterWrite    func()
}

func (s *projectStore) List(ctx context.Context) ([]domain.Project, error) {
	s.reads++
	if s.failure != nil {
		return nil, s.failure
	}
	return s.ProjectStore.List(ctx)
}
func (s *projectStore) Get(ctx context.Context, key domain.ProjectKey) (domain.Project, error) {
	s.reads++
	return s.ProjectStore.Get(ctx, key)
}
func (s *projectStore) RegisterClone(ctx context.Context, p domain.Project, clone domain.ProjectClone) (domain.Project, bool, error) {
	s.writes++
	if s.failure != nil {
		return domain.Project{}, false, s.failure
	}
	out, created, err := s.ProjectStore.RegisterClone(ctx, p, clone)
	if s.afterWrite != nil {
		s.afterWrite()
	}
	return out, created, err
}

func projectSetup(t *testing.T, inspect projectInspector) (*localauth.Auth, http.Handler, *projectStore) {
	t.Helper()
	auth, err := localauth.New(&net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 7777})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(auth.Close)
	s := &projectStore{ProjectStore: fakes.NewProjectStore()}
	h, err := ProjectsHandler(auth, coordinator.New(fakes.NewExecutionStore(), time.Now), s, inspect)
	if err != nil {
		t.Fatal(err)
	}
	return auth, h, s
}

func TestProjectsStrictAttachBody(t *testing.T) {
	inspections := 0
	auth, h, s := projectSetup(t, func(context.Context, string) (ports.ProjectInspection, error) {
		inspections++
		return ports.ProjectInspection{Key: projectA, Name: "A", Root: "/a"}, nil
	})
	for _, body := range []string{"", `{}`, `null`, `[]`, `{"root":null}`, `{"root":1}`, `{"root":""}`, `{"Root":"/a"}`, `{"root":"/a","root":"/b"}`, `{"root":"/a","extra":true}`, `{"root":"/a"} {}`, `{"root":"/a"} trailing`, strings.Repeat(" ", 4097) + `{"root":"/a"}`} {
		check(t, call(h, auth, "POST", "/v1/projects", body, nil), 400, "invalid_request")
	}
	check(t, call(h, auth, "POST", "/v1/projects", `{"root":"/a"}`, map[string]string{"Content-Type": "text/plain"}), 400, "invalid_request")
	if s.writes != 0 || inspections != 0 {
		t.Fatal("invalid body dispatched")
	}
	check(t, call(h, auth, "POST", "/v1/projects", `{"root":"/a"}`, nil), 200, "")
	if s.writes != 1 || inspections != 1 {
		t.Fatal("valid attach not dispatched once")
	}
}

func TestProjectsGuardsBeforeIO(t *testing.T) {
	auth, h, s := projectSetup(t, func(context.Context, string) (ports.ProjectInspection, error) {
		t.Fatal("guarded request inspected filesystem")
		return ports.ProjectInspection{}, nil
	})
	cookie, csrf := launch(t, auth, h)
	for _, tc := range []struct {
		name, method, target, header, value, code string
		native                                    bool
		status                                    int
	}{
		{"anonymous", "POST", "/v1/projects", "", "", "unauthorized", false, 401},
		{"anonymous list", "GET", "/v1/projects", "", "", "unauthorized", false, 401},
		{"anonymous detail", "GET", "/v1/projects/bad", "", "", "unauthorized", false, 401},
		{"host", "POST", "/v1/projects", "Host", "evil.invalid", "forbidden", true, 403},
		{"origin", "POST", "/v1/projects", "Origin", "https://evil.invalid", "forbidden_origin", true, 403},
		{"query", "POST", "/v1/projects?token=bad", "", "", "invalid_request", true, 400},
		{"browser csrf", "POST", "/v1/projects", "Cookie", cookie, "forbidden_origin", false, 403},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(tc.method, "http://localhost:7777"+tc.target, nil)
			if tc.native {
				r.Header.Set("Authorization", "Bearer "+auth.AdministratorSecret())
			}
			if tc.header == "Host" {
				r.Host = tc.value
			} else if tc.header != "" {
				r.Header.Set(tc.header, tc.value)
			}
			r.Body = readerFunc(func([]byte) (int, error) { t.Fatal("guard read body"); return 0, io.EOF })
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			check(t, w, tc.status, tc.code)
		})
	}
	for _, headers := range []map[string]string{
		{"Cookie": cookie, "Origin": "http://localhost:7777"},
		{"Cookie": cookie, "Origin": "http://localhost:7777", "X-Amaru-CSRF": "wrong"},
		{"Cookie": cookie, "Origin": "https://evil.invalid", "X-Amaru-CSRF": csrf},
	} {
		w := call(h, nil, "POST", "/v1/projects", `{"root":"/a"}`, headers)
		if w.Code != 403 {
			t.Fatalf("browser guard: %d", w.Code)
		}
	}
	if s.reads != 0 || s.writes != 0 {
		t.Fatal("guard accessed store")
	}
}

func TestProjectsRevocation(t *testing.T) {
	for _, stage := range []string{"body", "inspection", "after commit"} {
		t.Run(stage, func(t *testing.T) {
			var auth *localauth.Auth
			var inspected bool
			var h http.Handler
			var s *projectStore
			auth, h, s = projectSetup(t, func(ctx context.Context, root string) (ports.ProjectInspection, error) {
				inspected = true
				if stage == "inspection" {
					auth.Close()
				}
				return ports.ProjectInspection{Key: projectA, Name: "A", Root: root}, nil
			})
			if stage == "after commit" {
				s.afterWrite = auth.Close
			}
			r := httptest.NewRequest("POST", "http://localhost:7777/v1/projects", nil)
			r.Header.Set("Authorization", "Bearer "+auth.AdministratorSecret())
			r.Header.Set("Content-Type", "application/json")
			body := strings.NewReader(`{"root":"/a"}`)
			r.Body = readerFunc(func(p []byte) (int, error) {
				if stage == "body" {
					auth.Close()
				}
				return body.Read(p)
			})
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			check(t, w, 403, "forbidden")
			projects, _ := s.ProjectStore.List(context.Background())
			if stage == "after commit" {
				if len(projects) != 1 || s.writes != 1 {
					t.Fatal("committed write was rolled back")
				}
			} else if len(projects) != 0 || s.writes != 0 {
				t.Fatal("revoked request wrote")
			}
			if stage == "body" && inspected {
				t.Fatal("revoked body inspected")
			}
		})
	}
}

func TestProjectsRedactedErrors(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
		code   string
	}{
		{domain.ErrInvalid, 400, "invalid_request"}, {domain.ErrConflict, 409, "conflict"},
		{errors.New("private process path /secret/output"), 500, "internal"},
	} {
		auth, h, s := projectSetup(t, func(context.Context, string) (ports.ProjectInspection, error) {
			return ports.ProjectInspection{}, tc.err
		})
		check(t, call(h, auth, "POST", "/v1/projects", `{"root":"/private/root"}`, nil), tc.status, tc.code)
		if s.writes != 0 {
			t.Fatal("failed inspection wrote")
		}
	}
	auth, h, s := projectSetup(t, func(context.Context, string) (ports.ProjectInspection, error) {
		return ports.ProjectInspection{Key: projectA, Name: "A", Root: "/private/root"}, nil
	})
	s.failure = errors.New("private database path")
	check(t, call(h, auth, "POST", "/v1/projects", `{"root":"/private/root"}`, nil), 500, "internal")
	check(t, call(h, auth, "GET", "/v1/projects", "", nil), 500, "internal")
	check(t, call(h, auth, "GET", "/v1/projects/bad", "", nil), 404, "not_found")
	check(t, call(h, auth, "GET", "/v1/projects/"+url.PathEscape(string(projectB)), "", nil), 404, "not_found")
	if all, _ := s.ProjectStore.List(context.Background()); len(all) != 0 {
		t.Fatal("failed registration stored Project")
	}
	// An attached Project without a binding is a conflict, never a guessed clone.
	s.failure = nil
	if err := s.Create(context.Background(), domain.Project{Key: projectA, Name: "A", CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	check(t, call(h, auth, "GET", "/v1/projects/"+url.PathEscape(string(projectA)), "", nil), 409, "conflict")
	check(t, call(h, auth, "GET", "/v1/projects", "", nil), 409, "conflict")
}

func TestProjectsSharedBodyFixturesAndRoutes(t *testing.T) {
	files, err := filepath.Glob("../../../../../packages/contracts/fixtures/api/v1/AttachProject/*.json")
	if err != nil || len(files) != 3 {
		t.Fatal(files, err)
	}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		_, err = parseAttach(data)
		if (err != nil) != strings.HasPrefix(filepath.Base(file), "invalid-") {
			t.Fatalf("fixture %s: %v", file, err)
		}
	}
	data, err := os.ReadFile("../../../../../packages/contracts/openapi/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Paths map[string]map[string]any `yaml:"paths"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	methods := map[string]bool{}
	for _, resource := range []string{"/projects", "/projects/{projectId}"} {
		for method := range doc.Paths[resource] {
			if method != "parameters" {
				methods[strings.ToUpper(method)+" /v1"+resource] = true
			}
		}
	}
	for _, pattern := range projectPatterns {
		if !methods[pattern] {
			t.Fatal("undocumented route", pattern)
		}
		delete(methods, pattern)
	}
	if len(methods) != 0 {
		t.Fatal("unserved routes", methods)
	}
}

func TestProjectsConstructor(t *testing.T) {
	auth, _, s := projectSetup(t, func(context.Context, string) (ports.ProjectInspection, error) { return ports.ProjectInspection{}, nil })
	c := coordinator.New(fakes.NewExecutionStore(), time.Now)
	inspect := projectInspector(func(context.Context, string) (ports.ProjectInspection, error) { return ports.ProjectInspection{}, nil })
	for _, args := range []struct {
		a *localauth.Auth
		c *coordinator.Coordinator
		s ports.ProjectCloneStore
		i ports.ProjectInspector
	}{
		{nil, c, s, inspect}, {auth, nil, s, inspect}, {auth, c, nil, inspect}, {auth, c, s, nil},
	} {
		if _, err := ProjectsHandler(args.a, args.c, args.s, args.i); !errors.Is(err, domain.ErrInvalid) {
			t.Fatal(err)
		}
	}
}

func projectGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1", "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

type projectHTTPStore interface {
	ports.ProjectCloneStore
	ports.SettingsStore
}

func TestProjectsTwoClientsRealGitOverLoopback(t *testing.T) {
	testProjectsTwoClients(t, fakes.NewProjectStore(), coordinator.New(fakes.NewExecutionStore(), time.Now))
}

// The same authenticated HTTP/Git cases run against fake and PostgreSQL storage.
func testProjectsTwoClients(t *testing.T, store projectHTTPStore, c *coordinator.Coordinator) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not on PATH")
	}
	ctx := context.Background()
	srv := httptest.NewUnstartedServer(nil)
	auth, err := localauth.New(srv.Listener.Addr())
	if err != nil {
		t.Fatal(err)
	}
	defer auth.Close()
	inspector := attach.Attacher{Runner: osproc.Runner{}, Resolver: osproc.Resolver{}}
	projects, err := ProjectsHandler(auth, c, store, inspector)
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
	srv.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/settings") {
			settings.ServeHTTP(w, r)
		} else {
			projects.ServeHTTP(w, r)
		}
	})
	srv.Start()
	defer srv.Close()
	native := srv.Client()
	browser := &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	var cookie *http.Cookie
	var csrf string
	request := func(client *http.Client, method, target, body string, status int) []byte {
		t.Helper()
		r, err := http.NewRequest(method, srv.URL+target, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		r.Header.Set("Content-Type", "application/json")
		if client == native {
			r.Header.Set("Authorization", "Bearer "+auth.AdministratorSecret())
		} else if cookie != nil {
			r.AddCookie(cookie)
			r.Header.Set("Origin", srv.URL)
			r.Header.Set("X-Amaru-CSRF", csrf)
		}
		res, err := client.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		data, err := io.ReadAll(res.Body)
		if err != nil || res.StatusCode != status {
			t.Fatalf("%s %s: status %d want %d: %s %v", method, target, res.StatusCode, status, data, err)
		}
		if method == "GET" && strings.HasPrefix(target, "/auth/launch?") {
			cookie = res.Cookies()[0]
		}
		return data
	}
	var launchCode struct{ Code string }
	if err := json.Unmarshal(request(native, "POST", "/v1/auth/launch-codes", "", 201), &launchCode); err != nil {
		t.Fatal(err)
	}
	request(browser, "GET", "/auth/launch?code="+launchCode.Code, "", 303)
	var session struct {
		CSRF string `json:"csrfToken"`
	}
	if err := json.Unmarshal(request(browser, "GET", "/v1/auth/session", "", 200), &session); err != nil || session.CSRF == "" {
		t.Fatal("session", err)
	}
	csrf = session.CSRF
	roots := []string{}
	attached := []projectJSON{}
	for i, client := range []*http.Client{native, browser} {
		root := t.TempDir()
		roots = append(roots, root)
		projectGit(t, root, "init", "-q", "-b", "main")
		if err := os.Mkdir(filepath.Join(root, ".amaru"), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, ".amaru", "config.yaml"), []byte("project:\n  name: Same\n"), 0600); err != nil {
			t.Fatal(err)
		}
		projectGit(t, root, "add", ".")
		projectGit(t, root, "commit", "-q", "-m", "initial")
		remote := []string{"https://github.com/acme/a.git", "https://github.com/acme/b.git"}[i]
		projectGit(t, root, "remote", "add", "origin", remote)
		body, _ := json.Marshal(map[string]string{"root": filepath.Join(root, ".amaru")})
		data := request(client, "POST", "/v1/projects", string(body), 200)
		var p projectJSON
		if err := json.Unmarshal(data, &p); err != nil {
			t.Fatal(err)
		}
		if p.Root != root || p.Name != "Same" || !p.Git.Present || !p.Config.Present || p.Config.Path != ".amaru/config.yaml" {
			t.Fatalf("inspection: %+v", p)
		}
		var fields map[string]json.RawMessage
		_ = json.Unmarshal(data, &fields)
		if len(fields) != 5 || strings.Contains(string(fields["git"]), "remote") || strings.Contains(string(fields["git"]), "branch") {
			t.Fatal("optional data exposed", string(data))
		}
		attached = append(attached, p)
	}
	if attached[0].ID == attached[1].ID {
		t.Fatal("distinct Projects share identity")
	}
	projectPath := func(p projectJSON) string { return "/v1/projects/" + url.PathEscape(string(p.ID)) }
	request(browser, "PATCH", projectPath(attached[0])+"/settings", `{"importClaudeSkills":true}`, 200)
	if string(request(native, "GET", projectPath(attached[0])+"/settings", "", 200)) != "{\"importClaudeSkills\":true}\n" {
		t.Fatal("settings not shared")
	}
	if string(request(native, "GET", projectPath(attached[1])+"/settings", "", 200)) != "{\"importClaudeSkills\":false}\n" {
		t.Fatal("cross-Project settings leak")
	}
	// A dirty malformed config does not change committed inspection metadata.
	if err := os.WriteFile(filepath.Join(roots[0], ".amaru", "config.yaml"), []byte("[invalid"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, client := range []*http.Client{native, browser} {
		var all []projectJSON
		if err := json.Unmarshal(request(client, "GET", "/v1/projects", "", 200), &all); err != nil || len(all) != 2 || all[0].ID > all[1].ID {
			t.Fatal(all, err)
		}
		for _, p := range attached {
			var got projectJSON
			_ = json.Unmarshal(request(client, "GET", projectPath(p), "", 200), &got)
			if got != p {
				t.Fatal("inconsistent inspection", got, p)
			}
		}
	}
	// An explicit second-clone selection preserves stored name, settings and creation time.
	before, err := store.Get(ctx, attached[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	clone := filepath.Join(t.TempDir(), "clone")
	projectGit(t, "", "clone", "-q", roots[0], clone)
	projectGit(t, clone, "remote", "set-url", "origin", "https://github.com/acme/a.git")
	if err := os.WriteFile(filepath.Join(clone, ".amaru", "config.yaml"), []byte("project:\n  name: Changed suggestion\n"), 0600); err != nil {
		t.Fatal(err)
	}
	projectGit(t, clone, "add", ".")
	projectGit(t, clone, "commit", "-q", "-m", "new name")
	body, _ := json.Marshal(map[string]string{"root": clone})
	for range 2 {
		request(browser, "POST", "/v1/projects", string(body), 200)
	}
	after, err := store.Get(ctx, before.Key)
	if err != nil || after != before {
		t.Fatal("metadata changed", before, after, err)
	}
	if preference, err := store.Settings(ctx, before.Key); err != nil || !preference.ImportClaudeSkills {
		t.Fatal(preference, err)
	}
	var got projectJSON
	_ = json.Unmarshal(request(native, "GET", projectPath(attached[0]), "", 200), &got)
	if got.Root != clone || got.Name != "Same" {
		t.Fatal(got)
	}
	// A different identity cannot claim an already-owned root; failed list has no partial result.
	projectGit(t, clone, "remote", "set-url", "origin", "https://github.com/acme/foreign.git")
	for _, op := range []struct{ method, target, body string }{{"POST", "/v1/projects", string(body)}, {"GET", projectPath(got), ""}, {"GET", "/v1/projects", ""}} {
		data := request(native, op.method, op.target, op.body, 409)
		if string(data) != "{\"error\":{\"code\":\"conflict\",\"message\":\"Request rejected.\"}}\n" {
			t.Fatal("unredacted or partial result", string(data))
		}
	}
	if binding, err := store.ActiveClone(ctx, before.Key); err != nil || binding.Root != clone {
		t.Fatal("failed operation changed binding", binding, err)
	}
	if err := os.RemoveAll(clone); err != nil {
		t.Fatal(err)
	}
	request(native, "GET", projectPath(got), "", 409) // Original remains available, but no fallback.
	body, _ = json.Marshal(map[string]string{"root": roots[0]})
	request(native, "POST", "/v1/projects", string(body), 200)
	request(browser, "GET", projectPath(got), "", 200)
	request(browser, "DELETE", "/v1/auth/session", "", 204)
	request(browser, "POST", "/v1/projects", string(body), 401)
	request(browser, "PATCH", projectPath(attached[0])+"/settings", `{"importClaudeSkills":false}`, 401)
	if preference, err := store.Settings(ctx, before.Key); err != nil || !preference.ImportClaudeSkills {
		t.Fatal("revoked write changed settings", err)
	}
}

func TestProjectsAbsentConfigAndEmptyList(t *testing.T) {
	auth, h, _ := projectSetup(t, func(context.Context, string) (ports.ProjectInspection, error) {
		return ports.ProjectInspection{Key: projectA, Name: "A", Root: "/a"}, nil
	})
	w := call(h, auth, "GET", "/v1/projects", "", nil)
	check(t, w, 200, "")
	if w.Body.String() != "[]\n" {
		t.Fatal("empty list not an array")
	}
	w = call(h, auth, "POST", "/v1/projects", `{"root":"/a"}`, nil)
	check(t, w, 200, "")
	var result map[string]json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if string(result["config"]) != `{"present":false}` || string(result["git"]) != `{"present":true}` {
		t.Fatal("fabricated optional metadata", w.Body)
	}
}

func TestProjectsBodyReadFailure(t *testing.T) {
	auth, h, s := projectSetup(t, func(context.Context, string) (ports.ProjectInspection, error) {
		t.Fatal("read failure inspected")
		return ports.ProjectInspection{}, nil
	})
	r := httptest.NewRequest("POST", "http://localhost:7777/v1/projects", nil)
	r.Header.Set("Authorization", "Bearer "+auth.AdministratorSecret())
	r.Header.Set("Content-Type", "application/json")
	r.Body = readerFunc(func([]byte) (int, error) { return 0, errors.New("private read failure") })
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	check(t, w, 400, "invalid_request")
	if s.writes != 0 {
		t.Fatal("read failure wrote")
	}
}
