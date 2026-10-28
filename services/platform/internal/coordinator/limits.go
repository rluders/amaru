package coordinator

import (
	"context"
	"errors"
	"fmt"

	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
)

// Limits caps how many Steps hold a slot, that is are leased or running, at once (FEAT-EXE-006).
// The Platform checks them before it leases a Step, so an assignment is never published for a
// Step that would exceed a limit. A zero field means no limit in that scope.
//
// Executions without a ProjectID or EnvironmentID are not in that scope and do not count against
// its limit. Today every Step counts as a slot; once agent Steps exist (a Step with an Agent
// snapshot) the limits are meant to count those only. Per-Project overrides of PerProject are
// not implemented.
type Limits struct {
	Global         int // across the Platform
	PerProject     int // per ProjectID
	PerEnvironment int // per EnvironmentID
}

// DefaultLimits is what a Coordinator uses unless WithLimits says otherwise: two slots per
// Project, nothing else capped.
var DefaultLimits = Limits{PerProject: 2}

// WithLimits replaces the limits. WithLimits(Limits{}) removes them all.
func WithLimits(l Limits) Option { return func(c *Coordinator) { c.limits = l } }

// queueScan is how many queued Steps one pass looks at. Steps blocked by a limit stay queued, so it
// is larger than what a Worker can take; a queue where more than this many Steps in a row are
// blocked would hide the ones behind them until slots free.
const queueScan = 256

type usage struct {
	global  int
	project map[string]int
	env     map[string]int
}

func countUsage(active []ports.StepRef) usage {
	u := usage{project: map[string]int{}, env: map[string]int{}}
	for _, r := range active {
		u.global++
		if r.ProjectID != "" {
			u.project[r.ProjectID]++
		}
		if r.EnvironmentID != "" {
			u.env[r.EnvironmentID]++
		}
	}
	return u
}

// blocked returns the reason a queued Step cannot take a slot now, or "" when it can.
func (l Limits) blocked(u usage, r ports.StepRef) string {
	switch {
	case l.Global > 0 && u.global >= l.Global:
		return domain.ReasonLimitGlobal
	case l.PerProject > 0 && r.ProjectID != "" && u.project[r.ProjectID] >= l.PerProject:
		return domain.ReasonLimitProject
	case l.PerEnvironment > 0 && r.EnvironmentID != "" && u.env[r.EnvironmentID] >= l.PerEnvironment:
		return domain.ReasonLimitEnvironment
	}
	return ""
}

// wait records on a queued Step why it is not leased, once per reason: the Step stays queued and
// shows the reason, and an event says when it began to wait. A Step that moved on meanwhile is
// left alone.
func (c *Coordinator) wait(ctx context.Context, ref ports.StepRef, reason string) error {
	err := c.edit(ctx, ports.StepRef{ExecutionID: ref.ExecutionID, StepID: ref.StepID}, func(_ *domain.Execution, st *domain.StepExecution) (*change, error) {
		changed, err := st.Defer(reason)
		if err != nil || !changed {
			return nil, err
		}
		return &change{events: []domain.ExecutionEvent{{Kind: domain.EventStepDeferred, StepID: st.ID, Reason: reason, At: c.now()}}}, nil
	})
	if errors.Is(err, domain.ErrInvalidTransition) || errors.Is(err, domain.ErrConflict) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("record why step %s waits: %w", ref.StepID, err)
	}
	return nil
}
