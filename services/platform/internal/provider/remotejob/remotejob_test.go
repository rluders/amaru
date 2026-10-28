package remotejob

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestDoRetriesARateLimitAndKeepsCredentialsOffTheErrorPath(t *testing.T) {
	var n atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer k" {
			w.WriteHeader(401)
			return
		}
		if n.Add(1) < 3 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(429)
			_, _ = w.Write([]byte(`{"message":"slow down"}`))
			return
		}
		_, _ = w.Write([]byte(`{"result":"abc"}`))
	}))
	defer ts.Close()
	c := &Client{BaseURL: ts.URL, Header: func(h http.Header) { h.Set("Authorization", "Bearer k") }}
	var out struct{ Result string }
	// Not idempotent: a rate limit is still retried, because the service did not act.
	err := Retry(context.Background(), 5, time.Millisecond, false, func() error { return c.Do(context.Background(), "POST", "/x", map[string]int{"a": 1}, &out) })
	if err != nil || out.Result != "abc" || n.Load() != 3 {
		t.Fatalf("%v %v %d", err, out, n.Load())
	}
	c.Header = nil
	err = Retry(context.Background(), 5, time.Millisecond, true, func() error { return c.Do(context.Background(), "GET", "/x", nil, nil) })
	var he *HTTPError
	if !errors.As(err, &he) || he.Status != 401 || he.Retryable() {
		t.Fatalf("%v", err)
	}
}

// A submit that failed with 5xx may have been accepted. Repeating it would create a second paid
// job, so it is repeated only when the call is declared idempotent.
func TestRetryDoesNotRepeatARequestTheServiceMayHaveAccepted(t *testing.T) {
	for _, tc := range []struct {
		idempotent bool
		wantCalls  int32
	}{{false, 1}, {true, 3}} {
		var n atomic.Int32
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if n.Add(1) < 3 {
				w.WriteHeader(502)
				return
			}
			_, _ = w.Write([]byte(`{}`))
		}))
		c := &Client{BaseURL: ts.URL}
		err := Retry(context.Background(), 5, time.Millisecond, tc.idempotent, func() error { return c.Do(context.Background(), "POST", "/submit", nil, nil) })
		ts.Close()
		if n.Load() != tc.wantCalls {
			t.Errorf("idempotent=%v: %d calls, want %d (err %v)", tc.idempotent, n.Load(), tc.wantCalls, err)
		}
		if !tc.idempotent {
			var he *HTTPError
			if !errors.As(err, &he) || he.Status != 502 {
				t.Errorf("the 502 must be returned to the caller, got %v", err)
			}
		}
	}
}

func TestRetryHonorsRetryAfterAndTheContext(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(429)
	}))
	defer ts.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	c := &Client{BaseURL: ts.URL}
	began := time.Now()
	err := Retry(ctx, 5, time.Millisecond, false, func() error { return c.Do(ctx, "GET", "/x", nil, nil) })
	if !errors.Is(err, context.DeadlineExceeded) || time.Since(began) > 5*time.Second {
		t.Fatalf("a long Retry-After must be cut by the context, got %v after %s", err, time.Since(began))
	}
}

