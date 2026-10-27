package domain

import (
	"errors"
	"testing"
	"time"
)

var t0 = time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)

// documentedExec restates DOMAIN_MODEL §5.1 independently of the implementation table.
var documentedExec = map[[2]ExecState]bool{
	{ExecQueued, ExecRunning}:    true,
	{ExecRunning, ExecWaiting}:   true,
	{ExecWaiting, ExecRunning}:   true,
	{ExecRunning, ExecSucceeded}: true,
	{ExecRunning, ExecFailed}:    true,
	{ExecWaiting, ExecFailed}:    true,
	{ExecQueued, ExecCancelled}:  true,
	{ExecRunning, ExecCancelled}: true,
	{ExecWaiting, ExecCancelled}: true,
}

func TestExecutionOnlyDocumentedTransitions(t *testing.T) {
	for _, from := range ExecStates {
		for _, to := range ExecStates {
			if got, want := from.CanTransitionTo(to), documentedExec[[2]ExecState{from, to}]; got != want {
				t.Errorf("%s -> %s: allowed=%v, documented=%v", from, to, got, want)
			}
		}
	}
	for _, s := range []ExecState{ExecSucceeded, ExecFailed, ExecCancelled} {
		if !s.Terminal() {
			t.Errorf("%s must be terminal", s)
		}
	}
	for _, s := range []ExecState{ExecQueued, ExecRunning, ExecWaiting} {
		if s.Terminal() {
			t.Errorf("%s must not be terminal", s)
		}
	}
	if ExecState("bogus").Valid() || ExecState("bogus").Terminal() {
		t.Error("unknown state must be neither valid nor terminal")
	}
}

func TestExecutionMethodsRejectEveryOtherState(t *testing.T) {
	methods := map[string]struct {
		from ExecState
		call func(*Execution) error
	}{
		"Start":  {ExecQueued, func(e *Execution) error { return e.Start(t0) }},
		"Wait":   {ExecRunning, func(e *Execution) error { return e.Wait() }},
		"Resume": {ExecWaiting, func(e *Execution) error { return e.Resume() }},
	}
	for name, m := range methods {
		for _, s := range ExecStates {
			e := Execution{State: s}
			err := m.call(&e)
			if s == m.from {
				if err != nil {
					t.Errorf("%s from %s: %v", name, s, err)
				}
				continue
			}
			if !errors.Is(err, ErrInvalidTransition) || e.State != s {
				t.Errorf("%s from %s: want ErrInvalidTransition and no change, got %v (state %s)", name, s, err, e.State)
			}
		}
	}
}

func TestExecutionFinish(t *testing.T) {
	boom := &ExecError{Code: "x", Message: "y"}
	ok := []struct {
		from, to ExecState
	}{
		{ExecQueued, ExecCancelled}, {ExecRunning, ExecSucceeded}, {ExecRunning, ExecFailed}, {ExecRunning, ExecCancelled},
		{ExecWaiting, ExecFailed}, {ExecWaiting, ExecCancelled},
	}
	for _, c := range ok {
		e := Execution{State: c.from}
		if err := e.Finish(t0, c.to, boom); err != nil || e.State != c.to || e.EndedAt == nil {
			t.Errorf("%s -> %s: %v %+v", c.from, c.to, err, e)
		}
		if (c.to == ExecFailed) != (e.Error != nil) {
			t.Errorf("%s -> %s: error recorded only for failed, got %+v", c.from, c.to, e.Error)
		}
	}
	for _, c := range [][2]ExecState{{ExecQueued, ExecSucceeded}, {ExecQueued, ExecFailed}, {ExecWaiting, ExecSucceeded}} {
		e := Execution{State: c[0]}
		if err := e.Finish(t0, c[1], boom); !errors.Is(err, ErrInvalidTransition) || e.State != c[0] {
			t.Errorf("%s -> %s must be invalid: %v", c[0], c[1], err)
		}
	}
	for _, to := range []ExecState{ExecQueued, ExecRunning, ExecWaiting, "bogus"} {
		e := Execution{State: ExecRunning}
		if err := e.Finish(t0, to, nil); !errors.Is(err, ErrInvalidTransition) {
			t.Errorf("finish into non-terminal %s: %v", to, err)
		}
	}
	e := Execution{State: ExecRunning}
	if err := e.Finish(t0, ExecFailed, nil); !errors.Is(err, ErrErrorRequired) || e.State != ExecRunning {
		t.Errorf("failed needs an error: %v", err)
	}
	for _, s := range []ExecState{ExecSucceeded, ExecFailed, ExecCancelled} {
		e := Execution{State: s}
		if err := e.Finish(t0, ExecCancelled, nil); !errors.Is(err, ErrInvalidTransition) {
			t.Errorf("terminal %s must be final: %v", s, err)
		}
	}
}

func TestExecutionWaitingKeepsStartedAt(t *testing.T) {
	e := Execution{State: ExecQueued}
	_ = e.Start(t0)
	_ = e.Wait()
	_ = e.Resume()
	if e.StartedAt == nil || !e.StartedAt.Equal(t0) {
		t.Fatalf("StartedAt must stay the first start: %v", e.StartedAt)
	}
}
