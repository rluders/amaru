// Package localauth implements ADR-0034/0061 at the compact HTTP boundary.
// It owns ephemeral credentials and supplies the compact local administrator identity.
package localauth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rluders/amaru/services/platform/internal/domain"
)

const CookieName = "amaru_session"
const CodeLifetime = time.Minute
const SessionLifetime = 8 * time.Hour
const capacity = 1024

// authPatterns are registered by Handler, including absolute bases.
var authPatterns = []string{"POST /v1/auth/launch-codes", "GET /auth/launch", "GET /v1/auth/session", "DELETE /v1/auth/session"}

type session struct {
	csrf    string
	expires time.Time
	ctx     context.Context
	cancel  context.CancelFunc
	timer   *time.Timer
}

type Auth struct {
	mu           sync.Mutex
	secret       string
	port         string
	codes        map[string]time.Time
	sessions     map[string]*session
	closed       bool
	now          func() time.Time
	entropy      io.Reader
	limit        int
	nativeCtx    context.Context
	nativeCancel context.CancelFunc
}

// New accepts an already-bound loopback listener address, never an advertised
// endpoint. Publishing discovery is a separate step after binding.
func New(addr net.Addr) (*Auth, error) {
	tcp, ok := addr.(*net.TCPAddr)
	if !ok || (!tcp.IP.Equal(net.IPv4(127, 0, 0, 1)) && !tcp.IP.Equal(net.IPv6loopback)) || tcp.Zone != "" || tcp.Port < 1 || tcp.Port > 65535 {
		return nil, errors.New("authentication requires a bound loopback listener")
	}
	a := &Auth{port: strconv.Itoa(tcp.Port), codes: map[string]time.Time{}, sessions: map[string]*session{}, now: time.Now, entropy: rand.Reader, limit: capacity}
	var err error
	a.secret, err = a.token()
	if err != nil {
		return nil, errors.New("authentication entropy unavailable")
	}
	a.nativeCtx, a.nativeCancel = context.WithCancel(context.Background())
	return a, nil
}

// AdministratorSecret is for private OS-user discovery and native clients only.
// Never log it, render it, or place it in a URL.
func (a *Auth) AdministratorSecret() string { a.mu.Lock(); defer a.mu.Unlock(); return a.secret }

// Close invalidates every credential and closes all session watches.
func (a *Auth) Close() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.closed = true
	a.secret = ""
	clear(a.codes)
	a.nativeCancel()
	for id := range a.sessions {
		a.remove(id)
	}
}

