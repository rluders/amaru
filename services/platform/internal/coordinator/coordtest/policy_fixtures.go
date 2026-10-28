package coordtest

import (
	"context"
	"sync"

	"github.com/rluders/amaru/services/platform/internal/coordinator"
	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
)

// Requester is the human who submits in the suites that are not about Policy.
var Requester = domain.Principal{Kind: domain.PrincipalHuman, ID: "tester", Role: domain.RoleContributor}

// Permissive allows every Capability and trusts every Project, so a suite about something other
// than Policy is not stopped by it. The built-in deny of the reserved merge Capability still
// applies. A pinned Workflow definition is trusted only when the suite says so (OpenTrust.Workflow).
// Production code never uses it.
var Permissive = func() coordinator.Option {
	rules := coordinator.WithRules(domain.PolicyRule{
		ID: "test.allow-all", Source: domain.SourceOrganization, Effect: domain.EffectAllow, Capabilities: []string{"*"},
	})
	return func(c *coordinator.Coordinator) {
		rules(c)
		NewOpenTrust().Option()(c)
	}
}()

// OpenTrust is a ports.TrustStore that trusts every Project, for suites that are not about trust.
// It trusts a Workflow definition only after Workflow is called for it. It does not store Trust
// records or audit them. Production code never uses it.
type OpenTrust struct {
	ports.TrustStore
	mu        sync.Mutex
	workflows map[string]string
}

func NewOpenTrust() *OpenTrust { return &OpenTrust{workflows: map[string]string{}} }

// Option installs it on a Coordinator.
func (o *OpenTrust) Option() coordinator.Option { return coordinator.WithTrust(o) }

// Workflow trusts the definition of the Workflow with this hash.
func (o *OpenTrust) Workflow(id, hash string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.workflows[id] = hash
}

func (o *OpenTrust) Get(_ context.Context, key domain.ProjectKey, kind domain.TrustKind, ref string) (domain.Trust, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	switch {
	case kind == domain.TrustProject:
		return domain.Trust{ProjectKey: key, Kind: kind}, nil
	case kind == domain.TrustWorkflow && o.workflows[ref] != "":
		return domain.Trust{ProjectKey: key, Kind: kind, Ref: ref, Hash: o.workflows[ref]}, nil
	}
	return domain.Trust{}, domain.ErrNotFound
}
