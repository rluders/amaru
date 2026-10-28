package coordtest

import (
	"github.com/rluders/amaru/services/platform/internal/coordinator"
	"github.com/rluders/amaru/services/platform/internal/domain"
)

// Requester is the human who submits in the suites that are not about Policy.
var Requester = domain.Principal{Kind: domain.PrincipalHuman, ID: "tester", Role: domain.RoleContributor}

// Permissive allows every Capability, so a suite about something other than Policy is not
// stopped by it. The built-in deny of the reserved merge Capability still applies. Production
// code never uses it.
var Permissive = coordinator.WithRules(domain.PolicyRule{
	ID: "test.allow-all", Source: domain.SourceOrganization, Effect: domain.EffectAllow, Capabilities: []string{"*"},
})
