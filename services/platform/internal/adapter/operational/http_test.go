package operational

import (
	"bytes"
	"encoding/json"
	"github.com/rluders/amaru/services/platform/internal/adapter/localauth"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTPAuthCaptureOmitsCredentials(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	auth, err := localauth.New(listener.Addr())
	if err != nil {
		t.Fatal(err)
	}
	defer auth.Close()
	var buf bytes.Buffer
	l, _ := New(&buf)
	server := httptest.NewUnstartedServer(l.HTTP("instance", auth.Handler(http.NotFoundHandler()), func(err error) { t.Error(err) }))
	server.Listener.Close()
	server.Listener = listener
	server.Start()
	defer server.Close()
	client := server.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	request := func(method, path string, headers map[string]string, body io.Reader) *http.Response {
		t.Helper()
		req, err := http.NewRequest(method, server.URL+path, body)
		if err != nil {
			t.Fatal(err)
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		res, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		return res
	}
	res := request("POST", "/v1/auth/launch-codes", map[string]string{"Authorization": "Bearer " + auth.AdministratorSecret()}, nil)
	var launch struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(res.Body).Decode(&launch); err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if launch.Code == "" {
		t.Fatal("no launch code")
	}
	res = request("GET", "/auth/launch?code="+launch.Code, nil, nil)
	if res.StatusCode != 303 {
		t.Fatalf("launch: %d", res.StatusCode)
	}
	cookies := res.Cookies()
	res.Body.Close()
	if len(cookies) != 1 {
		t.Fatal("no cookie")
	}
	res = request("GET", "/v1/auth/session", map[string]string{"Cookie": cookies[0].String()}, nil)
	var session struct {
		CSRF string `json:"csrfToken"`
	}
	if err := json.NewDecoder(res.Body).Decode(&session); err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if session.CSRF == "" {
		t.Fatal("no CSRF")
	}
	res = request("DELETE", "/v1/auth/session?token=unregistered-query", map[string]string{"Cookie": cookies[0].String(), "X-Amaru-CSRF": session.CSRF, "Origin": "https://foreign.invalid", "Authorization": "Bearer opaque-header", "X-Correlation-ID": "untrusted-correlation"}, strings.NewReader("private-body"))
	if res.StatusCode < 400 {
		t.Fatal("hostile request accepted")
	}
	io.Copy(io.Discard, res.Body)
	res.Body.Close()
	server.Close()
	for _, secret := range []string{auth.AdministratorSecret(), launch.Code, cookies[0].Value, session.CSRF, "unregistered-query", "opaque-header", "private-body", "foreign.invalid", "untrusted-correlation"} {
		if strings.Contains(buf.String(), secret) {
			t.Fatal("HTTP log captured credential or untrusted input")
		}
	}
	recs := records(t, &buf)
	if len(recs) != 4 || recs[3]["outcome"] != "rejected" {
		t.Fatal("missing auth records")
	}
	seen := map[string]bool{}
	for _, r := range recs {
		id, ok := r["correlationId"].(string)
		if !ok || id == "" || seen[id] {
			t.Fatal("missing/duplicate correlation")
		}
		seen[id] = true
	}
}
func TestHTTPFailureAndPanic(t *testing.T) {
	for _, panics := range []bool{false, true} {
		l, _ := New(badWriter{})
		called := false
		handler := l.HTTP("instance", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if panics {
				panic("fixture panic")
			}
			w.WriteHeader(204)
		}), func(err error) {
			called = true
			if err.Error() != "operational log write failed" {
				t.Error("unsafe error")
			}
		})
		func() {
			defer func() {
				if got := recover(); (got != nil) != panics {
					t.Error("panic swallowed or introduced")
				}
			}()
			handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
		}()
		if !called {
			t.Fatal("lost logging failure")
		}
	}
}

// Exercise the optional interfaces against a real server, including panic output
// that net/http would otherwise send to its separate ErrorLog channel.
func TestHTTPUpgradeFlushAndPanicChannels(t *testing.T) {
	var output, serverErrors bytes.Buffer
	logger, _ := New(&output)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/panic":
			panic("unregistered-panic-credential")
		case "/stream":
			w.Write([]byte("ok"))
			if err := http.NewResponseController(w).Flush(); err != nil {
				t.Error(err)
			}
		case "/socket":
			conn, rw, err := http.NewResponseController(w).Hijack()
			if err != nil {
				t.Error(err)
				return
			}
			defer conn.Close()
			rw.WriteString("HTTP/1.1 101 Switching Protocols\r\nConnection: Upgrade\r\nUpgrade: websocket\r\n\r\n")
			rw.Flush()
		}
	})
	server := httptest.NewUnstartedServer(logger.HTTP("instance", next, func(err error) { t.Error(err) }))
	server.Config.ErrorLog = log.New(&serverErrors, "", 0)
	server.Start()
	defer server.Close()
	client := server.Client()
	for _, path := range []string{"/stream", "/socket", "/panic"} {
		res, err := client.Get(server.URL + path)
		if path == "/panic" {
			if err == nil {
				res.Body.Close()
				t.Fatal("panic did not abort connection")
			}
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		io.Copy(io.Discard, res.Body)
		res.Body.Close()
	}
	server.Close()
	if serverErrors.Len() != 0 || strings.Contains(output.String(), "unregistered-panic-credential") {
		t.Fatal("unsafe panic diagnostic emitted")
	}
	recs := records(t, &output)
	var upgraded, panicked bool
	for _, r := range recs {
		if r["status"] == float64(101) {
			upgraded = true
		}
		if r["outcome"] == "panicked" {
			panicked = true
			if _, ok := r["status"]; ok {
				t.Fatal("panic claimed a response status")
			}
		}
	}
	if !upgraded || !panicked {
		t.Fatal("missing upgrade/panic outcome")
	}
}
