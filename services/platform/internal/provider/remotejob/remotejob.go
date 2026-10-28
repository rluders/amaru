// Package remotejob holds the small pieces an HTTP Provider for an asynchronous remote service
// needs (FEAT-CAP-004, ADR-0054): JSON requests with typed HTTP errors, retry that honors
// Retry-After and never repeats a call the service may already have accepted, and bounded
// downloads of signed result URLs. It is a helper, not a framework: a Provider owns its flow.
//
// What makes a remote job survive a restart is not here. A Provider records the job with the
// Worker (embedded.ReportRemoteJob) and re-attaches to Assignment.RemoteJob; see
// docs/architecture/DOMAIN_MODEL.md.
//
// Adapted from the earlier internal implementation's remotejob package at commit c8a2f6e
// (ADR-0032); see PROVENANCE.md.
package remotejob

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Defaults of a Client; every one can be set on the Client.
const (
	DefaultTimeout     = 60 * time.Second
	DefaultMaxResponse = 16 << 20
	maxErrorBody       = 500
)

// HTTPError is a non-2xx response. Body is truncated and is the remote service's answer, never
// our request; it must still not be logged next to credentials.
type HTTPError struct {
	Status     int
	Body       string
	RetryAfter time.Duration
}

func (e *HTTPError) Error() string {
	if e.Body != "" {
		return fmt.Sprintf("HTTP %d: %s", e.Status, e.Body)
	}
	return fmt.Sprintf("HTTP %d", e.Status)
}

// Retryable reports whether the request may be repeated at all (rate limit or server trouble).
// Whether it is safe to repeat is Retry's decision, which depends on the call.
func (e *HTTPError) Retryable() bool {
	return e.Status == http.StatusTooManyRequests || e.Status >= 500
}

// NotAccepted reports whether the service certainly did not act on the request: a rate limit
// answers before it does. A 5xx may have followed an accepted request.
func (e *HTTPError) NotAccepted() bool { return e.Status == http.StatusTooManyRequests }

// Client sends authenticated JSON requests to one base URL.
type Client struct {
	BaseURL string
	// HTTP is the underlying client; nil means one with Timeout.
	HTTP *http.Client
	// Timeout bounds a request when HTTP is nil; zero means DefaultTimeout.
	Timeout time.Duration
	// MaxResponse bounds a JSON or raw response body; zero means DefaultMaxResponse. A longer
	// body is an error, never truncated.
	MaxResponse int64
	// Header sets authentication on API requests only, never on downloads.
	Header func(h http.Header)
}

func (c *Client) http() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	d := c.Timeout
	if d <= 0 {
		d = DefaultTimeout
	}
	return &http.Client{Timeout: d}
}

func (c *Client) maxResponse() int64 {
	if c.MaxResponse > 0 {
		return c.MaxResponse
	}
	return DefaultMaxResponse
}

func (c *Client) url(path string) string { return strings.TrimRight(c.BaseURL, "/") + path }

// Do sends method path with body (nil means none) and decodes a JSON response into out (nil means
// ignore it).
func (c *Client) Do(ctx context.Context, method, path string, body, out any) error {
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.url(path), rd)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	return c.send(req, out)
}

// send runs the request and decodes a 2xx JSON answer into out.
func (c *Client) send(req *http.Request, out any) error {
	if c.Header != nil {
		c.Header(req.Header)
	}
	resp, err := c.http().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return httpError(resp)
	}
	if out == nil {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
		return nil
	}
	data, err := readBounded(resp.Body, c.maxResponse())
	if err != nil {
		return err
	}
	return json.Unmarshal(data, out)
}

// DoRaw sends a JSON body and returns the response body (for services that answer with the file
// itself). A body over limit bytes is an error, never truncated.
func (c *Client) DoRaw(ctx context.Context, method, path string, body any, limit int64) ([]byte, http.Header, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return nil, nil, err
	}
	req, err := http.NewRequestWithContext(ctx, method, c.url(path), bytes.NewReader(b))
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.Header != nil {
		c.Header(req.Header)
	}
	resp, err := c.http().Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, nil, httpError(resp)
	}
	data, err := readBounded(resp.Body, limit)
	return data, resp.Header, err
}

// Upload sends one file as multipart/form-data under field and decodes the JSON answer into out
// (nil means ignore it). fields are plain form fields sent before the file.
func (c *Client) Upload(ctx context.Context, path, field, filename, contentType string, data []byte, fields map[string]string, out any) error {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	names := make([]string, 0, len(fields))
	for k := range fields {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, k := range names {
		if err := mw.WriteField(k, fields[k]); err != nil {
			return err
		}
	}
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name=%q; filename=%q`, field, filename))
	h.Set("Content-Type", contentType)
	part, err := mw.CreatePart(h)
	if err != nil {
		return err
	}
	if _, err := part.Write(data); err != nil {
		return err
	}
	if err := mw.Close(); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url(path), &buf)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Accept", "application/json")
	return c.send(req, out)
}

func httpError(resp *http.Response) *HTTPError {
	b, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBody))
	return &HTTPError{Status: resp.StatusCode, Body: strings.TrimSpace(string(b)), RetryAfter: RetryAfter(resp.Header)}
}

// readBounded reads at most limit bytes and fails with ErrTooLarge, rather than truncates, on a
// longer body.
func readBounded(r io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("%w: more than %d bytes", ErrTooLarge, limit)
	}
	return data, nil
}

// RetryAfter parses a Retry-After header given in seconds (zero when absent or a date).
func RetryAfter(h http.Header) time.Duration {
	if s, err := strconv.Atoi(strings.TrimSpace(h.Get("Retry-After"))); err == nil && s > 0 {
		return time.Duration(s) * time.Second
	}
	return 0
}

// Sleep waits d or until ctx ends.
func Sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Retry calls f until it succeeds, fails with an error it must not repeat, or attempts run out.
// A retried call waits the service's Retry-After, else an exponential backoff from base.
//
// idempotent says that repeating the call is safe, because the service deduplicates it (for
// example by the Step's idempotency key). Only then is a 5xx retried: the service may have
// accepted the request before it failed, and repeating a submit that is not idempotent would
// create a second paid job. A rate limit (429) is always retried, because the service answered
// before it acted. This is a retry inside one attempt of a Step; retrying a Step is FEAT-WFL-005.
func Retry(ctx context.Context, attempts int, base time.Duration, idempotent bool, f func() error) error {
	wait := base
	for i := 1; ; i++ {
		err := f()
		var he *HTTPError
		if err == nil || !errors.As(err, &he) || !he.Retryable() || (!idempotent && !he.NotAccepted()) || i >= attempts {
			return err
		}
		d := he.RetryAfter
		if d == 0 {
			d = wait
			wait *= 2
		}
		if err := Sleep(ctx, d); err != nil {
			return err
		}
	}
}
