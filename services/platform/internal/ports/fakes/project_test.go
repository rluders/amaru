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

func TestFakeBlobStore(t *testing.T) {
	porttest.BlobStore(t, func(*testing.T) ports.BlobStore { return NewBlobStore() })
}

func TestFakeArtifactIndex(t *testing.T) {
	porttest.ArtifactIndex(t, func(*testing.T) ports.ArtifactIndex { return NewArtifactIndex() })
}

func TestFakeSkillStore(t *testing.T) {
	porttest.SkillStore(t, func(*testing.T) ports.SkillStore { return NewSkillStore() })
}

func TestFakeContextStore(t *testing.T) {
	porttest.ContextStore(t, func(*testing.T) (ports.ContextStore, func(string, string)) {
		return NewContextStore(), func(string, string) {}
	})
}
