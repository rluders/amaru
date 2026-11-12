//go:build browser

package localauth

import (
	"context"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strconv"
	"sync"
	"testing"
	"time"
)

// Explicit real-tool validation; excluded from make check. No profile, cookie,
// credential, raw request URL or browser output is retained in the repository.
func TestFirefoxLaunchSessionAndLogout(t *testing.T) {
	executable, err := exec.LookPath("firefox")
	if err != nil {
		t.Fatal("Firefox not installed")
	}
	for _, host := range []string{"localhost", "127.0.0.1"} {
		t.Run(host, func(t *testing.T) {
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
			result := make(chan string, 1)
			var once sync.Once
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/":
					w.Header().Set("Content-Type", "text/html")
					io.WriteString(w, `<!doctype html><title>Auth validation</title><script>
(async()=>{try {
 const s=await fetch('/v1/auth/session');if(s.status!==200)throw 1;const session=await s.json();
 if(document.cookie.includes('amaru_session'))throw 2;
 const reject=await fetch('/probe',{method:'POST'});if(reject.status!==403)throw 3;
 const ok=await fetch('/probe',{method:'POST',headers:{'X-Amaru-CSRF':session.csrfToken}});if(ok.status!==204)throw 4;
 const ws=new WebSocket('ws://'+location.host+'/events');
 await new Promise((resolve,reject)=>{ws.onopen=resolve;ws.onerror=reject});
 const closed=new Promise(resolve=>ws.onclose=resolve);
 const logout=await fetch('/v1/auth/session',{method:'DELETE',headers:{'X-Amaru-CSRF':session.csrfToken}});if(logout.status!==204)throw 5;
 await closed;
 if((await fetch('/v1/auth/session')).status!==401)throw 6;
 await fetch('/v1/health?result=ok');
} catch {await fetch('/v1/health?result=failed')}})();</script>`)
				case "/probe":
					w.WriteHeader(204)
				case "/v1/health":
					if v := r.URL.Query().Get("result"); v != "" {
						once.Do(func() { result <- v })
					}
					io.WriteString(w, `{"status":"ok"}`)
				case "/events":
					hijacker, ok := w.(http.Hijacker)
					if !ok {
						t.Error("no hijacker")
						return
					}
					conn, b, err := hijacker.Hijack()
					if err != nil {
						return
					}
					key := r.Header.Get("Sec-WebSocket-Key")
					sum := sha1.Sum([]byte(key + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
					fmt.Fprintf(b, "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: %s\r\n\r\n", base64.StdEncoding.EncodeToString(sum[:]))
					b.Flush()
					WatchSocket(r, func() { conn.Write([]byte{0x88, 0x02, 0x03, 0xe8}); conn.Close() })
				default:
					w.WriteHeader(404)
				}
			})
			h := a.Handler(next)
			server := httptest.NewUnstartedServer(h)
			server.Listener.Close()
			server.Listener = l
			server.Start()
			defer server.Close()
			// Native issuance, then exactly the browser handoff permitted by ADR-0034.
			code := issueBrowser(t, a, h, l.Addr().(*net.TCPAddr).Port)
			target := "http://" + net.JoinHostPort(host, strconv.Itoa(l.Addr().(*net.TCPAddr).Port)) + "/auth/launch?code=" + code
			ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, executable, "--headless", "--no-remote", "--profile", t.TempDir(), target)
			// Browser diagnostics can contain navigation URLs, so never echo them.
			cmd.Stdout = io.Discard
			cmd.Stderr = io.Discard
			if cmd.Start() != nil {
				t.Fatal("Firefox start failed")
			}
			defer func() { cancel(); cmd.Wait() }()
			select {
			case status := <-result:
				if status != "ok" {
					t.Fatal("browser authentication probe failed")
				}
			case <-ctx.Done():
				t.Fatal("browser authentication probe timed out")
			}
		})
	}
}
func issueBrowser(t *testing.T, a *Auth, h http.Handler, port int) string {
	t.Helper()
	r := httptest.NewRequest("POST", "http://localhost:"+strconv.Itoa(port)+"/v1/auth/launch-codes", nil)
	r.Header.Set("Authorization", "Bearer "+a.AdministratorSecret())
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 201 {
		t.Fatal("browser issuance failed")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	for code := range a.codes {
		return code
	}
	t.Fatal("no code")
	return ""
}
