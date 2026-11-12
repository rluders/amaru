package fakes

import (
	"testing"

	"github.com/rluders/amaru/services/platform/internal/ports"
	"github.com/rluders/amaru/services/platform/internal/ports/porttest"
)

func TestEventStream(t *testing.T) {
	porttest.EventStream(t, func(*testing.T) (ports.ExecutionStore, ports.EventStream) {
		s := NewExecutionStore()
		return s, s
	})
}
