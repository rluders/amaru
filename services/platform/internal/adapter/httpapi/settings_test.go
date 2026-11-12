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
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rluders/amaru/services/platform/internal/adapter/localauth"
	"github.com/rluders/amaru/services/platform/internal/coordinator"
	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports/fakes"
	"go.yaml.in/yaml/v3"
)

const projectA = domain.ProjectKey("ssh://github.com/acme/a#0123456789abcdef0123456789abcdef01234567")
const projectB = domain.ProjectKey("id:01ARZ3NDEKTSV4RRFFQ69G5FAV")

func path(key domain.ProjectKey) string {
	return "/v1/projects/" + url.PathEscape(string(key)) + "/settings"
}

type settingsStore struct {
	*fakes.ProjectStore
	reads, writes int
	failure       error
	writeFailure  error
}

func (s *settingsStore) Settings(ctx context.Context, key domain.ProjectKey) (domain.ProjectSettings, error) {
	s.reads++
	if s.failure != nil {
		return domain.ProjectSettings{}, s.failure
	}
	return s.ProjectStore.Settings(ctx, key)
}
func (s *settingsStore) UpdateSettings(ctx context.Context, key domain.ProjectKey, patch domain.UpdateSettings) (domain.ProjectSettings, error) {
	s.writes++
	if s.writeFailure != nil {
		return domain.ProjectSettings{}, s.writeFailure
	}
	return s.ProjectStore.UpdateSettings(ctx, key, patch)
}

func setup(t *testing.T, addr net.Addr) (*localauth.Auth, http.Handler, *settingsStore) {
	t.Helper()
	auth, err := localauth.New(addr)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(auth.Close)
	projects := fakes.NewProjectStore()
	for _, key := range []domain.ProjectKey{projectA, projectB} {
		if err := projects.Create(context.Background(), domain.Project{Key: key, Name: "same", CreatedAt: time.Now()}); err != nil {
			t.Fatal(err)
		}
	}
	access, err := coordinator.NewProjectAccess(coordinator.New(fakes.NewExecutionStore(), time.Now), projects, auth.Principal)
	if err != nil {
		t.Fatal(err)
	}
	store := &settingsStore{ProjectStore: projects}
	h, err := SettingsHandler(auth, access, store)
	if err != nil {
		t.Fatal(err)
	}
	return auth, h, store
}
func localSetup(t *testing.T) (*localauth.Auth, http.Handler, *settingsStore) {
	return setup(t, &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 7777})
}
func call(h http.Handler, auth *localauth.Auth, method, target, body string, headers map[string]string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "http://localhost:7777"+target, strings.NewReader(body))
	if auth != nil {
		r.Header.Set("Authorization", "Bearer "+auth.AdministratorSecret())
	}
	r.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}
func check(t *testing.T, w *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if w.Code != status {
		t.Fatalf("status %d want %d: %s", w.Code, status, w.Body)
	}
	if w.Header().Get("Content-Type") != "application/json" || w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("X-Amaru-Protocol") != "1" {
		t.Fatal("missing response headers")
	}
	if code != "" {
		var result struct {
			Error struct{ Code, Message string }
		}
		if json.Unmarshal(w.Body.Bytes(), &result) != nil || result.Error.Code != code || result.Error.Message != "Request rejected." {
			t.Fatalf("unsafe error: %s", w.Body)
		}
	}
}
func launch(t *testing.T, auth *localauth.Auth, h http.Handler) (string, string) {
	t.Helper()
	w := call(h, auth, "POST", "/v1/auth/launch-codes", "", nil)
	var code struct{ Code string }
	if w.Code != 201 || json.Unmarshal(w.Body.Bytes(), &code) != nil {
		t.Fatal("launch code failed")
	}
	w = call(h, nil, "GET", "/auth/launch?code="+code.Code, "", nil)
	if w.Code != 303 || len(w.Result().Cookies()) != 1 {
		t.Fatal("exchange failed")
	}
	cookie := w.Result().Cookies()[0].String()
	w = call(h, nil, "GET", "/v1/auth/session", "", map[string]string{"Cookie": cookie})
	var session struct {
		CSRF string `json:"csrfToken"`
	}
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &session) != nil || session.CSRF == "" {
		t.Fatal("session failed")
	}
	return cookie, session.CSRF
}