func (a *Auth) token() (string, error) {
	var b [32]byte
	if _, err := io.ReadFull(a.entropy, b[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b[:]), nil
}
func equal(a, b string) bool {
	return a != "" && b != "" && subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
func validToken(s string) bool {
	b, err := base64.RawURLEncoding.Strict().DecodeString(s)
	return err == nil && len(b) == 32
}
func (a *Auth) host(h string) bool {
	return h == net.JoinHostPort("127.0.0.1", a.port) || h == net.JoinHostPort("::1", a.port) || h == net.JoinHostPort("localhost", a.port)
}
func (a *Auth) origin(h http.Header, required bool) bool {
	values, exists := h["Origin"]
	if !exists {
		return !required
	}
	if len(values) != 1 {
		return false
	}
	host, ok := strings.CutPrefix(values[0], "http://")
	return ok && a.host(host)
}
func safeHeaders(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Amaru-Protocol", "1")
}
func reject(w http.ResponseWriter, status int, code string) {
	safeHeaders(w)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	// Fixed messages prevent secrets from appearing in error bodies or diagnostics.
	_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": code, "message": "Request rejected."}})
}
func jsonResponse(w http.ResponseWriter, status int, v any) {
	safeHeaders(w)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func (a *Auth) remove(id string) {
	s := a.sessions[id]
	s.timer.Stop()
	s.cancel()
	delete(a.sessions, id)
}
func (a *Auth) expire() {
	now := a.now()
	for code, expiry := range a.codes {
		if !now.Before(expiry) {
			delete(a.codes, code)
		}
	}
	for id, s := range a.sessions {
		if !now.Before(s.expires) {
			a.remove(id)
		}
	}
}
func (a *Auth) native(r *http.Request) bool {
	if len(r.Header.Values("Authorization")) != 1 || len(r.Header.Values("Cookie")) != 0 {
		return false
	}
	if _, present := r.Header["Origin"]; present {
		return false
	}
	v, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	return ok && equal(v, a.secret)
}
func (a *Auth) browser(r *http.Request) *session {
	if _, present := r.Header["Authorization"]; present {
		return nil
	}
	var id string
	n := 0
	for _, c := range r.Cookies() {
		if c.Name == CookieName {
			id = c.Value
			n++
		}
	}
	if n != 1 || !validToken(id) {
		return nil
	}
	return a.sessions[id]
}

type administratorKey struct{}

type sessionKey struct{}

// WatchSocket registers a transport's close function before upgrading a cookie
// connection. Stop the watch when the socket closes. Native connections are
// revoked by Auth.Close too. No socket may outlive its credential context.
func WatchSocket(r *http.Request, closeSocket func()) func() bool {
	ctx, ok := r.Context().Value(sessionKey{}).(context.Context)
	if !ok {
		closeSocket()
		return func() bool { return false }
	}
	return context.AfterFunc(ctx, closeSocket)
}

// Handler guards all requests before dispatching to next. Auth operations are
// served here; health and the root page are the only other public paths.
// next must use WatchSocket for an authenticated WebSocket's lifetime.
func (a *Auth) Handler(next http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc(authPatterns[0], a.issue)
	mux.HandleFunc(authPatterns[1], func(w http.ResponseWriter, r *http.Request) { a.exchange(w, r, r.URL.Query()) })
	for _, pattern := range authPatterns[2:] {
		mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) { a.sessionOperation(w, r, a.browser(r)) })
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		safeHeaders(w)
		if !a.host(r.Host) {
			reject(w, 403, "forbidden")
			return
		}
		query, err := url.ParseQuery(r.URL.RawQuery)
		if err != nil {
			reject(w, 400, "invalid_request")
			return
		}
		if _, exists := query["token"]; exists {
			reject(w, 400, "invalid_request")
			return
		}
		if !a.origin(r.Header, false) {
			reject(w, 403, "forbidden_origin")
			return
		}
		a.mu.Lock()
		a.expire()
		if a.closed {
			a.mu.Unlock()
			reject(w, 401, "unauthorized")
			return
		}
		switch r.URL.Path {
		case "/auth/launch":
			mux.ServeHTTP(w, r)
			a.mu.Unlock()
			return
		case "/v1/auth/launch-codes":
			mux.ServeHTTP(w, r)
			a.mu.Unlock()
			return
		}
		s := a.browser(r)
		native := a.native(r)
		if r.URL.Path == "/v1/health" && r.Method == http.MethodGet {
			a.mu.Unlock()
			next.ServeHTTP(w, r)
			return
		}
		if r.URL.Path == "/" && r.Method == http.MethodGet && s == nil && !native {
			a.mu.Unlock()
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			_, _ = io.WriteString(w, "Run `amaru open` to open this Instance.\n")
			return
		}
		if s == nil && !native {
			a.mu.Unlock()
			reject(w, 401, "unauthorized")
			return
		}
		upgrade := strings.EqualFold(r.Header.Get("Upgrade"), "websocket")
		if s != nil {
			if !a.origin(r.Header, r.Method != http.MethodGet || upgrade) {
				a.mu.Unlock()
				reject(w, 403, "forbidden_origin")
				return
			}
			if r.Method != http.MethodGet && (len(r.Header.Values("X-Amaru-CSRF")) != 1 || !equal(r.Header.Get("X-Amaru-CSRF"), s.csrf)) {
				a.mu.Unlock()
				reject(w, 403, "forbidden")
				return
			}
		}
		if r.URL.Path == "/v1/auth/session" {
			mux.ServeHTTP(w, r)
			a.mu.Unlock()
			return
		}
		if s != nil {
			r = r.WithContext(context.WithValue(r.Context(), sessionKey{}, s.ctx))
		} else {
			// A native socket lasts at most until this Auth instance closes.
			r = r.WithContext(context.WithValue(r.Context(), sessionKey{}, a.nativeCtx))
		}
		r = r.WithContext(context.WithValue(r.Context(), administratorKey{}, a))
		a.mu.Unlock()
		next.ServeHTTP(w, r)
	})
}

