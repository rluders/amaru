package egress

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"time"

	"github.com/rluders/amaru/services/platform/internal/domain"
)

// Sender sends one consented request through the address Guard. Authorize must check live
// Platform Policy and durably record its decision; nil fails closed. A configured URL is not consent.
// Use Sender as remotejob.Client.HTTP and DownloadOptions.Client, including for signed URLs.
type Sender struct {
	Guard     Guard
	Authorize func(context.Context, domain.EgressTarget) error
	Timeout   time.Duration
}

// Do checks consent before DNS or body reads, follows no redirects and returns no raw URL or
// transport error (URLs and transport errors may contain credentials). The caller closes the body.
func (s Sender) Do(req *http.Request) (*http.Response, error) {
	// Match http.Client's ownership of the request body, including refusals before transport.
	handedToTransport := false
	defer func() {
		if !handedToTransport && req != nil && req.Body != nil {
			req.Body.Close()
		}
	}()
	if req == nil || req.URL == nil || s.Authorize == nil {
		return nil, errors.New("egress: consent checker is required")
	}
	target, err := domain.ParseEgressTarget(req.URL.String())
	if err != nil || (req.Host != "" && req.Host != req.URL.Host) {
		return nil, errors.New("egress: invalid destination")
	}
	if err := s.Authorize(req.Context(), target); err != nil {
		return nil, errors.New("egress: consent refused or unavailable")
	}
	// Send exactly the normalized endpoint Policy saw, retaining only the query on the wire.
	clone := req.Clone(req.Context())
	clone.URL, _ = url.Parse(target.Key())
	clone.URL.RawQuery = req.URL.RawQuery
	clone.URL.ForceQuery = req.URL.ForceQuery
	clone.Host = ""
	timeout := s.Timeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	handedToTransport = true
	resp, err := s.Guard.Client(timeout).Do(clone)
	if err != nil {
		if errors.Is(err, ErrBlocked) {
			return nil, ErrBlocked
		}
		return nil, errors.New("egress: request failed")
	}
	return resp, nil
}
