package operational

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/rluders/amaru/services/platform/internal/coordinator"
	"github.com/rluders/amaru/services/platform/internal/coordinator/coordtest"
	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
	"github.com/rluders/amaru/services/platform/internal/ports/fakes"
	"github.com/rluders/amaru/services/platform/internal/worker/embedded"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

func records(t *testing.T, b *bytes.Buffer) []map[string]any {
	t.Helper()
	var out []map[string]any
	dec := json.NewDecoder(bytes.NewReader(b.Bytes()))
	for {
		var rec map[string]any
		err := dec.Decode(&rec)
		if err == io.EOF {
			return out
		}
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, rec)
	}
}
func TestEveryFieldRedactedBeforeEncoding(t *testing.T) {
	var buf bytes.Buffer
	secrets := []string{"opaque-secret", "opaque-secret-long", "quote\"line\nsecret", "xy"}
	l, err := New(&buf, secrets...)
	if err != nil {
		t.Fatal(err)
	}
	gh := "ghp_" + strings.Repeat("A", 36)
	ctx := WithIdentity(context.Background(), Identity{InstanceID: secrets[0], CorrelationID: secrets[1], ProjectID: secrets[2], ExecutionID: gh, StepID: secrets[3]})
	if err := l.Write(ctx, Record{Operation: secrets[0], Outcome: secrets[1], Cause: "Provider unavailable: " + secrets[2] + " " + gh}); err != nil {
		t.Fatal(err)
	}
	for _, rec := range records(t, &buf) {
		for _, value := range rec {
			s, _ := value.(string)
			for _, secret := range append(secrets, gh) {
				if strings.Contains(s, secret) {
					t.Fatal("secret present in decoded record")
				}
			}
		}
		if !strings.Contains(rec["cause"].(string), "Provider unavailable") {
			t.Fatal("lost diagnostic cause")
		}
	}
}

type badWriter struct{ short bool }

func (b badWriter) Write(p []byte) (int, error) {
	if b.short {
		return len(p) - 1, nil
	}
	return 0, errors.New("credential-bearing sink error")
}
func TestWriteFailuresAndMissingContext(t *testing.T) {
	for _, w := range []io.Writer{badWriter{}, badWriter{short: true}} {
		l, _ := New(w)
		ctx := WithIdentity(context.Background(), Identity{InstanceID: "i", CorrelationID: "c"})
		err := l.Write(ctx, Record{Operation: "command", Outcome: "failed"})
		if err == nil || err.Error() != "operational log write failed" {
			t.Fatal("unsafe or missing sink error")
		}
	}
	var buf bytes.Buffer
	l, _ := New(&buf)
	if l.Write(context.Background(), Record{Operation: "command", Outcome: "failed"}) == nil || buf.Len() != 0 {
		t.Fatal("missing context emitted")
	}
	if _, err := New(&buf, ""); err == nil {
		t.Fatal("empty secret accepted")
	}
	if _, err := New(nil); err == nil {
		t.Fatal("nil writer accepted")
	}
}
func TestConcurrentRecordsStayWhole(t *testing.T) {
	var buf bytes.Buffer
	l, _ := New(&buf)
	ctx := WithIdentity(context.Background(), Identity{InstanceID: "i", CorrelationID: "c"})
	var wg sync.WaitGroup
	for range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := l.Write(ctx, Record{Operation: "command", Outcome: "succeeded"}); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if len(records(t, &buf)) != 50 {
		t.Fatal("lost records")
	}
}

