package coordtest

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rluders/amaru/services/platform/internal/coordinator"
	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/egress"
	"github.com/rluders/amaru/services/platform/internal/ports"
	"github.com/rluders/amaru/services/platform/internal/provider/remotejob"
)

// EgressSuite sends through the Provider, consent callback, durable decision log and real loopback
// HTTP server. Only trust standing is faked; both production and fake stores run these cases.
func EgressSuite(t *testing.T, factory func(*testing.T) (ports.ExecutionStore, ports.GrantStore)) {
	for _, mode := range []string{"no grant", "wrong URL", "allowed", "revoked", "expired grant", "expired lease", "wrong fence", "cancelled", "deny wins", "untrusted", "log unavailable"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			now := time.Date(2026, 11, 12, 15, 0, 0, 0, time.UTC)
			store, grants := factory(t)
			trust := NewOpenTrust()
			flow := domain.Workflow{ID: "send-it", Name: "Send", Status: domain.WorkflowActive, Steps: []domain.WorkflowStep{{ID: "send", Name: "Send", Type: domain.StepTypeTool, Capability: "code.test"}}}
			pin, err := domain.Pin(flow, "")
			if err != nil {
				t.Fatal(err)
			}
			trust.Workflow(pin.WorkflowID, pin.Hash)
			opts := []coordinator.Option{coordinator.WithGrants(grants), trust.Option()}
			c := coordinator.New(store, func() time.Time { return now }, opts...)
			out, _, err := c.Submit(ctx, coordinator.SubmitRequest{Key: "start", ProjectID: "project", By: Requester, Workflow: &flow})
			if err != nil {
				t.Fatal(err)
			}
			e, st := out.Execution, out.Steps[0]
			if err = st.Lease("worker", "attempt", now.Add(time.Minute)); err != nil {
				t.Fatal(err)
			}
			if err = st.Start(now); err != nil {
				t.Fatal(err)
			}
			if err = e.Start(now); err != nil {
				t.Fatal(err)
			}
			if _, err = store.Commit(ctx, ports.ExecutionChange{Execution: &e, Steps: []domain.StepExecution{st}}); err != nil {
				t.Fatal(err)
			}
			received := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				received++
				events, err := store.Events(ctx, e.ID)
				if err != nil {
					t.Error(err)
				}
				authorized := false
				for _, ev := range events {
					if ev.Kind == domain.EventEgressDecision && ev.Reason == string(domain.DecisionAllow) {
						authorized = true
					}
				}
				if !authorized {
					t.Error("request arrived before authorization event")
				}
				w.Write([]byte(`{}`))
			}))
			defer srv.Close()
			tg, err := domain.ParseEgressTarget(srv.URL + "/send")
			if err != nil {
				t.Fatal(err)
			}
			owner := domain.Principal{Kind: domain.PrincipalHuman, ID: "owner", Role: domain.RoleOwner}
			if mode != "no grant" {
				var expires *time.Time
				if mode == "expired grant" {
					at := now.Add(30 * time.Second)
					expires = &at
				}
				value := tg.Key()
				if mode == "wrong URL" {
					value += "-other"
				}
				_, err = c.CreateGrant(ctx, "consent", owner, domain.Grant{ID: "consent", ExpiresAt: expires, Level: domain.GrantProject, ProjectID: "project", Subject: domain.Subject{Kind: domain.SubjectWorkflow, ID: flow.ID}, Capabilities: []domain.Capability{domain.CapabilityNetEgress}, Resource: domain.Resource{Type: domain.ResourceURL, Value: value}, DefinitionHash: pin.Hash})
				if err != nil {
					t.Fatal(err)
				}
			}
			if mode == "revoked" {
				if _, err = c.RevokeGrant(ctx, "revoke", owner, "consent", "stop"); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "expired grant" {
				// Expiry is rechecked while the attempt itself is still live.
				now = now.Add(31 * time.Second)
			}
			if mode == "expired lease" {
				now = now.Add(time.Minute)
			}
			if mode == "cancelled" {
				e, _, err = store.Get(ctx, e.ID)
				if err != nil {
					t.Fatal(err)
				}
				e.State = domain.ExecCancelled
				if _, err = store.Commit(ctx, ports.ExecutionChange{Execution: &e}); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "deny wins" {
				opts = append(opts, coordinator.WithRules(domain.PolicyRule{ID: "deny-egress", Source: domain.SourceOrganization, Effect: domain.EffectDeny, Capabilities: []string{"net.egress"}}))
			}
			if mode == "untrusted" {
				opts = []coordinator.Option{coordinator.WithGrants(grants)}
			}
			if mode == "log unavailable" {
				store = failedEgressLog{store}
			}
			// New Coordinator proves consent is loaded from stored authority, not an in-memory cache.
			c = coordinator.New(store, func() time.Time { return now }, opts...)
			fence := st.FenceToken
			if mode == "wrong fence" {
				fence++
			}
			sender := egress.Sender{Guard: egress.Guard{Allow: domain.EgressAllowlist{PrivateHosts: []string{tg.Host}}}, Authorize: c.EgressConsent(ports.Assignment{AttemptID: st.AttemptID, Lease: ports.Lease{FencingToken: fence}})}
			client := remotejob.Client{BaseURL: srv.URL, HTTP: sender}
			err = client.Do(ctx, "POST", "/send?token=secret", map[string]string{"value": "secret"}, nil)
			allowed := mode == "allowed"
			if (err == nil) != allowed || (received == 1) != allowed {
				t.Fatalf("received=%d error=%v", received, err)
			}
			if err != nil && strings.Contains(err.Error(), "secret") {
				t.Fatal("secret exposed")
			}
			events, err := store.Events(ctx, e.ID)
			if err != nil {
				t.Fatal(err)
			}
			decisions := 0
			for _, ev := range events {
				if ev.Kind == domain.EventEgressDecision {
					decisions++
					if strings.Contains(ev.Reason, "secret") {
						t.Fatal("secret in event")
					}
				}
			}
			if mode != "expired lease" && mode != "wrong fence" && mode != "cancelled" && mode != "log unavailable" && decisions != 1 {
				t.Fatalf("decision events %d", decisions)
			}
		})
	}
}

type failedEgressLog struct{ ports.ExecutionStore }

func (s failedEgressLog) Commit(context.Context, ports.ExecutionChange) (ports.ExecutionChange, error) {
	return ports.ExecutionChange{}, errors.New("log unavailable")
}