func TestResponseBodiesAreBoundedNotTruncated(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"v":"` + strings.Repeat("x", 100) + `"}`))
	}))
	defer ts.Close()
	var out map[string]string
	if err := (&Client{BaseURL: ts.URL, MaxResponse: 50}).Do(context.Background(), "GET", "/", nil, &out); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("got %v, want ErrTooLarge", err)
	}
	if _, _, err := (&Client{BaseURL: ts.URL}).DoRaw(context.Background(), "POST", "/", map[string]int{}, 10); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("DoRaw: got %v, want ErrTooLarge", err)
	}
	if err := (&Client{BaseURL: ts.URL}).Do(context.Background(), "GET", "/", nil, &out); err != nil || len(out["v"]) != 100 {
		t.Fatalf("a body within the limit must decode whole: %v", err)
	}
}

func TestUploadSendsTheFileAndFields(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			w.WriteHeader(400)
			return
		}
		f, h, err := r.FormFile("file")
		if err != nil || h.Filename != "a.png" {
			w.WriteHeader(400)
			return
		}
		buf := make([]byte, 16)
		n, _ := f.Read(buf)
		_, _ = w.Write([]byte(`{"got":"` + string(buf[:n]) + `","mode":"` + r.FormValue("mode") + `"}`))
	}))
	defer ts.Close()
	var out struct{ Got, Mode string }
	err := (&Client{BaseURL: ts.URL}).Upload(context.Background(), "/up", "file", "a.png", "image/png", []byte("PNGDATA"), map[string]string{"mode": "fast"}, &out)
	if err != nil || out.Got != "PNGDATA" || out.Mode != "fast" {
		t.Fatalf("%v %+v", err, out)
	}
}

func TestDownload(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Error("downloads must not carry credentials")
		}
		_, _ = w.Write([]byte("0123456789"))
	}))
	defer ts.Close()
	ctx := context.Background()
	if _, err := Download(ctx, ts.URL, 100, DownloadOptions{}); err == nil {
		t.Fatal("http must be refused")
	}
	test := DownloadOptions{AllowHTTP: true, AllowPrivate: true}
	if b, err := Download(ctx, ts.URL, 100, test); err != nil || string(b) != "0123456789" {
		t.Fatal(err)
	}
	if _, err := Download(ctx, ts.URL, 5, test); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("a body over the limit: got %v, want ErrTooLarge", err)
	}
}

// The default client refuses to connect to a non-public address, whatever the URL says, so a
// result URL cannot point the Worker at an internal service.
func TestDownloadRefusesNonPublicAddresses(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("the server must not be reached")
	}))
	defer ts.Close()
	if _, err := Download(context.Background(), ts.URL, 100, DownloadOptions{AllowHTTP: true}); !errors.Is(err, ErrForbiddenAddress) {
		t.Fatalf("loopback: got %v, want ErrForbiddenAddress", err)
	}
	for _, addr := range []string{"127.0.0.1:80", "10.0.0.5:80", "192.168.1.1:443", "172.16.0.1:80", "169.254.169.254:80", "100.64.0.1:80", "[::1]:80", "[fe80::1]:80", "[fd00::1]:80", "0.0.0.0:80", "[::ffff:127.0.0.1]:80", "224.0.0.1:80"} {
		if err := checkAddress(addr); !errors.Is(err, ErrForbiddenAddress) {
			t.Errorf("%s: got %v, want ErrForbiddenAddress", addr, err)
		}
	}
	for _, addr := range []string{"93.184.216.34:443", "[2606:4700:4700::1111]:443"} {
		if err := checkAddress(addr); err != nil {
			t.Errorf("%s: %v", addr, err)
		}
	}
}

func TestDownloadRedirects(t *testing.T) {
	hops := 0
	var ts *httptest.Server
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hops++
		if r.URL.Path == "/loop" {
			http.Redirect(w, r, "/loop", http.StatusFound)
			return
		}
		if r.URL.Path == "/plain" {
			http.Redirect(w, r, "http://example.invalid/x", http.StatusFound)
			return
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer ts.Close()
	test := DownloadOptions{AllowHTTP: true, AllowPrivate: true}
	if _, err := Download(context.Background(), ts.URL+"/loop", 100, test); err == nil || hops > maxRedirects+2 {
		t.Fatalf("a redirect loop must stop after %d hops; err %v, %d requests", maxRedirects, err, hops)
	}
	// With http not allowed the check refuses a redirect to http (the first URL is https here only
	// in production; the redirect rule is the same function).
	c := safeClient(DownloadOptions{})
	req, _ := http.NewRequest("GET", "http://example.invalid/x", nil)
	if err := c.CheckRedirect(req, nil); err == nil {
		t.Fatal("a redirect to http must be refused")
	}
	req, _ = http.NewRequest("GET", "https://example.invalid/x", nil)
	if err := c.CheckRedirect(req, nil); err != nil {
		t.Fatalf("a redirect to https is fine: %v", err)
	}
}