func TestSettingsTwoClients(t *testing.T) {
	auth, h, store := localSetup(t)
	cookie, csrf := launch(t, auth, h)
	browser := map[string]string{"Cookie": cookie, "Origin": "http://localhost:7777", "X-Amaru-CSRF": csrf}
	for _, key := range []domain.ProjectKey{projectA, projectB} {
		w := call(h, auth, "GET", path(key), "", nil)
		check(t, w, 200, "")
		if strings.TrimSpace(w.Body.String()) != `{"importClaudeSkills":false}` {
			t.Fatal("default not false")
		}
	}
	for _, value := range []string{"true", "true", "false", "false"} {
		w := call(h, nil, "PATCH", path(projectA), `{"importClaudeSkills":`+value+`}`, browser)
		check(t, w, 200, "")
		if strings.TrimSpace(w.Body.String()) != `{"importClaudeSkills":`+value+`}` {
			t.Fatal("patch result")
		}
		w = call(h, auth, "GET", path(projectA), "", nil)
		check(t, w, 200, "")
		if strings.TrimSpace(w.Body.String()) != `{"importClaudeSkills":`+value+`}` {
			t.Fatal("native readback")
		}
		w = call(h, auth, "GET", path(projectB), "", nil)
		check(t, w, 200, "")
		if strings.TrimSpace(w.Body.String()) != `{"importClaudeSkills":false}` {
			t.Fatal("cross-Project write")
		}
	}
	if store.writes != 4 {
		t.Fatal("unexpected write count")
	}
	w := call(h, nil, "DELETE", "/v1/auth/session", "", browser)
	if w.Code != 204 {
		t.Fatal("logout failed")
	}
	before := store.reads
	check(t, call(h, nil, "PATCH", path(projectA), `{"importClaudeSkills":true}`, browser), 401, "unauthorized")
	if store.writes != 4 || store.reads != before {
		t.Fatal("logged-out dispatch")
	}
}

func TestSettingsRejectedRequests(t *testing.T) {
	for _, tc := range []struct {
		name, target, body, header, value, code string
		status                                  int
		native                                  bool
	}{
		{"anonymous", path(projectA), `{}`, "", "", "unauthorized", 401, false},
		{"forged host", path(projectA), `{}`, "Host", "evil.invalid", "forbidden", 403, true},
		{"foreign origin", path(projectA), `{}`, "Origin", "https://evil.invalid", "forbidden_origin", 403, true},
		{"query token", path(projectA) + "?token=bad", `{}`, "", "", "invalid_request", 400, true},
		{"foreign project", path("ssh://github.com/acme/other#0123456789abcdef0123456789abcdef01234567"), `{}`, "", "", "not_found", 404, true},
		{"invalid project", path("bad"), `{}`, "", "", "not_found", 404, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			auth, h, s := localSetup(t)
			r := httptest.NewRequest("PATCH", "http://localhost:7777"+tc.target, strings.NewReader(tc.body))
			if tc.native {
				r.Header.Set("Authorization", "Bearer "+auth.AdministratorSecret())
			}
			r.Header.Set("Content-Type", "application/json")
			if tc.header == "Host" {
				r.Host = tc.value
			} else if tc.header != "" {
				r.Header.Set(tc.header, tc.value)
			}
			read := false
			r.Body = readerFunc(func(p []byte) (int, error) { read = true; return 0, io.EOF })
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			check(t, w, tc.status, tc.code)
			if read || s.reads != 0 || s.writes != 0 {
				t.Fatal("rejected request accessed body or settings")
			}
		})
	}
}

type readerFunc func([]byte) (int, error)

func (f readerFunc) Read(p []byte) (int, error) { return f(p) }
func (f readerFunc) Close() error               { return nil }

func TestSettingsInvalidBodies(t *testing.T) {
	auth, h, s := localSetup(t)
	for _, body := range []string{"", `{}`, `null`, `[]`, `{"importClaudeSkills":null}`, `{"importClaudeSkills":"true"}`, `{"importClaudeSkills":1}`, `{"ImportClaudeSkills":true}`, `{"importClaudeSkills":true,"root":"/other"}`, `{"importClaudeSkills":true} {}`, `{"importClaudeSkills":true} trailing`, strings.Repeat(" ", 4097) + `{"importClaudeSkills":true}`} {
		check(t, call(h, auth, "PATCH", path(projectA), body, nil), 400, "invalid_request")
	}
	check(t, call(h, auth, "PATCH", path(projectA), `{"importClaudeSkills":true}`, map[string]string{"Content-Type": "text/plain"}), 400, "invalid_request")
	if s.writes != 0 {
		t.Fatal("invalid patch wrote settings")
	}
}

func TestSettingsBrowserGuards(t *testing.T) {
	auth, h, s := localSetup(t)
	cookie, csrf := launch(t, auth, h)
	for _, headers := range []map[string]string{
		{"Cookie": cookie},
		{"Cookie": cookie, "Origin": "http://localhost:7777"},
		{"Cookie": cookie, "Origin": "http://localhost:7777", "X-Amaru-CSRF": "wrong"},
		{"Cookie": cookie, "Origin": "https://evil.invalid", "X-Amaru-CSRF": csrf},
	} {
		w := call(h, nil, "PATCH", path(projectA), `{"importClaudeSkills":true}`, headers)
		if w.Code != 403 || s.reads != 0 || s.writes != 0 {
			t.Fatal("browser guard dispatched")
		}
	}
}