func (a *Auth) issue(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		reject(w, 405, "invalid_request")
		return
	}
	if !a.native(r) {
		reject(w, 401, "unauthorized")
		return
	}
	if r.ContentLength != 0 || len(r.TransferEncoding) != 0 {
		reject(w, 400, "invalid_request")
		return
	}
	if len(a.codes) >= a.limit {
		reject(w, 429, "rate_limited")
		return
	}
	code, err := a.token()
	if err != nil {
		reject(w, 500, "internal")
		return
	}
	expires := a.now().Add(CodeLifetime)
	a.codes[code] = expires
	jsonResponse(w, 201, map[string]any{"code": code, "expiresAt": expires.UTC()})
}
func cookie(value string, maxAge int, expiry time.Time) *http.Cookie {
	// localhost is the portable browser Secure exception on HTTP loopback.
	// Real browser evidence is required before claiming supported cookie delivery.
	return &http.Cookie{Name: CookieName, Value: value, Path: "/", HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode, MaxAge: maxAge, Expires: expiry.UTC()}
}
func (a *Auth) exchange(w http.ResponseWriter, r *http.Request, q url.Values) {
	if r.Method != http.MethodGet {
		reject(w, 405, "invalid_request")
		return
	}
	if len(q) != 1 || len(q["code"]) != 1 || !validToken(q.Get("code")) {
		reject(w, 401, "unauthorized")
		return
	}
	code := q.Get("code")
	expires, ok := a.codes[code]
	if !ok || !a.now().Before(expires) {
		reject(w, 401, "unauthorized")
		return
	}
	delete(a.codes, code) // Failed exchange never makes the code reusable.
	if len(a.sessions) >= a.limit {
		reject(w, 429, "rate_limited")
		return
	}
	id, err := a.token()
	if err != nil {
		reject(w, 500, "internal")
		return
	}
	csrf, err := a.token()
	if err != nil {
		reject(w, 500, "internal")
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	expiry := a.now().Add(SessionLifetime)
	s := &session{csrf: csrf, expires: expiry, ctx: ctx, cancel: cancel}
	s.timer = time.AfterFunc(SessionLifetime, func() {
		a.mu.Lock()
		defer a.mu.Unlock()
		if a.sessions[id] == s {
			a.remove(id)
		}
	})
	a.sessions[id] = s
	http.SetCookie(w, cookie(id, int(SessionLifetime.Seconds()), expiry))
	w.Header().Set("Location", "/")
	w.WriteHeader(303)
}
func (a *Auth) sessionOperation(w http.ResponseWriter, r *http.Request, s *session) {
	if s == nil {
		reject(w, 401, "unauthorized")
		return
	}
	switch r.Method {
	case http.MethodGet:
		jsonResponse(w, 200, map[string]any{"csrfToken": s.csrf, "expiresAt": s.expires.UTC()})
	case http.MethodDelete:
		c, _ := r.Cookie(CookieName)
		a.remove(c.Value)
		http.SetCookie(w, cookie("", -1, time.Unix(1, 0)))
		w.WriteHeader(204)
	default:
		reject(w, 405, "invalid_request")
	}
}

// Principal rechecks Handler-authenticated context on every application call, including replay.
// Health/root requests, another Auth instance, logout, expiry and shutdown grant no identity.
func (a *Auth) Principal(ctx context.Context) (domain.Principal, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.expire()
	lifetime, ok := ctx.Value(sessionKey{}).(context.Context)
	if a.closed || ctx.Err() != nil || ctx.Value(administratorKey{}) != a || !ok || lifetime.Err() != nil {
		return domain.Principal{}, domain.ErrNotPermitted
	}
	return domain.Principal{Kind: domain.PrincipalHuman, ID: "local-administrator", Role: domain.RoleOwner}, nil
}
