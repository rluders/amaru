package localauth

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func setup(t *testing.T) (*Auth, http.Handler, *atomic.Int32) {
	t.Helper()
	a, err := New(&net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 7777})
	if err != nil {
		t.Fatal("initialize authentication")
	}
	t.Cleanup(a.Close)
	count := new(atomic.Int32)
	return a, a.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { count.Add(1); w.WriteHeader(204) })), count
}
func call(h http.Handler, method, path string, headers map[string]string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "http://localhost:7777"+path, nil)
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}
func nativeHeaders(a *Auth) map[string]string {
	return map[string]string{"Authorization": "Bearer " + a.AdministratorSecret()}
}
func issue(t *testing.T, a *Auth, h http.Handler) string {
	t.Helper()
	w := call(h, "POST", "/v1/auth/launch-codes", nativeHeaders(a))
	if w.Code != 201 {
		t.Fatalf("issue status %d", w.Code)
	}
	var v struct{ Code string }
	if json.Unmarshal(w.Body.Bytes(), &v) != nil || !validToken(v.Code) {
		t.Fatal("invalid issue payload")
	}
	return v.Code
}
func launch(t *testing.T, a *Auth, h http.Handler) (*http.Cookie, string) {
	t.Helper()
	w := call(h, "GET", "/auth/launch?code="+issue(t, a, h), nil)
	if w.Code != 303 || w.Header().Get("Location") != "/" {
		t.Fatal("launch did not redirect to code-free root")
	}
	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatal("missing cookie")
	}
	c := cookies[0]
	if !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteStrictMode || c.Path != "/" || c.Domain != "" || c.MaxAge != 28800 || !validToken(c.Value) {
		t.Fatal("unsafe cookie attributes")
	}
	w = call(h, "GET", "/v1/auth/session", map[string]string{"Cookie": c.String()})
	var v struct {
		CSRF string `json:"csrfToken"`
	}
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &v) != nil || !validToken(v.CSRF) {
		t.Fatal("session read failed")
	}
	if strings.Contains(w.Body.String(), c.Value) || strings.Contains(w.Body.String(), a.AdministratorSecret()) {
		t.Fatal("credential exposed in session JSON")
	}
	return c, v.CSRF
}
func TestLaunchGuardsAndNoRejectedDispatch(t *testing.T) {
	a, h, count := setup(t)
	c, csrf := launch(t, a, h)
	for _, tc := range []struct {
		name, method, path string
		headers            map[string]string
		status             int
	}{
		{"missing", "POST", "/command", nil, 401},
		{"bearer mutation", "POST", "/command", nativeHeaders(a), 204},
		{"cookie mutation", "POST", "/command", map[string]string{"Cookie": c.String(), "Origin": "http://localhost:7777", "X-Amaru-CSRF": csrf}, 204},
		{"missing origin", "POST", "/command", map[string]string{"Cookie": c.String(), "X-Amaru-CSRF": csrf}, 403},
		{"foreign origin", "GET", "/command", map[string]string{"Cookie": c.String(), "Origin": "http://attacker.test"}, 403},
		{"null origin", "GET", "/command", map[string]string{"Cookie": c.String(), "Origin": "null"}, 403},
		{"missing csrf", "POST", "/command", map[string]string{"Cookie": c.String(), "Origin": "http://localhost:7777"}, 403},
		{"wrong csrf", "POST", "/command", map[string]string{"Cookie": c.String(), "Origin": "http://localhost:7777", "X-Amaru-CSRF": "wrong"}, 403},
		{"mixed", "POST", "/command", map[string]string{"Cookie": c.String(), "Authorization": "Bearer " + a.AdministratorSecret()}, 401},
		{"bearer origin", "POST", "/command", map[string]string{"Origin": "http://localhost:7777", "Authorization": "Bearer " + a.AdministratorSecret()}, 401},
		{"query token", "GET", "/projects/p/events?token=x", nativeHeaders(a), 400},
		{"cookie issuance", "POST", "/v1/auth/launch-codes", map[string]string{"Cookie": c.String(), "Origin": "http://localhost:7777", "X-Amaru-CSRF": csrf}, 401},
		{"bearer session", "GET", "/v1/auth/session", nativeHeaders(a), 401},
		{"websocket no origin", "GET", "/projects/p/events", map[string]string{"Cookie": c.String(), "Upgrade": "websocket"}, 403},
		{"websocket cookie", "GET", "/projects/p/events", map[string]string{"Cookie": c.String(), "Origin": "http://localhost:7777", "Upgrade": "websocket"}, 204},
		{"websocket native", "GET", "/projects/p/events", map[string]string{"Authorization": "Bearer " + a.AdministratorSecret(), "Upgrade": "websocket"}, 204},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := count.Load()
			w := call(h, tc.method, tc.path, tc.headers)
			if w.Code != tc.status {
				t.Fatalf("status %d want %d", w.Code, tc.status)
			}
			if tc.status != 204 && count.Load() != before {
				t.Fatal("rejection dispatched command")
			}
			if w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("Referrer-Policy") != "no-referrer" {
				t.Fatal("missing privacy headers")
			}
			if tc.status >= 400 && (strings.Contains(w.Body.String(), a.AdministratorSecret()) || strings.Contains(w.Body.String(), csrf) || strings.Contains(w.Body.String(), c.Value)) {
				t.Fatal("credential in error")
			}
		})
	}
	for _, host := range []string{"attacker.test:7777", "localhost:7778", "localhost", "LOCALHOST:7777", "127.0.0.1:07777", "localhost:7777@evil"} {
		r := httptest.NewRequest("GET", "http://localhost:7777/v1/health", nil)
		r.Host = host
		r.Header.Set("X-Forwarded-Host", "localhost:7777")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 403 {
			t.Fatal("host accepted")
		}
	}
	for _, host := range []string{"localhost:7777", "127.0.0.1:7777", "[::1]:7777"} {
		r := httptest.NewRequest("GET", "http://localhost:7777/v1/health", nil)
		r.Host = host
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 204 {
			t.Fatal("bound loopback host rejected")
		}
	}
}
func TestExactExpiryReplayAndConcurrentExchange(t *testing.T) {
	a, h, _ := setup(t)
	now := time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC)
	a.now = func() time.Time { return now }
	code := issue(t, a, h)
	now = now.Add(time.Minute)
	if call(h, "GET", "/auth/launch?code="+code, nil).Code != 401 {
		t.Fatal("code accepted at exact expiry")
	}
	code = issue(t, a, h)
	var ok atomic.Int32
	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			if call(h, "GET", "/auth/launch?code="+code, nil).Code == 303 {
				ok.Add(1)
			}
		})
	}
	wg.Wait()
	if ok.Load() != 1 {
		t.Fatal("code was not single-use")
	}
	c, _ := launch(t, a, h)
	now = now.Add(SessionLifetime - time.Nanosecond)
	if call(h, "GET", "/v1/auth/session", map[string]string{"Cookie": c.String()}).Code != 200 {
		t.Fatal("session expired early")
	}
	now = now.Add(time.Nanosecond)
	if call(h, "GET", "/v1/auth/session", map[string]string{"Cookie": c.String()}).Code != 401 {
		t.Fatal("session accepted at expiry")
	}
	for _, q := range []string{"", "?code=bad", "?code=" + code + "&code=" + code, "?code=" + code + "&redirect=https://evil.test"} {
		if call(h, "GET", "/auth/launch"+q, nil).Code != 401 {
			t.Fatal("malformed launch accepted")
		}
	}
}
func TestLogoutRestartAndSocketLifetime(t *testing.T) {
	a, _, _ := setup(t)
	done := make(chan struct{}, 2)
	h := a.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		WatchSocket(r, func() { done <- struct{}{} })
		w.WriteHeader(204)
	}))
	c, csrf := launch(t, a, h)
	other, _ := launch(t, a, h)
	headers := map[string]string{"Cookie": c.String(), "Origin": "http://localhost:7777", "Upgrade": "websocket"}
	if call(h, "GET", "/events", headers).Code != 204 {
		t.Fatal("socket authorization failed")
	}
	w := call(h, "DELETE", "/v1/auth/session", map[string]string{"Cookie": c.String(), "Origin": "http://localhost:7777", "X-Amaru-CSRF": csrf})
	if w.Code != 204 || len(w.Result().Cookies()) != 1 || w.Result().Cookies()[0].MaxAge != -1 {
		t.Fatal("logout did not expire cookie")
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("logout did not close socket")
	}
	if call(h, "GET", "/v1/auth/session", map[string]string{"Cookie": c.String()}).Code != 401 {
		t.Fatal("revoked session accepted")
	}
	if call(h, "GET", "/v1/auth/session", map[string]string{"Cookie": other.String()}).Code != 200 {
		t.Fatal("logout revoked independent session")
	}
	old := a.AdministratorSecret()
	code := issue(t, a, h)
	a.Close()
	replacement, err := New(&net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 7777})
	if err != nil {
		t.Fatal("restart failed")
	}
	defer replacement.Close()
	if equal(old, replacement.AdministratorSecret()) {
		t.Fatal("secret did not rotate")
	}
	rh := replacement.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("old credential dispatched") }))
	for _, request := range []struct {
		path    string
		headers map[string]string
	}{{"/command", map[string]string{"Authorization": "Bearer " + old}}, {"/v1/auth/session", map[string]string{"Cookie": other.String()}}, {"/auth/launch?code=" + code, nil}} {
		if call(rh, "GET", request.path, request.headers).Code != 401 {
			t.Fatal("restart accepted prior credential")
		}
	}
	// Expiry closes a registered socket even without a new request.
	replacement.mu.Lock()
	replacement.now = time.Now
	replacement.mu.Unlock()
	hc := replacement.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		WatchSocket(r, func() { done <- struct{}{} })
		w.WriteHeader(204)
	}))
	c, _ = launch(t, replacement, hc)
	call(hc, "GET", "/events", map[string]string{"Cookie": c.String(), "Origin": "http://localhost:7777", "Upgrade": "websocket"})
	replacement.mu.Lock()
	s := replacement.sessions[c.Value]
	s.timer.Reset(time.Millisecond)
	replacement.mu.Unlock()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("expiry did not close socket")
	}
}

