package fakes

import (
	"testing"

	"github.com/rluders/amaru/services/platform/internal/ports"
	"github.com/rluders/amaru/services/platform/internal/ports/porttest"
)

func TestGrantStoreConformance(t *testing.T) {
	porttest.GrantStore(t, func(*testing.T) ports.GrantStore { return NewGrantStore() })
}
