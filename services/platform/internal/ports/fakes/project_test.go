package fakes

import (
	"testing"

	"github.com/rluders/amaru/services/platform/internal/ports"
	"github.com/rluders/amaru/services/platform/internal/ports/porttest"
)

func TestProjectStoreConformance(t *testing.T) {
	porttest.ProjectStore(t, func(*testing.T) ports.ProjectStore { return NewProjectStore() })
}