type failedEntropy struct{}

func (failedEntropy) Read([]byte) (int, error) { return 0, errors.New("entropy failure") }
func TestAdmissionAndEntropyFailClosed(t *testing.T) {
	a, h, _ := setup(t)
	a.limit = 1
	first := issue(t, a, h)
	if call(h, "POST", "/v1/auth/launch-codes", nativeHeaders(a)).Code != 429 {
		t.Fatal("unbounded codes")
	}
	if call(h, "GET", "/auth/launch?code="+first, nil).Code != 303 {
		t.Fatal("launch failed")
	}
	second := issue(t, a, h)
	if call(h, "GET", "/auth/launch?code="+second, nil).Code != 429 {
		t.Fatal("unbounded sessions")
	}
	a.entropy = failedEntropy{}
	if call(h, "POST", "/v1/auth/launch-codes", nativeHeaders(a)).Code != 500 || len(a.codes) != 0 {
		t.Fatal("entropy failure issued code")
	}
	a.limit = 2
	a.entropy = bytes.NewReader(bytes.Repeat([]byte{1}, 32))
	code := issue(t, a, h)
	if call(h, "GET", "/auth/launch?code="+code, nil).Code != 500 {
		t.Fatal("entropy failure created session")
	}
	if len(a.sessions) != 1 {
		t.Fatal("partial session published")
	}
}
func TestLoopbackAndRandomTokens(t *testing.T) {
	for _, addr := range []net.Addr{&net.TCPAddr{IP: net.ParseIP("0.0.0.0"), Port: 7777}, &net.TCPAddr{IP: net.ParseIP("192.0.2.1"), Port: 7777}, &net.TCPAddr{IP: net.ParseIP("::"), Port: 7777}, &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0}} {
		if a, err := New(addr); err == nil {
			a.Close()
			t.Fatal("unsafe listener accepted")
		}
	}
	a, _, _ := setup(t)
	b, err := a.token()
	if err != nil || !validToken(b) || equal(a.secret, b) || equal("", "") {
		t.Fatal("invalid randomness or comparison")
	}
}
func TestRunningLoopbackAuth(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	a, err := New(l.Addr())
	if err != nil {
		l.Close()
		t.Fatal("auth init")
	}
	defer a.Close()
	srv := httptest.NewUnstartedServer(a.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) })))
	srv.Listener.Close()
	srv.Listener = l
	srv.Start()
	defer srv.Close()
	req, _ := http.NewRequest("POST", srv.URL+"/v1/auth/launch-codes", nil)
	req.Header.Set("Authorization", "Bearer "+a.AdministratorSecret())
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal("issue transport")
	}
	defer resp.Body.Close()
	var v struct{ Code string }
	if resp.StatusCode != 201 || json.NewDecoder(resp.Body).Decode(&v) != nil {
		t.Fatal("issue response")
	}
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err = client.Get(srv.URL + "/auth/launch?code=" + v.Code)
	if err != nil {
		t.Fatal("launch transport")
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 303 || resp.Header.Get("Location") != "/" || len(body) != 0 {
		t.Fatal("unsafe launch redirect")
	}
}

