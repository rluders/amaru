package coordinator_test

import (
	"testing"

	"github.com/rluders/amaru/services/platform/internal/coordinator/coordtest"
	"github.com/rluders/amaru/services/platform/internal/ports"
	"github.com/rluders/amaru/services/platform/internal/ports/fakes"
)

// The in-memory store keeps its data across a "restart", the way a database does.
func TestCrashInjectionWithFake(t *testing.T) {
	coordtest.CrashInjection(t,
		func(*testing.T) ports.ExecutionStore { return fakes.NewExecutionStore() },
		func(_ *testing.T, s ports.ExecutionStore) ports.ExecutionStore { return s })
}