func TestSettingsRevocationDuringBodyRead(t *testing.T) {
	auth, h, s := localSetup(t)
	r := httptest.NewRequest("PATCH", "http://localhost:7777"+path(projectA), nil)
	r.Header.Set("Authorization", "Bearer "+auth.AdministratorSecret())
	r.Header.Set("Content-Type", "application/json")
	data := strings.NewReader(`{"importClaudeSkills":true}`)
	r.Body = readerFunc(func(p []byte) (int, error) { auth.Close(); return data.Read(p) })
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	check(t, w, 403, "forbidden")
	if s.reads != 1 || s.writes != 0 {
		t.Fatal("revoked request wrote")
	}
}

func TestSettingsStoreFailureIsRedacted(t *testing.T) {
	auth, h, s := localSetup(t)
	s.failure = errors.New("sensitive database diagnostics")
	check(t, call(h, auth, "GET", path(projectA), "", nil), 500, "internal")
	if s.writes != 0 {
		t.Fatal("failure wrote")
	}
}

func TestSettingsOverLoopback(t *testing.T) {
	srv := httptest.NewUnstartedServer(nil)
	auth, h, _ := setup(t, srv.Listener.Addr())
	srv.Config.Handler = h
	srv.Start()
	defer srv.Close()
	for _, client := range []*http.Client{srv.Client(), {Timeout: time.Second * 5}} {
		r, err := http.NewRequest("PATCH", srv.URL+path(projectB), strings.NewReader(`{"importClaudeSkills":true}`))
		if err != nil {
			t.Fatal(err)
		}
		r.Header.Set("Authorization", "Bearer "+auth.AdministratorSecret())
		r.Header.Set("Content-Type", "application/json")
		res, err := client.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(res.Body)
		res.Body.Close()
		if err != nil || res.StatusCode != 200 || strings.TrimSpace(string(data)) != `{"importClaudeSkills":true}` {
			t.Fatal("loopback patch failed")
		}
	}
}

func TestSettingsRoutesMatchOpenAPIResource(t *testing.T) {
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
	resource := doc.Paths["/projects/{projectId}/settings"]
	methods := map[string]bool{}
	for method := range resource {
		if method != "parameters" {
			methods[strings.ToUpper(method)] = true
		}
	}
	for _, pattern := range settingsPatterns {
		method, target, _ := strings.Cut(pattern, " ")
		if !methods[method] || target != "/v1/projects/{projectId}/settings" {
			t.Fatalf("undocumented route: %s", pattern)
		}
		delete(methods, method)
	}
	if len(methods) != 0 {
		t.Fatal("unserved settings operation")
	}
}

func TestSettingsSharedPatchFixtures(t *testing.T) {
	auth, h, _ := localSetup(t)
	files, err := filepath.Glob("../../../../../packages/contracts/fixtures/api/v1/UpdateSettings/*.json")
	if err != nil || len(files) != 6 {
		t.Fatalf("settings fixtures: %d %v", len(files), err)
	}
	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) {
			data, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			status, code := 200, ""
			if strings.HasPrefix(filepath.Base(file), "invalid-") {
				status, code = 400, "invalid_request"
			}
			check(t, call(h, auth, "PATCH", path(projectA), string(data), nil), status, code)
		})
	}
}

func TestSettingsBodyReadFailure(t *testing.T) {
	auth, h, s := localSetup(t)
	r := httptest.NewRequest("PATCH", "http://localhost:7777"+path(projectA), nil)
	r.Header.Set("Authorization", "Bearer "+auth.AdministratorSecret())
	r.Header.Set("Content-Type", "application/json")
	r.Body = readerFunc(func([]byte) (int, error) { return 0, errors.New("private IO failure") })
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	check(t, w, 400, "invalid_request")
	if s.writes != 0 {
		t.Fatal("read failure wrote")
	}
}

func TestSettingsFailedWriteIsRedactedAndUnchanged(t *testing.T) {
	auth, h, s := localSetup(t)
	s.writeFailure = errors.New("private write failure")
	check(t, call(h, auth, "PATCH", path(projectA), `{"importClaudeSkills":true}`, nil), 500, "internal")
	w := call(h, auth, "GET", path(projectA), "", nil)
	check(t, w, 200, "")
	if s.writes != 1 || strings.TrimSpace(w.Body.String()) != `{"importClaudeSkills":false}` {
		t.Fatal("failed write changed value")
	}
}