func TestAmbiguousHeadersAndBodyCannotBypassGuards(t *testing.T) {
	a, h, count := setup(t)
	c, csrf := launch(t, a, h)
	for _, mutate := range []func(*http.Request){
		func(r *http.Request) { r.Header.Add("Authorization", "Bearer "+a.AdministratorSecret()) },
		func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+a.AdministratorSecret()) },
		func(r *http.Request) { r.Header.Add("Cookie", c.String()) },
		func(r *http.Request) { r.Header.Add("Origin", "http://localhost:7777") },
		func(r *http.Request) { r.Header.Add("X-Amaru-CSRF", csrf) },
	} {
		r := httptest.NewRequest("POST", "http://localhost:7777/command", nil)
		r.Header.Set("Cookie", c.String())
		r.Header.Set("Origin", "http://localhost:7777")
		r.Header.Set("X-Amaru-CSRF", csrf)
		mutate(r)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code < 400 || count.Load() != 0 {
			t.Fatal("ambiguous credentials bypassed guards")
		}
	}
	for _, body := range []io.Reader{strings.NewReader("unexpected"), io.NopCloser(strings.NewReader("chunked"))} {
		r := httptest.NewRequest("POST", "http://localhost:7777/v1/auth/launch-codes", body)
		r.Header.Set("Authorization", "Bearer "+a.AdministratorSecret())
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 400 {
			t.Fatal("issuance accepted body")
		}
	}
}
func TestNativeSocketClosesOnShutdown(t *testing.T) {
	a, _, _ := setup(t)
	done := make(chan struct{})
	h := a.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		WatchSocket(r, func() { close(done) })
		w.WriteHeader(204)
	}))
	if call(h, "GET", "/events", nativeHeaders(a)).Code != 204 {
		t.Fatal("native socket denied")
	}
	a.Close()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("native socket survived shutdown")
	}
}

func TestOriginMustBeExactSerializedLoopbackOrigin(t *testing.T) {
	a, h, count := setup(t)
	c, csrf := launch(t, a, h)
	for _, origin := range []string{"http://localhost:7777?", "http://localhost:7777#", "http://localhost:7777/", "http://localhost:7777/path", "http://localhost:7777@evil.test", "http://user@localhost:7777", "https://localhost:7777", "null", ""} {
		w := call(h, "POST", "/command", map[string]string{"Cookie": c.String(), "Origin": origin, "X-Amaru-CSRF": csrf})
		if w.Code != 403 || count.Load() != 0 {
			t.Fatal("non-exact Origin was accepted")
		}
	}
}
