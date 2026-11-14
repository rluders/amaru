package localauth

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

// No socket or browser is needed for exact serialized Host/Origin rejection.
func TestIPv6HostOriginSession(t *testing.T) {
	a, err := New(&net.TCPAddr{IP: net.IPv6loopback, Port: 7777})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	dispatched := 0
	h := a.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { dispatched++; w.WriteHeader(204) }))
	request := func(method, path, host, origin, cookie, csrf, bearer string, upgrade bool) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "http://[::1]:7777"+path, nil)
		r.Host = host
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		if cookie != "" {
			r.Header.Set("Cookie", cookie)
		}
		if csrf != "" {
			r.Header.Set("X-Amaru-CSRF", csrf)
		}
		if bearer != "" {
			r.Header.Set("Authorization", "Bearer "+bearer)
		}
		if upgrade {
			r.Header.Set("Upgrade", "websocket")
			r.Header.Set("Connection", "Upgrade")
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	host := "[::1]:7777"
	origin := "http://" + host
	w := request("POST", "/v1/auth/launch-codes", host, "", "", "", a.AdministratorSecret(), false)
	var code struct{ Code string }
	if w.Code != 201 || json.Unmarshal(w.Body.Bytes(), &code) != nil || !validToken(code.Code) {
		t.Fatal("IPv6 issuance failed")
	}
	// A rejected Host must not consume the launch code.
	if w := request("GET", "/auth/launch?code="+code.Code, "[::1]:7778", "", "", "", "", false); w.Code != 403 {
		t.Fatal("wrong-port launch accepted")
	}
	w = request("GET", "/auth/launch?code="+code.Code, host, "", "", "", "", false)
	if w.Code != 303 || w.Header().Get("Location") != "/" || len(w.Result().Cookies()) != 1 {
		t.Fatal("IPv6 launch failed")
	}
	c := w.Result().Cookies()[0]
	if !c.Secure || !c.HttpOnly || c.Domain != "" || c.Path != "/" || c.SameSite != http.SameSiteStrictMode {
		t.Fatal("unsafe IPv6 cookie")
	}
	cookie := c.Name + "=" + c.Value
	if w := request("GET", "/auth/launch?code="+code.Code, host, "", "", "", "", false); w.Code != 401 {
		t.Fatal("IPv6 replay accepted")
	}
	var session struct {
		CSRF string `json:"csrfToken"`
	}
	w = request("GET", "/v1/auth/session", host, origin, cookie, "", "", false)
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &session) != nil || !validToken(session.CSRF) {
		t.Fatal("IPv6 session read failed")
	}
	for _, badHost := range []string{"::1:7777", "[::1]", "[::1]:7778", "[::]:7777", "[::2]:7777", "[::1%lo]:7777", "[0:0:0:0:0:0:0:1]:7777", "attacker.invalid:7777"} {
		t.Run("host/"+badHost, func(t *testing.T) {
			for _, path := range []string{"/v1/health", "/v1/auth/session", "/probe"} {
				if w := request("GET", path, badHost, "", cookie, "", "", false); w.Code != 403 {
					t.Fatalf("Host accepted at %s: %d", path, w.Code)
				}
			}
		})
	}
	for _, badOrigin := range []string{"", "null", "http://[::1]:7778", "https://[::1]:7777", "http://::1:7777", "http://[::1]:7777/", "http://[::1%lo]:7777", "http://[::2]:7777", "http://attacker.invalid:7777"} {
		t.Run("origin/"+badOrigin, func(t *testing.T) {
			for _, op := range []struct {
				method, path string
				upgrade      bool
			}{{"POST", "/probe", false}, {"DELETE", "/v1/auth/session", false}, {"GET", "/events", true}} {
				if w := request(op.method, op.path, host, badOrigin, cookie, session.CSRF, "", op.upgrade); w.Code != 403 {
					t.Fatalf("Origin accepted for %s: %d", op.path, w.Code)
				}
			}
		})
	}
	if dispatched != 0 {
		t.Fatal("rejection dispatched downstream")
	}
	for _, csrf := range []string{"", "wrong"} {
		if w := request("POST", "/probe", host, origin, cookie, csrf, "", false); w.Code != 403 {
			t.Fatal("invalid CSRF accepted")
		}
	}
	if w := request("POST", "/probe", host, origin, "", session.CSRF, "", false); w.Code != 401 {
		t.Fatal("missing session accepted")
	}
	if w := request("POST", "/probe", host, origin, cookie, session.CSRF, a.AdministratorSecret(), false); w.Code != 401 {
		t.Fatal("mixed credentials accepted")
	}
	if dispatched != 0 {
		t.Fatal("rejected credentials dispatched downstream")
	}
	if w := request("POST", "/probe", host, origin, cookie, session.CSRF, "", false); w.Code != 204 {
		t.Fatal("IPv6 mutation failed")
	}
	if w := request("GET", "/events", host, origin, cookie, "", "", true); w.Code != 204 {
		t.Fatal("IPv6 socket authorization failed")
	}
	if dispatched != 2 {
		t.Fatal("positive controls not dispatched")
	}
	if w := request("DELETE", "/v1/auth/session", host, origin, cookie, session.CSRF, "", false); w.Code != 204 {
		t.Fatal("IPv6 logout failed")
	}
	if w := request("GET", "/v1/auth/session", host, "", cookie, "", "", false); w.Code != 401 {
		t.Fatal("revoked IPv6 session accepted")
	}
}
