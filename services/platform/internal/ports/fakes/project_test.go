package fakes

import (
	"testing"

	"github.com/rluders/amaru/services/platform/internal/ports"
	"github.com/rluders/amaru/services/platform/internal/ports/porttest"
)

func TestProjectStoreConformance(t *testing.T) {
	porttest.ProjectStore(t, func(*testing.T) ports.ProjectStore { return NewProjectStore() })
}

func TestExecutionStoreConformance(t *testing.T) {
	porttest.ExecutionStore(t, func(*testing.T) ports.ExecutionStore { return NewExecutionStore() }, nil)
}

func TestFakeApprovalStore(t *testing.T) {
	porttest.ApprovalStore(t, func(*testing.T) ports.ExecutionStore { return NewExecutionStore() }, nil)
}

func TestFakeTrustStore(t *testing.T) {
	porttest.TrustStore(t, func(*testing.T) (ports.TrustStore, ports.ProjectStore) {
		p := NewProjectStore()
		return NewTrustStore(p), p
	})
}

func TestFakeSkillStore(t *testing.T) {
	porttest.SkillStore(t, func(*testing.T) ports.SkillStore { return NewSkillStore() })
}
