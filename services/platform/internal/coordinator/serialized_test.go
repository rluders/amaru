package coordinator_test

import (
	"testing"

	"github.com/rluders/amaru/services/platform/internal/coordinator/coordtest"
	"github.com/rluders/amaru/services/platform/internal/ports"
	"github.com/rluders/amaru/services/platform/internal/worker/inproc"
)

func TestAssignmentOverSerializedLink(t *testing.T) {
	coordtest.AssignmentSuite(t, newFake, newSerializedLink)
}

func TestTrustOverSerializedLink(t *testing.T) {
	coordtest.TrustSuite(t, newTrustStores, newSerializedLink)
}

func TestSkillOverSerializedLink(t *testing.T) {
	coordtest.SkillSuite(t, newSkillStores, newSerializedLink)
}

func TestPolicyOverSerializedLink(t *testing.T) {
	coordtest.PolicySuite(t, newFakes, newSerializedLink)
}

func TestApprovalOverSerializedLink(t *testing.T) {
	coordtest.ApprovalSuite(t, newFake, newSerializedLink, nil)
}

func newSerializedLink(*testing.T) (ports.PlatformLink, ports.WorkerLink) {
	return coordtest.SerializedLink(inproc.Pair())
}

func TestCancelOverSerializedLink(t *testing.T) {
	coordtest.CancelSuite(t, newFake, newSerializedLink, nil)
}

func TestLimitsOverSerializedLink(t *testing.T) {
	coordtest.LimitsSuite(t, newFake, newSerializedLink)
}
