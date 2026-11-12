// Package operational emits operational diagnostics, separately from domain/audit Events.
package operational

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rluders/amaru/services/platform/internal/domain"
)

// Identity comes from trusted Instance state and the current command/Event context,
// never directly from HTTP headers, request targets or idempotency keys.
type Identity struct {
	InstanceID    string `json:"instanceId"`
	CorrelationID string `json:"correlationId"`
	ProjectID     string `json:"projectId,omitempty"`
	ExecutionID   string `json:"executionId,omitempty"`
	StepID        string `json:"stepId,omitempty"`
}
type identityKey struct{}

func WithIdentity(ctx context.Context, id Identity) context.Context {
	return context.WithValue(ctx, identityKey{}, id)
}
func IdentityFrom(ctx context.Context) Identity {
	id, _ := ctx.Value(identityKey{}).(Identity)
	return id
}

// Record deliberately has no arbitrary attributes, request or Provider output field.
// Operation/Outcome are static application labels. Cause is an optional diagnostic,
// redacted before encoding; never pass auth errors or raw requests here.
type Record struct {
	Operation string `json:"operation"`
	Outcome   string `json:"outcome"`
	Cause     string `json:"cause,omitempty"`
	Status    int    `json:"status,omitempty"`
}

type Logger struct {
	mu       sync.Mutex
	out      io.Writer
	patterns domain.Redactor
	values   *strings.Replacer
}

// New snapshots known secret values (including injected Provider credentials).
// Construct a new logger when that set changes. Auth request material is omitted
// by HTTP, so ephemeral launch/session/CSRF values need no registration there.
func New(out io.Writer, secrets ...string) (*Logger, error) {
	if out == nil {
		return nil, errors.New("operational logger requires a writer")
	}
	values := append([]string(nil), secrets...)
	sort.Slice(values, func(i, j int) bool { return len(values[i]) > len(values[j]) })
	pairs := make([]string, 0, len(values)*2)
	for _, value := range values {
		if value == "" {
			return nil, errors.New("operational logger refuses empty secret values")
		}
		pairs = append(pairs, value, domain.RedactionMarker)
	}
	return &Logger{out: out, patterns: domain.DefaultRedactor(), values: strings.NewReplacer(pairs...)}, nil
}

// Write emits one JSON line under a lock. Redact before JSON escaping, including
// identity fields; return only safe errors, never a sink's credential-bearing error.
// A log failure must not retry or roll back an already committed command.
func (l *Logger) Write(ctx context.Context, rec Record) error {
	id := IdentityFrom(ctx)
	if id.InstanceID == "" || id.CorrelationID == "" || rec.Operation == "" || rec.Outcome == "" {
		return errors.New("operational log requires identity, operation and outcome")
	}
	clean := func(s string) string {
		s = l.values.Replace(s)
		s, _ = l.patterns.Redact(s)
		return s
	}
	id.InstanceID = clean(id.InstanceID)
	id.CorrelationID = clean(id.CorrelationID)
	id.ProjectID = clean(id.ProjectID)
	id.ExecutionID = clean(id.ExecutionID)
	id.StepID = clean(id.StepID)
	rec.Operation = clean(rec.Operation)
	rec.Outcome = clean(rec.Outcome)
	rec.Cause = clean(rec.Cause)
	line, err := json.Marshal(struct {
		Time time.Time `json:"time"`
		Identity
		Record
	}{time.Now().UTC(), id, rec})
	if err != nil {
		return errors.New("operational log encoding failed")
	}
	line = append(line, '\n')
	l.mu.Lock()
	defer l.mu.Unlock()
	n, err := l.out.Write(line)
	if err != nil || n != len(line) {
		return errors.New("operational log write failed")
	}
	return nil
}
