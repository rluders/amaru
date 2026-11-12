package fakes

import (
	"testing"

	"github.com/rluders/amaru/services/platform/internal/ports"
	"github.com/rluders/amaru/services/platform/internal/ports/porttest"
)

func TestSettingsStore(t *testing.T) {
	porttest.SettingsStore(t, func(*testing.T) (ports.SettingsStore, ports.ProjectStore) { s := NewProjectStore(); return s, s }, func(s ports.SettingsStore) ports.SettingsStore { return s })
}