// Composition fixture: real commands, fake store/Provider; no product HTTP routes.
func TestCommandsProviderFailureAndCancellation(t *testing.T) {
	var buf bytes.Buffer
	secret := "injected-provider-value"
	l, _ := New(&buf, secret)
	store := fakes.NewExecutionStore()
	c := coordinator.New(store, time.Now, coordtest.Permissive)
	ctx := WithIdentity(context.Background(), Identity{InstanceID: "instance", CorrelationID: "command-correlation", ProjectID: "project"})
	got, _, err := c.Submit(ctx, coordinator.SubmitRequest{Key: "submit", ProjectID: "project", By: domain.Principal{ID: "human", Kind: domain.PrincipalHuman}, Steps: []coordinator.StepSpec{{Name: "check", Capability: "code.test"}}})
	if err != nil {
		t.Fatal(err)
	}
	id := IdentityFrom(ctx)
	id.ExecutionID = got.Execution.ID
	ctx = WithIdentity(ctx, id)
	if err := l.Write(ctx, Record{Operation: coordinator.CmdSubmit, Outcome: "succeeded"}); err != nil {
		t.Fatal(err)
	}
	runner := embedded.RunnerFunc(func(context.Context, ports.Assignment) embedded.Outcome {
		return embedded.Outcome{Result: ports.OutcomeFailed, ErrorCode: "unavailable", ErrorDetail: "Provider unavailable: " + secret}
	})
	assignment := ports.Assignment{ExecutionID: got.Execution.ID, StepID: got.Steps[0].ID, AttemptID: "attempt"}
	out := runner.Run(ctx, assignment)
	stepID := id
	stepID.StepID = assignment.StepID
	if err := l.Write(WithIdentity(ctx, stepID), Record{Operation: "step.result", Outcome: "failed", Cause: out.ErrorCode + ": " + out.ErrorDetail}); err != nil {
		t.Fatal(err)
	}
	_, _, err = c.Cancel(ctx, coordinator.CancelRequest{Key: "cancel", ExecutionID: got.Execution.ID, ActorID: "human", By: domain.Actor{Kind: domain.ActorHuman}, Reason: secret})
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Write(ctx, Record{Operation: coordinator.CmdCancel, Outcome: "succeeded"}); err != nil {
		t.Fatal(err)
	}
	// A rejected command keeps its useful, redacted cause without adding state.
	_, _, err = c.Cancel(ctx, coordinator.CancelRequest{Key: "invalid", ExecutionID: got.Execution.ID, ActorID: "human", By: domain.Actor{Kind: domain.ActorHuman}, Reason: secret})
	if err == nil {
		t.Fatal("ended execution accepted cancellation")
	}
	if err := l.Write(ctx, Record{Operation: coordinator.CmdCancel, Outcome: "rejected", Cause: err.Error()}); err != nil {
		t.Fatal(err)
	}

	e, _, err := store.Get(ctx, got.Execution.ID)
	if err != nil || e.State != domain.ExecCancelled {
		t.Fatal("cancel did not commit")
	}
	recs := records(t, &buf)
	if len(recs) != 4 {
		t.Fatal("missing records")
	}
	for _, r := range recs {
		if r["instanceId"] != "instance" || r["projectId"] != "project" || r["executionId"] != got.Execution.ID || r["correlationId"] != "command-correlation" {
			t.Fatal("lost identity")
		}
	}
	if recs[1]["stepId"] != assignment.StepID || !strings.Contains(recs[1]["cause"].(string), "unavailable") {
		t.Fatal("lost failure context")
	}
	if strings.Contains(buf.String(), secret) {
		t.Fatal("secret emitted")
	}
}

// A sink is an adapter boundary; its panic value may contain credentials.
type panicWriter struct{}

func (panicWriter) Write([]byte) (int, error) { panic("credential-bearing sink panic") }

func TestSinkPanicReturnsSafeError(t *testing.T) {
	l, _ := New(panicWriter{})
	ctx := WithIdentity(context.Background(), Identity{InstanceID: "i", CorrelationID: "c"})
	for range 2 {
		if err := l.Write(ctx, Record{Operation: "command", Outcome: "failed"}); err == nil || err.Error() != "operational log write failed" {
			t.Fatal("unsafe or missing sink error")
		}
	}
}
