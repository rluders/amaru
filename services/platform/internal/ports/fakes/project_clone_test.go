package fakes

import (
	"github.com/rluders/amaru/services/platform/internal/ports"
	"github.com/rluders/amaru/services/platform/internal/ports/porttest"
	"testing"
)

func TestProjectCloneStore(t *testing.T) {
	porttest.ProjectCloneStore(t, func(*testing.T) ports.ProjectCloneStore { return NewProjectStore() }, func(s ports.ProjectCloneStore) ports.ProjectCloneStore { return s })
}
