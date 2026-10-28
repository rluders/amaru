package domain

import (
	"fmt"
	"slices"
	"time"
)

// ExecState is the lifecycle state of an Execution (DOMAIN_MODEL §5.1).
type ExecState string

const (
	ExecQueued    ExecState = "queued"
	ExecRunning   ExecState = "running"
	ExecWaiting   ExecState = "waiting" // a Step awaits an Approval or a Worker
	ExecSucceeded ExecState = "succeeded"
	ExecFailed    ExecState = "failed"
	ExecCancelled ExecState = "cancelled"
)

// ExecStates lists every Execution state, in lifecycle order.
var ExecStates = []ExecState{ExecQueued, ExecRunning, ExecWaiting, ExecSucceeded, ExecFailed, ExecCancelled}

// execTransitions is the complete set of documented transitions. Anything absent is invalid.
var execTransitions = map[ExecState][]ExecState{
	ExecQueued:  {ExecRunning, ExecCancelled},
	ExecRunning: {ExecWaiting, ExecSucceeded, ExecFailed, ExecCancelled},
	ExecWaiting: {ExecRunning, ExecFailed, ExecCancelled},
}

func (s ExecState) Valid() bool { return slices.Contains(ExecStates, s) }

// Terminal reports whether the state is final: no transition leaves it.
func (s ExecState) Terminal() bool { return s.Valid() && len(execTransitions[s]) == 0 }

// CanTransitionTo reports whether the lifecycle documents the move s → to.
func (s ExecState) CanTransitionTo(to ExecState) bool { return slices.Contains(execTransitions[s], to) }

// Execution is one run of a Workflow. Only its lifecycle is modelled here.
type Execution struct {
	ID string `json:"id"`
	// Version is the optimistic-concurrency counter of the stored row, set by the store.
	Version int64 `json:"version"`
	// ProjectID and EnvironmentID are the scopes the concurrency limits count in (FEAT-EXE-006).
	// They are opaque ids set when the Execution is created and never change; empty means the
	// Execution is not in that scope and its Steps do not count against it.
	ProjectID     string    `json:"projectId,omitempty"`
	EnvironmentID string    `json:"environmentId,omitempty"`
	State         ExecState `json:"state"`
	// RequestedBy is who submitted the Execution and Trigger how it was started; Policy is
	// evaluated for them before each Step starts. Both are set when the Execution is created
	// and never change.
	RequestedBy Principal `json:"requestedBy"`
	Trigger     string    `json:"trigger,omitempty"`
	// Definition is the Workflow definition the Execution runs, pinned when it is created and
	// never changed (FEAT-WFL-002). Nil for an Execution submitted without a Workflow.
	Definition *DefinitionPin `json:"definition,omitempty"`
	CreatedAt  time.Time      `json:"createdAt"`
	StartedAt  *time.Time     `json:"startedAt,omitempty"`
	EndedAt    *time.Time     `json:"endedAt,omitempty"`
	Error      *ExecError     `json:"error,omitempty"`
}

// move changes state only from the one state the calling method is defined for, and only along
// a documented edge.
func (e *Execution) move(from, to ExecState) error {
	if e.State != from || !e.State.CanTransitionTo(to) {
		return fmt.Errorf("%w: execution %s -> %s", ErrInvalidTransition, e.State, to)
	}
	e.State = to
	return nil
}

// Start moves queued → running when the first Step starts.
func (e *Execution) Start(now time.Time) error {
	if err := e.move(ExecQueued, ExecRunning); err != nil {
		return err
	}
	e.StartedAt = &now
	return nil
}

// Wait moves running → waiting: a Step awaits an Approval, a Worker, or a person to resume or
// cancel an interrupted or timed-out Step (ADR-0042, ADR-0044).
func (e *Execution) Wait() error { return e.move(ExecRunning, ExecWaiting) }

// Resume moves waiting → running: the Approval was given or a Worker leased the Step.
func (e *Execution) Resume() error { return e.move(ExecWaiting, ExecRunning) }

// Finish moves the Execution to a terminal state. Failed requires err. Which terminal states
// are reachable depends on the current state (waiting cannot succeed; queued can only be
// cancelled).
func (e *Execution) Finish(now time.Time, to ExecState, err *ExecError) error {
	if !to.Terminal() {
		return fmt.Errorf("%w: execution %s -> %s is not terminal", ErrInvalidTransition, e.State, to)
	}
	if to == ExecFailed && err == nil {
		return ErrErrorRequired
	}
	if !e.State.CanTransitionTo(to) {
		return fmt.Errorf("%w: execution %s -> %s", ErrInvalidTransition, e.State, to)
	}
	e.State = to
	e.EndedAt = &now
	if to == ExecFailed {
		e.Error = err
	}
	return nil
}
