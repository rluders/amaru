package coordinator_test

import (
	"testing"

	"github.com/rluders/amaru/services/platform/internal/coordinator/coordtest"
	"github.com/rluders/amaru/services/platform/internal/ports"
	"github.com/rluders/amaru/services/platform/internal/ports/fakes"
	"github.com/rluders/amaru/services/platform/internal/worker/inproc"
)

func newFake(*testing.T) ports.ExecutionStore { return fakes.NewExecutionStore() }

func TestAssignmentOverInProcessLink(t *testing.T) {
	coordtest.AssignmentSuite(t, newFake, func(*testing.T) (ports.PlatformLink, ports.WorkerLink) { return inproc.Pair() })
}

func TestCommandsIdempotent(t *testing.T) { coordtest.CommandSuite(t, newFake, nil) }

func TestLimitsOverInProcessLink(t *testing.T) {
	coordtest.LimitsSuite(t, newFake, func(*testing.T) (ports.PlatformLink, ports.WorkerLink) { return inproc.Pair() })
}

func TestDefinitionPinned(t *testing.T) { coordtest.DefinitionSuite(t, newFake, nil) }

func TestTimeoutsOverInProcessLink(t *testing.T) {
	coordtest.TimeoutSuite(t, newFake, func(*testing.T) (ports.PlatformLink, ports.WorkerLink) { return inproc.Pair() }, nil)
}

func TestCancelOverInProcessLink(t *testing.T) {
	coordtest.CancelSuite(t, newFake, func(*testing.T) (ports.PlatformLink, ports.WorkerLink) { return inproc.Pair() }, nil)
}

func TestRemoteJobOverInProcessLink(t *testing.T) {
	coordtest.RemoteJobSuite(t, newFake, func(*testing.T) (ports.PlatformLink, ports.WorkerLink) { return inproc.Pair() }, nil)
}

func newFakes(*testing.T) (ports.ExecutionStore, ports.GrantStore) {
	e, g := fakes.NewExecutionStore(), fakes.NewGrantStore()
	fakes.LinkAudit(e, g)
	return e, g
}

func TestApprovalOverInProcessLink(t *testing.T) {
	coordtest.ApprovalSuite(t, newFake, func(*testing.T) (ports.PlatformLink, ports.WorkerLink) { return inproc.Pair() }, nil)
}

func TestPolicyOverInProcessLink(t *testing.T) {
	coordtest.PolicySuite(t, newFakes, func(*testing.T) (ports.PlatformLink, ports.WorkerLink) { return inproc.Pair() })
}
